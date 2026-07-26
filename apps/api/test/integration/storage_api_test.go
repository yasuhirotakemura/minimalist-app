//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"testing"
)

// ---------------------------------------------------------------------------
// response DTO (APIの契約をtest側でも明示するため、生成型ではなく素のstructで受ける)
// ---------------------------------------------------------------------------

type storageUnitReferenceBody struct {
	PublicID string `json:"publicId"`
	Name     string `json:"name"`
}

type capacityBody struct {
	AllocatedItemKindCount     int32  `json:"allocatedItemKindCount"`
	AllocatedQuantity          int64  `json:"allocatedQuantity"`
	TareWeightGram             int64  `json:"tareWeightGram"`
	ItemWeightGram             int64  `json:"itemWeightGram"`
	DescendantWeightGram       int64  `json:"descendantWeightGram"`
	TotalWeightGram            int64  `json:"totalWeightGram"`
	ItemVolumeMilliliter       int64  `json:"itemVolumeMilliliter"`
	DescendantVolumeMilliliter int64  `json:"descendantVolumeMilliliter"`
	TotalVolumeMilliliter      int64  `json:"totalVolumeMilliliter"`
	MaximumWeightGram          *int32 `json:"maximumWeightGram"`
	MaximumVolumeMilliliter    *int32 `json:"maximumVolumeMilliliter"`
	RemainingWeightGram        *int64 `json:"remainingWeightGram"`
	RemainingVolumeMilliliter  *int64 `json:"remainingVolumeMilliliter"`
	IsWeightExceeded           bool   `json:"isWeightExceeded"`
	IsVolumeExceeded           bool   `json:"isVolumeExceeded"`
	HasUnknownWeight           bool   `json:"hasUnknownWeight"`
	HasUnknownVolume           bool   `json:"hasUnknownVolume"`
}

type storageUnitResponseBody struct {
	PublicID                string                     `json:"publicId"`
	Name                    string                     `json:"name"`
	StorageTypeCode         string                     `json:"storageTypeCode"`
	StorageTypeLabel        string                     `json:"storageTypeLabel"`
	MobilityClassCode       string                     `json:"mobilityClassCode"`
	MobilityClassLabel      string                     `json:"mobilityClassLabel"`
	Parent                  *storageUnitReferenceBody  `json:"parent"`
	Ancestors               []storageUnitReferenceBody `json:"ancestors"`
	Depth                   int32                      `json:"depth"`
	ChildCount              int32                      `json:"childCount"`
	TareWeightGram          *int32                     `json:"tareWeightGram"`
	MaximumWeightGram       *int32                     `json:"maximumWeightGram"`
	MaximumVolumeMilliliter *int32                     `json:"maximumVolumeMilliliter"`
	Description             *string                    `json:"description"`
	SortOrder               int32                      `json:"sortOrder"`
	Capacity                capacityBody               `json:"capacity"`
	IsArchived              bool                       `json:"isArchived"`
	ArchivedAt              *string                    `json:"archivedAt"`
	Version                 int32                      `json:"version"`
	CreatedAt               string                     `json:"createdAt"`
	UpdatedAt               string                     `json:"updatedAt"`
}

type storageUnitListBody struct {
	Items      []storageUnitResponseBody `json:"items"`
	Pagination paginationBody            `json:"pagination"`
}

type allocatedItemBody struct {
	PublicID           string `json:"publicId"`
	Name               string `json:"name"`
	UnitName           string `json:"unitName"`
	Quantity           int32  `json:"quantity"`
	AssignedQuantity   int32  `json:"assignedQuantity"`
	UnassignedQuantity int32  `json:"unassignedQuantity"`
	WeightGram         *int32 `json:"weightGram"`
	VolumeMilliliter   *int32 `json:"volumeMilliliter"`
	IsArchived         bool   `json:"isArchived"`
}

type storageAllocationBody struct {
	PublicID string            `json:"publicId"`
	Item     allocatedItemBody `json:"item"`
	Quantity int32             `json:"quantity"`
	Version  int32             `json:"version"`
}

type storageUnitContentsBody struct {
	StorageUnit       storageUnitResponseBody   `json:"storageUnit"`
	Allocations       []storageAllocationBody   `json:"allocations"`
	ChildStorageUnits []storageUnitResponseBody `json:"childStorageUnits"`
}

type itemStorageAllocationBody struct {
	PublicID    string                   `json:"publicId"`
	StorageUnit storageUnitReferenceBody `json:"storageUnit"`
	Quantity    int32                    `json:"quantity"`
	Version     int32                    `json:"version"`
}

type itemStorageAllocationListBody struct {
	Items              []itemStorageAllocationBody `json:"items"`
	Quantity           int32                       `json:"quantity"`
	AssignedQuantity   int32                       `json:"assignedQuantity"`
	UnassignedQuantity int32                       `json:"unassignedQuantity"`
}

// ---------------------------------------------------------------------------
// helper
// ---------------------------------------------------------------------------

func createStorageUnitPayload(name string) map[string]any {
	return map[string]any{
		"name":              name,
		"storageTypeCode":   "bag",
		"mobilityClassCode": "daily_bag",
	}
}

func createStorageUnit(
	t *testing.T, client *apiClient, payload map[string]any,
) storageUnitResponseBody {
	t.Helper()

	response := client.do(http.MethodPost, "/api/storage-units", payload)
	if response.StatusCode != http.StatusCreated {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("createStorageUnit status = %d (%+v)", response.StatusCode, body)
	}
	return decodeBody[storageUnitResponseBody](t, response)
}

// assignItem は割当を1件追加し、更新後の収納内容を返す。
func assignItem(
	t *testing.T,
	client *apiClient,
	unitPublicID string,
	itemPublicID string,
	quantity int,
	expectedStorageUnitVersion int32,
) storageUnitContentsBody {
	t.Helper()

	response := client.do(
		http.MethodPost,
		"/api/storage-units/"+unitPublicID+"/allocations",
		map[string]any{
			"itemPublicId":               itemPublicID,
			"quantity":                   quantity,
			"expectedStorageUnitVersion": expectedStorageUnitVersion,
		},
	)
	if response.StatusCode != http.StatusCreated {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("createStorageAllocation status = %d (%+v)", response.StatusCode, body)
	}
	return decodeBody[storageUnitContentsBody](t, response)
}

