package auth

import (
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// passwordの長さ制限。OpenAPIのschemaと一致させる。
//
// 上限を設けるのは、極端に長い入力によるhash計算の負荷を防ぐためである。
const (
	MinPasswordLength = 12
	MaxPasswordLength = 128
)

// redactedText はlogや文字列化で平文を露出させないための代替文字列。
const redactedText = "[REDACTED]"

// RawPassword は検証済みの平文passwordを表すValueObject。
//
// 誤ってlogへ出力されないよう、String()は常にmask値を返す。
// hash化のための値取得は Expose() を明示的に呼ぶ必要がある。
type RawPassword struct {
	value string
}

// NewRawPassword は平文passwordを検証してRawPasswordを生成する。
//
// 前後空白は除去しない。passphraseの一部として空白を使う利用者を想定する。
func NewRawPassword(raw string) (RawPassword, error) {
	if raw == "" {
		return RawPassword{}, newPasswordError("REQUIRED", "パスワードを入力してください。")
	}
	length := utf8.RuneCountInString(raw)
	if length < MinPasswordLength {
		return RawPassword{}, newPasswordError("TOO_SHORT", "パスワードは12文字以上で入力してください。")
	}
	if length > MaxPasswordLength {
		return RawPassword{}, newPasswordError("TOO_LONG", "パスワードは128文字以内で入力してください。")
	}
	if strings.TrimSpace(raw) == "" {
		return RawPassword{}, newPasswordError("INVALID_FORMAT", "パスワードに空白のみを設定することはできません。")
	}
	return RawPassword{value: raw}, nil
}

// NewRawPasswordForVerification は長さ制約を緩めてRawPasswordを生成する。
//
// login時の入力へ登録時と同じ最小長制約を課すと、制約を変更した過去のpasswordで
// loginできなくなる。login時は上限のみを確認し、一致判定はhash比較へ委ねる。
func NewRawPasswordForVerification(raw string) (RawPassword, error) {
	if raw == "" {
		return RawPassword{}, newPasswordError("REQUIRED", "パスワードを入力してください。")
	}
	if utf8.RuneCountInString(raw) > MaxPasswordLength {
		return RawPassword{}, newPasswordError("TOO_LONG", "パスワードは128文字以内で入力してください。")
	}
	return RawPassword{value: raw}, nil
}

// Expose は平文passwordを返す。hash化・検証以外で呼び出してはならない。
func (p RawPassword) Expose() string {
	return p.value
}

// String はmask値を返す。log・fmtへの誤出力を防ぐ。
func (p RawPassword) String() string {
	return redactedText
}

// LogValue はslog.LogValuerを満たし、logへmask値を出力する。
func (p RawPassword) LogValue() slog.Value {
	return slog.StringValue(redactedText)
}

// PasswordHash はhash化済みpasswordを表すValueObject。
//
// 値はArgon2idのPHC文字列とする。
// 例: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
type PasswordHash struct {
	value string
}

// PasswordHashPrefix はArgon2idのPHC文字列prefix。DB制約と一致させる。
const PasswordHashPrefix = "$argon2id$"

// NewPasswordHash はPHC文字列を検証してPasswordHashを生成する。
func NewPasswordHash(encoded string) (PasswordHash, error) {
	if encoded == "" {
		return PasswordHash{}, shared.NewInternalError(
			"INVALID_PASSWORD_HASH", "認証情報の形式が正しくありません。")
	}
	if !strings.HasPrefix(encoded, PasswordHashPrefix) {
		return PasswordHash{}, shared.NewInternalError(
			"UNSUPPORTED_PASSWORD_HASH_ALGORITHM", "認証情報の形式が正しくありません。")
	}
	return PasswordHash{value: encoded}, nil
}

// Encoded は永続化用のPHC文字列を返す。
func (h PasswordHash) Encoded() string {
	return h.value
}

// String はmask値を返す。
func (h PasswordHash) String() string {
	return redactedText
}

// LogValue はslog.LogValuerを満たし、logへmask値を出力する。
func (h PasswordHash) LogValue() slog.Value {
	return slog.StringValue(redactedText)
}

// IsZero は未設定かどうかを返す。
func (h PasswordHash) IsZero() bool {
	return h.value == ""
}

// PasswordHasher はpasswordのhash化と検証を行う。
// 実装はinfrastructure layerへ置く (Argon2id)。
type PasswordHasher interface {
	// Hash は平文passwordからPasswordHashを生成する。
	Hash(password RawPassword) (PasswordHash, error)
	// Verify は平文passwordがhashと一致するかを返す。
	// 一致しない場合は (false, nil) を返し、hashの解析に失敗した場合のみerrorを返す。
	Verify(password RawPassword, hash PasswordHash) (bool, error)
}

func newPasswordError(code, message string) *shared.DomainError {
	return shared.NewInvalidInputError("INVALID_PASSWORD", message).
		WithFieldErrors(shared.NewFieldError("password", code, message))
}
