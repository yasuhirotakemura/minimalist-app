// Package item は所持品endpointのHTTP handlerを提供する。
package item

import (
	"time"

	"github.com/google/uuid"
	openapitypes "github.com/oapi-codegen/runtime/types"

	applicationitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/item"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/openapi"
)

// toItemResponse はApplication ResultをHTTP Response DTOへ変換する。
//
// Application ResultとResponse DTOを分離し、API契約の変更が
// Application layerへ波及しないようにする (設計書 11.1)。
func toItemResponse(result applicationitem.ItemResult) openapi.ItemResponse {
	return openapi.ItemResponse{
		PublicId: openapitypes.UUID(result.PublicID),
		Name:     result.Name,
		Category: openapi.CategoryReferenceResponse{
			PublicId: openapitypes.UUID(result.Category.PublicID),
			Name:     result.Category.Name,
		},
		ItemKindCode:          openapi.ItemKindCode(result.ItemKindCode),
		ItemKindLabel:         result.ItemKindLabel,
		Quantity:              result.Quantity,
		DesiredQuantity:       result.DesiredQuantity,
		UnitName:              result.UnitName,
		NecessityLevelCode:    openapi.NecessityLevelCode(result.NecessityLevelCode),
		NecessityLevelLabel:   result.NecessityLevelLabel,
		UsageFrequencyCode:    openapi.UsageFrequencyCode(result.UsageFrequencyCode),
		UsageFrequencyLabel:   result.UsageFrequencyLabel,
		SubstitutabilityCode:  openapi.SubstitutabilityCode(result.SubstitutabilityCode),
		SubstitutabilityLabel: result.SubstitutabilityLabel,
		MobilityClassCode:     openapi.MobilityClassCode(result.MobilityClassCode),
		MobilityClassLabel:    result.MobilityClassLabel,
		OwnershipReason:       result.OwnershipReason,
		DisposalCondition:     result.DisposalCondition,
		LastUsedAt:            utcTimePointer(result.LastUsedAt),
		PurchasedOn:           toOpenAPIDate(result.PurchasedOn),
		PurchaseAmount:        result.PurchaseAmount,
		ReplacementAmount:     result.ReplacementAmount,
		ResaleAmount:          result.ResaleAmount,
		WeightGram:            result.WeightGram,
		VolumeMilliliter:      result.VolumeMilliliter,
		IsFragile:             result.IsFragile,
		IsValuable:            result.IsValuable,
		IsSentimental:         result.IsSentimental,
		RequiresMaintenance:   result.RequiresMaintenance,
		ExpiresOn:             toOpenAPIDate(result.ExpiresOn),
		SourceUrl:             result.SourceURL,
		Notes:                 result.Notes,
		IsConfirmed:           result.IsConfirmed,
		ConfirmedAt:           utcTimePointer(result.ConfirmedAt),
		Tags:                  toTagReferenceResponses(result.Tags),
		StorageAllocations:    toItemStorageAllocationResponses(result.StorageAllocations),
		UnassignedQuantity:    result.UnassignedQuantity,
		IsArchived:            result.IsArchived,
		ArchivedAt:            utcTimePointer(result.ArchivedAt),
		Version:               result.Version,
		CreatedAt:             result.CreatedAt.UTC(),
		UpdatedAt:             result.UpdatedAt.UTC(),
	}
}

func toTagReferenceResponses(
	results []applicationitem.TagReferenceResult,
) []openapi.TagReferenceResponse {
	responses := make([]openapi.TagReferenceResponse, 0, len(results))
	for _, result := range results {
		responses = append(responses, openapi.TagReferenceResponse{
			PublicId: openapitypes.UUID(result.PublicID),
			Name:     result.Name,
		})
	}
	return responses
}

func toUsageRecordResponse(
	result applicationitem.UsageRecordResult,
) openapi.ItemUsageRecordResponse {
	return openapi.ItemUsageRecordResponse{
		PublicId:  openapitypes.UUID(result.PublicID),
		UsedAt:    result.UsedAt.UTC(),
		Quantity:  result.Quantity,
		Note:      result.Note,
		CreatedAt: result.CreatedAt.UTC(),
	}
}

func toPaginationResponse(
	result applicationitem.PaginationResult,
) openapi.PaginationResponse {
	return openapi.PaginationResponse{
		Limit:      result.Limit,
		Offset:     result.Offset,
		TotalCount: result.TotalCount,
		HasNext:    result.HasNext,
	}
}

