package storage

import (
	"strings"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// 一覧のpagination既定値 (OpenAPIの limit / offset と一致させる)。
const (
	DefaultListLimit int32 = 50
	MaxListLimit     int32 = 100
	MaxKeywordLength       = 100
)

// SortKey は一覧の並び替えkey。
//
// DBのcolumn名ではなくAPIのkey名を値とし、column名への対応付けは
// infrastructure layerが行う (設計書 3.3)。
//
// 総重量はDBへ保存しない集計値のためsort keyへ含めない。
type SortKey string

// SortKeyの値。
const (
	SortKeyName      SortKey = "name"
	SortKeySortOrder SortKey = "sortOrder"
	SortKeyUpdatedAt SortKey = "updatedAt"
)

// DefaultSortKey は未指定時の並び替えkey。
// 収納単位は利用者が並び順を決める前提のため sortOrder を既定とする。
const DefaultSortKey = SortKeySortOrder

var sortKeys = map[SortKey]struct{}{
	SortKeyName:      {},
	SortKeySortOrder: {},
	SortKeyUpdatedAt: {},
}

// String はkey名を返す。
func (k SortKey) String() string { return string(k) }

// ListCriteria は収納単位一覧の検索・絞り込み・並び替え条件。
type ListCriteria struct {
	// Keyword は収納単位名または説明の部分一致条件。空文字は条件なし。
	Keyword       string
	StorageType   *StorageType
	MobilityClass *item.MobilityClass
	// ParentPublicID は指定した収納単位の直接の子だけを返す条件。
	ParentPublicID *uuid.UUID
	// RootOnly は親を持たない収納単位だけを返す条件。
	RootOnly bool
	// IncludeArchived はarchive済みを含めるか。既定はfalse。
	IncludeArchived bool
	SortKey         SortKey
	Descending      bool
	Limit           int32
	Offset          int32
}

// ListCriteriaInput は正規化前の一覧条件。presentation layerからそのまま渡す。
type ListCriteriaInput struct {
	Keyword           string
	StorageTypeCode   string
	MobilityClassCode string
	ParentPublicID    *uuid.UUID
	RootOnly          bool
	IncludeArchived   bool
	SortKeyName       string
	Order             string
	Limit             *int32
	Offset            *int32
}

// NewListCriteria は入力を検証し、既定値を適用した条件を返す。
//
// 既定値: sort=sortOrder、order=asc、limit=50、offset=0。
// 所持品一覧 (sort=updatedAt / order=desc) と既定が異なるのは、
// 収納単位が利用者の指定した表示順を主たる並びとするためである。
func NewListCriteria(input ListCriteriaInput) (ListCriteria, error) {
	criteria := ListCriteria{
		ParentPublicID:  input.ParentPublicID,
		RootOnly:        input.RootOnly,
		IncludeArchived: input.IncludeArchived,
	}

	// 「親を持たない」と「特定の親の子」は同時に満たせない条件のため、
	// 空の結果を返さず入力errorとして知らせる。
	if input.RootOnly && input.ParentPublicID != nil {
		return ListCriteria{}, newCriteriaError(
			"rootOnly", "rootOnlyとparentStorageUnitPublicIdは同時に指定できません。")
	}

	keyword := strings.TrimSpace(input.Keyword)
	if len([]rune(keyword)) > MaxKeywordLength {
		return ListCriteria{}, newCriteriaError(
			"keyword", "検索キーワードは100文字以内で入力してください。")
	}
	criteria.Keyword = keyword

	if code := strings.TrimSpace(input.StorageTypeCode); code != "" {
		storageType, err := NewStorageType(code)
		if err != nil {
			return ListCriteria{}, err
		}
		criteria.StorageType = &storageType
	}
	if code := strings.TrimSpace(input.MobilityClassCode); code != "" {
		mobilityClass, err := item.NewMobilityClass(code)
		if err != nil {
			return ListCriteria{}, err
		}
		criteria.MobilityClass = &mobilityClass
	}

	sortKey := SortKey(strings.TrimSpace(input.SortKeyName))
	if sortKey == "" {
		sortKey = DefaultSortKey
	}
	if _, ok := sortKeys[sortKey]; !ok {
		return ListCriteria{}, newCriteriaError("sort", "並び替えの指定が正しくありません。")
	}
	criteria.SortKey = sortKey

	switch strings.TrimSpace(strings.ToLower(input.Order)) {
	case "", "asc":
		criteria.Descending = false
	case "desc":
		criteria.Descending = true
	default:
		return ListCriteria{}, newCriteriaError("order", "並び順の指定が正しくありません。")
	}

	criteria.Limit = DefaultListLimit
	if input.Limit != nil {
		if *input.Limit < 1 || *input.Limit > MaxListLimit {
			return ListCriteria{}, newCriteriaError(
				"limit", "取得件数は1以上100以下で指定してください。")
		}
		criteria.Limit = *input.Limit
	}

	if input.Offset != nil {
		if *input.Offset < 0 {
			return ListCriteria{}, newCriteriaError("offset", "offsetは0以上で指定してください。")
		}
		criteria.Offset = *input.Offset
	}

	return criteria, nil
}

func newCriteriaError(field, message string) *shared.DomainError {
	return shared.NewInvalidInputError("INVALID_LIST_CRITERIA", message).
		WithFieldErrors(shared.NewFieldError(field, "INVALID_VALUE", message))
}
