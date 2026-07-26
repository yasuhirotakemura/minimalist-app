package item_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domaincategory "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/category"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domainshared "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	domaintag "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/tag"
)

var (
	testNow      = time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	testUserID   = domainauth.UserID(1)
	testPublicID = uuid.MustParse("018f8d0a-1c2b-7a3d-9e4f-5a6b7c8d9e0f")
)

func testCategoryReference() domaincategory.Reference {
	return domaincategory.Reference{
		ID:       domaincategory.CategoryID(10),
		PublicID: uuid.MustParse("018f8d0a-1c2b-7a3d-9e4f-000000000010"),
		Name:     "外出・携行品",
	}
}

// validAttributes は必須項目を満たした属性を返す。
func validAttributes() domainitem.Attributes {
	return domainitem.Attributes{
		Name:             "折りたたみ傘",
		Category:         testCategoryReference(),
		Kind:             domainitem.ItemKindDurable,
		Quantity:         1,
		UnitName:         "本",
		NecessityLevel:   domainitem.NecessityLevelEssential,
		UsageFrequency:   domainitem.UsageFrequencyMonthly,
		Substitutability: domainitem.SubstitutabilityNone,
		MobilityClass:    domainitem.MobilityClassDailyBag,
	}
}

func newTestItem(t *testing.T, attributes domainitem.Attributes) domainitem.Item {
	t.Helper()

	created, err := domainitem.NewItem(testPublicID, testUserID, attributes, testNow)
	if err != nil {
		t.Fatalf("NewItem returned error: %v", err)
	}
	return created
}

func pointerTo[T any](value T) *T { return &value }

func assertFieldError(t *testing.T, err error, field string) {
	t.Helper()

	domainError, ok := domainshared.AsDomainError(err)
	if !ok {
		t.Fatalf("expected DomainError, got %T (%v)", err, err)
	}
	for _, fieldError := range domainError.FieldErrors {
		if fieldError.Field == field {
			return
		}
	}
	t.Fatalf("expected fieldError for %q, got %+v", field, domainError.FieldErrors)
}

func TestNewItem_正常系(t *testing.T) {
	created := newTestItem(t, validAttributes())

	if created.Version() != 1 {
		t.Errorf("Version = %d, want 1", created.Version())
	}
	if created.IsArchived() {
		t.Error("IsArchived = true, want false")
	}
	if created.IsConfirmed() {
		t.Error("IsConfirmed = true, want false")
	}
	if !created.CreatedAt().Equal(testNow) {
		t.Errorf("CreatedAt = %v, want %v", created.CreatedAt(), testNow)
	}
	if created.Name() != "折りたたみ傘" {
		t.Errorf("Name = %q, want 折りたたみ傘", created.Name())
	}
}

func TestNewItem_既定値を適用する(t *testing.T) {
	attributes := validAttributes()
	attributes.Kind = ""
	attributes.UnitName = "   "

	created := newTestItem(t, attributes)

	if got := created.Attributes().Kind; got != domainitem.DefaultItemKind {
		t.Errorf("Kind = %q, want %q", got, domainitem.DefaultItemKind)
	}
	if got := created.Attributes().UnitName; got != domainitem.DefaultUnitName {
		t.Errorf("UnitName = %q, want %q", got, domainitem.DefaultUnitName)
	}
}

func TestNewItem_前後の空白を除去する(t *testing.T) {
	attributes := validAttributes()
	attributes.Name = "  折りたたみ傘  "
	attributes.Notes = pointerTo("   ")

	created := newTestItem(t, attributes)

	if created.Name() != "折りたたみ傘" {
		t.Errorf("Name = %q, want 折りたたみ傘", created.Name())
	}
	if created.Attributes().Notes != nil {
		t.Errorf("Notes = %v, want nil", *created.Attributes().Notes)
	}
}

