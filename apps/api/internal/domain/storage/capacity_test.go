package storage_test

import (
	"testing"

	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
)

// newAllocation は集計入力となる割当を組み立てる。
func newAllocation(
	unitID domainstorage.StorageUnitID,
	itemID int64,
	quantity int32,
	weightGram *int32,
	volumeMilliliter *int32,
) domainstorage.StorageAllocation {
	return domainstorage.ReconstructStorageAllocation(
		domainstorage.ReconstructStorageAllocationParams{
			ID:          domainstorage.AllocationID(itemID),
			PublicID:    publicIDOf(pad(int(itemID))),
			UserID:      testUserID,
			StorageUnit: domainstorage.Reference{ID: unitID},
			Item: domainstorage.AllocatedItem{
				ID:               domainitem.ItemID(itemID),
				PublicID:         publicIDOf(pad(int(itemID))),
				Name:             "アイテム",
				UnitName:         "個",
				Quantity:         quantity,
				WeightGram:       weightGram,
				VolumeMilliliter: volumeMilliliter,
			},
			Quantity:  quantity,
			CreatedAt: testNow,
			UpdatedAt: testNow,
			Version:   1,
		})
}

func TestCalculateCapacitySumsDirectAllocations(t *testing.T) {
	capacity := domainstorage.CalculateCapacity(domainstorage.CapacityInput{
		TareWeightGram: pointerTo(int32(900)),
		Allocations: []domainstorage.StorageAllocation{
			newAllocation(1, 1, 2, pointerTo(int32(150)), pointerTo(int32(800))),
			newAllocation(1, 2, 1, pointerTo(int32(1200)), pointerTo(int32(2000))),
		},
	})

	if capacity.AllocatedItemKindCount != 2 {
		t.Errorf("AllocatedItemKindCount = %d, want 2", capacity.AllocatedItemKindCount)
	}
	if capacity.AllocatedQuantity != 3 {
		t.Errorf("AllocatedQuantity = %d, want 3", capacity.AllocatedQuantity)
	}
	// 150*2 + 1200*1
	if capacity.ItemWeightGram != 1500 {
		t.Errorf("ItemWeightGram = %d, want 1500", capacity.ItemWeightGram)
	}
	if capacity.TotalWeightGram != 2400 {
		t.Errorf("TotalWeightGram = %d, want 2400", capacity.TotalWeightGram)
	}
	// 800*2 + 2000*1
	if capacity.ItemVolumeMilliliter != 3600 {
		t.Errorf("ItemVolumeMilliliter = %d, want 3600", capacity.ItemVolumeMilliliter)
	}
	if capacity.TotalVolumeMilliliter != 3600 {
		t.Errorf("TotalVolumeMilliliter = %d, want 3600", capacity.TotalVolumeMilliliter)
	}
	if capacity.HasUnknownWeight || capacity.HasUnknownVolume {
		t.Error("HasUnknown* = true, want false")
	}
}

func TestCalculateCapacityMarksUnknownWeightAndVolume(t *testing.T) {
	capacity := domainstorage.CalculateCapacity(domainstorage.CapacityInput{
		TareWeightGram: pointerTo(int32(500)),
		Allocations: []domainstorage.StorageAllocation{
			newAllocation(1, 1, 2, pointerTo(int32(150)), nil),
			newAllocation(1, 2, 1, nil, pointerTo(int32(2000))),
		},
	})

	// 既知分のみを合計する。未設定を0として完全な値に見せない。
	if capacity.ItemWeightGram != 300 {
		t.Errorf("ItemWeightGram = %d, want 300", capacity.ItemWeightGram)
	}
	if capacity.ItemVolumeMilliliter != 2000 {
		t.Errorf("ItemVolumeMilliliter = %d, want 2000", capacity.ItemVolumeMilliliter)
	}
	if !capacity.HasUnknownWeight {
		t.Error("HasUnknownWeight = false, want true")
	}
	if !capacity.HasUnknownVolume {
		t.Error("HasUnknownVolume = false, want true")
	}
}

func TestCalculateCapacityMarksUnknownWhenTareWeightMissing(t *testing.T) {
	capacity := domainstorage.CalculateCapacity(domainstorage.CapacityInput{})

	if capacity.TareWeightGram != 0 {
		t.Errorf("TareWeightGram = %d, want 0", capacity.TareWeightGram)
	}
	if !capacity.HasUnknownWeight {
		t.Error("HasUnknownWeight = false, want true")
	}
	// 容積は収納単位自身が値を持たないため、割当が無ければ不明とならない。
	if capacity.HasUnknownVolume {
		t.Error("HasUnknownVolume = true, want false")
	}
}