// createItemWithMeasures は重量・容積を指定したアイテムを作成する。
func createItemWithMeasures(
	t *testing.T,
	client *apiClient,
	categoryPublicID string,
	name string,
	quantity int,
	weightGram any,
	volumeMilliliter any,
) itemResponseBody {
	t.Helper()

	payload := createItemPayload(categoryPublicID, name)
	payload["quantity"] = quantity
	payload["weightGram"] = weightGram
	payload["volumeMilliliter"] = volumeMilliliter
	return createItem(t, client, payload)
}

// ---------------------------------------------------------------------------
// 収納単位CRUD
// ---------------------------------------------------------------------------

func TestStorageUnitAPI_登録と取得(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-create@example.com")

	payload := createStorageUnitPayload("日常リュック")
	payload["tareWeightGram"] = 900
	payload["maximumWeightGram"] = 8000
	payload["maximumVolumeMilliliter"] = 25000
	payload["description"] = "通勤で毎日持ち出す"
	payload["sortOrder"] = 10

	created := createStorageUnit(t, client, payload)

	if created.Name != "日常リュック" {
		t.Errorf("Name = %q, want 日常リュック", created.Name)
	}
	if created.StorageTypeLabel == "" || created.MobilityClassLabel == "" {
		t.Error("labelが空である")
	}
	if created.Depth != 1 || created.Parent != nil {
		t.Errorf("Depth/Parent = %d/%v, want 1/nil", created.Depth, created.Parent)
	}
	if created.Version != 1 {
		t.Errorf("Version = %d, want 1", created.Version)
	}
	// 中身が無く自重が既知のため、集計は完全である。
	if created.Capacity.TotalWeightGram != 900 || created.Capacity.HasUnknownWeight {
		t.Errorf("capacity = %+v, want totalWeightGram=900 hasUnknownWeight=false",
			created.Capacity)
	}

	response := client.do(http.MethodGet, "/api/storage-units/"+created.PublicID, nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("getStorageUnit status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	fetched := decodeBody[storageUnitResponseBody](t, response)
	if fetched.PublicID != created.PublicID {
		t.Errorf("PublicID = %s, want %s", fetched.PublicID, created.PublicID)
	}
}

func TestStorageUnitAPI_未認証は401(t *testing.T) {
	truncateAll(t)

	client := newAPIClient(t)
	response := client.do(http.MethodGet, "/api/storage-units", nil)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
	}
	_ = response.Body.Close()
}

func TestStorageUnitAPI_validation_errorをfield単位で返す(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-validation@example.com")

	payload := createStorageUnitPayload("")
	response := client.do(http.MethodPost, "/api/storage-units", payload)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}
	body := decodeBody[errorResponseBody](t, response)
	if len(body.FieldErrors) == 0 || body.FieldErrors[0].Field != "name" {
		t.Errorf("fieldErrors = %+v, want name", body.FieldErrors)
	}
}

func TestStorageUnitAPI_他ユーザーの収納単位は404(t *testing.T) {
	truncateAll(t)

	owner := signedInClient(t, "storage-owner@example.com")
	intruder := signedInClient(t, "storage-intruder@example.com")

	unit := createStorageUnit(t, owner, createStorageUnitPayload("日常リュック"))

	// 他ユーザーのpublicIdを指定しても存在有無を推測させない (設計書 18.3)。
	for _, path := range []string{
		"/api/storage-units/" + unit.PublicID,
		"/api/storage-units/" + unit.PublicID + "/contents",
		"/api/storage-units/" + unit.PublicID + "/capacity",
	} {
		response := intruder.do(http.MethodGet, path, nil)
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", path, response.StatusCode,
				http.StatusNotFound)
		}
		_ = response.Body.Close()
	}

	response := intruder.do(
		http.MethodPut, "/api/storage-units/"+unit.PublicID,
		map[string]any{
			"name":              "乗っ取り",
			"storageTypeCode":   "bag",
			"mobilityClassCode": "daily_bag",
			"expectedVersion":   unit.Version,
		})
	if response.StatusCode != http.StatusNotFound {
		t.Errorf("PUT status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
	_ = response.Body.Close()
}

func TestStorageUnitAPI_更新と楽観ロック(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-update@example.com")
	unit := createStorageUnit(t, client, createStorageUnitPayload("日常リュック"))

	updatePayload := map[string]any{
		"name":              "通勤リュック",
		"storageTypeCode":   "bag",
		"mobilityClassCode": "daily_bag",
		"tareWeightGram":    1000,
		"expectedVersion":   unit.Version,
	}
	response := client.do(
		http.MethodPut, "/api/storage-units/"+unit.PublicID, updatePayload)
	if response.StatusCode != http.StatusOK {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("update status = %d (%+v)", response.StatusCode, body)
	}
	updated := decodeBody[storageUnitResponseBody](t, response)
	if updated.Name != "通勤リュック" || updated.Version != unit.Version+1 {
		t.Errorf("updated = %s/v%d, want 通勤リュック/v%d",
			updated.Name, updated.Version, unit.Version+1)
	}

	// 古いversionでの再更新は409。
	response = client.do(
		http.MethodPut, "/api/storage-units/"+unit.PublicID, updatePayload)
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("conflict status = %d, want %d", response.StatusCode, http.StatusConflict)
	}
	body := decodeBody[errorResponseBody](t, response)
	if body.Code != "STORAGE_UNIT_VERSION_CONFLICT" {
		t.Errorf("code = %s, want STORAGE_UNIT_VERSION_CONFLICT", body.Code)
	}
}

// ---------------------------------------------------------------------------
// 階層
// ---------------------------------------------------------------------------

