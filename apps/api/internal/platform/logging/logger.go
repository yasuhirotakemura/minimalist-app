// Package logging は構造化logを提供する。
//
// server logへ含めるもの (設計書 19.3):
//
//	requestId / userPublicId / method / path / status / durationMs / errorCode
//
// server logへ含めないもの:
//
//	password / session token / Cookie / CSRF secret / 機微情報
package logging

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

// 構造化logのkey。
const (
	KeyRequestID     = "requestId"
	KeyUserPublicID  = "userPublicId"
	KeyMethod        = "method"
	KeyPath          = "path"
	KeyStatus        = "status"
	KeyDurationMs    = "durationMs"
	KeyErrorCode     = "errorCode"
	KeyRemoteAddress = "remoteAddress"
)

// ParseLevel はLOG_LEVEL環境変数をslog.Levelへ変換する。
// 未知の値はinfoとして扱う。
func ParseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// New はJSON形式のloggerを生成する。
func New(out io.Writer, level slog.Level) *slog.Logger {
	handler := slog.NewJSONHandler(out, &slog.HandlerOptions{
		Level: level,
	})
	return slog.New(handler)
}

type loggerContextKey struct{}

// WithLogger はloggerをcontextへ格納する。
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

// FromContext はcontextのloggerを返す。存在しない場合は既定のloggerを返す。
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerContextKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}
