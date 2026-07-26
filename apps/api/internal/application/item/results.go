package item

import (
	"time"

	"github.com/google/uuid"

	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
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

// StorageUnitReferenceResult は割当先の収納単位の最小表現 (Phase 2)。
type StorageUnitReferenceResult struct {
	PublicID uuid.UUID
	Name     string
}

// StorageAllocationSummaryResult はアイテム側から見た収納割当1件 (Phase 2)。
//
// 同一アイテムを複数収納単位へ分割割当できるため、ItemResultは本型の配列を持つ。
type StorageAllocationSummaryResult struct {
	PublicID    uuid.UUID
	StorageUnit StorageUnitReferenceResult
	Quantity    int32
	Version     int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ItemResult はユースケースが返す所持品の表現。
//
// 内部IDを含めない (設計書 12.1)。codeとlabelはDomainのValueObjectが持つ
// 対応関係を展開した結果であり、presentation layerはそのままresponseへ写す。
type ItemResult struct {
	PublicID              uuid.UUID
	Name                  string
	Category              CategoryReferenceResult
	ItemKindCode          string
	ItemKindLabel         string
	Quantity              int32
	DesiredQuantity       *int32
	UnitName              string
	NecessityLevelCode    string
	NecessityLevelLabel   string
	UsageFrequencyCode    string
	UsageFrequencyLabel   string
	SubstitutabilityCode  string
	SubstitutabilityLabel string
	MobilityClassCode     string
	MobilityClassLabel    string
	OwnershipReason       *string
	DisposalCondition     *string
	LastUsedAt            *time.Time
	PurchasedOn           *time.Time
	PurchaseAmount        *int64
	ReplacementAmount     *int64
	ResaleAmount          *int64
	WeightGram            *int32
	VolumeMilliliter      *int32
	IsFragile             bool
	IsValuable            bool
	IsSentimental         bool
	RequiresMaintenance   bool
	ExpiresOn             *time.Time
	SourceURL             *string
	Notes                 *string
	IsConfirmed           bool
	ConfirmedAt           *time.Time
	Tags                  []TagReferenceResult
	// StorageAllocations は本アイテムがどの収納単位へ何個入っているか (Phase 2)。
	StorageAllocations []StorageAllocationSummaryResult
	// UnassignedQuantity は quantity - 割当数量合計。DBへ保存せず算出する。
	UnassignedQuantity int32
	IsArchived         bool
	ArchivedAt         *time.Time
	Version            int32
	CreatedAt          time.Time
	UpdatedAt          time.Time
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

// UsageRecordResult は使用記録の表現。
type UsageRecordResult struct {
	PublicID  uuid.UUID
	UsedAt    time.Time
	Quantity  int32
	Note      *string
	CreatedAt time.Time
}

// ListUsageRecordsResult は使用記録履歴の結果。
type ListUsageRecordsResult struct {
	Items      []UsageRecordResult
	Pagination PaginationResult
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
		ItemKindCode:          attributes.Kind.String(),
		ItemKindLabel:         attributes.Kind.Label(),
		Quantity:              attributes.Quantity,
		DesiredQuantity:       attributes.DesiredQuantity,
		UnitName:              attributes.UnitName,
		NecessityLevelCode:    attributes.NecessityLevel.String(),
		NecessityLevelLabel:   attributes.NecessityLevel.Label(),
		UsageFrequencyCode:    attributes.UsageFrequency.String(),
		UsageFrequencyLabel:   attributes.UsageFrequency.Label(),
		SubstitutabilityCode:  attributes.Substitutability.String(),
		SubstitutabilityLabel: attributes.Substitutability.Label(),
		MobilityClassCode:     attributes.MobilityClass.String(),
		MobilityClassLabel:    attributes.MobilityClass.Label(),
		OwnershipReason:       attributes.OwnershipReason,
		DisposalCondition:     attributes.DisposalCondition,
		LastUsedAt:            attributes.LastUsedAt,
		PurchasedOn:           attributes.PurchasedOn,
		PurchaseAmount:        attributes.PurchaseAmount,
		ReplacementAmount:     attributes.ReplacementAmount,
		ResaleAmount:          attributes.ResaleAmount,
		WeightGram:            attributes.WeightGram,
		VolumeMilliliter:      attributes.VolumeMilliliter,
		IsFragile:             attributes.IsFragile,
		IsValuable:            attributes.IsValuable,
		IsSentimental:         attributes.IsSentimental,
		RequiresMaintenance:   attributes.RequiresMaintenance,
		ExpiresOn:             attributes.ExpiresOn,
		SourceURL:             attributes.SourceURL,
		Notes:                 attributes.Notes,
		IsConfirmed:           source.IsConfirmed(),
		ConfirmedAt:           source.ConfirmedAt(),
		Tags:                  newTagReferenceResults(source),
		StorageAllocations:    []StorageAllocationSummaryResult{},
		UnassignedQuantity:    attributes.Quantity,
		IsArchived:            source.IsArchived(),
		ArchivedAt:            source.ArchivedAt(),
		Version:               source.Version(),
		CreatedAt:             source.CreatedAt(),
		UpdatedAt:             source.UpdatedAt(),
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

func newUsageRecordResult(source domainitem.UsageRecord) UsageRecordResult {
	return UsageRecordResult{
		PublicID:  source.PublicID(),
		UsedAt:    source.UsedAt(),
		Quantity:  source.Quantity(),
		Note:      source.Note(),
		CreatedAt: source.CreatedAt(),
	}
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

// withStorageAllocations は収納割当と未割当数量を付与した複製を返す (Phase 2)。
//
// 未割当数量はDBへ重複保存せず、取得時に算出する。
func (r ItemResult) withStorageAllocations(
	allocations []domainstorage.StorageAllocation,
) ItemResult {
	summaries := make([]StorageAllocationSummaryResult, 0, len(allocations))
	var assignedQuantity int64
	for _, allocation := range allocations {
		assignedQuantity += int64(allocation.Quantity())
		summaries = append(summaries, StorageAllocationSummaryResult{
			PublicID: allocation.PublicID(),
			StorageUnit: StorageUnitReferenceResult{
				PublicID: allocation.StorageUnit().PublicID,
				Name:     allocation.StorageUnit().Name,
			},
			Quantity:  allocation.Quantity(),
			Version:   allocation.Version(),
			CreatedAt: allocation.CreatedAt(),
			UpdatedAt: allocation.UpdatedAt(),
		})
	}

	r.StorageAllocations = summaries
	r.UnassignedQuantity = domainstorage.UnassignedQuantity(r.Quantity, assignedQuantity)
	return r
}
