package shared_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"
)

func TestRateLimiter_上限までは許可し超過で拒否する(t *testing.T) {
	t.Parallel()

	fixedClock := clock.NewFixedClock(time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC))
	limiter := shared.NewRateLimiter(3, time.Minute, fixedClock)

	for attempt := 1; attempt <= 3; attempt++ {
		if !limiter.Allow("key") {
			t.Fatalf("%d回目のrequestが拒否された", attempt)
		}
	}
	if limiter.Allow("key") {
		t.Error("上限超過のrequestが許可された")
	}
}

func TestRateLimiter_keyごとに独立して計測する(t *testing.T) {
	t.Parallel()

	fixedClock := clock.NewFixedClock(time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC))
	limiter := shared.NewRateLimiter(1, time.Minute, fixedClock)

	if !limiter.Allow("first") {
		t.Fatal("firstの1回目が拒否された")
	}
	if limiter.Allow("first") {
		t.Error("firstの2回目が許可された")
	}
	if !limiter.Allow("second") {
		t.Error("secondの1回目が拒否された")
	}
}

func TestRateLimiter_window経過後にresetされる(t *testing.T) {
	t.Parallel()

	fixedClock := clock.NewFixedClock(time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC))
	limiter := shared.NewRateLimiter(1, time.Minute, fixedClock)

	if !limiter.Allow("key") {
		t.Fatal("1回目が拒否された")
	}

	fixedClock.Advance(59 * time.Second)
	if limiter.Allow("key") {
		t.Error("window内なのに許可された")
	}

	fixedClock.Advance(time.Second)
	if !limiter.Allow("key") {
		t.Error("window経過後に拒否された")
	}
}

func TestRateLimiter_Resetで計測状態を破棄する(t *testing.T) {
	t.Parallel()

	fixedClock := clock.NewFixedClock(time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC))
	limiter := shared.NewRateLimiter(1, time.Minute, fixedClock)

	if !limiter.Allow("key") {
		t.Fatal("1回目が拒否された")
	}
	if limiter.Allow("key") {
		t.Fatal("2回目が許可された")
	}

	limiter.Reset("key")

	if !limiter.Allow("key") {
		t.Error("Reset後に拒否された")
	}
}

func TestRateLimiter_上限0以下は無制限として扱う(t *testing.T) {
	t.Parallel()

	fixedClock := clock.NewFixedClock(time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC))
	limiter := shared.NewRateLimiter(0, time.Minute, fixedClock)

	for range 100 {
		if !limiter.Allow("key") {
			t.Fatal("上限0のlimiterが拒否した")
		}
	}
}

func TestRateLimitByClientIP_超過時に429を返す(t *testing.T) {
	t.Parallel()

	fixedClock := clock.NewFixedClock(time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC))
	limiter := shared.NewRateLimiter(1, time.Minute, fixedClock)

	handler := shared.RequestID()(
		shared.RateLimitByClientIP(limiter, "auth")(okHandler()),
	)

	newRequest := func() *http.Request {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		request.RemoteAddr = "192.0.2.10:54321"
		return request
	}

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, newRequest())
	if first.Code != http.StatusOK {
		t.Fatalf("1回目 status = %d, want %d", first.Code, http.StatusOK)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, newRequest())
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("2回目 status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}

	// 別IPは影響を受けない。
	otherIP := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	otherIP.RemoteAddr = "192.0.2.11:54321"

	third := httptest.NewRecorder()
	handler.ServeHTTP(third, otherIP)
	if third.Code != http.StatusOK {
		t.Errorf("別IP status = %d, want %d", third.Code, http.StatusOK)
	}
}
