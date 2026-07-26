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

	applicationaudit "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/audit"
	applicationauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/auth"
	applicationcategory "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/category"
	applicationitem "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/item"
	applicationtag "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/tag"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/config"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/crypto"
	infrapostgresql "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/postgresql"
	repositories "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/repositories/postgresql"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/clock"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/idgenerator"
	presentationhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http"
	authhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/auth"
	categoryhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/category"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/health"
	itemhttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/item"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"
	taghttp "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/tag"
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

	categoryRepository := repositories.NewPostgresqlCategoryRepository(pool)
	tagRepository := repositories.NewPostgresqlTagRepository(pool)
	itemRepository := repositories.NewPostgresqlItemRepository(pool)
	usageRecordRepository := repositories.NewPostgresqlItemUsageRecordRepository(pool)
	auditRecorder := applicationaudit.NewRecorder(
		repositories.NewPostgresqlAuditLogRepository(pool), publicIDGenerator, systemClock)

	authDependencies := applicationauth.Dependencies{
		Users:                 repositories.NewPostgresqlUserRepository(pool),
		Sessions:              repositories.NewPostgresqlAuthSessionRepository(pool),
		Categories:            categoryRepository,
		PasswordHasher:        passwordHasher,
		SessionTokenGenerator: crypto.NewRandomSessionTokenGenerator(),
		SessionTTL:            cfg.SessionTTL,
	}

	itemDependencies := applicationitem.Dependencies{
		Items:         itemRepository,
		UsageRecords:  usageRecordRepository,
		Categories:    categoryRepository,
		Tags:          tagRepository,
		AuditRecorder: auditRecorder,
	}

	tagDependencies := applicationtag.Dependencies{
		Tags:          tagRepository,
		AuditRecorder: auditRecorder,
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

	categoryHandler := categoryhttp.NewHandler(categoryhttp.HandlerDependencies{
		ListCategories: applicationcategory.NewListCategoriesService(
			applicationcategory.Dependencies{Categories: categoryRepository}),
	})

	itemHandler := itemhttp.NewHandler(itemhttp.HandlerDependencies{
		CreateItem: applicationitem.NewCreateItemService(
			itemDependencies, publicIDGenerator, systemClock, transactionManager),
		UpdateItem: applicationitem.NewUpdateItemService(
			itemDependencies, systemClock, transactionManager),
		GetItem:   applicationitem.NewGetItemService(itemDependencies),
		ListItems: applicationitem.NewListItemsService(itemDependencies),
		ArchiveItem: applicationitem.NewArchiveItemService(
			itemDependencies, systemClock, transactionManager),
		RestoreItem: applicationitem.NewRestoreItemService(
			itemDependencies, systemClock, transactionManager),
		RecordItemUsage: applicationitem.NewRecordItemUsageService(
			itemDependencies, publicIDGenerator, systemClock, transactionManager),
		ListItemUsageRecords: applicationitem.NewListItemUsageRecordsService(itemDependencies),
	})

	tagHandler := taghttp.NewHandler(taghttp.HandlerDependencies{
		ListTags: applicationtag.NewListTagsService(tagDependencies),
		CreateTag: applicationtag.NewCreateTagService(
			tagDependencies, publicIDGenerator, systemClock, transactionManager),
		UpdateTag: applicationtag.NewUpdateTagService(
			tagDependencies, systemClock, transactionManager),
		DeleteTag: applicationtag.NewDeleteTagService(
			tagDependencies, systemClock, transactionManager),
	})

	return presentationhttp.NewRouter(presentationhttp.RouterDependencies{
		Logger: logger,
		Clock:  systemClock,
		APIServer: presentationhttp.NewAPIServer(
			authHandler, categoryHandler, itemHandler, tagHandler),
		Authenticator:       authhttp.NewAuthenticator(getAuthenticatedUserContext, sessionCookie),
		HealthHandler:       health.NewHandler(pool),
		CSRFTokenIssuer:     csrfTokenIssuer,
		CORSAllowedOrigins:  cfg.CORSAllowedOrigins,
		LoginAttemptLimiter: loginAttemptLimiter,
	}), nil
}
