// Package tag はタグのユースケースを実装する。
//
// 1操作あたりの手順が短いため、CRUDの4ユースケースを1fileへまとめる。
// Application Serviceはユースケース単位のtypeとして分離する (設計書 11.4)。
package tag

import (
	"context"
	"time"

	"github.com/google/uuid"

	applicationaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/audit"
	domainaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	domaintag "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/tag"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/idgenerator"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/transaction"
)

// Dependencies はタグユースケースが必要とする依存をまとめる。
type Dependencies struct {
	Tags          domaintag.TagRepository
	AuditRecorder *applicationaudit.Recorder
}

// TagResult はユースケースが返すタグの表現。内部IDを含めない。
type TagResult struct {
	PublicID  uuid.UUID
	Name      string
	ItemCount int64
	Version   int32
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ---------------------------------------------------------------------------
// 一覧
// ---------------------------------------------------------------------------

// ListTagsParams はタグ一覧取得の入力。
type ListTagsParams struct {
	UserID domainauth.UserID
}

// ListTagsResult はタグ一覧の結果。
type ListTagsResult struct {
	Tags []TagResult
}

// ListTagsService はタグ一覧を取得する。
type ListTagsService struct {
	dependencies Dependencies
}

// NewListTagsService はListTagsServiceを生成する。
func NewListTagsService(dependencies Dependencies) *ListTagsService {
	return &ListTagsService{dependencies: dependencies}
}

// Execute は認証ユーザーのタグを名称昇順で返す。
func (s *ListTagsService) Execute(
	ctx context.Context,
	params ListTagsParams,
) (ListTagsResult, error) {
	summaries, err := s.dependencies.Tags.ListActiveWithItemCount(ctx, params.UserID)
	if err != nil {
		return ListTagsResult{}, err
	}

	results := make([]TagResult, 0, len(summaries))
	for _, summary := range summaries {
		results = append(results, newTagResult(summary.Tag, summary.ItemCount))
	}
	return ListTagsResult{Tags: results}, nil
}

// ---------------------------------------------------------------------------
// 登録
// ---------------------------------------------------------------------------

// CreateTagParams はタグ登録の入力。
type CreateTagParams struct {
	UserID domainauth.UserID
	Name   string
}

// CreateTagResult はタグ登録の結果。
type CreateTagResult struct {
	Tag TagResult
}

// CreateTagService はタグを登録する。
type CreateTagService struct {
	dependencies       Dependencies
	publicIDGenerator  idgenerator.PublicIDGenerator
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewCreateTagService はCreateTagServiceを生成する。
func NewCreateTagService(
	dependencies Dependencies,
	publicIDGenerator idgenerator.PublicIDGenerator,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *CreateTagService {
	return &CreateTagService{
		dependencies:       dependencies,
		publicIDGenerator:  publicIDGenerator,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute はタグを登録する。
func (s *CreateTagService) Execute(
	ctx context.Context,
	params CreateTagParams,
) (CreateTagResult, error) {
	publicID, err := s.publicIDGenerator.NewPublicID()
	if err != nil {
		return CreateTagResult{}, shared.NewInternalError(
			"PUBLIC_ID_GENERATION_FAILED", "サーバーでエラーが発生しました。").WithCause(err)
	}

	var created domaintag.Tag
	err = s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		newTag, err := domaintag.NewTag(publicID, params.UserID, params.Name, s.clock.Now())
		if err != nil {
			return err
		}

		created, err = s.dependencies.Tags.Create(ctx, newTag)
		if err != nil {
			return err
		}

		targetPublicID := created.PublicID()
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionTagCreated,
			domainaudit.TargetTypeTag,
			&targetPublicID,
			domainaudit.Changes{
				"name": domainaudit.FieldChange{From: nil, To: created.Name()},
			},
		)
	})
	if err != nil {
		return CreateTagResult{}, err
	}

	// 登録直後は付与件数0で確定する。
	return CreateTagResult{Tag: newTagResult(created, 0)}, nil
}

// ---------------------------------------------------------------------------
// 更新
// ---------------------------------------------------------------------------

// UpdateTagParams はタグ更新の入力。
type UpdateTagParams struct {
	UserID          domainauth.UserID
	PublicID        uuid.UUID
	Name            string
	ExpectedVersion int32
}

// UpdateTagResult はタグ更新の結果。
type UpdateTagResult struct {
	Tag TagResult
}

// UpdateTagService はタグ名を更新する。
type UpdateTagService struct {
	dependencies       Dependencies
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewUpdateTagService はUpdateTagServiceを生成する。
func NewUpdateTagService(
	dependencies Dependencies,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *UpdateTagService {
	return &UpdateTagService{
		dependencies:       dependencies,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute はタグ名を更新する。
func (s *UpdateTagService) Execute(
	ctx context.Context,
	params UpdateTagParams,
) (UpdateTagResult, error) {
	var (
		updated   domaintag.Tag
		itemCount int64
	)

	err := s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		existing, err := s.dependencies.Tags.FindActiveByPublicID(
			ctx, params.UserID, params.PublicID)
		if err != nil {
			return err
		}
		previousName := existing.Name()

		renamed, err := existing.Rename(params.Name, params.ExpectedVersion, s.clock.Now())
		if err != nil {
			return err
		}

		updated, err = s.dependencies.Tags.Update(ctx, renamed, params.ExpectedVersion)
		if err != nil {
			return err
		}

		itemCount, err = s.dependencies.Tags.CountActiveItems(ctx, params.UserID, updated.ID())
		if err != nil {
			return err
		}

		if previousName == updated.Name() {
			return nil
		}

		targetPublicID := updated.PublicID()
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionTagUpdated,
			domainaudit.TargetTypeTag,
			&targetPublicID,
			domainaudit.Changes{
				"name": domainaudit.FieldChange{From: previousName, To: updated.Name()},
			},
		)
	})
	if err != nil {
		return UpdateTagResult{}, err
	}

	return UpdateTagResult{Tag: newTagResult(updated, itemCount)}, nil
}

// ---------------------------------------------------------------------------
// 削除
// ---------------------------------------------------------------------------

// DeleteTagParams はタグ削除の入力。
type DeleteTagParams struct {
	UserID          domainauth.UserID
	PublicID        uuid.UUID
	ExpectedVersion int32
}

// DeleteTagService はタグを削除する。
type DeleteTagService struct {
	dependencies       Dependencies
	clock              clock.Clock
	transactionManager transaction.Manager
}

// NewDeleteTagService はDeleteTagServiceを生成する。
func NewDeleteTagService(
	dependencies Dependencies,
	systemClock clock.Clock,
	transactionManager transaction.Manager,
) *DeleteTagService {
	return &DeleteTagService{
		dependencies:       dependencies,
		clock:              systemClock,
		transactionManager: transactionManager,
	}
}

// Execute はタグをsoft deleteする (設計書 1.4)。
//
// アイテムへの付与情報は保持したまま、一覧とアイテムresponseから除外する。
func (s *DeleteTagService) Execute(ctx context.Context, params DeleteTagParams) error {
	return s.transactionManager.WithinTransaction(ctx, func(ctx context.Context) error {
		existing, err := s.dependencies.Tags.FindActiveByPublicID(
			ctx, params.UserID, params.PublicID)
		if err != nil {
			return err
		}
		if err := existing.EnsureVersionMatches(params.ExpectedVersion); err != nil {
			return err
		}

		now := s.clock.Now()
		if err := s.dependencies.Tags.SoftDelete(
			ctx, params.UserID, params.PublicID, params.ExpectedVersion, now); err != nil {
			return err
		}

		targetPublicID := existing.PublicID()
		return s.dependencies.AuditRecorder.Record(
			ctx,
			params.UserID,
			domainaudit.ActionTagDeleted,
			domainaudit.TargetTypeTag,
			&targetPublicID,
			domainaudit.Changes{
				"name": domainaudit.FieldChange{From: existing.Name(), To: nil},
			},
		)
	})
}

func newTagResult(source domaintag.Tag, itemCount int64) TagResult {
	return TagResult{
		PublicID:  source.PublicID(),
		Name:      source.Name(),
		ItemCount: itemCount,
		Version:   source.Version(),
		CreatedAt: source.CreatedAt(),
		UpdatedAt: source.UpdatedAt(),
	}
}
