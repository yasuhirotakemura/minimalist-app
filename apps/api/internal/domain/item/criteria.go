package item

import (
	"strings"

	"github.com/google/uuid"

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
type SortKey string

// SortKeyの値。
const (
	SortKeyName       SortKey = "name"
	SortKeyQuantity   SortKey = "quantity"
	SortKeyLastUsedAt SortKey = "lastUsedAt"
	SortKeyUpdatedAt  SortKey = "updatedAt"
)

// DefaultSortKey は未指定時の並び替えkey。
const DefaultSortKey = SortKeyUpdatedAt

var sortKeys = map[SortKey]struct{}{
	SortKeyName:       {},
	SortKeyQuantity:   {},
	SortKeyLastUsedAt: {},
	SortKeyUpdatedAt:  {},
}

// String はkey名を返す。
func (k SortKey) String() string { return string(k) }

// ListCriteria は一覧の検索・絞り込み・並び替え条件 (設計書 9.4)。
//
// 見直しスコアに関する条件 (reviewRankCode) はPhase 3のスコープのため含めない。
type ListCriteria struct {
	// Keyword はアイテム名またはメモの部分一致条件。空文字は条件なし。
	Keyword          string
	CategoryPublicID *uuid.UUID
	TagPublicID      *uuid.UUID
	NecessityLevel   *NecessityLevel
	UsageFrequency   *UsageFrequency
	MobilityClass    *MobilityClass
	// StorageUnitPublicID は指定した収納単位へ直接割当されているアイテムに
	// 絞り込む条件 (Phase 2)。子収納単位の内容は含めない。
	StorageUnitPublicID *uuid.UUID
	// IsUnassigned は未割当数量が1以上のアイテムに絞り込む条件 (Phase 2)。
	IsUnassigned bool
	// IncludeArchived はarchive済みを含めるか。既定はfalse。
	IncludeArchived bool
	SortKey         SortKey
	Descending      bool
	Limit           int32
	Offset          int32
}

// ListCriteriaInput は正規化前の一覧条件。presentation layerからそのまま渡す。
type ListCriteriaInput struct {
	Keyword             string
	CategoryPublicID    *uuid.UUID
	TagPublicID         *uuid.UUID
	NecessityLevelCode  string
	UsageFrequencyCode  string
	MobilityClassCode   string
	StorageUnitPublicID *uuid.UUID
	IsUnassigned        bool
	IncludeArchived     bool
	SortKeyName         string
	Order               string
	Limit               *int32
	Offset              *int32
}

// NewListCriteria は入力を検証し、既定値を適用した条件を返す。
//
// 既定値: sort=updatedAt、order=desc、limit=50、offset=0。
func NewListCriteria(input ListCriteriaInput) (ListCriteria, error) {
	criteria := ListCriteria{
		CategoryPublicID:    input.CategoryPublicID,
		TagPublicID:         input.TagPublicID,
		StorageUnitPublicID: input.StorageUnitPublicID,
		IsUnassigned:        input.IsUnassigned,
		IncludeArchived:     input.IncludeArchived,
	}

	keyword := strings.TrimSpace(input.Keyword)
	if len([]rune(keyword)) > MaxKeywordLength {
		return ListCriteria{}, newCriteriaError(
			"keyword", "検索キーワードは100文字以内で入力してください。")
	}
	criteria.Keyword = keyword

	if code := strings.TrimSpace(input.NecessityLevelCode); code != "" {
		level, err := NewNecessityLevel(code)
		if err != nil {
			return ListCriteria{}, err
		}
		criteria.NecessityLevel = &level
	}
	if code := strings.TrimSpace(input.UsageFrequencyCode); code != "" {
		frequency, err := NewUsageFrequency(code)
		if err != nil {
			return ListCriteria{}, err
		}
		criteria.UsageFrequency = &frequency
	}
	if code := strings.TrimSpace(input.MobilityClassCode); code != "" {
		class, err := NewMobilityClass(code)
		if err != nil {
			return ListCriteria{}, err
		}
		criteria.MobilityClass = &class
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
	case "", "desc":
		criteria.Descending = true
	case "asc":
		criteria.Descending = false
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

// PageCriteria は使用記録履歴などの単純なpagination条件。
type PageCriteria struct {
	Limit  int32
	Offset int32
}

// NewPageCriteria は既定値を適用したpagination条件を返す。
func NewPageCriteria(limit, offset *int32) (PageCriteria, error) {
	page := PageCriteria{Limit: DefaultListLimit}
	if limit != nil {
		if *limit < 1 || *limit > MaxListLimit {
			return PageCriteria{}, newCriteriaError(
				"limit", "取得件数は1以上100以下で指定してください。")
		}
		page.Limit = *limit
	}
	if offset != nil {
		if *offset < 0 {
			return PageCriteria{}, newCriteriaError("offset", "offsetは0以上で指定してください。")
		}
		page.Offset = *offset
	}
	return page, nil
}

func newCriteriaError(field, message string) *shared.DomainError {
	return shared.NewInvalidInputError("INVALID_LIST_CRITERIA", message).
		WithFieldErrors(shared.NewFieldError(field, "INVALID_VALUE", message))
}
