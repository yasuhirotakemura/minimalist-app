package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/sqlc"
	infrapostgresql "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/postgresql"
)

// PostgresqlStorageAllocationRepository はStorageAllocationRepositoryのPostgreSQL実装。
type PostgresqlStorageAllocationRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresqlStorageAllocationRepository はPostgresqlStorageAllocationRepositoryを生成する。
func NewPostgresqlStorageAllocationRepository(
	pool *pgxpool.Pool,
) *PostgresqlStorageAllocationRepository {
	return &PostgresqlStorageAllocationRepository{pool: pool}
}

// 同一収納単位・同一アイテムの重複時に返されるconstraint名。migrationの定義と一致させる。
const allocationUniqueIndexName = "uq_storage_allocations__storage_unit_id_item_id"

var _ domainstorage.StorageAllocationRepository = (*PostgresqlStorageAllocationRepository)(nil)

func (r *PostgresqlStorageAllocationRepository) queries(ctx context.Context) *sqlc.Queries {
	return sqlc.New(infrapostgresql.Querier(ctx, r.pool))
}

// Create は収納割当を作成する。
func (r *PostgresqlStorageAllocationRepository) Create(
	ctx context.Context,
	allocation domainstorage.StorageAllocation,
) (domainstorage.StorageAllocation, error) {
	row, err := r.queries(ctx).InsertStorageAllocation(ctx, sqlc.InsertStorageAllocationParams{
		PublicID:      allocation.PublicID(),
		UserID:        allocation.UserID().Int64(),
		StorageUnitID: allocation.StorageUnitID().Int64(),
		ItemID:        allocation.ItemID().Int64(),
		Quantity:      allocation.Quantity(),
		CreatedAt:     timestamptz(allocation.CreatedAt()),
		UpdatedAt:     timestamptz(allocation.UpdatedAt()),
	})
	if err != nil {
		// uq_storage_allocations__storage_unit_id_item_id 違反。
		if isUniqueViolation(err, allocationUniqueIndexName) {
			return domainstorage.StorageAllocation{},
				domainstorage.ErrStorageAllocationAlreadyExists.WithCause(err)
		}
		// composite foreign keyにより、他ユーザーの収納単位・アイテムを
		// 指定した場合も本違反となる (設計書 18.3)。
		if isForeignKeyViolation(err) {
			return domainstorage.StorageAllocation{},
				domainstorage.ErrStorageUnitNotFound.WithCause(err)
		}
		return domainstorage.StorageAllocation{},
			fmt.Errorf("insert storage allocation: %w", err)
	}

	return domainstorage.ReconstructStorageAllocation(
		domainstorage.ReconstructStorageAllocationParams{
			ID:          domainstorage.AllocationID(row.ID),
			PublicID:    row.PublicID,
			UserID:      domainauth.UserID(row.UserID),
			StorageUnit: domainstorage.Reference{ID: domainstorage.StorageUnitID(row.StorageUnitID)},
			Item:        allocation.Item(),
			Quantity:    row.Quantity,
			CreatedAt:   utcTime(row.CreatedAt),
			UpdatedAt:   utcTime(row.UpdatedAt),
			Version:     row.Version,
		}), nil
}

// FindByPublicID は収納割当を取得する。
func (r *PostgresqlStorageAllocationRepository) FindByPublicID(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
) (domainstorage.StorageAllocation, error) {
	row, err := r.queries(ctx).FindStorageAllocationByPublicID(
		ctx,
		sqlc.FindStorageAllocationByPublicIDParams{
			PublicID: publicID,
			UserID:   userID.Int64(),
		},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainstorage.StorageAllocation{},
				domainstorage.ErrStorageAllocationNotFound
		}
		return domainstorage.StorageAllocation{},
			fmt.Errorf("find storage allocation by public id: %w", err)
	}

	return toDomainStorageAllocation(row.OwnershipStorageAllocation, domainstorage.AllocatedItem{
		ID:               domainitem.ItemID(row.OwnershipStorageAllocation.ItemID),
		PublicID:         row.ItemPublicID,
		Name:             row.ItemName,
		UnitName:         row.ItemUnitName,
		Quantity:         row.ItemQuantity,
		WeightGram:       row.ItemWeightGram,
		VolumeMilliliter: row.ItemVolumeMilliliter,
		IsArchived:       row.ItemIsArchived,
	}), nil
}

