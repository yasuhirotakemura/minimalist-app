//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	repositories "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/repositories/postgresql"
)

func newAuthSessionRepository() *repositories.PostgresqlAuthSessionRepository {
	return repositories.NewPostgresqlAuthSessionRepository(testPool)
}

// createUserForSession はsession testで使うユーザーを作成する。
func createUserForSession(t *testing.T, email string) domainauth.User {
	t.Helper()

	created, err := newUserRepository().Create(
		context.Background(), newTestUser(t, email), mustPasswordHash(t, "aGFzaA"))
	if err != nil {
		t.Fatalf("テストユーザーの作成に失敗した: %v", err)
	}
	return created
}

func issueTestSession(
	t *testing.T,
	user domainauth.User,
	tokenValue string,
	issuedAt time.Time,
	ttl time.Duration,
) domainauth.AuthSession {
	t.Helper()

	token, err := domainauth.NewSessionToken(tokenValue)
	if err != nil {
		t.Fatalf("NewSessionToken returned error: %v", err)
	}

	session, err := domainauth.IssueAuthSession(domainauth.IssueAuthSessionParams{
		PublicID:  mustPublicID(t),
		UserID:    user.ID(),
		Token:     token,
		IssuedAt:  issuedAt,
		TTL:       ttl,
		UserAgent: "Mozilla/5.0",
		IPAddress: "192.0.2.10",
	})
	if err != nil {
		t.Fatalf("IssueAuthSession returned error: %v", err)
	}
	return session
}

