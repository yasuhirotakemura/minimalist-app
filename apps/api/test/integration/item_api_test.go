//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// response DTO (APIの契約をtest側でも明示するため、生成型ではなく素のstructで受ける)
// ---------------------------------------------------------------------------

type categoryResponseBody struct {
	PublicID    string  `json:"publicId"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	SortOrder   int32   `json:"sortOrder"`
	Version     int32   `json:"version"`
}

type categoryListBody struct {
	Items []categoryResponseBody `json:"items"`
}

type tagResponseBody struct {
	PublicID  string `json:"publicId"`
	Name      string `json:"name"`
	ItemCount int64  `json:"itemCount"`
	Version   int32  `json:"version"`
}

type tagListBody struct {
	Items []tagResponseBody `json:"items"`
}

type categoryReferenceBody struct {
	PublicID string `json:"publicId"`
	Name     string `json:"name"`
}

type tagReferenceBody struct {
	PublicID string `json:"publicId"`
	Name     string `json:"name"`
}

type itemResponseBody struct {
	PublicID            string                `json:"publicId"`
	Name                string                `json:"name"`
	Category            categoryReferenceBody `json:"category"`
	ItemKindCode        string                `json:"itemKindCode"`
	ItemKindLabel       string                `json:"itemKindLabel"`
	Quantity            int32                 `json:"quantity"`
	UnitName            string                `json:"unitName"`
	NecessityLevelCode  string                `json:"necessityLevelCode"`
	NecessityLevelLabel string                `json:"necessityLevelLabel"`
	UsageFrequencyCode  string                `json:"usageFrequencyCode"`
	UsageFrequencyLabel string                `json:"usageFrequencyLabel"`
	PurchasedOn         *string               `json:"purchasedOn"`
	SourceURL           *string               `json:"sourceUrl"`
	Notes               *string               `json:"notes"`
	Tags                []tagReferenceBody    `json:"tags"`
	IsArchived          bool                  `json:"isArchived"`
	ArchivedAt          *string               `json:"archivedAt"`
	Version             int32                 `json:"version"`
	CreatedAt           string                `json:"createdAt"`
	UpdatedAt           string                `json:"updatedAt"`
}

type paginationBody struct {
	Limit      int32 `json:"limit"`
	Offset     int32 `json:"offset"`
	TotalCount int64 `json:"totalCount"`
	HasNext    bool  `json:"hasNext"`
}

type itemListBody struct {
	Items      []itemResponseBody `json:"items"`
	Pagination paginationBody     `json:"pagination"`
}

type dashboardCategoryBreakdownBody struct {
	Category      categoryReferenceBody `json:"category"`
	ItemTypeCount int64                 `json:"itemTypeCount"`
	TotalQuantity int64                 `json:"totalQuantity"`
}

type dashboardCodeBreakdownBody struct {
	Code          string `json:"code"`
	Label         string `json:"label"`
	ItemTypeCount int64  `json:"itemTypeCount"`
	TotalQuantity int64  `json:"totalQuantity"`
}

type dashboardSummaryBody struct {
	ItemTypeCount           int64                            `json:"itemTypeCount"`
	TotalQuantity           int64                            `json:"totalQuantity"`
	CategoryBreakdown       []dashboardCategoryBreakdownBody `json:"categoryBreakdown"`
	NecessityLevelBreakdown []dashboardCodeBreakdownBody     `json:"necessityLevelBreakdown"`
	UsageFrequencyBreakdown []dashboardCodeBreakdownBody     `json:"usageFrequencyBreakdown"`
}

type errorResponseBody struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	FieldErrors []struct {
		Field   string `json:"field"`
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"fieldErrors"`
	RequestID string `json:"requestId"`
}

// ---------------------------------------------------------------------------
// helper
// ---------------------------------------------------------------------------

// signedInClient は登録・loginを済ませたclientを返す。
func signedInClient(t *testing.T, email string) *apiClient {
	t.Helper()

	client := newAPIClient(t)
	client.bootstrapCSRF()

	response := client.do(http.MethodPost, "/api/auth/register", registerPayload(email))
	if response.StatusCode != http.StatusCreated {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("register status = %d (%+v)", response.StatusCode, body)
	}
	_ = response.Body.Close()

	response = client.do(
		http.MethodPost, "/api/auth/login",
		loginPayload(email, "correct-horse-battery"))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	_ = response.Body.Close()

	return client
}

