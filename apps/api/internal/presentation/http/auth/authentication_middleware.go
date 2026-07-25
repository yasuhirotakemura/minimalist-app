package auth

import (
	"context"
	"net/http"

	applicationauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/auth"
	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"
)

type authenticatedUserContextKey struct{}

// AuthenticatedUser は認証済みユーザーのcontext表現。
//
// 内部IDは後続のuser data queryで必須となるため保持するが、
// responseへは決して含めない (設計書 12.1 / 18.3)。
type AuthenticatedUser struct {
	ID   domainauth.UserID
	User applicationauth.UserResult
}

// WithAuthenticatedUser は認証済みユーザーをcontextへ格納する。
func WithAuthenticatedUser(ctx context.Context, user AuthenticatedUser) context.Context {
	return context.WithValue(ctx, authenticatedUserContextKey{}, user)
}

// AuthenticatedUserFromContext はcontextの認証済みユーザーを返す。
func AuthenticatedUserFromContext(ctx context.Context) (AuthenticatedUser, bool) {
	user, ok := ctx.Value(authenticatedUserContextKey{}).(AuthenticatedUser)
	return user, ok
}

// Authenticator はsession tokenからユーザーを特定する。
type Authenticator struct {
	service      *applicationauth.GetAuthenticatedUserContextService
	cookieWriter SessionCookieWriter
}

// NewAuthenticator はAuthenticatorを生成する。
func NewAuthenticator(
	service *applicationauth.GetAuthenticatedUserContextService,
	cookieWriter SessionCookieWriter,
) *Authenticator {
	return &Authenticator{service: service, cookieWriter: cookieWriter}
}

// RequireAuthenticatedUser は認証を必須とするmiddlewareを返す。
//
// 認証に失敗した場合は401を返し、後続handlerを実行しない。
func (a *Authenticator) RequireAuthenticatedUser() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			result, err := a.service.Execute(
				r.Context(),
				applicationauth.GetAuthenticatedUserContextParams{
					SessionToken: a.cookieWriter.Read(r),
				},
			)
			if err != nil {
				// 期限切れ・失効済みのCookieは残しておいても意味がないため削除する。
				a.cookieWriter.Clear(w)
				shared.WriteError(w, r, err)
				return
			}

			shared.RecordUserPublicID(r.Context(), result.User.PublicID.String())

			ctx := WithAuthenticatedUser(r.Context(), AuthenticatedUser{
				ID:   result.UserID,
				User: result.User,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
