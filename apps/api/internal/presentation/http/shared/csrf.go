package shared

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	domainshared "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/shared"
)

// CSRF対策の実装方針 (設計書 18.4 / 24.5):
//
//	signed double-submit cookie方式を採用する。
//	  1. middlewareが非HttpOnly Cookie (less_csrf) へ署名付きtokenを発行する。
//	  2. Webは起動時のGET /api/auth/context でCookieを受け取る。
//	  3. state変更requestでは、Cookieの値をそのまま X-CSRF-Token header へ設定する。
//	  4. serverはtokenの署名を検証し、Cookieとheaderが一致することを確認する。
//
//	Cookieを読めるのは同一オリジンのscriptだけであるため、
//	他オリジンからのform submitやimage tagでは header を付与できず、requestは拒否される。
//	SameSite=Laxだけに依存しないという要件を満たす。
const (
	// CSRFCookieName はCSRF tokenを保持するCookie名。JavaScriptから読む必要があるため非HttpOnly。
	CSRFCookieName = "less_csrf"
	// CSRFHeaderName はCSRF tokenを送信するheader名。
	CSRFHeaderName = "X-CSRF-Token"

	csrfNonceByteLength = 32
)

// ErrCSRFTokenInvalid はCSRF検証に失敗したことを表す。
var ErrCSRFTokenInvalid = domainshared.NewForbiddenError(
	"CSRF_TOKEN_INVALID",
	"セキュリティトークンが無効です。画面を再読み込みしてください。",
)

// CSRFTokenIssuer はCSRF tokenの発行と検証を行う。
type CSRFTokenIssuer struct {
	secret []byte
	secure bool
	ttl    time.Duration
}

// NewCSRFTokenIssuer はCSRFTokenIssuerを生成する。
//
// secureがtrueの場合、CookieへSecure属性を付与する。
// ttlはsession Cookieと同じ値を指定する。CSRF Cookieが先に失効すると、
// loginしたままstate変更requestが拒否される状態になる。
func NewCSRFTokenIssuer(secret string, secure bool, ttl time.Duration) (*CSRFTokenIssuer, error) {
	if len(secret) < 16 {
		return nil, errors.New("csrf: secret must be at least 16 characters")
	}
	if ttl <= 0 {
		return nil, errors.New("csrf: ttl must be positive")
	}
	return &CSRFTokenIssuer{secret: []byte(secret), secure: secure, ttl: ttl}, nil
}

// Issue は新しいCSRF tokenを生成する。
// 形式は "<base64url nonce>.<base64url HMAC-SHA256(nonce)>" とする。
func (i *CSRFTokenIssuer) Issue() (string, error) {
	nonce := make([]byte, csrfNonceByteLength)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("csrf: generate nonce: %w", err)
	}

	encodedNonce := base64.RawURLEncoding.EncodeToString(nonce)
	return encodedNonce + "." + i.sign(encodedNonce), nil
}

// Verify はtokenの署名が正しいかを返す。
func (i *CSRFTokenIssuer) Verify(token string) bool {
	encodedNonce, signature, found := strings.Cut(token, ".")
	if !found || encodedNonce == "" || signature == "" {
		return false
	}
	if _, err := base64.RawURLEncoding.DecodeString(encodedNonce); err != nil {
		return false
	}
	return hmac.Equal([]byte(signature), []byte(i.sign(encodedNonce)))
}

func (i *CSRFTokenIssuer) sign(encodedNonce string) string {
	mac := hmac.New(sha256.New, i.secret)
	mac.Write([]byte(encodedNonce))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// WriteCookie はCSRF token Cookieを書き込む。
//
// JavaScriptから読み取る必要があるためHttpOnlyは付与しない。
// tokenはsecretで署名されているため、値の読み取り自体は攻撃に利用できない。
func (i *CSRFTokenIssuer) WriteCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   i.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(i.ttl.Seconds()),
	})
}

// IssueAndWriteCookie は新しいtokenを発行してCookieへ書き込む。
func (i *CSRFTokenIssuer) IssueAndWriteCookie(w http.ResponseWriter) (string, error) {
	token, err := i.Issue()
	if err != nil {
		return "", err
	}
	i.WriteCookie(w, token)
	return token, nil
}

// CSRFProtection はCSRF tokenの発行と検証を行うmiddlewareを返す。
//
//   - 安全なmethod (GET/HEAD/OPTIONS) では、Cookieが無い・壊れている場合のみ再発行する。
//   - state変更methodでは、Cookieとheaderの一致および署名を検証する。
//     検証に失敗した場合は403を返し、後続処理を実行しない。
func CSRFProtection(issuer *CSRFTokenIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookieToken := readCSRFCookie(r)

			if isSafeMethod(r.Method) {
				if cookieToken == "" || !issuer.Verify(cookieToken) {
					if _, err := issuer.IssueAndWriteCookie(w); err != nil {
						WriteError(w, r, domainshared.NewInternalError(
							"CSRF_TOKEN_ISSUE_FAILED", "サーバーでエラーが発生しました。").WithCause(err))
						return
					}
				}
				next.ServeHTTP(w, r)
				return
			}

			headerToken := strings.TrimSpace(r.Header.Get(CSRFHeaderName))
			if cookieToken == "" || headerToken == "" {
				WriteError(w, r, ErrCSRFTokenInvalid)
				return
			}
			if subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
				WriteError(w, r, ErrCSRFTokenInvalid)
				return
			}
			if !issuer.Verify(cookieToken) {
				WriteError(w, r, ErrCSRFTokenInvalid)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func readCSRFCookie(r *http.Request) string {
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}
