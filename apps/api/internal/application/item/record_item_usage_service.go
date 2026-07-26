package item

import (
	"context"
	"time"

	"github.com/google/uuid"

	domainaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/idgenerator"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/transaction"
)

// RecordItemUsageParams は使用記録登録の入力。
type RecordItemUsageParams struct {
	UserID   domainauth.UserID
	PublicID uuid.UUID
	// UsedAt が未指定の場合はserverの現在時刻を使用する。
	UsedAt   *time.Time
	Quantity *int32
	Note     *string
}

// RecordItemUsageResult は使用記録登録の結果。
type RecordItemUsageResult struct {
	UsageRecord UsageRecordResult
	Item        ItemResult
}

// RecordItemUsageService は使用記録を登録する。
type RecordItemUsageService struct {
	dependencies       Dependencies
	publicIDGenerator  idgenerator.PublicIDGenerator
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewRecordItemUsageService はRecordItemUsageServiceを生成する。
func NewRecordItemUsageService(
	dependencies Dependencies,
	publicIDGenerator idgenerator.PublicIDGenerator,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *RecordItemUsageService {
	return &RecordItemUsageService{
		dependencies:       dependencies,
		publicIDGenerator:  publicIDGenerator,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute は使用記録を登録し、アイテムの最終使用日時を更新する。
//
// 使用記録の作成と最終使用日時の更新は同一transactionで実行する。
// archive済みのアイテムへは登録できない (設計書 7.2)。
func (s *RecordItemUsageService) Execute(
	ctx context.Context,
	params RecordItemUsageParams,
) (RecordItemUsageResult, error) {
	recordPublicID, err := s.publicIDGenerator.NewPublicID()
	if err != nil {
		return RecordItemUsageResult{}, shared.NewInternalError(
			"PUBLIC_ID_GENERATION_FAILED", "サーバーでエラーが発生しました。").WithCause(err)
	}

	var (
		createdRecord domainitem.UsageRecord
		updatedItem   domainitem.Item
	)

	err = s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		existing, err := s.dependencies.Items.FindByPublicID(ctx, params.UserID, params.PublicID)
		if err != nil {
			return err
		}

		now := s.clock.Now()
		usedAt := now
		if params.UsedAt != nil {
			usedAt = *params.UsedAt
		}
		quantity := domainitem.DefaultUsageQuantity
		if params.Quantity != nil {
			quantity = *params.Quantity
		}

		record, _, err := existing.RecordUsage(
			recordPublicID, usedAt, quantity, params.Note, now)
		if err != nil {
			return err
		}

		createdRecord, err = s.dependencies.UsageRecords.Create(ctx, record)
		if err != nil {
			return err
		}

		updatedItem, err = s.dependencies.Items.TouchLastUsedAt(
			ctx, params.UserID, params.PublicID, record.UsedAt(), now)
		if err != nil {
			return err
		}

		targetPublicID := updatedItem.PublicID()
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionItemUsageRecorded,
			domainaudit.TargetTypeItem,
			&targetPublicID,
			domainaudit.Changes{
				"lastUsedAt": domainaudit.FieldChange{
					From: formatOptionalTime(existing.LastUsedAt()),
					To:   formatOptionalTime(updatedItem.LastUsedAt()),
				},
			},
		)
	})
	if err != nil {
		return RecordItemUsageResult{}, err
	}

	return RecordItemUsageResult{
		UsageRecord: newUsageRecordResult(createdRecord),
		Item:        newItemResult(updatedItem),
	}, nil
}

func formatOptionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}
