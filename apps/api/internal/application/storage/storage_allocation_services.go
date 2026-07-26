package storage

import (
	"context"

	"github.com/google/uuid"

	domainaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/idgenerator"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/transaction"
)

// AssignItemToStorageUnitParams は収納割当追加の入力。
type AssignItemToStorageUnitParams struct {
	UserID                     domainauth.UserID
	StorageUnitPublicID        uuid.UUID
	ItemPublicID               uuid.UUID
	Quantity                   int32
	ExpectedStorageUnitVersion int32
}

// AssignItemToStorageUnitService は所持品を収納単位へ割り当てる。
type AssignItemToStorageUnitService struct {
	dependencies       Dependencies
	publicIDGenerator  idgenerator.PublicIDGenerator
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewAssignItemToStorageUnitService はAssignItemToStorageUnitServiceを生成する。
func NewAssignItemToStorageUnitService(
	dependencies Dependencies,
	publicIDGenerator idgenerator.PublicIDGenerator,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *AssignItemToStorageUnitService {
	return &AssignItemToStorageUnitService{
		dependencies:       dependencies,
		publicIDGenerator:  publicIDGenerator,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute は所持品を収納単位へ数量付きで割り当てる。
//
// transaction境界 (設計書 20章):
//
//	BEGIN
//	  storage_units      SELECT  (収納単位とversionの確認)
//	  items              SELECT FOR UPDATE (割当数量合計の直列化)
//	  storage_allocations SELECT / INSERT
//	  storage_units      UPDATE  (versionを増加させ割当集合の競合を検知)
//	  audit_logs         INSERT
//	COMMIT
func (s *AssignItemToStorageUnitService) Execute(
	ctx context.Context,
	params AssignItemToStorageUnitParams,
) (StorageUnitContentsResult, error) {
	publicID, err := s.publicIDGenerator.NewPublicID()
	if err != nil {
		return StorageUnitContentsResult{}, shared.NewInternalError(
			"PUBLIC_ID_GENERATION_FAILED", "サーバーでエラーが発生しました。").WithCause(err)
	}

	var contents StorageUnitContentsResult
	err = s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		unit, err := s.dependencies.StorageUnits.FindByPublicID(
			ctx, params.UserID, params.StorageUnitPublicID)
		if err != nil {
			return err
		}
		if err := unit.EnsureVersionMatches(params.ExpectedStorageUnitVersion); err != nil {
			return err
		}

		allocatedItem, err := s.dependencies.Allocations.FindAllocatedItemByPublicID(
			ctx, params.UserID, params.ItemPublicID)
		if err != nil {
			return err
		}

		allocation, err := domainstorage.NewStorageAllocation(
			publicID, params.UserID, unit, allocatedItem, params.Quantity, s.clock.Now())
		if err != nil {
			return err
		}

		// 対象アイテム行をロックしてから合計を検証する。
		// ロック前に検証すると、並行する別割当と合わせて所有数量を超えうる。
		ownedQuantity, allocatedQuantity, err :=
			s.dependencies.Allocations.SumQuantityByItemIDForUpdate(
				ctx, params.UserID, allocatedItem.ID, 0)
		if err != nil {
			return err
		}
		if err := domainstorage.EnsureAllocatedQuantityWithinOwned(
			ownedQuantity, allocatedQuantity+int64(params.Quantity)); err != nil {
			return err
		}

		before, err := s.dependencies.loadHierarchy(ctx, params.UserID)
		if err != nil {
			return err
		}

		created, err := s.dependencies.Allocations.Create(ctx, allocation)
		if err != nil {
			return err
		}

		updatedUnit, err := s.dependencies.StorageUnits.TouchVersion(
			ctx, params.UserID, params.StorageUnitPublicID,
			params.ExpectedStorageUnitVersion, s.clock.Now())
		if err != nil {
			return err
		}

		contents, err = s.dependencies.buildContents(ctx, params.UserID, updatedUnit)
		if err != nil {
			return err
		}

		targetPublicID := created.PublicID()
		changes := mergeChanges(
			domainaudit.Diff(nil, created.AuditSnapshot()),
			capacityChanges(
				before.capacityOf(unit.ID()),
				toDomainCapacity(contents.StorageUnit.Capacity)),
		)
		changes["storageUnitPublicId"] = domainaudit.FieldChange{
			From: nil, To: params.StorageUnitPublicID.String(),
		}
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionStorageAllocationCreated,
			domainaudit.TargetTypeStorageAllocation,
			&targetPublicID,
			changes,
		)
	})
	if err != nil {
		return StorageUnitContentsResult{}, recordVersionConflict(
			ctx, s.dependencies, s.transactionManager,
			params.UserID, params.StorageUnitPublicID, "assign_item_to_storage_unit", err)
	}
	return contents, nil
}

// UpdateStorageAllocationParams は収納割当数量変更の入力。
type UpdateStorageAllocationParams struct {
	UserID                     domainauth.UserID
	StorageUnitPublicID        uuid.UUID
	AllocationPublicID         uuid.UUID
	Quantity                   int32
	ExpectedVersion            int32
	ExpectedStorageUnitVersion int32
}

// UpdateStorageAllocationService は収納割当の数量を変更する。
type UpdateStorageAllocationService struct {
	dependencies       Dependencies
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewUpdateStorageAllocationService はUpdateStorageAllocationServiceを生成する。
func NewUpdateStorageAllocationService(
	dependencies Dependencies,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *UpdateStorageAllocationService {
	return &UpdateStorageAllocationService{
		dependencies:       dependencies,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute は収納割当の数量を変更する。
//
// 割当自身のversionと収納単位のversionの双方を検証し、
// 収納内容編集画面での同時編集を検知する。
func (s *UpdateStorageAllocationService) Execute(
	ctx context.Context,
	params UpdateStorageAllocationParams,
) (StorageUnitContentsResult, error) {
	var contents StorageUnitContentsResult
	err := s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		unit, err := s.dependencies.StorageUnits.FindByPublicID(
			ctx, params.UserID, params.StorageUnitPublicID)
		if err != nil {
			return err
		}
		if err := unit.EnsureVersionMatches(params.ExpectedStorageUnitVersion); err != nil {
			return err
		}

		allocation, err := s.dependencies.Allocations.FindByPublicID(
			ctx, params.UserID, params.AllocationPublicID)
		if err != nil {
			return err
		}
		// 別の収納単位の割当を、この収納単位のpathから操作させない。
		if allocation.StorageUnitID() != unit.ID() {
			return domainstorage.ErrStorageAllocationNotFound
		}

		beforeSnapshot := allocation.AuditSnapshot()
		next, err := allocation.ChangeQuantity(
			params.Quantity, params.ExpectedVersion, s.clock.Now())
		if err != nil {
			return err
		}

		// 変更対象の割当を合計から除き、変更後の値で検証する。
		ownedQuantity, otherAllocatedQuantity, err :=
			s.dependencies.Allocations.SumQuantityByItemIDForUpdate(
				ctx, params.UserID, allocation.ItemID(), allocation.ID())
		if err != nil {
			return err
		}
		if err := domainstorage.EnsureAllocatedQuantityWithinOwned(
			ownedQuantity, otherAllocatedQuantity+int64(params.Quantity)); err != nil {
			return err
		}

		before, err := s.dependencies.loadHierarchy(ctx, params.UserID)
		if err != nil {
			return err
		}

		updated, err := s.dependencies.Allocations.UpdateQuantity(
			ctx, next, params.ExpectedVersion)
		if err != nil {
			return err
		}

		updatedUnit, err := s.dependencies.StorageUnits.TouchVersion(
			ctx, params.UserID, params.StorageUnitPublicID,
			params.ExpectedStorageUnitVersion, s.clock.Now())
		if err != nil {
			return err
		}

		contents, err = s.dependencies.buildContents(ctx, params.UserID, updatedUnit)
		if err != nil {
			return err
		}

		targetPublicID := updated.PublicID()
		changes := mergeChanges(
			domainaudit.Diff(beforeSnapshot, updated.AuditSnapshot()),
			capacityChanges(
				before.capacityOf(unit.ID()),
				toDomainCapacity(contents.StorageUnit.Capacity)),
		)
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionStorageAllocationUpdated,
			domainaudit.TargetTypeStorageAllocation,
			&targetPublicID,
			changes,
		)
	})
	if err != nil {
		return StorageUnitContentsResult{}, recordVersionConflict(
			ctx, s.dependencies, s.transactionManager,
			params.UserID, params.StorageUnitPublicID, "update_storage_allocation", err)
	}
	return contents, nil
}

// RemoveStorageAllocationParams は収納割当削除の入力。
type RemoveStorageAllocationParams struct {
	UserID                     domainauth.UserID
	StorageUnitPublicID        uuid.UUID
	AllocationPublicID         uuid.UUID
	ExpectedVersion            int32
	ExpectedStorageUnitVersion int32
}

// RemoveStorageAllocationService は収納割当を削除する。
type RemoveStorageAllocationService struct {
	dependencies       Dependencies
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewRemoveStorageAllocationService はRemoveStorageAllocationServiceを生成する。
func NewRemoveStorageAllocationService(
	dependencies Dependencies,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *RemoveStorageAllocationService {
	return &RemoveStorageAllocationService{
		dependencies:       dependencies,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute は収納割当を削除する。
//
// 削除は数量整合性を緩める方向のため所有数量の再検証を必要としない。
func (s *RemoveStorageAllocationService) Execute(
	ctx context.Context,
	params RemoveStorageAllocationParams,
) (StorageUnitContentsResult, error) {
	var contents StorageUnitContentsResult
	err := s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		unit, err := s.dependencies.StorageUnits.FindByPublicID(
			ctx, params.UserID, params.StorageUnitPublicID)
		if err != nil {
			return err
		}
		if err := unit.EnsureVersionMatches(params.ExpectedStorageUnitVersion); err != nil {
			return err
		}

		allocation, err := s.dependencies.Allocations.FindByPublicID(
			ctx, params.UserID, params.AllocationPublicID)
		if err != nil {
			return err
		}
		if allocation.StorageUnitID() != unit.ID() {
			return domainstorage.ErrStorageAllocationNotFound
		}
		if err := allocation.EnsureVersionMatches(params.ExpectedVersion); err != nil {
			return err
		}

		before, err := s.dependencies.loadHierarchy(ctx, params.UserID)
		if err != nil {
			return err
		}

		if err := s.dependencies.Allocations.Delete(
			ctx, params.UserID, params.AllocationPublicID, params.ExpectedVersion); err != nil {
			return err
		}

		updatedUnit, err := s.dependencies.StorageUnits.TouchVersion(
			ctx, params.UserID, params.StorageUnitPublicID,
			params.ExpectedStorageUnitVersion, s.clock.Now())
		if err != nil {
			return err
		}

		contents, err = s.dependencies.buildContents(ctx, params.UserID, updatedUnit)
		if err != nil {
			return err
		}

		targetPublicID := allocation.PublicID()
		changes := mergeChanges(
			domainaudit.Diff(allocation.AuditSnapshot(), map[string]any{
				"itemPublicId": nil,
				"quantity":     nil,
			}),
			capacityChanges(
				before.capacityOf(unit.ID()),
				toDomainCapacity(contents.StorageUnit.Capacity)),
		)
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionStorageAllocationDeleted,
			domainaudit.TargetTypeStorageAllocation,
			&targetPublicID,
			changes,
		)
	})
	if err != nil {
		return StorageUnitContentsResult{}, recordVersionConflict(
			ctx, s.dependencies, s.transactionManager,
			params.UserID, params.StorageUnitPublicID, "remove_storage_allocation", err)
	}
	return contents, nil
}

// AllocationInput は一括置換で指定する割当1件。
type AllocationInput struct {
	ItemPublicID uuid.UUID
	Quantity     int32
}

// ReplaceStorageAllocationsParams は一括置換の入力。
type ReplaceStorageAllocationsParams struct {
	UserID                     domainauth.UserID
	StorageUnitPublicID        uuid.UUID
	Allocations                []AllocationInput
	ExpectedStorageUnitVersion int32
}

// ReplaceStorageAllocationsService は収納単位の割当集合を置き換える。
type ReplaceStorageAllocationsService struct {
	dependencies       Dependencies
	publicIDGenerator  idgenerator.PublicIDGenerator
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewReplaceStorageAllocationsService はReplaceStorageAllocationsServiceを生成する。
func NewReplaceStorageAllocationsService(
	dependencies Dependencies,
	publicIDGenerator idgenerator.PublicIDGenerator,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *ReplaceStorageAllocationsService {
	return &ReplaceStorageAllocationsService{
		dependencies:       dependencies,
		publicIDGenerator:  publicIDGenerator,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute は割当集合を指定内容へ置き換える。
//
// 含まれない既存割当は削除する。空配列は「中身を空にする」を意味する。
//
// transaction境界 (設計書 20章):
//
//	BEGIN
//	  storage_units       SELECT (versionの確認)
//	  items               SELECT FOR UPDATE (対象アイテムを内部ID昇順でロック)
//	  storage_allocations SELECT (他収納単位への割当合計)
//	  storage_allocations DELETE / INSERT
//	  storage_units       UPDATE (versionを増加)
//	  audit_logs          INSERT
//	COMMIT
//
// 途中で検証に失敗した場合はtransaction全体をrollbackし、
// 一部だけ適用された状態を残さない。
func (s *ReplaceStorageAllocationsService) Execute(
	ctx context.Context,
	params ReplaceStorageAllocationsParams,
) (StorageUnitContentsResult, error) {
	if err := ensureNoDuplicatedItem(params.Allocations); err != nil {
		return StorageUnitContentsResult{}, err
	}

	publicIDs := make([]uuid.UUID, 0, len(params.Allocations))
	for range params.Allocations {
		publicID, err := s.publicIDGenerator.NewPublicID()
		if err != nil {
			return StorageUnitContentsResult{}, shared.NewInternalError(
				"PUBLIC_ID_GENERATION_FAILED", "サーバーでエラーが発生しました。").WithCause(err)
		}
		publicIDs = append(publicIDs, publicID)
	}

	var contents StorageUnitContentsResult
	err := s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		unit, err := s.dependencies.StorageUnits.FindByPublicID(
			ctx, params.UserID, params.StorageUnitPublicID)
		if err != nil {
			return err
		}
		if err := unit.EnsureVersionMatches(params.ExpectedStorageUnitVersion); err != nil {
			return err
		}
		if err := unit.EnsureAssignable(); err != nil {
			return err
		}

		itemPublicIDs := make([]uuid.UUID, 0, len(params.Allocations))
		for _, input := range params.Allocations {
			itemPublicIDs = append(itemPublicIDs, input.ItemPublicID)
		}
		allocatedItems, err := s.dependencies.Allocations.ResolveAllocatedItems(
			ctx, params.UserID, itemPublicIDs)
		if err != nil {
			return err
		}

		before, err := s.dependencies.loadHierarchy(ctx, params.UserID)
		if err != nil {
			return err
		}
		beforeAllocations := before.allocations[unit.ID()]

		// 新しい割当集合を組み立てる。archive済みアイテムの新規割当禁止と
		// 数量の範囲はDomainが検証する。
		newAllocations := make([]domainstorage.StorageAllocation, 0, len(params.Allocations))
		for index, input := range params.Allocations {
			allocatedItem := allocatedItems[index]
			// 置換前から同じアイテムが入っている場合、archive済みでも
			// 数量の変更を許す。archiveは手放しではなく、物理的には
			// 収納単位へ入ったままであるため取り出しを強制しない。
			if allocatedItem.IsArchived &&
				!containsItem(beforeAllocations, allocatedItem.ID) {
				return domainstorage.ErrStorageAllocationItemArchived
			}

			allocation, err := domainstorage.NewStorageAllocation(
				publicIDs[index],
				params.UserID,
				unit,
				withArchivedAllowed(allocatedItem),
				input.Quantity,
				s.clock.Now(),
			)
			if err != nil {
				return err
			}
			newAllocations = append(newAllocations, allocation)
		}

		if err := s.ensureQuantitiesWithinOwned(
			ctx, params.UserID, unit.ID(), newAllocations); err != nil {
			return err
		}

		if err := s.dependencies.Allocations.DeleteByStorageUnitID(
			ctx, params.UserID, unit.ID()); err != nil {
			return err
		}
		for _, allocation := range newAllocations {
			if _, err := s.dependencies.Allocations.Create(ctx, allocation); err != nil {
				return err
			}
		}

		updatedUnit, err := s.dependencies.StorageUnits.TouchVersion(
			ctx, params.UserID, params.StorageUnitPublicID,
			params.ExpectedStorageUnitVersion, s.clock.Now())
		if err != nil {
			return err
		}

		contents, err = s.dependencies.buildContents(ctx, params.UserID, updatedUnit)
		if err != nil {
			return err
		}

		targetPublicID := unit.PublicID()
		changes := mergeChanges(
			domainaudit.Changes{
				"allocations": domainaudit.FieldChange{
					From: allocationSummaries(beforeAllocations),
					To:   allocationSummaries(newAllocations),
				},
			},
			capacityChanges(
				before.capacityOf(unit.ID()),
				toDomainCapacity(contents.StorageUnit.Capacity)),
		)
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionStorageAllocationsReplaced,
			domainaudit.TargetTypeStorageUnit,
			&targetPublicID,
			changes,
		)
	})
	if err != nil {
		return StorageUnitContentsResult{}, recordVersionConflict(
			ctx, s.dependencies, s.transactionManager,
			params.UserID, params.StorageUnitPublicID, "replace_storage_allocations", err)
	}
	return contents, nil
}

// ensureQuantitiesWithinOwned は対象アイテムをロックし、置換後の割当数量合計が
// 所有数量以下であることを検証する。
//
// 本収納単位の既存割当は置き換えられるため合計から除き、
// 他収納単位への割当だけを既存分として数える。
func (s *ReplaceStorageAllocationsService) ensureQuantitiesWithinOwned(
	ctx context.Context,
	userID domainauth.UserID,
	storageUnitID domainstorage.StorageUnitID,
	newAllocations []domainstorage.StorageAllocation,
) error {
	if len(newAllocations) == 0 {
		return nil
	}

	itemIDs := make([]domainitem.ItemID, 0, len(newAllocations))
	for _, allocation := range newAllocations {
		itemIDs = append(itemIDs, allocation.ItemID())
	}

	ownedQuantities, err := s.dependencies.Allocations.LockItemsForUpdate(ctx, userID, itemIDs)
	if err != nil {
		return err
	}

	// ロック後に他収納単位への割当合計を読み、並行追加を取りこぼさない。
	otherTotals := make(map[domainitem.ItemID]int64, len(itemIDs))
	allocationsByItemID, err := s.dependencies.Allocations.ListByItemIDs(ctx, userID, itemIDs)
	if err != nil {
		return err
	}
	for itemID, allocations := range allocationsByItemID {
		for _, allocation := range allocations {
			if allocation.StorageUnitID() == storageUnitID {
				continue
			}
			otherTotals[itemID] += int64(allocation.Quantity())
		}
	}

	for _, allocation := range newAllocations {
		itemID := allocation.ItemID()
		ownedQuantity, ok := ownedQuantities[itemID]
		if !ok {
			return domainitem.ErrItemNotFound
		}
		if err := domainstorage.EnsureAllocatedQuantityWithinOwned(
			ownedQuantity, otherTotals[itemID]+int64(allocation.Quantity())); err != nil {
			return err
		}
	}
	return nil
}

// ListItemStorageAllocationsParams はアイテム側の割当一覧の入力。
type ListItemStorageAllocationsParams struct {
	UserID       domainauth.UserID
	ItemPublicID uuid.UUID
}

// ListItemStorageAllocationsService は1アイテムの収納割当を取得する。
type ListItemStorageAllocationsService struct {
	dependencies Dependencies
}

// NewListItemStorageAllocationsService はListItemStorageAllocationsServiceを生成する。
func NewListItemStorageAllocationsService(
	dependencies Dependencies,
) *ListItemStorageAllocationsService {
	return &ListItemStorageAllocationsService{dependencies: dependencies}
}

// Execute は1アイテムがどの収納単位へ何個ずつ入っているかを返す。
//
// 未割当数量はDBへ保存せず取得時に算出する。
func (s *ListItemStorageAllocationsService) Execute(
	ctx context.Context,
	params ListItemStorageAllocationsParams,
) (ListItemStorageAllocationsResult, error) {
	allocatedItem, err := s.dependencies.Allocations.FindAllocatedItemByPublicID(
		ctx, params.UserID, params.ItemPublicID)
	if err != nil {
		return ListItemStorageAllocationsResult{}, err
	}

	allocations, err := s.dependencies.Allocations.ListByItemID(
		ctx, params.UserID, allocatedItem.ID)
	if err != nil {
		return ListItemStorageAllocationsResult{}, err
	}

	var assignedQuantity int64
	results := make([]ItemStorageAllocationResult, 0, len(allocations))
	for _, allocation := range allocations {
		assignedQuantity += int64(allocation.Quantity())
		results = append(results, newItemStorageAllocationResult(allocation))
	}

	return ListItemStorageAllocationsResult{
		Items:            results,
		Quantity:         allocatedItem.Quantity,
		AssignedQuantity: int32(assignedQuantity),
		UnassignedQuantity: domainstorage.UnassignedQuantity(
			allocatedItem.Quantity, assignedQuantity),
	}, nil
}

// ensureNoDuplicatedItem は一括置換の入力へ同一アイテムが複数含まれないことを確認する。
//
// 同一収納単位・同一アイテムは1件というDB制約に到達する前に、
// 利用者へ意味の分かるerrorを返す。
func ensureNoDuplicatedItem(inputs []AllocationInput) error {
	seen := make(map[uuid.UUID]struct{}, len(inputs))
	for _, input := range inputs {
		if _, ok := seen[input.ItemPublicID]; ok {
			return domainstorage.ErrStorageAllocationDuplicatedItem
		}
		seen[input.ItemPublicID] = struct{}{}
	}
	return nil
}

// containsItem は割当集合へ指定アイテムが含まれるかを返す。
func containsItem(
	allocations []domainstorage.StorageAllocation,
	itemID domainitem.ItemID,
) bool {
	for _, allocation := range allocations {
		if allocation.ItemID() == itemID {
			return true
		}
	}
	return false
}

// withArchivedAllowed はarchive状態を落とした複製を返す。
//
// 既存割当の数量変更としてEntityを再生成する場合に使用し、
// archive済みアイテムの「新規割当禁止」だけを回避する。
func withArchivedAllowed(
	allocatedItem domainstorage.AllocatedItem,
) domainstorage.AllocatedItem {
	allocatedItem.IsArchived = false
	return allocatedItem
}

// allocationSummaries は監査ログへ保存する割当集合の要約を返す。
//
// 内部IDを含めず、アイテムのpublicIdと数量だけを残す。
func allocationSummaries(allocations []domainstorage.StorageAllocation) []string {
	summaries := make([]string, 0, len(allocations))
	for _, allocation := range allocations {
		summaries = append(summaries, allocation.Item().PublicID.String()+
			":"+itoa(allocation.Quantity()))
	}
	return summaries
}

// toDomainCapacity は結果表現からDomainのCapacityへ戻す。
//
// 監査ログの超過状態の比較にのみ使用する。
func toDomainCapacity(capacity CapacityResult) domainstorage.Capacity {
	return domainstorage.Capacity{
		IsWeightExceeded: capacity.IsWeightExceeded,
		IsVolumeExceeded: capacity.IsVolumeExceeded,
	}
}