// UpdateQuantity は数量を変更する。
func (r *PostgresqlStorageAllocationRepository) UpdateQuantity(
	ctx context.Context,
	allocation domainstorage.StorageAllocation,
	expectedVersion int32,
) (domainstorage.StorageAllocation, error) {
	_, err := r.queries(ctx).UpdateStorageAllocationQuantity(
		ctx,
		sqlc.UpdateStorageAllocationQuantityParams{
			Quantity:        allocation.Quantity(),
			UpdatedAt:       timestamptz(allocation.UpdatedAt()),
			PublicID:        allocation.PublicID(),
			UserID:          allocation.UserID().Int64(),
			ExpectedVersion: expectedVersion,
		},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainstorage.StorageAllocation{},
				r.resolveUpdateFailure(ctx, allocation.UserID(), allocation.PublicID())
		}
		return domainstorage.StorageAllocation{},
			fmt.Errorf("update storage allocation quantity: %w", err)
	}
	return r.FindByPublicID(ctx, allocation.UserID(), allocation.PublicID())
}

// Delete は収納割当を削除する。
func (r *PostgresqlStorageAllocationRepository) Delete(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
	expectedVersion int32,
) error {
	_, err := r.queries(ctx).DeleteStorageAllocation(ctx, sqlc.DeleteStorageAllocationParams{
		PublicID:        publicID,
		UserID:          userID.Int64(),
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return r.resolveUpdateFailure(ctx, userID, publicID)
		}
		return fmt.Errorf("delete storage allocation: %w", err)
	}
	return nil
}

// DeleteByStorageUnitID は収納単位配下の割当をすべて削除する。
func (r *PostgresqlStorageAllocationRepository) DeleteByStorageUnitID(
	ctx context.Context,
	userID domainauth.UserID,
	storageUnitID domainstorage.StorageUnitID,
) error {
	err := r.queries(ctx).DeleteStorageAllocationsByStorageUnitID(
		ctx,
		sqlc.DeleteStorageAllocationsByStorageUnitIDParams{
			UserID:        userID.Int64(),
			StorageUnitID: storageUnitID.Int64(),
		},
	)
	if err != nil {
		return fmt.Errorf("delete storage allocations by storage unit id: %w", err)
	}
	return nil
}

// ListByStorageUnitID は収納単位へ直接割当されている割当を返す。
func (r *PostgresqlStorageAllocationRepository) ListByStorageUnitID(
	ctx context.Context,
	userID domainauth.UserID,
	storageUnitID domainstorage.StorageUnitID,
) ([]domainstorage.StorageAllocation, error) {
	byUnitID, err := r.ListByStorageUnitIDs(
		ctx, userID, []domainstorage.StorageUnitID{storageUnitID})
	if err != nil {
		return nil, err
	}
	return byUnitID[storageUnitID], nil
}

