package item

import "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"

// item aggregateのDomain error (設計書 19.1)。
// 呼び出し側は errors.Is で判定する。DomainError.Is はCodeで一致判定する。
var (
	// ErrItemNotFound はアイテムが存在しないことを表す。
	// 他ユーザーのpublicIdを指定した場合も本errorを返し、存在有無を公開しない (設計書 18.3)。
	ErrItemNotFound = shared.NewNotFoundError(
		"ITEM_NOT_FOUND",
		"アイテムが見つかりません。",
	)

	// ErrItemVersionConflict は楽観ロックの競合を表す (設計書 11.7 / 12.3)。
	ErrItemVersionConflict = shared.NewConflictError(
		"ITEM_VERSION_CONFLICT",
		"アイテムが別の操作で更新されています。最新の内容を読み込み直してください。",
	)

	// ErrItemArchived はarchive済みのアイテムへ操作しようとしたことを表す。
	ErrItemArchived = shared.NewRuleViolationError(
		"ITEM_ARCHIVED",
		"アーカイブ済みのアイテムは操作できません。復元してから操作してください。",
	)

	// ErrItemAlreadyArchived は既にarchive済みであることを表す。
	ErrItemAlreadyArchived = shared.NewRuleViolationError(
		"ITEM_ALREADY_ARCHIVED",
		"このアイテムは既にアーカイブされています。",
	)

	// ErrItemNotArchived はarchiveされていないアイテムを復元しようとしたことを表す。
	ErrItemNotArchived = shared.NewRuleViolationError(
		"ITEM_NOT_ARCHIVED",
		"このアイテムはアーカイブされていません。",
	)
)
