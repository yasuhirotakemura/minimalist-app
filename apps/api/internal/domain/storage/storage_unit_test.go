package storage_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domainshared "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
)

var (
	testNow      = time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	testUserID   = domainauth.UserID(1)
	otherUserID  = domainauth.UserID(2)
	testPublicID = uuid.MustParse("018f8d0a-1c2b-7a3d-9e4f-5a6b7c8d9e00")
)

func pointerTo[T any](value T) *T { return &value }

func publicIDOf(suffix string) uuid.UUID {
	return uuid.MustParse("018f8d0a-1c2b-7a3d-9e4f-5a6b7c8d9e" + suffix)
}

// validAttributes は必須項目を満たした属性を返す。
func validAttributes() domainstorage.Attributes {
	return domainstorage.Attributes{
		Name:          "日常リュック",
		StorageType:   domainstorage.StorageTypeBag,
		MobilityClass: domainitem.MobilityClassDailyBag,
	}
}

// newPersistedUnit は永続化済みの収納単位を組み立てる。
func newPersistedUnit(
	id domainstorage.StorageUnitID,
	name string,
	ancestors []domainstorage.Reference,
) domainstorage.StorageUnit {
	attributes := validAttributes()
	attributes.Name = name
	if len(ancestors) > 0 {
		attributes.Parent = ancestors[len(ancestors)-1]
	}

	return domainstorage.ReconstructStorageUnit(domainstorage.ReconstructStorageUnitParams{
		ID:         id,
		PublicID:   publicIDOf(pad(int(id))),
		UserID:     testUserID,
		Attributes: attributes,
		Ancestors:  ancestors,
		CreatedAt:  testNow,
		UpdatedAt:  testNow,
		Version:    1,
	})
}

func pad(value int) string {
	digits := "0123456789abcdef"
	return string([]byte{digits[(value/16)%16], digits[value%16]})
}

func assertFieldError(t *testing.T, err error, field string) {
	t.Helper()

	domainError, ok := domainshared.AsDomainError(err)
	if !ok {
		t.Fatalf("expected DomainError, got %v", err)
	}
	if domainError.Kind != domainshared.KindInvalidInput {
		t.Fatalf("expected KindInvalidInput, got %v", domainError.Kind)
	}
	for _, fieldError := range domainError.FieldErrors {
		if fieldError.Field == field {
			return
		}
	}
	t.Fatalf("expected field error for %q, got %+v", field, domainError.FieldErrors)
}

func TestNewStorageUnitCreatesRootUnit(t *testing.T) {
	attributes := validAttributes()
	attributes.TareWeightGram = pointerTo(int32(900))
	attributes.MaximumWeightGram = pointerTo(int32(8000))
	attributes.SortOrder = 10

	unit, err := domainstorage.NewStorageUnit(
		testPublicID, testUserID, attributes, nil, testNow)
	if err != nil {
		t.Fatalf("NewStorageUnit returned error: %v", err)
	}

	if unit.Depth() != 1 {
		t.Errorf("Depth = %d, want 1", unit.Depth())
	}
	if unit.HasParent() {
		t.Error("HasParent = true, want false")
	}
	if unit.Version() != 1 {
		t.Errorf("Version = %d, want 1", unit.Version())
	}
	if unit.IsArchived() {
		t.Error("IsArchived = true, want false")
	}
	if len(unit.Ancestors()) != 0 {
		t.Errorf("Ancestors = %v, want empty", unit.Ancestors())
	}
}

func TestNewStorageUnitTrimsNameAndDescription(t *testing.T) {
	attributes := validAttributes()
	attributes.Name = "  ガジェットポーチ  "
	attributes.Description = pointerTo("   ")

	unit, err := domainstorage.NewStorageUnit(
		testPublicID, testUserID, attributes, nil, testNow)
	if err != nil {
		t.Fatalf("NewStorageUnit returned error: %v", err)
	}

	if unit.Name() != "ガジェットポーチ" {
		t.Errorf("Name = %q, want %q", unit.Name(), "ガジェットポーチ")
	}
	// 空白のみの説明は未入力として扱う。
	if unit.Attributes().Description != nil {
		t.Errorf("Description = %v, want nil", *unit.Attributes().Description)
	}
}