func TestNewItem_異常系(t *testing.T) {
	testCases := map[string]struct {
		mutate    func(*domainitem.Attributes)
		wantField string
	}{
		"アイテム名が空": {
			mutate:    func(a *domainitem.Attributes) { a.Name = "   " },
			wantField: "name",
		},
		"アイテム名が上限超過": {
			mutate: func(a *domainitem.Attributes) {
				a.Name = strings.Repeat("あ", domainitem.MaxNameLength+1)
			},
			wantField: "name",
		},
		"カテゴリー未指定": {
			mutate:    func(a *domainitem.Attributes) { a.Category = domaincategory.Reference{} },
			wantField: "categoryPublicId",
		},
		"数量が負": {
			mutate:    func(a *domainitem.Attributes) { a.Quantity = -1 },
			wantField: "quantity",
		},
		"数量が上限超過": {
			mutate: func(a *domainitem.Attributes) {
				a.Quantity = domainitem.MaxQuantity + 1
			},
			wantField: "quantity",
		},
		"希望数量が負": {
			mutate:    func(a *domainitem.Attributes) { a.DesiredQuantity = pointerTo(int32(-1)) },
			wantField: "desiredQuantity",
		},
		"単位が上限超過": {
			mutate: func(a *domainitem.Attributes) {
				a.UnitName = strings.Repeat("個", domainitem.MaxUnitNameLength+1)
			},
			wantField: "unitName",
		},
		"必要度が不正": {
			mutate:    func(a *domainitem.Attributes) { a.NecessityLevel = "unknown_level" },
			wantField: "necessityLevelCode",
		},
		"使用頻度が不正": {
			mutate:    func(a *domainitem.Attributes) { a.UsageFrequency = "sometimes" },
			wantField: "usageFrequencyCode",
		},
		"代替可能性が不正": {
			mutate:    func(a *domainitem.Attributes) { a.Substitutability = "maybe" },
			wantField: "substitutabilityCode",
		},
		"携行区分が不正": {
			mutate:    func(a *domainitem.Attributes) { a.MobilityClass = "truck" },
			wantField: "mobilityClassCode",
		},
		"種別が不正": {
			mutate:    func(a *domainitem.Attributes) { a.Kind = "rental" },
			wantField: "itemKindCode",
		},
		"金額が負": {
			mutate:    func(a *domainitem.Attributes) { a.PurchaseAmount = pointerTo(int64(-1)) },
			wantField: "purchaseAmount",
		},
		"重量が負": {
			mutate:    func(a *domainitem.Attributes) { a.WeightGram = pointerTo(int32(-1)) },
			wantField: "weightGram",
		},
		"容積が負": {
			mutate:    func(a *domainitem.Attributes) { a.VolumeMilliliter = pointerTo(int32(-1)) },
			wantField: "volumeMilliliter",
		},
		"商品URLのschemeが不正": {
			mutate:    func(a *domainitem.Attributes) { a.SourceURL = pointerTo("javascript:alert(1)") },
			wantField: "sourceUrl",
		},
		"最終使用日時が未来": {
			mutate: func(a *domainitem.Attributes) {
				a.LastUsedAt = pointerTo(testNow.Add(time.Hour))
			},
			wantField: "lastUsedAt",
		},
		"タグが上限超過": {
			mutate: func(a *domainitem.Attributes) {
				references := make([]domaintag.Reference, 0, domainitem.MaxTagsPerItem+1)
				for index := 0; index <= domainitem.MaxTagsPerItem; index++ {
					references = append(references, domaintag.Reference{
						ID: domaintag.TagID(index + 1), PublicID: uuid.New(), Name: "タグ",
					})
				}
				a.Tags = references
			},
			wantField: "tagPublicIds",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			attributes := validAttributes()
			testCase.mutate(&attributes)

			if _, err := domainitem.NewItem(
				testPublicID, testUserID, attributes, testNow); err == nil {
				t.Fatal("NewItem returned nil error, want error")
			} else {
				assertFieldError(t, err, testCase.wantField)
			}
		})
	}
}