func TestCalculateCapacityDetectsExceeded(t *testing.T) {
	capacity := domainstorage.CalculateCapacity(domainstorage.CapacityInput{
		TareWeightGram:          pointerTo(int32(1000)),
		MaximumWeightGram:       pointerTo(int32(2000)),
		MaximumVolumeMilliliter: pointerTo(int32(5000)),
		Allocations: []domainstorage.StorageAllocation{
			newAllocation(1, 1, 2, pointerTo(int32(800)), pointerTo(int32(1000))),
		},
	})

	if capacity.TotalWeightGram != 2600 {
		t.Errorf("TotalWeightGram = %d, want 2600", capacity.TotalWeightGram)
	}
	if !capacity.IsWeightExceeded {
		t.Error("IsWeightExceeded = false, want true")
	}
	if capacity.RemainingWeightGram == nil || *capacity.RemainingWeightGram != -600 {
		t.Errorf("RemainingWeightGram = %v, want -600", capacity.RemainingWeightGram)
	}
	if capacity.IsVolumeExceeded {
		t.Error("IsVolumeExceeded = true, want false")
	}
	if capacity.RemainingVolumeMilliliter == nil || *capacity.RemainingVolumeMilliliter != 3000 {
		t.Errorf("RemainingVolumeMilliliter = %v, want 3000", capacity.RemainingVolumeMilliliter)
	}
	if !capacity.IsExceeded() {
		t.Error("IsExceeded = false, want true")
	}
}

func TestCalculateCapacityWithoutMaximumDoesNotJudge(t *testing.T) {
	capacity := domainstorage.CalculateCapacity(domainstorage.CapacityInput{
		TareWeightGram: pointerTo(int32(0)),
		Allocations: []domainstorage.StorageAllocation{
			newAllocation(1, 1, 1, pointerTo(int32(100000)), pointerTo(int32(100000))),
		},
	})

	if capacity.RemainingWeightGram != nil {
		t.Errorf("RemainingWeightGram = %v, want nil", *capacity.RemainingWeightGram)
	}
	if capacity.IsWeightExceeded || capacity.IsVolumeExceeded {
		t.Error("IsExceeded = true, want false")
	}
}

func TestCalculateHierarchyCapacitiesAvoidsDoubleCounting(t *testing.T) {
	// 部屋 (自重なし)
	//   └ 日常リュック (自重900)
	//       └ ガジェットポーチ (自重100)
	rootAttributes := validAttributes()
	rootAttributes.Name = "部屋"
	rootAttributes.StorageType = domainstorage.StorageTypeRoom
	rootAttributes.TareWeightGram = pointerTo(int32(0))
	root := domainstorage.ReconstructStorageUnit(domainstorage.ReconstructStorageUnitParams{
		ID: 1, PublicID: publicIDOf("01"), UserID: testUserID,
		Attributes: rootAttributes, CreatedAt: testNow, UpdatedAt: testNow, Version: 1,
	})

	bagAttributes := validAttributes()
	bagAttributes.TareWeightGram = pointerTo(int32(900))
	bagAttributes.Parent = root.Reference()
	bag := domainstorage.ReconstructStorageUnit(domainstorage.ReconstructStorageUnitParams{
		ID: 2, PublicID: publicIDOf("02"), UserID: testUserID,
		Attributes: bagAttributes,
		Ancestors:  []domainstorage.Reference{root.Reference()},
		CreatedAt:  testNow, UpdatedAt: testNow, Version: 1,
	})

	pouchAttributes := validAttributes()
	pouchAttributes.Name = "ガジェットポーチ"
	pouchAttributes.StorageType = domainstorage.StorageTypePouch
	pouchAttributes.TareWeightGram = pointerTo(int32(100))
	pouchAttributes.Parent = bag.Reference()
	pouch := domainstorage.ReconstructStorageUnit(domainstorage.ReconstructStorageUnitParams{
		ID: 3, PublicID: publicIDOf("03"), UserID: testUserID,
		Attributes: pouchAttributes,
		Ancestors:  []domainstorage.Reference{root.Reference(), bag.Reference()},
		CreatedAt:  testNow, UpdatedAt: testNow, Version: 1,
	})

	allocations := map[domainstorage.StorageUnitID][]domainstorage.StorageAllocation{
		2: {newAllocation(2, 10, 1, pointerTo(int32(1500)), pointerTo(int32(3000)))},
		3: {newAllocation(3, 11, 2, pointerTo(int32(200)), pointerTo(int32(150)))},
	}

	capacities := domainstorage.CalculateHierarchyCapacities(
		[]domainstorage.StorageUnit{root, bag, pouch}, allocations)

	pouchCapacity := capacities[3]
	// 100 + 200*2
	if pouchCapacity.TotalWeightGram != 500 {
		t.Errorf("pouch TotalWeightGram = %d, want 500", pouchCapacity.TotalWeightGram)
	}

	bagCapacity := capacities[2]
	if bagCapacity.ItemWeightGram != 1500 {
		t.Errorf("bag ItemWeightGram = %d, want 1500", bagCapacity.ItemWeightGram)
	}
	if bagCapacity.DescendantWeightGram != 500 {
		t.Errorf("bag DescendantWeightGram = %d, want 500", bagCapacity.DescendantWeightGram)
	}
	// 900 + 1500 + 500
	if bagCapacity.TotalWeightGram != 2900 {
		t.Errorf("bag TotalWeightGram = %d, want 2900", bagCapacity.TotalWeightGram)
	}
	// 直接割当のみを数える。
	if bagCapacity.AllocatedItemKindCount != 1 {
		t.Errorf("bag AllocatedItemKindCount = %d, want 1", bagCapacity.AllocatedItemKindCount)
	}

	rootCapacity := capacities[1]
	// 親の自重0 + 直接割当0 + 子孫 (リュックのtotal)
	if rootCapacity.DescendantWeightGram != 2900 {
		t.Errorf("root DescendantWeightGram = %d, want 2900", rootCapacity.DescendantWeightGram)
	}
	if rootCapacity.TotalWeightGram != 2900 {
		t.Errorf("root TotalWeightGram = %d, want 2900", rootCapacity.TotalWeightGram)
	}
	// 3000 + 150*2 が子孫容積として1度だけ計上される。
	if rootCapacity.TotalVolumeMilliliter != 3300 {
		t.Errorf("root TotalVolumeMilliliter = %d, want 3300",
			rootCapacity.TotalVolumeMilliliter)
	}
}

