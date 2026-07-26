package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	domaintag "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/tag"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/sqlc"
	infrapostgresql "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/postgresql"
)

// タグ名重複時に返されるconstraint名。migrationの定義と一致させる。
const tagNameUniqueIndexName = "uq_tags__user_id_name_active"

// PostgresqlTagRepository はTagRepositoryのPostgreSQL実装。
type PostgresqlTagRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresqlTagRepository はPostgresqlTagRepositoryを生成する。
func NewPostgresqlTagRepository(pool *pgxpool.Pool) *PostgresqlTagRepository {
	return &PostgresqlTagRepository{pool: pool}
}

var _ domaintag.TagRepository = (*PostgresqlTagRepository)(nil)

func (r *PostgresqlTagRepository) queries(ctx context.Context) *sqlc.Queries {
	return sqlc.New(infrapostgresql.Querier(ctx, r.pool))
}

// Create はタグを作成する。
func (r *PostgresqlTagRepository) Create(
	ctx context.Context,
	tag domaintag.Tag,
) (domaintag.Tag, error) {
	row, err := r.queries(ctx).InsertTag(ctx, sqlc.InsertTagParams{
		PublicID:  tag.PublicID(),
		UserID:    tag.UserID().Int64(),
		Name:      tag.Name(),
		CreatedAt: timestamptz(tag.CreatedAt()),
		UpdatedAt: timestamptz(tag.UpdatedAt()),
	})
	if err != nil {
		if isUniqueViolation(err, tagNameUniqueIndexName) {
			return domaintag.Tag{}, domaintag.ErrTagNameAlreadyUsed.WithCause(err)
		}
		return domaintag.Tag{}, fmt.Errorf("insert tag: %w", err)
	}
	return toDomainTag(row), nil
}

// ListActiveWithItemCount は有効なタグと付与件数を名称昇順で取得する。
func (r *PostgresqlTagRepository) ListActiveWithItemCount(
	ctx context.Context,
	userID domainauth.UserID,
) ([]domaintag.Summary, error) {
	rows, err := r.queries(ctx).ListActiveTagsWithItemCountByUserID(ctx, userID.Int64())
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	summaries := make([]domaintag.Summary, 0, len(rows))
	for _, row := range rows {
		summaries = append(summaries, domaintag.Summary{
			Tag:       toDomainTag(row.OwnershipTag),
			ItemCount: row.ItemCount,
		})
	}
	return summaries, nil
}

// FindActiveByPublicID は有効なタグを取得する。
func (r *PostgresqlTagRepository) FindActiveByPublicID(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
) (domaintag.Tag, error) {
	row, err := r.queries(ctx).FindActiveTagByPublicID(
		ctx,
		sqlc.FindActiveTagByPublicIDParams{PublicID: publicID, UserID: userID.Int64()},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domaintag.Tag{}, domaintag.ErrTagNotFound
		}
		return domaintag.Tag{}, fmt.Errorf("find tag by public id: %w", err)
	}
	return toDomainTag(row), nil
}

// ResolveActiveReferences は指定した公開IDのタグ参照を解決する。
//
// 1件でも解決できない場合は ErrTagNotFound を返す。
// 他ユーザーのタグを指定した場合も解決できないため同一errorとなる (設計書 18.3)。
func (r *PostgresqlTagRepository) ResolveActiveReferences(
	ctx context.Context,
	userID domainauth.UserID,
	publicIDs []uuid.UUID,
) ([]domaintag.Reference, error) {
	if len(publicIDs) == 0 {
		return nil, nil
	}

	requested := make(map[uuid.UUID]struct{}, len(publicIDs))
	for _, publicID := range publicIDs {
		requested[publicID] = struct{}{}
	}

	rows, err := r.queries(ctx).ListActiveTagsByPublicIDs(
		ctx,
		sqlc.ListActiveTagsByPublicIDsParams{
			UserID:    userID.Int64(),
			PublicIds: publicIDs,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("resolve tags by public ids: %w", err)
	}
	if len(rows) != len(requested) {
		return nil, domaintag.ErrTagNotFound
	}

	references := make([]domaintag.Reference, 0, len(rows))
	for _, row := range rows {
		references = append(references, toDomainTag(row).Reference())
	}
	return references, nil
}

// Update は名称を更新する。
func (r *PostgresqlTagRepository) Update(
	ctx context.Context,
	tag domaintag.Tag,
	expectedVersion int32,
) (domaintag.Tag, error) {
	row, err := r.queries(ctx).UpdateTag(ctx, sqlc.UpdateTagParams{
		Name:            tag.Name(),
		UpdatedAt:       timestamptz(tag.UpdatedAt()),
		PublicID:        tag.PublicID(),
		UserID:          tag.UserID().Int64(),
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		if isUniqueViolation(err, tagNameUniqueIndexName) {
			return domaintag.Tag{}, domaintag.ErrTagNameAlreadyUsed.WithCause(err)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return domaintag.Tag{}, r.resolveUpdateFailure(ctx, tag.UserID(), tag.PublicID())
		}
		return domaintag.Tag{}, fmt.Errorf("update tag: %w", err)
	}
	return toDomainTag(row), nil
}

// SoftDelete はタグをsoft deleteする。
func (r *PostgresqlTagRepository) SoftDelete(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
	expectedVersion int32,
	deletedAt time.Time,
) error {
	_, err := r.queries(ctx).SoftDeleteTag(ctx, sqlc.SoftDeleteTagParams{
		DeletedAt:       timestamptz(deletedAt),
		UpdatedAt:       timestamptz(deletedAt),
		PublicID:        publicID,
		UserID:          userID.Int64(),
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return r.resolveUpdateFailure(ctx, userID, publicID)
		}
		return fmt.Errorf("soft delete tag: %w", err)
	}
	return nil
}

// CountActiveItems はタグが付与されているarchive前アイテムの件数を返す。
func (r *PostgresqlTagRepository) CountActiveItems(
	ctx context.Context,
	userID domainauth.UserID,
	id domaintag.TagID,
) (int64, error) {
	count, err := r.queries(ctx).CountActiveItemsByTagID(
		ctx,
		sqlc.CountActiveItemsByTagIDParams{TagID: id.Int64(), UserID: userID.Int64()},
	)
	if err != nil {
		return 0, fmt.Errorf("count active items by tag: %w", err)
	}
	return count, nil
}

// resolveUpdateFailure は更新件数0の理由を判定する。
//
// 対象が存在しなければ不存在、存在すればversion競合として扱う (設計書 11.7)。
func (r *PostgresqlTagRepository) resolveUpdateFailure(
	ctx context.Context,
	userID domainauth.UserID,
	publicID uuid.UUID,
) error {
	exists, err := r.queries(ctx).ExistsActiveTagByPublicID(
		ctx,
		sqlc.ExistsActiveTagByPublicIDParams{PublicID: publicID, UserID: userID.Int64()},
	)
	if err != nil {
		return fmt.Errorf("check tag existence: %w", err)
	}
	if !exists {
		return domaintag.ErrTagNotFound
	}
	return domaintag.ErrTagVersionConflict
}

func toDomainTag(row sqlc.OwnershipTag) domaintag.Tag {
	return domaintag.ReconstructTag(domaintag.ReconstructTagParams{
		ID:        domaintag.TagID(row.ID),
		PublicID:  row.PublicID,
		UserID:    domainauth.UserID(row.UserID),
		Name:      row.Name,
		CreatedAt: utcTime(row.CreatedAt),
		UpdatedAt: utcTime(row.UpdatedAt),
		DeletedAt: optionalTime(row.DeletedAt),
		Version:   row.Version,
	})
}
