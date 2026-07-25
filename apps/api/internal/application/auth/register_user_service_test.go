package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	applicationauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/auth"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/transaction"
)

var testNow = time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)

type registerFixture struct {
	service        *applicationauth.RegisterUserService
	users          *fakeUserRepository
	passwordHasher *fakePasswordHasher
}

func newRegisterFixture() registerFixture {
	users := newFakeUserRepository()
	passwordHasher := &fakePasswordHasher{}

	dependencies := applicationauth.Dependencies{
		Users:                 users,
		Sessions:              newFakeAuthSessionRepository(),
		PasswordHasher:        passwordHasher,
		SessionTokenGenerator: newFixedSessionTokenGenerator(),
		SessionTTL:            30 * 24 * time.Hour,
	}

	return registerFixture{
		service: applicationauth.NewRegisterUserService(
			dependencies,
			&sequentialPublicIDGenerator{},
			clock.NewFixedClock(testNow),
			transaction.NewPassthroughManager(),
		),
		users:          users,
		passwordHasher: passwordHasher,
	}
}

func validRegisterParams() applicationauth.RegisterUserParams {
	return applicationauth.RegisterUserParams{
		Email:       "User@Example.com",
		Password:    "correct-horse-battery",
		DisplayName: "山田太郎",
		Timezone:    "Asia/Tokyo",
		Locale:      "ja-JP",
	}
}

func TestRegisterUserService_正常系(t *testing.T) {
	t.Parallel()

	fixture := newRegisterFixture()

	result, err := fixture.service.Execute(context.Background(), validRegisterParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.User.Email != "user@example.com" {
		t.Errorf("Email = %q, want lowercase正規化された値", result.User.Email)
	}
	if result.User.DisplayName != "山田太郎" {
		t.Errorf("DisplayName = %q", result.User.DisplayName)
	}
	if result.User.PublicID.String() == "" {
		t.Error("PublicIDが空")
	}
	if !result.User.CreatedAt.Equal(testNow) {
		t.Errorf("CreatedAt = %v, want %v", result.User.CreatedAt, testNow)
	}

	// password hashが保存されていることを確認する。
	stored, err := fixture.users.FindActiveByEmail(
		context.Background(), domainauth.MustNewEmail("user@example.com"))
	if err != nil {
		t.Fatalf("登録したユーザーを取得できない: %v", err)
	}
	if stored.ID().IsZero() {
		t.Error("内部IDが払い出されていない")
	}
	if _, err := fixture.users.FindPasswordHashByUserID(context.Background(), stored.ID()); err != nil {
		t.Errorf("password認証情報が保存されていない: %v", err)
	}
}

func TestRegisterUserService_timezone未指定時は既定値を使う(t *testing.T) {
	t.Parallel()

	fixture := newRegisterFixture()

	params := validRegisterParams()
	params.Timezone = ""
	params.Locale = ""

	result, err := fixture.service.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.User.Timezone != domainauth.DefaultTimezone {
		t.Errorf("Timezone = %q, want %q", result.User.Timezone, domainauth.DefaultTimezone)
	}
	if result.User.Locale != domainauth.DefaultLocale {
		t.Errorf("Locale = %q, want %q", result.User.Locale, domainauth.DefaultLocale)
	}
}

func TestRegisterUserService_email重複を拒否する(t *testing.T) {
	t.Parallel()

	fixture := newRegisterFixture()

	if _, err := fixture.service.Execute(context.Background(), validRegisterParams()); err != nil {
		t.Fatalf("1件目の登録に失敗した: %v", err)
	}

	// 大文字小文字が異なっていても重複として扱う。
	params := validRegisterParams()
	params.Email = "USER@EXAMPLE.COM"

	_, err := fixture.service.Execute(context.Background(), params)
	if !errors.Is(err, domainauth.ErrEmailAlreadyRegistered) {
		t.Fatalf("err = %v, want ErrEmailAlreadyRegistered", err)
	}

	domainError, ok := shared.AsDomainError(err)
	if !ok {
		t.Fatalf("expected DomainError, got %v", err)
	}
	if domainError.Kind != shared.KindConflict {
		t.Errorf("Kind = %v, want %v", domainError.Kind, shared.KindConflict)
	}
}

func TestRegisterUserService_入力検証(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		mutate        func(*applicationauth.RegisterUserParams)
		expectedField string
	}{
		{
			name:          "emailが不正",
			mutate:        func(p *applicationauth.RegisterUserParams) { p.Email = "invalid" },
			expectedField: "email",
		},
		{
			name:          "passwordが短い",
			mutate:        func(p *applicationauth.RegisterUserParams) { p.Password = "short" },
			expectedField: "password",
		},
		{
			name:          "表示名が空",
			mutate:        func(p *applicationauth.RegisterUserParams) { p.DisplayName = "  " },
			expectedField: "displayName",
		},
		{
			name:          "timezoneが不正",
			mutate:        func(p *applicationauth.RegisterUserParams) { p.Timezone = "Invalid/Zone" },
			expectedField: "timezone",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fixture := newRegisterFixture()
			params := validRegisterParams()
			testCase.mutate(&params)

			_, err := fixture.service.Execute(context.Background(), params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			domainError, ok := shared.AsDomainError(err)
			if !ok {
				t.Fatalf("expected DomainError, got %v", err)
			}
			if domainError.Kind != shared.KindInvalidInput {
				t.Errorf("Kind = %v, want %v", domainError.Kind, shared.KindInvalidInput)
			}
			if len(domainError.FieldErrors) != 1 ||
				domainError.FieldErrors[0].Field != testCase.expectedField {
				t.Errorf("FieldErrors = %+v, want field %q",
					domainError.FieldErrors, testCase.expectedField)
			}
		})
	}
}

func TestRegisterUserService_検証失敗時はhash計算を行わない(t *testing.T) {
	t.Parallel()

	fixture := newRegisterFixture()

	params := validRegisterParams()
	params.Email = "invalid"

	if _, err := fixture.service.Execute(context.Background(), params); err == nil {
		t.Fatal("expected error, got nil")
	}
	if fixture.passwordHasher.HashCount() != 0 {
		t.Errorf("HashCount = %d, want 0", fixture.passwordHasher.HashCount())
	}
}

func TestRegisterUserService_repository失敗を伝播する(t *testing.T) {
	t.Parallel()

	fixture := newRegisterFixture()
	fixture.users.failOnCreate = true

	_, err := fixture.service.Execute(context.Background(), validRegisterParams())
	if !errors.Is(err, errRepository) {
		t.Fatalf("err = %v, want errRepository", err)
	}
}
