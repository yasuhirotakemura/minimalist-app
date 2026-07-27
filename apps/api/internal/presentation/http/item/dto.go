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
		ItemKindCode:        openapi.ItemKindCode(result.ItemKindCode),
		ItemKindLabel:       result.ItemKindLabel,
		Quantity:            result.Quantity,
		UnitName:            result.UnitName,
		NecessityLevelCode:  openapi.NecessityLevelCode(result.NecessityLevelCode),
		NecessityLevelLabel: result.NecessityLevelLabel,
		UsageFrequencyCode:  openapi.UsageFrequencyCode(result.UsageFrequencyCode),
		UsageFrequencyLabel: result.UsageFrequencyLabel,
		PurchasedOn:         toOpenAPIDate(result.PurchasedOn),
		SourceUrl:           result.SourceURL,
		Notes:               result.Notes,
		Tags:                toTagReferenceResponses(result.Tags),
		IsArchived:          result.IsArchived,
		ArchivedAt:          utcTimePointer(result.ArchivedAt),
		Version:             result.Version,
		CreatedAt:           result.CreatedAt.UTC(),
		UpdatedAt:           result.UpdatedAt.UTC(),
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

// toDashboardSummaryResponse は集計結果をresponse DTOへ変換する。
func toDashboardSummaryResponse(
	result applicationitem.DashboardSummaryResult,
) openapi.DashboardSummaryResponse {
	categories := make(
		[]openapi.DashboardCategoryBreakdownResponse, 0, len(result.CategoryBreakdown))
	for _, source := range result.CategoryBreakdown {
		categories = append(categories, openapi.DashboardCategoryBreakdownResponse{
			Category: openapi.CategoryReferenceResponse{
				PublicId: openapitypes.UUID(source.Category.PublicID),
				Name:     source.Category.Name,
			},
			ItemTypeCount: source.Counts.TypeCount,
			TotalQuantity: source.Counts.TotalQuantity,
		})
	}

	return openapi.DashboardSummaryResponse{
		ItemTypeCount:           result.Total.TypeCount,
		TotalQuantity:           result.Total.TotalQuantity,
		CategoryBreakdown:       categories,
		NecessityLevelBreakdown: toCodeBreakdownResponses(result.NecessityLevelBreakdown),
		UsageFrequencyBreakdown: toCodeBreakdownResponses(result.UsageFrequencyBreakdown),
	}
}

func toCodeBreakdownResponses(
	sources []applicationitem.CodeBreakdownResult,
) []openapi.DashboardCodeBreakdownResponse {
	responses := make([]openapi.DashboardCodeBreakdownResponse, 0, len(sources))
	for _, source := range sources {
		responses = append(responses, openapi.DashboardCodeBreakdownResponse{
			Code:          source.Code,
			Label:         source.Label,
			ItemTypeCount: source.Counts.TypeCount,
			TotalQuantity: source.Counts.TotalQuantity,
		})
	}
	return responses
}

// toAttributesParams はrequest DTOをApplication Paramsへ変換する。
//
// 未指定のcodeは空文字として渡し、既定値の適用はDomainが行う。
func toAttributesParams(
	name string,
	categoryPublicID openapitypes.UUID,
	itemKindCode *openapi.ItemKindCode,
	quantity int32,
	unitName *string,
	necessityLevelCode openapi.NecessityLevelCode,
	usageFrequencyCode openapi.UsageFrequencyCode,
	optional optionalAttributes,
) applicationitem.AttributesParams {
	return applicationitem.AttributesParams{
		Name:               name,
		CategoryPublicID:   uuid.UUID(categoryPublicID),
		ItemKindCode:       optionalItemKindCode(itemKindCode),
		Quantity:           quantity,
		UnitName:           stringValue(unitName),
		NecessityLevelCode: string(necessityLevelCode),
		UsageFrequencyCode: string(usageFrequencyCode),
		PurchasedOn:        fromOpenAPIDate(optional.PurchasedOn),
		SourceURL:          optional.SourceURL,
		Notes:              optional.Notes,
		TagPublicIDs:       toUUIDs(optional.TagPublicIDs),
	}
}

// optionalAttributes は任意入力項目をまとめ、変換関数の引数を短く保つ。
type optionalAttributes struct {
	PurchasedOn  *openapitypes.Date
	SourceURL    *string
	Notes        *string
	TagPublicIDs *[]openapitypes.UUID
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
