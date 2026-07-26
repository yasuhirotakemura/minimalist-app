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
	domainitem.SortKeyName:       "name",
	domainitem.SortKeyQuantity:   "quantity",
	domainitem.SortKeyLastUsedAt: "last_used_at",
	domainitem.SortKeyUpdatedAt:  "updated_at",
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
		PublicID:             item.PublicID(),
		UserID:               item.UserID().Int64(),
		CategoryID:           attributes.Category.ID.Int64(),
		Name:                 attributes.Name,
		ItemKindCode:         attributes.Kind.String(),
		Quantity:             attributes.Quantity,
		DesiredQuantity:      attributes.DesiredQuantity,
		UnitName:             attributes.UnitName,
		NecessityLevelCode:   attributes.NecessityLevel.String(),
		UsageFrequencyCode:   attributes.UsageFrequency.String(),
		SubstitutabilityCode: attributes.Substitutability.String(),
		MobilityClassCode:    attributes.MobilityClass.String(),
		OwnershipReason:      attributes.OwnershipReason,
		DisposalCondition:    attributes.DisposalCondition,
		LastUsedAt:           nullableTimestamptz(attributes.LastUsedAt),
		PurchasedOn:          nullableDate(attributes.PurchasedOn),
		PurchaseAmount:       attributes.PurchaseAmount,
		ReplacementAmount:    attributes.ReplacementAmount,
		ResaleAmount:         attributes.ResaleAmount,
		WeightGram:           attributes.WeightGram,
		VolumeMilliliter:     attributes.VolumeMilliliter,
		IsFragile:            attributes.IsFragile,
		IsValuable:           attributes.IsValuable,
		IsSentimental:        attributes.IsSentimental,
		RequiresMaintenance:  attributes.RequiresMaintenance,
		ExpiresOn:            nullableDate(attributes.ExpiresOn),
		SourceUrl:            attributes.SourceURL,
		Notes:                attributes.Notes,
		IsConfirmed:          item.IsConfirmed(),
		ConfirmedAt:          nullableTimestamptz(item.ConfirmedAt()),
		CreatedAt:            timestamptz(item.CreatedAt()),
		UpdatedAt:            timestamptz(item.UpdatedAt()),
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
		CategoryID:           attributes.Category.ID.Int64(),
		Name:                 attributes.Name,
		ItemKindCode:         attributes.Kind.String(),
		Quantity:             attributes.Quantity,
		DesiredQuantity:      attributes.DesiredQuantity,
		UnitName:             attributes.UnitName,
		NecessityLevelCode:   attributes.NecessityLevel.String(),
		UsageFrequencyCode:   attributes.UsageFrequency.String(),
		SubstitutabilityCode: attributes.Substitutability.String(),
		MobilityClassCode:    attributes.MobilityClass.String(),
		OwnershipReason:      attributes.OwnershipReason,
		DisposalCondition:    attributes.DisposalCondition,
		LastUsedAt:           nullableTimestamptz(attributes.LastUsedAt),
		PurchasedOn:          nullableDate(attributes.PurchasedOn),
		PurchaseAmount:       attributes.PurchaseAmount,
		ReplacementAmount:    attributes.ReplacementAmount,
		ResaleAmount:         attributes.ResaleAmount,
		WeightGram:           attributes.WeightGram,
		VolumeMilliliter:     attributes.VolumeMilliliter,
		IsFragile:            attributes.IsFragile,
		IsValuable:           attributes.IsValuable,
		IsSentimental:        attributes.IsSentimental,
		RequiresMaintenance:  attributes.RequiresMaintenance,
		ExpiresOn:            nullableDate(attributes.ExpiresOn),
		SourceUrl:            attributes.SourceURL,
		Notes:                attributes.Notes,
		UpdatedAt:            timestamptz(item.UpdatedAt()),
		PublicID:             item.PublicID(),
		UserID:               item.UserID().Int64(),
		ExpectedVersion:      expectedVersion,
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

