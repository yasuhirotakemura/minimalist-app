package storage

import (
	"time"

	"github.com/google/uuid"

	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
)

// StorageUnitReferenceResult は他resourceから参照する収納単位の最小表現。
type StorageUnitReferenceResult struct {
	PublicID uuid.UUID
	Name     string
}

// CapacityResult は重量・容積の集計と超過判定 (設計書 16.2 / 16.3)。
type CapacityResult struct {
	AllocatedItemKindCount int32
	AllocatedQuantity      int64

	TareWeightGram       int64
	ItemWeightGram       int64
	DescendantWeightGram int64
	TotalWeightGram      int64

	ItemVolumeMilliliter       int64
	DescendantVolumeMilliliter int64
	TotalVolumeMilliliter      int64

	MaximumWeightGram       *int32
	MaximumVolumeMilliliter *int32

	RemainingWeightGram       *int64
	RemainingVolumeMilliliter *int64

	IsWeightExceeded bool
	IsVolumeExceeded bool
	HasUnknownWeight bool
	HasUnknownVolume bool
}

// StorageUnitResult はユースケースが返す収納単位の表現。
//
// 内部IDを含めない (設計書 12.1)。codeとlabelはDomainのValueObjectが持つ
// 対応関係を展開した結果であり、presentation layerはそのままresponseへ写す。
type StorageUnitResult struct {
	PublicID                uuid.UUID
	Name                    string
	StorageTypeCode         string
	StorageTypeLabel        string
	MobilityClassCode       string
	MobilityClassLabel      string
	Parent                  *StorageUnitReferenceResult
	Ancestors               []StorageUnitReferenceResult
	Depth                   int32
	ChildCount              int32
	TareWeightGram          *int32
	MaximumWeightGram       *int32
	MaximumVolumeMilliliter *int32
	Description             *string
	SortOrder               int32
	Capacity                CapacityResult
	IsArchived              bool
	ArchivedAt              *time.Time
	Version                 int32
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// PaginationResult はoffset paginationの結果。
type PaginationResult struct {
	Limit      int32
	Offset     int32
	TotalCount int64
	HasNext    bool
}

// ListStorageUnitsResult は収納単位一覧の結果。
type ListStorageUnitsResult struct {
	Items      []StorageUnitResult
	Pagination PaginationResult
}

// AllocatedItemResult は収納割当から参照するアイテムの表現。
//
// 収納内容編集画面が整合性表示 (所有数量・他収納への割当数量・未割当数量)
// を行うために必要な項目だけを持つ。
type AllocatedItemResult struct {
	PublicID uuid.UUID
	Name     string
	UnitName string
	Quantity int32
	// AssignedQuantity は全収納単位への割当数量の合計。
	AssignedQuantity int32
	// UnassignedQuantity は quantity - assignedQuantity。DBへ保存せず算出する。
	UnassignedQuantity int32
	WeightGram         *int32
	VolumeMilliliter   *int32
	IsArchived         bool
}

// StorageAllocationResult は収納単位配下の割当1件。
type StorageAllocationResult struct {
	PublicID  uuid.UUID
	Item      AllocatedItemResult
	Quantity  int32
	Version   int32
	CreatedAt time.Time
	UpdatedAt time.Time
}

// StorageUnitContentsResult は収納単位の内容。
//
// 割当の追加・変更・削除・一括置換の結果にも使用する。更新後のversion・
// 容量集計・超過判定を1度に返すことで、追加requestなしに整合した画面を描ける。
type StorageUnitContentsResult struct {
	StorageUnit       StorageUnitResult
	Allocations       []StorageAllocationResult
	ChildStorageUnits []StorageUnitResult
}

// ItemStorageAllocationResult はアイテム側から見た割当1件。
type ItemStorageAllocationResult struct {
	PublicID    uuid.UUID
	StorageUnit StorageUnitReferenceResult
	Quantity    int32
	Version     int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListItemStorageAllocationsResult は1アイテムの収納割当一覧。
//
// 件数が収納単位数で上限づけられるためpaginationを持たない。
type ListItemStorageAllocationsResult struct {
	Items              []ItemStorageAllocationResult
	Quantity           int32
	AssignedQuantity   int32
	UnassignedQuantity int32
}

func newCapacityResult(capacity domainstorage.Capacity) CapacityResult {
	return CapacityResult{
		AllocatedItemKindCount:     capacity.AllocatedItemKindCount,
		AllocatedQuantity:          capacity.AllocatedQuantity,
		TareWeightGram:             capacity.TareWeightGram,
		ItemWeightGram:             capacity.ItemWeightGram,
		DescendantWeightGram:       capacity.DescendantWeightGram,
		TotalWeightGram:            capacity.TotalWeightGram,
		ItemVolumeMilliliter:       capacity.ItemVolumeMilliliter,
		DescendantVolumeMilliliter: capacity.DescendantVolumeMilliliter,
		TotalVolumeMilliliter:      capacity.TotalVolumeMilliliter,
		MaximumWeightGram:          capacity.MaximumWeightGram,
		MaximumVolumeMilliliter:    capacity.MaximumVolumeMilliliter,
		RemainingWeightGram:        capacity.RemainingWeightGram,
		RemainingVolumeMilliliter:  capacity.RemainingVolumeMilliliter,
		IsWeightExceeded:           capacity.IsWeightExceeded,
		IsVolumeExceeded:           capacity.IsVolumeExceeded,
		HasUnknownWeight:           capacity.HasUnknownWeight,
		HasUnknownVolume:           capacity.HasUnknownVolume,
	}
}

func newStorageUnitResult(
	unit domainstorage.StorageUnit,
	capacity domainstorage.Capacity,
) StorageUnitResult {
	attributes := unit.Attributes()

	var parent *StorageUnitReferenceResult
	if unit.HasParent() {
		parent = &StorageUnitReferenceResult{
			PublicID: attributes.Parent.PublicID,
			Name:     attributes.Parent.Name,
		}
	}

	ancestors := make([]StorageUnitReferenceResult, 0, len(unit.Ancestors()))
	for _, ancestor := range unit.Ancestors() {
		ancestors = append(ancestors, StorageUnitReferenceResult{
			PublicID: ancestor.PublicID,
			Name:     ancestor.Name,
		})
	}

	return StorageUnitResult{
		PublicID:                unit.PublicID(),
		Name:                    attributes.Name,
		StorageTypeCode:         attributes.StorageType.String(),
		StorageTypeLabel:        attributes.StorageType.Label(),
		MobilityClassCode:       attributes.MobilityClass.String(),
		MobilityClassLabel:      attributes.MobilityClass.Label(),
		Parent:                  parent,
		Ancestors:               ancestors,
		Depth:                   unit.Depth(),
		ChildCount:              unit.ChildCount(),
		TareWeightGram:          attributes.TareWeightGram,
		MaximumWeightGram:       attributes.MaximumWeightGram,
		MaximumVolumeMilliliter: attributes.MaximumVolumeMilliliter,
		Description:             attributes.Description,
		SortOrder:               attributes.SortOrder,
		Capacity:                newCapacityResult(capacity),
		IsArchived:              unit.IsArchived(),
		ArchivedAt:              unit.ArchivedAt(),
		Version:                 unit.Version(),
		CreatedAt:               unit.CreatedAt(),
		UpdatedAt:               unit.UpdatedAt(),
	}
}

// newAllocationResult は収納割当1件の結果を組み立てる。
//
// assignedTotalsは全収納単位を対象としたアイテムごとの割当数量合計であり、
// 未割当数量の算出に使用する。
func newAllocationResult(
	allocation domainstorage.StorageAllocation,
	assignedQuantity int64,
) StorageAllocationResult {
	allocatedItem := allocation.Item()

	return StorageAllocationResult{
		PublicID: allocation.PublicID(),
		Item: AllocatedItemResult{
			PublicID:         allocatedItem.PublicID,
			Name:             allocatedItem.Name,
			UnitName:         allocatedItem.UnitName,
			Quantity:         allocatedItem.Quantity,
			AssignedQuantity: int32(assignedQuantity),
			UnassignedQuantity: domainstorage.UnassignedQuantity(
				allocatedItem.Quantity, assignedQuantity),
			WeightGram:       allocatedItem.WeightGram,
			VolumeMilliliter: allocatedItem.VolumeMilliliter,
			IsArchived:       allocatedItem.IsArchived,
		},
		Quantity:  allocation.Quantity(),
		Version:   allocation.Version(),
		CreatedAt: allocation.CreatedAt(),
		UpdatedAt: allocation.UpdatedAt(),
	}
}

func newItemStorageAllocationResult(
	allocation domainstorage.StorageAllocation,
) ItemStorageAllocationResult {
	return ItemStorageAllocationResult{
		PublicID: allocation.PublicID(),
		StorageUnit: StorageUnitReferenceResult{
			PublicID: allocation.StorageUnit().PublicID,
			Name:     allocation.StorageUnit().Name,
		},
		Quantity:  allocation.Quantity(),
		Version:   allocation.Version(),
		CreatedAt: allocation.CreatedAt(),
		UpdatedAt: allocation.UpdatedAt(),
	}
}

// newPaginationResult はpagination結果を組み立てる。
func newPaginationResult(limit, offset int32, totalCount int64) PaginationResult {
	return PaginationResult{
		Limit:      limit,
		Offset:     offset,
		TotalCount: totalCount,
		HasNext:    int64(offset)+int64(limit) < totalCount,
	}
}
