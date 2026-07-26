package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/sqlc"
	infrapostgresql "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/postgresql"
)

// storageUnitSortKeyColumns はDomainの並び替えkeyをDBのcolumn名へ対応付ける。
//
// ORDER BYのCASE式が比較する値と一致させる (sql/queries/storage_units.sql)。
var storageUnitSortKeyColumns = map[domainstorage.SortKey]string{
	domainstorage.SortKeyName:      "name",
	domainstorage.SortKeySortOrder: "sort_order",
	domainstorage.SortKeyUpdatedAt: "updated_at",
}

// PostgresqlStorageUnitRepository はStorageUnitRepositoryのPostgreSQL実装。
type PostgresqlStorageUnitRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresqlStorageUnitRepository はPostgresqlStorageUnitRepositoryを生成する。
func NewPostgresqlStorageUnitRepository(pool *pgxpool.Pool) *PostgresqlStorageUnitRepository {
	return &PostgresqlStorageUnitRepository{pool: pool}
}

var _ domainstorage.StorageUnitRepository = (*PostgresqlStorageUnitRepository)(nil)

func (r *PostgresqlStorageUnitRepository) queries(ctx context.Context) *sqlc.Queries {
	return sqlc.New(infrapostgresql.Querier(ctx, r.pool))
}

// Create は収納単位を作成する。
func (r *PostgresqlStorageUnitRepository) Create(
	ctx context.Context,
	unit domainstorage.StorageUnit,
) (domainstorage.StorageUnit, error) {
	attributes := unit.Attributes()

	row, err := r.queries(ctx).InsertStorageUnit(ctx, sqlc.InsertStorageUnitParams{
		PublicID:                unit.PublicID(),
		UserID:                  unit.UserID().Int64(),
		ParentID:                optionalParentID(attributes.Parent),
		Name:                    attributes.Name,
		StorageTypeCode:         attributes.StorageType.String(),
		MobilityClassCode:       attributes.MobilityClass.String(),
		TareWeightGram:          attributes.TareWeightGram,
		MaximumWeightGram:       attributes.MaximumWeightGram,
		MaximumVolumeMilliliter: attributes.MaximumVolumeMilliliter,
		Description:             attributes.Description,
		SortOrder:               attributes.SortOrder,
		CreatedAt:               timestamptz(unit.CreatedAt()),
		UpdatedAt:               timestamptz(unit.UpdatedAt()),
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			// composite foreign key (user_id, parent_id) により、
			// 他ユーザーの収納単位を親に指定した場合も本違反となる。
			return domainstorage.StorageUnit{},
				domainstorage.ErrStorageUnitNotFound.WithCause(err)
		}
		return domainstorage.StorageUnit{}, fmt.Errorf("insert storage unit: %w", err)
	}

	// 祖先はDomainが検証済みのため、insert結果へそのまま引き継ぐ。
	return toDomainStorageUnit(row, attributes.Parent, unit.Ancestors(), 0), nil
}

// FindByPublicID は収納単位を取得する。archive済みも返す。
func (r *PostgresqlStorageUnitRepository) FindByPublicID(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
) (domainstorage.StorageUnit, error) {
	row, err := r.queries(ctx).FindStorageUnitByPublicID(
		ctx,
		sqlc.FindStorageUnitByPublicIDParams{PublicID: publicID, UserID: userID.Int64()},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainstorage.StorageUnit{}, domainstorage.ErrStorageUnitNotFound
		}
		return domainstorage.StorageUnit{}, fmt.Errorf("find storage unit by public id: %w", err)
	}

	parent, ancestors := hierarchyOf(
		row.OwnershipStorageUnit,
		row.ParentPublicID, row.ParentName,
		row.GrandparentID, row.GrandparentPublicID, row.GrandparentName,
	)
	return toDomainStorageUnit(
		row.OwnershipStorageUnit, parent, ancestors, int32(row.ChildCount)), nil
}

