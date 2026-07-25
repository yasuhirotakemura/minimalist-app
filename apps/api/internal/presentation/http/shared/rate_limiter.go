package shared

import (
	"net/http"
	"sync"
	"time"

	domainshared "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
)

// ErrTooManyRequests はrate limit超過を表す。
var ErrTooManyRequests = domainshared.NewRateLimitedError(
	"TOO_MANY_REQUESTS",
	"試行回数が多すぎます。しばらく待ってから再度お試しください。",
)

// RateLimiter はkeyごとのfixed window rate limiterである。
//
// 本実装はprocess内memoryで状態を保持する。
// 複数instanceで運用する場合は、Redis等の共有storeを使う実装へ差し替える必要がある。
// Phase 0は単一instance構成を前提とする。
type RateLimiter struct {
	mutex   sync.Mutex
	windows map[string]*rateWindow
	limit   int
	window  time.Duration
	clock   clock.Clock
	// lastPrunedAt は不要になったentryを掃除した時刻。
	lastPrunedAt time.Time
}

type rateWindow struct {
	count     int
	expiresAt time.Time
}

// NewRateLimiter はRateLimiterを生成する。
// limitはwindow内で許可するrequest数。
func NewRateLimiter(limit int, window time.Duration, systemClock clock.Clock) *RateLimiter {
	return &RateLimiter{
		windows:      make(map[string]*rateWindow),
		limit:        limit,
		window:       window,
		clock:        systemClock,
		lastPrunedAt: systemClock.Now(),
	}
}

// Allow はkeyに対するrequestを許可するかを返す。
func (l *RateLimiter) Allow(key string) bool {
	if l.limit <= 0 {
		return true
	}

	now := l.clock.Now()

	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.pruneLocked(now)

	current, ok := l.windows[key]
	if !ok || !now.Before(current.expiresAt) {
		l.windows[key] = &rateWindow{count: 1, expiresAt: now.Add(l.window)}
		return true
	}

	if current.count >= l.limit {
		return false
	}
	current.count++
	return true
}

// Reset はkeyの計測状態を破棄する。login成功時などに使用する。
func (l *RateLimiter) Reset(key string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	delete(l.windows, key)
}

// pruneLocked は期限切れentryを削除する。呼び出し側でlockを取得していること。
func (l *RateLimiter) pruneLocked(now time.Time) {
	if now.Sub(l.lastPrunedAt) < l.window {
		return
	}
	for key, window := range l.windows {
		if !now.Before(window.expiresAt) {
			delete(l.windows, key)
		}
	}
	l.lastPrunedAt = now
}

// RateLimitByClientIP は接続元IPアドレス単位でrequestを制限するmiddlewareを返す。
//
// scopeは複数endpointで計測が混ざらないようにするためのprefix。
func RateLimitByClientIP(limiter *RateLimiter, scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow(scope + "|ip|" + ClientIPAddress(r)) {
				WriteError(w, r, ErrTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
