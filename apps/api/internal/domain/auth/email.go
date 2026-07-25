package auth

import (
	"strings"
	"unicode"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// emailの長さ制限。RFC 5321のpath上限に合わせる。
const (
	MinEmailLength = 3
	MaxEmailLength = 254
)

// Email はlowercase正規化済みのメールアドレスを表すValueObject。
//
// DBでも lowercase 保持を制約で保証する (ck_users__email_lowercase)。
type Email struct {
	value string
}

// NewEmail は入力を正規化・検証してEmailを生成する。
//
// 正規化は前後空白の除去とlowercase化のみ行う。
// local部の大文字小文字を区別する実装も存在するが、本アプリケーションでは
// 1アカウント1利用者であり、利用者の混乱を避けるため区別しない。
func NewEmail(raw string) (Email, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))

	if normalized == "" {
		return Email{}, newEmailError("REQUIRED", "メールアドレスを入力してください。")
	}
	if len(normalized) < MinEmailLength || len(normalized) > MaxEmailLength {
		return Email{}, newEmailError("INVALID_LENGTH", "メールアドレスの長さが正しくありません。")
	}
	if strings.ContainsFunc(normalized, unicode.IsSpace) {
		return Email{}, newEmailError("INVALID_FORMAT", "メールアドレスに空白を含めることはできません。")
	}

	localPart, domainPart, found := strings.Cut(normalized, "@")
	if !found || strings.Contains(domainPart, "@") {
		return Email{}, newEmailError("INVALID_FORMAT", "メールアドレスの形式が正しくありません。")
	}
	if localPart == "" || domainPart == "" {
		return Email{}, newEmailError("INVALID_FORMAT", "メールアドレスの形式が正しくありません。")
	}
	if !strings.Contains(domainPart, ".") || strings.HasPrefix(domainPart, ".") || strings.HasSuffix(domainPart, ".") {
		return Email{}, newEmailError("INVALID_FORMAT", "メールアドレスのドメイン部が正しくありません。")
	}
	if strings.Contains(normalized, "..") {
		return Email{}, newEmailError("INVALID_FORMAT", "メールアドレスの形式が正しくありません。")
	}

	return Email{value: normalized}, nil
}

// MustNewEmail はEmailを生成し、失敗した場合にpanicする。test・seed専用。
func MustNewEmail(raw string) Email {
	email, err := NewEmail(raw)
	if err != nil {
		panic(err)
	}
	return email
}

// String は正規化済みのメールアドレスを返す。
func (e Email) String() string {
	return e.value
}

// IsZero は未設定かどうかを返す。
func (e Email) IsZero() bool {
	return e.value == ""
}

func newEmailError(code, message string) *shared.DomainError {
	return shared.NewInvalidInputError("INVALID_EMAIL", message).
		WithFieldErrors(shared.NewFieldError("email", code, message))
}
