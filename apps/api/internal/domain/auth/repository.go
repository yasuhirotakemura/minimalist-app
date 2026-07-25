package auth

import (
	"context"
	"time"
)

// UserRepository はUser Aggregateの永続化を担う。
//
// interfaceはdomainへ置き、PostgreSQL実装はinfrastructureへ置く (設計書 11.6)。
// 実装は全てのqueryで deleted_at IS NULL を条件へ含める。
type UserRepository interface {
	// ExistsActiveByEmail は有効なユーザーが存在するかを返す。
	ExistsActiveByEmail(ctx context.Context, email Email) (bool, error)

	// FindActiveByEmail は有効なユーザーを取得する。
	// 存在しない場合は ErrUserNotFound を返す。
	FindActiveByEmail(ctx context.Context, email Email) (User, error)

	// FindActiveByID は有効なユーザーを内部IDで取得する。
	// 存在しない場合は ErrUserNotFound を返す。
	FindActiveByID(ctx context.Context, id UserID) (User, error)

	// Create はユーザーとpassword認証情報を作成し、内部IDを付与したUserを返す。
	// email重複時は ErrEmailAlreadyRegistered を返す。
	Create(ctx context.Context, user User, passwordHash PasswordHash) (User, error)

	// FindPasswordHashByUserID はpassword認証情報を取得する。
	// 存在しない場合は ErrPasswordAuthNotFound を返す。
	FindPasswordHashByUserID(ctx context.Context, id UserID) (PasswordHash, error)
}

// AuthSessionRepository はsessionの永続化を担う。
type AuthSessionRepository interface {
	// Create はsessionを作成し、内部IDを付与したAuthSessionを返す。
	Create(ctx context.Context, session AuthSession) (AuthSession, error)

	// FindLiveWithUserByTokenHash は有効なsessionと所有ユーザーを取得する。
	// 有効なsessionが存在しない場合は ErrAuthSessionNotFound を返す。
	FindLiveWithUserByTokenHash(
		ctx context.Context,
		tokenHash SessionTokenHash,
		evaluatedAt time.Time,
	) (AuthSession, User, error)

	// RefreshLastUsedAt は last_used_at を更新する。
	RefreshLastUsedAt(ctx context.Context, tokenHash SessionTokenHash, usedAt time.Time) error

	// RevokeByTokenHash はsessionを失効させる。
	// 対象が存在しない、または既に失効済みの場合もerrorとしない (冪等)。
	RevokeByTokenHash(ctx context.Context, tokenHash SessionTokenHash, revokedAt time.Time) error

	// RevokeAllByUserID は指定ユーザーの全sessionを失効させ、件数を返す。
	RevokeAllByUserID(ctx context.Context, id UserID, revokedAt time.Time) (int64, error)

	// DeleteExpired は有効期限を過ぎたsessionを削除し、件数を返す。
	DeleteExpired(ctx context.Context, expiredBefore time.Time) (int64, error)
}
