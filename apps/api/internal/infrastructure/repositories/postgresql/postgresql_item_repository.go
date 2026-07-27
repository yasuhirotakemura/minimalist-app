package postgresql

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domaincategory "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/category"
	domainitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/item"
	domaintag "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/tag"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/sqlc"
	infrapostgresql "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/postgresql"
)

// PostgreSQLのforeign key違反を表すSQLSTATE。
const foreignKeyViolationCode = "23503"

// sortKeyColumns はDomainの並び替えkeyをDBのcolumn名へ対応付ける。
//
// ORDER BYのCASE式が比較する値と一致させる (sql/queries/items.sql)。
var sortKeyColumns = map[domainitem.SortKey]string{
	domainitem.SortKeyName:      "name",
	domainitem.SortKeyQuantity:  "quantity",
	domainitem.SortKeyUpdatedAt: "updated_at",
}

// PostgresqlItemRepository はItemRepositoryのPostgreSQL実装。
type PostgresqlItemRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresqlItemRepository はPostgresqlItemRepositoryを生成する。
func NewPostgresqlItemRepository(pool *pgxpool.Pool) *PostgresqlItemRepository {
	return &PostgresqlItemRepository{pool: pool}
}

var _ domainitem.ItemRepository = (*PostgresqlItemRepository)(nil)

func (r *PostgresqlItemRepository) queries(ctx context.Context) *sqlc.Queries {
	return sqlc.New(infrapostgresql.Querier(ctx, r.pool))
}

// Create はアイテムとタグ付与を作成する。
//
// 呼び出し元がtransaction境界を制御する。両者は必ず同一transactionで作成する。
func (r *PostgresqlItemRepository) Create(
	ctx context.Context,
	item domainitem.Item,
) (domainitem.Item, error) {
	attributes := item.Attributes()
	queries := r.queries(ctx)

	row, err := queries.InsertItem(ctx, sqlc.InsertItemParams{
		PublicID:           item.PublicID(),
		UserID:             item.UserID().Int64(),
		CategoryID:         attributes.Category.ID.Int64(),
		Name:               attributes.Name,
		ItemKindCode:       attributes.Kind.String(),
		Quantity:           attributes.Quantity,
		UnitName:           attributes.UnitName,
		NecessityLevelCode: attributes.NecessityLevel.String(),
		UsageFrequencyCode: attributes.UsageFrequency.String(),
		PurchasedOn:        nullableDate(attributes.PurchasedOn),
		SourceUrl:          attributes.SourceURL,
		Notes:              attributes.Notes,
		CreatedAt:          timestamptz(item.CreatedAt()),
		UpdatedAt:          timestamptz(item.UpdatedAt()),
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return domainitem.Item{}, domaincategory.ErrCategoryNotFound.WithCause(err)
		}
		return domainitem.Item{}, fmt.Errorf("insert item: %w", err)
	}

	if err := r.replaceTags(ctx, item.UserID(), row.ID, attributes.Tags); err != nil {
		return domainitem.Item{}, err
	}

	return toDomainItem(row, attributes.Category, attributes.Tags), nil
}

// FindByPublicID はアイテムを取得する。archive済みも返す。
func (r *PostgresqlItemRepository) FindByPublicID(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
) (domainitem.Item, error) {
	row, err := r.queries(ctx).FindItemByPublicID(
		ctx,
		sqlc.FindItemByPublicIDParams{PublicID: publicID, UserID: userID.Int64()},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainitem.Item{}, domainitem.ErrItemNotFound
		}
		return domainitem.Item{}, fmt.Errorf("find item by public id: %w", err)
	}

	tagsByItemID, err := r.loadTags(ctx, userID, []int64{row.OwnershipItem.ID})
	if err != nil {
		return domainitem.Item{}, err
	}

	reference := domaincategory.Reference{
		ID:       domaincategory.CategoryID(row.OwnershipItem.CategoryID),
		PublicID: row.CategoryPublicID,
		Name:     row.CategoryName,
	}
	return toDomainItem(row.OwnershipItem, reference, tagsByItemID[row.OwnershipItem.ID]), nil
}