func TestCalculateHierarchyCapacitiesPropagatesUnknown(t *testing.T) {
	parentAttributes := validAttributes()
	parentAttributes.TareWeightGram = pointerTo(int32(500))
	parent := domainstorage.ReconstructStorageUnit(domainstorage.ReconstructStorageUnitParams{
		ID: 1, PublicID: publicIDOf("01"), UserID: testUserID,
		Attributes: parentAttributes, CreatedAt: testNow, UpdatedAt: testNow, Version: 1,
	})

	// 子は自重が未設定のため、親の集計も不完全となる。
	childAttributes := validAttributes()
	childAttributes.Parent = parent.Reference()
	child := domainstorage.ReconstructStorageUnit(domainstorage.ReconstructStorageUnitParams{
		ID: 2, PublicID: publicIDOf("02"), UserID: testUserID,
		Attributes: childAttributes,
		Ancestors:  []domainstorage.Reference{parent.Reference()},
		CreatedAt:  testNow, UpdatedAt: testNow, Version: 1,
	})

	capacities := domainstorage.CalculateHierarchyCapacities(
		[]domainstorage.StorageUnit{parent, child},
		map[domainstorage.StorageUnitID][]domainstorage.StorageAllocation{})

	if !capacities[1].HasUnknownWeight {
		t.Error("parent HasUnknownWeight = false, want true")
	}
}

func TestCalculateHierarchyCapacitiesExcludesArchivedChildren(t *testing.T) {
	parentAttributes := validAttributes()
	parentAttributes.TareWeightGram = pointerTo(int32(500))
	parent := domainstorage.ReconstructStorageUnit(domainstorage.ReconstructStorageUnitParams{
		ID: 1, PublicID: publicIDOf("01"), UserID: testUserID,
		Attributes: parentAttributes, CreatedAt: testNow, UpdatedAt: testNow, Version: 1,
	})

	archivedAttributes := validAttributes()
	archivedAttributes.TareWeightGram = pointerTo(int32(300))
	archivedAttributes.Parent = parent.Reference()
	archived := domainstorage.ReconstructStorageUnit(domainstorage.ReconstructStorageUnitParams{
		ID: 2, PublicID: publicIDOf("02"), UserID: testUserID,
		Attributes: archivedAttributes,
		Ancestors:  []domainstorage.Reference{parent.Reference()},
		CreatedAt:  testNow, UpdatedAt: testNow, ArchivedAt: &testNow, Version: 1,
	})

	capacities := domainstorage.CalculateHierarchyCapacities(
		[]domainstorage.StorageUnit{parent, archived},
		map[domainstorage.StorageUnitID][]domainstorage.StorageAllocation{})

	if capacities[1].DescendantWeightGram != 0 {
		t.Errorf("DescendantWeightGram = %d, want 0", capacities[1].DescendantWeightGram)
	}
	if capacities[1].TotalWeightGram != 500 {
		t.Errorf("TotalWeightGram = %d, want 500", capacities[1].TotalWeightGram)
	}
}