// ListByStorageUnitIDs は複数収納単位の割当をまとめて返す。
func (r *PostgresqlStorageAllocationRepository) ListByStorageUnitIDs(
	ctx context.Context,
	userID domainauth.UserID,
	storageUnitIDs []domainstorage.StorageUnitID,
) (map[domainstorage.StorageUnitID][]domainstorage.StorageAllocation, error) {
	result := make(
		map[domainstorage.StorageUnitID][]domainstorage.StorageAllocation, len(storageUnitIDs))
	if len(storageUnitIDs) == 0 {
		return result, nil
	}

	ids := make([]int64, 0, len(storageUnitIDs))
	for _, id := range storageUnitIDs {
		ids = append(ids, id.Int64())
	}

	rows, err := r.queries(ctx).ListStorageAllocationsByStorageUnitIDs(
		ctx,
		sqlc.ListStorageAllocationsByStorageUnitIDsParams{
			UserID:         userID.Int64(),
			StorageUnitIds: ids,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("list storage allocations by storage unit ids: %w", err)
	}

	for _, row := range rows {
		unitID := domainstorage.StorageUnitID(row.OwnershipStorageAllocation.StorageUnitID)
		result[unitID] = append(result[unitID], toDomainStorageAllocation(
			row.OwnershipStorageAllocation,
			domainstorage.AllocatedItem{
				ID:               domainitem.ItemID(row.OwnershipStorageAllocation.ItemID),
				PublicID:         row.ItemPublicID,
				Name:             row.ItemName,
				UnitName:         row.ItemUnitName,
				Quantity:         row.ItemQuantity,
				WeightGram:       row.ItemWeightGram,
				VolumeMilliliter: row.ItemVolumeMilliliter,
				IsArchived:       row.ItemIsArchived,
			}))
	}
	return result, nil
}

// ListByItemID は1アイテムの割当を返す。
func (r *PostgresqlStorageAllocationRepository) ListByItemID(
	ctx context.Context,
	userID domainauth.UserID,
	itemID domainitem.ItemID,
) ([]domainstorage.StorageAllocation, error) {
	byItemID, err := r.ListByItemIDs(ctx, userID, []domainitem.ItemID{itemID})
	if err != nil {
		return nil, err
	}
	return byItemID[itemID], nil
}

// ListByItemIDs は複数アイテムの割当をまとめて返す。
//
// 収納単位側の情報だけを持つ表現を返す。アイテム側の項目は
// 呼び出し元が既に保持しているため重複して取得しない。
func (r *PostgresqlStorageAllocationRepository) ListByItemIDs(
	ctx context.Context,
	userID domainauth.UserID,
	itemIDs []domainitem.ItemID,
) (map[domainitem.ItemID][]domainstorage.StorageAllocation, error) {
	result := make(map[domainitem.ItemID][]domainstorage.StorageAllocation, len(itemIDs))
	if len(itemIDs) == 0 {
		return result, nil
	}

	ids := make([]int64, 0, len(itemIDs))
	for _, id := range itemIDs {
		ids = append(ids, id.Int64())
	}

	rows, err := r.queries(ctx).ListStorageAllocationsByItemIDs(
		ctx,
		sqlc.ListStorageAllocationsByItemIDsParams{UserID: userID.Int64(), ItemIds: ids},
	)
	if err != nil {
		return nil, fmt.Errorf("list storage allocations by item ids: %w", err)
	}

	for _, row := range rows {
		itemID := domainitem.ItemID(row.OwnershipStorageAllocation.ItemID)
		result[itemID] = append(result[itemID], domainstorage.ReconstructStorageAllocation(
			domainstorage.ReconstructStorageAllocationParams{
				ID:        domainstorage.AllocationID(row.OwnershipStorageAllocation.ID),
				PublicID:  row.OwnershipStorageAllocation.PublicID,
				UserID:    userID,
				Item:      domainstorage.AllocatedItem{ID: itemID},
				Quantity:  row.OwnershipStorageAllocation.Quantity,
				CreatedAt: utcTime(row.OwnershipStorageAllocation.CreatedAt),
				UpdatedAt: utcTime(row.OwnershipStorageAllocation.UpdatedAt),
				Version:   row.OwnershipStorageAllocation.Version,
				StorageUnit: domainstorage.Reference{
					ID: domainstorage.StorageUnitID(
						row.OwnershipStorageAllocation.StorageUnitID),
					PublicID: row.StorageUnitPublicID,
					Name:     row.StorageUnitName,
				},
			}))
	}
	return result, nil
}

// CountByStorageUnitID は収納単位配下の割当件数を返す。
func (r *PostgresqlStorageAllocationRepository) CountByStorageUnitID(
	ctx context.Context,
	userID domainauth.UserID,
	storageUnitID domainstorage.StorageUnitID,
) (int64, error) {
	count, err := r.queries(ctx).CountStorageAllocationsByStorageUnitID(
		ctx,
		sqlc.CountStorageAllocationsByStorageUnitIDParams{
			UserID:        userID.Int64(),
			StorageUnitID: storageUnitID.Int64(),
		},
	)
	if err != nil {
		return 0, fmt.Errorf("count storage allocations by storage unit id: %w", err)
	}
	return count, nil
}

// SumQuantityByItemIDForUpdate はアイテム行をロックし、割当数量合計を返す。
func (r *PostgresqlStorageAllocationRepository) SumQuantityByItemIDForUpdate(
	ctx context.Context,
	userID domainauth.UserID,
	itemID domainitem.ItemID,
	excludeAllocationID domainstorage.AllocationID,
) (int32, int64, error) {
	quantities, err := r.LockItemsForUpdate(ctx, userID, []domainitem.ItemID{itemID})
	if err != nil {
		return 0, 0, err
	}
	ownedQuantity, ok := quantities[itemID]
	if !ok {
		return 0, 0, domainitem.ErrItemNotFound
	}

	total, err := r.queries(ctx).SumStorageAllocationQuantityByItemID(
		ctx,
		sqlc.SumStorageAllocationQuantityByItemIDParams{
			UserID:              userID.Int64(),
			ItemID:              itemID.Int64(),
			ExcludeAllocationID: excludeAllocationID.Int64(),
		},
	)
	if err != nil {
		return 0, 0, fmt.Errorf("sum storage allocation quantity by item id: %w", err)
	}
	return ownedQuantity, total, nil
}

// LockItemsForUpdate は複数アイテム行をロックし、所有数量を返す。
func (r *PostgresqlStorageAllocationRepository) LockItemsForUpdate(
	ctx context.Context,
	userID domainauth.UserID,
	itemIDs []domainitem.ItemID,
) (map[domainitem.ItemID]int32, error) {
	quantities := make(map[domainitem.ItemID]int32, len(itemIDs))
	if len(itemIDs) == 0 {
		return quantities, nil
	}

	ids := make([]int64, 0, len(itemIDs))
	for _, id := range itemIDs {
		ids = append(ids, id.Int64())
	}

	rows, err := r.queries(ctx).LockItemQuantitiesForUpdate(
		ctx,
		sqlc.LockItemQuantitiesForUpdateParams{UserID: userID.Int64(), ItemIds: ids},
	)
	if err != nil {
		return nil, fmt.Errorf("lock item quantities for update: %w", err)
	}

	for _, row := range rows {
		quantities[domainitem.ItemID(row.ID)] = row.Quantity
	}
	return quantities, nil
}

// FindAllocatedItemByPublicID は割当対象のアイテムを取得する。
func (r *PostgresqlStorageAllocationRepository) FindAllocatedItemByPublicID(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
) (domainstorage.AllocatedItem, error) {
	row, err := r.queries(ctx).FindAllocatedItemByPublicID(
		ctx,
		sqlc.FindAllocatedItemByPublicIDParams{UserID: userID.Int64(), PublicID: publicID},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainstorage.AllocatedItem{}, domainitem.ErrItemNotFound
		}
		return domainstorage.AllocatedItem{}, fmt.Errorf("find allocated item: %w", err)
	}

	return domainstorage.AllocatedItem{
		ID:               domainitem.ItemID(row.ID),
		PublicID:         row.PublicID,
		Name:             row.Name,
		UnitName:         row.UnitName,
		Quantity:         row.Quantity,
		WeightGram:       row.WeightGram,
		VolumeMilliliter: row.VolumeMilliliter,
		IsArchived:       row.IsArchived,
	}, nil
}

