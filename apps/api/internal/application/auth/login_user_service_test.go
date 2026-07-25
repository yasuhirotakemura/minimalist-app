package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	applicationauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/auth"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/transaction"
)

const (
	testSessionTTL   = 30 * 24 * time.Hour
	firstTokenValue  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	secondTokenValue = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

type loginFixture struct {
	register       *applicationauth.RegisterUserService
	login          *applicationauth.LoginUserService
	logout         *applicationauth.LogoutUserService
	context        *applicationauth.GetAuthenticatedUserContextService
	users          *fakeUserRepository
	sessions       *fakeAuthSessionRepository
	passwordHasher *fakePasswordHasher
	clock          *clock.FixedClock
}

func newLoginFixture() loginFixture {
	users := newFakeUserRepository()
	sessions := newFakeAuthSessionRepository()
	passwordHasher := &fakePasswordHasher{}
	fixedClock := clock.NewFixedClock(testNow)

	dependencies := applicationauth.Dependencies{
		Users:                 users,
		Sessions:              sessions,
		PasswordHasher:        passwordHasher,
		SessionTokenGenerator: newFixedSessionTokenGenerator(firstTokenValue, secondTokenValue),
		SessionTTL:            testSessionTTL,
	}

	publicIDGenerator := &sequentialPublicIDGenerator{}
	transactionManager := transaction.NewPassthroughManager()

	return loginFixture{
		register: applicationauth.NewRegisterUserService(
			dependencies, publicIDGenerator, fixedClock, transactionManager),
		login: applicationauth.NewLoginUserService(
			dependencies, publicIDGenerator, fixedClock, transactionManager),
		logout:         applicationauth.NewLogoutUserService(dependencies, fixedClock),
		context:        applicationauth.NewGetAuthenticatedUserContextService(dependencies, fixedClock),
		users:          users,
		sessions:       sessions,
		passwordHasher: passwordHasher,
		clock:          fixedClock,
	}
}

// registerTestUser はloginできるユーザーを1件用意する。
func (f loginFixture) registerTestUser(t *testing.T) {
	t.Helper()

	if _, err := f.register.Execute(context.Background(), applicationauth.RegisterUserParams{
		Email:       "user@example.com",
		Password:    "correct-horse-battery",
		DisplayName: "山田太郎",
	}); err != nil {
		t.Fatalf("テストユーザーの登録に失敗した: %v", err)
	}

	// fakeAuthSessionRepositoryはuser解決のためのmapを別に持つ。
	user, err := f.users.FindActiveByEmail(
		context.Background(), domainauth.MustNewEmail("user@example.com"))
	if err != nil {
		t.Fatalf("テストユーザーを取得できない: %v", err)
	}
	f.sessions.users[user.ID()] = user
}

func TestLoginUserService_正常系(t *testing.T) {
	t.Parallel()

	fixture := newLoginFixture()
	fixture.registerTestUser(t)

	result, err := fixture.login.Execute(context.Background(), applicationauth.LoginUserParams{
		Email:     "USER@example.com",
		Password:  "correct-horse-battery",
		UserAgent: "Mozilla/5.0",
		IPAddress: "192.0.2.10",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.User.Email != "user@example.com" {
		t.Errorf("Email = %q", result.User.Email)
	}
	if result.SessionToken.Expose() != firstTokenValue {
		t.Errorf("SessionToken = %q, want %q", result.SessionToken.Expose(), firstTokenValue)
	}
	if want := testNow.Add(testSessionTTL); !result.SessionExpiresAt.Equal(want) {
		t.Errorf("SessionExpiresAt = %v, want %v", result.SessionExpiresAt, want)
	}
	if len(fixture.sessions.sessions) != 1 {
		t.Errorf("保存されたsession数 = %d, want 1", len(fixture.sessions.sessions))
	}
}

func TestLoginUserService_認証失敗はemailの存在有無を区別しない(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		email    string
		password string
	}{
		{
			name:     "存在しないemail",
			email:    "unknown@example.com",
			password: "correct-horse-battery",
		},
		{
			name:     "passwordが誤り",
			email:    "user@example.com",
			password: "wrong-password-value",
		},
		{
			name:     "emailの形式が不正",
			email:    "not-an-email",
			password: "correct-horse-battery",
		},
		{
			name:     "passwordが空",
			email:    "user@example.com",
			password: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fixture := newLoginFixture()
			fixture.registerTestUser(t)

			_, err := fixture.login.Execute(context.Background(), applicationauth.LoginUserParams{
				Email:    testCase.email,
				Password: testCase.password,
			})
			if !errors.Is(err, domainauth.ErrInvalidCredentials) {
				t.Fatalf("err = %v, want ErrInvalidCredentials", err)
			}

			domainError, ok := shared.AsDomainError(err)
			if !ok {
				t.Fatalf("expected DomainError, got %v", err)
			}
			if domainError.Kind != shared.KindUnauthenticated {
				t.Errorf("Kind = %v, want %v", domainError.Kind, shared.KindUnauthenticated)
			}
			// error messageからもemailの存在有無を推測できないこと。
			if strings.Contains(domainError.Message, "登録されていません") {
				t.Errorf("Message がemailの存在有無を示唆している: %q", domainError.Message)
			}
		})
	}
}

func TestLoginUserService_存在しないemailでもhash計算を行う(t *testing.T) {
	t.Parallel()

	fixture := newLoginFixture()
	fixture.registerTestUser(t)

	before := fixture.passwordHasher.HashCount()

	_, err := fixture.login.Execute(context.Background(), applicationauth.LoginUserParams{
		Email:    "unknown@example.com",
		Password: "correct-horse-battery",
	})
	if !errors.Is(err, domainauth.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}

	// 応答時間の差からemailの登録有無を推測されないようにする。
	if fixture.passwordHasher.HashCount() == before {
		t.Error("存在しないemailでhash計算が行われなかった")
	}
}

func TestLoginUserService_認証失敗時はsessionを作らない(t *testing.T) {
	t.Parallel()

	fixture := newLoginFixture()
	fixture.registerTestUser(t)

	_, err := fixture.login.Execute(context.Background(), applicationauth.LoginUserParams{
		Email:    "user@example.com",
		Password: "wrong-password-value",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(fixture.sessions.sessions) != 0 {
		t.Errorf("認証失敗でsessionが作成された: %d件", len(fixture.sessions.sessions))
	}
}

func TestLoginUserService_session作成失敗を伝播する(t *testing.T) {
	t.Parallel()

	fixture := newLoginFixture()
	fixture.registerTestUser(t)
	fixture.sessions.failOnCreate = true

	_, err := fixture.login.Execute(context.Background(), applicationauth.LoginUserParams{
		Email:    "user@example.com",
		Password: "correct-horse-battery",
	})
	if !errors.Is(err, errRepository) {
		t.Fatalf("err = %v, want errRepository", err)
	}
}

func TestGetAuthenticatedUserContextService_正常系(t *testing.T) {
	t.Parallel()

	fixture := newLoginFixture()
	fixture.registerTestUser(t)

	login, err := fixture.login.Execute(context.Background(), applicationauth.LoginUserParams{
		Email:    "user@example.com",
		Password: "correct-horse-battery",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	result, err := fixture.context.Execute(
		context.Background(),
		applicationauth.GetAuthenticatedUserContextParams{
			SessionToken: login.SessionToken.Expose(),
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.User.Email != "user@example.com" {
		t.Errorf("Email = %q", result.User.Email)
	}
	if result.UserID.IsZero() {
		t.Error("UserIDが未設定")
	}
}

func TestGetAuthenticatedUserContextService_未認証(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		token string
	}{
		{name: "tokenが空", token: ""},
		{name: "token形式が不正", token: "short"},
		{name: "存在しないtoken", token: secondTokenValue},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fixture := newLoginFixture()
			fixture.registerTestUser(t)

			_, err := fixture.context.Execute(
				context.Background(),
				applicationauth.GetAuthenticatedUserContextParams{SessionToken: testCase.token},
			)
			if !errors.Is(err, domainauth.ErrUnauthenticated) {
				t.Fatalf("err = %v, want ErrUnauthenticated", err)
			}
		})
	}
}

func TestGetAuthenticatedUserContextService_期限切れsessionを拒否する(t *testing.T) {
	t.Parallel()

	fixture := newLoginFixture()
	fixture.registerTestUser(t)

	login, err := fixture.login.Execute(context.Background(), applicationauth.LoginUserParams{
		Email:    "user@example.com",
		Password: "correct-horse-battery",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	fixture.clock.Advance(testSessionTTL)

	_, err = fixture.context.Execute(
		context.Background(),
		applicationauth.GetAuthenticatedUserContextParams{
			SessionToken: login.SessionToken.Expose(),
		},
	)
	if !errors.Is(err, domainauth.ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
}

func TestGetAuthenticatedUserContextService_lastUsedAtの更新間隔(t *testing.T) {
	t.Parallel()

	fixture := newLoginFixture()
	fixture.registerTestUser(t)

	login, err := fixture.login.Execute(context.Background(), applicationauth.LoginUserParams{
		Email:    "user@example.com",
		Password: "correct-horse-battery",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	params := applicationauth.GetAuthenticatedUserContextParams{
		SessionToken: login.SessionToken.Expose(),
	}

	// 更新間隔未満では書き込まない。
	fixture.clock.Advance(time.Minute)
	if _, err := fixture.context.Execute(context.Background(), params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fixture.sessions.refreshCount != 0 {
		t.Errorf("refreshCount = %d, want 0", fixture.sessions.refreshCount)
	}

	// 更新間隔を超えたら書き込む。
	fixture.clock.Advance(domainauth.SessionTouchInterval)
	if _, err := fixture.context.Execute(context.Background(), params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fixture.sessions.refreshCount != 1 {
		t.Errorf("refreshCount = %d, want 1", fixture.sessions.refreshCount)
	}
}

func TestGetAuthenticatedUserContextService_lastUsedAt更新失敗でも認証は成功する(t *testing.T) {
	t.Parallel()

	fixture := newLoginFixture()
	fixture.registerTestUser(t)

	login, err := fixture.login.Execute(context.Background(), applicationauth.LoginUserParams{
		Email:    "user@example.com",
		Password: "correct-horse-battery",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	fixture.sessions.failOnRefresh = true
	fixture.clock.Advance(domainauth.SessionTouchInterval)

	if _, err := fixture.context.Execute(
		context.Background(),
		applicationauth.GetAuthenticatedUserContextParams{
			SessionToken: login.SessionToken.Expose(),
		},
	); err != nil {
		t.Fatalf("last_used_atの更新失敗で認証が失敗した: %v", err)
	}
}

func TestLogoutUserService_sessionを失効させる(t *testing.T) {
	t.Parallel()

	fixture := newLoginFixture()
	fixture.registerTestUser(t)

	login, err := fixture.login.Execute(context.Background(), applicationauth.LoginUserParams{
		Email:    "user@example.com",
		Password: "correct-horse-battery",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	if err := fixture.logout.Execute(context.Background(), applicationauth.LogoutUserParams{
		SessionToken: login.SessionToken.Expose(),
	}); err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	_, err = fixture.context.Execute(
		context.Background(),
		applicationauth.GetAuthenticatedUserContextParams{
			SessionToken: login.SessionToken.Expose(),
		},
	)
	if !errors.Is(err, domainauth.ErrUnauthenticated) {
		t.Fatalf("logout後のsessionが有効: %v", err)
	}
}

func TestLogoutUserService_冪等である(t *testing.T) {
	t.Parallel()

	fixture := newLoginFixture()

	testCases := []struct {
		name  string
		token string
	}{
		{name: "tokenが空", token: ""},
		{name: "token形式が不正", token: "short"},
		{name: "存在しないtoken", token: firstTokenValue},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := fixture.logout.Execute(
				context.Background(),
				applicationauth.LogoutUserParams{SessionToken: testCase.token},
			); err != nil {
				t.Errorf("logoutがerrorを返した: %v", err)
			}
		})
	}
}

func TestLogoutUserService_他のsessionへ影響しない(t *testing.T) {
	t.Parallel()

	fixture := newLoginFixture()
	fixture.registerTestUser(t)

	first, err := fixture.login.Execute(context.Background(), applicationauth.LoginUserParams{
		Email:    "user@example.com",
		Password: "correct-horse-battery",
	})
	if err != nil {
		t.Fatalf("1回目のloginに失敗した: %v", err)
	}
	second, err := fixture.login.Execute(context.Background(), applicationauth.LoginUserParams{
		Email:    "user@example.com",
		Password: "correct-horse-battery",
	})
	if err != nil {
		t.Fatalf("2回目のloginに失敗した: %v", err)
	}

	if err := fixture.logout.Execute(context.Background(), applicationauth.LogoutUserParams{
		SessionToken: first.SessionToken.Expose(),
	}); err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	// 失効させたのは1つ目のsessionのみ。
	if _, err := fixture.context.Execute(
		context.Background(),
		applicationauth.GetAuthenticatedUserContextParams{
			SessionToken: second.SessionToken.Expose(),
		},
	); err != nil {
		t.Errorf("別端末のsessionまで失効した: %v", err)
	}
}
