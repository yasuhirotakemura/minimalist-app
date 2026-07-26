package item

import (
	"net/http"

	"github.com/google/uuid"
	openapitypes "github.com/oapi-codegen/runtime/types"

	applicationitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/item"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/openapi"
	authhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"
)

// Handler は所持品endpointのHTTP handlerである。
//
// Handlerが行うこと (設計書 11.2): body・path・query取得、Application Service呼出、
// DTO変換、HTTP error変換。
// Handlerが行わないこと (設計書 11.3): SQL、業務ルール、Entityの状態遷移。
type Handler struct {
	createItem           *applicationitem.CreateItemService
	updateItem           *applicationitem.UpdateItemService
	getItem              *applicationitem.GetItemService
	listItems            *applicationitem.ListItemsService
	archiveItem          *applicationitem.ArchiveItemService
	restoreItem          *applicationitem.RestoreItemService
	recordItemUsage      *applicationitem.RecordItemUsageService
	listItemUsageRecords *applicationitem.ListItemUsageRecordsService
}

// HandlerDependencies はHandlerの依存。
type HandlerDependencies struct {
	CreateItem           *applicationitem.CreateItemService
	UpdateItem           *applicationitem.UpdateItemService
	GetItem              *applicationitem.GetItemService
	ListItems            *applicationitem.ListItemsService
	ArchiveItem          *applicationitem.ArchiveItemService
	RestoreItem          *applicationitem.RestoreItemService
	RecordItemUsage      *applicationitem.RecordItemUsageService
	ListItemUsageRecords *applicationitem.ListItemUsageRecordsService
}

// NewHandler はHandlerを生成する。
func NewHandler(dependencies HandlerDependencies) *Handler {
	return &Handler{
		createItem:           dependencies.CreateItem,
		updateItem:           dependencies.UpdateItem,
		getItem:              dependencies.GetItem,
		listItems:            dependencies.ListItems,
		archiveItem:          dependencies.ArchiveItem,
		restoreItem:          dependencies.RestoreItem,
		recordItemUsage:      dependencies.RecordItemUsage,
		listItemUsageRecords: dependencies.ListItemUsageRecords,
	}
}

// ListItems は所持品一覧を取得する。
// GET /api/items
func (h *Handler) ListItems(
	w http.ResponseWriter,
	r *http.Request,
	params openapi.ListItemsParams,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	result, err := h.listItems.Execute(r.Context(), applicationitem.ListItemsParams{
		UserID: authenticated.ID,
		Criteria: domainitem.ListCriteriaInput{
			Keyword:             stringValue(params.Keyword),
			CategoryPublicID:    toUUIDPointer(params.CategoryPublicId),
			TagPublicID:         toUUIDPointer(params.TagPublicId),
			NecessityLevelCode:  codeValue(params.NecessityLevelCode),
			UsageFrequencyCode:  codeValue(params.UsageFrequencyCode),
			MobilityClassCode:   codeValue(params.MobilityClassCode),
			StorageUnitPublicID: toUUIDPointer(params.StorageUnitPublicId),
			IsUnassigned:        booleanValue(params.IsUnassigned),
			IncludeArchived:     booleanValue(params.IncludeDeleted),
			SortKeyName:         codeValue(params.Sort),
			Order:               codeValue(params.Order),
			Limit:               params.Limit,
			Offset:              params.Offset,
		},
	})
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	items := make([]openapi.ItemResponse, 0, len(result.Items))
	for _, source := range result.Items {
		items = append(items, toItemResponse(source))
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, openapi.ItemListResponse{
		Items:      items,
		Pagination: toPaginationResponse(result.Pagination),
	})
}

// CreateItem は所持品を登録する。
// POST /api/items
func (h *Handler) CreateItem(w http.ResponseWriter, r *http.Request) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	var body openapi.CreateItemJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	attributes := toAttributesParams(
		body.Name,
		body.CategoryPublicId,
		body.ItemKindCode,
		body.Quantity,
		body.DesiredQuantity,
		body.UnitName,
		body.NecessityLevelCode,
		body.UsageFrequencyCode,
		body.SubstitutabilityCode,
		body.MobilityClassCode,
		optionalAttributes{
			OwnershipReason:     body.OwnershipReason,
			DisposalCondition:   body.DisposalCondition,
			LastUsedAt:          body.LastUsedAt,
			PurchasedOn:         body.PurchasedOn,
			PurchaseAmount:      body.PurchaseAmount,
			ReplacementAmount:   body.ReplacementAmount,
			ResaleAmount:        body.ResaleAmount,
			WeightGram:          body.WeightGram,
			VolumeMilliliter:    body.VolumeMilliliter,
			IsFragile:           body.IsFragile,
			IsValuable:          body.IsValuable,
			IsSentimental:       body.IsSentimental,
			RequiresMaintenance: body.RequiresMaintenance,
			ExpiresOn:           body.ExpiresOn,
			SourceURL:           body.SourceUrl,
			Notes:               body.Notes,
			TagPublicIDs:        body.TagPublicIds,
		},
	)

	result, err := h.createItem.Execute(r.Context(), applicationitem.CreateItemParams{
		UserID:     authenticated.ID,
		Attributes: attributes,
	})
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusCreated, toItemResponse(result.Item))
}

