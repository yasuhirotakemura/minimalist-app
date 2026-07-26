// Package http はHTTP routingを組み立てる。
package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	domainshared "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	authhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/health"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"
)

// requestTimeout はrequest処理の上限時間。
const requestTimeout = 30 * time.Second

// login/registerのrate limit設定 (設計書 24.8)。
const (
	authRateLimitPerIP     = 20
	authRateLimitPerEmail  = 5
	authRateLimitWindow    = 5 * time.Minute
	authRateLimitScopeIP   = "auth"
	corsPreflightMaxAgeSec = 300
)

// RouterDependencies はrouter構築の依存。
type RouterDependencies struct {
	Logger             *slog.Logger
	Clock              clock.Clock
	APIServer          *APIServer
	Authenticator      *authhttp.Authenticator
	HealthHandler      *health.Handler
	CSRFTokenIssuer    *shared.CSRFTokenIssuer
	CORSAllowedOrigins []string
	// LoginAttemptLimiter はauth handlerと共有し、email単位の制限に使用する。
	LoginAttemptLimiter *shared.RateLimiter
}

// NewRouter はHTTP routerを構築する。
//
// routeとmiddlewareの対応:
//
//	/health/*            : 監視用。認証・CSRF検証を行わない。
//	/api/*               : request ID、log、security header、CORS、CSRF検証。
//	/api/auth/register   : IP単位のrate limit。
//	/api/auth/login      : IP単位のrate limit (email単位はhandler内)。
//	/api/auth/context    : 認証必須。
//	/api/categories      : 認証必須。
//	/api/tags            : 認証必須。
//	/api/items           : 認証必須。
//
// path・query parameterの解釈はOpenAPI生成のServerInterfaceWrapperへ委ね、
// routeごとのmiddleware適用のためroute登録は明示的に行う。
func NewRouter(dependencies RouterDependencies) http.Handler {
	router := chi.NewRouter()
	wrapper := newServerWrapper(dependencies.APIServer)

	router.Use(shared.RequestID())
	router.Use(shared.InjectLogger(dependencies.Logger))
	router.Use(shared.Recover())
	router.Use(shared.AccessLog(dependencies.Clock))
	router.Use(shared.SecurityHeaders())
	router.Use(shared.Timeout(requestTimeout))

	// 監視endpointは認証・CSRF検証の対象外とする。
	router.Get("/health/live", dependencies.HealthHandler.Live)
	router.Get("/health/ready", dependencies.HealthHandler.Ready)

	ipRateLimiter := shared.NewRateLimiter(
		authRateLimitPerIP, authRateLimitWindow, dependencies.Clock)

	router.Route("/api", func(apiRouter chi.Router) {
		if len(dependencies.CORSAllowedOrigins) > 0 {
			apiRouter.Use(newCORSMiddleware(dependencies.CORSAllowedOrigins))
		}
		apiRouter.Use(shared.CSRFProtection(dependencies.CSRFTokenIssuer))

		apiRouter.Route("/auth", func(authRouter chi.Router) {
			authRouter.Group(func(publicRouter chi.Router) {
				publicRouter.Use(shared.RateLimitByClientIP(ipRateLimiter, authRateLimitScopeIP))
				publicRouter.Post("/register", wrapper.RegisterUser)
				publicRouter.Post("/login", wrapper.LoginUser)
			})

			// logoutは未認証でも204を返すため、認証middlewareを適用しない。
			authRouter.Post("/logout", wrapper.LogoutUser)

			authRouter.Group(func(protectedRouter chi.Router) {
				protectedRouter.Use(dependencies.Authenticator.RequireAuthenticatedUser())
				protectedRouter.Get("/context", wrapper.GetAuthenticatedUserContext)
			})
		})

		// 所持品・カテゴリー・タグは全て認証必須とする。
		apiRouter.Group(func(protectedRouter chi.Router) {
			protectedRouter.Use(dependencies.Authenticator.RequireAuthenticatedUser())

			protectedRouter.Get("/categories", wrapper.ListCategories)

			protectedRouter.Route("/tags", func(tagRouter chi.Router) {
				tagRouter.Get("/", wrapper.ListTags)
				tagRouter.Post("/", wrapper.CreateTag)
				tagRouter.Put("/{publicId}", wrapper.UpdateTag)
				tagRouter.Delete("/{publicId}", wrapper.DeleteTag)
			})

			protectedRouter.Route("/items", func(itemRouter chi.Router) {
				itemRouter.Get("/", wrapper.ListItems)
				itemRouter.Post("/", wrapper.CreateItem)
				itemRouter.Get("/{publicId}", wrapper.GetItemByPublicId)
				itemRouter.Put("/{publicId}", wrapper.UpdateItem)
				itemRouter.Post("/{publicId}/archive", wrapper.ArchiveItem)
				itemRouter.Post("/{publicId}/restore", wrapper.RestoreItem)
				itemRouter.Get("/{publicId}/usage-records", wrapper.ListItemUsageRecords)
				itemRouter.Post("/{publicId}/usage-records", wrapper.CreateItemUsageRecord)
			})
		})

		apiRouter.NotFound(writeNotFound)
		apiRouter.MethodNotAllowed(writeMethodNotAllowed)
	})

	router.NotFound(writeNotFound)
	router.MethodNotAllowed(writeMethodNotAllowed)

	return router
}

// NewLoginAttemptLimiter はemail単位のlogin試行制限を生成する。
func NewLoginAttemptLimiter(systemClock clock.Clock) *shared.RateLimiter {
	return shared.NewRateLimiter(authRateLimitPerEmail, authRateLimitWindow, systemClock)
}

func newCORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders:   []string{"Content-Type", shared.CSRFHeaderName},
		ExposedHeaders:   []string{shared.RequestIDHeader},
		AllowCredentials: true,
		MaxAge:           corsPreflightMaxAgeSec,
	})
}

func writeNotFound(w http.ResponseWriter, r *http.Request) {
	shared.WriteError(w, r, domainshared.NewNotFoundError(
		"RESOURCE_NOT_FOUND", "指定されたリソースが見つかりません。"))
}

func writeMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	shared.WriteError(w, r, domainshared.NewInvalidInputError(
		"METHOD_NOT_ALLOWED", "このリクエストメソッドは許可されていません。"))
}
