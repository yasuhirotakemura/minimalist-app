package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
)

func mustSessionToken(t *testing.T, raw string) auth.SessionToken {
	t.Helper()
	token, err := auth.NewSessionToken(raw)
	if err != nil {
		t.Fatalf("NewSessionToken(%q) returned error: %v", raw, err)
	}
	return token
}

const validTokenValue = "0123456789abcdef0123456789abcdef0123456789a"

func baseIssueParams(t *testing.T, issuedAt time.Time) auth.IssueAuthSessionParams {
	t.Helper()
	return auth.IssueAuthSessionParams{
		PublicID:  uuid.MustParse("018f8d0a-1c2b-7a3d-9e4f-5a6b7c8d9e0f"),
		UserID:    auth.UserID(1),
		Token:     mustSessionToken(t, validTokenValue),
		IssuedAt:  issuedAt,
		TTL:       30 * 24 * time.Hour,
		UserAgent: "Mozilla/5.0",
		IPAddress: "192.0.2.10",
	}
}

func TestIssueAuthSession_正常系(t *testing.T) {
	t.Parallel()

	issuedAt := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	session, err := auth.IssueAuthSession(baseIssueParams(t, issuedAt))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !session.IssuedAt().Equal(issuedAt) {
		t.Errorf("IssuedAt() = %v, want %v", session.IssuedAt(), issuedAt)
	}
	if want := issuedAt.Add(30 * 24 * time.Hour); !session.ExpiresAt().Equal(want) {
		t.Errorf("ExpiresAt() = %v, want %v", session.ExpiresAt(), want)
	}
	if !session.LastUsedAt().Equal(issuedAt) {
		t.Errorf("LastUsedAt() = %v, want %v", session.LastUsedAt(), issuedAt)
	}
	if session.IsRevoked() {
		t.Error("発行直後のsessionがrevoked扱いになっている")
	}
	if !session.IsLive(issuedAt) {
		t.Error("発行直後のsessionがliveではない")
	}

	// token本体ではなくhashを保持する。
	expectedHash := mustSessionToken(t, validTokenValue).Hash()
	if !session.TokenHash().Equals(expectedHash) {
		t.Error("TokenHash()が期待値と一致しない")
	}
}

func TestIssueAuthSession_時刻はUTCへ正規化される(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("JST", 9*60*60)
	issuedAt := time.Date(2026, 7, 25, 9, 0, 0, 0, location)

	session, err := auth.IssueAuthSession(baseIssueParams(t, issuedAt))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if session.IssuedAt().Location() != time.UTC {
		t.Errorf("IssuedAt().Location() = %v, want UTC", session.IssuedAt().Location())
	}
	if !session.IssuedAt().Equal(issuedAt) {
		t.Errorf("UTC変換で時刻がずれた: %v != %v", session.IssuedAt(), issuedAt)
	}
}

func TestIssueAuthSession_異常系(t *testing.T) {
	t.Parallel()

	issuedAt := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)

	t.Run("publicIDが未設定", func(t *testing.T) {
		t.Parallel()
		params := baseIssueParams(t, issuedAt)
		params.PublicID = uuid.Nil
		if _, err := auth.IssueAuthSession(params); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("userIDが未設定", func(t *testing.T) {
		t.Parallel()
		params := baseIssueParams(t, issuedAt)
		params.UserID = 0
		if _, err := auth.IssueAuthSession(params); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("TTLが0以下", func(t *testing.T) {
		t.Parallel()
		for _, ttl := range []time.Duration{0, -time.Hour} {
			params := baseIssueParams(t, issuedAt)
			params.TTL = ttl
			if _, err := auth.IssueAuthSession(params); err == nil {
				t.Errorf("TTL=%v で error が返らなかった", ttl)
			}
		}
	})
}

func TestAuthSession_IsExpired境界値(t *testing.T) {
	t.Parallel()

	issuedAt := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	params := baseIssueParams(t, issuedAt)
	params.TTL = time.Hour

	session, err := auth.IssueAuthSession(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expiresAt := issuedAt.Add(time.Hour)

	if session.IsExpired(expiresAt.Add(-time.Nanosecond)) {
		t.Error("期限直前を期限切れと判定した")
	}
	// 期限と同時刻は期限切れとして扱う。
	if !session.IsExpired(expiresAt) {
		t.Error("期限ちょうどを有効と判定した")
	}
	if !session.IsExpired(expiresAt.Add(time.Nanosecond)) {
		t.Error("期限経過後を有効と判定した")
	}
	if session.IsLive(expiresAt) {
		t.Error("期限切れsessionがliveと判定された")
	}
}

func TestAuthSession_失効済みはliveではない(t *testing.T) {
	t.Parallel()

	issuedAt := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	revokedAt := issuedAt.Add(time.Minute)

	session := auth.ReconstructAuthSession(auth.ReconstructAuthSessionParams{
		ID:         1,
		PublicID:   uuid.MustParse("018f8d0a-1c2b-7a3d-9e4f-5a6b7c8d9e0f"),
		UserID:     auth.UserID(1),
		TokenHash:  mustSessionToken(t, validTokenValue).Hash(),
		IssuedAt:   issuedAt,
		ExpiresAt:  issuedAt.Add(time.Hour),
		LastUsedAt: issuedAt,
		RevokedAt:  &revokedAt,
	})

	if !session.IsRevoked() {
		t.Error("IsRevoked()がfalseを返した")
	}
	if session.IsLive(issuedAt.Add(2 * time.Minute)) {
		t.Error("失効済みsessionがliveと判定された")
	}
}

func TestAuthSession_NeedsLastUsedAtRefresh(t *testing.T) {
	t.Parallel()

	issuedAt := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	session, err := auth.IssueAuthSession(baseIssueParams(t, issuedAt))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if session.NeedsLastUsedAtRefresh(issuedAt.Add(auth.SessionTouchInterval - time.Second)) {
		t.Error("更新間隔未満で更新が必要と判定された")
	}
	if !session.NeedsLastUsedAtRefresh(issuedAt.Add(auth.SessionTouchInterval)) {
		t.Error("更新間隔ちょうどで更新が不要と判定された")
	}
}

func TestSessionToken_Hashは同じtokenで一致する(t *testing.T) {
	t.Parallel()

	first := mustSessionToken(t, validTokenValue).Hash()
	second := mustSessionToken(t, validTokenValue).Hash()
	other := mustSessionToken(t, "ffffffffffffffffffffffffffffffffffffffffffff").Hash()

	if !first.Equals(second) {
		t.Error("同一tokenのhashが一致しない")
	}
	if first.Equals(other) {
		t.Error("異なるtokenのhashが一致した")
	}
	if len(first.Bytes()) != auth.SessionTokenHashByteLength {
		t.Errorf("hash長 = %d, want %d", len(first.Bytes()), auth.SessionTokenHashByteLength)
	}
}

func TestNewSessionToken_異常系(t *testing.T) {
	t.Parallel()

	if _, err := auth.NewSessionToken(""); err == nil {
		t.Error("空tokenを受け入れた")
	}
	if _, err := auth.NewSessionToken("too-short"); err == nil {
		t.Error("短すぎるtokenを受け入れた")
	}
}