func TestNewItem_境界値(t *testing.T) {
	testCases := map[string]func(*domainitem.Attributes){
		"数量0": func(a *domainitem.Attributes) { a.Quantity = 0 },
		"数量上限": func(a *domainitem.Attributes) {
			a.Quantity = domainitem.MaxQuantity
		},
		"希望数量0": func(a *domainitem.Attributes) { a.DesiredQuantity = pointerTo(int32(0)) },
		"アイテム名上限": func(a *domainitem.Attributes) {
			a.Name = strings.Repeat("あ", domainitem.MaxNameLength)
		},
		"単位上限": func(a *domainitem.Attributes) {
			a.UnitName = strings.Repeat("個", domainitem.MaxUnitNameLength)
		},
		"金額0":         func(a *domainitem.Attributes) { a.PurchaseAmount = pointerTo(int64(0)) },
		"最終使用日時が現在時刻": func(a *domainitem.Attributes) { a.LastUsedAt = pointerTo(testNow) },
		"httpsのURL": func(a *domainitem.Attributes) {
			a.SourceURL = pointerTo("https://example.com/item")
		},
		"httpのURL": func(a *domainitem.Attributes) {
			a.SourceURL = pointerTo("http://example.com/item")
		},
	}

	for name, mutate := range testCases {
		t.Run(name, func(t *testing.T) {
			attributes := validAttributes()
			mutate(&attributes)

			if _, err := domainitem.NewItem(
				testPublicID, testUserID, attributes, testNow); err != nil {
				t.Fatalf("NewItem returned error: %v", err)
			}
		})
	}
}

func TestNewItem_タグの重複を除去する(t *testing.T) {
	reference := domaintag.Reference{
		ID: domaintag.TagID(5), PublicID: uuid.New(), Name: "防災",
	}
	attributes := validAttributes()
	attributes.Tags = []domaintag.Reference{reference, reference}

	created := newTestItem(t, attributes)

	if got := len(created.Tags()); got != 1 {
		t.Errorf("len(Tags) = %d, want 1", got)
	}
}

func TestItem_Update_正常系(t *testing.T) {
	created := newTestItem(t, validAttributes())

	next := validAttributes()
	next.Name = "長傘"
	next.Quantity = 2

	updated, err := created.Update(next, created.Version(), testNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	if updated.Name() != "長傘" {
		t.Errorf("Name = %q, want 長傘", updated.Name())
	}
	if updated.Quantity() != 2 {
		t.Errorf("Quantity = %d, want 2", updated.Quantity())
	}
	if updated.Version() != created.Version()+1 {
		t.Errorf("Version = %d, want %d", updated.Version(), created.Version()+1)
	}
	// 元のEntityは変更されない。
	if created.Name() != "折りたたみ傘" {
		t.Errorf("original Name = %q, want 折りたたみ傘", created.Name())
	}
}

func TestItem_Update_version不一致で競合(t *testing.T) {
	created := newTestItem(t, validAttributes())

	_, err := created.Update(validAttributes(), created.Version()+1, testNow)
	if !errors.Is(err, domainitem.ErrItemVersionConflict) {
		t.Fatalf("Update error = %v, want ErrItemVersionConflict", err)
	}
}

func TestItem_Update_archive済みは編集できない(t *testing.T) {
	created := newTestItem(t, validAttributes())
	archived, err := created.Archive(created.Version(), testNow)
	if err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}

	_, err = archived.Update(validAttributes(), archived.Version(), testNow)
	if !errors.Is(err, domainitem.ErrItemArchived) {
		t.Fatalf("Update error = %v, want ErrItemArchived", err)
	}
}

func TestItem_Archive(t *testing.T) {
	created := newTestItem(t, validAttributes())
	archivedAt := testNow.Add(2 * time.Hour)

	archived, err := created.Archive(created.Version(), archivedAt)
	if err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}

	if !archived.IsArchived() {
		t.Error("IsArchived = false, want true")
	}
	if archived.ArchivedAt() == nil || !archived.ArchivedAt().Equal(archivedAt) {
		t.Errorf("ArchivedAt = %v, want %v", archived.ArchivedAt(), archivedAt)
	}
	if archived.Version() != created.Version()+1 {
		t.Errorf("Version = %d, want %d", archived.Version(), created.Version()+1)
	}

	if _, err := archived.Archive(archived.Version(), archivedAt); !errors.Is(
		err, domainitem.ErrItemAlreadyArchived) {
		t.Fatalf("second Archive error = %v, want ErrItemAlreadyArchived", err)
	}
}

