package item

import (
	"context"

	domainaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/idgenerator"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/transaction"
)

// CreateItemParams は所持品登録の入力。
type CreateItemParams struct {
	UserID     domainauth.UserID
	Attributes AttributesParams
}

// CreateItemResult は所持品登録の結果。
type CreateItemResult struct {
	Item ItemResult
}

// CreateItemService は所持品を登録する。
type CreateItemService struct {
	dependencies       Dependencies
	publicIDGenerator  idgenerator.PublicIDGenerator
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewCreateItemService はCreateItemServiceを生成する。
func NewCreateItemService(
	dependencies Dependencies,
	publicIDGenerator idgenerator.PublicIDGenerator,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *CreateItemService {
	return &CreateItemService{
		dependencies:       dependencies,
		publicIDGenerator:  publicIDGenerator,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute は所持品を登録する。
//
// カテゴリー・タグの解決、アイテム作成、タグ付与、監査ログ記録を
// 単一transactionで実行する (設計書 20章)。
func (s *CreateItemService) Execute(
	ctx context.Context,
	params CreateItemParams,
) (CreateItemResult, error) {
	publicID, err := s.publicIDGenerator.NewPublicID()
	if err != nil {
		return CreateItemResult{}, shared.NewInternalError(
			"PUBLIC_ID_GENERATION_FAILED", "サーバーでエラーが発生しました。").WithCause(err)
	}

	var created domainitem.Item
	err = s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		attributes, err := s.dependencies.resolveAttributes(ctx, params.UserID, params.Attributes)
		if err != nil {
			return err
		}

		newItem, err := domainitem.NewItem(publicID, params.UserID, attributes, s.clock.Now())
		if err != nil {
			return err
		}

		created, err = s.dependencies.Items.Create(ctx, newItem)
		if err != nil {
			return err
		}

		targetPublicID := created.PublicID()
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionItemCreated,
			domainaudit.TargetTypeItem,
			&targetPublicID,
			domainaudit.Diff(nil, created.AuditSnapshot()),
		)
	})
	if err != nil {
		return CreateItemResult{}, err
	}

	return CreateItemResult{Item: newItemResult(created)}, nil
}
