package auth

import "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"

// auth aggregateのDomain error (設計書 19.1)。
// 呼び出し側は errors.Is で判定する。DomainError.Is はCodeで一致判定する。
var (
	// ErrEmailAlreadyRegistered は同一emailのユーザーが既に存在することを表す。
	ErrEmailAlreadyRegistered = shared.NewConflictError(
		"EMAIL_ALREADY_REGISTERED",
		"このメールアドレスは既に登録されています。",
	)

	// ErrInvalidCredentials はemailまたはpasswordが正しくないことを表す。
	// emailの存在有無を推測させないため、両者を同一のerrorとして扱う。
	ErrInvalidCredentials = shared.NewUnauthenticatedError(
		"INVALID_CREDENTIALS",
		"メールアドレスまたはパスワードが正しくありません。",
	)

	// ErrUnauthenticated は有効なsessionが存在しないことを表す。
	ErrUnauthenticated = shared.NewUnauthenticatedError(
		"UNAUTHENTICATED",
		"ログインが必要です。",
	)

	// ErrUserNotFound はユーザーが存在しないことを表す。
	// 他ユーザーのpublicIdを指定した場合も本errorを返し、存在有無を公開しない (設計書 18.3)。
	ErrUserNotFound = shared.NewNotFoundError(
		"USER_NOT_FOUND",
		"ユーザーが見つかりません。",
	)

	// ErrAuthSessionNotFound は有効なsessionが存在しないことを表す。
	ErrAuthSessionNotFound = shared.NewNotFoundError(
		"AUTH_SESSION_NOT_FOUND",
		"セッションが見つかりません。",
	)

	// ErrPasswordAuthNotFound はpassword認証情報が存在しないことを表す。
	ErrPasswordAuthNotFound = shared.NewNotFoundError(
		"PASSWORD_AUTH_NOT_FOUND",
		"認証情報が見つかりません。",
	)
)