func firstCategory(t *testing.T, client *apiClient) categoryResponseBody {
	t.Helper()

	response := client.do(http.MethodGet, "/api/categories", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("listCategories status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	body := decodeBody[categoryListBody](t, response)
	if len(body.Items) == 0 {
		t.Fatal("既定カテゴリーが作成されていない")
	}
	return body.Items[0]
}

func createItemPayload(categoryPublicID, name string) map[string]any {
	return map[string]any{
		"name":               name,
		"categoryPublicId":   categoryPublicID,
		"quantity":           1,
		"necessityLevelCode": "essential",
		"usageFrequencyCode": "monthly",
	}
}

func createItem(t *testing.T, client *apiClient, payload map[string]any) itemResponseBody {
	t.Helper()

	response := client.do(http.MethodPost, "/api/items", payload)
	if response.StatusCode != http.StatusCreated {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("createItem status = %d (%+v)", response.StatusCode, body)
	}
	return decodeBody[itemResponseBody](t, response)
}

// ---------------------------------------------------------------------------
// カテゴリー
// ---------------------------------------------------------------------------

func TestCategoryAPI_登録時に既定カテゴリーが作成される(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "owner@example.com")

	response := client.do(http.MethodGet, "/api/categories", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	body := decodeBody[categoryListBody](t, response)

	if len(body.Items) == 0 {
		t.Fatal("既定カテゴリーが1件も作成されていない")
	}

	found := false
	previousSortOrder := int32(-1)
	for _, category := range body.Items {
		if category.Name == "外出・携行品" {
			found = true
		}
		if category.PublicID == "" {
			t.Error("publicIdが空")
		}
		if category.SortOrder < previousSortOrder {
			t.Errorf("sortOrderの昇順になっていない: %+v", body.Items)
		}
		previousSortOrder = category.SortOrder
	}
	if !found {
		t.Errorf("既定カテゴリーに 外出・携行品 が含まれない: %+v", body.Items)
	}
}

func TestCategoryAPI_未認証は401(t *testing.T) {
	truncateAll(t)

	client := newAPIClient(t)
	client.bootstrapCSRF()

	response := client.do(http.MethodGet, "/api/categories", nil)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
	}
	body := decodeBody[errorResponseBody](t, response)
	if body.Code != "UNAUTHENTICATED" {
		t.Errorf("code = %q, want UNAUTHENTICATED", body.Code)
	}
}

func TestCategoryAPI_ユーザーごとに分離される(t *testing.T) {
	truncateAll(t)

	owner := signedInClient(t, "owner@example.com")
	other := signedInClient(t, "other@example.com")

	ownerCategory := firstCategory(t, owner)
	otherCategory := firstCategory(t, other)

	if ownerCategory.PublicID == otherCategory.PublicID {
		t.Error("別ユーザーで同一のカテゴリーが返された")
	}
}

// ---------------------------------------------------------------------------
// アイテム
// ---------------------------------------------------------------------------

func TestItemAPI_登録と取得(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "owner@example.com")
	category := firstCategory(t, client)

	payload := createItemPayload(category.PublicID, "折りたたみ傘")
	payload["unitName"] = "本"
	payload["purchasedOn"] = "2026-06-01"
	payload["sourceUrl"] = "https://example.com/items/umbrella"
	payload["notes"] = "駅で買い足さないための最低限の1本"

	created := createItem(t, client, payload)

	if created.Name != "折りたたみ傘" {
		t.Errorf("name = %q, want 折りたたみ傘", created.Name)
	}
	if created.Version != 1 {
		t.Errorf("version = %d, want 1", created.Version)
	}
	if created.Category.PublicID != category.PublicID {
		t.Errorf("category.publicId = %q, want %q", created.Category.PublicID, category.PublicID)
	}
	// 既定値が適用される。
	if created.ItemKindCode != "durable" {
		t.Errorf("itemKindCode = %q, want durable", created.ItemKindCode)
	}
	// labelがcodeと対で返る (設計書 12.6)。
	if created.NecessityLevelLabel != "必須" {
		t.Errorf("necessityLevelLabel = %q, want 必須", created.NecessityLevelLabel)
	}
	if created.UsageFrequencyLabel != "月に1回程度" {
		t.Errorf("usageFrequencyLabel = %q, want 月に1回程度", created.UsageFrequencyLabel)
	}
	if created.IsArchived || created.ArchivedAt != nil {
		t.Errorf("archive状態が不正: %+v", created)
	}
	if created.Tags == nil {
		t.Error("tagsがnull。空配列を返すべき")
	}

	response := client.do(http.MethodGet, "/api/items/"+created.PublicID, nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("getItem status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	found := decodeBody[itemResponseBody](t, response)
	if found.PublicID != created.PublicID {
		t.Errorf("publicId = %q, want %q", found.PublicID, created.PublicID)
	}
	if found.PurchasedOn == nil || *found.PurchasedOn != "2026-06-01" {
		t.Errorf("purchasedOn = %v, want 2026-06-01", found.PurchasedOn)
	}
	if found.SourceURL == nil || *found.SourceURL != "https://example.com/items/umbrella" {
		t.Errorf("sourceUrl = %v", found.SourceURL)
	}
}

func TestItemAPI_未認証は401(t *testing.T) {
	truncateAll(t)

	client := newAPIClient(t)
	client.bootstrapCSRF()

	response := client.do(http.MethodGet, "/api/items", nil)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
	}
}

func TestItemAPI_CSRF_tokenが無い更新は403(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "owner@example.com")
	category := firstCategory(t, client)

	response := client.do(
		http.MethodPost,
		"/api/items",
		createItemPayload(category.PublicID, "折りたたみ傘"),
		func(request *http.Request) { request.Header.Del("X-CSRF-Token") },
	)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}
}