func TestNewStorageUnitRejectsInvalidAttributes(t *testing.T) {
	testCases := map[string]struct {
		mutate func(*domainstorage.Attributes)
		field  string
	}{
		"空の名前": {
			mutate: func(a *domainstorage.Attributes) { a.Name = "   " },
			field:  "name",
		},
		"長すぎる名前": {
			mutate: func(a *domainstorage.Attributes) { a.Name = strings.Repeat("あ", 101) },
			field:  "name",
		},
		"長すぎる説明": {
			mutate: func(a *domainstorage.Attributes) {
				a.Description = pointerTo(strings.Repeat("あ", 501))
			},
			field: "description",
		},
		"未定義の収納種別": {
			mutate: func(a *domainstorage.Attributes) { a.StorageType = "suitcase" },
			field:  "storageTypeCode",
		},
		"未定義の携行区分": {
			mutate: func(a *domainstorage.Attributes) { a.MobilityClass = "teleport" },
			field:  "mobilityClassCode",
		},
		"負の自重": {
			mutate: func(a *domainstorage.Attributes) { a.TareWeightGram = pointerTo(int32(-1)) },
			field:  "tareWeightGram",
		},
		"負の最大重量": {
			mutate: func(a *domainstorage.Attributes) {
				a.MaximumWeightGram = pointerTo(int32(-1))
			},
			field: "maximumWeightGram",
		},
		"負の最大容積": {
			mutate: func(a *domainstorage.Attributes) {
				a.MaximumVolumeMilliliter = pointerTo(int32(-1))
			},
			field: "maximumVolumeMilliliter",
		},
		"上限を超える自重": {
			mutate: func(a *domainstorage.Attributes) {
				a.TareWeightGram = pointerTo(int32(1_000_001))
			},
			field: "tareWeightGram",
		},
		"負の表示順": {
			mutate: func(a *domainstorage.Attributes) { a.SortOrder = -1 },
			field:  "sortOrder",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			attributes := validAttributes()
			testCase.mutate(&attributes)

			_, err := domainstorage.NewStorageUnit(
				testPublicID, testUserID, attributes, nil, testNow)
			if err == nil {
				t.Fatal("NewStorageUnit returned nil error, want error")
			}
			assertFieldError(t, err, testCase.field)
		})
	}
}

func TestNewStorageUnitUnderParentSetsHierarchy(t *testing.T) {
	parent := newPersistedUnit(1, "日常リュック", nil)

	attributes := validAttributes()
	attributes.Name = "ガジェットポーチ"
	attributes.StorageType = domainstorage.StorageTypePouch

	child, err := domainstorage.NewStorageUnit(
		testPublicID, testUserID, attributes, &parent, testNow)
	if err != nil {
		t.Fatalf("NewStorageUnit returned error: %v", err)
	}

	if child.Depth() != 2 {
		t.Errorf("Depth = %d, want 2", child.Depth())
	}
	if child.Parent().ID != parent.ID() {
		t.Errorf("Parent.ID = %d, want %d", child.Parent().ID, parent.ID())
	}
	if len(child.Ancestors()) != 1 || child.Ancestors()[0].ID != parent.ID() {
		t.Errorf("Ancestors = %+v, want [%d]", child.Ancestors(), parent.ID())
	}
}

func TestNewStorageUnitAllowsThirdLevel(t *testing.T) {
	root := newPersistedUnit(1, "部屋", nil)
	middle := newPersistedUnit(2, "日常リュック", []domainstorage.Reference{root.Reference()})

	unit, err := domainstorage.NewStorageUnit(
		testPublicID, testUserID, validAttributes(), &middle, testNow)
	if err != nil {
		t.Fatalf("NewStorageUnit returned error: %v", err)
	}
	if unit.Depth() != 3 {
		t.Errorf("Depth = %d, want 3", unit.Depth())
	}
}

func TestNewStorageUnitRejectsFourthLevel(t *testing.T) {
	root := newPersistedUnit(1, "部屋", nil)
	middle := newPersistedUnit(2, "日常リュック", []domainstorage.Reference{root.Reference()})
	leaf := newPersistedUnit(
		3, "ガジェットポーチ", []domainstorage.Reference{root.Reference(), middle.Reference()})

	_, err := domainstorage.NewStorageUnit(
		testPublicID, testUserID, validAttributes(), &leaf, testNow)
	if !errors.Is(err, domainstorage.ErrStorageHierarchyTooDeep) {
		t.Fatalf("error = %v, want ErrStorageHierarchyTooDeep", err)
	}
}

func TestNewStorageUnitRejectsArchivedParent(t *testing.T) {
	parent := domainstorage.ReconstructStorageUnit(
		domainstorage.ReconstructStorageUnitParams{
			ID:         1,
			PublicID:   publicIDOf("01"),
			UserID:     testUserID,
			Attributes: validAttributes(),
			CreatedAt:  testNow,
			UpdatedAt:  testNow,
			ArchivedAt: &testNow,
			Version:    1,
		})

	_, err := domainstorage.NewStorageUnit(
		testPublicID, testUserID, validAttributes(), &parent, testNow)
	if !errors.Is(err, domainstorage.ErrStorageUnitParentArchived) {
		t.Fatalf("error = %v, want ErrStorageUnitParentArchived", err)
	}
}

