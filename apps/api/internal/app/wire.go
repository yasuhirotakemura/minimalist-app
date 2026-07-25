// Package app はapplicationの依存関係を組み立てる。
//
// 各layerの生成順序を1箇所へ集約し、cmd/serverを薄く保つ。
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	applicationauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/config"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/crypto"
	infrapostgresql "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/postgresql"
	repositories "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/repositories/postgresql"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/idgenerator"
	presentationhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http"
	authhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/health"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"
)

// Application は組み立て済みのapplicationを表す。
type Application struct {
	Handler http.Handler
	Pool    *pgxpool.Pool
}

// Close は保持している資源を解放する。
func (a *Application) Close() {
	if a.Pool != nil {
		a.Pool.Close()
	}
}

// New はconfigからapplicationを組み立てる。
func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Application, error) {
	pool, err := infrapostgresql.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("initialize database pool: %w", err)
	}

	handler, err := NewHandler(cfg, logger, pool)
	if err != nil {
		pool.Close()
		return nil, err
	}

	return &Application{Handler: handler, Pool: pool}, nil
}

// NewHandler は既存の接続poolからHTTP handlerを組み立てる。
//
// integration testが独自のpoolを渡してrouter全体を検証できるよう公開する。
func NewHandler(
	cfg config.Config,
	logger *slog.Logger,
	pool *pgxpool.Pool,
) (http.Handler, error) {
	systemClock := clock.NewSystemClock()
	publicIDGenerator := idgenerator.NewUUIDv7Generator()
	transactionManager := infrapostgresql.NewTransactionManager(pool)

	passwordHasher, err := crypto.NewArgon2idPasswordHasher(
		cfg.PasswordPepper,
		crypto.DefaultArgon2idParameters(),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize password hasher: %w", err)
	}

	csrfTokenIssuer, err := shared.NewCSRFTokenIssuer(
		cfg.CSRFSecret, cfg.UseSecureCookies(), cfg.SessionTTL)
	if err != nil {
		return nil, fmt.Errorf("initialize csrf token issuer: %w", err)
	}

	authDependencies := applicationauth.Dependencies{
		Users:                 repositories.NewPostgresqlUserRepository(pool),
		Sessions:              repositories.NewPostgresqlAuthSessionRepository(pool),
		PasswordHasher:        passwordHasher,
		SessionTokenGenerator: crypto.NewRandomSessionTokenGenerator(),
		SessionTTL:            cfg.SessionTTL,
	}

	getAuthenticatedUserContext := applicationauth.NewGetAuthenticatedUserContextService(
		authDependencies, systemClock)

	sessionCookie := authhttp.NewSessionCookieWriter(
		cfg.SessionCookieName, cfg.SessionTTL, cfg.UseSecureCookies())

	loginAttemptLimiter := presentationhttp.NewLoginAttemptLimiter(systemClock)

	authHandler := authhttp.NewHandler(authhttp.HandlerDependencies{
		RegisterUser: applicationauth.NewRegisterUserService(
			authDependencies, publicIDGenerator, systemClock, transactionManager),
		LoginUser: applicationauth.NewLoginUserService(
			authDependencies, publicIDGenerator, systemClock, transactionManager),
		LogoutUser:          applicationauth.NewLogoutUserService(authDependencies, systemClock),
		SessionCookie:       sessionCookie,
		CSRFTokenIssuer:     csrfTokenIssuer,
		LoginAttemptLimiter: loginAttemptLimiter,
	})

	return presentationhttp.NewRouter(presentationhttp.RouterDependencies{
		Logger:              logger,
		Clock:               systemClock,
		AuthHandler:         authHandler,
		Authenticator:       authhttp.NewAuthenticator(getAuthenticatedUserContext, sessionCookie),
		HealthHandler:       health.NewHandler(pool),
		CSRFTokenIssuer:     csrfTokenIssuer,
		CORSAllowedOrigins:  cfg.CORSAllowedOrigins,
		LoginAttemptLimiter: loginAttemptLimiter,
	}), nil
}