func TestItemAPI_他ユーザーのアイテムは404(t *testing.T) {
	truncateAll(t)

	owner := signedInClient(t, "owner@example.com")
	category := firstCategory(t, owner)
	created := createItem(t, owner, createItemPayload(category.PublicID, "折りたたみ傘"))

	intruder := signedInClient(t, "intruder@example.com")

	t.Run("取得", func(t *testing.T) {
		response := intruder.do(http.MethodGet, "/api/items/"+created.PublicID, nil)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
		}
		body := decodeBody[errorResponseBody](t, response)
		if body.Code != "ITEM_NOT_FOUND" {
			t.Errorf("code = %q, want ITEM_NOT_FOUND", body.Code)
		}
	})

	t.Run("更新", func(t *testing.T) {
		intruderCategory := firstCategory(t, intruder)
		payload := createItemPayload(intruderCategory.PublicID, "乗っ取り")
		payload["expectedVersion"] = created.Version

		response := intruder.do(http.MethodPut, "/api/items/"+created.PublicID, payload)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("archive", func(t *testing.T) {
		response := intruder.do(
			http.MethodPost,
			"/api/items/"+created.PublicID+"/archive",
			map[string]any{"expectedVersion": created.Version},
		)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
		}
	})

	// 所有者からは引き続き取得できる。
	response := owner.do(http.MethodGet, "/api/items/"+created.PublicID, nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("owner getItem status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	_ = response.Body.Close()
}

func TestItemAPI_更新と楽観ロック(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "owner@example.com")
	category := firstCategory(t, client)
	created := createItem(t, client, createItemPayload(category.PublicID, "折りたたみ傘"))

	payload := createItemPayload(category.PublicID, "長傘")
	payload["quantity"] = 2
	payload["expectedVersion"] = created.Version

	response := client.do(http.MethodPut, "/api/items/"+created.PublicID, payload)
	if response.StatusCode != http.StatusOK {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("updateItem status = %d (%+v)", response.StatusCode, body)
	}
	updated := decodeBody[itemResponseBody](t, response)
	if updated.Name != "長傘" || updated.Quantity != 2 {
		t.Errorf("updated = %+v, want 長傘/2", updated)
	}
	if updated.Version != created.Version+1 {
		t.Errorf("version = %d, want %d", updated.Version, created.Version+1)
	}

	// 古いversionでの再更新は409。
	conflictResponse := client.do(http.MethodPut, "/api/items/"+created.PublicID, payload)
	if conflictResponse.StatusCode != http.StatusConflict {
		t.Fatalf("conflict status = %d, want %d",
			conflictResponse.StatusCode, http.StatusConflict)
	}
	conflictBody := decodeBody[errorResponseBody](t, conflictResponse)
	if conflictBody.Code != "ITEM_VERSION_CONFLICT" {
		t.Errorf("code = %q, want ITEM_VERSION_CONFLICT", conflictBody.Code)
	}
}

func TestItemAPI_validation_errorをfield単位で返す(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "owner@example.com")
	category := firstCategory(t, client)

	payload := createItemPayload(category.PublicID, "   ")
	response := client.do(http.MethodPost, "/api/items", payload)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}

	body := decodeBody[errorResponseBody](t, response)
	if len(body.FieldErrors) == 0 || body.FieldErrors[0].Field != "name" {
		t.Errorf("fieldErrors = %+v, want name", body.FieldErrors)
	}
	if body.RequestID == "" {
		t.Error("requestIdが空")
	}
}

func TestItemAPI_未知のカテゴリーは404(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "owner@example.com")

	payload := createItemPayload("018f8d0a-1c2b-7a3d-9e4f-5a6b7c8d9e0f", "折りたたみ傘")
	response := client.do(http.MethodPost, "/api/items", payload)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
	body := decodeBody[errorResponseBody](t, response)
	if body.Code != "CATEGORY_NOT_FOUND" {
		t.Errorf("code = %q, want CATEGORY_NOT_FOUND", body.Code)
	}
}

