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

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/config"
)

// devStorageSeedFileName はlocal開発用の収納単位seed file (Phase 2)。
const devStorageSeedFileName = "dev_storage_units.json"

// storageSeedFile はseed fileの構造。
type storageSeedFile struct {
	OwnerEmail   string            `json:"ownerEmail"`
	StorageUnits []seedStorageUnit `json:"storageUnits"`
	Allocations  []seedAllocation  `json:"allocations"`
}

type seedStorageUnit struct {
	Name string `json:"name"`
	// ParentName は同じfile内の収納単位名。rootの場合はnull。
	ParentName              *string `json:"parentName"`
	StorageTypeCode         string  `json:"storageTypeCode"`
	MobilityClassCode       string  `json:"mobilityClassCode"`
	TareWeightGram          *int32  `json:"tareWeightGram"`
	MaximumWeightGram       *int32  `json:"maximumWeightGram"`
	MaximumVolumeMilliliter *int32  `json:"maximumVolumeMilliliter"`
	Description             *string `json:"description"`
	SortOrder               int32   `json:"sortOrder"`
}

type seedAllocation struct {
	StorageUnitName string `json:"storageUnitName"`
	ItemName        string `json:"itemName"`
	Quantity        int32  `json:"quantity"`
}

// runStorageSeed は収納単位と収納割当のseedを投入する。
//
// 対象ユーザーに既に収納単位が存在する場合は何もしない (冪等)。
// 所持品seedの後に実行する前提であり、割当対象のアイテムが存在しない場合は
// その割当だけをskipする。
func runStorageSeed(
	ctx context.Context,
	database *sql.DB,
	cfg config.Config,
	logger *slog.Logger,
) error {
	if !cfg.IsLocal() {
		return fmt.Errorf("seed is allowed only when APP_ENV=local (got %q)", cfg.AppEnv)
	}

	seeded, err := readStorageSeedFile(filepath.Join(cfg.SeedsDir, devStorageSeedFileName))
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
		logger.Warn("storage seed skipped: owner not found", "email", seeded.OwnerEmail)
		return nil
	}
	if err != nil {
		return fmt.Errorf("find seed owner %s: %w", seeded.OwnerEmail, err)
	}

	var existingUnits int
	if err := database.QueryRowContext(
		ctx,
		`SELECT count(*) FROM ownership.storage_units WHERE user_id = $1`,
		userID,
	).Scan(&existingUnits); err != nil {
		return fmt.Errorf("count existing storage units: %w", err)
	}
	if existingUnits > 0 {
		logger.Info("storage seed skipped: storage units already exist", "count", existingUnits)
		return nil
	}

	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin storage seed transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	now := time.Now().UTC()

	unitIDsByName, err := insertSeedStorageUnits(
		ctx, transaction, userID, seeded.StorageUnits, now)
	if err != nil {
		return err
	}

	itemIDsByName, err := loadItemIDs(ctx, transaction, userID)
	if err != nil {
		return err
	}

	inserted, err := insertSeedAllocations(
		ctx, transaction, userID, seeded.Allocations, unitIDsByName, itemIDsByName, now, logger)
	if err != nil {
		return err
	}

	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit storage seed transaction: %w", err)
	}

	logger.Info("storage seed completed",
		"owner", seeded.OwnerEmail,
		"storageUnits", len(seeded.StorageUnits),
		"allocations", inserted)
	return nil
}

func readStorageSeedFile(path string) (storageSeedFile, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // pathは環境変数由来のseed置き場である。
	if err != nil {
		return storageSeedFile{}, fmt.Errorf("read seed file %s: %w", path, err)
	}

	var parsed storageSeedFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return storageSeedFile{}, fmt.Errorf("parse seed file %s: %w", path, err)
	}
	if parsed.OwnerEmail == "" {
		return storageSeedFile{}, fmt.Errorf("seed file %s has no ownerEmail", path)
	}
	return parsed, nil
}

