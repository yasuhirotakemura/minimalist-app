//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/app"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/infrastructure/config"
	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"
)

// testConfig はintegration test用の設定。
//
// Argon2idの計算量は本番既定値のまま使用し、実際のlogin経路を検証する。
func testConfig() config.Config {
	return config.Config{
		AppEnv:            config.EnvLocal,
		WebBaseURL:        "http://localhost:8080",
		APIPort:           8081,
		SessionCookieName: "less_session",
		SessionTTL:        30 * 24 * time.Hour,
		PasswordPepper:    "integration-test-pepper-value",
		CSRFSecret:        "integration-test-csrf-secret",
		LogLevel:          slog.LevelError,
		MaxImportSizeMB:   10,
		ExportTTL:         15 * time.Minute,
	}
}

// apiClient はCookieを保持しつつAPIを呼び出すtest用clientである。
type apiClient struct {
	t       *testing.T
	handler http.Handler
	cookies map[string]string
}

func newAPIClient(t *testing.T) *apiClient {
	t.Helper()

	handler, err := app.NewHandler(
		testConfig(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		testPool,
	)
	if err != nil {
		t.Fatalf("app.NewHandler returned error: %v", err)
	}

	return &apiClient{t: t, handler: handler, cookies: make(map[string]string)}
}

// do はrequestを送信し、Set-Cookieを保持する。
func (c *apiClient) do(
	method, path string,
	body any,
	options ...func(*http.Request),
) *http.Response {
	c.t.Helper()

	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("failed to encode request body: %v", err)
		}
		payload = bytes.NewReader(encoded)
	}

	request := httptest.NewRequest(method, path, payload)
	request.RemoteAddr = "192.0.2.10:54321"
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for name, value := range c.cookies {
		request.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	// CSRF tokenはCookieの値をそのままheaderへ設定する (double-submit)。
	if token, ok := c.cookies[shared.CSRFCookieName]; ok {
		request.Header.Set(shared.CSRFHeaderName, token)
	}
	for _, option := range options {
		option(request)
	}

	recorder := httptest.NewRecorder()
	c.handler.ServeHTTP(recorder, request)

	response := recorder.Result()
	for _, cookie := range response.Cookies() {
		if cookie.MaxAge < 0 {
			delete(c.cookies, cookie.Name)
			continue
		}
		c.cookies[cookie.Name] = cookie.Value
	}
	return response
}

// bootstrapCSRF はWeb起動時と同じくGET /api/auth/context でCSRF Cookieを受け取る。
func (c *apiClient) bootstrapCSRF() {
	c.t.Helper()
	response := c.do(http.MethodGet, "/api/auth/context", nil)
	_ = response.Body.Close()

	if _, ok := c.cookies[shared.CSRFCookieName]; !ok {
		c.t.Fatal("CSRF Cookieが発行されなかった")
	}
}

func decodeBody[T any](t *testing.T, response *http.Response) T {
	t.Helper()
	defer func() { _ = response.Body.Close() }()

	var decoded T
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return decoded
}

