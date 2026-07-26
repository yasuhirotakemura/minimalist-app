// Package tag はタグendpointのHTTP handlerを提供する。
package tag

import (
	"net/http"

	"github.com/google/uuid"
	openapitypes "github.com/oapi-codegen/runtime/types"

	applicationtag "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/tag"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/openapi"
	authhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"
)

// Handler はタグendpointのHTTP handlerである。
type Handler struct {
	listTags  *applicationtag.ListTagsService
	createTag *applicationtag.CreateTagService
	updateTag *applicationtag.UpdateTagService
	deleteTag *applicationtag.DeleteTagService
}

// HandlerDependencies はHandlerの依存。
type HandlerDependencies struct {
	ListTags  *applicationtag.ListTagsService
	CreateTag *applicationtag.CreateTagService
	UpdateTag *applicationtag.UpdateTagService
	DeleteTag *applicationtag.DeleteTagService
}

// NewHandler はHandlerを生成する。
func NewHandler(dependencies HandlerDependencies) *Handler {
	return &Handler{
		listTags:  dependencies.ListTags,
		createTag: dependencies.CreateTag,
		updateTag: dependencies.UpdateTag,
		deleteTag: dependencies.DeleteTag,
	}
}

// ListTags はタグ一覧を取得する。
// GET /api/tags
func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	result, err := h.listTags.Execute(
		r.Context(),
		applicationtag.ListTagsParams{UserID: authenticated.ID},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	tags := make([]openapi.TagResponse, 0, len(result.Tags))
	for _, source := range result.Tags {
		tags = append(tags, toTagResponse(source))
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, openapi.TagListResponse{Items: tags})
}

// CreateTag はタグを登録する。
// POST /api/tags
func (h *Handler) CreateTag(w http.ResponseWriter, r *http.Request) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	var body openapi.CreateTagJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	result, err := h.createTag.Execute(r.Context(), applicationtag.CreateTagParams{
		UserID: authenticated.ID,
		Name:   body.Name,
	})
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusCreated, toTagResponse(result.Tag))
}

// UpdateTag はタグを更新する。
// PUT /api/tags/{publicId}
func (h *Handler) UpdateTag(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	var body openapi.UpdateTagJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	result, err := h.updateTag.Execute(r.Context(), applicationtag.UpdateTagParams{
		UserID:          authenticated.ID,
		PublicID:        uuid.UUID(publicID),
		Name:            body.Name,
		ExpectedVersion: body.ExpectedVersion,
	})
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, toTagResponse(result.Tag))
}

// DeleteTag はタグを削除する。
// DELETE /api/tags/{publicId}
func (h *Handler) DeleteTag(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
	params openapi.DeleteTagParams,
) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	err := h.deleteTag.Execute(r.Context(), applicationtag.DeleteTagParams{
		UserID:          authenticated.ID,
		PublicID:        uuid.UUID(publicID),
		ExpectedVersion: params.ExpectedVersion,
	})
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.WriteNoContent(w)
}

func toTagResponse(result applicationtag.TagResult) openapi.TagResponse {
	return openapi.TagResponse{
		PublicId:  openapitypes.UUID(result.PublicID),
		Name:      result.Name,
		ItemCount: result.ItemCount,
		Version:   result.Version,
		CreatedAt: result.CreatedAt.UTC(),
		UpdatedAt: result.UpdatedAt.UTC(),
	}
}
