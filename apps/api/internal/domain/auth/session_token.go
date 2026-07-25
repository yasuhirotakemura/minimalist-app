package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// session tokenの長さ。
//
// 生成時は32byteのrandom値をbase64url (padding無し) へ符号化するため43文字となる。
// 受入時は将来の長さ変更に備え範囲で検証する。
const (
	SessionTokenByteLength = 32
	MinSessionTokenLength  = 32
	MaxSessionTokenLength  = 512
)

// SessionTokenHashByteLength はSHA-256のbyte長。DB制約と一致させる。
const SessionTokenHashByteLength = sha256.Size

// SessionToken はCookieへ格納するsession tokenを表すValueObject。
//
// 暗号学的に安全なrandom値であり、DBへは保存しない (設計書 18.1)。
// 誤出力を防ぐためString()はmask値を返す。
type SessionToken struct {
	value string
}

// NewSessionToken は文字列からSessionTokenを生成する。
func NewSessionToken(raw string) (SessionToken, error) {
	if raw == "" {
		return SessionToken{}, ErrUnauthenticated
	}
	if len(raw) < MinSessionTokenLength || len(raw) > MaxSessionTokenLength {
		return SessionToken{}, ErrUnauthenticated
	}
	return SessionToken{value: raw}, nil
}

// Expose はCookieへ設定するためのtoken文字列を返す。
// Cookie書き込み以外で呼び出してはならない。
func (t SessionToken) Expose() string {
	return t.value
}

// String はmask値を返す。
func (t SessionToken) String() string {
	return redactedText
}

// LogValue はslog.LogValuerを満たし、logへmask値を出力する。
func (t SessionToken) LogValue() slog.Value {
	return slog.StringValue(redactedText)
}

// Hash はtokenのSHA-256 hashを返す。DB検索にはこの値のみを使用する。
func (t SessionToken) Hash() SessionTokenHash {
	return SessionTokenHash{value: sha256.Sum256([]byte(t.value))}
}

// SessionTokenHash はsession tokenのSHA-256 hashを表すValueObject。
type SessionTokenHash struct {
	value [SessionTokenHashByteLength]byte
}

// NewSessionTokenHash はbyte列からSessionTokenHashを生成する。
// repositoryがDBの値を復元する際に使用する。
func NewSessionTokenHash(raw []byte) (SessionTokenHash, error) {
	if len(raw) != SessionTokenHashByteLength {
		return SessionTokenHash{}, shared.NewInternalError(
			"INVALID_SESSION_TOKEN_HASH", "セッション情報の形式が正しくありません。")
	}
	var hash SessionTokenHash
	copy(hash.value[:], raw)
	return hash, nil
}

// Bytes は永続化用のbyte列を返す。
func (h SessionTokenHash) Bytes() []byte {
	out := make([]byte, SessionTokenHashByteLength)
	copy(out, h.value[:])
	return out
}

// Equals は定数時間で比較する。
func (h SessionTokenHash) Equals(other SessionTokenHash) bool {
	return subtle.ConstantTimeCompare(h.value[:], other.value[:]) == 1
}

// IsZero は未設定かどうかを返す。
func (h SessionTokenHash) IsZero() bool {
	return h == SessionTokenHash{}
}

// SessionTokenGenerator はsession tokenを生成する。
// 実装はinfrastructure layerへ置く (crypto/rand)。
type SessionTokenGenerator interface {
	Generate() (SessionToken, error)
}
