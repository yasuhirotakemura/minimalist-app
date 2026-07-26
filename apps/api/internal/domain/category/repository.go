package category

import (
	"context"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// CategoryRepository はCategory Aggregateの永続化を担う (設計書 6.5 / 11.6)。
//
// 実装は全てのqueryで user internal ID と deleted_at IS NULL を条件へ含める。
type CategoryRepository interface {
	// CreateAll はカテゴリーをまとめて作成し、内部IDを付与した結果を返す。
	// ユーザー登録時の既定カテゴリー作成で使用する。
	CreateAll(ctx context.Context, categories []Category) ([]Category, error)

	// ListActiveByUserID は有効なカテゴリーを表示順で取得する。
	ListActiveByUserID(ctx context.Context, userID auth.UserID) ([]Category, error)

	// FindActiveByPublicID は有効なカテゴリーを取得する。
	// 存在しない場合、および他ユーザーのカテゴリーを指定した場合は
	// ErrCategoryNotFound を返す (設計書 18.3)。
	FindActiveByPublicID(
		ctx context.Context,
		userID auth.UserID,
		publicID uuid.UUID,
	) (Category, error)
}

// ErrCategoryNotFound はカテゴリーが存在しないことを表す。
var ErrCategoryNotFound = shared.NewNotFoundError(
	"CATEGORY_NOT_FOUND",
	"カテゴリーが見つかりません。",
)
