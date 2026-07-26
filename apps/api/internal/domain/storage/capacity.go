package storage

// 収納単位の重量・容積の集計と超過判定 (設計書 16.2 / 16.3)。
//
// 設計書 16.2 は使用量を
//
//	usedWeight = tareWeight + Σ(item.weightGram * allocatedQuantity)
//	usedVolume = Σ(item.volumeMilliliter * allocatedQuantity)
//
// と定めるが、子孫収納単位を含めるかを規定していない。
// 本実装は直接割当分と子孫分を別に保持し、合計で両者を足す。
//
//	totalWeightGram = tareWeightGram + itemWeightGram + descendantWeightGram
//
// descendantWeightGram には子孫自身の自重と内容物を含め、
// 親の自重・直接割当分を含めない。これにより親子で同じ重量を二重計上しない。
//
// 未設定 (NULL) の重量・容積を0として合計へ混ぜると、利用者は不完全な値を
// 完全な値と誤認する。既知分だけを合計し、hasUnknown* で不完全であることを示す
// (設計書 16.2)。

// CapacityInput は1収納単位の集計入力。
type CapacityInput struct {
	TareWeightGram          *int32
	MaximumWeightGram       *int32
	MaximumVolumeMilliliter *int32
	// Allocations は直接割当されている収納割当。子孫の割当を含めない。
	Allocations []StorageAllocation
	// Descendants は直接の子収納単位の集計済みCapacity。
	// 子のCapacityは既に孫を含むため、直接の子だけを渡せば全子孫を集計できる。
	Descendants []Capacity
}

// Capacity は収納単位の集計結果と超過判定。
type Capacity struct {
	// AllocatedItemKindCount は直接割当されているアイテムの種類数。
	AllocatedItemKindCount int32
	// AllocatedQuantity は直接割当されている数量の合計。
	AllocatedQuantity int64

	TareWeightGram       int64
	ItemWeightGram       int64
	DescendantWeightGram int64
	TotalWeightGram      int64

	ItemVolumeMilliliter       int64
	DescendantVolumeMilliliter int64
	TotalVolumeMilliliter      int64

	MaximumWeightGram       *int32
	MaximumVolumeMilliliter *int32

	// RemainingWeightGram は残り容量。上限が未設定の場合はnil。超過時は負値。
	RemainingWeightGram       *int64
	RemainingVolumeMilliliter *int64

	IsWeightExceeded bool
	IsVolumeExceeded bool

	// HasUnknownWeight は合計が「入力済み分のみ」であることを表す。
	HasUnknownWeight bool
	HasUnknownVolume bool
}

// CalculateCapacity は収納単位の集計と超過判定を行う。
func CalculateCapacity(input CapacityInput) Capacity {
	capacity := Capacity{
		MaximumWeightGram:       input.MaximumWeightGram,
		MaximumVolumeMilliliter: input.MaximumVolumeMilliliter,
	}

	// 自重が未設定の場合、総重量は自重の分だけ不足する。
	// 0として扱ったうえで不完全であることを示す。
	if input.TareWeightGram != nil {
		capacity.TareWeightGram = int64(*input.TareWeightGram)
	} else {
		capacity.HasUnknownWeight = true
	}

	for _, allocation := range input.Allocations {
		allocatedItem := allocation.Item()
		quantity := int64(allocation.Quantity())

		capacity.AllocatedItemKindCount++
		capacity.AllocatedQuantity += quantity

		if allocatedItem.WeightGram != nil {
			capacity.ItemWeightGram += int64(*allocatedItem.WeightGram) * quantity
		} else {
			capacity.HasUnknownWeight = true
		}

		if allocatedItem.VolumeMilliliter != nil {
			capacity.ItemVolumeMilliliter += int64(*allocatedItem.VolumeMilliliter) * quantity
		} else {
			capacity.HasUnknownVolume = true
		}
	}

	for _, descendant := range input.Descendants {
		capacity.DescendantWeightGram += descendant.TotalWeightGram
		capacity.DescendantVolumeMilliliter += descendant.TotalVolumeMilliliter
		capacity.HasUnknownWeight = capacity.HasUnknownWeight || descendant.HasUnknownWeight
		capacity.HasUnknownVolume = capacity.HasUnknownVolume || descendant.HasUnknownVolume
	}

	capacity.TotalWeightGram =
		capacity.TareWeightGram + capacity.ItemWeightGram + capacity.DescendantWeightGram
	capacity.TotalVolumeMilliliter =
		capacity.ItemVolumeMilliliter + capacity.DescendantVolumeMilliliter

	if input.MaximumWeightGram != nil {
		remaining := int64(*input.MaximumWeightGram) - capacity.TotalWeightGram
		capacity.RemainingWeightGram = &remaining
		capacity.IsWeightExceeded = remaining < 0
	}
	if input.MaximumVolumeMilliliter != nil {
		remaining := int64(*input.MaximumVolumeMilliliter) - capacity.TotalVolumeMilliliter
		capacity.RemainingVolumeMilliliter = &remaining
		capacity.IsVolumeExceeded = remaining < 0
	}

	return capacity
}