// Update は属性とタグ付与を置き換える。
func (r *PostgresqlItemRepository) Update(
	ctx context.Context,
	item domainitem.Item,
	expectedVersion int32,
) (domainitem.Item, error) {
	attributes := item.Attributes()
	queries := r.queries(ctx)

	row, err := queries.UpdateItem(ctx, sqlc.UpdateItemParams{
		CategoryID:         attributes.Category.ID.Int64(),
		Name:               attributes.Name,
		ItemKindCode:       attributes.Kind.String(),
		Quantity:           attributes.Quantity,
		UnitName:           attributes.UnitName,
		NecessityLevelCode: attributes.NecessityLevel.String(),
		UsageFrequencyCode: attributes.UsageFrequency.String(),
		PurchasedOn:        nullableDate(attributes.PurchasedOn),
		SourceUrl:          attributes.SourceURL,
		Notes:              attributes.Notes,
		UpdatedAt:          timestamptz(item.UpdatedAt()),
		PublicID:           item.PublicID(),
		UserID:             item.UserID().Int64(),
		ExpectedVersion:    expectedVersion,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return domainitem.Item{}, domaincategory.ErrCategoryNotFound.WithCause(err)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return domainitem.Item{}, r.resolveUpdateFailure(ctx, item.UserID(), item.PublicID())
		}
		return domainitem.Item{}, fmt.Errorf("update item: %w", err)
	}

	if err := r.replaceTags(ctx, item.UserID(), row.ID, attributes.Tags); err != nil {
		return domainitem.Item{}, err
	}

	return toDomainItem(row, attributes.Category, attributes.Tags), nil
}

// Archive はarchive (soft delete) する。
func (r *PostgresqlItemRepository) Archive(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
	expectedVersion int32,
	archivedAt time.Time,
) (domainitem.Item, error) {
	_, err := r.queries(ctx).ArchiveItem(ctx, sqlc.ArchiveItemParams{
		ArchivedAt:      timestamptz(archivedAt),
		UpdatedAt:       timestamptz(archivedAt),
		PublicID:        publicID,
		UserID:          userID.Int64(),
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainitem.Item{}, r.resolveUpdateFailure(ctx, userID, publicID)
		}
		return domainitem.Item{}, fmt.Errorf("archive item: %w", err)
	}
	return r.FindByPublicID(ctx, userID, publicID)
}

// Restore はarchiveを解除する。
func (r *PostgresqlItemRepository) Restore(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
	expectedVersion int32,
	now time.Time,
) (domainitem.Item, error) {
	_, err := r.queries(ctx).RestoreItem(ctx, sqlc.RestoreItemParams{
		UpdatedAt:       timestamptz(now),
		PublicID:        publicID,
		UserID:          userID.Int64(),
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainitem.Item{}, r.resolveUpdateFailure(ctx, userID, publicID)
		}
		return domainitem.Item{}, fmt.Errorf("restore item: %w", err)
	}
	return r.FindByPublicID(ctx, userID, publicID)
}

