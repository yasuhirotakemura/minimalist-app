// Package auth は認証endpointのHTTP handlerを提供する。
package auth

import (
	"net/http"
	"time"

	domainauth "github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/domain/auth"
)

// SessionCookieWriter はsession Cookieの読み書きを担う (設計書 18.2)。
//
//	HttpOnly : true  (JavaScriptからtokenを読めないようにする)
//	Secure   : 本番true
//	SameSite : Lax
//	Path     : /
//	MaxAge   : SESSION_TTL_HOURS (既定30日)
type SessionCookieWriter struct {
	name   string
	ttl    time.Duration
	secure bool
}

// NewSessionCookieWriter はSessionCookieWriterを生成する。
func NewSessionCookieWriter(name string, ttl time.Duration, secure bool) SessionCookieWriter {
	return SessionCookieWriter{name: name, ttl: ttl, secure: secure}
}

// Name はCookie名を返す。
func (c SessionCookieWriter) Name() string {
	return c.name
}

// Write はsession tokenをCookieへ設定する。
func (c SessionCookieWriter) Write(w http.ResponseWriter, token domainauth.SessionToken) {
	http.SetCookie(w, &http.Cookie{
		Name:     c.name,
		Value:    token.Expose(),
		Path:     "/",
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(c.ttl.Seconds()),
	})
}

// Clear はsession Cookieを削除する。
func (c SessionCookieWriter) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     c.name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// Read はrequestからsession tokenの生の値を取得する。
// Cookieが存在しない場合は空文字を返す。
func (c SessionCookieWriter) Read(r *http.Request) string {
	cookie, err := r.Cookie(c.name)
	if err != nil {
		return ""
	}
	return cookie.Value
}