func TestItemAPI_publicIdの形式が不正なら400(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "owner@example.com")

	response := client.do(http.MethodGet, "/api/items/not-a-uuid", nil)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}
	body := decodeBody[errorResponseBody](t, response)
	if len(body.FieldErrors) == 0 || body.FieldErrors[0].Field != "publicId" {
		t.Errorf("fieldErrors = %+v, want publicId", body.FieldErrors)
	}
}

func TestItemAPI_archiveと復元(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "owner@example.com")
	category := firstCategory(t, client)
	created := createItem(t, client, createItemPayload(category.PublicID, "折りたたみ傘"))
	createItem(t, client, createItemPayload(category.PublicID, "長傘"))

	// version不一致は409。
	conflict := client.do(
		http.MethodPost,
		"/api/items/"+created.PublicID+"/archive",
		map[string]any{"expectedVersion": created.Version + 5},
	)
	if conflict.StatusCode != http.StatusConflict {
		t.Fatalf("archive conflict status = %d, want %d",
			conflict.StatusCode, http.StatusConflict)
	}

	response := client.do(
		http.MethodPost,
		"/api/items/"+created.PublicID+"/archive",
		map[string]any{"expectedVersion": created.Version},
	)
	if response.StatusCode != http.StatusOK {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("archive status = %d (%+v)", response.StatusCode, body)
	}
	archived := decodeBody[itemResponseBody](t, response)
	if !archived.IsArchived || archived.ArchivedAt == nil {
		t.Fatalf("archive状態が反映されていない: %+v", archived)
	}

	// 既定の一覧からは除外される。
	listResponse := client.do(http.MethodGet, "/api/items", nil)
	list := decodeBody[itemListBody](t, listResponse)
	if list.Pagination.TotalCount != 1 {
		t.Errorf("totalCount = %d, want 1", list.Pagination.TotalCount)
	}

	// includeDeleted=true で含まれる。
	listResponse = client.do(http.MethodGet, "/api/items?includeDeleted=true", nil)
	list = decodeBody[itemListBody](t, listResponse)
	if list.Pagination.TotalCount != 2 {
		t.Errorf("totalCount (includeDeleted) = %d, want 2", list.Pagination.TotalCount)
	}

	// archive済みへの更新は422。
	updatePayload := createItemPayload(category.PublicID, "長傘")
	updatePayload["expectedVersion"] = archived.Version
	updateResponse := client.do(
		http.MethodPut, "/api/items/"+created.PublicID, updatePayload)
	if updateResponse.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("update archived status = %d, want %d",
			updateResponse.StatusCode, http.StatusUnprocessableEntity)
	}

	// 復元する。
	restoreResponse := client.do(
		http.MethodPost,
		"/api/items/"+created.PublicID+"/restore",
		map[string]any{"expectedVersion": archived.Version},
	)
	if restoreResponse.StatusCode != http.StatusOK {
		t.Fatalf("restore status = %d, want %d", restoreResponse.StatusCode, http.StatusOK)
	}
	restored := decodeBody[itemResponseBody](t, restoreResponse)
	if restored.IsArchived {
		t.Error("isArchived = true, want false")
	}
}

