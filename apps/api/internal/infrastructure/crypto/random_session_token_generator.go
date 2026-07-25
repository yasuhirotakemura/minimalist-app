package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
)

// RandomSessionTokenGenerator は暗号学的に安全なrandom値からsession tokenを生成する
// (設計書 18.1)。
//
// 32byteのrandom値をbase64url (padding無し) へ符号化する。
// Cookie値として安全な文字のみを含む。
type RandomSessionTokenGenerator struct{}

// NewRandomSessionTokenGenerator はRandomSessionTokenGeneratorを返す。
func NewRandomSessionTokenGenerator() RandomSessionTokenGenerator {
	return RandomSessionTokenGenerator{}
}

var _ auth.SessionTokenGenerator = RandomSessionTokenGenerator{}

// Generate はsession tokenを生成する。
func (RandomSessionTokenGenerator) Generate() (auth.SessionToken, error) {
	raw := make([]byte, auth.SessionTokenByteLength)
	if _, err := rand.Read(raw); err != nil {
		return auth.SessionToken{}, fmt.Errorf("generate session token: %w", err)
	}
	return auth.NewSessionToken(base64.RawURLEncoding.EncodeToString(raw))
}
