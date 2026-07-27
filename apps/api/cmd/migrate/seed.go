package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/config"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/crypto"
)

// devUsersSeedFileName はlocal開発用ユーザーのseed file。
const devUsersSeedFileName = "dev_users.json"

// seedFile はseed fileの構造。
//
// password hashはArgon2id + PASSWORD_PEPPERに依存するため、SQLでは表現できない。
// 平文passwordをここで読み込み、application側と同じhasherでhash化する。
type seedFile struct {
	Users []seedUser `json:"users"`
}

type seedUser struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
	Timezone    string `json:"timezone"`
	Locale      string `json:"locale"`
}

// runSeed はlocal開発用dataを投入する。
//
// 既に同一emailのユーザーが存在する場合は何もしない (冪等)。
// 本番dataを破壊しないよう、APP_ENV=local 以外では実行を拒否する。
func runSeed(
	ctx context.Context,
	database *sql.DB,
	cfg config.Config,
	logger *slog.Logger,
) error {
	if !cfg.IsLocal() {
		return fmt.Errorf("seed is allowed only when APP_ENV=local (got %q)", cfg.AppEnv)
	}

	users, err := readSeedUsers(filepath.Join(cfg.SeedsDir, devUsersSeedFileName))
	if err != nil {
		return err
	}

	passwordHasher, err := crypto.NewArgon2idPasswordHasher(
		cfg.PasswordPepper,
		crypto.DefaultArgon2idParameters(),
	)
	if err != nil {
		return fmt.Errorf("initialize password hasher: %w", err)
	}

	inserted := 0
	skipped := 0

	for _, seeded := range users {
		created, err := insertSeedUser(ctx, database, passwordHasher, seeded)
		if err != nil {
			return err
		}
		if created {
			inserted++
		} else {
			skipped++
		}
	}

	logger.Info("seed completed", "inserted", inserted, "skipped", skipped)

	// 所持品・タグはユーザー作成後に投入する。
	return runItemSeed(ctx, database, cfg, logger)
}

func readSeedUsers(path string) ([]seedUser, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // pathは環境変数由来のseed置き場である。
	if err != nil {
		return nil, fmt.Errorf("read seed file %s: %w", path, err)
	}

	var parsed seedFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse seed file %s: %w", path, err)
	}
	if len(parsed.Users) == 0 {
		return nil, fmt.Errorf("seed file %s contains no users", path)
	}
	return parsed.Users, nil
}

func insertSeedUser(
	ctx context.Context,
	database *sql.DB,
	passwordHasher auth.PasswordHasher,
	seeded seedUser,
) (bool, error) {
	email, err := auth.NewEmail(seeded.Email)
	if err != nil {
		return false, fmt.Errorf("invalid seed email %q: %w", seeded.Email, err)
	}

	rawPassword, err := auth.NewRawPassword(seeded.Password)
	if err != nil {
		return false, fmt.Errorf("invalid seed password for %s: %w", email, err)
	}

	publicID, err := uuid.NewV7()
	if err != nil {
		return false, fmt.Errorf("generate public id: %w", err)
	}

	user, err := auth.NewUser(publicID, email, seeded.DisplayName, seeded.Timezone, seeded.Locale, time.Now())
	if err != nil {
		return false, fmt.Errorf("invalid seed user %s: %w", email, err)
	}

	passwordHash, err := passwordHasher.Hash(rawPassword)
	if err != nil {
		return false, fmt.Errorf("hash seed password for %s: %w", email, err)
	}

	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin seed transaction: %w", err)
	}
	defer func() {
		_ = transaction.Rollback()
	}()

	var userID int64
	err = transaction.QueryRowContext(
		ctx,
		`INSERT INTO identity.users
		     (public_id, email, display_name, timezone, locale, created_at, updated_at, version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 1)
		 ON CONFLICT DO NOTHING
		 RETURNING id`,
		user.PublicID(),
		user.Email().String(),
		user.DisplayName(),
		user.Timezone(),
		user.Locale(),
		user.CreatedAt(),
		user.UpdatedAt(),
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		// 既に同一emailのユーザーが存在する。
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("insert seed user %s: %w", email, err)
	}

	if _, err := transaction.ExecContext(
		ctx,
		`INSERT INTO identity.user_password_auths
		     (user_id, password_hash, algorithm, password_updated_at, created_at, updated_at, version)
		 VALUES ($1, $2, 'argon2id', $3, $4, $5, 1)`,
		userID,
		passwordHash.Encoded(),
		user.CreatedAt(),
		user.CreatedAt(),
		user.UpdatedAt(),
	); err != nil {
		return false, fmt.Errorf("insert seed password auth for %s: %w", email, err)
	}

	// API経由の登録と同様に既定カテゴリーを作成する (設計書 28章 Phase 1)。
	if err := insertDefaultCategories(ctx, transaction, userID, user.CreatedAt()); err != nil {
		return false, fmt.Errorf("insert default categories for %s: %w", email, err)
	}

	if err := transaction.Commit(); err != nil {
		return false, fmt.Errorf("commit seed transaction: %w", err)
	}

	return true, nil
}
