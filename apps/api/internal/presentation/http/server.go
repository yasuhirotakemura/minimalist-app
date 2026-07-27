package http

import (
	"net/http"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/openapi"
	authhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/auth"
	categoryhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/category"
	itemhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/item"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"
	taghttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/tag"
)

// APIServer はfeatureごとのHandlerを束ねてOpenAPIのServerInterfaceを満たす。
//
// featureごとのHandler型はいずれも `Handler` という名前のため、embedすると
// field名が衝突する。そのため名前付きfieldとし、委譲methodを明示する。
//
// 本型がServerInterfaceを満たすことをcompile時に確認することで、
// OpenAPIへendpointを追加した際に実装漏れがbuild errorとなる。
type APIServer struct {
	auth     *authhttp.Handler
	category *categoryhttp.Handler
	item     *itemhttp.Handler
	tag      *taghttp.Handler
}

// NewAPIServer はAPIServerを生成する。
func NewAPIServer(
	auth *authhttp.Handler,
	category *categoryhttp.Handler,
	item *itemhttp.Handler,
	tag *taghttp.Handler,
) *APIServer {
	return &APIServer{
		auth:     auth,
		category: category,
		item:     item,
		tag:      tag,
	}
}

var _ openapi.ServerInterface = (*APIServer)(nil)

// --- auth -------------------------------------------------------------------

// RegisterUser はユーザーを登録する。
func (s *APIServer) RegisterUser(w http.ResponseWriter, r *http.Request) {
	s.auth.RegisterUser(w, r)
}

// LoginUser はloginする。
func (s *APIServer) LoginUser(w http.ResponseWriter, r *http.Request) {
	s.auth.LoginUser(w, r)
}

// LogoutUser はlogoutする。
func (s *APIServer) LogoutUser(w http.ResponseWriter, r *http.Request) {
	s.auth.LogoutUser(w, r)
}

// GetAuthenticatedUserContext は認証contextを取得する。
func (s *APIServer) GetAuthenticatedUserContext(w http.ResponseWriter, r *http.Request) {
	s.auth.GetAuthenticatedUserContext(w, r)
}

// --- category ---------------------------------------------------------------

// ListCategories はカテゴリー一覧を取得する。
func (s *APIServer) ListCategories(w http.ResponseWriter, r *http.Request) {
	s.category.ListCategories(w, r)
}

// --- item -------------------------------------------------------------------

// ListItems は所持品一覧を取得する。
func (s *APIServer) ListItems(
	w http.ResponseWriter, r *http.Request, params openapi.ListItemsParams,
) {
	s.item.ListItems(w, r, params)
}

// CreateItem は所持品を登録する。
func (s *APIServer) CreateItem(w http.ResponseWriter, r *http.Request) {
	s.item.CreateItem(w, r)
}

// GetItemByPublicId は所持品を取得する。
func (s *APIServer) GetItemByPublicId(
	w http.ResponseWriter, r *http.Request, publicID openapi.PublicIdPathParameter,
) {
	s.item.GetItemByPublicId(w, r, publicID)
}

// UpdateItem は所持品を更新する。
func (s *APIServer) UpdateItem(
	w http.ResponseWriter, r *http.Request, publicID openapi.PublicIdPathParameter,
) {
	s.item.UpdateItem(w, r, publicID)
}

// ArchiveItem は所持品をarchiveする。
func (s *APIServer) ArchiveItem(
	w http.ResponseWriter, r *http.Request, publicID openapi.PublicIdPathParameter,
) {
	s.item.ArchiveItem(w, r, publicID)
}

// RestoreItem はarchive済みの所持品を復元する。
func (s *APIServer) RestoreItem(
	w http.ResponseWriter, r *http.Request, publicID openapi.PublicIdPathParameter,
) {
	s.item.RestoreItem(w, r, publicID)
}

// --- dashboard --------------------------------------------------------------

// GetDashboardSummary はダッシュボードの集計値を取得する。
func (s *APIServer) GetDashboardSummary(w http.ResponseWriter, r *http.Request) {
	s.item.GetDashboardSummary(w, r)
}

// --- tag --------------------------------------------------------------------

// ListTags はタグ一覧を取得する。
func (s *APIServer) ListTags(w http.ResponseWriter, r *http.Request) {
	s.tag.ListTags(w, r)
}

// CreateTag はタグを登録する。
func (s *APIServer) CreateTag(w http.ResponseWriter, r *http.Request) {
	s.tag.CreateTag(w, r)
}

// UpdateTag はタグを更新する。
func (s *APIServer) UpdateTag(
	w http.ResponseWriter, r *http.Request, publicID openapi.PublicIdPathParameter,
) {
	s.tag.UpdateTag(w, r, publicID)
}

// DeleteTag はタグを削除する。
func (s *APIServer) DeleteTag(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
	params openapi.DeleteTagParams,
) {
	s.tag.DeleteTag(w, r, publicID, params)
}

// newServerWrapper はOpenAPI生成のparameter binding付きhandlerを返す。
//
// routeごとに異なるmiddlewareを適用するため、生成された HandlerFromMux ではなく
// ServerInterfaceWrapper を使い、routeの登録はrouter.goで明示的に行う。
func newServerWrapper(server openapi.ServerInterface) *openapi.ServerInterfaceWrapper {
	return &openapi.ServerInterfaceWrapper{
		Handler:          server,
		ErrorHandlerFunc: writeParameterBindingError,
	}
}

// writeParameterBindingError はpath・query parameterの解釈失敗を400へ変換する。
func writeParameterBindingError(w http.ResponseWriter, r *http.Request, err error) {
	shared.WriteError(w, r, shared.NewParameterBindingError(err))
}
