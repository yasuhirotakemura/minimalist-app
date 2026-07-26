package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/category"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/config"
)

// devItemsSeedFileName はlocal開発用の所持品seed file。
const devItemsSeedFileName = "dev_items.json"

// itemSeedFile はseed fileの構造。
type itemSeedFile struct {
	OwnerEmail string     `json:"ownerEmail"`
	Tags       []string   `json:"tags"`
	Items      []seedItem `json:"items"`
}

type seedItem struct {
	Name                 string   `json:"name"`
	CategoryName         string   `json:"categoryName"`
	ItemKindCode         string   `json:"itemKindCode"`
	Quantity             int32    `json:"quantity"`
	DesiredQuantity      *int32   `json:"desiredQuantity"`
	UnitName             string   `json:"unitName"`
	NecessityLevelCode   string   `json:"necessityLevelCode"`
	UsageFrequencyCode   string   `json:"usageFrequencyCode"`
	SubstitutabilityCode string   `json:"substitutabilityCode"`
	MobilityClassCode    string   `json:"mobilityClassCode"`
	OwnershipReason      *string  `json:"ownershipReason"`
	DisposalCondition    *string  `json:"disposalCondition"`
	PurchaseAmount       *int64   `json:"purchaseAmount"`
	ReplacementAmount    *int64   `json:"replacementAmount"`
	ResaleAmount         *int64   `json:"resaleAmount"`
	WeightGram           *int32   `json:"weightGram"`
	VolumeMilliliter     *int32   `json:"volumeMilliliter"`
	IsValuable           bool     `json:"isValuable"`
	IsFragile            bool     `json:"isFragile"`
	ExpiresOn            *string  `json:"expiresOn"`
	Notes                *string  `json:"notes"`
	Tags                 []string `json:"tags"`
}

// insertDefaultCategories は既定カテゴリーを作成する。
//
// API経由の登録では RegisterUserService が同じ処理を行う。
// seedはSQLで直接ユーザーを作成するため、定義をdomainから読み込んで同じ内容を投入する。
func insertDefaultCategories(
	ctx context.Context,
	transaction *sql.Tx,
	userID int64,
	now time.Time,
) error {
	for _, definition := range category.DefaultCategoryDefinitions() {
		publicID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate category public id: %w", err)
		}

		if _, err := transaction.ExecContext(
			ctx,
			`INSERT INTO ownership.categories
			     (public_id, user_id, name, description, sort_order, created_at, updated_at, version)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, 1)`,
			publicID,
			userID,
			definition.Name,
			definition.Description,
			definition.SortOrder,
			now,
			now,
		); err != nil {
			return fmt.Errorf("insert default category %q: %w", definition.Name, err)
		}
	}
	return nil
}

// runItemSeed は所持品とタグのseedを投入する。
//
// 対象ユーザーに既に所持品が存在する場合は何もしない (冪等)。
func runItemSeed(
	ctx context.Context,
	database *sql.DB,
	cfg config.Config,
	logger *slog.Logger,
) error {
	if !cfg.IsLocal() {
		return fmt.Errorf("seed is allowed only when APP_ENV=local (got %q)", cfg.AppEnv)
	}

	seeded, err := readItemSeedFile(filepath.Join(cfg.SeedsDir, devItemsSeedFileName))
	if err != nil {
		return err
	}

	var userID int64
	err = database.QueryRowContext(
		ctx,
		`SELECT id FROM identity.users WHERE email = $1 AND deleted_at IS NULL`,
		seeded.OwnerEmail,
	).Scan(&userID)
	if err == sql.ErrNoRows {
		logger.Warn("item seed skipped: owner not found", "email", seeded.OwnerEmail)
		return nil
	}
	if err != nil {
		return fmt.Errorf("find seed owner %s: %w", seeded.OwnerEmail, err)
	}

	var existingItems int
	if err := database.QueryRowContext(
		ctx,
		`SELECT count(*) FROM ownership.items WHERE user_id = $1`,
		userID,
	).Scan(&existingItems); err != nil {
		return fmt.Errorf("count existing items: %w", err)
	}
	if existingItems > 0 {
		logger.Info("item seed skipped: items already exist", "count", existingItems)
		return nil
	}

	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin item seed transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	now := time.Now().UTC()

	tagIDsByName, err := insertSeedTags(ctx, transaction, userID, seeded.Tags, now)
	if err != nil {
		return err
	}
	categoryIDsByName, err := loadCategoryIDs(ctx, transaction, userID)
	if err != nil {
		return err
	}

	for _, item := range seeded.Items {
		if err := insertSeedItem(
			ctx, transaction, userID, item, categoryIDsByName, tagIDsByName, now); err != nil {
			return err
		}
	}

	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit item seed transaction: %w", err)
	}

	logger.Info("item seed completed",
		"owner", seeded.OwnerEmail, "tags", len(seeded.Tags), "items", len(seeded.Items))
	return nil
}

