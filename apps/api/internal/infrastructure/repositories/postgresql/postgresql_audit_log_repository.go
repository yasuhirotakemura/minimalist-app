package postgresql

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	domainaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/audit"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/sqlc"
	infrapostgresql "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/postgresql"
)

// PostgresqlAuditLogRepository はAuditLogRepositoryのPostgreSQL実装。
type PostgresqlAuditLogRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresqlAuditLogRepository はPostgresqlAuditLogRepositoryを生成する。
func NewPostgresqlAuditLogRepository(pool *pgxpool.Pool) *PostgresqlAuditLogRepository {
	return &PostgresqlAuditLogRepository{pool: pool}
}

var _ domainaudit.AuditLogRepository = (*PostgresqlAuditLogRepository)(nil)

func (r *PostgresqlAuditLogRepository) queries(ctx context.Context) *sqlc.Queries {
	return sqlc.New(infrapostgresql.Querier(ctx, r.pool))
}

// Create は操作履歴を1件記録する。
//
// 呼び出し元の業務操作と同一transaction内で実行する (設計書 20.1)。
func (r *PostgresqlAuditLogRepository) Create(
	ctx context.Context,
	log domainaudit.AuditLog,
) error {
	changes, err := json.Marshal(log.Changes())
	if err != nil {
		return fmt.Errorf("encode audit log changes: %w", err)
	}

	if err := r.queries(ctx).InsertAuditLog(ctx, sqlc.InsertAuditLogParams{
		PublicID:       log.PublicID(),
		UserID:         log.UserID().Int64(),
		ActionCode:     log.Action().String(),
		TargetTypeCode: log.TargetType().String(),
		TargetPublicID: log.TargetPublicID(),
		Changes:        changes,
		CreatedAt:      timestamptz(log.CreatedAt()),
	}); err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}
