// Package storage は収納単位・収納割当endpointのHTTP層である。
package storage

import (
	openapitypes "github.com/oapi-codegen/runtime/types"

	applicationstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/storage"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/openapi"
)

// toStorageUnitResponse はApplicationの結果をresponse DTOへ変換する。
func toStorageUnitResponse(
	source applicationstorage.StorageUnitResult,
) openapi.StorageUnitResponse {
	return openapi.StorageUnitResponse{
		PublicId:                openapitypes.UUID(source.PublicID),
		Name:                    source.Name,
		StorageTypeCode:         openapi.StorageTypeCode(source.StorageTypeCode),
		StorageTypeLabel:        source.StorageTypeLabel,
		MobilityClassCode:       openapi.MobilityClassCode(source.MobilityClassCode),
		MobilityClassLabel:      source.MobilityClassLabel,
		Parent:                  toStorageUnitReferenceResponsePointer(source.Parent),
		Ancestors:               toStorageUnitReferenceResponses(source.Ancestors),
		Depth:                   source.Depth,
		ChildCount:              source.ChildCount,
		TareWeightGram:          source.TareWeightGram,
		MaximumWeightGram:       source.MaximumWeightGram,
		MaximumVolumeMilliliter: source.MaximumVolumeMilliliter,
		Description:             source.Description,
		SortOrder:               source.SortOrder,
		Capacity:                toCapacityResponse(source.Capacity),
		IsArchived:              source.IsArchived,
		ArchivedAt:              source.ArchivedAt,
		Version:                 source.Version,
		CreatedAt:               source.CreatedAt,
		UpdatedAt:               source.UpdatedAt,
	}
}

func toStorageUnitReferenceResponsePointer(
	source *applicationstorage.StorageUnitReferenceResult,
) *openapi.StorageUnitReferenceResponse {
	if source == nil {
		return nil
	}
	reference := openapi.StorageUnitReferenceResponse{
		PublicId: openapitypes.UUID(source.PublicID),
		Name:     source.Name,
	}
	return &reference
}

func toStorageUnitReferenceResponses(
	sources []applicationstorage.StorageUnitReferenceResult,
) []openapi.StorageUnitReferenceResponse {
	references := make([]openapi.StorageUnitReferenceResponse, 0, len(sources))
	for _, source := range sources {
		references = append(references, openapi.StorageUnitReferenceResponse{
			PublicId: openapitypes.UUID(source.PublicID),
			Name:     source.Name,
		})
	}
	return references
}

func toCapacityResponse(
	source applicationstorage.CapacityResult,
) openapi.StorageUnitCapacityResponse {
	return openapi.StorageUnitCapacityResponse{
		AllocatedItemKindCount:     source.AllocatedItemKindCount,
		AllocatedQuantity:          source.AllocatedQuantity,
		TareWeightGram:             source.TareWeightGram,
		ItemWeightGram:             source.ItemWeightGram,
		DescendantWeightGram:       source.DescendantWeightGram,
		TotalWeightGram:            source.TotalWeightGram,
		ItemVolumeMilliliter:       source.ItemVolumeMilliliter,
		DescendantVolumeMilliliter: source.DescendantVolumeMilliliter,
		TotalVolumeMilliliter:      source.TotalVolumeMilliliter,
		MaximumWeightGram:          source.MaximumWeightGram,
		MaximumVolumeMilliliter:    source.MaximumVolumeMilliliter,
		RemainingWeightGram:        source.RemainingWeightGram,
		RemainingVolumeMilliliter:  source.RemainingVolumeMilliliter,
		IsWeightExceeded:           source.IsWeightExceeded,
		IsVolumeExceeded:           source.IsVolumeExceeded,
		HasUnknownWeight:           source.HasUnknownWeight,
		HasUnknownVolume:           source.HasUnknownVolume,
	}
}

func toContentsResponse(
	source applicationstorage.StorageUnitContentsResult,
) openapi.StorageUnitContentsResponse {
	allocations := make([]openapi.StorageAllocationResponse, 0, len(source.Allocations))
	for _, allocation := range source.Allocations {
		allocations = append(allocations, openapi.StorageAllocationResponse{
			PublicId: openapitypes.UUID(allocation.PublicID),
			Item: openapi.AllocatedItemResponse{
				PublicId:           openapitypes.UUID(allocation.Item.PublicID),
				Name:               allocation.Item.Name,
				UnitName:           allocation.Item.UnitName,
				Quantity:           allocation.Item.Quantity,
				AssignedQuantity:   allocation.Item.AssignedQuantity,
				UnassignedQuantity: allocation.Item.UnassignedQuantity,
				WeightGram:         allocation.Item.WeightGram,
				VolumeMilliliter:   allocation.Item.VolumeMilliliter,
				IsArchived:         allocation.Item.IsArchived,
			},
			Quantity:  allocation.Quantity,
			Version:   allocation.Version,
			CreatedAt: allocation.CreatedAt,
			UpdatedAt: allocation.UpdatedAt,
		})
	}

	children := make([]openapi.StorageUnitResponse, 0, len(source.ChildStorageUnits))
	for _, child := range source.ChildStorageUnits {
		children = append(children, toStorageUnitResponse(child))
	}

	return openapi.StorageUnitContentsResponse{
		StorageUnit:       toStorageUnitResponse(source.StorageUnit),
		Allocations:       allocations,
		ChildStorageUnits: children,
	}
}

func toItemStorageAllocationListResponse(
	source applicationstorage.ListItemStorageAllocationsResult,
) openapi.ItemStorageAllocationListResponse {
	items := make([]openapi.ItemStorageAllocationResponse, 0, len(source.Items))
	for _, allocation := range source.Items {
		items = append(items, openapi.ItemStorageAllocationResponse{
			PublicId: openapitypes.UUID(allocation.PublicID),
			StorageUnit: openapi.StorageUnitReferenceResponse{
				PublicId: openapitypes.UUID(allocation.StorageUnit.PublicID),
				Name:     allocation.StorageUnit.Name,
			},
			Quantity:  allocation.Quantity,
			Version:   allocation.Version,
			CreatedAt: allocation.CreatedAt,
			UpdatedAt: allocation.UpdatedAt,
		})
	}

	return openapi.ItemStorageAllocationListResponse{
		Items:              items,
		Quantity:           source.Quantity,
		AssignedQuantity:   source.AssignedQuantity,
		UnassignedQuantity: source.UnassignedQuantity,
	}
}

func toPaginationResponse(
	source applicationstorage.PaginationResult,
) openapi.PaginationResponse {
	return openapi.PaginationResponse{
		Limit:      source.Limit,
		Offset:     source.Offset,
		TotalCount: source.TotalCount,
		HasNext:    source.HasNext,
	}
}
