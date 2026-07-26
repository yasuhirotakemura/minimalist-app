package http

import (
	"net/http"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/openapi"
	authhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/auth"
	categoryhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/category"
	itemhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/item"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"
	storagehttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/storage"
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
	storage  *storagehttp.Handler
	tag      *taghttp.Handler
}

// NewAPIServer はAPIServerを生成する。
func NewAPIServer(
	auth *authhttp.Handler,
	category *categoryhttp.Handler,
	item *itemhttp.Handler,
	storage *storagehttp.Handler,
	tag *taghttp.Handler,
) *APIServer {
	return &APIServer{
		auth:     auth,
		category: category,
		item:     item,
		storage:  storage,
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

// CreateItemUsageRecord は使用記録を登録する。
func (s *APIServer) CreateItemUsageRecord(
	w http.ResponseWriter, r *http.Request, publicID openapi.PublicIdPathParameter,
) {
	s.item.CreateItemUsageRecord(w, r, publicID)
}

// ListItemUsageRecords は使用記録の履歴を取得する。
func (s *APIServer) ListItemUsageRecords(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
	params openapi.ListItemUsageRecordsParams,
) {
	s.item.ListItemUsageRecords(w, r, publicID, params)
}

// --- storage ----------------------------------------------------------------

// ListStorageUnits は収納単位一覧を取得する。
func (s *APIServer) ListStorageUnits(
	w http.ResponseWriter, r *http.Request, params openapi.ListStorageUnitsParams,
) {
	s.storage.ListStorageUnits(w, r, params)
}

// CreateStorageUnit は収納単位を登録する。
func (s *APIServer) CreateStorageUnit(w http.ResponseWriter, r *http.Request) {
	s.storage.CreateStorageUnit(w, r)
}

// GetStorageUnitByPublicId は収納単位を取得する。
func (s *APIServer) GetStorageUnitByPublicId(
	w http.ResponseWriter, r *http.Request, publicID openapi.PublicIdPathParameter,
) {
	s.storage.GetStorageUnitByPublicId(w, r, publicID)
}

// UpdateStorageUnit は収納単位を更新する。
func (s *APIServer) UpdateStorageUnit(
	w http.ResponseWriter, r *http.Request, publicID openapi.PublicIdPathParameter,
) {
	s.storage.UpdateStorageUnit(w, r, publicID)
}

// ArchiveStorageUnit は収納単位をarchiveする。
func (s *APIServer) ArchiveStorageUnit(
	w http.ResponseWriter, r *http.Request, publicID openapi.PublicIdPathParameter,
) {
	s.storage.ArchiveStorageUnit(w, r, publicID)
}

// RestoreStorageUnit はarchive済みの収納単位を復元する。
func (s *APIServer) RestoreStorageUnit(
	w http.ResponseWriter, r *http.Request, publicID openapi.PublicIdPathParameter,
) {
	s.storage.RestoreStorageUnit(w, r, publicID)
}

// GetStorageUnitCapacity は収納単位の重量・容積を取得する。
func (s *APIServer) GetStorageUnitCapacity(
	w http.ResponseWriter, r *http.Request, publicID openapi.PublicIdPathParameter,
) {
	s.storage.GetStorageUnitCapacity(w, r, publicID)
}

// GetStorageUnitContents は収納単位の内容を取得する。
func (s *APIServer) GetStorageUnitContents(
	w http.ResponseWriter, r *http.Request, publicID openapi.PublicIdPathParameter,
) {
	s.storage.GetStorageUnitContents(w, r, publicID)
}

// CreateStorageAllocation は所持品を収納単位へ割り当てる。
func (s *APIServer) CreateStorageAllocation(
	w http.ResponseWriter, r *http.Request, publicID openapi.PublicIdPathParameter,
) {
	s.storage.CreateStorageAllocation(w, r, publicID)
}

// SetStorageUnitAllocations は収納単位の割当を一括置換する。
func (s *APIServer) SetStorageUnitAllocations(
	w http.ResponseWriter, r *http.Request, publicID openapi.PublicIdPathParameter,
) {
	s.storage.SetStorageUnitAllocations(w, r, publicID)
}

// UpdateStorageAllocation は収納割当の数量を変更する。
func (s *APIServer) UpdateStorageAllocation(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
	allocationPublicID openapi.AllocationPublicIdPathParameter,
) {
	s.storage.UpdateStorageAllocation(w, r, publicID, allocationPublicID)
}

// DeleteStorageAllocation は収納割当を削除する。
func (s *APIServer) DeleteStorageAllocation(
	w http.ResponseWriter,
	r *http.Request,
	publicID openapi.PublicIdPathParameter,
	allocationPublicID openapi.AllocationPublicIdPathParameter,
	params openapi.DeleteStorageAllocationParams,
) {
	s.storage.DeleteStorageAllocation(w, r, publicID, allocationPublicID, params)
}

// ListItemStorageAllocations は所持品の収納割当を取得する。
func (s *APIServer) ListItemStorageAllocations(
	w http.ResponseWriter, r *http.Request, publicID openapi.PublicIdPathParameter,
) {
	s.storage.ListItemStorageAllocations(w, r, publicID)
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