type userResponseBody struct {
	PublicID    string `json:"publicId"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Timezone    string `json:"timezone"`
	Locale      string `json:"locale"`
}

type authContextBody struct {
	User userResponseBody `json:"user"`
}

func registerPayload(email string) map[string]any {
	return map[string]any{
		"email":       email,
		"password":    "correct-horse-battery",
		"displayName": "テストユーザー",
	}
}

func loginPayload(email, password string) map[string]any {
	return map[string]any{"email": email, "password": password}
}

func TestAuthAPI_register_login_context_logout(t *testing.T) {
	truncateAll(t)

	client := newAPIClient(t)
	client.bootstrapCSRF()

	// register
	registerResponse := client.do(
		http.MethodPost, "/api/auth/register", registerPayload("user@example.com"))
	if registerResponse.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d, want %d", registerResponse.StatusCode, http.StatusCreated)
	}
	registered := decodeBody[userResponseBody](t, registerResponse)
	if registered.Email != "user@example.com" {
		t.Errorf("email = %q", registered.Email)
	}
	if registered.PublicID == "" {
		t.Error("publicIdが空")
	}
	if registered.Timezone != "Asia/Tokyo" || registered.Locale != "ja-JP" {
		t.Errorf("既定値が適用されていない: %+v", registered)
	}

	// registerはsessionを発行しない。
	if _, ok := client.cookies["less_session"]; ok {
		t.Error("registerでsession Cookieが発行された")
	}

	// login
	loginResponse := client.do(
		http.MethodPost, "/api/auth/login",
		loginPayload("user@example.com", "correct-horse-battery"))
	if loginResponse.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginResponse.StatusCode, http.StatusOK)
	}

	sessionCookie := findResponseCookie(loginResponse, "less_session")
	if sessionCookie == nil {
		t.Fatal("session Cookieが発行されなかった")
	}
	if !sessionCookie.HttpOnly {
		t.Error("session CookieがHttpOnlyではない")
	}
	if sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", sessionCookie.SameSite)
	}
	if sessionCookie.Secure {
		t.Error("APP_ENV=local でSecureが付与された")
	}

	loggedIn := decodeBody[authContextBody](t, loginResponse)
	if loggedIn.User.PublicID != registered.PublicID {
		t.Errorf("publicId = %q, want %q", loggedIn.User.PublicID, registered.PublicID)
	}

	// DBへsessionが保存され、token本体は保存されていない。
	assertSessionCount(t, 1)
	assertTokenNotStored(t, sessionCookie.Value)

	// context
	contextResponse := client.do(http.MethodGet, "/api/auth/context", nil)
	if contextResponse.StatusCode != http.StatusOK {
		t.Fatalf("context status = %d, want %d", contextResponse.StatusCode, http.StatusOK)
	}
	authenticated := decodeBody[authContextBody](t, contextResponse)
	if authenticated.User.PublicID != registered.PublicID {
		t.Errorf("publicId = %q, want %q", authenticated.User.PublicID, registered.PublicID)
	}

	// logout
	logoutResponse := client.do(http.MethodPost, "/api/auth/logout", nil)
	if logoutResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", logoutResponse.StatusCode, http.StatusNoContent)
	}
	_ = logoutResponse.Body.Close()

	if _, ok := client.cookies["less_session"]; ok {
		t.Error("logout後もsession Cookieが残っている")
	}

	// logout後はcontextを取得できない。
	afterLogout := client.do(http.MethodGet, "/api/auth/context", nil)
	if afterLogout.StatusCode != http.StatusUnauthorized {
		t.Errorf("logout後のcontext status = %d, want %d",
			afterLogout.StatusCode, http.StatusUnauthorized)
	}
	_ = afterLogout.Body.Close()
}

func TestAuthAPI_未認証でcontextは401(t *testing.T) {
	truncateAll(t)

	client := newAPIClient(t)

	response := client.do(http.MethodGet, "/api/auth/context", nil)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
	}

	body := decodeBody[shared.ErrorResponse](t, response)
	if body.Code != "UNAUTHENTICATED" {
		t.Errorf("code = %q, want UNAUTHENTICATED", body.Code)
	}
	if body.RequestID == "" {
		t.Error("requestIdが空")
	}
	if body.FieldErrors == nil {
		t.Error("fieldErrorsがnull (空配列であるべき)")
	}
}

func TestAuthAPI_CSRF_tokenなしのstate変更requestは403(t *testing.T) {
	truncateAll(t)

	client := newAPIClient(t)
	// bootstrapCSRFを呼ばず、CSRF Cookieを持たない状態で送信する。

	response := client.do(
		http.MethodPost, "/api/auth/register", registerPayload("user@example.com"))
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}

	body := decodeBody[shared.ErrorResponse](t, response)
	if body.Code != "CSRF_TOKEN_INVALID" {
		t.Errorf("code = %q, want CSRF_TOKEN_INVALID", body.Code)
	}

	// 拒否されたrequestでユーザーが作成されていないこと。
	assertUserCount(t, 0)
}

func TestAuthAPI_CSRF_headerが一致しない場合は403(t *testing.T) {
	truncateAll(t)

	client := newAPIClient(t)
	client.bootstrapCSRF()

	response := client.do(
		http.MethodPost, "/api/auth/register", registerPayload("user@example.com"),
		func(request *http.Request) {
			request.Header.Set(shared.CSRFHeaderName, "tampered.token")
		},
	)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}
	_ = response.Body.Close()

	assertUserCount(t, 0)
}

func TestAuthAPI_email重複は409(t *testing.T) {
	truncateAll(t)

	client := newAPIClient(t)
	client.bootstrapCSRF()

	first := client.do(http.MethodPost, "/api/auth/register", registerPayload("user@example.com"))
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("1件目 status = %d, want %d", first.StatusCode, http.StatusCreated)
	}
	_ = first.Body.Close()

	second := client.do(http.MethodPost, "/api/auth/register", registerPayload("USER@example.com"))
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("2件目 status = %d, want %d", second.StatusCode, http.StatusConflict)
	}

	body := decodeBody[shared.ErrorResponse](t, second)
	if body.Code != "EMAIL_ALREADY_REGISTERED" {
		t.Errorf("code = %q, want EMAIL_ALREADY_REGISTERED", body.Code)
	}

	assertUserCount(t, 1)
}

func TestAuthAPI_入力不正は400でfieldErrorを返す(t *testing.T) {
	truncateAll(t)

	client := newAPIClient(t)
	client.bootstrapCSRF()

	response := client.do(http.MethodPost, "/api/auth/register", map[string]any{
		"email":       "invalid-email",
		"password":    "correct-horse-battery",
		"displayName": "テストユーザー",
	})
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}

	body := decodeBody[shared.ErrorResponse](t, response)
	if len(body.FieldErrors) != 1 || body.FieldErrors[0].Field != "email" {
		t.Errorf("fieldErrors = %+v, want email error", body.FieldErrors)
	}
}

func TestAuthAPI_未知のfieldを拒否する(t *testing.T) {
	truncateAll(t)

	client := newAPIClient(t)
	client.bootstrapCSRF()

	response := client.do(http.MethodPost, "/api/auth/register", map[string]any{
		"email":       "user@example.com",
		"password":    "correct-horse-battery",
		"displayName": "テストユーザー",
		"isAdmin":     true,
	})
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}
	_ = response.Body.Close()

	assertUserCount(t, 0)
}

func TestAuthAPI_login失敗は401でemailの存在有無を区別しない(t *testing.T) {
	truncateAll(t)

	client := newAPIClient(t)
	client.bootstrapCSRF()

	registerResponse := client.do(
		http.MethodPost, "/api/auth/register", registerPayload("user@example.com"))
	if registerResponse.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d", registerResponse.StatusCode)
	}
	_ = registerResponse.Body.Close()

	testCases := []struct {
		name     string
		email    string
		password string
	}{
		{name: "存在しないemail", email: "unknown@example.com", password: "correct-horse-battery"},
		{name: "passwordが誤り", email: "user@example.com", password: "wrong-password-value"},
	}

	var messages []string
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			response := client.do(
				http.MethodPost, "/api/auth/login",
				loginPayload(testCase.email, testCase.password))
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
			}

			body := decodeBody[shared.ErrorResponse](t, response)
			if body.Code != "INVALID_CREDENTIALS" {
				t.Errorf("code = %q, want INVALID_CREDENTIALS", body.Code)
			}
			messages = append(messages, body.Message)
		})
	}

	if len(messages) == 2 && messages[0] != messages[1] {
		t.Errorf("errorの内容がemailの存在有無で異なる: %q vs %q", messages[0], messages[1])
	}
}

func TestAuthAPI_未認証のlogoutも204(t *testing.T) {
	truncateAll(t)

	client := newAPIClient(t)
	client.bootstrapCSRF()

	response := client.do(http.MethodPost, "/api/auth/logout", nil)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	_ = response.Body.Close()
}

func TestAuthAPI_他ユーザーのsessionでは自分の情報しか見えない(t *testing.T) {
	truncateAll(t)

	first := newAPIClient(t)
	first.bootstrapCSRF()
	registerAndLogin(t, first, "first@example.com")

	second := newAPIClient(t)
	second.bootstrapCSRF()
	registerAndLogin(t, second, "second@example.com")

	firstContext := decodeBody[authContextBody](t, first.do(http.MethodGet, "/api/auth/context", nil))
	secondContext := decodeBody[authContextBody](t, second.do(http.MethodGet, "/api/auth/context", nil))

	if firstContext.User.Email != "first@example.com" {
		t.Errorf("first email = %q", firstContext.User.Email)
	}
	if secondContext.User.Email != "second@example.com" {
		t.Errorf("second email = %q", secondContext.User.Email)
	}
	if firstContext.User.PublicID == secondContext.User.PublicID {
		t.Error("異なるsessionで同一ユーザーが解決された")
	}
}

func TestAuthAPI_login_rate_limit(t *testing.T) {
	truncateAll(t)

	client := newAPIClient(t)
	client.bootstrapCSRF()

	registerResponse := client.do(
		http.MethodPost, "/api/auth/register", registerPayload("user@example.com"))
	if registerResponse.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d", registerResponse.StatusCode)
	}
	_ = registerResponse.Body.Close()

	// email単位の上限は5回。6回目で429になる。
	for attempt := 1; attempt <= 5; attempt++ {
		response := client.do(
			http.MethodPost, "/api/auth/login",
			loginPayload("user@example.com", "wrong-password-value"))
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%d回目 status = %d, want %d",
				attempt, response.StatusCode, http.StatusUnauthorized)
		}
		_ = response.Body.Close()
	}

	blocked := client.do(
		http.MethodPost, "/api/auth/login",
		loginPayload("user@example.com", "correct-horse-battery"))
	if blocked.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("6回目 status = %d, want %d", blocked.StatusCode, http.StatusTooManyRequests)
	}

	body := decodeBody[shared.ErrorResponse](t, blocked)
	if body.Code != "TOO_MANY_REQUESTS" {
		t.Errorf("code = %q, want TOO_MANY_REQUESTS", body.Code)
	}
}

func TestAuthAPI_health(t *testing.T) {
	client := newAPIClient(t)

	live := client.do(http.MethodGet, "/health/live", nil)
	if live.StatusCode != http.StatusOK {
		t.Errorf("live status = %d, want %d", live.StatusCode, http.StatusOK)
	}
	_ = live.Body.Close()

	ready := client.do(http.MethodGet, "/health/ready", nil)
	if ready.StatusCode != http.StatusOK {
		t.Errorf("ready status = %d, want %d", ready.StatusCode, http.StatusOK)
	}
	_ = ready.Body.Close()
}

func TestAuthAPI_security_headerとrequestIDを返す(t *testing.T) {
	client := newAPIClient(t)

	response := client.do(http.MethodGet, "/api/auth/context", nil)
	defer func() { _ = response.Body.Close() }()

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Cache-Control":          "no-store",
	}
	for name, expected := range expectedHeaders {
		if actual := response.Header.Get(name); actual != expected {
			t.Errorf("%s = %q, want %q", name, actual, expected)
		}
	}
	if response.Header.Get(shared.RequestIDHeader) == "" {
		t.Error("X-Request-Idが空")
	}
}

func TestAuthAPI_存在しないpathは404(t *testing.T) {
	client := newAPIClient(t)

	response := client.do(http.MethodGet, "/api/unknown", nil)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}

	body := decodeBody[shared.ErrorResponse](t, response)
	if body.Code != "RESOURCE_NOT_FOUND" {
		t.Errorf("code = %q, want RESOURCE_NOT_FOUND", body.Code)
	}
}

// --- helpers -------------------------------------------------------------

func registerAndLogin(t *testing.T, client *apiClient, email string) {
	t.Helper()

	registerResponse := client.do(http.MethodPost, "/api/auth/register", registerPayload(email))
	if registerResponse.StatusCode != http.StatusCreated {
		t.Fatalf("register(%s) status = %d", email, registerResponse.StatusCode)
	}
	_ = registerResponse.Body.Close()

	loginResponse := client.do(
		http.MethodPost, "/api/auth/login", loginPayload(email, "correct-horse-battery"))
	if loginResponse.StatusCode != http.StatusOK {
		t.Fatalf("login(%s) status = %d", email, loginResponse.StatusCode)
	}
	_ = loginResponse.Body.Close()
}

func findResponseCookie(response *http.Response, name string) *http.Cookie {
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func assertUserCount(t *testing.T, expected int) {
	t.Helper()

	var count int
	if err := testPool.QueryRow(
		context.Background(), `SELECT count(*) FROM identity.users`,
	).Scan(&count); err != nil {
		t.Fatalf("件数取得に失敗した: %v", err)
	}
	if count != expected {
		t.Errorf("users件数 = %d, want %d", count, expected)
	}
}

func assertSessionCount(t *testing.T, expected int) {
	t.Helper()

	var count int
	if err := testPool.QueryRow(
		context.Background(), `SELECT count(*) FROM identity.auth_sessions`,
	).Scan(&count); err != nil {
		t.Fatalf("件数取得に失敗した: %v", err)
	}
	if count != expected {
		t.Errorf("auth_sessions件数 = %d, want %d", count, expected)
	}
}

// assertTokenNotStored はsession token本体がDBへ保存されていないことを確認する。
func assertTokenNotStored(t *testing.T, token string) {
	t.Helper()

	var count int
	if err := testPool.QueryRow(
		context.Background(),
		`SELECT count(*) FROM identity.auth_sessions WHERE token_hash = $1::bytea`,
		[]byte(token),
	).Scan(&count); err != nil {
		t.Fatalf("件数取得に失敗した: %v", err)
	}
	if count != 0 {
		t.Error("session token本体がDBへ保存されている")
	}
}
