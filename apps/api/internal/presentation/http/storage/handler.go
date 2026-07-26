package storage

import (
	"net/http"

	"github.com/google/uuid"
	openapitypes "github.com/oapi-codegen/runtime/types"

	applicationstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/storage"
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/openapi"
	authhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"
)

// Handler は収納単位・収納割当endpointのHTTP handlerである。
//
// Handlerが行うこと (設計書 11.2): body・path・query取得、Application Service呼出、
// DTO変換、HTTP error変換。
// Handlerが行わないこと (設計書 11.3): SQL、業務ルール、Entityの状態遷移。
type Handler struct {
	createStorageUnit          *applicationstorage.CreateStorageUnitService
	updateStorageUnit          *applicationstorage.UpdateStorageUnitService
	archiveStorageUnit         *applicationstorage.ArchiveStorageUnitService
	restoreStorageUnit         *applicationstorage.RestoreStorageUnitService
	getStorageUnit             *applicationstorage.GetStorageUnitService
	listStorageUnits           *applicationstorage.ListStorageUnitsService
	getStorageUnitContents     *applicationstorage.GetStorageUnitContentsService
	calculateCapacity          *applicationstorage.CalculateStorageUnitCapacityService
	assignItem                 *applicationstorage.AssignItemToStorageUnitService
	updateAllocation           *applicationstorage.UpdateStorageAllocationService
	removeAllocation           *applicationstorage.RemoveStorageAllocationService
	replaceAllocations         *applicationstorage.ReplaceStorageAllocationsService
	listItemStorageAllocations *applicationstorage.ListItemStorageAllocationsService
}

// HandlerDependencies はHandlerの依存。
type HandlerDependencies struct {
	CreateStorageUnit          *applicationstorage.CreateStorageUnitService
	UpdateStorageUnit          *applicationstorage.UpdateStorageUnitService
	ArchiveStorageUnit         *applicationstorage.ArchiveStorageUnitService
	RestoreStorageUnit         *applicationstorage.RestoreStorageUnitService
	GetStorageUnit             *applicationstorage.GetStorageUnitService
	ListStorageUnits           *applicationstorage.ListStorageUnitsService
	GetStorageUnitContents     *applicationstorage.GetStorageUnitContentsService
	CalculateCapacity          *applicationstorage.CalculateStorageUnitCapacityService
	AssignItem                 *applicationstorage.AssignItemToStorageUnitService
	UpdateAllocation           *applicationstorage.UpdateStorageAllocationService
	RemoveAllocation           *applicationstorage.RemoveStorageAllocationService
	ReplaceAllocations         *applicationstorage.ReplaceStorageAllocationsService
	ListItemStorageAllocations *applicationstorage.ListItemStorageAllocationsService
}

// NewHandler はHandlerを生成する。
func NewHandler(dependencies HandlerDependencies) *Handler {
	return &Handler{
		createStorageUnit:          dependencies.CreateStorageUnit,
		updateStorageUnit:          dependencies.UpdateStorageUnit,
		archiveStorageUnit:         dependencies.ArchiveStorageUnit,
		restoreStorageUnit:         dependencies.RestoreStorageUnit,
		getStorageUnit:             dependencies.GetStorageUnit,
		listStorageUnits:           dependencies.ListStorageUnits,
		getStorageUnitContents:     dependencies.GetStorageUnitContents,
		calculateCapacity:          dependencies.CalculateCapacity,
		assignItem:                 dependencies.AssignItem,
		updateAllocation:           dependencies.UpdateAllocation,
		removeAllocation:           dependencies.RemoveAllocation,
		replaceAllocations:         dependencies.ReplaceAllocations,
		listItemStorageAllocations: dependencies.ListItemStorageAllocations,
	}
}