// insertSeedStorageUnits は収納単位を並び順に投入する。
//
// 親は同じfile内で先に現れている必要がある。解決できない親は
// seed fileの誤りであるためerrorとする。
func insertSeedStorageUnits(
	ctx context.Context,
	transaction *sql.Tx,
	userID int64,
	units []seedStorageUnit,
	now time.Time,
) (map[string]int64, error) {
	unitIDsByName := make(map[string]int64, len(units))

	for _, unit := range units {
		publicID, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generate storage unit public id: %w", err)
		}

		var parentID *int64
		if unit.ParentName != nil {
			resolved, ok := unitIDsByName[*unit.ParentName]
			if !ok {
				return nil, fmt.Errorf(
					"seed storage unit %q references unknown parent %q",
					unit.Name, *unit.ParentName)
			}
			parentID = &resolved
		}

		var unitID int64
		if err := transaction.QueryRowContext(
			ctx,
			`INSERT INTO ownership.storage_units
			     (public_id, user_id, parent_id, name, storage_type_code, mobility_class_code,
			      tare_weight_gram, maximum_weight_gram, maximum_volume_milliliter,
			      description, sort_order, created_at, updated_at, version)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 1)
			 RETURNING id`,
			publicID,
			userID,
			parentID,
			unit.Name,
			unit.StorageTypeCode,
			unit.MobilityClassCode,
			unit.TareWeightGram,
			unit.MaximumWeightGram,
			unit.MaximumVolumeMilliliter,
			unit.Description,
			unit.SortOrder,
			now,
			now,
		).Scan(&unitID); err != nil {
			return nil, fmt.Errorf("insert seed storage unit %q: %w", unit.Name, err)
		}

		unitIDsByName[unit.Name] = unitID
	}

	return unitIDsByName, nil
}

// loadItemIDs はユーザーの所持品を名称から引けるようにする。
func loadItemIDs(
	ctx context.Context,
	transaction *sql.Tx,
	userID int64,
) (map[string]int64, error) {
	rows, err := transaction.QueryContext(
		ctx,
		`SELECT name, id FROM ownership.items WHERE user_id = $1 AND deleted_at IS NULL`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("load item ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	itemIDsByName := map[string]int64{}
	for rows.Next() {
		var name string
		var id int64
		if err := rows.Scan(&name, &id); err != nil {
			return nil, fmt.Errorf("scan item id: %w", err)
		}
		itemIDsByName[name] = id
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate item ids: %w", err)
	}
	return itemIDsByName, nil
}

// insertSeedAllocations は収納割当を投入する。
//
// 割当対象のアイテムが存在しない場合、seed全体を失敗させず該当行だけをskipする。
// 所持品seedは別fileで管理されており、片方だけを差し替える運用を許容する。
func insertSeedAllocations(
	ctx context.Context,
	transaction *sql.Tx,
	userID int64,
	allocations []seedAllocation,
	unitIDsByName map[string]int64,
	itemIDsByName map[string]int64,
	now time.Time,
	logger *slog.Logger,
) (int, error) {
	inserted := 0

	for _, allocation := range allocations {
		unitID, ok := unitIDsByName[allocation.StorageUnitName]
		if !ok {
			return 0, fmt.Errorf(
				"seed allocation references unknown storage unit %q",
				allocation.StorageUnitName)
		}

		itemID, ok := itemIDsByName[allocation.ItemName]
		if !ok {
			logger.Warn("storage allocation seed skipped: item not found",
				"item", allocation.ItemName, "storageUnit", allocation.StorageUnitName)
			continue
		}

		publicID, err := uuid.NewV7()
		if err != nil {
			return 0, fmt.Errorf("generate storage allocation public id: %w", err)
		}

		if _, err := transaction.ExecContext(
			ctx,
			`INSERT INTO ownership.storage_allocations
			     (public_id, user_id, storage_unit_id, item_id, quantity,
			      created_at, updated_at, version)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, 1)`,
			publicID,
			userID,
			unitID,
			itemID,
			allocation.Quantity,
			now,
			now,
		); err != nil {
			return 0, fmt.Errorf(
				"insert seed allocation %q -> %q: %w",
				allocation.ItemName, allocation.StorageUnitName, err)
		}
		inserted++
	}

	// 割当数量合計が所有数量を超えていないことを確認する。
	// seed fileの誤りを起動時に気付けるようにする (設計書 13.9)。
	var violating int
	if err := transaction.QueryRowContext(
		ctx,
		`SELECT count(*)
		 FROM (
		     SELECT sa.item_id, SUM(sa.quantity) AS allocated, MAX(i.quantity) AS owned
		     FROM ownership.storage_allocations sa
		     JOIN ownership.items i ON i.id = sa.item_id AND i.user_id = sa.user_id
		     WHERE sa.user_id = $1
		     GROUP BY sa.item_id
		 ) totals
		 WHERE totals.allocated > totals.owned`,
		userID,
	).Scan(&violating); err != nil {
		return 0, fmt.Errorf("verify seed allocation quantities: %w", err)
	}
	if violating > 0 {
		return 0, fmt.Errorf(
			"seed allocations exceed owned quantity for %d item(s)", violating)
	}

	return inserted, nil
}