func TestItemAPI_一覧のfilterとsortとpagination(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "owner@example.com")
	categoriesResponse := client.do(http.MethodGet, "/api/categories", nil)
	categories := decodeBody[categoryListBody](t, categoriesResponse).Items
	if len(categories) < 2 {
		t.Fatalf("既定カテゴリーが2件未満: %d", len(categories))
	}

	umbrella := createItemPayload(categories[0].PublicID, "折りたたみ傘")
	umbrella["quantity"] = 3
	umbrella["notes"] = "雨の日に使う"
	createItem(t, client, umbrella)

	jacket := createItemPayload(categories[1].PublicID, "ジャケット")
	jacket["quantity"] = 1
	jacket["necessityLevelCode"] = "optional"
	jacket["usageFrequencyCode"] = "yearly"
	createItem(t, client, jacket)

	testCases := map[string]struct {
		query     string
		wantNames []string
		wantTotal int64
	}{
		"条件なし": {
			query:     "?sort=name&order=asc",
			wantNames: []string{"ジャケット", "折りたたみ傘"},
			wantTotal: 2,
		},
		"keyword": {
			query:     "?keyword=傘",
			wantNames: []string{"折りたたみ傘"},
			wantTotal: 1,
		},
		"keywordはメモも対象": {
			query:     "?keyword=雨の日",
			wantNames: []string{"折りたたみ傘"},
			wantTotal: 1,
		},
		"necessityLevelCode": {
			query:     "?necessityLevelCode=optional",
			wantNames: []string{"ジャケット"},
			wantTotal: 1,
		},
		"usageFrequencyCode": {
			query:     "?usageFrequencyCode=yearly",
			wantNames: []string{"ジャケット"},
			wantTotal: 1,
		},
		"該当なしのusageFrequencyCode": {
			query:     "?usageFrequencyCode=daily",
			wantNames: nil,
			wantTotal: 0,
		},
		"quantityの降順": {
			query:     "?sort=quantity&order=desc",
			wantNames: []string{"折りたたみ傘", "ジャケット"},
			wantTotal: 2,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			response := client.do(http.MethodGet, "/api/items"+testCase.query, nil)
			if response.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
			}
			list := decodeBody[itemListBody](t, response)

			if list.Pagination.TotalCount != testCase.wantTotal {
				t.Errorf("totalCount = %d, want %d",
					list.Pagination.TotalCount, testCase.wantTotal)
			}
			if len(list.Items) != len(testCase.wantNames) {
				t.Fatalf("len(items) = %d, want %d", len(list.Items), len(testCase.wantNames))
			}
			for index, want := range testCase.wantNames {
				if list.Items[index].Name != want {
					t.Errorf("items[%d].name = %q, want %q",
						index, list.Items[index].Name, want)
				}
			}
		})
	}

	t.Run("categoryPublicIdで絞り込む", func(t *testing.T) {
		response := client.do(
			http.MethodGet, "/api/items?categoryPublicId="+categories[1].PublicID, nil)
		list := decodeBody[itemListBody](t, response)
		if list.Pagination.TotalCount != 1 || list.Items[0].Name != "ジャケット" {
			t.Errorf("list = %+v, want ジャケットのみ", list)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		response := client.do(http.MethodGet, "/api/items?sort=name&order=asc&limit=1", nil)
		list := decodeBody[itemListBody](t, response)

		if list.Pagination.Limit != 1 || list.Pagination.Offset != 0 {
			t.Errorf("pagination = %+v, want limit 1 offset 0", list.Pagination)
		}
		if !list.Pagination.HasNext {
			t.Error("hasNext = false, want true")
		}
		if len(list.Items) != 1 || list.Items[0].Name != "ジャケット" {
			t.Errorf("items = %+v, want ジャケットのみ", list.Items)
		}

		response = client.do(
			http.MethodGet, "/api/items?sort=name&order=asc&limit=1&offset=1", nil)
		list = decodeBody[itemListBody](t, response)
		if list.Pagination.HasNext {
			t.Error("hasNext = true, want false")
		}
		if len(list.Items) != 1 || list.Items[0].Name != "折りたたみ傘" {
			t.Errorf("items = %+v, want 折りたたみ傘のみ", list.Items)
		}
	})

	t.Run("不正なsortは400", func(t *testing.T) {
		response := client.do(http.MethodGet, "/api/items?sort=score", nil)
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("limitの上限超過は400", func(t *testing.T) {
		response := client.do(http.MethodGet, "/api/items?limit=101", nil)
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
		}
	})
}

// ---------------------------------------------------------------------------
// ダッシュボード
// ---------------------------------------------------------------------------

func TestDashboardAPI_集計値を返す(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "owner@example.com")
	categoriesResponse := client.do(http.MethodGet, "/api/categories", nil)
	categories := decodeBody[categoryListBody](t, categoriesResponse).Items
	if len(categories) < 2 {
		t.Fatalf("既定カテゴリーが2件未満: %d", len(categories))
	}

	umbrella := createItemPayload(categories[0].PublicID, "折りたたみ傘")
	umbrella["quantity"] = 2
	createItem(t, client, umbrella)

	jacket := createItemPayload(categories[1].PublicID, "ジャケット")
	jacket["necessityLevelCode"] = "optional"
	jacket["usageFrequencyCode"] = "daily"
	createItem(t, client, jacket)

	// archive済みは集計へ含めない。
	archived := createItem(t, client, createItemPayload(categories[0].PublicID, "使わない傘"))
	archiveResponse := client.do(
		http.MethodPost,
		"/api/items/"+archived.PublicID+"/archive",
		map[string]any{"expectedVersion": archived.Version},
	)
	if archiveResponse.StatusCode != http.StatusOK {
		t.Fatalf("archive status = %d, want %d", archiveResponse.StatusCode, http.StatusOK)
	}
	_ = archiveResponse.Body.Close()

	response := client.do(http.MethodGet, "/api/dashboard/summary", nil)
	if response.StatusCode != http.StatusOK {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("getDashboardSummary status = %d (%+v)", response.StatusCode, body)
	}
	summary := decodeBody[dashboardSummaryBody](t, response)

	if summary.ItemTypeCount != 2 {
		t.Errorf("itemTypeCount = %d, want 2", summary.ItemTypeCount)
	}
	if summary.TotalQuantity != 3 {
		t.Errorf("totalQuantity = %d, want 3", summary.TotalQuantity)
	}

	if len(summary.CategoryBreakdown) != 2 {
		t.Fatalf("len(categoryBreakdown) = %d, want 2", len(summary.CategoryBreakdown))
	}
	if summary.CategoryBreakdown[0].Category.PublicID != categories[0].PublicID {
		t.Errorf("categoryBreakdown[0].category.publicId = %q, want %q",
			summary.CategoryBreakdown[0].Category.PublicID, categories[0].PublicID)
	}
	if summary.CategoryBreakdown[0].TotalQuantity != 2 {
		t.Errorf("categoryBreakdown[0].totalQuantity = %d, want 2",
			summary.CategoryBreakdown[0].TotalQuantity)
	}

	// 必要度はcode体系の定義順 (essential が optional より先) で返る。
	if len(summary.NecessityLevelBreakdown) != 2 {
		t.Fatalf("len(necessityLevelBreakdown) = %d, want 2",
			len(summary.NecessityLevelBreakdown))
	}
	if summary.NecessityLevelBreakdown[0].Code != "essential" {
		t.Errorf("necessityLevelBreakdown[0].code = %q, want essential",
			summary.NecessityLevelBreakdown[0].Code)
	}
	if summary.NecessityLevelBreakdown[0].Label != "必須" {
		t.Errorf("necessityLevelBreakdown[0].label = %q, want 必須",
			summary.NecessityLevelBreakdown[0].Label)
	}

	// 使用頻度も定義順 (daily が monthly より先)。
	if len(summary.UsageFrequencyBreakdown) != 2 {
		t.Fatalf("len(usageFrequencyBreakdown) = %d, want 2",
			len(summary.UsageFrequencyBreakdown))
	}
	if summary.UsageFrequencyBreakdown[0].Code != "daily" {
		t.Errorf("usageFrequencyBreakdown[0].code = %q, want daily",
			summary.UsageFrequencyBreakdown[0].Code)
	}
}

