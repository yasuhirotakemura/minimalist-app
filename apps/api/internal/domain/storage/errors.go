package storage

import "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"

// storage aggregateのDomain error (設計書 19.1)。
// 呼び出し側は errors.Is で判定する。DomainError.Is はCodeで一致判定する。
var (
	// ErrStorageUnitNotFound は収納単位が存在しないことを表す。
	// 他ユーザーのpublicIdを指定した場合も本errorを返し、存在有無を公開しない (設計書 18.3)。
	ErrStorageUnitNotFound = shared.NewNotFoundError(
		"STORAGE_UNIT_NOT_FOUND",
		"収納単位が見つかりません。",
	)

	// ErrStorageUnitVersionConflict は収納単位の楽観ロック競合を表す (設計書 11.7)。
	ErrStorageUnitVersionConflict = shared.NewConflictError(
		"STORAGE_UNIT_VERSION_CONFLICT",
		"収納単位が別の操作で更新されています。最新の内容を読み込み直してください。",
	)

	// ErrStorageUnitArchived はarchive済みの収納単位を操作しようとしたことを表す。
	ErrStorageUnitArchived = shared.NewRuleViolationError(
		"STORAGE_UNIT_ARCHIVED",
		"アーカイブ済みの収納単位は操作できません。復元してから操作してください。",
	)

	// ErrStorageUnitAlreadyArchived は既にarchive済みであることを表す。
	ErrStorageUnitAlreadyArchived = shared.NewRuleViolationError(
		"STORAGE_UNIT_ALREADY_ARCHIVED",
		"この収納単位は既にアーカイブされています。",
	)

	// ErrStorageUnitNotArchived はarchiveされていない収納単位を復元しようとしたことを表す。
	ErrStorageUnitNotArchived = shared.NewRuleViolationError(
		"STORAGE_UNIT_NOT_ARCHIVED",
		"この収納単位はアーカイブされていません。",
	)

	// ErrStorageUnitHasChildren は子収納単位が残ったままarchiveしようとしたことを表す。
	//
	// 親のarchiveで子を暗黙にarchiveしない。復元時の状態を予測可能に保つため、
	// 利用者へ子を先にarchiveすることを求める。
	ErrStorageUnitHasChildren = shared.NewRuleViolationError(
		"STORAGE_UNIT_HAS_CHILDREN",
		"子の収納単位が残っています。先に子の収納単位をアーカイブしてください。",
	)

	// ErrStorageUnitHasAllocations は収納割当が残ったままarchiveしようとしたことを表す。
	ErrStorageUnitHasAllocations = shared.NewRuleViolationError(
		"STORAGE_UNIT_HAS_ALLOCATIONS",
		"収納されているアイテムが残っています。先に中身を取り出してください。",
	)

	// ErrStorageUnitSelfParent は自分自身を親に指定したことを表す (設計書 7.3)。
	ErrStorageUnitSelfParent = shared.NewRuleViolationError(
		"STORAGE_UNIT_SELF_PARENT",
		"収納単位自身を親に指定できません。",
	)

	// ErrStorageUnitCircularParent は子孫を親に指定したことを表す。
	ErrStorageUnitCircularParent = shared.NewRuleViolationError(
		"STORAGE_UNIT_CIRCULAR_PARENT",
		"配下の収納単位を親に指定できません。",
	)

	// ErrStorageUnitParentArchived はarchive済みの収納単位を親に指定したことを表す。
	ErrStorageUnitParentArchived = shared.NewRuleViolationError(
		"STORAGE_UNIT_PARENT_ARCHIVED",
		"アーカイブ済みの収納単位を親に指定できません。",
	)

	// ErrStorageHierarchyTooDeep は階層上限を超えたことを表す (設計書 7.3)。
	ErrStorageHierarchyTooDeep = shared.NewRuleViolationError(
		"STORAGE_HIERARCHY_TOO_DEEP",
		"収納単位の階層は3段までです。",
	)

	// ErrStorageAllocationNotFound は収納割当が存在しないことを表す。
	ErrStorageAllocationNotFound = shared.NewNotFoundError(
		"STORAGE_ALLOCATION_NOT_FOUND",
		"収納割当が見つかりません。",
	)

	// ErrStorageAllocationVersionConflict は収納割当の楽観ロック競合を表す。
	ErrStorageAllocationVersionConflict = shared.NewConflictError(
		"STORAGE_ALLOCATION_VERSION_CONFLICT",
		"収納割当が別の操作で更新されています。最新の内容を読み込み直してください。",
	)

	// ErrStorageAllocationAlreadyExists は同一収納単位・同一アイテムの重複割当を表す。
	//
	// 同一アイテムを同じ収納単位へ2行に分けても意味が無いため、
	// 数量の変更として扱わせる (設計書 13.9)。
	ErrStorageAllocationAlreadyExists = shared.NewConflictError(
		"STORAGE_ALLOCATION_ALREADY_EXISTS",
		"このアイテムは既にこの収納単位へ割り当てられています。数量を変更してください。",
	)

	// ErrStorageAllocationDuplicatedItem は一括置換の入力へ同一アイテムが複数含まれることを表す。
	ErrStorageAllocationDuplicatedItem = shared.NewInvalidInputError(
		"STORAGE_ALLOCATION_DUPLICATED_ITEM",
		"同じアイテムを複数指定できません。数量をまとめて指定してください。",
	)

	// ErrStorageAllocationExceedsQuantity は割当数量合計が所有数量を超えたことを表す
	// (設計書 7.2 / 13.9 / 19.1)。
	ErrStorageAllocationExceedsQuantity = shared.NewRuleViolationError(
		"STORAGE_ALLOCATION_EXCEEDS_QUANTITY",
		"収納割当の合計数量が所有数量を超えています。",
	)

	// ErrStorageAllocationItemArchived はarchive済みアイテムを新規割当しようとしたことを表す。
	//
	// 既存の割当は保持する。archiveは手放しではなく、物理的には収納単位へ
	// 入ったままであるため取り出しを強制しない。
	ErrStorageAllocationItemArchived = shared.NewRuleViolationError(
		"STORAGE_ALLOCATION_ITEM_ARCHIVED",
		"アーカイブ済みのアイテムは新しく割り当てられません。",
	)
)

// newAttributeError は入力項目単位のerrorを生成する。
//
// fieldはrequest bodyのfield名 (camelCase) と一致させ、
// 画面が該当入力欄へerrorを表示できるようにする (設計書 10.6 / 12.3)。
func newAttributeError(field, message string) *shared.DomainError {
	return shared.NewInvalidInputError("INVALID_STORAGE_UNIT", message).
		WithFieldErrors(shared.NewFieldError(field, "INVALID_VALUE", message))
}
