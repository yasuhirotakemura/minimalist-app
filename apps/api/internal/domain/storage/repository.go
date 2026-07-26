package storage

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
)

// StorageUnitRepository は収納単位Aggregateの永続化を担う (設計書 6.5 / 11.6)。
//
// 実装は全てのqueryで user internal ID を条件へ含める (設計書 18.3)。
// 更新系は version をWHERE条件へ含め、更新件数0を競合として扱う (設計書 11.7)。
type StorageUnitRepository interface {
	// Create は収納単位を作成し、内部IDを付与した収納単位を返す。
	Create(ctx context.Context, unit StorageUnit) (StorageUnit, error)

	// FindByPublicID は収納単位を取得する。archive済みも返す。
	// 存在しない場合、および他ユーザーの収納単位を指定した場合は
	// ErrStorageUnitNotFound を返す。
	FindByPublicID(
		ctx context.Context, userID auth.UserID, publicID uuid.UUID) (StorageUnit, error)

	// Update は属性と親を置き換える。
	// versionが一致しない場合は ErrStorageUnitVersionConflict を返す。
	Update(ctx context.Context, unit StorageUnit, expectedVersion int32) (StorageUnit, error)

	// Archive はarchive (soft delete) する。
	Archive(
		ctx context.Context,
		userID auth.UserID,
		publicID uuid.UUID,
		expectedVersion int32,
		archivedAt time.Time,
	) (StorageUnit, error)

	// Restore はarchiveを解除する。
	Restore(
		ctx context.Context,
		userID auth.UserID,
		publicID uuid.UUID,
		expectedVersion int32,
		now time.Time,
	) (StorageUnit, error)

	// TouchVersion は収納割当の変更に伴い収納単位のversionを1増加させる。
	//
	// 割当集合の競合を検知するため、割当の追加・変更・削除・一括置換で使用する。
	// versionが一致しない場合は ErrStorageUnitVersionConflict を返す。
	TouchVersion(
		ctx context.Context,
		userID auth.UserID,
		publicID uuid.UUID,
		expectedVersion int32,
		now time.Time,
	) (StorageUnit, error)

	// List は条件に一致する収納単位を返す。
	List(ctx context.Context, userID auth.UserID, criteria ListCriteria) ([]StorageUnit, error)

	// Count は条件に一致する収納単位の総件数を返す。
	Count(ctx context.Context, userID auth.UserID, criteria ListCriteria) (int64, error)

	// ListAll はユーザーの全収納単位を返す。
	//
	// 階層をまたぐ容量集計と循環参照の検証は木全体を必要とするため、
	// pageに含まれない収納単位も取得する。階層上限が3であり
	// 個人利用の規模では件数が限られるため全件取得で足りる。
	ListAll(
		ctx context.Context, userID auth.UserID, includeArchived bool) ([]StorageUnit, error)

	// ListChildren は直接の子収納単位を返す。archive済みは含めない。
	ListChildren(
		ctx context.Context, userID auth.UserID, parentID StorageUnitID) ([]StorageUnit, error)

	// CountActiveChildren はarchive前の直接の子収納単位の件数を返す。
	CountActiveChildren(
		ctx context.Context, userID auth.UserID, parentID StorageUnitID) (int64, error)
}