func TestDashboardAPI_未認証は401(t *testing.T) {
	truncateAll(t)

	client := newAPIClient(t)
	client.bootstrapCSRF()

	response := client.do(http.MethodGet, "/api/dashboard/summary", nil)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
	}
	_ = response.Body.Close()
}

func TestDashboardAPI_アイテムが無い場合は0を返す(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "owner@example.com")

	response := client.do(http.MethodGet, "/api/dashboard/summary", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	summary := decodeBody[dashboardSummaryBody](t, response)

	if summary.ItemTypeCount != 0 || summary.TotalQuantity != 0 {
		t.Errorf("summary = %+v, want zero counts", summary)
	}
	// 空配列を返す (nullを返さない)。
	if summary.CategoryBreakdown == nil ||
		summary.NecessityLevelBreakdown == nil ||
		summary.UsageFrequencyBreakdown == nil {
		t.Errorf("内訳がnull。空配列を返すべき: %+v", summary)
	}
}

// ---------------------------------------------------------------------------
// タグ
// ---------------------------------------------------------------------------

func TestTagAPI_CRUD(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "owner@example.com")

	// 登録
	response := client.do(http.MethodPost, "/api/tags", map[string]any{"name": "防災"})
	if response.StatusCode != http.StatusCreated {
		body := decodeBody[errorResponseBody](t, response)
		t.Fatalf("createTag status = %d (%+v)", response.StatusCode, body)
	}
	created := decodeBody[tagResponseBody](t, response)
	if created.Name != "防災" || created.Version != 1 {
		t.Errorf("created = %+v, want 防災/version 1", created)
	}

	// 同名は409
	duplicate := client.do(http.MethodPost, "/api/tags", map[string]any{"name": "防災"})
	if duplicate.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want %d", duplicate.StatusCode, http.StatusConflict)
	}
	duplicateBody := decodeBody[errorResponseBody](t, duplicate)
	if duplicateBody.Code != "TAG_NAME_ALREADY_USED" {
		t.Errorf("code = %q, want TAG_NAME_ALREADY_USED", duplicateBody.Code)
	}

	// 一覧
	listResponse := client.do(http.MethodGet, "/api/tags", nil)
	list := decodeBody[tagListBody](t, listResponse)
	if len(list.Items) != 1 || list.Items[0].ItemCount != 0 {
		t.Errorf("list = %+v, want 1件/付与0件", list)
	}

	// 更新 (version不一致は409)
	conflict := client.do(
		http.MethodPut, "/api/tags/"+created.PublicID,
		map[string]any{"name": "防災用品", "expectedVersion": created.Version + 1})
	if conflict.StatusCode != http.StatusConflict {
		t.Fatalf("update conflict status = %d, want %d", conflict.StatusCode, http.StatusConflict)
	}

	updateResponse := client.do(
		http.MethodPut, "/api/tags/"+created.PublicID,
		map[string]any{"name": "防災用品", "expectedVersion": created.Version})
	if updateResponse.StatusCode != http.StatusOK {
		t.Fatalf("updateTag status = %d, want %d", updateResponse.StatusCode, http.StatusOK)
	}
	updated := decodeBody[tagResponseBody](t, updateResponse)
	if updated.Name != "防災用品" || updated.Version != created.Version+1 {
		t.Errorf("updated = %+v, want 防災用品/version 2", updated)
	}

	// 削除 (expectedVersionはquery parameter)
	deleteResponse := client.do(
		http.MethodDelete,
		"/api/tags/"+created.PublicID+"?expectedVersion=999",
		nil,
	)
	if deleteResponse.StatusCode != http.StatusConflict {
		t.Fatalf("delete conflict status = %d, want %d",
			deleteResponse.StatusCode, http.StatusConflict)
	}

	deleteResponse = client.do(
		http.MethodDelete,
		"/api/tags/"+created.PublicID+"?expectedVersion=2",
		nil,
	)
	if deleteResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("deleteTag status = %d, want %d",
			deleteResponse.StatusCode, http.StatusNoContent)
	}

	listResponse = client.do(http.MethodGet, "/api/tags", nil)
	list = decodeBody[tagListBody](t, listResponse)
	if len(list.Items) != 0 {
		t.Errorf("list = %+v, want empty", list)
	}
}

