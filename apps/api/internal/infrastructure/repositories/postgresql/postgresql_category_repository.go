package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domaincategory "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/category"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/sqlc"
	infrapostgresql "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/postgresql"
)

// PostgresqlCategoryRepository はCategoryRepositoryのPostgreSQL実装。
type PostgresqlCategoryRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresqlCategoryRepository はPostgresqlCategoryRepositoryを生成する。
func NewPostgresqlCategoryRepository(pool *pgxpool.Pool) *PostgresqlCategoryRepository {
	return &PostgresqlCategoryRepository{pool: pool}
}

var _ domaincategory.CategoryRepository = (*PostgresqlCategoryRepository)(nil)

func (r *PostgresqlCategoryRepository) queries(ctx context.Context) *sqlc.Queries {
	return sqlc.New(infrapostgresql.Querier(ctx, r.pool))
}

// CreateAll はカテゴリーをまとめて作成する。
//
// 呼び出し元がtransaction境界を制御する。1件でも失敗した場合は全体をrollbackする。
func (r *PostgresqlCategoryRepository) CreateAll(
	ctx context.Context,
	categories []domaincategory.Category,
) ([]domaincategory.Category, error) {
	queries := r.queries(ctx)
	created := make([]domaincategory.Category, 0, len(categories))

	for _, category := range categories {
		row, err := queries.InsertCategory(ctx, sqlc.InsertCategoryParams{
			PublicID:    category.PublicID(),
			UserID:      category.UserID().Int64(),
			Name:        category.Name(),
			Description: category.Description(),
			SortOrder:   category.SortOrder(),
			CreatedAt:   timestamptz(category.CreatedAt()),
			UpdatedAt:   timestamptz(category.UpdatedAt()),
		})
		if err != nil {
			return nil, fmt.Errorf("insert category %q: %w", category.Name(), err)
		}
		created = append(created, toDomainCategory(row))
	}

	return created, nil
}

// ListActiveByUserID は有効なカテゴリーを表示順で取得する。
func (r *PostgresqlCategoryRepository) ListActiveByUserID(
	ctx context.Context,
	userID domainauth.UserID,
) ([]domaincategory.Category, error) {
	rows, err := r.queries(ctx).ListActiveCategoriesByUserID(ctx, userID.Int64())
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}

	categories := make([]domaincategory.Category, 0, len(rows))
	for _, row := range rows {
		categories = append(categories, toDomainCategory(row))
	}
	return categories, nil
}

// FindActiveByPublicID は有効なカテゴリーを取得する。
func (r *PostgresqlCategoryRepository) FindActiveByPublicID(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
) (domaincategory.Category, error) {
	row, err := r.queries(ctx).FindActiveCategoryByPublicID(
		ctx,
		sqlc.FindActiveCategoryByPublicIDParams{
			PublicID: publicID,
			UserID:   userID.Int64(),
		},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domaincategory.Category{}, domaincategory.ErrCategoryNotFound
		}
		return domaincategory.Category{}, fmt.Errorf("find category by public id: %w", err)
	}
	return toDomainCategory(row), nil
}

func toDomainCategory(row sqlc.OwnershipCategory) domaincategory.Category {
	return domaincategory.ReconstructCategory(domaincategory.ReconstructCategoryParams{
		ID:          domaincategory.CategoryID(row.ID),
		PublicID:    row.PublicID,
		UserID:      domainauth.UserID(row.UserID),
		Name:        row.Name,
		Description: row.Description,
		SortOrder:   row.SortOrder,
		CreatedAt:   utcTime(row.CreatedAt),
		UpdatedAt:   utcTime(row.UpdatedAt),
		DeletedAt:   optionalTime(row.DeletedAt),
		Version:     row.Version,
	})
}
