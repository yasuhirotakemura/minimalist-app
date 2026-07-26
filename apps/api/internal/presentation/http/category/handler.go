// Package category はカテゴリーendpointのHTTP handlerを提供する。
package category

import (
	"net/http"

	openapitypes "github.com/oapi-codegen/runtime/types"

	applicationcategory "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/category"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/openapi"
	authhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"
)

// Handler はカテゴリーendpointのHTTP handlerである。
type Handler struct {
	listCategories *applicationcategory.ListCategoriesService
}

// HandlerDependencies はHandlerの依存。
type HandlerDependencies struct {
	ListCategories *applicationcategory.ListCategoriesService
}

// NewHandler はHandlerを生成する。
func NewHandler(dependencies HandlerDependencies) *Handler {
	return &Handler{listCategories: dependencies.ListCategories}
}

// ListCategories はカテゴリー一覧を取得する。
// GET /api/categories
func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	authenticated, ok := authhttp.AuthenticatedUserFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, shared.ErrAuthenticationMiddlewareMissing)
		return
	}

	result, err := h.listCategories.Execute(
		r.Context(),
		applicationcategory.ListCategoriesParams{UserID: authenticated.ID},
	)
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	categories := make([]openapi.CategoryResponse, 0, len(result.Categories))
	for _, source := range result.Categories {
		categories = append(categories, openapi.CategoryResponse{
			PublicId:    openapitypes.UUID(source.PublicID),
			Name:        source.Name,
			Description: source.Description,
			SortOrder:   source.SortOrder,
			Version:     source.Version,
			CreatedAt:   source.CreatedAt.UTC(),
			UpdatedAt:   source.UpdatedAt.UTC(),
		})
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, openapi.CategoryListResponse{
		Items: categories,
	})
}
