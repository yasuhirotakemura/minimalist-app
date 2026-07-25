package auth

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// SessionTouchInterval は last_used_at を更新する最小間隔。
//
// requestごとにUPDATEを発行すると書き込み負荷が高くなるため、
// 一定時間が経過した場合のみ更新する。
const SessionTouchInterval = 5 * time.Minute

// MaxUserAgentLength はuser_agentの保存上限。DB制約と一致させる。
const MaxUserAgentLength = 512

// AuthSession はsessionを表すEntity。
//
// token本体は保持せず、SHA-256 hashのみを保持する (設計書 18.1)。
type AuthSession struct {
	id         int64
	publicID   uuid.UUID
	userID     UserID
	tokenHash  SessionTokenHash
	issuedAt   time.Time
	expiresAt  time.Time
	lastUsedAt time.Time
	revokedAt  *time.Time
	userAgent  string
	ipAddress  string
}

// IssueAuthSessionParams はsession発行の入力。
type IssueAuthSessionParams struct {
	PublicID  uuid.UUID
	UserID    UserID
	Token     SessionToken
	IssuedAt  time.Time
	TTL       time.Duration
	UserAgent string
	IPAddress string
}

// IssueAuthSession は新しいsessionを発行する。
func IssueAuthSession(params IssueAuthSessionParams) (AuthSession, error) {
	if params.PublicID == uuid.Nil {
		return AuthSession{}, shared.NewInternalError("INVALID_PUBLIC_ID", "内部エラーが発生しました。")
	}
	if params.UserID.IsZero() {
		return AuthSession{}, shared.NewInternalError("INVALID_USER_ID", "内部エラーが発生しました。")
	}
	if params.TTL <= 0 {
		return AuthSession{}, shared.NewInternalError("INVALID_SESSION_TTL", "内部エラーが発生しました。")
	}

	tokenHash := params.Token.Hash()
	if tokenHash.IsZero() {
		return AuthSession{}, shared.NewInternalError("INVALID_SESSION_TOKEN", "内部エラーが発生しました。")
	}

	issuedAt := params.IssuedAt.UTC()
	return AuthSession{
		publicID:   params.PublicID,
		userID:     params.UserID,
		tokenHash:  tokenHash,
		issuedAt:   issuedAt,
		expiresAt:  issuedAt.Add(params.TTL),
		lastUsedAt: issuedAt,
		userAgent:  truncateUserAgent(params.UserAgent),
		ipAddress:  strings.TrimSpace(params.IPAddress),
	}, nil
}

// ReconstructAuthSessionParams は永続化済みsessionの復元に使用する。
type ReconstructAuthSessionParams struct {
	ID         int64
	PublicID   uuid.UUID
	UserID     UserID
	TokenHash  SessionTokenHash
	IssuedAt   time.Time
	ExpiresAt  time.Time
	LastUsedAt time.Time
	RevokedAt  *time.Time
	UserAgent  string
	IPAddress  string
}

// ReconstructAuthSession はRepositoryが取得したdataからAuthSessionを復元する。
func ReconstructAuthSession(params ReconstructAuthSessionParams) AuthSession {
	session := AuthSession{
		id:         params.ID,
		publicID:   params.PublicID,
		userID:     params.UserID,
		tokenHash:  params.TokenHash,
		issuedAt:   params.IssuedAt.UTC(),
		expiresAt:  params.ExpiresAt.UTC(),
		lastUsedAt: params.LastUsedAt.UTC(),
		userAgent:  params.UserAgent,
		ipAddress:  params.IPAddress,
	}
	if params.RevokedAt != nil {
		revokedAt := params.RevokedAt.UTC()
		session.revokedAt = &revokedAt
	}
	return session
}

// ID は内部主キーを返す。
func (s AuthSession) ID() int64 { return s.id }

// PublicID は外部公開IDを返す。
func (s AuthSession) PublicID() uuid.UUID { return s.publicID }

// UserID は所有ユーザーの内部IDを返す。
func (s AuthSession) UserID() UserID { return s.userID }

// TokenHash はsession token hashを返す。
func (s AuthSession) TokenHash() SessionTokenHash { return s.tokenHash }

// IssuedAt は発行日時を返す。
func (s AuthSession) IssuedAt() time.Time { return s.issuedAt }

// ExpiresAt は有効期限を返す。
func (s AuthSession) ExpiresAt() time.Time { return s.expiresAt }

// LastUsedAt は最終利用日時を返す。
func (s AuthSession) LastUsedAt() time.Time { return s.lastUsedAt }

// RevokedAt は失効日時を返す。有効な場合はnil。
func (s AuthSession) RevokedAt() *time.Time { return s.revokedAt }

// UserAgent はlogin時のUser-Agentを返す。
func (s AuthSession) UserAgent() string { return s.userAgent }

// IPAddress はlogin時のIPアドレスを返す。空文字の場合は未記録。
func (s AuthSession) IPAddress() string { return s.ipAddress }

// WithID は内部主キーを設定した複製を返す。
func (s AuthSession) WithID(id int64) AuthSession {
	s.id = id
	return s
}

// IsRevoked は失効済みかどうかを返す。
func (s AuthSession) IsRevoked() bool { return s.revokedAt != nil }

// IsExpired は有効期限を過ぎているかどうかを返す。
// 有効期限と同時刻は期限切れとして扱う。
func (s AuthSession) IsExpired(now time.Time) bool {
	return !now.UTC().Before(s.expiresAt)
}

// IsLive は認証に使用できるかどうかを返す。
func (s AuthSession) IsLive(now time.Time) bool {
	return !s.IsRevoked() && !s.IsExpired(now)
}

// NeedsLastUsedAtRefresh は last_used_at を更新すべきかどうかを返す。
func (s AuthSession) NeedsLastUsedAtRefresh(now time.Time) bool {
	return now.UTC().Sub(s.lastUsedAt) >= SessionTouchInterval
}

func truncateUserAgent(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if utf8.RuneCountInString(trimmed) <= MaxUserAgentLength {
		return trimmed
	}
	runes := []rune(trimmed)
	return string(runes[:MaxUserAgentLength])
}