func TestStorageUnitAPI_3階層まで作成でき4階層目は422(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-hierarchy@example.com")

	rootPayload := createStorageUnitPayload("部屋")
	rootPayload["storageTypeCode"] = "room"
	root := createStorageUnit(t, client, rootPayload)

	middlePayload := createStorageUnitPayload("日常リュック")
	middlePayload["parentStorageUnitPublicId"] = root.PublicID
	middle := createStorageUnit(t, client, middlePayload)
	if middle.Depth != 2 {
		t.Errorf("Depth = %d, want 2", middle.Depth)
	}

	leafPayload := createStorageUnitPayload("ガジェットポーチ")
	leafPayload["storageTypeCode"] = "pouch"
	leafPayload["parentStorageUnitPublicId"] = middle.PublicID
	leaf := createStorageUnit(t, client, leafPayload)
	if leaf.Depth != 3 {
		t.Errorf("Depth = %d, want 3", leaf.Depth)
	}
	if len(leaf.Ancestors) != 2 ||
		leaf.Ancestors[0].PublicID != root.PublicID ||
		leaf.Ancestors[1].PublicID != middle.PublicID {
		t.Errorf("Ancestors = %+v, want [root, middle]", leaf.Ancestors)
	}

	fourthPayload := createStorageUnitPayload("4段目")
	fourthPayload["parentStorageUnitPublicId"] = leaf.PublicID
	response := client.do(http.MethodPost, "/api/storage-units", fourthPayload)
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnprocessableEntity)
	}
	body := decodeBody[errorResponseBody](t, response)
	if body.Code != "STORAGE_HIERARCHY_TOO_DEEP" {
		t.Errorf("code = %s, want STORAGE_HIERARCHY_TOO_DEEP", body.Code)
	}
}

func TestStorageUnitAPI_自己参照と循環参照を拒否する(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-cycle@example.com")

	root := createStorageUnit(t, client, createStorageUnitPayload("日常リュック"))
	childPayload := createStorageUnitPayload("ガジェットポーチ")
	childPayload["parentStorageUnitPublicId"] = root.PublicID
	child := createStorageUnit(t, client, childPayload)

	// 自分自身を親に指定する。
	response := client.do(
		http.MethodPut, "/api/storage-units/"+root.PublicID,
		map[string]any{
			"name":                      "日常リュック",
			"storageTypeCode":           "bag",
			"mobilityClassCode":         "daily_bag",
			"parentStorageUnitPublicId": root.PublicID,
			"expectedVersion":           root.Version,
		})
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("self parent status = %d, want %d",
			response.StatusCode, http.StatusUnprocessableEntity)
	}
	body := decodeBody[errorResponseBody](t, response)
	if body.Code != "STORAGE_UNIT_SELF_PARENT" {
		t.Errorf("code = %s, want STORAGE_UNIT_SELF_PARENT", body.Code)
	}

	// 子を親に指定する (循環参照)。
	response = client.do(
		http.MethodPut, "/api/storage-units/"+root.PublicID,
		map[string]any{
			"name":                      "日常リュック",
			"storageTypeCode":           "bag",
			"mobilityClassCode":         "daily_bag",
			"parentStorageUnitPublicId": child.PublicID,
			"expectedVersion":           root.Version,
		})
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("circular status = %d, want %d",
			response.StatusCode, http.StatusUnprocessableEntity)
	}
	body = decodeBody[errorResponseBody](t, response)
	if body.Code != "STORAGE_UNIT_CIRCULAR_PARENT" {
		t.Errorf("code = %s, want STORAGE_UNIT_CIRCULAR_PARENT", body.Code)
	}
}

func TestStorageUnitAPI_他ユーザーの収納単位は親にできない(t *testing.T) {
	truncateAll(t)

	owner := signedInClient(t, "storage-parent-owner@example.com")
	intruder := signedInClient(t, "storage-parent-intruder@example.com")

	ownerUnit := createStorageUnit(t, owner, createStorageUnitPayload("日常リュック"))

	payload := createStorageUnitPayload("ポーチ")
	payload["parentStorageUnitPublicId"] = ownerUnit.PublicID
	response := intruder.do(http.MethodPost, "/api/storage-units", payload)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
	_ = response.Body.Close()
}

// ---------------------------------------------------------------------------
// archive / restore
// ---------------------------------------------------------------------------

func TestStorageUnitAPI_子や中身が残る収納単位はarchiveできない(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-archive-guard@example.com")
	category := firstCategory(t, client)

	parent := createStorageUnit(t, client, createStorageUnitPayload("日常リュック"))
	childPayload := createStorageUnitPayload("ガジェットポーチ")
	childPayload["parentStorageUnitPublicId"] = parent.PublicID
	child := createStorageUnit(t, client, childPayload)

	// 子が残っているためarchiveできない。親のarchiveで子を暗黙にarchiveしない。
	response := client.do(
		http.MethodPost, "/api/storage-units/"+parent.PublicID+"/archive",
		map[string]any{"expectedVersion": parent.Version})
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnprocessableEntity)
	}
	body := decodeBody[errorResponseBody](t, response)
	if body.Code != "STORAGE_UNIT_HAS_CHILDREN" {
		t.Errorf("code = %s, want STORAGE_UNIT_HAS_CHILDREN", body.Code)
	}

	// 中身が残っているためarchiveできない。
	item := createItemWithMeasures(t, client, category.PublicID, "充電器", 1, 200, 300)
	contents := assignItem(t, client, child.PublicID, item.PublicID, 1, child.Version)

	response = client.do(
		http.MethodPost, "/api/storage-units/"+child.PublicID+"/archive",
		map[string]any{"expectedVersion": contents.StorageUnit.Version})
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnprocessableEntity)
	}
	body = decodeBody[errorResponseBody](t, response)
	if body.Code != "STORAGE_UNIT_HAS_ALLOCATIONS" {
		t.Errorf("code = %s, want STORAGE_UNIT_HAS_ALLOCATIONS", body.Code)
	}
}

func TestStorageUnitAPI_archiveと復元(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-archive@example.com")
	unit := createStorageUnit(t, client, createStorageUnitPayload("使わない箱"))

	response := client.do(
		http.MethodPost, "/api/storage-units/"+unit.PublicID+"/archive",
		map[string]any{"expectedVersion": unit.Version})
	if response.StatusCode != http.StatusOK {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("archive status = %d (%+v)", response.StatusCode, body)
	}
	archived := decodeBody[storageUnitResponseBody](t, response)
	if !archived.IsArchived || archived.ArchivedAt == nil {
		t.Errorf("archived = %+v, want isArchived=true", archived)
	}

	// 既定の一覧はarchive済みを除外する。
	response = client.do(http.MethodGet, "/api/storage-units", nil)
	list := decodeBody[storageUnitListBody](t, response)
	if len(list.Items) != 0 {
		t.Errorf("items = %d, want 0", len(list.Items))
	}

	response = client.do(http.MethodGet, "/api/storage-units?includeArchived=true", nil)
	list = decodeBody[storageUnitListBody](t, response)
	if len(list.Items) != 1 {
		t.Errorf("items = %d, want 1", len(list.Items))
	}

	response = client.do(
		http.MethodPost, "/api/storage-units/"+unit.PublicID+"/restore",
		map[string]any{"expectedVersion": archived.Version})
	if response.StatusCode != http.StatusOK {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("restore status = %d (%+v)", response.StatusCode, body)
	}
	restored := decodeBody[storageUnitResponseBody](t, response)
	if restored.IsArchived {
		t.Error("IsArchived = true, want false")
	}
}

