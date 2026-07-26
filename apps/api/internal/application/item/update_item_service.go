package item

import (
	"context"

	"github.com/google/uuid"

	domainaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/transaction"
)

// UpdateItemParams は所持品更新の入力。
type UpdateItemParams struct {
	UserID          domainauth.UserID
	PublicID        uuid.UUID
	Attributes      AttributesParams
	ExpectedVersion int32
}

// UpdateItemResult は所持品更新の結果。
type UpdateItemResult struct {
	Item ItemResult
}

// UpdateItemService は所持品を更新する。
type UpdateItemService struct {
	dependencies       Dependencies
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewUpdateItemService はUpdateItemServiceを生成する。
func NewUpdateItemService(
	dependencies Dependencies,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *UpdateItemService {
	return &UpdateItemService{
		dependencies:       dependencies,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute は所持品を更新する。
//
// 更新前のEntityを取得して監査ログの差分を作り、Domainでversionを検証した上で
// Repositoryの更新条件にもversionを含める (設計書 11.7)。
func (s *UpdateItemService) Execute(
	ctx context.Context,
	params UpdateItemParams,
) (UpdateItemResult, error) {
	var updated domainitem.Item

	err := s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		existing, err := s.dependencies.Items.FindByPublicID(ctx, params.UserID, params.PublicID)
		if err != nil {
			return err
		}
		before := existing.AuditSnapshot()

		attributes, err := s.dependencies.resolveAttributes(ctx, params.UserID, params.Attributes)
		if err != nil {
			return err
		}

		next, err := existing.Update(attributes, params.ExpectedVersion, s.clock.Now())
		if err != nil {
			return err
		}

		// 所有数量を収納割当数量の合計未満へ変更できない (設計書 7.2 / Phase 2)。
		// 対象アイテム行をロックしたうえで合計を読み、並行する割当追加と
		// 競合しても不変条件が破られないようにする (設計書 20章)。
		if attributes.Quantity < existing.Quantity() {
			_, allocatedQuantity, err := s.dependencies.StorageAllocations.
				SumQuantityByItemIDForUpdate(ctx, params.UserID, existing.ID(), 0)
			if err != nil {
				return err
			}
			if err := domainstorage.EnsureAllocatedQuantityWithinOwned(
				attributes.Quantity, allocatedQuantity); err != nil {
				return err
			}
		}

		updated, err = s.dependencies.Items.Update(ctx, next, params.ExpectedVersion)
		if err != nil {
			return err
		}

		changes := domainaudit.Diff(before, updated.AuditSnapshot())
		if len(changes) == 0 {
			// 実質的な変更が無い場合も更新操作としては成立しているが、
			// 差分の無い履歴は残さない (設計書 22章)。
			return nil
		}

		targetPublicID := updated.PublicID()
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionItemUpdated,
			domainaudit.TargetTypeItem,
			&targetPublicID,
			changes,
		)
	})
	if err != nil {
		return UpdateItemResult{}, err
	}

	result, err := s.dependencies.newItemResultWithStorage(ctx, params.UserID, updated)
	if err != nil {
		return UpdateItemResult{}, err
	}
	return UpdateItemResult{Item: result}, nil
}
