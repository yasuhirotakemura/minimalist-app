//go:build integration

// Package integration はPostgreSQLを使うintegration testを提供する (設計書 25.2)。
//
// 実行方法:
//
//	make test-integration                 : compose の postgres を再利用する
//	make test-integration-testcontainers  : testcontainers-go でPostgreSQLを起動する
//
// TEST_DATABASE_URL が設定されている場合はそのPostgreSQLを再利用し、
// 未設定の場合は testcontainers-go がPostgreSQLを起動する。
package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// migrationsDirectory はmigration fileの位置 (本packageのdirectoryからの相対path)。
const migrationsDirectory = "../../../../db/migrations"

// testPool は全testで共有する接続pool。
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	ctx := context.Background()

	databaseURL, terminate, err := prepareDatabase(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to prepare test database: %v\n", err)
		return 1
	}
	defer terminate()

	if err := applyMigrations(databaseURL); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply migrations: %v\n", err)
		return 1
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create connection pool: %v\n", err)
		return 1
	}
	defer pool.Close()

	testPool = pool
	return m.Run()
}

// prepareDatabase はtest用PostgreSQLを用意し、接続URLと後始末関数を返す。
func prepareDatabase(ctx context.Context) (string, func(), error) {
	if databaseURL := os.Getenv("TEST_DATABASE_URL"); databaseURL != "" {
		// 既存のPostgreSQLを再利用する。container起動を伴わないため高速。
		return databaseURL, func() {}, nil
	}

	container, err := tcpostgres.Run(
		ctx,
		"postgres:17-alpine",
		tcpostgres.WithDatabase("less_test"),
		tcpostgres.WithUsername("less"),
		tcpostgres.WithPassword("less_test_password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return "", func() {}, fmt.Errorf("start postgres container: %w", err)
	}

	terminate := func() { _ = testcontainers.TerminateContainer(container) }

	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		terminate()
		return "", func() {}, fmt.Errorf("resolve connection string: %w", err)
	}

	return databaseURL, terminate, nil
}

func applyMigrations(databaseURL string) error {
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = database.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	goose.SetLogger(goose.NopLogger())

	if err := goose.Up(database, migrationsDirectory); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// truncateAll はtest間の独立性を保つためtest dataを消去する。
func truncateAll(t *testing.T) {
	t.Helper()

	_, err := testPool.Exec(
		context.Background(),
		`TRUNCATE
		     audit.audit_logs,
		     ownership.item_usage_records,
		     ownership.item_tags,
		     ownership.items,
		     ownership.tags,
		     ownership.categories,
		     identity.auth_sessions,
		     identity.user_password_auths,
		     identity.users
		 RESTART IDENTITY CASCADE`,
	)
	if err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}
}