func TestStorageUnitAPI_親がarchive済みの場合は復元できない(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-restore-guard@example.com")

	parent := createStorageUnit(t, client, createStorageUnitPayload("日常リュック"))
	childPayload := createStorageUnitPayload("ガジェットポーチ")
	childPayload["parentStorageUnitPublicId"] = parent.PublicID
	child := createStorageUnit(t, client, childPayload)

	// 子 -> 親の順にarchiveする。
	response := client.do(
		http.MethodPost, "/api/storage-units/"+child.PublicID+"/archive",
		map[string]any{"expectedVersion": child.Version})
	archivedChild := decodeBody[storageUnitResponseBody](t, response)

	response = client.do(
		http.MethodPost, "/api/storage-units/"+parent.PublicID+"/archive",
		map[string]any{"expectedVersion": parent.Version})
	if response.StatusCode != http.StatusOK {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("archive parent status = %d (%+v)", response.StatusCode, body)
	}
	_ = response.Body.Close()

	// 親がarchive済みのため子だけを復元できない。
	response = client.do(
		http.MethodPost, "/api/storage-units/"+child.PublicID+"/restore",
		map[string]any{"expectedVersion": archivedChild.Version})
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnprocessableEntity)
	}
	body := decodeBody[errorResponseBody](t, response)
	if body.Code != "STORAGE_UNIT_PARENT_ARCHIVED" {
		t.Errorf("code = %s, want STORAGE_UNIT_PARENT_ARCHIVED", body.Code)
	}
}

// ---------------------------------------------------------------------------
// 収納割当
// ---------------------------------------------------------------------------

func TestStorageAllocationAPI_割当と分割と未割当数量(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-allocation@example.com")
	category := firstCategory(t, client)

	bag := createStorageUnit(t, client, createStorageUnitPayload("衣服圧縮バッグ"))
	backpackPayload := createStorageUnitPayload("日常リュック")
	backpackPayload["sortOrder"] = 20
	backpack := createStorageUnit(t, client, backpackPayload)

	shirt := createItemWithMeasures(t, client, category.PublicID, "半袖シャツ", 3, 150, 800)

	// 同一アイテムを2つの収納単位へ分割して割り当てる。
	first := assignItem(t, client, bag.PublicID, shirt.PublicID, 2, bag.Version)
	if first.Allocations[0].Item.UnassignedQuantity != 1 {
		t.Errorf("UnassignedQuantity = %d, want 1",
			first.Allocations[0].Item.UnassignedQuantity)
	}

	second := assignItem(t, client, backpack.PublicID, shirt.PublicID, 1, backpack.Version)
	if second.Allocations[0].Item.AssignedQuantity != 3 ||
		second.Allocations[0].Item.UnassignedQuantity != 0 {
		t.Errorf("assigned/unassigned = %d/%d, want 3/0",
			second.Allocations[0].Item.AssignedQuantity,
			second.Allocations[0].Item.UnassignedQuantity)
	}

	// アイテム側からも割当を確認できる。
	response := client.do(
		http.MethodGet, "/api/items/"+shirt.PublicID+"/storage-allocations", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	allocations := decodeBody[itemStorageAllocationListBody](t, response)
	if len(allocations.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(allocations.Items))
	}
	if allocations.AssignedQuantity != 3 || allocations.UnassignedQuantity != 0 {
		t.Errorf("assigned/unassigned = %d/%d, want 3/0",
			allocations.AssignedQuantity, allocations.UnassignedQuantity)
	}
}

func TestStorageAllocationAPI_所有数量を超える割当は422(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-quantity@example.com")
	category := firstCategory(t, client)

	unit := createStorageUnit(t, client, createStorageUnitPayload("衣服圧縮バッグ"))
	shirt := createItemWithMeasures(t, client, category.PublicID, "半袖シャツ", 2, 150, 800)

	response := client.do(
		http.MethodPost, "/api/storage-units/"+unit.PublicID+"/allocations",
		map[string]any{
			"itemPublicId":               shirt.PublicID,
			"quantity":                   3,
			"expectedStorageUnitVersion": unit.Version,
		})
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnprocessableEntity)
	}
	body := decodeBody[errorResponseBody](t, response)
	if body.Code != "STORAGE_ALLOCATION_EXCEEDS_QUANTITY" {
		t.Errorf("code = %s, want STORAGE_ALLOCATION_EXCEEDS_QUANTITY", body.Code)
	}
}

func TestStorageAllocationAPI_同一収納単位への重複割当は409(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-duplicate@example.com")
	category := firstCategory(t, client)

	unit := createStorageUnit(t, client, createStorageUnitPayload("衣服圧縮バッグ"))
	shirt := createItemWithMeasures(t, client, category.PublicID, "半袖シャツ", 5, 150, 800)

	contents := assignItem(t, client, unit.PublicID, shirt.PublicID, 1, unit.Version)

	response := client.do(
		http.MethodPost, "/api/storage-units/"+unit.PublicID+"/allocations",
		map[string]any{
			"itemPublicId":               shirt.PublicID,
			"quantity":                   1,
			"expectedStorageUnitVersion": contents.StorageUnit.Version,
		})
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusConflict)
	}
	body := decodeBody[errorResponseBody](t, response)
	if body.Code != "STORAGE_ALLOCATION_ALREADY_EXISTS" {
		t.Errorf("code = %s, want STORAGE_ALLOCATION_ALREADY_EXISTS", body.Code)
	}
}

