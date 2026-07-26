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
	domainstorage "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/storage"
	domaintag "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/tag"
)

// Dependencies は所持品ユースケースが必要とする依存をまとめる。
type Dependencies struct {
	Items        domainitem.ItemRepository
	UsageRecords domainitem.ItemUsageRecordRepository
	Categories   domaincategory.CategoryRepository
	Tags         domaintag.TagRepository
	// StorageAllocations は収納割当の読み取りと数量整合性の検証に使用する (Phase 2)。
	// 所持品の応答は収納情報と未割当数量を含むため、Item Aggregateの外側にある
	// 収納割当をApplication層で合成する。
	StorageAllocations domainstorage.StorageAllocationRepository
	AuditRecorder      *applicationaudit.Recorder
}

// newItemResultWithStorage は1件の所持品へ収納割当を付与した結果を返す。
func (d Dependencies) newItemResultWithStorage(
	ctx context.Context,
	userID domainauth.UserID,
	source domainitem.Item,
) (ItemResult, error) {
	allocations, err := d.StorageAllocations.ListByItemID(ctx, userID, source.ID())
	if err != nil {
		return ItemResult{}, err
	}
	return newItemResult(source).withStorageAllocations(allocations), nil
}

// newItemResultsWithStorage は一覧の所持品へ収納割当をまとめて付与する。
//
// 割当は1度のqueryでまとめて取得し、N+1 queryを避ける。
func (d Dependencies) newItemResultsWithStorage(
	ctx context.Context,
	userID domainauth.UserID,
	sources []domainitem.Item,
) ([]ItemResult, error) {
	itemIDs := make([]domainitem.ItemID, 0, len(sources))
	for _, source := range sources {
		itemIDs = append(itemIDs, source.ID())
	}

	allocationsByItemID, err := d.StorageAllocations.ListByItemIDs(ctx, userID, itemIDs)
	if err != nil {
		return nil, err
	}

	results := make([]ItemResult, 0, len(sources))
	for _, source := range sources {
		results = append(results,
			newItemResult(source).withStorageAllocations(allocationsByItemID[source.ID()]))
	}
	return results, nil
}

// AttributesParams は利用者が指定できる所持品の属性。
//
// codeは文字列のまま受け取り、ValueObjectへの変換はDomainが行う。
// これによりpresentation layerがDomainのcode体系へ依存しない。
type AttributesParams struct {
	Name                 string
	CategoryPublicID     uuid.UUID
	ItemKindCode         string
	Quantity             int32
	DesiredQuantity      *int32
	UnitName             string
	NecessityLevelCode   string
	UsageFrequencyCode   string
	SubstitutabilityCode string
	MobilityClassCode    string
	OwnershipReason      *string
	DisposalCondition    *string
	LastUsedAt           *time.Time
	PurchasedOn          *time.Time
	PurchaseAmount       *int64
	ReplacementAmount    *int64
	ResaleAmount         *int64
	WeightGram           *int32
	VolumeMilliliter     *int32
	IsFragile            bool
	IsValuable           bool
	IsSentimental        bool
	RequiresMaintenance  bool
	ExpiresOn            *time.Time
	SourceURL            *string
	Notes                *string
	TagPublicIDs         []uuid.UUID
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
		Name:                params.Name,
		Category:            resolvedCategory.Reference(),
		Kind:                domainitem.ItemKind(params.ItemKindCode),
		Quantity:            params.Quantity,
		DesiredQuantity:     params.DesiredQuantity,
		UnitName:            params.UnitName,
		NecessityLevel:      domainitem.NecessityLevel(params.NecessityLevelCode),
		UsageFrequency:      domainitem.UsageFrequency(params.UsageFrequencyCode),
		Substitutability:    domainitem.Substitutability(params.SubstitutabilityCode),
		MobilityClass:       domainitem.MobilityClass(params.MobilityClassCode),
		OwnershipReason:     params.OwnershipReason,
		DisposalCondition:   params.DisposalCondition,
		LastUsedAt:          params.LastUsedAt,
		PurchasedOn:         params.PurchasedOn,
		PurchaseAmount:      params.PurchaseAmount,
		ReplacementAmount:   params.ReplacementAmount,
		ResaleAmount:        params.ResaleAmount,
		WeightGram:          params.WeightGram,
		VolumeMilliliter:    params.VolumeMilliliter,
		IsFragile:           params.IsFragile,
		IsValuable:          params.IsValuable,
		IsSentimental:       params.IsSentimental,
		RequiresMaintenance: params.RequiresMaintenance,
		ExpiresOn:           params.ExpiresOn,
		SourceURL:           params.SourceURL,
		Notes:               params.Notes,
		Tags:                tagReferences,
	}, nil
}
