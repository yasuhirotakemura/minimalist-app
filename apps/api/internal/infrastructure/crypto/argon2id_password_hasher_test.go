package crypto_test

import (
	"strings"
	"testing"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/crypto"
)

// testParameters はtestを高速化するため計算量を下げたparameter。
// 本番既定値の検証は TestDefaultArgon2idParameters で行う。
func testParameters() crypto.Argon2idParameters {
	return crypto.Argon2idParameters{
		MemoryKiB:   8 * 1024,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
}

func newTestHasher(t *testing.T, pepper string) *crypto.Argon2idPasswordHasher {
	t.Helper()
	hasher, err := crypto.NewArgon2idPasswordHasher(pepper, testParameters())
	if err != nil {
		t.Fatalf("NewArgon2idPasswordHasher returned error: %v", err)
	}
	return hasher
}

func mustRawPassword(t *testing.T, raw string) auth.RawPassword {
	t.Helper()
	password, err := auth.NewRawPassword(raw)
	if err != nil {
		t.Fatalf("NewRawPassword(%q) returned error: %v", raw, err)
	}
	return password
}

func TestArgon2idPasswordHasher_正しいpasswordを検証できる(t *testing.T) {
	t.Parallel()

	hasher := newTestHasher(t, "local_password_pepper")
	password := mustRawPassword(t, "correct-horse-battery")

	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}

	matched, err := hasher.Verify(password, hash)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if !matched {
		t.Error("正しいpasswordの検証がfalseを返した")
	}
}

func TestArgon2idPasswordHasher_誤ったpasswordを拒否する(t *testing.T) {
	t.Parallel()

	hasher := newTestHasher(t, "local_password_pepper")

	hash, err := hasher.Hash(mustRawPassword(t, "correct-horse-battery"))
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}

	matched, err := hasher.Verify(mustRawPassword(t, "incorrect-horse-batt"), hash)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if matched {
		t.Error("誤ったpasswordの検証がtrueを返した")
	}
}

func TestArgon2idPasswordHasher_saltにより毎回異なるhashを生成する(t *testing.T) {
	t.Parallel()

	hasher := newTestHasher(t, "local_password_pepper")
	password := mustRawPassword(t, "correct-horse-battery")

	first, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}
	second, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}

	if first.Encoded() == second.Encoded() {
		t.Error("同一passwordから同一hashが生成された (saltが機能していない)")
	}

	// どちらのhashでも検証できる。
	for index, hash := range []auth.PasswordHash{first, second} {
		matched, err := hasher.Verify(password, hash)
		if err != nil {
			t.Fatalf("Verify(%d) returned error: %v", index, err)
		}
		if !matched {
			t.Errorf("Verify(%d) = false, want true", index)
		}
	}
}

func TestArgon2idPasswordHasher_pepperが異なると検証に失敗する(t *testing.T) {
	t.Parallel()

	password := mustRawPassword(t, "correct-horse-battery")

	hash, err := newTestHasher(t, "pepper-one").Hash(password)
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}

	matched, err := newTestHasher(t, "pepper-two").Verify(password, hash)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if matched {
		t.Error("異なるpepperで検証が成功した")
	}
}

func TestArgon2idPasswordHasher_PHC形式で保存する(t *testing.T) {
	t.Parallel()

	hasher := newTestHasher(t, "local_password_pepper")

	hash, err := hasher.Hash(mustRawPassword(t, "correct-horse-battery"))
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}

	encoded := hash.Encoded()
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=8192,t=1,p=1$") {
		t.Errorf("PHC prefixが不正: %q", encoded)
	}
	if segments := strings.Split(encoded, "$"); len(segments) != 6 {
		t.Errorf("PHC segment数 = %d, want 6 (%q)", len(segments), encoded)
	}
}

func TestArgon2idPasswordHasher_parameter変更後も既存hashを検証できる(t *testing.T) {
	t.Parallel()

	password := mustRawPassword(t, "correct-horse-battery")

	// 弱いparameterで生成する。
	weak := newTestHasher(t, "local_password_pepper")
	hash, err := weak.Hash(password)
	if err != nil {
		t.Fatalf("Hash returned error: %v", err)
	}

	// parameterを強化したhasherでも、hashへ埋め込まれたparameterで検証する。
	strongParameters := testParameters()
	strongParameters.Iterations = 3
	strongParameters.MemoryKiB = 16 * 1024
	strong, err := crypto.NewArgon2idPasswordHasher("local_password_pepper", strongParameters)
	if err != nil {
		t.Fatalf("NewArgon2idPasswordHasher returned error: %v", err)
	}

	matched, err := strong.Verify(password, hash)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if !matched {
		t.Error("parameter変更後に既存hashの検証が失敗した")
	}
}

func TestArgon2idPasswordHasher_壊れたhashはerrorを返す(t *testing.T) {
	t.Parallel()

	hasher := newTestHasher(t, "local_password_pepper")
	password := mustRawPassword(t, "correct-horse-battery")

	testCases := []struct {
		name    string
		encoded string
	}{
		{name: "segment不足", encoded: "$argon2id$v=19$m=8192,t=1,p=1$c2FsdA"},
		{name: "未対応version", encoded: "$argon2id$v=16$m=8192,t=1,p=1$c2FsdA$aGFzaA"},
		{name: "parameter不正", encoded: "$argon2id$v=19$m=abc,t=1,p=1$c2FsdA$aGFzaA"},
		{name: "salt復号不可", encoded: "$argon2id$v=19$m=8192,t=1,p=1$!!!!$aGFzaA"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			hash, err := auth.NewPasswordHash(testCase.encoded)
			if err != nil {
				t.Fatalf("NewPasswordHash returned error: %v", err)
			}
			if _, err := hasher.Verify(password, hash); err == nil {
				t.Error("壊れたhashでerrorが返らなかった")
			}
		})
	}
}

func TestNewArgon2idPasswordHasher_異常系(t *testing.T) {
	t.Parallel()

	if _, err := crypto.NewArgon2idPasswordHasher("", testParameters()); err == nil {
		t.Error("空のpepperを受け入れた")
	}

	weak := testParameters()
	weak.MemoryKiB = 1024
	if _, err := crypto.NewArgon2idPasswordHasher("pepper", weak); err == nil {
		t.Error("memory costが小さすぎるparameterを受け入れた")
	}

	shortSalt := testParameters()
	shortSalt.SaltLength = 8
	if _, err := crypto.NewArgon2idPasswordHasher("pepper", shortSalt); err == nil {
		t.Error("saltが短すぎるparameterを受け入れた")
	}
}

func TestDefaultArgon2idParameters(t *testing.T) {
	t.Parallel()

	parameters := crypto.DefaultArgon2idParameters()

	if parameters.MemoryKiB != 64*1024 {
		t.Errorf("MemoryKiB = %d, want %d", parameters.MemoryKiB, 64*1024)
	}
	if parameters.Iterations != 3 {
		t.Errorf("Iterations = %d, want 3", parameters.Iterations)
	}
	if parameters.Parallelism != 4 {
		t.Errorf("Parallelism = %d, want 4", parameters.Parallelism)
	}
	if parameters.SaltLength != 16 {
		t.Errorf("SaltLength = %d, want 16", parameters.SaltLength)
	}
	if parameters.KeyLength != 32 {
		t.Errorf("KeyLength = %d, want 32", parameters.KeyLength)
	}
}
