package item

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
)

// ItemRepository はItem Aggregateの永続化を担う (設計書 6.5 / 11.6)。
//
// 実装は全てのqueryで user internal ID を条件へ含める (設計書 18.3)。
// 更新系は version をWHERE条件へ含め、更新件数0を競合として扱う (設計書 11.7)。
type ItemRepository interface {
	// Create はアイテムとタグ付与を作成し、内部IDを付与したItemを返す。
	Create(ctx context.Context, item Item) (Item, error)

	// FindByPublicID はアイテムを取得する。archive済みも返す。
	// 存在しない場合、および他ユーザーのアイテムを指定した場合は ErrItemNotFound を返す。
	FindByPublicID(ctx context.Context, userID auth.UserID, publicID uuid.UUID) (Item, error)

	// Update は属性とタグ付与を置き換える。
	// versionが一致しない場合は ErrItemVersionConflict を返す。
	Update(ctx context.Context, item Item, expectedVersion int32) (Item, error)

	// Archive はarchive (soft delete) する。
	Archive(
		ctx context.Context,
		userID auth.UserID,
		publicID uuid.UUID,
		expectedVersion int32,
		archivedAt time.Time,
	) (Item, error)

	// Restore はarchiveを解除する。
	Restore(
		ctx context.Context,
		userID auth.UserID,
		publicID uuid.UUID,
		expectedVersion int32,
		now time.Time,
	) (Item, error)

	// List は条件に一致するアイテムを返す。
	List(ctx context.Context, userID auth.UserID, criteria ListCriteria) ([]Item, error)

	// Count は条件に一致するアイテムの総件数を返す。
	Count(ctx context.Context, userID auth.UserID, criteria ListCriteria) (int64, error)

	// AggregateSummary はダッシュボード向けの集計値を返す (設計書 9.3)。
	//
	// archive済みのアイテムは集計へ含めない。
	// 内訳の並びはDomainが決めるため、実装は集計結果をそのまま返し
	// 表示順の整列は行わない。
	AggregateSummary(ctx context.Context, userID auth.UserID) (SummaryTotals, error)
}