// Update は属性と親を置き換える。
func (r *PostgresqlStorageUnitRepository) Update(
	ctx context.Context,
	unit domainstorage.StorageUnit,
	expectedVersion int32,
) (domainstorage.StorageUnit, error) {
	attributes := unit.Attributes()

	_, err := r.queries(ctx).UpdateStorageUnit(ctx, sqlc.UpdateStorageUnitParams{
		ParentID:                optionalParentID(attributes.Parent),
		Name:                    attributes.Name,
		StorageTypeCode:         attributes.StorageType.String(),
		MobilityClassCode:       attributes.MobilityClass.String(),
		TareWeightGram:          attributes.TareWeightGram,
		MaximumWeightGram:       attributes.MaximumWeightGram,
		MaximumVolumeMilliliter: attributes.MaximumVolumeMilliliter,
		Description:             attributes.Description,
		SortOrder:               attributes.SortOrder,
		UpdatedAt:               timestamptz(unit.UpdatedAt()),
		PublicID:                unit.PublicID(),
		UserID:                  unit.UserID().Int64(),
		ExpectedVersion:         expectedVersion,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return domainstorage.StorageUnit{},
				domainstorage.ErrStorageUnitNotFound.WithCause(err)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return domainstorage.StorageUnit{},
				r.resolveUpdateFailure(ctx, unit.UserID(), unit.PublicID())
		}
		return domainstorage.StorageUnit{}, fmt.Errorf("update storage unit: %w", err)
	}

	return r.FindByPublicID(ctx, unit.UserID(), unit.PublicID())
}

// Archive はarchive (soft delete) する。
func (r *PostgresqlStorageUnitRepository) Archive(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
	expectedVersion int32,
	archivedAt time.Time,
) (domainstorage.StorageUnit, error) {
	_, err := r.queries(ctx).ArchiveStorageUnit(ctx, sqlc.ArchiveStorageUnitParams{
		ArchivedAt:      timestamptz(archivedAt),
		UpdatedAt:       timestamptz(archivedAt),
		PublicID:        publicID,
		UserID:          userID.Int64(),
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainstorage.StorageUnit{}, r.resolveUpdateFailure(ctx, userID, publicID)
		}
		return domainstorage.StorageUnit{}, fmt.Errorf("archive storage unit: %w", err)
	}
	return r.FindByPublicID(ctx, userID, publicID)
}

// Restore はarchiveを解除する。
func (r *PostgresqlStorageUnitRepository) Restore(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
	expectedVersion int32,
	now time.Time,
) (domainstorage.StorageUnit, error) {
	_, err := r.queries(ctx).RestoreStorageUnit(ctx, sqlc.RestoreStorageUnitParams{
		UpdatedAt:       timestamptz(now),
		PublicID:        publicID,
		UserID:          userID.Int64(),
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainstorage.StorageUnit{}, r.resolveUpdateFailure(ctx, userID, publicID)
		}
		return domainstorage.StorageUnit{}, fmt.Errorf("restore storage unit: %w", err)
	}
	return r.FindByPublicID(ctx, userID, publicID)
}

// TouchVersion は収納割当の変更に伴い収納単位のversionを1増加させる。
func (r *PostgresqlStorageUnitRepository) TouchVersion(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
	expectedVersion int32,
	now time.Time,
) (domainstorage.StorageUnit, error) {
	_, err := r.queries(ctx).TouchStorageUnitVersion(ctx, sqlc.TouchStorageUnitVersionParams{
		UpdatedAt:       timestamptz(now),
		PublicID:        publicID,
		UserID:          userID.Int64(),
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainstorage.StorageUnit{}, r.resolveUpdateFailure(ctx, userID, publicID)
		}
		return domainstorage.StorageUnit{}, fmt.Errorf("touch storage unit version: %w", err)
	}
	return r.FindByPublicID(ctx, userID, publicID)
}

