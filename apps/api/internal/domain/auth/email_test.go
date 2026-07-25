package auth_test

import (
	"strings"
	"testing"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

func TestNewEmail_正常系(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "そのまま受け入れる", input: "user@example.com", expected: "user@example.com"},
		{name: "大文字をlowercase化する", input: "User@Example.COM", expected: "user@example.com"},
		{name: "前後の空白を除去する", input: "  user@example.com  ", expected: "user@example.com"},
		{name: "subdomainを許可する", input: "user@mail.example.co.jp", expected: "user@mail.example.co.jp"},
		{name: "plus addressを許可する", input: "user+tag@example.com", expected: "user+tag@example.com"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			email, err := auth.NewEmail(testCase.input)
			if err != nil {
				t.Fatalf("NewEmail(%q) returned error: %v", testCase.input, err)
			}
			if email.String() != testCase.expected {
				t.Errorf("NewEmail(%q) = %q, want %q", testCase.input, email.String(), testCase.expected)
			}
		})
	}
}

func TestNewEmail_異常系(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input string
	}{
		{name: "空文字", input: ""},
		{name: "空白のみ", input: "   "},
		{name: "アットマークなし", input: "user.example.com"},
		{name: "アットマークが複数", input: "user@@example.com"},
		{name: "local部が空", input: "@example.com"},
		{name: "domain部が空", input: "user@"},
		{name: "domainにドットがない", input: "user@localhost"},
		{name: "domainがドットで終わる", input: "user@example."},
		{name: "連続ドット", input: "user..name@example.com"},
		{name: "空白を含む", input: "user name@example.com"},
		{name: "上限超過", input: strings.Repeat("a", 250) + "@example.com"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := auth.NewEmail(testCase.input)
			if err == nil {
				t.Fatalf("NewEmail(%q) expected error, got nil", testCase.input)
			}

			domainError, ok := shared.AsDomainError(err)
			if !ok {
				t.Fatalf("NewEmail(%q) returned non-domain error: %v", testCase.input, err)
			}
			if domainError.Kind != shared.KindInvalidInput {
				t.Errorf("Kind = %v, want %v", domainError.Kind, shared.KindInvalidInput)
			}
			if len(domainError.FieldErrors) != 1 || domainError.FieldErrors[0].Field != "email" {
				t.Errorf("FieldErrors = %+v, want single error on email", domainError.FieldErrors)
			}
		})
	}
}

func TestNewEmail_境界値(t *testing.T) {
	t.Parallel()

	// 254文字ちょうどは受け入れる。
	longLocalPart := strings.Repeat("a", 254-len("@example.com"))
	if _, err := auth.NewEmail(longLocalPart + "@example.com"); err != nil {
		t.Errorf("254文字のemailを拒否した: %v", err)
	}

	// 255文字は拒否する。
	if _, err := auth.NewEmail(longLocalPart + "a@example.com"); err == nil {
		t.Error("255文字のemailを受け入れた")
	}
}

func TestEmail_IsZero(t *testing.T) {
	t.Parallel()

	var zero auth.Email
	if !zero.IsZero() {
		t.Error("zero valueのIsZero()がfalseを返した")
	}

	email := auth.MustNewEmail("user@example.com")
	if email.IsZero() {
		t.Error("生成済みEmailのIsZero()がtrueを返した")
	}
}
