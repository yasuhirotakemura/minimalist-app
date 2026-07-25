// Package crypto はpassword hash化とtoken生成の実装を提供する。
package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
)

// Argon2idParameters はArgon2idの計算parameter。
//
// 設計書はアルゴリズムのみを規定しているため、以下を既定値とする。
// OWASP Password Storage Cheat Sheetのm=64MiB, t=3, p=4を基準にした。
type Argon2idParameters struct {
	// MemoryKiB はmemory cost (KiB)。
	MemoryKiB uint32
	// Iterations はtime cost。
	Iterations uint32
	// Parallelism は並列度。
	Parallelism uint8
	// SaltLength はsaltのbyte長。
	SaltLength uint32
	// KeyLength は導出鍵のbyte長。
	KeyLength uint32
}

// DefaultArgon2idParameters は既定のArgon2id parameterを返す。
func DefaultArgon2idParameters() Argon2idParameters {
	return Argon2idParameters{
		MemoryKiB:   64 * 1024,
		Iterations:  3,
		Parallelism: 4,
		SaltLength:  16,
		KeyLength:   32,
	}
}

func (p Argon2idParameters) validate() error {
	if p.MemoryKiB < 8*1024 {
		return errors.New("argon2id: MemoryKiB must be at least 8192")
	}
	if p.Iterations < 1 {
		return errors.New("argon2id: Iterations must be at least 1")
	}
	if p.Parallelism < 1 {
		return errors.New("argon2id: Parallelism must be at least 1")
	}
	if p.SaltLength < 16 {
		return errors.New("argon2id: SaltLength must be at least 16")
	}
	if p.KeyLength < 32 {
		return errors.New("argon2id: KeyLength must be at least 32")
	}
	return nil
}

// argon2idVersion はPHC文字列へ記録するArgon2のversion。
const argon2idVersion = argon2.Version

// Argon2idPasswordHasher はArgon2idでpasswordをhash化する。
//
// PASSWORD_PEPPERの適用方法:
//
//	golang.org/x/crypto/argon2 はArgon2の secret (key) parameterを公開していない。
//	そのため HMAC-SHA256(password, pepper) を求め、その結果をArgon2idの入力とする。
//	pepperはDBへ保存しないため、DBのみが漏洩した場合の総当たり攻撃を困難にする。
//
// 保存形式はPHC文字列とする。
//
//	$argon2id$v=19$m=65536,t=3,p=4$<base64 salt>$<base64 hash>
//
// parameterをhashへ同梱するため、将来parameterを強化しても既存passwordを検証できる。
type Argon2idPasswordHasher struct {
	pepper     []byte
	parameters Argon2idParameters
}

// NewArgon2idPasswordHasher はArgon2idPasswordHasherを生成する。
func NewArgon2idPasswordHasher(
	pepper string,
	parameters Argon2idParameters,
) (*Argon2idPasswordHasher, error) {
	if len(pepper) == 0 {
		return nil, errors.New("argon2id: pepper must not be empty")
	}
	if err := parameters.validate(); err != nil {
		return nil, err
	}
	return &Argon2idPasswordHasher{
		pepper:     []byte(pepper),
		parameters: parameters,
	}, nil
}

var _ auth.PasswordHasher = (*Argon2idPasswordHasher)(nil)

// Hash は平文passwordからPHC文字列を生成する。
func (h *Argon2idPasswordHasher) Hash(password auth.RawPassword) (auth.PasswordHash, error) {
	salt := make([]byte, h.parameters.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return auth.PasswordHash{}, fmt.Errorf("argon2id: generate salt: %w", err)
	}

	key := h.deriveKey(password, salt, h.parameters)
	encoded := encodePHC(h.parameters, salt, key)

	return auth.NewPasswordHash(encoded)
}

// Verify は平文passwordがhashと一致するかを返す。
//
// 比較は定数時間で行う。hashの解析に失敗した場合のみerrorを返す。
func (h *Argon2idPasswordHasher) Verify(
	password auth.RawPassword,
	hash auth.PasswordHash,
) (bool, error) {
	parameters, salt, expectedKey, err := decodePHC(hash.Encoded())
	if err != nil {
		return false, err
	}

	actualKey := h.deriveKey(password, salt, parameters)
	return subtle.ConstantTimeCompare(actualKey, expectedKey) == 1, nil
}

// Parameters は使用中のparameterを返す。
func (h *Argon2idPasswordHasher) Parameters() Argon2idParameters {
	return h.parameters
}

func (h *Argon2idPasswordHasher) deriveKey(
	password auth.RawPassword,
	salt []byte,
	parameters Argon2idParameters,
) []byte {
	peppered := h.applyPepper(password)
	return argon2.IDKey(
		peppered,
		salt,
		parameters.Iterations,
		parameters.MemoryKiB,
		parameters.Parallelism,
		parameters.KeyLength,
	)
}

func (h *Argon2idPasswordHasher) applyPepper(password auth.RawPassword) []byte {
	mac := hmac.New(sha256.New, h.pepper)
	mac.Write([]byte(password.Expose()))
	return mac.Sum(nil)
}

func encodePHC(parameters Argon2idParameters, salt, key []byte) string {
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2idVersion,
		parameters.MemoryKiB,
		parameters.Iterations,
		parameters.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

// errInvalidPHC はPHC文字列の解析に失敗したことを表す。
var errInvalidPHC = errors.New("argon2id: invalid PHC string")

func decodePHC(encoded string) (Argon2idParameters, []byte, []byte, error) {
	// "$argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>"
	segments := strings.Split(encoded, "$")
	if len(segments) != 6 || segments[0] != "" {
		return Argon2idParameters{}, nil, nil, errInvalidPHC
	}
	if segments[1] != "argon2id" {
		return Argon2idParameters{}, nil, nil, fmt.Errorf(
			"%w: unsupported algorithm %q", errInvalidPHC, segments[1])
	}

	var version int
	if _, err := fmt.Sscanf(segments[2], "v=%d", &version); err != nil {
		return Argon2idParameters{}, nil, nil, fmt.Errorf("%w: version: %w", errInvalidPHC, err)
	}
	if version != argon2idVersion {
		return Argon2idParameters{}, nil, nil, fmt.Errorf(
			"%w: unsupported version %d", errInvalidPHC, version)
	}

	var parameters Argon2idParameters
	if _, err := fmt.Sscanf(
		segments[3],
		"m=%d,t=%d,p=%d",
		&parameters.MemoryKiB,
		&parameters.Iterations,
		&parameters.Parallelism,
	); err != nil {
		return Argon2idParameters{}, nil, nil, fmt.Errorf("%w: parameters: %w", errInvalidPHC, err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(segments[4])
	if err != nil {
		return Argon2idParameters{}, nil, nil, fmt.Errorf("%w: salt: %w", errInvalidPHC, err)
	}
	key, err := base64.RawStdEncoding.DecodeString(segments[5])
	if err != nil {
		return Argon2idParameters{}, nil, nil, fmt.Errorf("%w: hash: %w", errInvalidPHC, err)
	}
	if len(salt) == 0 || len(key) == 0 {
		return Argon2idParameters{}, nil, nil, errInvalidPHC
	}

	parameters.SaltLength = uint32(len(salt))
	parameters.KeyLength = uint32(len(key))

	return parameters, salt, key, nil
}