// toAttributesParams はrequest DTOをApplication Paramsへ変換する。
//
// 未指定のbooleanはfalse、未指定のcodeは空文字として渡し、
// 既定値の適用はDomainが行う。
func toAttributesParams(
	name string,
	categoryPublicID openapitypes.UUID,
	itemKindCode *openapi.ItemKindCode,
	quantity int32,
	desiredQuantity *int32,
	unitName *string,
	necessityLevelCode openapi.NecessityLevelCode,
	usageFrequencyCode openapi.UsageFrequencyCode,
	substitutabilityCode openapi.SubstitutabilityCode,
	mobilityClassCode openapi.MobilityClassCode,
	optional optionalAttributes,
) applicationitem.AttributesParams {
	return applicationitem.AttributesParams{
		Name:                 name,
		CategoryPublicID:     uuid.UUID(categoryPublicID),
		ItemKindCode:         optionalItemKindCode(itemKindCode),
		Quantity:             quantity,
		DesiredQuantity:      desiredQuantity,
		UnitName:             stringValue(unitName),
		NecessityLevelCode:   string(necessityLevelCode),
		UsageFrequencyCode:   string(usageFrequencyCode),
		SubstitutabilityCode: string(substitutabilityCode),
		MobilityClassCode:    string(mobilityClassCode),
		OwnershipReason:      optional.OwnershipReason,
		DisposalCondition:    optional.DisposalCondition,
		LastUsedAt:           optional.LastUsedAt,
		PurchasedOn:          fromOpenAPIDate(optional.PurchasedOn),
		PurchaseAmount:       optional.PurchaseAmount,
		ReplacementAmount:    optional.ReplacementAmount,
		ResaleAmount:         optional.ResaleAmount,
		WeightGram:           optional.WeightGram,
		VolumeMilliliter:     optional.VolumeMilliliter,
		IsFragile:            booleanValue(optional.IsFragile),
		IsValuable:           booleanValue(optional.IsValuable),
		IsSentimental:        booleanValue(optional.IsSentimental),
		RequiresMaintenance:  booleanValue(optional.RequiresMaintenance),
		ExpiresOn:            fromOpenAPIDate(optional.ExpiresOn),
		SourceURL:            optional.SourceURL,
		Notes:                optional.Notes,
		TagPublicIDs:         toUUIDs(optional.TagPublicIDs),
	}
}

// optionalAttributes は任意入力項目をまとめ、変換関数の引数を短く保つ。
type optionalAttributes struct {
	OwnershipReason     *string
	DisposalCondition   *string
	LastUsedAt          *time.Time
	PurchasedOn         *openapitypes.Date
	PurchaseAmount      *int64
	ReplacementAmount   *int64
	ResaleAmount        *int64
	WeightGram          *int32
	VolumeMilliliter    *int32
	IsFragile           *bool
	IsValuable          *bool
	IsSentimental       *bool
	RequiresMaintenance *bool
	ExpiresOn           *openapitypes.Date
	SourceURL           *string
	Notes               *string
	TagPublicIDs        *[]openapitypes.UUID
}

func optionalItemKindCode(value *openapi.ItemKindCode) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func booleanValue(value *bool) bool {
	return value != nil && *value
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func toOpenAPIDate(value *time.Time) *openapitypes.Date {
	if value == nil {
		return nil
	}
	return &openapitypes.Date{Time: value.UTC()}
}

func fromOpenAPIDate(value *openapitypes.Date) *time.Time {
	if value == nil {
		return nil
	}
	instant := value.Time.UTC()
	return &instant
}

func toUUIDs(values *[]openapitypes.UUID) []uuid.UUID {
	if values == nil {
		return nil
	}
	converted := make([]uuid.UUID, 0, len(*values))
	for _, value := range *values {
		converted = append(converted, uuid.UUID(value))
	}
	return converted
}

// toItemStorageAllocationResponses は収納割当をresponse DTOへ変換する (Phase 2)。
//
// 同一アイテムを複数収納単位へ分割割当できるため配列で返す。
func toItemStorageAllocationResponses(
	sources []applicationitem.StorageAllocationSummaryResult,
) []openapi.ItemStorageAllocationResponse {
	allocations := make([]openapi.ItemStorageAllocationResponse, 0, len(sources))
	for _, source := range sources {
		allocations = append(allocations, openapi.ItemStorageAllocationResponse{
			PublicId: openapitypes.UUID(source.PublicID),
			StorageUnit: openapi.StorageUnitReferenceResponse{
				PublicId: openapitypes.UUID(source.StorageUnit.PublicID),
				Name:     source.StorageUnit.Name,
			},
			Quantity:  source.Quantity,
			Version:   source.Version,
			CreatedAt: source.CreatedAt.UTC(),
			UpdatedAt: source.UpdatedAt.UTC(),
		})
	}
	return allocations
}
