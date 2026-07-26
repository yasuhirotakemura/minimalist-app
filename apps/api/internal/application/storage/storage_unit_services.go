package storage

import (
	"context"

	"github.com/google/uuid"

	domainaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/idgenerator"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/transaction"
)

// CreateStorageUnitParams は収納単位登録の入力。
type CreateStorageUnitParams struct {
	UserID     domainauth.UserID
	Attributes AttributesParams
}

// CreateStorageUnitResult は収納単位登録の結果。
type CreateStorageUnitResult struct {
	StorageUnit StorageUnitResult
}

// CreateStorageUnitService は収納単位を登録する。
type CreateStorageUnitService struct {
	dependencies       Dependencies
	publicIDGenerator  idgenerator.PublicIDGenerator
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewCreateStorageUnitService はCreateStorageUnitServiceを生成する。
func NewCreateStorageUnitService(
	dependencies Dependencies,
	publicIDGenerator idgenerator.PublicIDGenerator,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *CreateStorageUnitService {
	return &CreateStorageUnitService{
		dependencies:       dependencies,
		publicIDGenerator:  publicIDGenerator,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute は収納単位を登録する。
//
// 親の解決、階層制約の検証、作成、監査ログ記録を単一transactionで実行する。
func (s *CreateStorageUnitService) Execute(
	ctx context.Context,
	params CreateStorageUnitParams,
) (CreateStorageUnitResult, error) {
	publicID, err := s.publicIDGenerator.NewPublicID()
	if err != nil {
		return CreateStorageUnitResult{}, shared.NewInternalError(
			"PUBLIC_ID_GENERATION_FAILED", "サーバーでエラーが発生しました。").WithCause(err)
	}

	var created domainstorage.StorageUnit
	err = s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		parent, err := s.dependencies.resolveParent(
			ctx, params.UserID, params.Attributes.ParentStorageUnitPublicID)
		if err != nil {
			return err
		}

		newUnit, err := domainstorage.NewStorageUnit(
			publicID,
			params.UserID,
			params.Attributes.toDomainAttributes(),
			parent,
			s.clock.Now(),
		)
		if err != nil {
			return err
		}

		created, err = s.dependencies.StorageUnits.Create(ctx, newUnit)
		if err != nil {
			return err
		}

		targetPublicID := created.PublicID()
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionStorageUnitCreated,
			domainaudit.TargetTypeStorageUnit,
			&targetPublicID,
			domainaudit.Diff(nil, created.AuditSnapshot()),
		)
	})
	if err != nil {
		return CreateStorageUnitResult{}, err
	}

	// 作成直後は中身も子も無いため、集計は自重のみとなる。
	capacity := domainstorage.CalculateCapacity(domainstorage.CapacityInput{
		TareWeightGram:          created.Attributes().TareWeightGram,
		MaximumWeightGram:       created.Attributes().MaximumWeightGram,
		MaximumVolumeMilliliter: created.Attributes().MaximumVolumeMilliliter,
	})
	return CreateStorageUnitResult{
		StorageUnit: newStorageUnitResult(created, capacity),
	}, nil
}

// UpdateStorageUnitParams は収納単位更新の入力。
type UpdateStorageUnitParams struct {
	UserID          domainauth.UserID
	PublicID        uuid.UUID
	Attributes      AttributesParams
	ExpectedVersion int32
}

// UpdateStorageUnitResult は収納単位更新の結果。
type UpdateStorageUnitResult struct {
	StorageUnit StorageUnitResult
}

// UpdateStorageUnitService は収納単位を更新する。
type UpdateStorageUnitService struct {
	dependencies       Dependencies
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewUpdateStorageUnitService はUpdateStorageUnitServiceを生成する。
func NewUpdateStorageUnitService(
	dependencies Dependencies,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *UpdateStorageUnitService {
	return &UpdateStorageUnitService{
		dependencies:       dependencies,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute は収納単位を更新する。
//
// 親の変更で階層が移動するため、自身を根とする部分木の高さを求め、
// 移動後も階層上限を超えないことをDomainが検証する。
func (s *UpdateStorageUnitService) Execute(
	ctx context.Context,
	params UpdateStorageUnitParams,
) (UpdateStorageUnitResult, error) {
	var updated domainstorage.StorageUnit
	err := s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		current, err := s.dependencies.StorageUnits.FindByPublicID(
			ctx, params.UserID, params.PublicID)
		if err != nil {
			return err
		}

		parent, err := s.dependencies.resolveParent(
			ctx, params.UserID, params.Attributes.ParentStorageUnitPublicID)
		if err != nil {
			return err
		}

		snapshot, err := s.dependencies.loadHierarchy(ctx, params.UserID)
		if err != nil {
			return err
		}

		before := current.AuditSnapshot()
		next, err := current.Update(
			params.Attributes.toDomainAttributes(),
			parent,
			snapshot.subtreeHeight(current.ID()),
			params.ExpectedVersion,
			s.clock.Now(),
		)
		if err != nil {
			return err
		}

		updated, err = s.dependencies.StorageUnits.Update(ctx, next, params.ExpectedVersion)
		if err != nil {
			return err
		}

		targetPublicID := updated.PublicID()
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionStorageUnitUpdated,
			domainaudit.TargetTypeStorageUnit,
			&targetPublicID,
			domainaudit.Diff(before, updated.AuditSnapshot()),
		)
	})
	if err != nil {
		return UpdateStorageUnitResult{}, s.recordConflict(ctx, params.UserID, params.PublicID, err)
	}

	snapshot, err := s.dependencies.loadHierarchy(ctx, params.UserID)
	if err != nil {
		return UpdateStorageUnitResult{}, err
	}
	return UpdateStorageUnitResult{
		StorageUnit: newStorageUnitResult(updated, snapshot.capacityOf(updated.ID())),
	}, nil
}

func (s *UpdateStorageUnitService) recordConflict(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
	cause error,
) error {
	return recordVersionConflict(
		ctx, s.dependencies, s.transactionManager, userID, publicID, "update_storage_unit", cause)
}

// ArchiveStorageUnitParams は収納単位archiveの入力。
type ArchiveStorageUnitParams struct {
	UserID          domainauth.UserID
	PublicID        uuid.UUID
	ExpectedVersion int32
}

// ArchiveStorageUnitResult は収納単位archiveの結果。
type ArchiveStorageUnitResult struct {
	StorageUnit StorageUnitResult
}

// ArchiveStorageUnitService は収納単位をarchiveする。
type ArchiveStorageUnitService struct {
	dependencies       Dependencies
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewArchiveStorageUnitService はArchiveStorageUnitServiceを生成する。
func NewArchiveStorageUnitService(
	dependencies Dependencies,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *ArchiveStorageUnitService {
	return &ArchiveStorageUnitService{
		dependencies:       dependencies,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute は収納単位をarchiveする。
//
// 子収納単位・収納割当が残る場合は拒否し、親のarchiveで子を暗黙に
// archiveしない。利用者に順序を明示させることで復元時の状態を予測可能にする。
func (s *ArchiveStorageUnitService) Execute(
	ctx context.Context,
	params ArchiveStorageUnitParams,
) (ArchiveStorageUnitResult, error) {
	var archived domainstorage.StorageUnit
	err := s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		current, err := s.dependencies.StorageUnits.FindByPublicID(
			ctx, params.UserID, params.PublicID)
		if err != nil {
			return err
		}

		childCount, err := s.dependencies.StorageUnits.CountActiveChildren(
			ctx, params.UserID, current.ID())
		if err != nil {
			return err
		}
		allocationCount, err := s.dependencies.Allocations.CountByStorageUnitID(
			ctx, params.UserID, current.ID())
		if err != nil {
			return err
		}

		before := current.AuditSnapshot()
		if _, err := current.Archive(
			int32(childCount), allocationCount, params.ExpectedVersion, s.clock.Now(),
		); err != nil {
			return err
		}

		archived, err = s.dependencies.StorageUnits.Archive(
			ctx, params.UserID, params.PublicID, params.ExpectedVersion, s.clock.Now())
		if err != nil {
			return err
		}

		targetPublicID := archived.PublicID()
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionStorageUnitArchived,
			domainaudit.TargetTypeStorageUnit,
			&targetPublicID,
			domainaudit.Diff(before, archived.AuditSnapshot()),
		)
	})
	if err != nil {
		return ArchiveStorageUnitResult{}, recordVersionConflict(
			ctx, s.dependencies, s.transactionManager,
			params.UserID, params.PublicID, "archive_storage_unit", err)
	}

	snapshot, err := s.dependencies.loadHierarchy(ctx, params.UserID)
	if err != nil {
		return ArchiveStorageUnitResult{}, err
	}
	return ArchiveStorageUnitResult{
		StorageUnit: newStorageUnitResult(archived, snapshot.capacityOf(archived.ID())),
	}, nil
}

// RestoreStorageUnitParams は収納単位復元の入力。
type RestoreStorageUnitParams struct {
	UserID          domainauth.UserID
	PublicID        uuid.UUID
	ExpectedVersion int32
}

// RestoreStorageUnitResult は収納単位復元の結果。
type RestoreStorageUnitResult struct {
	StorageUnit StorageUnitResult
}

// RestoreStorageUnitService はarchive済みの収納単位を復元する。
type RestoreStorageUnitService struct {
	dependencies       Dependencies
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewRestoreStorageUnitService はRestoreStorageUnitServiceを生成する。
func NewRestoreStorageUnitService(
	dependencies Dependencies,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *RestoreStorageUnitService {
	return &RestoreStorageUnitService{
		dependencies:       dependencies,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute はarchive済みの収納単位を復元する。
//
// 親がarchive済みの場合は拒否し、階層の途中が欠けた状態を作らない。
func (s *RestoreStorageUnitService) Execute(
	ctx context.Context,
	params RestoreStorageUnitParams,
) (RestoreStorageUnitResult, error) {
	var restored domainstorage.StorageUnit
	err := s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		current, err := s.dependencies.StorageUnits.FindByPublicID(
			ctx, params.UserID, params.PublicID)
		if err != nil {
			return err
		}

		var parent *domainstorage.StorageUnit
		if current.HasParent() {
			parentPublicID := current.Parent().PublicID
			parent, err = s.dependencies.resolveParent(ctx, params.UserID, &parentPublicID)
			if err != nil {
				return err
			}
		}

		before := current.AuditSnapshot()
		if _, err := current.Restore(
			parent, params.ExpectedVersion, s.clock.Now()); err != nil {
			return err
		}

		restored, err = s.dependencies.StorageUnits.Restore(
			ctx, params.UserID, params.PublicID, params.ExpectedVersion, s.clock.Now())
		if err != nil {
			return err
		}

		targetPublicID := restored.PublicID()
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionStorageUnitRestored,
			domainaudit.TargetTypeStorageUnit,
			&targetPublicID,
			domainaudit.Diff(before, restored.AuditSnapshot()),
		)
	})
	if err != nil {
		return RestoreStorageUnitResult{}, recordVersionConflict(
			ctx, s.dependencies, s.transactionManager,
			params.UserID, params.PublicID, "restore_storage_unit", err)
	}

	snapshot, err := s.dependencies.loadHierarchy(ctx, params.UserID)
	if err != nil {
		return RestoreStorageUnitResult{}, err
	}
	return RestoreStorageUnitResult{
		StorageUnit: newStorageUnitResult(restored, snapshot.capacityOf(restored.ID())),
	}, nil
}

// GetStorageUnitParams は収納単位取得の入力。
type GetStorageUnitParams struct {
	UserID   domainauth.UserID
	PublicID uuid.UUID
}

// GetStorageUnitResult は収納単位取得の結果。
type GetStorageUnitResult struct {
	StorageUnit StorageUnitResult
}

// GetStorageUnitService は収納単位を取得する。
type GetStorageUnitService struct {
	dependencies Dependencies
}

// NewGetStorageUnitService はGetStorageUnitServiceを生成する。
func NewGetStorageUnitService(dependencies Dependencies) *GetStorageUnitService {
	return &GetStorageUnitService{dependencies: dependencies}
}

// Execute は収納単位を取得する。
func (s *GetStorageUnitService) Execute(
	ctx context.Context,
	params GetStorageUnitParams,
) (GetStorageUnitResult, error) {
	unit, err := s.dependencies.StorageUnits.FindByPublicID(
		ctx, params.UserID, params.PublicID)
	if err != nil {
		return GetStorageUnitResult{}, err
	}

	snapshot, err := s.dependencies.loadHierarchy(ctx, params.UserID)
	if err != nil {
		return GetStorageUnitResult{}, err
	}
	return GetStorageUnitResult{
		StorageUnit: newStorageUnitResult(unit, snapshot.capacityOf(unit.ID())),
	}, nil
}

// ListStorageUnitsParams は収納単位一覧の入力。
type ListStorageUnitsParams struct {
	UserID   domainauth.UserID
	Criteria domainstorage.ListCriteriaInput
}

// ListStorageUnitsService は収納単位一覧を取得する。
type ListStorageUnitsService struct {
	dependencies Dependencies
}

// NewListStorageUnitsService はListStorageUnitsServiceを生成する。
func NewListStorageUnitsService(dependencies Dependencies) *ListStorageUnitsService {
	return &ListStorageUnitsService{dependencies: dependencies}
}

// Execute は収納単位一覧を取得する。
//
// 集計は子孫を含むため、pageに含まれない収納単位も含めて木全体を読み込む。
func (s *ListStorageUnitsService) Execute(
	ctx context.Context,
	params ListStorageUnitsParams,
) (ListStorageUnitsResult, error) {
	criteria, err := domainstorage.NewListCriteria(params.Criteria)
	if err != nil {
		return ListStorageUnitsResult{}, err
	}

	units, err := s.dependencies.StorageUnits.List(ctx, params.UserID, criteria)
	if err != nil {
		return ListStorageUnitsResult{}, err
	}
	totalCount, err := s.dependencies.StorageUnits.Count(ctx, params.UserID, criteria)
	if err != nil {
		return ListStorageUnitsResult{}, err
	}

	snapshot, err := s.dependencies.loadHierarchy(ctx, params.UserID)
	if err != nil {
		return ListStorageUnitsResult{}, err
	}

	results := make([]StorageUnitResult, 0, len(units))
	for _, unit := range units {
		results = append(results, newStorageUnitResult(unit, snapshot.capacityOf(unit.ID())))
	}

	return ListStorageUnitsResult{
		Items:      results,
		Pagination: newPaginationResult(criteria.Limit, criteria.Offset, totalCount),
	}, nil
}

// CalculateStorageUnitCapacityParams は容量集計の入力。
type CalculateStorageUnitCapacityParams struct {
	UserID   domainauth.UserID
	PublicID uuid.UUID
}

// CalculateStorageUnitCapacityResult は容量集計の結果。
type CalculateStorageUnitCapacityResult struct {
	Capacity CapacityResult
}

// CalculateStorageUnitCapacityService は収納単位の重量・容積を集計する。
type CalculateStorageUnitCapacityService struct {
	dependencies Dependencies
}

// NewCalculateStorageUnitCapacityService はCalculateStorageUnitCapacityServiceを生成する。
func NewCalculateStorageUnitCapacityService(
	dependencies Dependencies,
) *CalculateStorageUnitCapacityService {
	return &CalculateStorageUnitCapacityService{dependencies: dependencies}
}

// Execute は収納単位の集計と超過判定を返す (設計書 16.2 / 16.3)。
func (s *CalculateStorageUnitCapacityService) Execute(
	ctx context.Context,
	params CalculateStorageUnitCapacityParams,
) (CalculateStorageUnitCapacityResult, error) {
	unit, err := s.dependencies.StorageUnits.FindByPublicID(
		ctx, params.UserID, params.PublicID)
	if err != nil {
		return CalculateStorageUnitCapacityResult{}, err
	}

	snapshot, err := s.dependencies.loadHierarchy(ctx, params.UserID)
	if err != nil {
		return CalculateStorageUnitCapacityResult{}, err
	}
	return CalculateStorageUnitCapacityResult{
		Capacity: newCapacityResult(snapshot.capacityOf(unit.ID())),
	}, nil
}

// GetStorageUnitContentsParams は収納内容取得の入力。
type GetStorageUnitContentsParams struct {
	UserID   domainauth.UserID
	PublicID uuid.UUID
}

// GetStorageUnitContentsService は収納単位の内容を取得する。
type GetStorageUnitContentsService struct {
	dependencies Dependencies
}

// NewGetStorageUnitContentsService はGetStorageUnitContentsServiceを生成する。
func NewGetStorageUnitContentsService(
	dependencies Dependencies,
) *GetStorageUnitContentsService {
	return &GetStorageUnitContentsService{dependencies: dependencies}
}

// Execute は直接割当されたアイテム、直接の子収納単位、容量集計を返す。
func (s *GetStorageUnitContentsService) Execute(
	ctx context.Context,
	params GetStorageUnitContentsParams,
) (StorageUnitContentsResult, error) {
	unit, err := s.dependencies.StorageUnits.FindByPublicID(
		ctx, params.UserID, params.PublicID)
	if err != nil {
		return StorageUnitContentsResult{}, err
	}
	return s.dependencies.buildContents(ctx, params.UserID, unit)
}
