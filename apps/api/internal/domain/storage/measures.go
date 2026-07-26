package storage

// 重量・容積・割当数量・階層の深さを表すValueObject。
//
// いずれも「不正な値を持つインスタンスを作れない」ことを型で保証し、
// Entityと集計処理が範囲検査を繰り返さずに済むようにする。

// 入力の上限。DB制約 (ck_storage_units__*, ck_storage_allocations__*) と一致させる。
const (
	// MaxWeightGram は自重・最大重量の上限 (1トン)。
	// 個人の持ち物管理として現実的な上限を置き、桁の入力ミスを弾く。
	MaxWeightGram int32 = 1_000_000

	// MaxVolumeMilliliter は最大容積の上限 (100立方メートル)。
	// 部屋 (room) を収納単位として登録する場合を想定した上限とする。
	MaxVolumeMilliliter int32 = 100_000_000

	// MaxAllocationQuantity は1割当あたりの数量上限。items.quantity と揃える。
	MaxAllocationQuantity int32 = 1_000_000

	// MaxHierarchyDepth は収納単位の階層上限 (設計書 7.3)。
	MaxHierarchyDepth int32 = 3
)

// Weight は0以上のグラム値。
type Weight struct {
	gram int32
}

// NewWeight はWeightを生成する。負値と上限超過を拒否する。
func NewWeight(gram int32, field, label string) (Weight, error) {
	if gram < 0 {
		return Weight{}, newAttributeError(field, label+"は0以上で入力してください。")
	}
	if gram > MaxWeightGram {
		return Weight{}, newAttributeError(field, label+"が大きすぎます。")
	}
	return Weight{gram: gram}, nil
}

// Gram はグラム値を返す。
func (w Weight) Gram() int32 { return w.gram }

// Volume は0以上のミリリットル値。
type Volume struct {
	milliliter int32
}

// NewVolume はVolumeを生成する。負値と上限超過を拒否する。
func NewVolume(milliliter int32, field, label string) (Volume, error) {
	if milliliter < 0 {
		return Volume{}, newAttributeError(field, label+"は0以上で入力してください。")
	}
	if milliliter > MaxVolumeMilliliter {
		return Volume{}, newAttributeError(field, label+"が大きすぎます。")
	}
	return Volume{milliliter: milliliter}, nil
}

// Milliliter はミリリットル値を返す。
func (v Volume) Milliliter() int32 { return v.milliliter }

// AllocationQuantity は収納割当の数量。1以上とする。
//
// 0個の割当は「割り当てていない」と同義であり、行を残す意味がないため
// 0を許可しない (設計書 13.9)。
type AllocationQuantity struct {
	value int32
}

// NewAllocationQuantity はAllocationQuantityを生成する。
func NewAllocationQuantity(value int32) (AllocationQuantity, error) {
	if value < 1 {
		return AllocationQuantity{}, newAllocationAttributeError(
			"quantity", "収納数量は1以上で入力してください。")
	}
	if value > MaxAllocationQuantity {
		return AllocationQuantity{}, newAllocationAttributeError(
			"quantity", "収納数量は1000000以下で入力してください。")
	}
	return AllocationQuantity{value: value}, nil
}

// Value は数量を返す。
func (q AllocationQuantity) Value() int32 { return q.value }

// HierarchyDepth は収納単位の階層の深さ。rootを1とし、最大3とする (設計書 7.3)。
type HierarchyDepth struct {
	value int32
}

// NewHierarchyDepth はHierarchyDepthを生成する。
// 上限を超える深さは ErrStorageHierarchyTooDeep とし、入力errorと区別する。
func NewHierarchyDepth(value int32) (HierarchyDepth, error) {
	if value < 1 {
		return HierarchyDepth{}, newAttributeError(
			"parentStorageUnitPublicId", "収納単位の階層が正しくありません。")
	}
	if value > MaxHierarchyDepth {
		return HierarchyDepth{}, ErrStorageHierarchyTooDeep
	}
	return HierarchyDepth{value: value}, nil
}

// Value は深さを返す。
func (d HierarchyDepth) Value() int32 { return d.value }

// IsRoot は親を持たない階層かどうかを返す。
func (d HierarchyDepth) IsRoot() bool { return d.value == 1 }
