package auth_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

func TestNewRawPassword_境界値(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		input       string
		expectError bool
	}{
		{name: "11文字は拒否する", input: strings.Repeat("a", 11), expectError: true},
		{name: "12文字は受け入れる", input: strings.Repeat("a", 12), expectError: false},
		{name: "128文字は受け入れる", input: strings.Repeat("a", 128), expectError: false},
		{name: "129文字は拒否する", input: strings.Repeat("a", 129), expectError: true},
		{name: "空文字は拒否する", input: "", expectError: true},
		{name: "空白のみは拒否する", input: strings.Repeat(" ", 12), expectError: true},
		{name: "マルチバイトはrune数で数える", input: strings.Repeat("あ", 12), expectError: false},
		{name: "マルチバイト11文字は拒否する", input: strings.Repeat("あ", 11), expectError: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := auth.NewRawPassword(testCase.input)
			if testCase.expectError && err == nil {
				t.Fatalf("expected error for %d chars, got nil", len([]rune(testCase.input)))
			}
			if !testCase.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNewRawPassword_異常系はpasswordのfieldErrorを返す(t *testing.T) {
	t.Parallel()

	_, err := auth.NewRawPassword("short")

	domainError, ok := shared.AsDomainError(err)
	if !ok {
		t.Fatalf("expected DomainError, got %v", err)
	}
	if domainError.Kind != shared.KindInvalidInput {
		t.Errorf("Kind = %v, want %v", domainError.Kind, shared.KindInvalidInput)
	}
	if len(domainError.FieldErrors) != 1 {
		t.Fatalf("FieldErrors = %+v, want 1 element", domainError.FieldErrors)
	}
	if domainError.FieldErrors[0].Field != "password" {
		t.Errorf("Field = %q, want %q", domainError.FieldErrors[0].Field, "password")
	}
	if domainError.FieldErrors[0].Code != "TOO_SHORT" {
		t.Errorf("Code = %q, want %q", domainError.FieldErrors[0].Code, "TOO_SHORT")
	}
}

func TestNewRawPasswordForVerification_最小長を課さない(t *testing.T) {
	t.Parallel()

	// 過去の制約で登録された短いpasswordでもloginできる必要がある。
	password, err := auth.NewRawPasswordForVerification("short")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if password.Expose() != "short" {
		t.Errorf("Expose() = %q, want %q", password.Expose(), "short")
	}

	if _, err := auth.NewRawPasswordForVerification(""); err == nil {
		t.Error("空passwordを受け入れた")
	}
	if _, err := auth.NewRawPasswordForVerification(strings.Repeat("a", 129)); err == nil {
		t.Error("129文字のpasswordを受け入れた")
	}
}

func TestRawPassword_平文をlogへ出力しない(t *testing.T) {
	t.Parallel()

	const plaintext = "super-secret-password"
	password, err := auth.NewRawPassword(plaintext)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if password.String() == plaintext {
		t.Error("String()が平文を返した")
	}
	if formatted := fmt.Sprintf("%v", password); strings.Contains(formatted, plaintext) {
		t.Errorf("fmt出力へ平文が含まれた: %q", formatted)
	}
	if logged := password.LogValue().String(); strings.Contains(logged, plaintext) {
		t.Errorf("LogValue()へ平文が含まれた: %q", logged)
	}
	if password.Expose() != plaintext {
		t.Error("Expose()が平文を返さなかった")
	}
}

func TestNewPasswordHash(t *testing.T) {
	t.Parallel()

	const encoded = "$argon2id$v=19$m=65536,t=3,p=4$c2FsdHNhbHRzYWx0c2FsdA$aGFzaGhhc2hoYXNoaGFzaA"

	hash, err := auth.NewPasswordHash(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash.Encoded() != encoded {
		t.Errorf("Encoded() = %q, want %q", hash.Encoded(), encoded)
	}
	if strings.Contains(hash.String(), "argon2id") {
		t.Errorf("String()がhashを露出した: %q", hash.String())
	}

	if _, err := auth.NewPasswordHash(""); err == nil {
		t.Error("空文字を受け入れた")
	}
	if _, err := auth.NewPasswordHash("$bcrypt$whatever"); err == nil {
		t.Error("argon2id以外のalgorithmを受け入れた")
	}
}