// List は条件に一致するアイテムを返す。
func (r *PostgresqlItemRepository) List(
	ctx context.Context,
	userID domainauth.UserID,
	criteria domainitem.ListCriteria,
) ([]domainitem.Item, error) {
	rows, err := r.queries(ctx).ListItems(ctx, sqlc.ListItemsParams{
		UserID:             userID.Int64(),
		IncludeDeleted:     criteria.IncludeArchived,
		KeywordPattern:     likePattern(criteria.Keyword),
		CategoryPublicID:   criteria.CategoryPublicID,
		NecessityLevelCode: optionalCode(criteria.NecessityLevel),
		UsageFrequencyCode: optionalCode(criteria.UsageFrequency),
		TagPublicID:        criteria.TagPublicID,
		SortKey:            sortKeyColumns[criteria.SortKey],
		Descending:         criteria.Descending,
		RowLimit:           criteria.Limit,
		RowOffset:          criteria.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	itemIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		itemIDs = append(itemIDs, row.OwnershipItem.ID)
	}
	tagsByItemID, err := r.loadTags(ctx, userID, itemIDs)
	if err != nil {
		return nil, err
	}

	items := make([]domainitem.Item, 0, len(rows))
	for _, row := range rows {
		reference := domaincategory.Reference{
			ID:       domaincategory.CategoryID(row.OwnershipItem.CategoryID),
			PublicID: row.CategoryPublicID,
			Name:     row.CategoryName,
		}
		items = append(items,
			toDomainItem(row.OwnershipItem, reference, tagsByItemID[row.OwnershipItem.ID]))
	}
	return items, nil
}

// Count は条件に一致するアイテムの総件数を返す。
func (r *PostgresqlItemRepository) Count(
	ctx context.Context,
	userID domainauth.UserID,
	criteria domainitem.ListCriteria,
) (int64, error) {
	count, err := r.queries(ctx).CountItems(ctx, sqlc.CountItemsParams{
		UserID:             userID.Int64(),
		IncludeDeleted:     criteria.IncludeArchived,
		KeywordPattern:     likePattern(criteria.Keyword),
		CategoryPublicID:   criteria.CategoryPublicID,
		NecessityLevelCode: optionalCode(criteria.NecessityLevel),
		UsageFrequencyCode: optionalCode(criteria.UsageFrequency),
		TagPublicID:        criteria.TagPublicID,
	})
	if err != nil {
		return 0, fmt.Errorf("count items: %w", err)
	}
	return count, nil
}

// AggregateSummary はダッシュボード向けの集計値を返す (設計書 9.3)。
//
// 合計と3種の内訳をそれぞれのGROUP BY queryで取得する。
// 単一queryへまとめるとcategoryとcodeの直積で行数が膨らむため分割する。
// いずれも参照のみで、呼び出し元はtransactionを張らない。
func (r *PostgresqlItemRepository) AggregateSummary(
	ctx context.Context,
	userID domainauth.UserID,
) (domainitem.SummaryTotals, error) {
	queries := r.queries(ctx)

	totals, err := queries.AggregateItemTotals(ctx, userID.Int64())
	if err != nil {
		return domainitem.SummaryTotals{}, fmt.Errorf("aggregate item totals: %w", err)
	}

	categoryRows, err := queries.AggregateItemCountsByCategory(ctx, userID.Int64())
	if err != nil {
		return domainitem.SummaryTotals{}, fmt.Errorf("aggregate item counts by category: %w", err)
	}

	necessityRows, err := queries.AggregateItemCountsByNecessityLevel(ctx, userID.Int64())
	if err != nil {
		return domainitem.SummaryTotals{}, fmt.Errorf(
			"aggregate item counts by necessity level: %w", err)
	}

	frequencyRows, err := queries.AggregateItemCountsByUsageFrequency(ctx, userID.Int64())
	if err != nil {
		return domainitem.SummaryTotals{}, fmt.Errorf(
			"aggregate item counts by usage frequency: %w", err)
	}

	summary := domainitem.SummaryTotals{
		Total: domainitem.Counts{
			TypeCount:     totals.ItemTypeCount,
			TotalQuantity: totals.TotalQuantity,
		},
		ByCategory:           make([]domainitem.CategoryCounts, 0, len(categoryRows)),
		ByNecessityLevelCode: make(map[string]domainitem.Counts, len(necessityRows)),
		ByUsageFrequencyCode: make(map[string]domainitem.Counts, len(frequencyRows)),
	}

	for _, row := range categoryRows {
		summary.ByCategory = append(summary.ByCategory, domainitem.CategoryCounts{
			Category: domaincategory.Reference{
				PublicID: row.CategoryPublicID,
				Name:     row.CategoryName,
			},
			Counts: domainitem.Counts{
				TypeCount:     row.ItemTypeCount,
				TotalQuantity: row.TotalQuantity,
			},
		})
	}

	for _, row := range necessityRows {
		summary.ByNecessityLevelCode[row.NecessityLevelCode] = domainitem.Counts{
			TypeCount:     row.ItemTypeCount,
			TotalQuantity: row.TotalQuantity,
		}
	}

	for _, row := range frequencyRows {
		summary.ByUsageFrequencyCode[row.UsageFrequencyCode] = domainitem.Counts{
			TypeCount:     row.ItemTypeCount,
			TotalQuantity: row.TotalQuantity,
		}
	}

	return summary, nil
}

// replaceTags はアイテムのタグ付与を指定内容へ置き換える。
func (r *PostgresqlItemRepository) replaceTags(
	ctx context.Context,
	userID domainauth.UserID,
	itemID int64,
	references []domaintag.Reference,
) error {
	queries := r.queries(ctx)

	if err := queries.DeleteItemTagsByItemID(ctx, sqlc.DeleteItemTagsByItemIDParams{
		UserID: userID.Int64(),
		ItemID: itemID,
	}); err != nil {
		return fmt.Errorf("delete item tags: %w", err)
	}
	if len(references) == 0 {
		return nil
	}

	tagIDs := make([]int64, 0, len(references))
	for _, reference := range references {
		tagIDs = append(tagIDs, reference.ID.Int64())
	}
	if err := queries.InsertItemTags(ctx, sqlc.InsertItemTagsParams{
		UserID: userID.Int64(),
		ItemID: itemID,
		TagIds: tagIDs,
	}); err != nil {
		if isForeignKeyViolation(err) {
			return domaintag.ErrTagNotFound.WithCause(err)
		}
		return fmt.Errorf("insert item tags: %w", err)
	}
	return nil
}

// loadTags は複数アイテムのタグをまとめて取得し、N+1 queryを避ける。
func (r *PostgresqlItemRepository) loadTags(
	ctx context.Context,
	userID domainauth.UserID,
	itemIDs []int64,
) (map[int64][]domaintag.Reference, error) {
	if len(itemIDs) == 0 {
		return map[int64][]domaintag.Reference{}, nil
	}

	rows, err := r.queries(ctx).ListItemTagsByItemIDs(ctx, sqlc.ListItemTagsByItemIDsParams{
		UserID:  userID.Int64(),
		ItemIds: itemIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list item tags: %w", err)
	}

	tagsByItemID := make(map[int64][]domaintag.Reference, len(itemIDs))
	for _, row := range rows {
		tagsByItemID[row.ItemID] = append(tagsByItemID[row.ItemID], domaintag.Reference{
			PublicID: row.PublicID,
			Name:     row.Name,
		})
	}
	return tagsByItemID, nil
}

// resolveUpdateFailure は更新件数0の理由を判定する。
func (r *PostgresqlItemRepository) resolveUpdateFailure(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
) error {
	exists, err := r.queries(ctx).ExistsItemByPublicID(
		ctx,
		sqlc.ExistsItemByPublicIDParams{PublicID: publicID, UserID: userID.Int64()},
	)
	if err != nil {
		return fmt.Errorf("check item existence: %w", err)
	}
	if !exists {
		return domainitem.ErrItemNotFound
	}
	return domainitem.ErrItemVersionConflict
}

func toDomainItem(
	row sqlc.OwnershipItem,
	categoryReference domaincategory.Reference,
	tags []domaintag.Reference,
) domainitem.Item {
	return domainitem.ReconstructItem(domainitem.ReconstructItemParams{
		ID:       domainitem.ItemID(row.ID),
		PublicID: row.PublicID,
		UserID:   domainauth.UserID(row.UserID),
		Attributes: domainitem.Attributes{
			Name:           row.Name,
			Category:       categoryReference,
			Kind:           domainitem.ItemKind(row.ItemKindCode),
			Quantity:       row.Quantity,
			UnitName:       row.UnitName,
			NecessityLevel: domainitem.NecessityLevel(row.NecessityLevelCode),
			UsageFrequency: domainitem.UsageFrequency(row.UsageFrequencyCode),
			PurchasedOn:    optionalDate(row.PurchasedOn),
			SourceURL:      row.SourceUrl,
			Notes:          row.Notes,
			Tags:           tags,
		},
		CreatedAt:  utcTime(row.CreatedAt),
		UpdatedAt:  utcTime(row.UpdatedAt),
		ArchivedAt: optionalTime(row.DeletedAt),
		Version:    row.Version,
	})
}

// likePattern はkeywordをILIKEのpatternへ変換する。
//
// keyword内のワイルドカード (% _) とescape文字を無効化し、
// 利用者の入力が意図しない全件一致にならないようにする。
func likePattern(keyword string) *string {
	if keyword == "" {
		return nil
	}
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(keyword)
	pattern := "%" + escaped + "%"
	return &pattern
}

// optionalCode はDomainのcode ValueObjectをNULL許容のqueryパラメータへ変換する。
func optionalCode[T ~string](value *T) *string {
	if value == nil {
		return nil
	}
	code := string(*value)
	return &code
}

// isForeignKeyViolation はforeign key制約違反かどうかを返す。
//
// composite foreign key (user_id, category_id) / (user_id, tag_id) により、
// 他ユーザーのresourceを参照した場合も本違反となる。
func isForeignKeyViolation(err error) bool {
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) {
		return false
	}
	return pgError.Code == foreignKeyViolationCode
}