func TestTagAPI_expectedVersionの省略は400(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "owner@example.com")
	response := client.do(http.MethodPost, "/api/tags", map[string]any{"name": "防災"})
	created := decodeBody[tagResponseBody](t, response)

	deleteResponse := client.do(http.MethodDelete, "/api/tags/"+created.PublicID, nil)
	if deleteResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", deleteResponse.StatusCode, http.StatusBadRequest)
	}
	body := decodeBody[errorResponseBody](t, deleteResponse)
	if len(body.FieldErrors) == 0 || body.FieldErrors[0].Field != "expectedVersion" {
		t.Errorf("fieldErrors = %+v, want expectedVersion", body.FieldErrors)
	}
}

func TestTagAPI_他ユーザーのタグは404(t *testing.T) {
	truncateAll(t)

	owner := signedInClient(t, "owner@example.com")
	response := owner.do(http.MethodPost, "/api/tags", map[string]any{"name": "防災"})
	created := decodeBody[tagResponseBody](t, response)

	intruder := signedInClient(t, "intruder@example.com")
	updateResponse := intruder.do(
		http.MethodPut, "/api/tags/"+created.PublicID,
		map[string]any{"name": "乗っ取り", "expectedVersion": created.Version})
	if updateResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", updateResponse.StatusCode, http.StatusNotFound)
	}
	body := decodeBody[errorResponseBody](t, updateResponse)
	if body.Code != "TAG_NOT_FOUND" {
		t.Errorf("code = %q, want TAG_NOT_FOUND", body.Code)
	}
}

func TestTagAPI_アイテムへの付与と付与件数(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "owner@example.com")
	category := firstCategory(t, client)

	tagResponse := client.do(http.MethodPost, "/api/tags", map[string]any{"name": "防災"})
	tag := decodeBody[tagResponseBody](t, tagResponse)

	payload := createItemPayload(category.PublicID, "非常用ライト")
	payload["tagPublicIds"] = []string{tag.PublicID}
	created := createItem(t, client, payload)

	if len(created.Tags) != 1 || created.Tags[0].Name != "防災" {
		t.Fatalf("tags = %+v, want 防災", created.Tags)
	}

	listResponse := client.do(http.MethodGet, "/api/tags", nil)
	list := decodeBody[tagListBody](t, listResponse)
	if len(list.Items) != 1 || list.Items[0].ItemCount != 1 {
		t.Errorf("list = %+v, want 付与1件", list)
	}

	// タグで絞り込める。
	itemsResponse := client.do(http.MethodGet, "/api/items?tagPublicId="+tag.PublicID, nil)
	items := decodeBody[itemListBody](t, itemsResponse)
	if items.Pagination.TotalCount != 1 {
		t.Errorf("totalCount = %d, want 1", items.Pagination.TotalCount)
	}

	// タグを外す。
	updatePayload := createItemPayload(category.PublicID, "非常用ライト")
	updatePayload["expectedVersion"] = created.Version
	updatePayload["tagPublicIds"] = []string{}
	updateResponse := client.do(http.MethodPut, "/api/items/"+created.PublicID, updatePayload)
	if updateResponse.StatusCode != http.StatusOK {
		t.Fatalf("updateItem status = %d, want %d", updateResponse.StatusCode, http.StatusOK)
	}
	updated := decodeBody[itemResponseBody](t, updateResponse)
	if len(updated.Tags) != 0 {
		t.Errorf("tags = %+v, want empty", updated.Tags)
	}
}