func TestStorageAllocationAPI_数量変更と削除(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-allocation-edit@example.com")
	category := firstCategory(t, client)

	unit := createStorageUnit(t, client, createStorageUnitPayload("衣服圧縮バッグ"))
	shirt := createItemWithMeasures(t, client, category.PublicID, "半袖シャツ", 5, 150, 800)
	contents := assignItem(t, client, unit.PublicID, shirt.PublicID, 2, unit.Version)
	allocation := contents.Allocations[0]

	allocationPath := fmt.Sprintf("/api/storage-units/%s/allocations/%s",
		unit.PublicID, allocation.PublicID)

	response := client.do(http.MethodPut, allocationPath, map[string]any{
		"quantity":                   4,
		"expectedVersion":            allocation.Version,
		"expectedStorageUnitVersion": contents.StorageUnit.Version,
	})
	if response.StatusCode != http.StatusOK {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("update status = %d (%+v)", response.StatusCode, body)
	}
	updated := decodeBody[storageUnitContentsBody](t, response)
	if updated.Allocations[0].Quantity != 4 {
		t.Errorf("Quantity = %d, want 4", updated.Allocations[0].Quantity)
	}

	// 古いversionでの再更新は409。
	response = client.do(http.MethodPut, allocationPath, map[string]any{
		"quantity":                   3,
		"expectedVersion":            allocation.Version,
		"expectedStorageUnitVersion": updated.StorageUnit.Version,
	})
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("conflict status = %d, want %d", response.StatusCode, http.StatusConflict)
	}
	_ = response.Body.Close()

	deletePath := fmt.Sprintf(
		"%s?expectedVersion=%d&expectedStorageUnitVersion=%d",
		allocationPath, updated.Allocations[0].Version, updated.StorageUnit.Version)
	response = client.do(http.MethodDelete, deletePath, nil)
	if response.StatusCode != http.StatusOK {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("delete status = %d (%+v)", response.StatusCode, body)
	}
	afterDelete := decodeBody[storageUnitContentsBody](t, response)
	if len(afterDelete.Allocations) != 0 {
		t.Errorf("allocations = %d, want 0", len(afterDelete.Allocations))
	}
}

func TestStorageAllocationAPI_一括置換(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-replace@example.com")
	category := firstCategory(t, client)

	unit := createStorageUnit(t, client, createStorageUnitPayload("衣服圧縮バッグ"))
	shirt := createItemWithMeasures(t, client, category.PublicID, "半袖シャツ", 5, 150, 800)
	towel := createItemWithMeasures(t, client, category.PublicID, "タオル", 2, 300, 1200)

	contents := assignItem(t, client, unit.PublicID, shirt.PublicID, 2, unit.Version)

	response := client.do(
		http.MethodPut, "/api/storage-units/"+unit.PublicID+"/allocations",
		map[string]any{
			"allocations": []map[string]any{
				{"itemPublicId": shirt.PublicID, "quantity": 3},
				{"itemPublicId": towel.PublicID, "quantity": 2},
			},
			"expectedStorageUnitVersion": contents.StorageUnit.Version,
		})
	if response.StatusCode != http.StatusOK {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("replace status = %d (%+v)", response.StatusCode, body)
	}
	replaced := decodeBody[storageUnitContentsBody](t, response)
	if len(replaced.Allocations) != 2 {
		t.Fatalf("allocations = %d, want 2", len(replaced.Allocations))
	}

	// 空配列で中身を空にできる。
	response = client.do(
		http.MethodPut, "/api/storage-units/"+unit.PublicID+"/allocations",
		map[string]any{
			"allocations":                []map[string]any{},
			"expectedStorageUnitVersion": replaced.StorageUnit.Version,
		})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("empty replace status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	emptied := decodeBody[storageUnitContentsBody](t, response)
	if len(emptied.Allocations) != 0 {
		t.Errorf("allocations = %d, want 0", len(emptied.Allocations))
	}
}

func TestStorageAllocationAPI_一括置換の数量違反でrollbackする(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-replace-rollback@example.com")
	category := firstCategory(t, client)

	bag := createStorageUnit(t, client, createStorageUnitPayload("衣服圧縮バッグ"))
	backpackPayload := createStorageUnitPayload("日常リュック")
	backpackPayload["sortOrder"] = 20
	backpack := createStorageUnit(t, client, backpackPayload)

	shirt := createItemWithMeasures(t, client, category.PublicID, "半袖シャツ", 3, 150, 800)

	assignItem(t, client, backpack.PublicID, shirt.PublicID, 2, backpack.Version)
	contents := assignItem(t, client, bag.PublicID, shirt.PublicID, 1, bag.Version)

	// 他収納へ2枚割当済みのため、本収納へ2枚は所有数量3を超える。
	response := client.do(
		http.MethodPut, "/api/storage-units/"+bag.PublicID+"/allocations",
		map[string]any{
			"allocations": []map[string]any{
				{"itemPublicId": shirt.PublicID, "quantity": 2},
			},
			"expectedStorageUnitVersion": contents.StorageUnit.Version,
		})
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnprocessableEntity)
	}
	_ = response.Body.Close()

	// rollbackにより既存の割当が残っている。
	response = client.do(
		http.MethodGet, "/api/storage-units/"+bag.PublicID+"/contents", nil)
	current := decodeBody[storageUnitContentsBody](t, response)
	if len(current.Allocations) != 1 || current.Allocations[0].Quantity != 1 {
		t.Errorf("allocations = %+v, want 1件 quantity=1", current.Allocations)
	}
}

func TestStorageAllocationAPI_収納単位のversion競合で409(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-allocation-conflict@example.com")
	category := firstCategory(t, client)

	unit := createStorageUnit(t, client, createStorageUnitPayload("衣服圧縮バッグ"))
	shirt := createItemWithMeasures(t, client, category.PublicID, "半袖シャツ", 5, 150, 800)

	response := client.do(
		http.MethodPost, "/api/storage-units/"+unit.PublicID+"/allocations",
		map[string]any{
			"itemPublicId":               shirt.PublicID,
			"quantity":                   1,
			"expectedStorageUnitVersion": unit.Version + 3,
		})
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusConflict)
	}
	_ = response.Body.Close()
}