// StorageAllocationRepository は収納割当の永続化を担う。
//
// 収納割当は収納単位とアイテムの双方から参照され、
// 収納単位Aggregateとは別のqueryとなるためinterfaceを分離する。
type StorageAllocationRepository interface {
	// Create は収納割当を作成する。
	// 同一収納単位・同一アイテムが既に存在する場合は
	// ErrStorageAllocationAlreadyExists を返す。
	Create(ctx context.Context, allocation StorageAllocation) (StorageAllocation, error)

	// FindByPublicID は収納割当を取得する。
	FindByPublicID(
		ctx context.Context, userID auth.UserID, publicID uuid.UUID) (StorageAllocation, error)

	// UpdateQuantity は数量を変更する。
	// versionが一致しない場合は ErrStorageAllocationVersionConflict を返す。
	UpdateQuantity(
		ctx context.Context,
		allocation StorageAllocation,
		expectedVersion int32,
	) (StorageAllocation, error)

	// Delete は収納割当を削除する。
	//
	// 「今どこに入っているか」を表す現在状態であり、取り出した履歴を
	// 保持する要件が無いため物理削除する。
	Delete(
		ctx context.Context,
		userID auth.UserID,
		publicID uuid.UUID,
		expectedVersion int32,
	) error

	// DeleteByStorageUnitID は収納単位配下の割当をすべて削除する。一括置換で使用する。
	DeleteByStorageUnitID(
		ctx context.Context, userID auth.UserID, storageUnitID StorageUnitID) error

	// ListByStorageUnitID は収納単位へ直接割当されている割当をアイテム名昇順で返す。
	ListByStorageUnitID(
		ctx context.Context,
		userID auth.UserID,
		storageUnitID StorageUnitID,
	) ([]StorageAllocation, error)

	// ListByStorageUnitIDs は複数収納単位の割当をまとめて返す。
	// 一覧の容量集計でN+1 queryを避けるために使用する。
	ListByStorageUnitIDs(
		ctx context.Context,
		userID auth.UserID,
		storageUnitIDs []StorageUnitID,
	) (map[StorageUnitID][]StorageAllocation, error)

	// ListByItemID は1アイテムの割当を収納単位名昇順で返す。
	ListByItemID(
		ctx context.Context, userID auth.UserID, itemID item.ItemID) ([]StorageAllocation, error)

	// ListByItemIDs は複数アイテムの割当をまとめて返す。
	// 所持品一覧のN+1 queryを避けるために使用する。
	ListByItemIDs(
		ctx context.Context,
		userID auth.UserID,
		itemIDs []item.ItemID,
	) (map[item.ItemID][]StorageAllocation, error)

	// CountByStorageUnitID は収納単位配下の割当件数を返す。archive可否の判定に使用する。
	CountByStorageUnitID(
		ctx context.Context, userID auth.UserID, storageUnitID StorageUnitID) (int64, error)

	// SumQuantityByItemIDForUpdate はアイテムの割当数量合計を返す。
	//
	// 対象アイテム行を SELECT FOR UPDATE でロックしたうえで集計し、
	// 並行更新で「割当数量合計 <= 所有数量」が破られないよう直列化する
	// (設計書 20章)。excludeAllocationID が0でない場合、その割当を合計から除く。
	// 返り値は対象アイテムの所有数量と、ロック時点の割当数量合計である。
	SumQuantityByItemIDForUpdate(
		ctx context.Context,
		userID auth.UserID,
		itemID item.ItemID,
		excludeAllocationID AllocationID,
	) (ownedQuantity int32, allocatedQuantity int64, err error)

	// LockItemsForUpdate は複数アイテム行をロックし、所有数量を返す。
	// 一括置換で複数アイテムの数量整合性を同時に検証するために使用する。
	// deadlockを避けるため、実装は内部IDの昇順でロックする。
	LockItemsForUpdate(
		ctx context.Context,
		userID auth.UserID,
		itemIDs []item.ItemID,
	) (map[item.ItemID]int32, error)

	// FindAllocatedItemByPublicID は割当対象のアイテムを取得する。
	// 存在しない場合、および他ユーザーのアイテムを指定した場合は
	// item.ErrItemNotFound を返す。
	FindAllocatedItemByPublicID(
		ctx context.Context, userID auth.UserID, publicID uuid.UUID) (AllocatedItem, error)

	// ResolveAllocatedItems はpublicIdの並び順でアイテムを解決する。一括置換で使用する。
	ResolveAllocatedItems(
		ctx context.Context, userID auth.UserID, publicIDs []uuid.UUID) ([]AllocatedItem, error)
}
