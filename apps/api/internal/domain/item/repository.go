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

	// TouchLastUsedAt は最終使用日時を更新する。
	// 既存値より古い使用日時では最終使用日時を後退させない。
	TouchLastUsedAt(
		ctx context.Context,
		userID auth.UserID,
		publicID uuid.UUID,
		usedAt time.Time,
		now time.Time,
	) (Item, error)

	// List は条件に一致するアイテムを返す。
	List(ctx context.Context, userID auth.UserID, criteria ListCriteria) ([]Item, error)

	// Count は条件に一致するアイテムの総件数を返す。
	Count(ctx context.Context, userID auth.UserID, criteria ListCriteria) (int64, error)
}

// ItemUsageRecordRepository は使用記録の永続化を担う。
//
// 使用記録はItem Aggregateに属するが、追記と履歴取得のみを行い
// Itemとは別のqueryとなるため、interfaceを分離する。
type ItemUsageRecordRepository interface {
	// Create は使用記録を作成し、内部IDを付与したUsageRecordを返す。
	Create(ctx context.Context, record UsageRecord) (UsageRecord, error)

	// ListByItemID は使用日時の降順で履歴を返す。
	ListByItemID(
		ctx context.Context,
		userID auth.UserID,
		itemID ItemID,
		page PageCriteria,
	) ([]UsageRecord, error)

	// CountByItemID は履歴の総件数を返す。
	CountByItemID(ctx context.Context, userID auth.UserID, itemID ItemID) (int64, error)
}