func TestNewStorageUnitRejectsParentOfOtherUser(t *testing.T) {
	parent := domainstorage.ReconstructStorageUnit(
		domainstorage.ReconstructStorageUnitParams{
			ID:         1,
			PublicID:   publicIDOf("01"),
			UserID:     otherUserID,
			Attributes: validAttributes(),
			CreatedAt:  testNow,
			UpdatedAt:  testNow,
			Version:    1,
		})

	// 他ユーザーの収納単位は存在有無を公開せず not found として扱う (設計書 18.3)。
	_, err := domainstorage.NewStorageUnit(
		testPublicID, testUserID, validAttributes(), &parent, testNow)
	if !errors.Is(err, domainstorage.ErrStorageUnitNotFound) {
		t.Fatalf("error = %v, want ErrStorageUnitNotFound", err)
	}
}

func TestUpdateRejectsSelfAsParent(t *testing.T) {
	unit := newPersistedUnit(1, "日常リュック", nil)

	_, err := unit.Update(validAttributes(), &unit, 1, 1, testNow)
	if !errors.Is(err, domainstorage.ErrStorageUnitSelfParent) {
		t.Fatalf("error = %v, want ErrStorageUnitSelfParent", err)
	}
}

func TestUpdateRejectsDescendantAsParent(t *testing.T) {
	root := newPersistedUnit(1, "日常リュック", nil)
	child := newPersistedUnit(2, "ガジェットポーチ", []domainstorage.Reference{root.Reference()})

	// rootを子の下へ移動しようとすると循環参照になる。
	_, err := root.Update(validAttributes(), &child, 2, 1, testNow)
	if !errors.Is(err, domainstorage.ErrStorageUnitCircularParent) {
		t.Fatalf("error = %v, want ErrStorageUnitCircularParent", err)
	}
}

func TestUpdateRejectsMoveThatExceedsDepth(t *testing.T) {
	root := newPersistedUnit(1, "部屋", nil)
	middle := newPersistedUnit(2, "日常リュック", []domainstorage.Reference{root.Reference()})
	// 移動対象は子を1つ持つため部分木の高さは2。
	target := newPersistedUnit(3, "衣服圧縮バッグ", nil)

	_, err := target.Update(validAttributes(), &middle, 2, 1, testNow)
	if !errors.Is(err, domainstorage.ErrStorageHierarchyTooDeep) {
		t.Fatalf("error = %v, want ErrStorageHierarchyTooDeep", err)
	}

	// 高さ1 (子を持たない) なら3階層目へ収まる。
	moved, err := target.Update(validAttributes(), &middle, 1, 1, testNow)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if moved.Depth() != 3 {
		t.Errorf("Depth = %d, want 3", moved.Depth())
	}
}

func TestUpdateMovesToRoot(t *testing.T) {
	root := newPersistedUnit(1, "日常リュック", nil)
	child := newPersistedUnit(2, "ガジェットポーチ", []domainstorage.Reference{root.Reference()})

	moved, err := child.Update(validAttributes(), nil, 1, 1, testNow)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if moved.HasParent() {
		t.Error("HasParent = true, want false")
	}
	if moved.Depth() != 1 {
		t.Errorf("Depth = %d, want 1", moved.Depth())
	}
	if moved.Version() != 2 {
		t.Errorf("Version = %d, want 2", moved.Version())
	}
}

func TestUpdateDetectsVersionConflict(t *testing.T) {
	unit := newPersistedUnit(1, "日常リュック", nil)

	_, err := unit.Update(validAttributes(), nil, 1, 2, testNow)
	if !errors.Is(err, domainstorage.ErrStorageUnitVersionConflict) {
		t.Fatalf("error = %v, want ErrStorageUnitVersionConflict", err)
	}
}

func TestUpdateRejectsArchivedUnit(t *testing.T) {
	unit := domainstorage.ReconstructStorageUnit(
		domainstorage.ReconstructStorageUnitParams{
			ID:         1,
			PublicID:   publicIDOf("01"),
			UserID:     testUserID,
			Attributes: validAttributes(),
			CreatedAt:  testNow,
			UpdatedAt:  testNow,
			ArchivedAt: &testNow,
			Version:    1,
		})

	_, err := unit.Update(validAttributes(), nil, 1, 1, testNow)
	if !errors.Is(err, domainstorage.ErrStorageUnitArchived) {
		t.Fatalf("error = %v, want ErrStorageUnitArchived", err)
	}
}

