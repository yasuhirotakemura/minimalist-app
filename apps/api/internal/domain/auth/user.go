package auth

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// displayNameの長さ制限。DB制約 ck_users__display_name_length と一致させる。
const (
	MinDisplayNameLength = 1
	MaxDisplayNameLength = 100
)

// 初期値 (設計書 2.2 / 13.6)。
const (
	DefaultTimezone = "Asia/Tokyo"
	DefaultLocale   = "ja-JP"
)

// UserID は内部主キー。APIへ公開しない (設計書 12.1)。
type UserID int64

// Int64 はDB問い合わせ用の値を返す。
func (id UserID) Int64() int64 {
	return int64(id)
}

// IsZero は未永続化かどうかを返す。
func (id UserID) IsZero() bool {
	return id == 0
}

// User はUser Aggregateのroot Entity。
//
// password hashは同一Aggregate内の別tableへ保持するため、Entityへは含めない。
// 認証時はRepositoryから明示的に取得する。
type User struct {
	id          UserID
	publicID    uuid.UUID
	email       Email
	displayName string
	timezone    string
	locale      string
	createdAt   time.Time
	updatedAt   time.Time
	deletedAt   *time.Time
	version     int32
}

// NewUser は未永続化のUserを生成する。内部IDはDBが払い出す。
func NewUser(
	publicID uuid.UUID,
	email Email,
	displayName string,
	timezone string,
	locale string,
	now time.Time,
) (User, error) {
	if publicID == uuid.Nil {
		return User{}, shared.NewInternalError("INVALID_PUBLIC_ID", "内部エラーが発生しました。")
	}
	if email.IsZero() {
		return User{}, newEmailError("REQUIRED", "メールアドレスを入力してください。")
	}

	normalizedDisplayName, err := normalizeDisplayName(displayName)
	if err != nil {
		return User{}, err
	}
	normalizedTimezone, err := normalizeTimezone(timezone)
	if err != nil {
		return User{}, err
	}
	normalizedLocale, err := normalizeLocale(locale)
	if err != nil {
		return User{}, err
	}

	instant := now.UTC()
	return User{
		publicID:    publicID,
		email:       email,
		displayName: normalizedDisplayName,
		timezone:    normalizedTimezone,
		locale:      normalizedLocale,
		createdAt:   instant,
		updatedAt:   instant,
		version:     1,
	}, nil
}

// ReconstructUserParams は永続化済みUserの復元に使用する。
type ReconstructUserParams struct {
	ID          UserID
	PublicID    uuid.UUID
	Email       Email
	DisplayName string
	Timezone    string
	Locale      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
	Version     int32
}

// ReconstructUser はRepositoryが取得したdataからUserを復元する。
// 復元時は業務ルールの再検証を行わず、保存済みの状態をそのまま表現する。
func ReconstructUser(params ReconstructUserParams) User {
	user := User{
		id:          params.ID,
		publicID:    params.PublicID,
		email:       params.Email,
		displayName: params.DisplayName,
		timezone:    params.Timezone,
		locale:      params.Locale,
		createdAt:   params.CreatedAt.UTC(),
		updatedAt:   params.UpdatedAt.UTC(),
		version:     params.Version,
	}
	if params.DeletedAt != nil {
		deletedAt := params.DeletedAt.UTC()
		user.deletedAt = &deletedAt
	}
	return user
}

// ID は内部主キーを返す。
func (u User) ID() UserID { return u.id }

// PublicID は外部公開IDを返す。
func (u User) PublicID() uuid.UUID { return u.publicID }

// Email はメールアドレスを返す。
func (u User) Email() Email { return u.email }

// DisplayName は表示名を返す。
func (u User) DisplayName() string { return u.displayName }

// Timezone は利用者のtimezoneを返す。
func (u User) Timezone() string { return u.timezone }

// Locale は表示言語を返す。
func (u User) Locale() string { return u.locale }

// CreatedAt は作成日時を返す。
func (u User) CreatedAt() time.Time { return u.createdAt }

// UpdatedAt は更新日時を返す。
func (u User) UpdatedAt() time.Time { return u.updatedAt }

// DeletedAt はsoft delete日時を返す。未削除の場合はnil。
func (u User) DeletedAt() *time.Time { return u.deletedAt }

// Version は楽観ロック用のversionを返す。
func (u User) Version() int32 { return u.version }

// IsDeleted はsoft delete済みかどうかを返す。
func (u User) IsDeleted() bool { return u.deletedAt != nil }

// WithID は内部主キーを設定した複製を返す。Repositoryがinsert後に使用する。
func (u User) WithID(id UserID) User {
	u.id = id
	return u
}

func normalizeDisplayName(raw string) (string, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return "", newDisplayNameError("REQUIRED", "表示名を入力してください。")
	}
	length := utf8.RuneCountInString(normalized)
	if length < MinDisplayNameLength || length > MaxDisplayNameLength {
		return "", newDisplayNameError("INVALID_LENGTH", "表示名は1文字以上100文字以内で入力してください。")
	}
	return normalized, nil
}

func normalizeTimezone(raw string) (string, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return DefaultTimezone, nil
	}
	if _, err := time.LoadLocation(normalized); err != nil {
		return "", shared.NewInvalidInputError("INVALID_TIMEZONE", "タイムゾーンの指定が正しくありません。").
			WithFieldErrors(shared.NewFieldError("timezone", "UNKNOWN_TIMEZONE", "タイムゾーンの指定が正しくありません。")).
			WithCause(err)
	}
	return normalized, nil
}

func normalizeLocale(raw string) (string, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return DefaultLocale, nil
	}
	if utf8.RuneCountInString(normalized) > 16 {
		return "", shared.NewInvalidInputError("INVALID_LOCALE", "表示言語の指定が正しくありません。").
			WithFieldErrors(shared.NewFieldError("locale", "INVALID_LENGTH", "表示言語の指定が正しくありません。"))
	}
	return normalized, nil
}

func newDisplayNameError(code, message string) *shared.DomainError {
	return shared.NewInvalidInputError("INVALID_DISPLAY_NAME", message).
		WithFieldErrors(shared.NewFieldError("displayName", code, message))
}