func TestTagAPI_他ユーザーのタグは付与できない(t *testing.T) {
	truncateAll(t)

	owner := signedInClient(t, "owner@example.com")
	response := owner.do(http.MethodPost, "/api/tags", map[string]any{"name": "防災"})
	ownerTag := decodeBody[tagResponseBody](t, response)

	intruder := signedInClient(t, "intruder@example.com")
	intruderCategory := firstCategory(t, intruder)

	payload := createItemPayload(intruderCategory.PublicID, "非常用ライト")
	payload["tagPublicIds"] = []string{ownerTag.PublicID}

	itemResponse := intruder.do(http.MethodPost, "/api/items", payload)
	if itemResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", itemResponse.StatusCode, http.StatusNotFound)
	}
	body := decodeBody[errorResponseBody](t, itemResponse)
	if body.Code != "TAG_NOT_FOUND" {
		t.Errorf("code = %q, want TAG_NOT_FOUND", body.Code)
	}
}

// ---------------------------------------------------------------------------
// 監査ログ
// ---------------------------------------------------------------------------

func TestAuditLog_アイテム操作を記録する(t *testing.T) {
	truncateAll(t)

	client := signedInClient(t, "owner@example.com")
	category := firstCategory(t, client)
	created := createItem(t, client, createItemPayload(category.PublicID, "折りたたみ傘"))

	updatePayload := createItemPayload(category.PublicID, "長傘")
	updatePayload["expectedVersion"] = created.Version
	updateResponse := client.do(http.MethodPut, "/api/items/"+created.PublicID, updatePayload)
	if updateResponse.StatusCode != http.StatusOK {
		t.Fatalf("updateItem status = %d, want %d", updateResponse.StatusCode, http.StatusOK)
	}
	updated := decodeBody[itemResponseBody](t, updateResponse)

	archiveResponse := client.do(
		http.MethodPost,
		"/api/items/"+created.PublicID+"/archive",
		map[string]any{"expectedVersion": updated.Version},
	)
	if archiveResponse.StatusCode != http.StatusOK {
		t.Fatalf("archive status = %d, want %d", archiveResponse.StatusCode, http.StatusOK)
	}
	_ = archiveResponse.Body.Close()

	rows, err := testPool.Query(
		t.Context(),
		`SELECT action_code, target_type_code, changes::text
		 FROM audit.audit_logs
		 ORDER BY id`,
	)
	if err != nil {
		t.Fatalf("query audit logs returned error: %v", err)
	}
	defer rows.Close()

	type auditRow struct {
		action     string
		targetType string
		changes    string
	}
	var logs []auditRow
	for rows.Next() {
		var row auditRow
		if err := rows.Scan(&row.action, &row.targetType, &row.changes); err != nil {
			t.Fatalf("scan audit log returned error: %v", err)
		}
		logs = append(logs, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate audit logs returned error: %v", err)
	}

	if len(logs) != 3 {
		t.Fatalf("len(logs) = %d, want 3 (%+v)", len(logs), logs)
	}
	wantActions := []string{"item_created", "item_updated", "item_archived"}
	for index, want := range wantActions {
		if logs[index].action != want {
			t.Errorf("logs[%d].action = %q, want %q", index, logs[index].action, want)
		}
		if logs[index].targetType != "item" {
			t.Errorf("logs[%d].targetType = %q, want item", index, logs[index].targetType)
		}
	}

	// 更新の差分は変更項目のみを含む。
	if !strings.Contains(logs[1].changes, `"name"`) {
		t.Errorf("update changes = %s, want name", logs[1].changes)
	}
	if strings.Contains(logs[1].changes, `"unitName"`) {
		t.Errorf("update changes = %s, 変更していない項目を含む", logs[1].changes)
	}
}
