package auth

import (
	"net/http"
	"strings"

	openapitypes "github.com/oapi-codegen/runtime/types"

	applicationauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/application/auth"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/generated/openapi"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/logging"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"
)

// Handler は認証endpointのHTTP handlerである。
//
// Handlerが行うこと (設計書 11.2): body取得、Application Service呼出、
// DTO変換、HTTP error変換、Cookie操作。
// Handlerが行わないこと (設計書 11.3): SQL、業務ルール、Entityの状態遷移。
type Handler struct {
	registerUser          *applicationauth.RegisterUserService
	loginUser             *applicationauth.LoginUserService
	logoutUser            *applicationauth.LogoutUserService
	sessionCookie         SessionCookieWriter
	csrfTokenIssuer       *shared.CSRFTokenIssuer
	loginAttemptLimiter   *shared.RateLimiter
	loginRateLimiterScope string
}

// HandlerDependencies はHandlerの依存。
type HandlerDependencies struct {
	RegisterUser    *applicationauth.RegisterUserService
	LoginUser       *applicationauth.LoginUserService
	LogoutUser      *applicationauth.LogoutUserService
	SessionCookie   SessionCookieWriter
	CSRFTokenIssuer *shared.CSRFTokenIssuer
	// LoginAttemptLimiter はemail単位のlogin試行を制限する (設計書 24.8)。
	LoginAttemptLimiter *shared.RateLimiter
}

// NewHandler はHandlerを生成する。
func NewHandler(dependencies HandlerDependencies) *Handler {
	return &Handler{
		registerUser:          dependencies.RegisterUser,
		loginUser:             dependencies.LoginUser,
		logoutUser:            dependencies.LogoutUser,
		sessionCookie:         dependencies.SessionCookie,
		csrfTokenIssuer:       dependencies.CSRFTokenIssuer,
		loginAttemptLimiter:   dependencies.LoginAttemptLimiter,
		loginRateLimiterScope: "auth.login",
	}
}

// OpenAPIのServerInterfaceを満たすことをcompile時に確認する。
// これによりOpenAPIへendpointを追加した際、実装漏れがbuild errorとなる。
var _ openapi.ServerInterface = (*Handler)(nil)

// RegisterUser はユーザーを登録する。
// POST /api/auth/register
func (h *Handler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var body openapi.RegisterUserJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	result, err := h.registerUser.Execute(r.Context(), applicationauth.RegisterUserParams{
		Email:       body.Email,
		Password:    body.Password,
		DisplayName: body.DisplayName,
		Timezone:    optionalString(body.Timezone),
		Locale:      optionalString(body.Locale),
	})
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	shared.RecordUserPublicID(r.Context(), result.User.PublicID.String())
	shared.WriteJSON(r.Context(), w, http.StatusCreated, toUserResponse(result.User))
}

// LoginUser はloginする。
// POST /api/auth/login
func (h *Handler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var body openapi.LoginUserJSONRequestBody
	if err := shared.DecodeJSONBody(w, r, &body); err != nil {
		shared.WriteError(w, r, err)
		return
	}

	// email単位でも試行回数を制限し、IPを変えた総当たりを抑止する。
	// keyはlowercase化した値とし、大文字小文字の違いで回避されないようにする。
	limiterKey := h.loginRateLimiterScope + "|email|" +
		strings.ToLower(strings.TrimSpace(body.Email))
	if !h.loginAttemptLimiter.Allow(limiterKey) {
		shared.WriteError(w, r, shared.ErrTooManyRequests)
		return
	}

	result, err := h.loginUser.Execute(r.Context(), applicationauth.LoginUserParams{
		Email:     body.Email,
		Password:  body.Password,
		UserAgent: r.UserAgent(),
		IPAddress: shared.ClientIPAddress(r),
	})
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	// login成功で試行回数をresetし、正当な利用者がlockされ続けないようにする。
	h.loginAttemptLimiter.Reset(limiterKey)

	h.sessionCookie.Write(w, result.SessionToken)
	h.rotateCSRFToken(w, r)

	shared.RecordUserPublicID(r.Context(), result.User.PublicID.String())
	shared.WriteJSON(r.Context(), w, http.StatusOK, openapi.AuthenticatedUserContextResponse{
		User: toUserResponse(result.User),
	})
}

// LogoutUser はlogoutする。
// POST /api/auth/logout
//
// 未認証状態で呼び出した場合も204を返す (冪等)。
func (h *Handler) LogoutUser(w http.ResponseWriter, r *http.Request) {
	err := h.logoutUser.Execute(r.Context(), applicationauth.LogoutUserParams{
		SessionToken: h.sessionCookie.Read(r),
	})
	if err != nil {
		shared.WriteError(w, r, err)
		return
	}

	h.sessionCookie.Clear(w)
	h.rotateCSRFToken(w, r)

	shared.WriteNoContent(w)
}

// GetAuthenticatedUserContext は認証contextを取得する。
// GET /api/auth/context
//
// 認証はRequireAuthenticatedUser middlewareが行うため、
// ここではcontextから取り出すだけとする。
func (h *Handler) GetAuthenticatedUserContext(w http.ResponseWriter, r *http.Request) {
	authenticated, ok := AuthenticatedUserFromContext(r.Context())
	if !ok {
		// middlewareの適用漏れ。設定不備として扱う。
		shared.WriteError(w, r, errAuthenticationMiddlewareMissing)
		return
	}

	shared.WriteJSON(r.Context(), w, http.StatusOK, openapi.AuthenticatedUserContextResponse{
		User: toUserResponse(authenticated.User),
	})
}

// rotateCSRFToken は認証状態の変化に合わせてCSRF tokenを再発行する。
//
// login/logoutでtokenを更新することで、認証前に取得したtokenが
// 認証後も有効であり続ける状態を避ける。
// 発行に失敗してもlogin/logout自体は成立しているため、warnのみ記録する。
func (h *Handler) rotateCSRFToken(w http.ResponseWriter, r *http.Request) {
	if _, err := h.csrfTokenIssuer.IssueAndWriteCookie(w); err != nil {
		logging.FromContext(r.Context()).Warn(
			"failed to rotate csrf token",
			"error", err.Error(),
		)
	}
}

func toUserResponse(user applicationauth.UserResult) openapi.UserResponse {
	return openapi.UserResponse{
		PublicId:    openapitypes.UUID(user.PublicID),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Timezone:    user.Timezone,
		Locale:      user.Locale,
		CreatedAt:   user.CreatedAt.UTC(),
		UpdatedAt:   user.UpdatedAt.UTC(),
	}
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