// ResolveAllocatedItems はpublicIdの並び順でアイテムを解決する。
//
// 1件でも解決できない場合は ErrItemNotFound を返し、
// 存在しないアイテムを含む一括置換を部分的に適用しない。
func (r *PostgresqlStorageAllocationRepository) ResolveAllocatedItems(
	ctx context.Context,
	userID domainauth.UserID,
	publicIDs []uuid.UUID,
) ([]domainstorage.AllocatedItem, error) {
	if len(publicIDs) == 0 {
		return nil, nil
	}

	rows, err := r.queries(ctx).ListAllocatedItemsByPublicIDs(
		ctx,
		sqlc.ListAllocatedItemsByPublicIDsParams{UserID: userID.Int64(), PublicIds: publicIDs},
	)
	if err != nil {
		return nil, fmt.Errorf("list allocated items by public ids: %w", err)
	}

	byPublicID := make(map[uuid.UUID]domainstorage.AllocatedItem, len(rows))
	for _, row := range rows {
		byPublicID[row.PublicID] = domainstorage.AllocatedItem{
			ID:               domainitem.ItemID(row.ID),
			PublicID:         row.PublicID,
			Name:             row.Name,
			UnitName:         row.UnitName,
			Quantity:         row.Quantity,
			WeightGram:       row.WeightGram,
			VolumeMilliliter: row.VolumeMilliliter,
			IsArchived:       row.IsArchived,
		}
	}

	resolved := make([]domainstorage.AllocatedItem, 0, len(publicIDs))
	for _, publicID := range publicIDs {
		allocatedItem, ok := byPublicID[publicID]
		if !ok {
			return nil, domainitem.ErrItemNotFound
		}
		resolved = append(resolved, allocatedItem)
	}
	return resolved, nil
}

// resolveUpdateFailure は更新件数0の理由を判定する。
func (r *PostgresqlStorageAllocationRepository) resolveUpdateFailure(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
) error {
	exists, err := r.queries(ctx).ExistsStorageAllocationByPublicID(
		ctx,
		sqlc.ExistsStorageAllocationByPublicIDParams{
			PublicID: publicID,
			UserID:   userID.Int64(),
		},
	)
	if err != nil {
		return fmt.Errorf("check storage allocation existence: %w", err)
	}
	if !exists {
		return domainstorage.ErrStorageAllocationNotFound
	}
	return domainstorage.ErrStorageAllocationVersionConflict
}

func toDomainStorageAllocation(
	row sqlc.OwnershipStorageAllocation,
	allocatedItem domainstorage.AllocatedItem,
) domainstorage.StorageAllocation {
	return domainstorage.ReconstructStorageAllocation(
		domainstorage.ReconstructStorageAllocationParams{
			ID:          domainstorage.AllocationID(row.ID),
			PublicID:    row.PublicID,
			UserID:      domainauth.UserID(row.UserID),
			StorageUnit: domainstorage.Reference{ID: domainstorage.StorageUnitID(row.StorageUnitID)},
			Item:        allocatedItem,
			Quantity:    row.Quantity,
			CreatedAt:   utcTime(row.CreatedAt),
			UpdatedAt:   utcTime(row.UpdatedAt),
			Version:     row.Version,
		})
}
