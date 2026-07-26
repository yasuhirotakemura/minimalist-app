package category_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domaincategory "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/category"
	domainshared "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

var (
	testNow      = time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	testUserID   = domainauth.UserID(1)
	testPublicID = uuid.MustParse("018f8d0a-1c2b-7a3d-9e4f-5a6b7c8d9e0f")
)

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

func TestNewCategory_正常系(t *testing.T) {
	created, err := domaincategory.NewCategory(
		testPublicID, testUserID, "  外出・携行品  ", pointerTo("  外出時に持ち出す物  "), 30, testNow)
	if err != nil {
		t.Fatalf("NewCategory returned error: %v", err)
	}

	if created.Name() != "外出・携行品" {
		t.Errorf("Name = %q, want 外出・携行品", created.Name())
	}
	if created.Description() == nil || *created.Description() != "外出時に持ち出す物" {
		t.Errorf("Description = %v, want 外出時に持ち出す物", created.Description())
	}
	if created.SortOrder() != 30 {
		t.Errorf("SortOrder = %d, want 30", created.SortOrder())
	}
	if created.Version() != 1 {
		t.Errorf("Version = %d, want 1", created.Version())
	}
	if created.IsDeleted() {
		t.Error("IsDeleted = true, want false")
	}
	if !created.ID().IsZero() {
		t.Error("ID should be zero before persistence")
	}
}

func TestNewCategory_説明が空文字ならnil(t *testing.T) {
	created, err := domaincategory.NewCategory(
		testPublicID, testUserID, "衣類", pointerTo("   "), 0, testNow)
	if err != nil {
		t.Fatalf("NewCategory returned error: %v", err)
	}
	if created.Description() != nil {
		t.Errorf("Description = %v, want nil", *created.Description())
	}
}

func TestNewCategory_異常系(t *testing.T) {
	testCases := map[string]struct {
		name        string
		description *string
		sortOrder   int32
		wantField   string
	}{
		"名称が空": {
			name: "   ", sortOrder: 0, wantField: "name",
		},
		"名称が上限超過": {
			name:      strings.Repeat("あ", domaincategory.MaxNameLength+1),
			sortOrder: 0,
			wantField: "name",
		},
		"説明が上限超過": {
			name:        "衣類",
			description: pointerTo(strings.Repeat("あ", domaincategory.MaxDescriptionLength+1)),
			sortOrder:   0,
			wantField:   "description",
		},
		"表示順が負": {
			name: "衣類", sortOrder: -1, wantField: "sortOrder",
		},
	}

	for label, testCase := range testCases {
		t.Run(label, func(t *testing.T) {
			_, err := domaincategory.NewCategory(
				testPublicID, testUserID, testCase.name,
				testCase.description, testCase.sortOrder, testNow)
			if err == nil {
				t.Fatal("NewCategory returned nil error, want error")
			}
			assertFieldError(t, err, testCase.wantField)
		})
	}
}

func TestNewCategory_境界値(t *testing.T) {
	testCases := map[string]struct {
		name        string
		description *string
		sortOrder   int32
	}{
		"名称が1文字": {name: "服"},
		"名称が上限": {
			name: strings.Repeat("あ", domaincategory.MaxNameLength),
		},
		"説明が上限": {
			name:        "衣類",
			description: pointerTo(strings.Repeat("あ", domaincategory.MaxDescriptionLength)),
		},
		"表示順が0": {name: "衣類", sortOrder: 0},
	}

	for label, testCase := range testCases {
		t.Run(label, func(t *testing.T) {
			if _, err := domaincategory.NewCategory(
				testPublicID, testUserID, testCase.name,
				testCase.description, testCase.sortOrder, testNow); err != nil {
				t.Fatalf("NewCategory returned error: %v", err)
			}
		})
	}
}

func TestNewCategory_識別子の欠落は内部エラー(t *testing.T) {
	if _, err := domaincategory.NewCategory(
		uuid.Nil, testUserID, "衣類", nil, 0, testNow); err == nil {
		t.Error("NewCategory(publicID=nil) returned nil error, want error")
	}
	if _, err := domaincategory.NewCategory(
		testPublicID, domainauth.UserID(0), "衣類", nil, 0, testNow); err == nil {
		t.Error("NewCategory(userID=0) returned nil error, want error")
	}
}

func TestCategory_Reference(t *testing.T) {
	created, err := domaincategory.NewCategory(
		testPublicID, testUserID, "衣類", nil, 10, testNow)
	if err != nil {
		t.Fatalf("NewCategory returned error: %v", err)
	}
	stored := created.WithID(domaincategory.CategoryID(7))

	reference := stored.Reference()
	if reference.ID != domaincategory.CategoryID(7) {
		t.Errorf("Reference.ID = %d, want 7", reference.ID)
	}
	if reference.PublicID != testPublicID {
		t.Errorf("Reference.PublicID = %v, want %v", reference.PublicID, testPublicID)
	}
	if reference.Name != "衣類" {
		t.Errorf("Reference.Name = %q, want 衣類", reference.Name)
	}
	if reference.IsZero() {
		t.Error("Reference.IsZero = true, want false")
	}
	if !(domaincategory.Reference{}).IsZero() {
		t.Error("empty Reference.IsZero = false, want true")
	}
}

func TestDefaultCategoryDefinitions(t *testing.T) {
	definitions := domaincategory.DefaultCategoryDefinitions()

	if len(definitions) == 0 {
		t.Fatal("DefaultCategoryDefinitions returned no definitions")
	}

	// 設計書 12.6 のresponse例に登場する分類を含む。
	found := false
	names := make(map[string]struct{}, len(definitions))
	sortOrders := make(map[int32]struct{}, len(definitions))
	for _, definition := range definitions {
		if definition.Name == "外出・携行品" {
			found = true
		}
		if _, duplicated := names[definition.Name]; duplicated {
			t.Errorf("duplicated default category name %q", definition.Name)
		}
		names[definition.Name] = struct{}{}

		if _, duplicated := sortOrders[definition.SortOrder]; duplicated {
			t.Errorf("duplicated sortOrder %d", definition.SortOrder)
		}
		sortOrders[definition.SortOrder] = struct{}{}

		// 全定義がEntityとして成立することを確認する。
		if _, err := domaincategory.NewCategory(
			testPublicID, testUserID, definition.Name,
			pointerTo(definition.Description), definition.SortOrder, testNow); err != nil {
			t.Errorf("default category %q is invalid: %v", definition.Name, err)
		}
	}
	if !found {
		t.Error("default categories do not contain 外出・携行品")
	}
}

func TestDefaultCategoryDefinitions_呼び出し側の変更が影響しない(t *testing.T) {
	first := domaincategory.DefaultCategoryDefinitions()
	first[0].Name = "書き換え"

	second := domaincategory.DefaultCategoryDefinitions()
	if second[0].Name == "書き換え" {
		t.Error("DefaultCategoryDefinitions returned a shared slice")
	}
}