const (
	tokenA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tokenB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestPostgresqlAuthSessionRepository_作成と取得(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newAuthSessionRepository()

	user := createUserForSession(t, "user@example.com")
	issuedAt := time.Now().UTC().Truncate(time.Microsecond)
	session := issueTestSession(t, user, tokenA, issuedAt, time.Hour)

	created, err := repository.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID() == 0 {
		t.Fatal("内部IDが払い出されていない")
	}
	// user_agent と ip_address は監査補助であり、認証queryでは取得しない。
	// 保存されていることはCreateの戻り値で確認する。
	if created.UserAgent() != "Mozilla/5.0" {
		t.Errorf("UserAgent = %q, want %q", created.UserAgent(), "Mozilla/5.0")
	}
	if created.IPAddress() != "192.0.2.10" {
		t.Errorf("IPAddress = %q, want %q", created.IPAddress(), "192.0.2.10")
	}

	foundSession, foundUser, err := repository.FindLiveWithUserByTokenHash(
		ctx, session.TokenHash(), issuedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("FindLiveWithUserByTokenHash returned error: %v", err)
	}
	if foundSession.ID() != created.ID() {
		t.Errorf("session ID = %d, want %d", foundSession.ID(), created.ID())
	}
	if foundUser.ID() != user.ID() {
		t.Errorf("user ID = %d, want %d", foundUser.ID(), user.ID())
	}
	if foundUser.Email().String() != "user@example.com" {
		t.Errorf("user email = %q", foundUser.Email().String())
	}
}

func TestPostgresqlAuthSessionRepository_期限切れsessionは取得できない(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newAuthSessionRepository()

	user := createUserForSession(t, "user@example.com")
	issuedAt := time.Now().UTC().Truncate(time.Microsecond)
	session := issueTestSession(t, user, tokenA, issuedAt, time.Hour)

	if _, err := repository.Create(ctx, session); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	// 期限ちょうどでも取得できない (query条件は expires_at > evaluatedAt)。
	_, _, err := repository.FindLiveWithUserByTokenHash(
		ctx, session.TokenHash(), issuedAt.Add(time.Hour))
	if !errors.Is(err, domainauth.ErrAuthSessionNotFound) {
		t.Fatalf("err = %v, want ErrAuthSessionNotFound", err)
	}
}

func TestPostgresqlAuthSessionRepository_失効させたsessionは取得できない(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newAuthSessionRepository()

	user := createUserForSession(t, "user@example.com")
	issuedAt := time.Now().UTC().Truncate(time.Microsecond)
	session := issueTestSession(t, user, tokenA, issuedAt, time.Hour)

	if _, err := repository.Create(ctx, session); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if err := repository.RevokeByTokenHash(
		ctx, session.TokenHash(), issuedAt.Add(time.Minute),
	); err != nil {
		t.Fatalf("RevokeByTokenHash returned error: %v", err)
	}

	_, _, err := repository.FindLiveWithUserByTokenHash(
		ctx, session.TokenHash(), issuedAt.Add(2*time.Minute))
	if !errors.Is(err, domainauth.ErrAuthSessionNotFound) {
		t.Fatalf("err = %v, want ErrAuthSessionNotFound", err)
	}

	// 二重失効でもerrorとしない (冪等)。
	if err := repository.RevokeByTokenHash(
		ctx, session.TokenHash(), issuedAt.Add(3*time.Minute),
	); err != nil {
		t.Errorf("二重失効でerrorが返った: %v", err)
	}
}

func TestPostgresqlAuthSessionRepository_退会済みユーザーのsessionは取得できない(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newAuthSessionRepository()

	user := createUserForSession(t, "user@example.com")
	issuedAt := time.Now().UTC().Truncate(time.Microsecond)
	session := issueTestSession(t, user, tokenA, issuedAt, time.Hour)

	if _, err := repository.Create(ctx, session); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, err := testPool.Exec(
		ctx, `UPDATE identity.users SET deleted_at = now() WHERE id = $1`, user.ID().Int64(),
	); err != nil {
		t.Fatalf("soft deleteに失敗した: %v", err)
	}

	_, _, err := repository.FindLiveWithUserByTokenHash(
		ctx, session.TokenHash(), issuedAt.Add(time.Minute))
	if !errors.Is(err, domainauth.ErrAuthSessionNotFound) {
		t.Fatalf("err = %v, want ErrAuthSessionNotFound", err)
	}
}

func TestPostgresqlAuthSessionRepository_他ユーザーのsessionは見えない(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newAuthSessionRepository()

	owner := createUserForSession(t, "owner@example.com")
	other := createUserForSession(t, "other@example.com")

	issuedAt := time.Now().UTC().Truncate(time.Microsecond)
	ownerSession := issueTestSession(t, owner, tokenA, issuedAt, time.Hour)
	otherSession := issueTestSession(t, other, tokenB, issuedAt, time.Hour)

	if _, err := repository.Create(ctx, ownerSession); err != nil {
		t.Fatalf("owner session Create returned error: %v", err)
	}
	if _, err := repository.Create(ctx, otherSession); err != nil {
		t.Fatalf("other session Create returned error: %v", err)
	}

	// owner のtokenで引くと owner のuserだけが返る。
	_, foundUser, err := repository.FindLiveWithUserByTokenHash(
		ctx, ownerSession.TokenHash(), issuedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("FindLiveWithUserByTokenHash returned error: %v", err)
	}
	if foundUser.ID() != owner.ID() {
		t.Fatalf("user ID = %d, want %d (他ユーザーのsessionが解決された)",
			foundUser.ID(), owner.ID())
	}

	// owner のsessionを失効させても other のsessionは有効なまま。
	if err := repository.RevokeByTokenHash(
		ctx, ownerSession.TokenHash(), issuedAt.Add(time.Minute),
	); err != nil {
		t.Fatalf("RevokeByTokenHash returned error: %v", err)
	}
	if _, _, err := repository.FindLiveWithUserByTokenHash(
		ctx, otherSession.TokenHash(), issuedAt.Add(2*time.Minute),
	); err != nil {
		t.Errorf("他ユーザーのsessionまで失効した: %v", err)
	}
}

func TestPostgresqlAuthSessionRepository_RevokeAllByUserID(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newAuthSessionRepository()

	owner := createUserForSession(t, "owner@example.com")
	other := createUserForSession(t, "other@example.com")

	issuedAt := time.Now().UTC().Truncate(time.Microsecond)
	ownerSession := issueTestSession(t, owner, tokenA, issuedAt, time.Hour)
	otherSession := issueTestSession(t, other, tokenB, issuedAt, time.Hour)

	if _, err := repository.Create(ctx, ownerSession); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := repository.Create(ctx, otherSession); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	revoked, err := repository.RevokeAllByUserID(ctx, owner.ID(), issuedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("RevokeAllByUserID returned error: %v", err)
	}
	if revoked != 1 {
		t.Errorf("revoked = %d, want 1", revoked)
	}

	if _, _, err := repository.FindLiveWithUserByTokenHash(
		ctx, otherSession.TokenHash(), issuedAt.Add(2*time.Minute),
	); err != nil {
		t.Errorf("他ユーザーのsessionまで失効した: %v", err)
	}
}

func TestPostgresqlAuthSessionRepository_RefreshLastUsedAt(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newAuthSessionRepository()

	user := createUserForSession(t, "user@example.com")
	issuedAt := time.Now().UTC().Truncate(time.Microsecond)
	session := issueTestSession(t, user, tokenA, issuedAt, time.Hour)

	if _, err := repository.Create(ctx, session); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	usedAt := issuedAt.Add(10 * time.Minute)
	if err := repository.RefreshLastUsedAt(ctx, session.TokenHash(), usedAt); err != nil {
		t.Fatalf("RefreshLastUsedAt returned error: %v", err)
	}

	refreshed, _, err := repository.FindLiveWithUserByTokenHash(
		ctx, session.TokenHash(), usedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("FindLiveWithUserByTokenHash returned error: %v", err)
	}
	if !refreshed.LastUsedAt().Equal(usedAt) {
		t.Errorf("LastUsedAt = %v, want %v", refreshed.LastUsedAt(), usedAt)
	}
}

func TestPostgresqlAuthSessionRepository_DeleteExpired(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newAuthSessionRepository()

	user := createUserForSession(t, "user@example.com")
	issuedAt := time.Now().UTC().Truncate(time.Microsecond)

	expired := issueTestSession(t, user, tokenA, issuedAt, time.Hour)
	live := issueTestSession(t, user, tokenB, issuedAt, 24*time.Hour)

	if _, err := repository.Create(ctx, expired); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := repository.Create(ctx, live); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	deleted, err := repository.DeleteExpired(ctx, issuedAt.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("DeleteExpired returned error: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	if _, _, err := repository.FindLiveWithUserByTokenHash(
		ctx, live.TokenHash(), issuedAt.Add(2*time.Hour),
	); err != nil {
		t.Errorf("有効なsessionまで削除された: %v", err)
	}
}

func TestPostgresqlAuthSessionRepository_tokenHashのunique制約(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newAuthSessionRepository()

	user := createUserForSession(t, "user@example.com")
	issuedAt := time.Now().UTC().Truncate(time.Microsecond)

	session := issueTestSession(t, user, tokenA, issuedAt, time.Hour)
	if _, err := repository.Create(ctx, session); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	duplicate := issueTestSession(t, user, tokenA, issuedAt, time.Hour)
	if _, err := repository.Create(ctx, duplicate); err == nil {
		t.Error("同一token hashのsessionが2件作成できた")
	}
}

func TestPostgresqlAuthSessionRepository_ユーザー削除でsessionもCASCADE削除される(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	repository := newAuthSessionRepository()

	user := createUserForSession(t, "user@example.com")
	issuedAt := time.Now().UTC().Truncate(time.Microsecond)
	session := issueTestSession(t, user, tokenA, issuedAt, time.Hour)

	if _, err := repository.Create(ctx, session); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, err := testPool.Exec(
		ctx, `DELETE FROM identity.users WHERE id = $1`, user.ID().Int64(),
	); err != nil {
		t.Fatalf("ユーザーの物理削除に失敗した: %v", err)
	}

	var remaining int
	if err := testPool.QueryRow(
		ctx, `SELECT count(*) FROM identity.auth_sessions WHERE user_id = $1`, user.ID().Int64(),
	).Scan(&remaining); err != nil {
		t.Fatalf("件数取得に失敗した: %v", err)
	}
	if remaining != 0 {
		t.Errorf("CASCADE削除されていない: %d件", remaining)
	}
}

func TestAuthSessions_期限がissuedAt以前のsessionはCHECK制約で拒否される(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	user := createUserForSession(t, "user@example.com")
	now := time.Now().UTC()

	_, err := testPool.Exec(
		ctx,
		`INSERT INTO identity.auth_sessions (public_id, user_id, token_hash, issued_at, expires_at)
		 VALUES ($1, $2, $3, $4, $4)`,
		mustPublicID(t), user.ID().Int64(), make([]byte, 32), now,
	)
	if err == nil {
		t.Error("expires_at <= issued_at のsessionが登録できた")
	}
}

func TestAuthSessions_tokenHash長のCHECK制約(t *testing.T) {
	truncateAll(t)

	ctx := context.Background()
	user := createUserForSession(t, "user@example.com")
	now := time.Now().UTC()

	_, err := testPool.Exec(
		ctx,
		`INSERT INTO identity.auth_sessions (public_id, user_id, token_hash, issued_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		mustPublicID(t), user.ID().Int64(), make([]byte, 16), now, now.Add(time.Hour),
	)
	if err == nil {
		t.Error("32byte以外のtoken hashが登録できた")
	}
}