// ListStorageUnits は収納単位一覧を取得する。
// GET /api/storage-units
func (h *Handler) ListStorageUnits(
	w http.ResponseWriter,
	r *http.Request,
	params openapi.ListStorageUnitsParams,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	result, err := h.listStorageUnits.Execute(
		r.Context(),
		applicationstorage.ListStorageUnitsParams{
			UserID: authenticated.ID,
			Criteria: domainstorage.ListCriteriaInput{
				Keyword:           stringValue(params.Keyword),
				StorageTypeCode:   codeValue(params.StorageTypeCode),
				MobilityClassCode: codeValue(params.MobilityClassCode),
				ParentPublicID:    toUUIDPointer(params.ParentStorageUnitPublicId),
				RootOnly:          booleanValue(params.RootOnly),
				IncludeArchived:   booleanValue(params.IncludeArchived),
				SortKeyName:       codeValue(params.Sort),
				Order:             codeValue(params.Order),
				Limit:             params.Limit,
				Offset:            params.Offset,
			},
		},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	units := make([]openapi.StorageUnitResponse, 0, len(result.Items))
	for _, source := range result.Items {
		units = append(units, toStorageUnitResponse(source))
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, openapi.StorageUnitListResponse{
		Items:      units,
		Pagination: toPaginationResponse(result.Pagination),
	})
}

// CreateStorageUnit は収納単位を登録する。
// POST /api/storage-units
func (h *Handler) CreateStorageUnit(w http.ResponseWriter, r *http.Request) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	var body openapi.CreateStorageUnitJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	result, err := h.createStorageUnit.Execute(
		r.Context(),
		applicationstorage.CreateStorageUnitParams{
			UserID: authenticated.ID,
			Attributes: applicationstorage.AttributesParams{
				Name:                      body.Name,
				StorageTypeCode:           string(body.StorageTypeCode),
				MobilityClassCode:         string(body.MobilityClassCode),
				ParentStorageUnitPublicID: toUUIDPointer(body.ParentStorageUnitPublicId),
				TareWeightGram:            body.TareWeightGram,
				MaximumWeightGram:         body.MaximumWeightGram,
				MaximumVolumeMilliliter:   body.MaximumVolumeMilliliter,
				Description:               body.Description,
				SortOrder:                 body.SortOrder,
			},
		},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(
		r.Context(), w, http.StatusCreated, toStorageUnitResponse(result.StorageUnit))
}

// GetStorageUnitByPublicId は収納単位を取得する。
// GET /api/storage-units/{publicId}
func (h *Handler) GetStorageUnitByPublicId(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	result, err := h.getStorageUnit.Execute(
		r.Context(),
		applicationstorage.GetStorageUnitParams{
			UserID:   authenticated.ID,
			PublicID: uuid.UUID(publicID),
		},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, toStorageUnitResponse(result.StorageUnit))
}

// UpdateStorageUnit は収納単位を更新する。
// PUT /api/storage-units/{publicId}
func (h *Handler) UpdateStorageUnit(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	var body openapi.UpdateStorageUnitJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	result, err := h.updateStorageUnit.Execute(
		r.Context(),
		applicationstorage.UpdateStorageUnitParams{
			UserID:   authenticated.ID,
			PublicID: uuid.UUID(publicID),
			Attributes: applicationstorage.AttributesParams{
				Name:                      body.Name,
				StorageTypeCode:           string(body.StorageTypeCode),
				MobilityClassCode:         string(body.MobilityClassCode),
				ParentStorageUnitPublicID: toUUIDPointer(body.ParentStorageUnitPublicId),
				TareWeightGram:            body.TareWeightGram,
				MaximumWeightGram:         body.MaximumWeightGram,
				MaximumVolumeMilliliter:   body.MaximumVolumeMilliliter,
				Description:               body.Description,
				SortOrder:                 body.SortOrder,
			},
			ExpectedVersion: body.ExpectedVersion,
		},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, toStorageUnitResponse(result.StorageUnit))
}

// ArchiveStorageUnit は収納単位をarchiveする。
// POST /api/storage-units/{publicId}/archive
func (h *Handler) ArchiveStorageUnit(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	var body openapi.ArchiveStorageUnitJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	result, err := h.archiveStorageUnit.Execute(
		r.Context(),
		applicationstorage.ArchiveStorageUnitParams{
			UserID:          authenticated.ID,
			PublicID:        uuid.UUID(publicID),
			ExpectedVersion: body.ExpectedVersion,
		},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, toStorageUnitResponse(result.StorageUnit))
}

// RestoreStorageUnit はarchive済みの収納単位を復元する。
// POST /api/storage-units/{publicId}/restore
func (h *Handler) RestoreStorageUnit(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	var body openapi.RestoreStorageUnitJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	result, err := h.restoreStorageUnit.Execute(
		r.Context(),
		applicationstorage.RestoreStorageUnitParams{
			UserID:          authenticated.ID,
			PublicID:        uuid.UUID(publicID),
			ExpectedVersion: body.ExpectedVersion,
		},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, toStorageUnitResponse(result.StorageUnit))
}

// GetStorageUnitCapacity は収納単位の重量・容積を取得する。
// GET /api/storage-units/{publicId}/capacity
func (h *Handler) GetStorageUnitCapacity(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	result, err := h.calculateCapacity.Execute(
		r.Context(),
		applicationstorage.CalculateStorageUnitCapacityParams{
			UserID:   authenticated.ID,
			PublicID: uuid.UUID(publicID),
		},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, toCapacityResponse(result.Capacity))
}

// GetStorageUnitContents は収納単位の内容を取得する。
// GET /api/storage-units/{publicId}/contents
func (h *Handler) GetStorageUnitContents(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	result, err := h.getStorageUnitContents.Execute(
		r.Context(),
		applicationstorage.GetStorageUnitContentsParams{
			UserID:   authenticated.ID,
			PublicID: uuid.UUID(publicID),
		},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, toContentsResponse(result))
}

// CreateStorageAllocation は所持品を収納単位へ割り当てる。
// POST /api/storage-units/{publicId}/allocations
func (h *Handler) CreateStorageAllocation(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	var body openapi.CreateStorageAllocationJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	result, err := h.assignItem.Execute(
		r.Context(),
		applicationstorage.AssignItemToStorageUnitParams{
			UserID:                     authenticated.ID,
			StorageUnitPublicID:        uuid.UUID(publicID),
			ItemPublicID:               uuid.UUID(body.ItemPublicId),
			Quantity:                   body.Quantity,
			ExpectedStorageUnitVersion: body.ExpectedStorageUnitVersion,
		},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusCreated, toContentsResponse(result))
}

// SetStorageUnitAllocations は収納単位の割当を一括置換する。
// PUT /api/storage-units/{publicId}/allocations
func (h *Handler) SetStorageUnitAllocations(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	var body openapi.SetStorageUnitAllocationsJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	allocations := make([]applicationstorage.AllocationInput, 0, len(body.Allocations))
	for _, input := range body.Allocations {
		allocations = append(allocations, applicationstorage.AllocationInput{
			ItemPublicID: uuid.UUID(input.ItemPublicId),
			Quantity:     input.Quantity,
		})
	}

	result, err := h.replaceAllocations.Execute(
		r.Context(),
		applicationstorage.ReplaceStorageAllocationsParams{
			UserID:                     authenticated.ID,
			StorageUnitPublicID:        uuid.UUID(publicID),
			Allocations:                allocations,
			ExpectedStorageUnitVersion: body.ExpectedStorageUnitVersion,
		},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, toContentsResponse(result))
}

// UpdateStorageAllocation は収納割当の数量を変更する。
// PUT /api/storage-units/{publicId}/allocations/{allocationPublicId}
func (h *Handler) UpdateStorageAllocation(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
	allocationPublicID openapi.AllocationPublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	var body openapi.UpdateStorageAllocationJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	result, err := h.updateAllocation.Execute(
		r.Context(),
		applicationstorage.UpdateStorageAllocationParams{
			UserID:                     authenticated.ID,
			StorageUnitPublicID:        uuid.UUID(publicID),
			AllocationPublicID:         uuid.UUID(allocationPublicID),
			Quantity:                   body.Quantity,
			ExpectedVersion:            body.ExpectedVersion,
			ExpectedStorageUnitVersion: body.ExpectedStorageUnitVersion,
		},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, toContentsResponse(result))
}

// DeleteStorageAllocation は収納割当を削除する。
// DELETE /api/storage-units/{publicId}/allocations/{allocationPublicId}
func (h *Handler) DeleteStorageAllocation(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
	allocationPublicID openapi.AllocationPublicIdPathParameter,
	params openapi.DeleteStorageAllocationParams,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	result, err := h.removeAllocation.Execute(
		r.Context(),
		applicationstorage.RemoveStorageAllocationParams{
			UserID:                     authenticated.ID,
			StorageUnitPublicID:        uuid.UUID(publicID),
			AllocationPublicID:         uuid.UUID(allocationPublicID),
			ExpectedVersion:            params.ExpectedVersion,
			ExpectedStorageUnitVersion: params.ExpectedStorageUnitVersion,
		},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	// 更新後の収納単位versionをクライアントへ返すため、204ではなく200で内容を返す。
	shared.WriteJSON(r.Context(), w, http.StatusOK, toContentsResponse(result))
}

// ListItemStorageAllocations は所持品の収納割当を取得する。
// GET /api/items/{publicId}/storage-allocations
func (h *Handler) ListItemStorageAllocations(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	result, err := h.listItemStorageAllocations.Execute(
		r.Context(),
		applicationstorage.ListItemStorageAllocationsParams{
			UserID:       authenticated.ID,
			ItemPublicID: uuid.UUID(publicID),
		},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(
		r.Context(), w, http.StatusOK, toItemStorageAllocationListResponse(result))
}

func toUUIDPointer(value *openapitypes.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	converted := uuid.UUID(*value)
	return &converted
}

// codeValue はOpenAPI生成のenum pointerを文字列へ変換する。
func codeValue[T ~string](value *T) string {
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
