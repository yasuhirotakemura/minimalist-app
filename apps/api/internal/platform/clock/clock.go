// Package clock は現在時刻の取得を抽象化する。
// 時刻はUTCで扱う (設計書 4.3)。
package clock

import "time"

// Clock は現在時刻を返す。testでは固定時刻へ差し替える。
type Clock interface {
	Now() time.Time
}

// SystemClock はOSの時刻をUTCで返す。
type SystemClock struct{}

// NewSystemClock はSystemClockを返す。
func NewSystemClock() SystemClock {
	return SystemClock{}
}

// Now は現在時刻をUTCで返す。
func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

// FixedClock は常に同じ時刻を返す。test専用。
type FixedClock struct {
	Instant time.Time
}

// NewFixedClock は指定時刻を返すFixedClockを生成する。
func NewFixedClock(instant time.Time) *FixedClock {
	return &FixedClock{Instant: instant.UTC()}
}

// Now は保持している時刻を返す。
func (c *FixedClock) Now() time.Time {
	return c.Instant
}

// Advance は保持している時刻を進める。
func (c *FixedClock) Advance(d time.Duration) {
	c.Instant = c.Instant.Add(d)
}
