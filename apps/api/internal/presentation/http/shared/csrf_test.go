package shared_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/presentation/http/shared"
)

const testCSRFSecret = "test-csrf-secret-value"

func newTestIssuer(t *testing.T) *shared.CSRFTokenIssuer {
	t.Helper()
	issuer, err := shared.NewCSRFTokenIssuer(testCSRFSecret, false, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("NewCSRFTokenIssuer returned error: %v", err)
	}
	return issuer
}

func TestNewCSRFTokenIssuer_異常系(t *testing.T) {
	t.Parallel()

	if _, err := shared.NewCSRFTokenIssuer("short", false, time.Hour); err == nil {
		t.Error("短すぎるsecretを受け入れた")
	}
	if _, err := shared.NewCSRFTokenIssuer(testCSRFSecret, false, 0); err == nil {
		t.Error("ttl=0を受け入れた")
	}
}

func TestCSRFTokenIssuer_発行したtokenを検証できる(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)

	token, err := issuer.Issue()
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	if !issuer.Verify(token) {
		t.Error("発行したtokenの検証に失敗した")
	}
}

func TestCSRFTokenIssuer_改竄されたtokenを拒否する(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)

	token, err := issuer.Issue()
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	testCases := []struct {
		name  string
		token string
	}{
		{name: "空文字", token: ""},
		{name: "区切り文字なし", token: "nosignature"},
		{name: "署名が空", token: "abcdef."},
		{name: "nonceが空", token: ".signature"},
		{name: "署名を差し替え", token: token[:len(token)-4] + "AAAA"},
		{name: "nonceを差し替え", token: "AAAA" + token[4:]},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if issuer.Verify(testCase.token) {
				t.Errorf("不正なtokenを受け入れた: %q", testCase.token)
			}
		})
	}
}

func TestCSRFTokenIssuer_別のsecretで署名したtokenを拒否する(t *testing.T) {
	t.Parallel()

	other, err := shared.NewCSRFTokenIssuer("another-csrf-secret-value", false, time.Hour)
	if err != nil {
		t.Fatalf("NewCSRFTokenIssuer returned error: %v", err)
	}

	token, err := other.Issue()
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	if newTestIssuer(t).Verify(token) {
		t.Error("別secretで署名されたtokenを受け入れた")
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func findCookie(t *testing.T, response *http.Response, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func TestCSRFProtection_安全なmethodでtokenを発行する(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)
	handler := shared.RequestID()(shared.CSRFProtection(issuer)(okHandler()))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/auth/context", nil))

	response := recorder.Result()
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	cookie := findCookie(t, response, shared.CSRFCookieName)
	if cookie == nil {
		t.Fatal("CSRF Cookieが発行されなかった")
	}
	if cookie.HttpOnly {
		t.Error("CSRF CookieがHttpOnlyになっている (JavaScriptから読めない)")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
	}
	if !issuer.Verify(cookie.Value) {
		t.Error("発行されたCookie値の署名が不正")
	}
}

func TestCSRFProtection_有効なtokenがある場合は再発行しない(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)
	handler := shared.RequestID()(shared.CSRFProtection(issuer)(okHandler()))

	token, err := issuer.Issue()
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/auth/context", nil)
	request.AddCookie(&http.Cookie{Name: shared.CSRFCookieName, Value: token})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	response := recorder.Result()
	defer func() { _ = response.Body.Close() }()

	if cookie := findCookie(t, response, shared.CSRFCookieName); cookie != nil {
		t.Error("有効なtokenがあるのに再発行された")
	}
}

func TestCSRFProtection_state変更requestを検証する(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)
	handler := shared.RequestID()(shared.CSRFProtection(issuer)(okHandler()))

	validToken, err := issuer.Issue()
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	otherToken, err := issuer.Issue()
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	testCases := []struct {
		name           string
		cookieValue    string
		headerValue    string
		expectedStatus int
	}{
		{
			name:           "Cookieとheaderが一致すれば通す",
			cookieValue:    validToken,
			headerValue:    validToken,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Cookieが無ければ拒否する",
			cookieValue:    "",
			headerValue:    validToken,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "headerが無ければ拒否する",
			cookieValue:    validToken,
			headerValue:    "",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Cookieとheaderが異なれば拒否する",
			cookieValue:    validToken,
			headerValue:    otherToken,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "署名が不正なtokenを拒否する",
			cookieValue:    "forged.signature",
			headerValue:    "forged.signature",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
			if testCase.cookieValue != "" {
				request.AddCookie(&http.Cookie{
					Name:  shared.CSRFCookieName,
					Value: testCase.cookieValue,
				})
			}
			if testCase.headerValue != "" {
				request.Header.Set(shared.CSRFHeaderName, testCase.headerValue)
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			response := recorder.Result()
			defer func() { _ = response.Body.Close() }()

			if response.StatusCode != testCase.expectedStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, testCase.expectedStatus)
			}
			if testCase.expectedStatus != http.StatusForbidden {
				return
			}

			var body shared.ErrorResponse
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			if body.Code != "CSRF_TOKEN_INVALID" {
				t.Errorf("code = %q, want CSRF_TOKEN_INVALID", body.Code)
			}
			if body.RequestID == "" {
				t.Error("requestIdが空")
			}
		})
	}
}

func TestCSRFProtection_拒否時は後続handlerを実行しない(t *testing.T) {
	t.Parallel()

	executed := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		executed = true
		w.WriteHeader(http.StatusOK)
	})

	handler := shared.RequestID()(shared.CSRFProtection(newTestIssuer(t))(next))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/auth/login", nil))

	if executed {
		t.Error("CSRF検証に失敗したのに後続handlerが実行された")
	}
}
