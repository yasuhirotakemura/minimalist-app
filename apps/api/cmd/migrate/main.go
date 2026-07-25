// Command migrate はDB migrationとlocal開発用seedの投入を行う。
//
//	migrate up      : 未適用のmigrationを適用する
//	migrate down    : 直近のmigrationを1つ戻す
//	migrate status  : 適用状況を表示する
//	migrate version : 現在のversionを表示する
//	migrate seed    : local開発用dataを投入する (APP_ENV=local のみ)
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/config"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/logging"
)

const databaseConnectTimeout = 30 * time.Second

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("migrate failed", "error", err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	command := "up"
	if len(args) > 0 {
		command = args[0]
	}

	// migrationはDB接続情報のみ、seedは追加でPASSWORD_PEPPERを必要とする。
	validationMode := config.ValidateDatabaseOnly
	if command == "seed" {
		validationMode = config.ValidateSeed
	}

	cfg, err := config.LoadWithMode(validationMode)
	if err != nil {
		return err
	}

	logger := logging.New(os.Stdout, cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, cancel := context.WithTimeout(context.Background(), databaseConnectTimeout)
	defer cancel()

	database, err := openDatabase(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			logger.Warn("failed to close database", "error", closeErr.Error())
		}
	}()

	if command == "seed" {
		return runSeed(context.Background(), database, cfg, logger)
	}

	return runGoose(context.Background(), database, cfg, command, logger)
}

func openDatabase(ctx context.Context, databaseURL string) (*sql.DB, error) {
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// PostgreSQLの起動直後は接続できないことがあるため、短い間隔で再試行する。
	deadline := time.Now().Add(databaseConnectTimeout)
	for {
		pingErr := database.PingContext(ctx)
		if pingErr == nil {
			return database, nil
		}
		if time.Now().After(deadline) {
			_ = database.Close()
			return nil, fmt.Errorf("ping database: %w", pingErr)
		}
		time.Sleep(time.Second)
	}
}

func runGoose(
	ctx context.Context,
	database *sql.DB,
	cfg config.Config,
	command string,
	logger *slog.Logger,
) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	goose.SetLogger(gooseLogger{logger: logger})

	switch command {
	case "up":
		if err := goose.UpContext(ctx, database, cfg.MigrationsDir); err != nil {
			return fmt.Errorf("apply migrations: %w", err)
		}
	case "down":
		// 適用済みmigrationの巻き戻しはlocal開発の試行用に限定する。
		if !cfg.IsLocal() {
			return errors.New("migrate down is allowed only when APP_ENV=local")
		}
		if err := goose.DownContext(ctx, database, cfg.MigrationsDir); err != nil {
			return fmt.Errorf("rollback migration: %w", err)
		}
	case "status":
		if err := goose.StatusContext(ctx, database, cfg.MigrationsDir); err != nil {
			return fmt.Errorf("show migration status: %w", err)
		}
	case "version":
		if err := goose.VersionContext(ctx, database, cfg.MigrationsDir); err != nil {
			return fmt.Errorf("show migration version: %w", err)
		}
	default:
		return fmt.Errorf("unknown command %q (expected up/down/status/version/seed)", command)
	}

	return nil
}

// gooseLogger はgooseの出力をslogへ転送する。
type gooseLogger struct {
	logger *slog.Logger
}

func (l gooseLogger) Fatalf(format string, v ...any) {
	l.logger.Error(fmt.Sprintf(format, v...))
	os.Exit(1)
}

func (l gooseLogger) Printf(format string, v ...any) {
	l.logger.Info(fmt.Sprintf(format, v...))
}