// List は条件に一致する収納単位を返す。
func (r *PostgresqlStorageUnitRepository) List(
	ctx context.Context,
	userID domainauth.UserID,
	criteria domainstorage.ListCriteria,
) ([]domainstorage.StorageUnit, error) {
	rows, err := r.queries(ctx).ListStorageUnits(ctx, sqlc.ListStorageUnitsParams{
		UserID:            userID.Int64(),
		IncludeArchived:   criteria.IncludeArchived,
		KeywordPattern:    likePattern(criteria.Keyword),
		StorageTypeCode:   optionalCode(criteria.StorageType),
		MobilityClassCode: optionalCode(criteria.MobilityClass),
		ParentPublicID:    criteria.ParentPublicID,
		RootOnly:          criteria.RootOnly,
		SortKey:           storageUnitSortKeyColumns[criteria.SortKey],
		Descending:        criteria.Descending,
		RowLimit:          criteria.Limit,
		RowOffset:         criteria.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list storage units: %w", err)
	}

	units := make([]domainstorage.StorageUnit, 0, len(rows))
	for _, row := range rows {
		parent, ancestors := hierarchyOf(
			row.OwnershipStorageUnit,
			row.ParentPublicID, row.ParentName,
			row.GrandparentID, row.GrandparentPublicID, row.GrandparentName,
		)
		units = append(units, toDomainStorageUnit(
			row.OwnershipStorageUnit, parent, ancestors, int32(row.ChildCount)))
	}
	return units, nil
}

// Count は条件に一致する収納単位の総件数を返す。
func (r *PostgresqlStorageUnitRepository) Count(
	ctx context.Context,
	userID domainauth.UserID,
	criteria domainstorage.ListCriteria,
) (int64, error) {
	count, err := r.queries(ctx).CountStorageUnits(ctx, sqlc.CountStorageUnitsParams{
		UserID:            userID.Int64(),
		IncludeArchived:   criteria.IncludeArchived,
		KeywordPattern:    likePattern(criteria.Keyword),
		StorageTypeCode:   optionalCode(criteria.StorageType),
		MobilityClassCode: optionalCode(criteria.MobilityClass),
		ParentPublicID:    criteria.ParentPublicID,
		RootOnly:          criteria.RootOnly,
	})
	if err != nil {
		return 0, fmt.Errorf("count storage units: %w", err)
	}
	return count, nil
}

// ListAll はユーザーの全収納単位を返す。
func (r *PostgresqlStorageUnitRepository) ListAll(
	ctx context.Context,
	userID domainauth.UserID,
	includeArchived bool,
) ([]domainstorage.StorageUnit, error) {
	rows, err := r.queries(ctx).ListAllStorageUnits(ctx, sqlc.ListAllStorageUnitsParams{
		UserID:          userID.Int64(),
		IncludeArchived: includeArchived,
	})
	if err != nil {
		return nil, fmt.Errorf("list all storage units: %w", err)
	}

	units := make([]domainstorage.StorageUnit, 0, len(rows))
	for _, row := range rows {
		parent, ancestors := hierarchyOf(
			row.OwnershipStorageUnit,
			row.ParentPublicID, row.ParentName,
			row.GrandparentID, row.GrandparentPublicID, row.GrandparentName,
		)
		units = append(units, toDomainStorageUnit(
			row.OwnershipStorageUnit, parent, ancestors, int32(row.ChildCount)))
	}
	return units, nil
}

// ListChildren は直接の子収納単位を返す。archive済みは含めない。
func (r *PostgresqlStorageUnitRepository) ListChildren(
	ctx context.Context,
	userID domainauth.UserID,
	parentID domainstorage.StorageUnitID,
) ([]domainstorage.StorageUnit, error) {
	rows, err := r.queries(ctx).ListChildStorageUnits(ctx, sqlc.ListChildStorageUnitsParams{
		UserID:   userID.Int64(),
		ParentID: pointerTo(parentID.Int64()),
	})
	if err != nil {
		return nil, fmt.Errorf("list child storage units: %w", err)
	}

	units := make([]domainstorage.StorageUnit, 0, len(rows))
	for _, row := range rows {
		parent, ancestors := hierarchyOf(
			row.OwnershipStorageUnit,
			row.ParentPublicID, row.ParentName,
			row.GrandparentID, row.GrandparentPublicID, row.GrandparentName,
		)
		units = append(units, toDomainStorageUnit(
			row.OwnershipStorageUnit, parent, ancestors, int32(row.ChildCount)))
	}
	return units, nil
}

// CountActiveChildren はarchive前の直接の子収納単位の件数を返す。
func (r *PostgresqlStorageUnitRepository) CountActiveChildren(
	ctx context.Context,
	userID domainauth.UserID,
	parentID domainstorage.StorageUnitID,
) (int64, error) {
	count, err := r.queries(ctx).CountActiveChildStorageUnits(
		ctx,
		sqlc.CountActiveChildStorageUnitsParams{
			UserID:   userID.Int64(),
			ParentID: pointerTo(parentID.Int64()),
		},
	)
	if err != nil {
		return 0, fmt.Errorf("count active child storage units: %w", err)
	}
	return count, nil
}

// resolveUpdateFailure は更新件数0の理由を判定する。
func (r *PostgresqlStorageUnitRepository) resolveUpdateFailure(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
) error {
	exists, err := r.queries(ctx).ExistsStorageUnitByPublicID(
		ctx,
		sqlc.ExistsStorageUnitByPublicIDParams{PublicID: publicID, UserID: userID.Int64()},
	)
	if err != nil {
		return fmt.Errorf("check storage unit existence: %w", err)
	}
	if !exists {
		return domainstorage.ErrStorageUnitNotFound
	}
	return domainstorage.ErrStorageUnitVersionConflict
}

// hierarchyOf は親・祖父母のcolumnからDomainの親参照と祖先の並びを組み立てる。
//
// 階層上限が3のため、祖先はrootから直接の親までの最大2件となる。
func hierarchyOf(
	row sqlc.OwnershipStorageUnit,
	parentPublicID *uuid.UUID,
	parentName *string,
	grandparentID *int64,
	grandparentPublicID *uuid.UUID,
	grandparentName *string,
) (domainstorage.Reference, []domainstorage.Reference) {
	if row.ParentID == nil || parentPublicID == nil {
		return domainstorage.Reference{}, nil
	}

	parent := domainstorage.Reference{
		ID:       domainstorage.StorageUnitID(*row.ParentID),
		PublicID: *parentPublicID,
		Name:     stringValue(parentName),
	}

	ancestors := make([]domainstorage.Reference, 0, 2)
	if grandparentID != nil && grandparentPublicID != nil {
		ancestors = append(ancestors, domainstorage.Reference{
			ID:       domainstorage.StorageUnitID(*grandparentID),
			PublicID: *grandparentPublicID,
			Name:     stringValue(grandparentName),
		})
	}
	return parent, append(ancestors, parent)
}

func toDomainStorageUnit(
	row sqlc.OwnershipStorageUnit,
	parent domainstorage.Reference,
	ancestors []domainstorage.Reference,
	childCount int32,
) domainstorage.StorageUnit {
	return domainstorage.ReconstructStorageUnit(
		domainstorage.ReconstructStorageUnitParams{
			ID:       domainstorage.StorageUnitID(row.ID),
			PublicID: row.PublicID,
			UserID:   domainauth.UserID(row.UserID),
			Attributes: domainstorage.Attributes{
				Name:                    row.Name,
				StorageType:             domainstorage.StorageType(row.StorageTypeCode),
				MobilityClass:           domainitem.MobilityClass(row.MobilityClassCode),
				Parent:                  parent,
				TareWeightGram:          row.TareWeightGram,
				MaximumWeightGram:       row.MaximumWeightGram,
				MaximumVolumeMilliliter: row.MaximumVolumeMilliliter,
				Description:             row.Description,
				SortOrder:               row.SortOrder,
			},
			Ancestors:  ancestors,
			ChildCount: childCount,
			CreatedAt:  utcTime(row.CreatedAt),
			UpdatedAt:  utcTime(row.UpdatedAt),
			ArchivedAt: optionalTime(row.DeletedAt),
			Version:    row.Version,
		})
}

// optionalParentID は親参照をNULL許容のquery parameterへ変換する。
func optionalParentID(parent domainstorage.Reference) *int64 {
	if parent.IsZero() {
		return nil
	}
	return pointerTo(parent.ID.Int64())
}

func pointerTo[T any](value T) *T { return &value }
