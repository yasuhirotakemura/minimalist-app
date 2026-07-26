package tag

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// TagRepository はTag Aggregateの永続化を担う (設計書 6.5 / 11.6)。
//
// 実装は全てのqueryで user internal ID と deleted_at IS NULL を条件へ含める。
type TagRepository interface {
	// Create はタグを作成し、内部IDを付与したTagを返す。
	// 同名のタグが既に存在する場合は ErrTagNameAlreadyUsed を返す。
	Create(ctx context.Context, tag Tag) (Tag, error)

	// ListActiveWithItemCount は有効なタグを名称昇順で取得する。
	ListActiveWithItemCount(ctx context.Context, userID auth.UserID) ([]Summary, error)

	// FindActiveByPublicID は有効なタグを取得する。
	// 存在しない場合、および他ユーザーのタグを指定した場合は ErrTagNotFound を返す。
	FindActiveByPublicID(
		ctx context.Context,
		userID auth.UserID,
		publicID uuid.UUID,
	) (Tag, error)

	// ResolveActiveReferences は指定した公開IDのタグ参照を解決する。
	// 1件でも解決できない場合は ErrTagNotFound を返す。
	ResolveActiveReferences(
		ctx context.Context,
		userID auth.UserID,
		publicIDs []uuid.UUID,
	) ([]Reference, error)

	// Update は名称を更新する。versionが一致しない場合は ErrTagVersionConflict を返す。
	Update(ctx context.Context, tag Tag, expectedVersion int32) (Tag, error)

	// SoftDelete はタグをsoft deleteする (設計書 1.4)。
	// アイテムへの付与情報は保持し、一覧・アイテムresponseから除外する。
	SoftDelete(
		ctx context.Context,
		userID auth.UserID,
		publicID uuid.UUID,
		expectedVersion int32,
		deletedAt time.Time,
	) error

	// CountActiveItems はタグが付与されているarchive前アイテムの件数を返す。
	CountActiveItems(ctx context.Context, userID auth.UserID, id TagID) (int64, error)
}

// tag aggregateのDomain error (設計書 19.1)。
var (
	// ErrTagNotFound はタグが存在しないことを表す。
	// 他ユーザーのpublicIdを指定した場合も本errorを返す (設計書 18.3)。
	ErrTagNotFound = shared.NewNotFoundError(
		"TAG_NOT_FOUND",
		"タグが見つかりません。",
	)

	// ErrTagNameAlreadyUsed は同名のタグが既に存在することを表す。
	ErrTagNameAlreadyUsed = shared.NewConflictError(
		"TAG_NAME_ALREADY_USED",
		"同じ名前のタグが既に登録されています。",
	)

	// ErrTagVersionConflict は楽観ロックの競合を表す。
	ErrTagVersionConflict = shared.NewConflictError(
		"TAG_VERSION_CONFLICT",
		"タグが別の操作で更新されています。最新の内容を読み込み直してください。",
	)
)
