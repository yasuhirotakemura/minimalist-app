package crypto_test

import (
	"encoding/base64"
	"testing"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/crypto"
)

func TestRandomSessionTokenGenerator_毎回異なるtokenを生成する(t *testing.T) {
	t.Parallel()

	generator := crypto.NewRandomSessionTokenGenerator()
	seen := make(map[string]struct{}, 100)

	for range 100 {
		token, err := generator.Generate()
		if err != nil {
			t.Fatalf("Generate returned error: %v", err)
		}

		raw := token.Expose()
		if _, duplicated := seen[raw]; duplicated {
			t.Fatal("同一tokenが2回生成された")
		}
		seen[raw] = struct{}{}

		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			t.Fatalf("tokenがbase64urlとして復号できない: %v", err)
		}
		if len(decoded) != auth.SessionTokenByteLength {
			t.Errorf("token byte長 = %d, want %d", len(decoded), auth.SessionTokenByteLength)
		}
	}
}

func TestRandomSessionTokenGenerator_tokenはlogへ露出しない(t *testing.T) {
	t.Parallel()

	token, err := crypto.NewRandomSessionTokenGenerator().Generate()
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if token.String() == token.Expose() {
		t.Error("String()がtoken本体を返した")
	}
}
