package shared

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/logging"
)

// RequestIDHeader はresponseへ返すrequest IDのheader名。
const RequestIDHeader = "X-Request-Id"

// RequestID はrequestごとに一意なIDを採番し、contextとresponse headerへ設定する。
//
// 受信したX-Request-Idは信用せず、常にserver側で採番する。
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := newRequestID()
			w.Header().Set(RequestIDHeader, requestID)

			ctx := WithRequestID(r.Context(), requestID)
			ctx = WithRequestScope(ctx, &RequestScope{})

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func newRequestID() string {
	generated, err := uuid.NewV7()
	if err != nil {
		// UUID生成に失敗してもrequest処理は継続する。
		return "req_unknown"
	}
	return "req_" + strings.ReplaceAll(generated.String(), "-", "")
}

// InjectLogger はrequest IDを付与したloggerをcontextへ格納する。
func InjectLogger(baseLogger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := baseLogger.With(logging.KeyRequestID, RequestIDFromContext(r.Context()))
			next.ServeHTTP(w, r.WithContext(logging.WithLogger(r.Context(), logger)))
		})
	}
}

// statusRecorder はresponse statusとbody長を記録する。
type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytesWitten int
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	written, err := w.ResponseWriter.Write(body)
	w.bytesWitten += written
	return written, err
}

// AccessLog はrequest単位のaccess logを出力する (設計書 19.3)。
//
// 出力する項目: requestId / userPublicId / method / path / status / durationMs / errorCode
// 出力しない項目: password / session token / Cookie / CSRF secret
func AccessLog(systemClock clock.Clock) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := systemClock.Now()
			recorder := &statusRecorder{ResponseWriter: w}

			next.ServeHTTP(recorder, r)

			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}
			durationMs := systemClock.Now().Sub(startedAt).Milliseconds()

			attributes := []any{
				logging.KeyMethod, r.Method,
				logging.KeyPath, r.URL.Path,
				logging.KeyStatus, status,
				logging.KeyDurationMs, durationMs,
				logging.KeyRemoteAddress, ClientIPAddress(r),
			}
			if scope, ok := RequestScopeFromContext(r.Context()); ok {
				if errorCode := scope.ErrorCode(); errorCode != "" {
					attributes = append(attributes, logging.KeyErrorCode, errorCode)
				}
				if userPublicID := scope.UserPublicID(); userPublicID != "" {
					attributes = append(attributes, logging.KeyUserPublicID, userPublicID)
				}
			}

			logger := logging.FromContext(r.Context())
			switch {
			case status >= http.StatusInternalServerError:
				logger.Error("request completed", attributes...)
			case status >= http.StatusBadRequest:
				logger.Warn("request completed", attributes...)
			default:
				logger.Info("request completed", attributes...)
			}
		})
	}
}

// Recover はpanicを500へ変換する。stack traceはresponseへ含めない (設計書 19.3)。
func Recover() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}
				if recovered == http.ErrAbortHandler {
					panic(recovered)
				}

				ctx := r.Context()
				RecordErrorCode(ctx, internalErrorCode)
				logging.FromContext(ctx).Error(
					"panic recovered",
					"panic", recovered,
					logging.KeyRequestID, RequestIDFromContext(ctx),
				)

				WriteJSON(ctx, w, http.StatusInternalServerError, ErrorResponse{
					Code:        internalErrorCode,
					Message:     internalErrorMessage,
					FieldErrors: []FieldErrorResponse{},
					RequestID:   RequestIDFromContext(ctx),
				})
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders はAPI responseへsecurity headerを付与する (設計書 24)。
//
// APIはJSONのみを返すため、script実行を全面的に禁止するCSPを設定する。
// 認証情報を含むresponseがcacheされないよう no-store を指定する。
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := w.Header()
			header.Set("X-Content-Type-Options", "nosniff")
			header.Set("Referrer-Policy", "no-referrer")
			header.Set("X-Frame-Options", "DENY")
			header.Set("Content-Security-Policy",
				"default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
			header.Set("Cache-Control", "no-store")
			header.Set("Pragma", "no-cache")

			next.ServeHTTP(w, r)
		})
	}
}

// Timeout はrequest処理へ上限時間を設定する。
func Timeout(limit time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, limit, "")
	}
}

// ClientIPAddress は接続元IPアドレスを返す。
//
// X-Forwarded-For等のheaderは偽装可能なため信用しない。
// 前段のreverse proxyを信用する構成へ変更する場合は、
// 信頼できるproxyのIP範囲を検証したうえで参照する。
func ClientIPAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