// GetItemByPublicId は所持品を取得する。
// GET /api/items/{publicId}
func (h *Handler) GetItemByPublicId(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	result, err := h.getItem.Execute(r.Context(), applicationitem.GetItemParams{
		UserID:   authenticated.ID,
		PublicID: uuid.UUID(publicID),
	})
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, toItemResponse(result.Item))
}

// UpdateItem は所持品を更新する。
// PUT /api/items/{publicId}
func (h *Handler) UpdateItem(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	var body openapi.UpdateItemJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	attributes := toAttributesParams(
		body.Name,
		body.CategoryPublicId,
		body.ItemKindCode,
		body.Quantity,
		body.DesiredQuantity,
		body.UnitName,
		body.NecessityLevelCode,
		body.UsageFrequencyCode,
		body.SubstitutabilityCode,
		body.MobilityClassCode,
		optionalAttributes{
			OwnershipReason:     body.OwnershipReason,
			DisposalCondition:   body.DisposalCondition,
			LastUsedAt:          body.LastUsedAt,
			PurchasedOn:         body.PurchasedOn,
			PurchaseAmount:      body.PurchaseAmount,
			ReplacementAmount:   body.ReplacementAmount,
			ResaleAmount:        body.ResaleAmount,
			WeightGram:          body.WeightGram,
			VolumeMilliliter:    body.VolumeMilliliter,
			IsFragile:           body.IsFragile,
			IsValuable:          body.IsValuable,
			IsSentimental:       body.IsSentimental,
			RequiresMaintenance: body.RequiresMaintenance,
			ExpiresOn:           body.ExpiresOn,
			SourceURL:           body.SourceUrl,
			Notes:               body.Notes,
			TagPublicIDs:        body.TagPublicIds,
		},
	)

	result, err := h.updateItem.Execute(r.Context(), applicationitem.UpdateItemParams{
		UserID:          authenticated.ID,
		PublicID:        uuid.UUID(publicID),
		Attributes:      attributes,
		ExpectedVersion: body.ExpectedVersion,
	})
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, toItemResponse(result.Item))
}

// ArchiveItem は所持品をarchiveする。
// POST /api/items/{publicId}/archive
func (h *Handler) ArchiveItem(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	var body openapi.ArchiveItemJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	result, err := h.archiveItem.Execute(r.Context(), applicationitem.ArchiveItemParams{
		UserID:          authenticated.ID,
		PublicID:        uuid.UUID(publicID),
		ExpectedVersion: body.ExpectedVersion,
	})
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, toItemResponse(result.Item))
}

// RestoreItem はarchive済みの所持品を復元する。
// POST /api/items/{publicId}/restore
func (h *Handler) RestoreItem(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	var body openapi.RestoreItemJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	result, err := h.restoreItem.Execute(r.Context(), applicationitem.RestoreItemParams{
		UserID:          authenticated.ID,
		PublicID:        uuid.UUID(publicID),
		ExpectedVersion: body.ExpectedVersion,
	})
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, toItemResponse(result.Item))
}

// CreateItemUsageRecord は使用記録を登録する。
// POST /api/items/{publicId}/usage-records
func (h *Handler) CreateItemUsageRecord(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	var body openapi.CreateItemUsageRecordJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	result, err := h.recordItemUsage.Execute(r.Context(), applicationitem.RecordItemUsageParams{
		UserID:   authenticated.ID,
		PublicID: uuid.UUID(publicID),
		UsedAt:   body.UsedAt,
		Quantity: body.Quantity,
		Note:     body.Note,
	})
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(
		r.Context(), w, http.StatusCreated, toUsageRecordResponse(result.UsageRecord))
}

// ListItemUsageRecords は使用記録の履歴を取得する。
// GET /api/items/{publicId}/usage-records
func (h *Handler) ListItemUsageRecords(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
	params openapi.ListItemUsageRecordsParams,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	result, err := h.listItemUsageRecords.Execute(
		r.Context(),
		applicationitem.ListItemUsageRecordsParams{
			UserID:   authenticated.ID,
			PublicID: uuid.UUID(publicID),
			Limit:    params.Limit,
			Offset:   params.Offset,
		},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	records := make([]openapi.ItemUsageRecordResponse, 0, len(result.Items))
	for _, source := range result.Items {
		records = append(records, toUsageRecordResponse(source))
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, openapi.ItemUsageRecordListResponse{
		Items:      records,
		Pagination: toPaginationResponse(result.Pagination),
	})
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
