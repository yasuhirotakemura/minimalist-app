package item

import (
	"time"

	"github.com/google/uuid"

	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
)

// CategoryReferenceResult は所持品に紐づくカテゴリーの表現。
type CategoryReferenceResult struct {
	PublicID uuid.UUID
	Name     string
}

// TagReferenceResult は所持品に付与されたタグの表現。
type TagReferenceResult struct {
	PublicID uuid.UUID
	Name     string
}

// ItemResult はユースケースが返す所持品の表現。
//
// 内部IDを含めない (設計書 12.1)。codeとlabelはDomainのValueObjectが持つ
// 対応関係を展開した結果であり、presentation layerはそのままresponseへ写す。
type ItemResult struct {
	PublicID            uuid.UUID
	Name                string
	Category            CategoryReferenceResult
	ItemKindCode        string
	ItemKindLabel       string
	Quantity            int32
	UnitName            string
	NecessityLevelCode  string
	NecessityLevelLabel string
	UsageFrequencyCode  string
	UsageFrequencyLabel string
	PurchasedOn         *time.Time
	SourceURL           *string
	Notes               *string
	Tags                []TagReferenceResult
	IsArchived          bool
	ArchivedAt          *time.Time
	Version             int32
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// PaginationResult はoffset paginationの結果。
type PaginationResult struct {
	Limit      int32
	Offset     int32
	TotalCount int64
	HasNext    bool
}

// ListItemsResult は所持品一覧の結果。
type ListItemsResult struct {
	Items      []ItemResult
	Pagination PaginationResult
}

// CountsResult は集計の一組。
//
// TypeCount はアイテム種別の数、TotalQuantity は所有数量の合計を表す。
type CountsResult struct {
	TypeCount     int64
	TotalQuantity int64
}

// CategoryBreakdownResult はカテゴリー別の内訳1件。
type CategoryBreakdownResult struct {
	Category CategoryReferenceResult
	Counts   CountsResult
}

// CodeBreakdownResult はcode体系別の内訳1件。
//
// 必要度・使用頻度で共通の形とする。labelはDomainのValueObjectが持つ表示名。
type CodeBreakdownResult struct {
	Code   string
	Label  string
	Counts CountsResult
}

// DashboardSummaryResult はダッシュボードの集計結果 (設計書 9.3)。
type DashboardSummaryResult struct {
	Total                   CountsResult
	CategoryBreakdown       []CategoryBreakdownResult
	NecessityLevelBreakdown []CodeBreakdownResult
	UsageFrequencyBreakdown []CodeBreakdownResult
}

func newItemResult(source domainitem.Item) ItemResult {
	attributes := source.Attributes()

	return ItemResult{
		PublicID: source.PublicID(),
		Name:     attributes.Name,
		Category: CategoryReferenceResult{
			PublicID: attributes.Category.PublicID,
			Name:     attributes.Category.Name,
		},
		ItemKindCode:        attributes.Kind.String(),
		ItemKindLabel:       attributes.Kind.Label(),
		Quantity:            attributes.Quantity,
		UnitName:            attributes.UnitName,
		NecessityLevelCode:  attributes.NecessityLevel.String(),
		NecessityLevelLabel: attributes.NecessityLevel.Label(),
		UsageFrequencyCode:  attributes.UsageFrequency.String(),
		UsageFrequencyLabel: attributes.UsageFrequency.Label(),
		PurchasedOn:         attributes.PurchasedOn,
		SourceURL:           attributes.SourceURL,
		Notes:               attributes.Notes,
		Tags:                newTagReferenceResults(source),
		IsArchived:          source.IsArchived(),
		ArchivedAt:          source.ArchivedAt(),
		Version:             source.Version(),
		CreatedAt:           source.CreatedAt(),
		UpdatedAt:           source.UpdatedAt(),
	}
}

func newTagReferenceResults(source domainitem.Item) []TagReferenceResult {
	references := source.Tags()
	results := make([]TagReferenceResult, 0, len(references))
	for _, reference := range references {
		results = append(results, TagReferenceResult{
			PublicID: reference.PublicID,
			Name:     reference.Name,
		})
	}
	return results
}

// newPaginationResult はpagination結果を組み立てる。
func newPaginationResult(limit, offset int32, totalCount int64) PaginationResult {
	return PaginationResult{
		Limit:      limit,
		Offset:     offset,
		TotalCount: totalCount,
		HasNext:    int64(offset)+int64(limit) < totalCount,
	}
}

func newCountsResult(source domainitem.Counts) CountsResult {
	return CountsResult{
		TypeCount:     source.TypeCount,
		TotalQuantity: source.TotalQuantity,
	}
}

func newDashboardSummaryResult(source domainitem.Summary) DashboardSummaryResult {
	result := DashboardSummaryResult{
		Total:                   newCountsResult(source.Total),
		CategoryBreakdown:       make([]CategoryBreakdownResult, 0, len(source.ByCategory)),
		NecessityLevelBreakdown: make([]CodeBreakdownResult, 0, len(source.ByNecessityLevel)),
		UsageFrequencyBreakdown: make([]CodeBreakdownResult, 0, len(source.ByUsageFrequency)),
	}

	for _, entry := range source.ByCategory {
		result.CategoryBreakdown = append(result.CategoryBreakdown, CategoryBreakdownResult{
			Category: CategoryReferenceResult{
				PublicID: entry.Category.PublicID,
				Name:     entry.Category.Name,
			},
			Counts: newCountsResult(entry.Counts),
		})
	}

	for _, entry := range source.ByNecessityLevel {
		result.NecessityLevelBreakdown = append(
			result.NecessityLevelBreakdown,
			CodeBreakdownResult{
				Code:   entry.Level.String(),
				Label:  entry.Level.Label(),
				Counts: newCountsResult(entry.Counts),
			},
		)
	}

	for _, entry := range source.ByUsageFrequency {
		result.UsageFrequencyBreakdown = append(
			result.UsageFrequencyBreakdown,
			CodeBreakdownResult{
				Code:   entry.Frequency.String(),
				Label:  entry.Frequency.Label(),
				Counts: newCountsResult(entry.Counts),
			},
		)
	}

	return result
}