func TestItem_Archive_version不一致で競合(t *testing.T) {
	created := newTestItem(t, validAttributes())

	_, err := created.Archive(created.Version()+1, testNow)
	if !errors.Is(err, domainitem.ErrItemVersionConflict) {
		t.Fatalf("Archive error = %v, want ErrItemVersionConflict", err)
	}
}

func TestItem_Restore(t *testing.T) {
	created := newTestItem(t, validAttributes())
	archived, err := created.Archive(created.Version(), testNow)
	if err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}

	restored, err := archived.Restore(archived.Version(), testNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	if restored.IsArchived() {
		t.Error("IsArchived = true, want false")
	}
	if restored.ArchivedAt() != nil {
		t.Errorf("ArchivedAt = %v, want nil", restored.ArchivedAt())
	}

	if _, err := restored.Restore(restored.Version(), testNow); !errors.Is(
		err, domainitem.ErrItemNotArchived) {
		t.Fatalf("Restore on active item error = %v, want ErrItemNotArchived", err)
	}
}

func TestItem_RecordUsage_最終使用日時を更新する(t *testing.T) {
	attributes := validAttributes()
	attributes.LastUsedAt = pointerTo(testNow.Add(-48 * time.Hour))
	created := newTestItem(t, attributes)

	usedAt := testNow.Add(-time.Hour)
	record, updated, err := created.RecordUsage(
		uuid.New(), usedAt, 1, pointerTo("通勤で使用"), testNow)
	if err != nil {
		t.Fatalf("RecordUsage returned error: %v", err)
	}

	if !record.UsedAt().Equal(usedAt) {
		t.Errorf("record.UsedAt = %v, want %v", record.UsedAt(), usedAt)
	}
	if updated.LastUsedAt() == nil || !updated.LastUsedAt().Equal(usedAt) {
		t.Errorf("LastUsedAt = %v, want %v", updated.LastUsedAt(), usedAt)
	}
	if updated.Version() != created.Version()+1 {
		t.Errorf("Version = %d, want %d", updated.Version(), created.Version()+1)
	}
}

func TestItem_RecordUsage_古い使用日時では後退させない(t *testing.T) {
	latest := testNow.Add(-time.Hour)
	attributes := validAttributes()
	attributes.LastUsedAt = pointerTo(latest)
	created := newTestItem(t, attributes)

	_, updated, err := created.RecordUsage(
		uuid.New(), testNow.Add(-72*time.Hour), 1, nil, testNow)
	if err != nil {
		t.Fatalf("RecordUsage returned error: %v", err)
	}

	if updated.LastUsedAt() == nil || !updated.LastUsedAt().Equal(latest) {
		t.Errorf("LastUsedAt = %v, want %v (unchanged)", updated.LastUsedAt(), latest)
	}
}

func TestItem_RecordUsage_archive済みへは追加できない(t *testing.T) {
	created := newTestItem(t, validAttributes())
	archived, err := created.Archive(created.Version(), testNow)
	if err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}

	_, _, err = archived.RecordUsage(uuid.New(), testNow, 1, nil, testNow)
	if !errors.Is(err, domainitem.ErrItemArchived) {
		t.Fatalf("RecordUsage error = %v, want ErrItemArchived", err)
	}
}

func TestItem_AuditSnapshot_機微でない項目を含む(t *testing.T) {
	created := newTestItem(t, validAttributes())

	snapshot := created.AuditSnapshot()

	if snapshot["name"] != "折りたたみ傘" {
		t.Errorf("snapshot[name] = %v, want 折りたたみ傘", snapshot["name"])
	}
	if snapshot["quantity"] != int32(1) {
		t.Errorf("snapshot[quantity] = %v, want 1", snapshot["quantity"])
	}
	if snapshot["isArchived"] != false {
		t.Errorf("snapshot[isArchived] = %v, want false", snapshot["isArchived"])
	}
	if _, ok := snapshot["categoryPublicId"]; !ok {
		t.Error("snapshot does not contain categoryPublicId")
	}
}