func TestStorageAllocationAPI_他ユーザーのアイテムは割り当てられない(t *testing.T) {
	truncateAll(t)

	owner := signedInClient(t, "storage-alloc-owner@example.com")
	intruder := signedInClient(t, "storage-alloc-intruder@example.com")

	intruderCategory := firstCategory(t, intruder)
	intruderItem := createItemWithMeasures(
		t, intruder, intruderCategory.PublicID, "他人のシャツ", 3, 150, 800)

	ownerUnit := createStorageUnit(t, owner, createStorageUnitPayload("衣服圧縮バッグ"))

	response := owner.do(
		http.MethodPost, "/api/storage-units/"+ownerUnit.PublicID+"/allocations",
		map[string]any{
			"itemPublicId":               intruderItem.PublicID,
			"quantity":                   1,
			"expectedStorageUnitVersion": ownerUnit.Version,
		})
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
	_ = response.Body.Close()
}

// ---------------------------------------------------------------------------
// 容量集計
// ---------------------------------------------------------------------------

func TestStorageCapacityAPI_子孫を含めて集計し二重計上しない(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-capacity@example.com")
	category := firstCategory(t, client)

	backpackPayload := createStorageUnitPayload("日常リュック")
	backpackPayload["tareWeightGram"] = 900
	backpack := createStorageUnit(t, client, backpackPayload)

	pouchPayload := createStorageUnitPayload("ガジェットポーチ")
	pouchPayload["storageTypeCode"] = "pouch"
	pouchPayload["tareWeightGram"] = 100
	pouchPayload["parentStorageUnitPublicId"] = backpack.PublicID
	pouch := createStorageUnit(t, client, pouchPayload)

	laptop := createItemWithMeasures(t, client, category.PublicID, "ノートPC", 1, 1500, 3000)
	charger := createItemWithMeasures(t, client, category.PublicID, "充電器", 2, 200, 150)

	backpackContents := assignItem(
		t, client, backpack.PublicID, laptop.PublicID, 1, backpack.Version)
	assignItem(t, client, pouch.PublicID, charger.PublicID, 2, pouch.Version)
	_ = backpackContents

	response := client.do(
		http.MethodGet, "/api/storage-units/"+backpack.PublicID+"/capacity", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	capacity := decodeBody[capacityBody](t, response)

	if capacity.ItemWeightGram != 1500 {
		t.Errorf("ItemWeightGram = %d, want 1500", capacity.ItemWeightGram)
	}
	// 子の自重100 + 充電器200*2
	if capacity.DescendantWeightGram != 500 {
		t.Errorf("DescendantWeightGram = %d, want 500", capacity.DescendantWeightGram)
	}
	// 900 + 1500 + 500
	if capacity.TotalWeightGram != 2900 {
		t.Errorf("TotalWeightGram = %d, want 2900", capacity.TotalWeightGram)
	}
	// 3000 + 150*2
	if capacity.TotalVolumeMilliliter != 3300 {
		t.Errorf("TotalVolumeMilliliter = %d, want 3300", capacity.TotalVolumeMilliliter)
	}
	// 直接割当のみを数える。
	if capacity.AllocatedItemKindCount != 1 {
		t.Errorf("AllocatedItemKindCount = %d, want 1", capacity.AllocatedItemKindCount)
	}
	if capacity.HasUnknownWeight || capacity.HasUnknownVolume {
		t.Error("HasUnknown* = true, want false")
	}
}

func TestStorageCapacityAPI_未設定の重量を0として扱わない(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-unknown@example.com")
	category := firstCategory(t, client)

	unitPayload := createStorageUnitPayload("書類ケース")
	unitPayload["storageTypeCode"] = "box"
	unitPayload["tareWeightGram"] = 300
	unit := createStorageUnit(t, client, unitPayload)

	// 重量未設定のアイテムを割り当てる。
	passport := createItemWithMeasures(t, client, category.PublicID, "パスポート", 1, nil, nil)
	notebook := createItemWithMeasures(t, client, category.PublicID, "ノート", 2, 180, nil)

	contents := assignItem(t, client, unit.PublicID, passport.PublicID, 1, unit.Version)
	contents = assignItem(
		t, client, unit.PublicID, notebook.PublicID, 2, contents.StorageUnit.Version)

	capacity := contents.StorageUnit.Capacity
	// 既知分のみを合計する。
	if capacity.ItemWeightGram != 360 {
		t.Errorf("ItemWeightGram = %d, want 360", capacity.ItemWeightGram)
	}
	if !capacity.HasUnknownWeight {
		t.Error("HasUnknownWeight = false, want true")
	}
	if !capacity.HasUnknownVolume {
		t.Error("HasUnknownVolume = false, want true")
	}
}

func TestStorageCapacityAPI_最大重量の超過を判定する(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-exceeded@example.com")
	category := firstCategory(t, client)

	unitPayload := createStorageUnitPayload("日常リュック")
	unitPayload["tareWeightGram"] = 900
	unitPayload["maximumWeightGram"] = 2000
	unitPayload["maximumVolumeMilliliter"] = 10000
	unit := createStorageUnit(t, client, unitPayload)

	brick := createItemWithMeasures(t, client, category.PublicID, "重い本", 2, 900, 1000)
	contents := assignItem(t, client, unit.PublicID, brick.PublicID, 2, unit.Version)

	capacity := contents.StorageUnit.Capacity
	// 900 + 900*2 = 2700 > 2000
	if !capacity.IsWeightExceeded {
		t.Errorf("IsWeightExceeded = false, want true (total=%d)", capacity.TotalWeightGram)
	}
	if capacity.RemainingWeightGram == nil || *capacity.RemainingWeightGram != -700 {
		t.Errorf("RemainingWeightGram = %v, want -700", capacity.RemainingWeightGram)
	}
	if capacity.IsVolumeExceeded {
		t.Error("IsVolumeExceeded = true, want false")
	}
}

// ---------------------------------------------------------------------------
// 一覧のfilter / sort / pagination
// ---------------------------------------------------------------------------

func TestStorageUnitAPI_一覧のfilterとsortとpagination(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-list@example.com")

	rootPayload := createStorageUnitPayload("部屋")
	rootPayload["storageTypeCode"] = "room"
	rootPayload["mobilityClassCode"] = "fixed"
	rootPayload["sortOrder"] = 10
	root := createStorageUnit(t, client, rootPayload)

	bagPayload := createStorageUnitPayload("日常リュック")
	bagPayload["parentStorageUnitPublicId"] = root.PublicID
	bagPayload["sortOrder"] = 20
	bagPayload["description"] = "通勤で使う"
	bag := createStorageUnit(t, client, bagPayload)

	pouchPayload := createStorageUnitPayload("ガジェットポーチ")
	pouchPayload["storageTypeCode"] = "pouch"
	pouchPayload["parentStorageUnitPublicId"] = bag.PublicID
	pouchPayload["sortOrder"] = 30
	createStorageUnit(t, client, pouchPayload)

	testCases := map[string]struct {
		query     string
		wantCount int
	}{
		"絞り込みなし":           {query: "", wantCount: 3},
		"rootOnly":         {query: "?rootOnly=true", wantCount: 1},
		"親で絞り込む":           {query: "?parentStorageUnitPublicId=" + root.PublicID, wantCount: 1},
		"storageTypeCode":  {query: "?storageTypeCode=pouch", wantCount: 1},
		"mobilityClassCode": {query: "?mobilityClassCode=fixed", wantCount: 1},
		"keyword_名前":       {query: "?keyword=リュック", wantCount: 1},
		"keyword_説明":       {query: "?keyword=通勤", wantCount: 1},
		"該当なし":             {query: "?keyword=存在しない", wantCount: 0},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			response := client.do(http.MethodGet, "/api/storage-units"+testCase.query, nil)
			if response.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
			}
			body := decodeBody[storageUnitListBody](t, response)
			if len(body.Items) != testCase.wantCount {
				t.Errorf("items = %d, want %d", len(body.Items), testCase.wantCount)
			}
		})
	}

	// rootOnly と親指定の同時指定は400。
	response := client.do(
		http.MethodGet,
		"/api/storage-units?rootOnly=true&parentStorageUnitPublicId="+root.PublicID, nil)
	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}
	_ = response.Body.Close()

	// pagination
	response = client.do(http.MethodGet, "/api/storage-units?limit=2&offset=0", nil)
	page := decodeBody[storageUnitListBody](t, response)
	if len(page.Items) != 2 || page.Pagination.TotalCount != 3 || !page.Pagination.HasNext {
		t.Errorf("pagination = %+v (items=%d), want totalCount=3 hasNext=true",
			page.Pagination, len(page.Items))
	}

	// sort=name / order=desc
	response = client.do(http.MethodGet, "/api/storage-units?sort=name&order=desc", nil)
	sorted := decodeBody[storageUnitListBody](t, response)
	if len(sorted.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(sorted.Items))
	}
	for index := 1; index < len(sorted.Items); index++ {
		if sorted.Items[index-1].Name < sorted.Items[index].Name {
			t.Errorf("name desc order broken: %v", []string{
				sorted.Items[index-1].Name, sorted.Items[index].Name})
		}
	}
}