func readItemSeedFile(path string) (itemSeedFile, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // pathは環境変数由来のseed置き場である。
	if err != nil {
		return itemSeedFile{}, fmt.Errorf("read seed file %s: %w", path, err)
	}

	var parsed itemSeedFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return itemSeedFile{}, fmt.Errorf("parse seed file %s: %w", path, err)
	}
	if parsed.OwnerEmail == "" {
		return itemSeedFile{}, fmt.Errorf("seed file %s has no ownerEmail", path)
	}
	return parsed, nil
}

func insertSeedTags(
	ctx context.Context,
	transaction *sql.Tx,
	userID int64,
	names []string,
	now time.Time,
) (map[string]int64, error) {
	tagIDsByName := make(map[string]int64, len(names))

	for _, name := range names {
		publicID, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generate tag public id: %w", err)
		}

		var tagID int64
		if err := transaction.QueryRowContext(
			ctx,
			`INSERT INTO ownership.tags
			     (public_id, user_id, name, created_at, updated_at, version)
			 VALUES ($1, $2, $3, $4, $5, 1)
			 RETURNING id`,
			publicID, userID, name, now, now,
		).Scan(&tagID); err != nil {
			return nil, fmt.Errorf("insert seed tag %q: %w", name, err)
		}
		tagIDsByName[name] = tagID
	}
	return tagIDsByName, nil
}

func loadCategoryIDs(
	ctx context.Context,
	transaction *sql.Tx,
	userID int64,
) (map[string]int64, error) {
	rows, err := transaction.QueryContext(
		ctx,
		`SELECT id, name FROM ownership.categories
		 WHERE user_id = $1 AND deleted_at IS NULL`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	categoryIDsByName := make(map[string]int64)
	for rows.Next() {
		var (
			id   int64
			name string
		)
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categoryIDsByName[name] = id
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate categories: %w", err)
	}
	return categoryIDsByName, nil
}

func insertSeedItem(
	ctx context.Context,
	transaction *sql.Tx,
	userID int64,
	item seedItem,
	categoryIDsByName map[string]int64,
	tagIDsByName map[string]int64,
	now time.Time,
) error {
	categoryID, ok := categoryIDsByName[item.CategoryName]
	if !ok {
		return fmt.Errorf("seed item %q refers to unknown category %q", item.Name, item.CategoryName)
	}

	publicID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate item public id: %w", err)
	}

	var expiresOn any
	if item.ExpiresOn != nil {
		parsed, err := time.Parse(time.DateOnly, *item.ExpiresOn)
		if err != nil {
			return fmt.Errorf("seed item %q has invalid expiresOn: %w", item.Name, err)
		}
		expiresOn = parsed
	}

	var itemID int64
	if err := transaction.QueryRowContext(
		ctx,
		`INSERT INTO ownership.items (
		     public_id, user_id, category_id, name, item_kind_code,
		     quantity, desired_quantity, unit_name,
		     necessity_level_code, usage_frequency_code,
		     substitutability_code, mobility_class_code,
		     ownership_reason, disposal_condition,
		     purchase_amount, replacement_amount, resale_amount,
		     weight_gram, volume_milliliter,
		     is_valuable, is_fragile, expires_on, notes,
		     created_at, updated_at, version
		 ) VALUES (
		     $1, $2, $3, $4, $5,
		     $6, $7, $8,
		     $9, $10,
		     $11, $12,
		     $13, $14,
		     $15, $16, $17,
		     $18, $19,
		     $20, $21, $22, $23,
		     $24, $25, 1
		 )
		 RETURNING id`,
		publicID, userID, categoryID, item.Name, item.ItemKindCode,
		item.Quantity, item.DesiredQuantity, item.UnitName,
		item.NecessityLevelCode, item.UsageFrequencyCode,
		item.SubstitutabilityCode, item.MobilityClassCode,
		item.OwnershipReason, item.DisposalCondition,
		item.PurchaseAmount, item.ReplacementAmount, item.ResaleAmount,
		item.WeightGram, item.VolumeMilliliter,
		item.IsValuable, item.IsFragile, expiresOn, item.Notes,
		now, now,
	).Scan(&itemID); err != nil {
		return fmt.Errorf("insert seed item %q: %w", item.Name, err)
	}

	for _, tagName := range item.Tags {
		tagID, ok := tagIDsByName[tagName]
		if !ok {
			return fmt.Errorf("seed item %q refers to unknown tag %q", item.Name, tagName)
		}
		if _, err := transaction.ExecContext(
			ctx,
			`INSERT INTO ownership.item_tags (user_id, item_id, tag_id, created_at)
			 VALUES ($1, $2, $3, $4)`,
			userID, itemID, tagID, now,
		); err != nil {
			return fmt.Errorf("insert seed item tag %q/%q: %w", item.Name, tagName, err)
		}
	}
	return nil
}
