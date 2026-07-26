package storage

import (
	"time"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// AllocationID は内部主キー。APIへ公開しない (設計書 12.1)。
type AllocationID int64

// Int64 はDB問い合わせ用の値を返す。
func (id AllocationID) Int64() int64 { return int64(id) }

// IsZero は未永続化かどうかを返す。
func (id AllocationID) IsZero() bool { return id == 0 }

// AllocatedItem は収納割当から参照するアイテムの表現。
//
// 収納内容編集画面の整合性表示 (所有数量・他収納への割当数量・未割当数量) と
// 容量集計に必要な項目だけを持つ。Item Aggregate全体を持ち込まない。
type AllocatedItem struct {
	ID               item.ItemID
	PublicID         uuid.UUID
	Name             string
	UnitName         string
	Quantity         int32
	WeightGram       *int32
	VolumeMilliliter *int32
	IsArchived       bool
}

// IsZero は未設定かどうかを返す。
func (i AllocatedItem) IsZero() bool { return i.ID.IsZero() }

// StorageAllocation は「アイテムが今どの収納単位へ何個入っているか」を表すEntity。
//
// 取り出した履歴を保持する要件が無いため、割当解除はsoft deleteではなく
// 行の削除として表現する。
type StorageAllocation struct {
	id       AllocationID
	publicID uuid.UUID
	userID   auth.UserID
	// storageUnit は割当先。アイテム側から割当を見る画面が
	// 収納単位名を表示するため、内部IDだけでなく参照を保持する。
	storageUnit Reference
	item        AllocatedItem
	quantity    AllocationQuantity
	createdAt   time.Time
	updatedAt   time.Time
	version     int32
}

// NewStorageAllocation は未永続化の収納割当を生成する。
//
// archive済みのアイテム・収納単位へは新規割当できない。
// 割当数量合計と所有数量の関係は集計が必要なため、
// Application Serviceがtransaction内で検証する (設計書 20章)。
func NewStorageAllocation(
	publicID uuid.UUID,
	userID auth.UserID,
	unit StorageUnit,
	allocatedItem AllocatedItem,
	quantity int32,
	now time.Time,
) (StorageAllocation, error) {
	if publicID == uuid.Nil {
		return StorageAllocation{}, shared.NewInternalError(
			"INVALID_PUBLIC_ID", "内部エラーが発生しました。")
	}
	if userID.IsZero() || allocatedItem.IsZero() {
		return StorageAllocation{}, shared.NewInternalError(
			"INVALID_STORAGE_ALLOCATION", "内部エラーが発生しました。")
	}
	if unit.UserID() != userID {
		return StorageAllocation{}, ErrStorageUnitNotFound
	}
	if err := unit.EnsureAssignable(); err != nil {
		return StorageAllocation{}, err
	}
	if allocatedItem.IsArchived {
		return StorageAllocation{}, ErrStorageAllocationItemArchived
	}

	allocationQuantity, err := NewAllocationQuantity(quantity)
	if err != nil {
		return StorageAllocation{}, err
	}

	instant := now.UTC()
	return StorageAllocation{
		publicID:    publicID,
		userID:      userID,
		storageUnit: unit.Reference(),
		item:        allocatedItem,
		quantity:    allocationQuantity,
		createdAt:   instant,
		updatedAt:   instant,
		version:     1,
	}, nil
}

// ReconstructStorageAllocationParams は永続化済み収納割当の復元に使用する。
type ReconstructStorageAllocationParams struct {
	ID       AllocationID
	PublicID uuid.UUID
	UserID   auth.UserID
	// StorageUnit は割当先。収納単位側から取得した場合は文脈から
	// 収納単位が明らかなため、内部IDだけを設定してもよい。
	StorageUnit Reference
	Item        AllocatedItem
	Quantity    int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Version     int32
}

// ReconstructStorageAllocation はRepositoryが取得したdataから収納割当を復元する。
func ReconstructStorageAllocation(
	params ReconstructStorageAllocationParams,
) StorageAllocation {
	return StorageAllocation{
		id:          params.ID,
		publicID:    params.PublicID,
		userID:      params.UserID,
		storageUnit: params.StorageUnit,
		item:        params.Item,
		quantity:    AllocationQuantity{value: params.Quantity},
		createdAt:   params.CreatedAt.UTC(),
		updatedAt:   params.UpdatedAt.UTC(),
		version:     params.Version,
	}
}

// ID は内部主キーを返す。
func (a StorageAllocation) ID() AllocationID { return a.id }

// PublicID は外部公開IDを返す。
func (a StorageAllocation) PublicID() uuid.UUID { return a.publicID }

// UserID は所有者の内部IDを返す。
func (a StorageAllocation) UserID() auth.UserID { return a.userID }

// StorageUnit は割当先の収納単位の参照を返す。
func (a StorageAllocation) StorageUnit() Reference { return a.storageUnit }

// StorageUnitID は割当先の収納単位の内部IDを返す。
func (a StorageAllocation) StorageUnitID() StorageUnitID { return a.storageUnit.ID }

// Item は割当対象のアイテムを返す。
func (a StorageAllocation) Item() AllocatedItem { return a.item }

// ItemID は割当対象のアイテムの内部IDを返す。
func (a StorageAllocation) ItemID() item.ItemID { return a.item.ID }

// Quantity は収納数量を返す。
func (a StorageAllocation) Quantity() int32 { return a.quantity.Value() }

// CreatedAt は作成日時を返す。
func (a StorageAllocation) CreatedAt() time.Time { return a.createdAt }

// UpdatedAt は更新日時を返す。
func (a StorageAllocation) UpdatedAt() time.Time { return a.updatedAt }

// Version は楽観ロック用のversionを返す。
func (a StorageAllocation) Version() int32 { return a.version }

// WithID は内部主キーを設定した複製を返す。Repositoryがinsert後に使用する。
func (a StorageAllocation) WithID(id AllocationID) StorageAllocation {
	a.id = id
	return a
}

// ChangeQuantity は数量を変更した複製を返す。
func (a StorageAllocation) ChangeQuantity(
	quantity int32,
	expectedVersion int32,
	now time.Time,
) (StorageAllocation, error) {
	if err := a.EnsureVersionMatches(expectedVersion); err != nil {
		return StorageAllocation{}, err
	}

	allocationQuantity, err := NewAllocationQuantity(quantity)
	if err != nil {
		return StorageAllocation{}, err
	}

	a.quantity = allocationQuantity
	a.updatedAt = now.UTC()
	a.version = expectedVersion + 1
	return a, nil
}

// EnsureVersionMatches は楽観ロックのversionが一致することを確認する。
func (a StorageAllocation) EnsureVersionMatches(expectedVersion int32) error {
	if a.version != expectedVersion {
		return ErrStorageAllocationVersionConflict
	}
	return nil
}

// AuditSnapshot は監査ログの差分計算に使用する項目の写しを返す (設計書 22章)。
func (a StorageAllocation) AuditSnapshot() map[string]any {
	return map[string]any{
		"itemPublicId": a.item.PublicID.String(),
		"quantity":     a.quantity.Value(),
	}
}

// EnsureAllocatedQuantityWithinOwned は割当数量合計が所有数量以下であることを検証する
// (設計書 7.2 / 13.9)。
//
// 複数行の集計を伴うためCHECK constraintでは表現できない。
// Application Serviceが対象アイテム行をロックしたうえで本関数を呼ぶ。
func EnsureAllocatedQuantityWithinOwned(ownedQuantity int32, allocatedQuantity int64) error {
	if allocatedQuantity > int64(ownedQuantity) {
		return ErrStorageAllocationExceedsQuantity
	}
	return nil
}

// UnassignedQuantity は未割当数量を算出する。
//
// DBへ重複保存せず取得時に算出する。所有数量を割当合計が上回る状態は
// 不変条件違反だが、表示時に負値を見せないため0で下限を切る。
func UnassignedQuantity(ownedQuantity int32, allocatedQuantity int64) int32 {
	unassigned := int64(ownedQuantity) - allocatedQuantity
	if unassigned < 0 {
		return 0
	}
	return int32(unassigned)
}

func newAllocationAttributeError(field, message string) *shared.DomainError {
	return shared.NewInvalidInputError("INVALID_STORAGE_ALLOCATION", message).
		WithFieldErrors(shared.NewFieldError(field, "INVALID_VALUE", message))
}
