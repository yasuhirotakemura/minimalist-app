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

// ArchiveItemParams は所持品archiveの入力。
type ArchiveItemParams struct {
	UserID          domainauth.UserID
	PublicID        uuid.UUID
	ExpectedVersion int32
}

// ArchiveItemResult は所持品archiveの結果。
type ArchiveItemResult struct {
	Item ItemResult
}

// ArchiveItemService は所持品をarchive (soft delete) する。
type ArchiveItemService struct {
	dependencies       Dependencies
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewArchiveItemService はArchiveItemServiceを生成する。
func NewArchiveItemService(
	dependencies Dependencies,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *ArchiveItemService {
	return &ArchiveItemService{
		dependencies:       dependencies,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute は所持品をarchiveする。
func (s *ArchiveItemService) Execute(
	ctx context.Context,
	params ArchiveItemParams,
) (ArchiveItemResult, error) {
	var archived domainitem.Item

	err := s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		existing, err := s.dependencies.Items.FindByPublicID(ctx, params.UserID, params.PublicID)
		if err != nil {
			return err
		}

		now := s.clock.Now()
		// archive済み判定とversion検証をDomainで行う。
		if _, err := existing.Archive(params.ExpectedVersion, now); err != nil {
			return err
		}

		archived, err = s.dependencies.Items.Archive(
			ctx, params.UserID, params.PublicID, params.ExpectedVersion, now)
		if err != nil {
			return err
		}

		targetPublicID := archived.PublicID()
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionItemArchived,
			domainaudit.TargetTypeItem,
			&targetPublicID,
			domainaudit.Changes{
				"isArchived": domainaudit.FieldChange{From: false, To: true},
			},
		)
	})
	if err != nil {
		return ArchiveItemResult{}, err
	}

	result, err := s.dependencies.newItemResultWithStorage(ctx, params.UserID, archived)
	if err != nil {
		return ArchiveItemResult{}, err
	}
	return ArchiveItemResult{Item: result}, nil
}
