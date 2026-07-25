package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/sqlc"
	infrapostgresql "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/postgresql"
)

// PostgresqlAuthSessionRepository はAuthSessionRepositoryのPostgreSQL実装。
type PostgresqlAuthSessionRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresqlAuthSessionRepository はPostgresqlAuthSessionRepositoryを生成する。
func NewPostgresqlAuthSessionRepository(pool *pgxpool.Pool) *PostgresqlAuthSessionRepository {
	return &PostgresqlAuthSessionRepository{pool: pool}
}

var _ domainauth.AuthSessionRepository = (*PostgresqlAuthSessionRepository)(nil)

func (r *PostgresqlAuthSessionRepository) queries(ctx context.Context) *sqlc.Queries {
	return sqlc.New(infrapostgresql.Querier(ctx, r.pool))
}

// Create はsessionを作成する。
func (r *PostgresqlAuthSessionRepository) Create(
	ctx context.Context,
	session domainauth.AuthSession,
) (domainauth.AuthSession, error) {
	row, err := r.queries(ctx).InsertAuthSession(ctx, sqlc.InsertAuthSessionParams{
		PublicID:   session.PublicID(),
		UserID:     session.UserID().Int64(),
		TokenHash:  session.TokenHash().Bytes(),
		IssuedAt:   timestamptz(session.IssuedAt()),
		ExpiresAt:  timestamptz(session.ExpiresAt()),
		LastUsedAt: timestamptz(session.LastUsedAt()),
		UserAgent:  optionalString(session.UserAgent()),
		IpAddress:  optionalIPAddress(session.IPAddress()),
		CreatedAt:  timestamptz(session.IssuedAt()),
		UpdatedAt:  timestamptz(session.IssuedAt()),
	})
	if err != nil {
		return domainauth.AuthSession{}, fmt.Errorf("insert auth session: %w", err)
	}

	tokenHash, err := domainauth.NewSessionTokenHash(row.TokenHash)
	if err != nil {
		return domainauth.AuthSession{}, fmt.Errorf("restore session token hash: %w", err)
	}

	return domainauth.ReconstructAuthSession(domainauth.ReconstructAuthSessionParams{
		ID:         row.ID,
		PublicID:   row.PublicID,
		UserID:     domainauth.UserID(row.UserID),
		TokenHash:  tokenHash,
		IssuedAt:   utcTime(row.IssuedAt),
		ExpiresAt:  utcTime(row.ExpiresAt),
		LastUsedAt: utcTime(row.LastUsedAt),
		RevokedAt:  optionalTime(row.RevokedAt),
		UserAgent:  stringValue(row.UserAgent),
		IPAddress:  ipAddressValue(row.IpAddress),
	}), nil
}

// FindLiveWithUserByTokenHash は有効なsessionと所有ユーザーを取得する。
//
// queryは revoked_at IS NULL / expires_at > evaluatedAt / users.deleted_at IS NULL を
// 条件に含めるため、失効・期限切れ・退会済みは全て「存在しない」として扱う。
func (r *PostgresqlAuthSessionRepository) FindLiveWithUserByTokenHash(
	ctx context.Context,
	tokenHash domainauth.SessionTokenHash,
	evaluatedAt time.Time,
) (domainauth.AuthSession, domainauth.User, error) {
	row, err := r.queries(ctx).FindLiveAuthSessionWithUserByTokenHash(
		ctx,
		sqlc.FindLiveAuthSessionWithUserByTokenHashParams{
			TokenHash:   tokenHash.Bytes(),
			EvaluatedAt: timestamptz(evaluatedAt),
		},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainauth.AuthSession{}, domainauth.User{}, domainauth.ErrAuthSessionNotFound
		}
		return domainauth.AuthSession{}, domainauth.User{},
			fmt.Errorf("find live auth session: %w", err)
	}

	email, err := domainauth.NewEmail(row.Email)
	if err != nil {
		return domainauth.AuthSession{}, domainauth.User{},
			fmt.Errorf("restore user email: %w", err)
	}

	session := domainauth.ReconstructAuthSession(domainauth.ReconstructAuthSessionParams{
		ID:         row.SessionID,
		PublicID:   row.SessionPublicID,
		UserID:     domainauth.UserID(row.UserID),
		TokenHash:  tokenHash,
		IssuedAt:   utcTime(row.IssuedAt),
		ExpiresAt:  utcTime(row.ExpiresAt),
		LastUsedAt: utcTime(row.LastUsedAt),
	})

	user := domainauth.ReconstructUser(domainauth.ReconstructUserParams{
		ID:          domainauth.UserID(row.UserID),
		PublicID:    row.UserPublicID,
		Email:       email,
		DisplayName: row.DisplayName,
		Timezone:    row.Timezone,
		Locale:      row.Locale,
		CreatedAt:   utcTime(row.UserCreatedAt),
		UpdatedAt:   utcTime(row.UserUpdatedAt),
		Version:     row.UserVersion,
	})

	return session, user, nil
}

// RefreshLastUsedAt は last_used_at を更新する。
func (r *PostgresqlAuthSessionRepository) RefreshLastUsedAt(
	ctx context.Context,
	tokenHash domainauth.SessionTokenHash,
	usedAt time.Time,
) error {
	if _, err := r.queries(ctx).TouchAuthSessionLastUsedAt(
		ctx,
		sqlc.TouchAuthSessionLastUsedAtParams{
			LastUsedAt: timestamptz(usedAt),
			UpdatedAt:  timestamptz(usedAt),
			TokenHash:  tokenHash.Bytes(),
		},
	); err != nil {
		return fmt.Errorf("touch auth session last_used_at: %w", err)
	}
	return nil
}

// RevokeByTokenHash はsessionを失効させる。
//
// 更新件数が0でもerrorとしない。logoutは冪等である必要がある。
func (r *PostgresqlAuthSessionRepository) RevokeByTokenHash(
	ctx context.Context,
	tokenHash domainauth.SessionTokenHash,
	revokedAt time.Time,
) error {
	if _, err := r.queries(ctx).RevokeAuthSessionByTokenHash(
		ctx,
		sqlc.RevokeAuthSessionByTokenHashParams{
			RevokedAt: timestamptz(revokedAt),
			UpdatedAt: timestamptz(revokedAt),
			TokenHash: tokenHash.Bytes(),
		},
	); err != nil {
		return fmt.Errorf("revoke auth session: %w", err)
	}
	return nil
}

// RevokeAllByUserID は指定ユーザーの全sessionを失効させる。
func (r *PostgresqlAuthSessionRepository) RevokeAllByUserID(
	ctx context.Context,
	id domainauth.UserID,
	revokedAt time.Time,
) (int64, error) {
	affected, err := r.queries(ctx).RevokeAllAuthSessionsByUserID(
		ctx,
		sqlc.RevokeAllAuthSessionsByUserIDParams{
			RevokedAt: timestamptz(revokedAt),
			UpdatedAt: timestamptz(revokedAt),
			UserID:    id.Int64(),
		},
	)
	if err != nil {
		return 0, fmt.Errorf("revoke all auth sessions: %w", err)
	}
	return affected, nil
}

// DeleteExpired は有効期限を過ぎたsessionを削除する。
func (r *PostgresqlAuthSessionRepository) DeleteExpired(
	ctx context.Context,
	expiredBefore time.Time,
) (int64, error) {
	affected, err := r.queries(ctx).DeleteExpiredAuthSessions(ctx, timestamptz(expiredBefore))
	if err != nil {
		return 0, fmt.Errorf("delete expired auth sessions: %w", err)
	}
	return affected, nil
}
