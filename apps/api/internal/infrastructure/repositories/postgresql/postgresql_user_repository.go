package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/sqlc"
	infrapostgresql "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/postgresql"
)

// PostgreSQLのunique制約違反を表すSQLSTATE。
const uniqueViolationCode = "23505"

// email重複時に返されるconstraint名。migrationの定義と一致させる。
const emailUniqueIndexName = "uq_users__email_active"

// PostgresqlUserRepository はUserRepositoryのPostgreSQL実装。
type PostgresqlUserRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresqlUserRepository はPostgresqlUserRepositoryを生成する。
func NewPostgresqlUserRepository(pool *pgxpool.Pool) *PostgresqlUserRepository {
	return &PostgresqlUserRepository{pool: pool}
}

var _ domainauth.UserRepository = (*PostgresqlUserRepository)(nil)

func (r *PostgresqlUserRepository) queries(ctx context.Context) *sqlc.Queries {
	return sqlc.New(infrapostgresql.Querier(ctx, r.pool))
}

// ExistsActiveByEmail は有効なユーザーが存在するかを返す。
func (r *PostgresqlUserRepository) ExistsActiveByEmail(
	ctx context.Context,
	email domainauth.Email,
) (bool, error) {
	exists, err := r.queries(ctx).ExistsActiveUserByEmail(ctx, email.String())
	if err != nil {
		return false, fmt.Errorf("check user existence by email: %w", err)
	}
	return exists, nil
}

// FindActiveByEmail は有効なユーザーを取得する。
func (r *PostgresqlUserRepository) FindActiveByEmail(
	ctx context.Context,
	email domainauth.Email,
) (domainauth.User, error) {
	row, err := r.queries(ctx).FindActiveUserByEmail(ctx, email.String())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainauth.User{}, domainauth.ErrUserNotFound
		}
		return domainauth.User{}, fmt.Errorf("find user by email: %w", err)
	}
	return toDomainUser(row)
}

// FindActiveByID は有効なユーザーを内部IDで取得する。
func (r *PostgresqlUserRepository) FindActiveByID(
	ctx context.Context,
	id domainauth.UserID,
) (domainauth.User, error) {
	row, err := r.queries(ctx).FindActiveUserByID(ctx, id.Int64())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainauth.User{}, domainauth.ErrUserNotFound
		}
		return domainauth.User{}, fmt.Errorf("find user by id: %w", err)
	}
	return toDomainUser(row)
}

// Create はユーザーとpassword認証情報を作成する。
//
// 呼び出し元がtransaction境界を制御する。両者は必ず同一transactionで作成する。
func (r *PostgresqlUserRepository) Create(
	ctx context.Context,
	user domainauth.User,
	passwordHash domainauth.PasswordHash,
) (domainauth.User, error) {
	queries := r.queries(ctx)

	inserted, err := queries.InsertUser(ctx, sqlc.InsertUserParams{
		PublicID:    user.PublicID(),
		Email:       user.Email().String(),
		DisplayName: user.DisplayName(),
		Timezone:    user.Timezone(),
		Locale:      user.Locale(),
		CreatedAt:   timestamptz(user.CreatedAt()),
		UpdatedAt:   timestamptz(user.UpdatedAt()),
	})
	if err != nil {
		if isUniqueViolation(err, emailUniqueIndexName) {
			return domainauth.User{}, domainauth.ErrEmailAlreadyRegistered.WithCause(err)
		}
		return domainauth.User{}, fmt.Errorf("insert user: %w", err)
	}

	if _, err := queries.InsertUserPasswordAuth(ctx, sqlc.InsertUserPasswordAuthParams{
		UserID:            inserted.ID,
		PasswordHash:      passwordHash.Encoded(),
		Algorithm:         "argon2id",
		PasswordUpdatedAt: timestamptz(user.CreatedAt()),
		CreatedAt:         timestamptz(user.CreatedAt()),
		UpdatedAt:         timestamptz(user.UpdatedAt()),
	}); err != nil {
		return domainauth.User{}, fmt.Errorf("insert user password auth: %w", err)
	}

	return toDomainUser(inserted)
}

// FindPasswordHashByUserID はpassword認証情報を取得する。
func (r *PostgresqlUserRepository) FindPasswordHashByUserID(
	ctx context.Context,
	id domainauth.UserID,
) (domainauth.PasswordHash, error) {
	row, err := r.queries(ctx).FindUserPasswordAuthByUserID(ctx, id.Int64())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainauth.PasswordHash{}, domainauth.ErrPasswordAuthNotFound
		}
		return domainauth.PasswordHash{}, fmt.Errorf("find user password auth: %w", err)
	}
	return domainauth.NewPasswordHash(row.PasswordHash)
}

func toDomainUser(row sqlc.IdentityUser) (domainauth.User, error) {
	email, err := domainauth.NewEmail(row.Email)
	if err != nil {
		return domainauth.User{}, fmt.Errorf("restore user email: %w", err)
	}

	return domainauth.ReconstructUser(domainauth.ReconstructUserParams{
		ID:          domainauth.UserID(row.ID),
		PublicID:    row.PublicID,
		Email:       email,
		DisplayName: row.DisplayName,
		Timezone:    row.Timezone,
		Locale:      row.Locale,
		CreatedAt:   utcTime(row.CreatedAt),
		UpdatedAt:   utcTime(row.UpdatedAt),
		DeletedAt:   optionalTime(row.DeletedAt),
		Version:     row.Version,
	}), nil
}

// isUniqueViolation は指定constraintのunique制約違反かどうかを返す。
func isUniqueViolation(err error, constraintName string) bool {
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) {
		return false
	}
	if pgError.Code != uniqueViolationCode {
		return false
	}
	return constraintName == "" || pgError.ConstraintName == constraintName
}
