// Package audit は操作履歴の記録をApplication Serviceへ提供する (設計書 11.4 / 22章)。
//
// 監査ログの記録はApplication Serviceの責務であり、複数のユースケースから
// 同じ手順で呼び出されるため、publicIDの採番と時刻取得を本packageへ集約する。
package audit

import (
	"context"

	"github.com/google/uuid"

	domainaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/idgenerator"
)

// Recorder は操作履歴を記録する。
//
// 業務操作と同一transaction内で呼び出す。記録に失敗した場合はerrorを返し、
// 呼び出し元のtransactionをrollbackさせる (設計書 20.1)。
type Recorder struct {
	logs              domainaudit.AuditLogRepository
	publicIDGenerator idgenerator.PublicIDGenerator
	clock             clock.Clock
}

// NewRecorder はRecorderを生成する。
func NewRecorder(
	logs domainaudit.AuditLogRepository,
	publicIDGenerator idgenerator.PublicIDGenerator,
	systemClock clock.Clock,
) *Recorder {
	return &Recorder{
		logs:              logs,
		publicIDGenerator: publicIDGenerator,
		clock:             systemClock,
	}
}

// Record は操作履歴を1件記録する。
func (r *Recorder) Record(
	ctx context.Context,
	userID domainauth.UserID,
	action domainaudit.ActionCode,
	targetType domainaudit.TargetTypeCode,
	targetPublicID *uuid.UUID,
	changes domainaudit.Changes,
) error {
	publicID, err := r.publicIDGenerator.NewPublicID()
	if err != nil {
		return shared.NewInternalError(
			"PUBLIC_ID_GENERATION_FAILED", "サーバーでエラーが発生しました。").WithCause(err)
	}

	log, err := domainaudit.NewAuditLog(
		publicID, userID, action, targetType, targetPublicID, changes, r.clock.Now())
	if err != nil {
		return err
	}

	return r.logs.Create(ctx, log)
}
