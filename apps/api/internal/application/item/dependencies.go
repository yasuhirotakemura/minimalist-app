// Package item は所持品のユースケースを実装する。
//
// Application Serviceの責務 (設計書 11.4):
//   - Repository呼出
//   - Entity生成 / ValueObject生成
//   - transaction制御
//   - 複数処理の順序制御
//   - Domain errorの伝播
//   - audit log記録
//
// HTTP status codeやSQLをここへ書かない。
package item

import (
	"context"
	"time"

	"github.com/google/uuid"

	applicationaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/audit"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domaincategory "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/category"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domaintag "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/tag"
)

// Dependencies は所持品ユースケースが必要とする依存をまとめる。
type Dependencies struct {
	Items         domainitem.ItemRepository
	Categories    domaincategory.CategoryRepository
	Tags          domaintag.TagRepository
	AuditRecorder *applicationaudit.Recorder
}

// AttributesParams は利用者が指定できる所持品の属性。
//
// codeは文字列のまま受け取り、ValueObjectへの変換はDomainが行う。
// これによりpresentation layerがDomainのcode体系へ依存しない。
type AttributesParams struct {
	Name               string
	CategoryPublicID   uuid.UUID
	ItemKindCode       string
	Quantity           int32
	UnitName           string
	NecessityLevelCode string
	UsageFrequencyCode string
	PurchasedOn        *time.Time
	SourceURL          *string
	Notes              *string
	TagPublicIDs       []uuid.UUID
}

// resolveAttributes はカテゴリーとタグを解決し、Domainの属性を組み立てる。
//
// 存在しないカテゴリー・タグ、および他ユーザーのカテゴリー・タグを指定した場合は
// それぞれ ErrCategoryNotFound / ErrTagNotFound を返す (設計書 18.3)。
func (d Dependencies) resolveAttributes(
	ctx context.Context,
	userID domainauth.UserID,
	params AttributesParams,
) (domainitem.Attributes, error) {
	resolvedCategory, err := d.Categories.FindActiveByPublicID(
		ctx, userID, params.CategoryPublicID)
	if err != nil {
		return domainitem.Attributes{}, err
	}

	tagReferences, err := d.Tags.ResolveActiveReferences(ctx, userID, params.TagPublicIDs)
	if err != nil {
		return domainitem.Attributes{}, err
	}

	return domainitem.Attributes{
		Name:           params.Name,
		Category:       resolvedCategory.Reference(),
		Kind:           domainitem.ItemKind(params.ItemKindCode),
		Quantity:       params.Quantity,
		UnitName:       params.UnitName,
		NecessityLevel: domainitem.NecessityLevel(params.NecessityLevelCode),
		UsageFrequency: domainitem.UsageFrequency(params.UsageFrequencyCode),
		PurchasedOn:    params.PurchasedOn,
		SourceURL:      params.SourceURL,
		Notes:          params.Notes,
		Tags:           tagReferences,
	}, nil
}
