// Package postgresql はPostgreSQL接続とtransaction制御を提供する。
package postgresql

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 接続poolの既定値。負荷試験後に環境変数化する。
const (
	defaultMaxConnections    = 10
	defaultMinConnections    = 2
	defaultMaxConnLifetime   = time.Hour
	defaultMaxConnIdleTime   = 30 * time.Minute
	defaultHealthCheckPeriod = time.Minute
	defaultConnectTimeout    = 5 * time.Second
)

// NewPool は接続poolを生成し、疎通確認を行う。
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	poolConfig.MaxConns = defaultMaxConnections
	poolConfig.MinConns = defaultMinConnections
	poolConfig.MaxConnLifetime = defaultMaxConnLifetime
	poolConfig.MaxConnIdleTime = defaultMaxConnIdleTime
	poolConfig.HealthCheckPeriod = defaultHealthCheckPeriod
	poolConfig.ConnConfig.ConnectTimeout = defaultConnectTimeout

	// 時刻は常にUTCで扱う (設計書 4.3)。
	poolConfig.ConnConfig.RuntimeParams["timezone"] = "UTC"

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	pingContext, cancel := context.WithTimeout(ctx, defaultConnectTimeout)
	defer cancel()

	if err := pool.Ping(pingContext); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
