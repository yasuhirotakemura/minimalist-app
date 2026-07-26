package item

import (
	"context"

	"github.com/google/uuid"

	domainaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/transaction"
)

// RestoreItemParams は所持品復元の入力。
type RestoreItemParams struct {
	UserID          domainauth.UserID
	PublicID        uuid.UUID
	ExpectedVersion int32
}

// RestoreItemResult は所持品復元の結果。
type RestoreItemResult struct {
	Item ItemResult
}

// RestoreItemService はarchive済みの所持品を復元する。
type RestoreItemService struct {
	dependencies       Dependencies
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewRestoreItemService はRestoreItemServiceを生成する。
func NewRestoreItemService(
	dependencies Dependencies,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *RestoreItemService {
	return &RestoreItemService{
		dependencies:       dependencies,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute はarchive済みの所持品を復元する。
func (s *RestoreItemService) Execute(
	ctx context.Context,
	params RestoreItemParams,
) (RestoreItemResult, error) {
	var restored domainitem.Item

	err := s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		existing, err := s.dependencies.Items.FindByPublicID(ctx, params.UserID, params.PublicID)
		if err != nil {
			return err
		}

		now := s.clock.Now()
		// archive状態の確認とversion検証をDomainで行う。
		if _, err := existing.Restore(params.ExpectedVersion, now); err != nil {
			return err
		}

		restored, err = s.dependencies.Items.Restore(
			ctx, params.UserID, params.PublicID, params.ExpectedVersion, now)
		if err != nil {
			return err
		}

		targetPublicID := restored.PublicID()
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionItemRestored,
			domainaudit.TargetTypeItem,
			&targetPublicID,
			domainaudit.Changes{
				"isArchived": domainaudit.FieldChange{From: true, To: false},
			},
		)
	})
	if err != nil {
		return RestoreItemResult{}, err
	}

	result, err := s.dependencies.newItemResultWithStorage(ctx, params.UserID, restored)
	if err != nil {
		return RestoreItemResult{}, err
	}
	return RestoreItemResult{Item: result}, nil
}
