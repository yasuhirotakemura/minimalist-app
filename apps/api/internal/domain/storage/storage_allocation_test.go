package storage_test

import (
	"errors"
	"testing"

	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
)

func testAllocatedItem() domainstorage.AllocatedItem {
	return domainstorage.AllocatedItem{
		ID:               domainitem.ItemID(100),
		PublicID:         publicIDOf("f1"),
		Name:             "半袖シャツ",
		UnitName:         "枚",
		Quantity:         3,
		WeightGram:       pointerTo(int32(150)),
		VolumeMilliliter: pointerTo(int32(800)),
	}
}

func TestNewStorageAllocation(t *testing.T) {
	unit := newPersistedUnit(1, "衣服圧縮バッグ", nil)

	allocation, err := domainstorage.NewStorageAllocation(
		testPublicID, testUserID, unit, testAllocatedItem(), 2, testNow)
	if err != nil {
		t.Fatalf("NewStorageAllocation returned error: %v", err)
	}

	if allocation.Quantity() != 2 {
		t.Errorf("Quantity = %d, want 2", allocation.Quantity())
	}
	if allocation.Version() != 1 {
		t.Errorf("Version = %d, want 1", allocation.Version())
	}
	if allocation.StorageUnitID() != unit.ID() {
		t.Errorf("StorageUnitID = %d, want %d", allocation.StorageUnitID(), unit.ID())
	}
}

func TestNewStorageAllocationRejectsNonPositiveQuantity(t *testing.T) {
	unit := newPersistedUnit(1, "衣服圧縮バッグ", nil)

	for _, quantity := range []int32{0, -1} {
		_, err := domainstorage.NewStorageAllocation(
			testPublicID, testUserID, unit, testAllocatedItem(), quantity, testNow)
		if err == nil {
			t.Fatalf("quantity %d: returned nil error, want error", quantity)
		}
		assertFieldError(t, err, "quantity")
	}
}

func TestNewStorageAllocationRejectsQuantityAboveUpperBound(t *testing.T) {
	unit := newPersistedUnit(1, "衣服圧縮バッグ", nil)

	_, err := domainstorage.NewStorageAllocation(
		testPublicID, testUserID, unit, testAllocatedItem(), 1_000_001, testNow)
	if err == nil {
		t.Fatal("returned nil error, want error")
	}
	assertFieldError(t, err, "quantity")
}

func TestNewStorageAllocationRejectsArchivedStorageUnit(t *testing.T) {
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

	_, err := domainstorage.NewStorageAllocation(
		testPublicID, testUserID, unit, testAllocatedItem(), 1, testNow)
	if !errors.Is(err, domainstorage.ErrStorageUnitArchived) {
		t.Fatalf("error = %v, want ErrStorageUnitArchived", err)
	}
}

func TestNewStorageAllocationRejectsArchivedItem(t *testing.T) {
	unit := newPersistedUnit(1, "衣服圧縮バッグ", nil)
	allocatedItem := testAllocatedItem()
	allocatedItem.IsArchived = true

	_, err := domainstorage.NewStorageAllocation(
		testPublicID, testUserID, unit, allocatedItem, 1, testNow)
	if !errors.Is(err, domainstorage.ErrStorageAllocationItemArchived) {
		t.Fatalf("error = %v, want ErrStorageAllocationItemArchived", err)
	}
}

func TestNewStorageAllocationRejectsStorageUnitOfOtherUser(t *testing.T) {
	unit := domainstorage.ReconstructStorageUnit(
		domainstorage.ReconstructStorageUnitParams{
			ID:         1,
			PublicID:   publicIDOf("01"),
			UserID:     otherUserID,
			Attributes: validAttributes(),
			CreatedAt:  testNow,
			UpdatedAt:  testNow,
			Version:    1,
		})

	_, err := domainstorage.NewStorageAllocation(
		testPublicID, testUserID, unit, testAllocatedItem(), 1, testNow)
	if !errors.Is(err, domainstorage.ErrStorageUnitNotFound) {
		t.Fatalf("error = %v, want ErrStorageUnitNotFound", err)
	}
}

func TestChangeQuantity(t *testing.T) {
	allocation := domainstorage.ReconstructStorageAllocation(
		domainstorage.ReconstructStorageAllocationParams{
			ID:          1,
			PublicID:    publicIDOf("a1"),
			UserID:      testUserID,
			StorageUnit: domainstorage.Reference{ID: 1},
			Item:        testAllocatedItem(),
			Quantity:    2,
			CreatedAt:   testNow,
			UpdatedAt:   testNow,
			Version:     1,
		})

	changed, err := allocation.ChangeQuantity(3, 1, testNow)
	if err != nil {
		t.Fatalf("ChangeQuantity returned error: %v", err)
	}
	if changed.Quantity() != 3 {
		t.Errorf("Quantity = %d, want 3", changed.Quantity())
	}
	if changed.Version() != 2 {
		t.Errorf("Version = %d, want 2", changed.Version())
	}

	if _, err := allocation.ChangeQuantity(3, 9, testNow); !errors.Is(
		err, domainstorage.ErrStorageAllocationVersionConflict) {
		t.Fatalf("error = %v, want ErrStorageAllocationVersionConflict", err)
	}
	if _, err := allocation.ChangeQuantity(0, 1, testNow); err == nil {
		t.Fatal("ChangeQuantity(0) returned nil error, want error")
	}
}

func TestEnsureAllocatedQuantityWithinOwned(t *testing.T) {
	testCases := map[string]struct {
		owned     int32
		allocated int64
		wantError bool
	}{
		"合計が所有数量未満":  {owned: 3, allocated: 2, wantError: false},
		"合計が所有数量と一致": {owned: 3, allocated: 3, wantError: false},
		"合計が所有数量を超過": {owned: 3, allocated: 4, wantError: true},
		"所有数量0で割当なし": {owned: 0, allocated: 0, wantError: false},
		"所有数量0で割当あり": {owned: 0, allocated: 1, wantError: true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			err := domainstorage.EnsureAllocatedQuantityWithinOwned(
				testCase.owned, testCase.allocated)
			if testCase.wantError {
				if !errors.Is(err, domainstorage.ErrStorageAllocationExceedsQuantity) {
					t.Fatalf("error = %v, want ErrStorageAllocationExceedsQuantity", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("returned error: %v", err)
			}
		})
	}
}

func TestUnassignedQuantity(t *testing.T) {
	testCases := map[string]struct {
		owned     int32
		allocated int64
		want      int32
	}{
		"一部割当": {owned: 3, allocated: 2, want: 1},
		"全数割当": {owned: 3, allocated: 3, want: 0},
		"未割当":  {owned: 3, allocated: 0, want: 3},
		// 不変条件違反の状態でも負の未割当数量を見せない。
		"超過状態": {owned: 3, allocated: 5, want: 0},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got := domainstorage.UnassignedQuantity(testCase.owned, testCase.allocated)
			if got != testCase.want {
				t.Errorf("UnassignedQuantity = %d, want %d", got, testCase.want)
			}
		})
	}
}