// IsExceeded は重量・容積のいずれかが上限を超えているかを返す (設計書 16.3)。
func (c Capacity) IsExceeded() bool { return c.IsWeightExceeded || c.IsVolumeExceeded }

// CalculateHierarchyCapacities は収納単位の木全体のCapacityをまとめて算出する。
//
// 子から親の順に計算し、親の集計で子の結果を再利用することで、
// 同じ子孫を複数回たどらないようにする。
//
// unitsは同一ユーザーの収納単位の集合であり、部分集合ではなく
// 集計対象となる木全体を含める必要がある。
// allocationsByUnitIDは収納単位ごとの直接割当である。
func CalculateHierarchyCapacities(
	units []StorageUnit,
	allocationsByUnitID map[StorageUnitID][]StorageAllocation,
) map[StorageUnitID]Capacity {
	childIDs := make(map[StorageUnitID][]StorageUnitID, len(units))
	for _, unit := range units {
		// archive済みの収納単位は使わなくなった収納具であり、
		// 親の「今の総重量」へ自重を加算しない。
		// archiveには中身と子が空であることを要求するため、除外しても
		// 収納中のアイテムが集計から漏れることはない。
		if unit.IsArchived() || !unit.HasParent() {
			continue
		}
		parentID := unit.Parent().ID
		childIDs[parentID] = append(childIDs[parentID], unit.ID())
	}

	capacities := make(map[StorageUnitID]Capacity, len(units))
	// 深い階層から計算すると、親を計算する時点で子の結果が揃う。
	// 階層上限は3のため、深さの降順に3回走査すれば全件を計算できる。
	maxDepth := int32(1)
	for _, unit := range units {
		if unit.Depth() > maxDepth {
			maxDepth = unit.Depth()
		}
	}
	for depth := maxDepth; depth >= 1; depth-- {
		for _, unit := range units {
			if unit.Depth() != depth {
				continue
			}
			descendants := make([]Capacity, 0, len(childIDs[unit.ID()]))
			for _, childID := range childIDs[unit.ID()] {
				if childCapacity, ok := capacities[childID]; ok {
					descendants = append(descendants, childCapacity)
				}
			}

			attributes := unit.Attributes()
			capacities[unit.ID()] = CalculateCapacity(CapacityInput{
				TareWeightGram:          attributes.TareWeightGram,
				MaximumWeightGram:       attributes.MaximumWeightGram,
				MaximumVolumeMilliliter: attributes.MaximumVolumeMilliliter,
				Allocations:             allocationsByUnitID[unit.ID()],
				Descendants:             descendants,
			})
		}
	}

	return capacities
}