// ---------------------------------------------------------------------------
// 所持品側との連携
// ---------------------------------------------------------------------------

func TestItemAPI_収納単位と未割当で絞り込む(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-item-filter@example.com")
	category := firstCategory(t, client)

	unit := createStorageUnit(t, client, createStorageUnitPayload("衣服圧縮バッグ"))

	stored := createItemWithMeasures(t, client, category.PublicID, "収納済み", 1, 100, 100)
	partial := createItemWithMeasures(t, client, category.PublicID, "一部収納", 3, 100, 100)
	createItemWithMeasures(t, client, category.PublicID, "未収納", 1, 100, 100)

	contents := assignItem(t, client, unit.PublicID, stored.PublicID, 1, unit.Version)
	assignItem(t, client, unit.PublicID, partial.PublicID, 1, contents.StorageUnit.Version)

	response := client.do(
		http.MethodGet, "/api/items?storageUnitPublicId="+unit.PublicID, nil)
	byUnit := decodeBody[itemListBody](t, response)
	if len(byUnit.Items) != 2 {
		t.Errorf("storageUnitPublicId items = %d, want 2", len(byUnit.Items))
	}

	response = client.do(http.MethodGet, "/api/items?isUnassigned=true", nil)
	unassigned := decodeBody[itemListBody](t, response)
	// 「一部収納」(残2) と「未収納」(残1) が該当する。
	if len(unassigned.Items) != 2 {
		t.Errorf("isUnassigned items = %d, want 2", len(unassigned.Items))
	}
}

func TestItemAPI_割当数量合計未満へ所有数量を減らせない(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-item-quantity@example.com")
	category := firstCategory(t, client)

	unit := createStorageUnit(t, client, createStorageUnitPayload("衣服圧縮バッグ"))
	shirt := createItemWithMeasures(t, client, category.PublicID, "半袖シャツ", 3, 150, 800)
	assignItem(t, client, unit.PublicID, shirt.PublicID, 3, unit.Version)

	response := client.do(http.MethodGet, "/api/items/"+shirt.PublicID, nil)
	current := decodeBody[itemResponseBody](t, response)

	payload := createItemPayload(category.PublicID, "半袖シャツ")
	payload["quantity"] = 2
	payload["expectedVersion"] = current.Version

	response = client.do(http.MethodPut, "/api/items/"+shirt.PublicID, payload)
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnprocessableEntity)
	}
	body := decodeBody[errorResponseBody](t, response)
	if body.Code != "STORAGE_ALLOCATION_EXCEEDS_QUANTITY" {
		t.Errorf("code = %s, want STORAGE_ALLOCATION_EXCEEDS_QUANTITY", body.Code)
	}
}

// ---------------------------------------------------------------------------
// 監査ログ
// ---------------------------------------------------------------------------

func TestAuditLog_収納操作を記録する(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-audit@example.com")
	category := firstCategory(t, client)

	unit := createStorageUnit(t, client, createStorageUnitPayload("衣服圧縮バッグ"))
	shirt := createItemWithMeasures(t, client, category.PublicID, "半袖シャツ", 3, 150, 800)
	contents := assignItem(t, client, unit.PublicID, shirt.PublicID, 1, unit.Version)

	// 楽観ロック競合も記録する。
	response := client.do(
		http.MethodPut, "/api/storage-units/"+unit.PublicID,
		map[string]any{
			"name":              "改名",
			"storageTypeCode":   "bag",
			"mobilityClassCode": "daily_bag",
			"expectedVersion":   unit.Version,
		})
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("conflict status = %d, want %d", response.StatusCode, http.StatusConflict)
	}
	_ = response.Body.Close()
	_ = contents

	for _, action := range []string{
		"storage_unit_created",
		"storage_allocation_created",
		"version_conflict_detected",
	} {
		var count int64
		if err := testPool.QueryRow(
			t.Context(),
			`SELECT count(*) FROM audit.audit_logs WHERE action_code = $1`,
			action,
		).Scan(&count); err != nil {
			t.Fatalf("count audit logs: %v", err)
		}
		if count == 0 {
			t.Errorf("audit log %q が記録されていない", action)
		}
	}
}