func TestArchive(t *testing.T) {
	unit := newPersistedUnit(1, "日常リュック", nil)

	archived, err := unit.Archive(0, 0, 1, testNow)
	if err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}
	if !archived.IsArchived() {
		t.Error("IsArchived = false, want true")
	}
	if archived.Version() != 2 {
		t.Errorf("Version = %d, want 2", archived.Version())
	}
}

func TestArchiveRejectsUnitWithChildrenOrAllocations(t *testing.T) {
	unit := newPersistedUnit(1, "日常リュック", nil)

	if _, err := unit.Archive(1, 0, 1, testNow); !errors.Is(
		err, domainstorage.ErrStorageUnitHasChildren) {
		t.Fatalf("error = %v, want ErrStorageUnitHasChildren", err)
	}
	if _, err := unit.Archive(0, 3, 1, testNow); !errors.Is(
		err, domainstorage.ErrStorageUnitHasAllocations) {
		t.Fatalf("error = %v, want ErrStorageUnitHasAllocations", err)
	}
}

func TestArchiveDetectsVersionConflict(t *testing.T) {
	unit := newPersistedUnit(1, "日常リュック", nil)

	_, err := unit.Archive(0, 0, 5, testNow)
	if !errors.Is(err, domainstorage.ErrStorageUnitVersionConflict) {
		t.Fatalf("error = %v, want ErrStorageUnitVersionConflict", err)
	}
}

func TestRestore(t *testing.T) {
	unit := domainstorage.ReconstructStorageUnit(
		domainstorage.ReconstructStorageUnitParams{
			ID:         1,
			PublicID:   publicIDOf("01"),
			UserID:     testUserID,
			Attributes: validAttributes(),
			CreatedAt:  testNow,
			UpdatedAt:  testNow,
			ArchivedAt: &testNow,
			Version:    2,
		})

	restored, err := unit.Restore(nil, 2, testNow)
	if err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	if restored.IsArchived() {
		t.Error("IsArchived = true, want false")
	}
	if restored.Version() != 3 {
		t.Errorf("Version = %d, want 3", restored.Version())
	}
}

func TestRestoreRejectsArchivedParent(t *testing.T) {
	parentAttributes := validAttributes()
	parent := domainstorage.ReconstructStorageUnit(
		domainstorage.ReconstructStorageUnitParams{
			ID:         1,
			PublicID:   publicIDOf("01"),
			UserID:     testUserID,
			Attributes: parentAttributes,
			CreatedAt:  testNow,
			UpdatedAt:  testNow,
			ArchivedAt: &testNow,
			Version:    1,
		})

	childAttributes := validAttributes()
	childAttributes.Parent = parent.Reference()
	child := domainstorage.ReconstructStorageUnit(
		domainstorage.ReconstructStorageUnitParams{
			ID:         2,
			PublicID:   publicIDOf("02"),
			UserID:     testUserID,
			Attributes: childAttributes,
			Ancestors:  []domainstorage.Reference{parent.Reference()},
			CreatedAt:  testNow,
			UpdatedAt:  testNow,
			ArchivedAt: &testNow,
			Version:    1,
		})

	_, err := child.Restore(&parent, 1, testNow)
	if !errors.Is(err, domainstorage.ErrStorageUnitParentArchived) {
		t.Fatalf("error = %v, want ErrStorageUnitParentArchived", err)
	}
}

func TestRestoreRejectsUnarchivedUnit(t *testing.T) {
	unit := newPersistedUnit(1, "日常リュック", nil)

	_, err := unit.Restore(nil, 1, testNow)
	if !errors.Is(err, domainstorage.ErrStorageUnitNotArchived) {
		t.Fatalf("error = %v, want ErrStorageUnitNotArchived", err)
	}
}

func TestAuditSnapshotExcludesInternalIdentifiers(t *testing.T) {
	root := newPersistedUnit(1, "日常リュック", nil)
	child := newPersistedUnit(2, "ガジェットポーチ", []domainstorage.Reference{root.Reference()})

	snapshot := child.AuditSnapshot()
	if _, ok := snapshot["id"]; ok {
		t.Error("snapshot contains internal id")
	}
	if snapshot["parentStorageUnitPublicId"] != root.PublicID().String() {
		t.Errorf("parentStorageUnitPublicId = %v, want %s",
			snapshot["parentStorageUnitPublicId"], root.PublicID())
	}
	if snapshot["name"] != "ガジェットポーチ" {
		t.Errorf("name = %v, want ガジェットポーチ", snapshot["name"])
	}
}