// TouchLastUsedAt は最終使用日時を更新する。
func (r *PostgresqlItemRepository) TouchLastUsedAt(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
	usedAt time.Time,
	now time.Time,
) (domainitem.Item, error) {
	_, err := r.queries(ctx).TouchItemLastUsedAt(ctx, sqlc.TouchItemLastUsedAtParams{
		UsedAt:    timestamptz(usedAt),
		UpdatedAt: timestamptz(now),
		PublicID:  publicID,
		UserID:    userID.Int64(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainitem.Item{}, domainitem.ErrItemNotFound
		}
		return domainitem.Item{}, fmt.Errorf("touch item last used at: %w", err)
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
		MobilityClassCode:  optionalCode(criteria.MobilityClass),
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
		MobilityClassCode:  optionalCode(criteria.MobilityClass),
		TagPublicID:        criteria.TagPublicID,
	})
	if err != nil {
		return 0, fmt.Errorf("count items: %w", err)
	}
	return count, nil
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

// PostgresqlItemUsageRecordRepository はItemUsageRecordRepositoryのPostgreSQL実装。
type PostgresqlItemUsageRecordRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresqlItemUsageRecordRepository はPostgresqlItemUsageRecordRepositoryを生成する。
func NewPostgresqlItemUsageRecordRepository(
	pool *pgxpool.Pool,
) *PostgresqlItemUsageRecordRepository {
	return &PostgresqlItemUsageRecordRepository{pool: pool}
}

var _ domainitem.ItemUsageRecordRepository = (*PostgresqlItemUsageRecordRepository)(nil)

func (r *PostgresqlItemUsageRecordRepository) queries(ctx context.Context) *sqlc.Queries {
	return sqlc.New(infrapostgresql.Querier(ctx, r.pool))
}

// Create は使用記録を作成する。
func (r *PostgresqlItemUsageRecordRepository) Create(
	ctx context.Context,
	record domainitem.UsageRecord,
) (domainitem.UsageRecord, error) {
	row, err := r.queries(ctx).InsertItemUsageRecord(ctx, sqlc.InsertItemUsageRecordParams{
		PublicID:  record.PublicID(),
		UserID:    record.UserID().Int64(),
		ItemID:    record.ItemID().Int64(),
		UsedAt:    timestamptz(record.UsedAt()),
		Quantity:  record.Quantity(),
		Note:      record.Note(),
		CreatedAt: timestamptz(record.CreatedAt()),
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return domainitem.UsageRecord{}, domainitem.ErrItemNotFound.WithCause(err)
		}
		return domainitem.UsageRecord{}, fmt.Errorf("insert item usage record: %w", err)
	}
	return toDomainUsageRecord(row), nil
}

// ListByItemID は使用日時の降順で履歴を返す。
func (r *PostgresqlItemUsageRecordRepository) ListByItemID(
	ctx context.Context,
	userID domainauth.UserID,
	itemID domainitem.ItemID,
	page domainitem.PageCriteria,
) ([]domainitem.UsageRecord, error) {
	rows, err := r.queries(ctx).ListItemUsageRecordsByItemID(
		ctx,
		sqlc.ListItemUsageRecordsByItemIDParams{
			UserID:    userID.Int64(),
			ItemID:    itemID.Int64(),
			RowLimit:  page.Limit,
			RowOffset: page.Offset,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("list item usage records: %w", err)
	}

	records := make([]domainitem.UsageRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, toDomainUsageRecord(row))
	}
	return records, nil
}

// CountByItemID は履歴の総件数を返す。
func (r *PostgresqlItemUsageRecordRepository) CountByItemID(
	ctx context.Context,
	userID domainauth.UserID,
	itemID domainitem.ItemID,
) (int64, error) {
	count, err := r.queries(ctx).CountItemUsageRecordsByItemID(
		ctx,
		sqlc.CountItemUsageRecordsByItemIDParams{
			UserID: userID.Int64(),
			ItemID: itemID.Int64(),
		},
	)
	if err != nil {
		return 0, fmt.Errorf("count item usage records: %w", err)
	}
	return count, nil
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
			Name:                row.Name,
			Category:            categoryReference,
			Kind:                domainitem.ItemKind(row.ItemKindCode),
			Quantity:            row.Quantity,
			DesiredQuantity:     row.DesiredQuantity,
			UnitName:            row.UnitName,
			NecessityLevel:      domainitem.NecessityLevel(row.NecessityLevelCode),
			UsageFrequency:      domainitem.UsageFrequency(row.UsageFrequencyCode),
			Substitutability:    domainitem.Substitutability(row.SubstitutabilityCode),
			MobilityClass:       domainitem.MobilityClass(row.MobilityClassCode),
			OwnershipReason:     row.OwnershipReason,
			DisposalCondition:   row.DisposalCondition,
			LastUsedAt:          optionalTime(row.LastUsedAt),
			PurchasedOn:         optionalDate(row.PurchasedOn),
			PurchaseAmount:      row.PurchaseAmount,
			ReplacementAmount:   row.ReplacementAmount,
			ResaleAmount:        row.ResaleAmount,
			WeightGram:          row.WeightGram,
			VolumeMilliliter:    row.VolumeMilliliter,
			IsFragile:           row.IsFragile,
			IsValuable:          row.IsValuable,
			IsSentimental:       row.IsSentimental,
			RequiresMaintenance: row.RequiresMaintenance,
			ExpiresOn:           optionalDate(row.ExpiresOn),
			SourceURL:           row.SourceUrl,
			Notes:               row.Notes,
			Tags:                tags,
		},
		IsConfirmed: row.IsConfirmed,
		ConfirmedAt: optionalTime(row.ConfirmedAt),
		CreatedAt:   utcTime(row.CreatedAt),
		UpdatedAt:   utcTime(row.UpdatedAt),
		ArchivedAt:  optionalTime(row.DeletedAt),
		Version:     row.Version,
	})
}

func toDomainUsageRecord(row sqlc.OwnershipItemUsageRecord) domainitem.UsageRecord {
	return domainitem.ReconstructUsageRecord(domainitem.ReconstructUsageRecordParams{
		ID:        domainitem.UsageRecordID(row.ID),
		PublicID:  row.PublicID,
		UserID:    domainauth.UserID(row.UserID),
		ItemID:    domainitem.ItemID(row.ItemID),
		UsedAt:    utcTime(row.UsedAt),
		Quantity:  row.Quantity,
		Note:      row.Note,
		CreatedAt: utcTime(row.CreatedAt),
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