// ---------------------------------------------------------------------------
// E2E相当のscenario
//
// 設計書 28章の Phase 7 でPlaywrightによるE2Eを実装するまで、
// 指示されたE2E手順をAPI levelで通しで検証する。
// ---------------------------------------------------------------------------

func TestStorageScenario_収納単位の作成から他ユーザー分離まで(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "storage-scenario@example.com")
	intruder := signedInClient(t, "storage-scenario-intruder@example.com")
	category := firstCategory(t, client)

	// 1. 収納単位を作成する。
	parentPayload := createStorageUnitPayload("日常リュック")
	parentPayload["tareWeightGram"] = 900
	parentPayload["maximumWeightGram"] = 3000
	parent := createStorageUnit(t, client, parentPayload)

	// 2. 子収納単位を作成する。
	childPayload := createStorageUnitPayload("ガジェットポーチ")
	childPayload["storageTypeCode"] = "pouch"
	childPayload["tareWeightGram"] = 100
	childPayload["parentStorageUnitPublicId"] = parent.PublicID
	child := createStorageUnit(t, client, childPayload)
	if child.Depth != 2 {
		t.Fatalf("child depth = %d, want 2", child.Depth)
	}

	// 3. 所持品を割り当てる。
	shirt := createItemWithMeasures(t, client, category.PublicID, "半袖シャツ", 3, 400, 900)
	parentContents := assignItem(t, client, parent.PublicID, shirt.PublicID, 2, parent.Version)

	// 4. 同じ所持品を複数収納へ分割する。
	childContents := assignItem(t, client, child.PublicID, shirt.PublicID, 1, child.Version)

	// 5. 未割当数量を確認する。
	if childContents.Allocations[0].Item.UnassignedQuantity != 0 {
		t.Errorf("UnassignedQuantity = %d, want 0",
			childContents.Allocations[0].Item.UnassignedQuantity)
	}

	// 6. 容量超過を確認する。
	// 親: 900 + 400*2 + 子孫 (100 + 400*1) = 2200 <= 3000 なので未超過。
	response := client.do(
		http.MethodGet, "/api/storage-units/"+parent.PublicID+"/capacity", nil)
	capacity := decodeBody[capacityBody](t, response)
	if capacity.TotalWeightGram != 2200 {
		t.Errorf("TotalWeightGram = %d, want 2200", capacity.TotalWeightGram)
	}
	if capacity.IsWeightExceeded {
		t.Error("IsWeightExceeded = true, want false")
	}

	// 7. 割当を更新して超過させる。
	response = client.do(http.MethodGet, "/api/storage-units/"+parent.PublicID, nil)
	currentParent := decodeBody[storageUnitResponseBody](t, response)

	allocationPath := fmt.Sprintf("/api/storage-units/%s/allocations/%s",
		parent.PublicID, parentContents.Allocations[0].PublicID)
	response = client.do(http.MethodPut, allocationPath, map[string]any{
		"quantity":                   2,
		"expectedVersion":            parentContents.Allocations[0].Version,
		"expectedStorageUnitVersion": currentParent.Version,
	})
	if response.StatusCode != http.StatusOK {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("update allocation status = %d (%+v)", response.StatusCode, body)
	}
	_ = response.Body.Close()

	// 最大重量を下げて超過状態を作る。
	response = client.do(http.MethodGet, "/api/storage-units/"+parent.PublicID, nil)
	currentParent = decodeBody[storageUnitResponseBody](t, response)
	response = client.do(
		http.MethodPut, "/api/storage-units/"+parent.PublicID,
		map[string]any{
			"name":              "日常リュック",
			"storageTypeCode":   "bag",
			"mobilityClassCode": "daily_bag",
			"tareWeightGram":    900,
			"maximumWeightGram": 1500,
			"expectedVersion":   currentParent.Version,
		})
	if response.StatusCode != http.StatusOK {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("shrink maximum status = %d (%+v)", response.StatusCode, body)
	}
	exceeded := decodeBody[storageUnitResponseBody](t, response)
	if !exceeded.Capacity.IsWeightExceeded {
		t.Errorf("IsWeightExceeded = false, want true (total=%d max=%v)",
			exceeded.Capacity.TotalWeightGram, exceeded.Capacity.MaximumWeightGram)
	}

	// 8. 収納単位をarchiveする。中身と子を先に片付ける必要がある。
	response = client.do(
		http.MethodGet, "/api/storage-units/"+child.PublicID+"/contents", nil)
	childCurrent := decodeBody[storageUnitContentsBody](t, response)

	deletePath := fmt.Sprintf(
		"/api/storage-units/%s/allocations/%s?expectedVersion=%d&expectedStorageUnitVersion=%d",
		child.PublicID,
		childCurrent.Allocations[0].PublicID,
		childCurrent.Allocations[0].Version,
		childCurrent.StorageUnit.Version)
	response = client.do(http.MethodDelete, deletePath, nil)
	if response.StatusCode != http.StatusOK {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("delete allocation status = %d (%+v)", response.StatusCode, body)
	}
	emptiedChild := decodeBody[storageUnitContentsBody](t, response)

	response = client.do(
		http.MethodPost, "/api/storage-units/"+child.PublicID+"/archive",
		map[string]any{"expectedVersion": emptiedChild.StorageUnit.Version})
	if response.StatusCode != http.StatusOK {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("archive child status = %d (%+v)", response.StatusCode, body)
	}
	_ = response.Body.Close()

	// 9. 他ユーザーから参照できない。
	for _, path := range []string{
		"/api/storage-units/" + parent.PublicID,
		"/api/storage-units/" + child.PublicID + "/contents",
	} {
		response := intruder.do(http.MethodGet, path, nil)
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("intruder GET %s status = %d, want %d",
				path, response.StatusCode, http.StatusNotFound)
		}
		_ = response.Body.Close()
	}
}
