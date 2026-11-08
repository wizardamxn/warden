package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"warden/internal/auth"
	"warden/internal/core"
)

// These paths stop before the database, so no queue is needed. The rest of the
// API is tested against a real Postgres in tests/integration.
func send(t *testing.T, webOrigin, method, path, origin, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response := httptest.NewRecorder()
	New(nil, auth.New(nil), core.Settings{WebOrigin: webOrigin}).ServeHTTP(response, request)
	return response
}

func errorOf(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var parsed struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("body = %s", response.Body)
	}
	return parsed.Error
}

func TestHealthz(t *testing.T) {
	response := send(t, core.LocalWebOrigin, http.MethodGet, "/healthz", "", "")
	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != `{"ok":true}` {
		t.Errorf("%d %s", response.Code, response.Body)
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Error("no request id")
	}
}

func TestRoutesNeedALogin(t *testing.T) {
	tests := []struct{ method, path string }{
		{http.MethodGet, "/v1/me"},
		{http.MethodPatch, "/v1/me"},
		{http.MethodPost, "/v1/notes"},
		{http.MethodGet, "/v1/notes/VjnHhmST8GY"},
		{http.MethodPost, "/v1/notes/VjnHhmST8GY/retry"},
		{http.MethodGet, "/v1/library"},
		{http.MethodGet, "/v1/revise"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			response := send(t, core.LocalWebOrigin, tt.method, tt.path, "", `{"url":"https://youtu.be/VjnHhmST8GY"}`)
			if response.Code != http.StatusUnauthorized || errorOf(t, response) != "Log in to continue." {
				t.Errorf("%d %s", response.Code, response.Body)
			}
		})
	}
}

func TestSignupRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "not json", body: `hello`, want: "Send an email"},
		{name: "too big", body: `{"email":"` + strings.Repeat("a", maxBodyBytes) + `"}`, want: "Send an email"},
		{name: "no email", body: `{"password":"long enough","template":"neet"}`, want: "email address"},
		{name: "not an email", body: `{"email":"student","password":"long enough","template":"neet"}`, want: "email address"},
		{name: "a name and an email", body: `{"email":"Student <a@b.com>","password":"long enough","template":"neet"}`, want: "email address"},
		{name: "no dot in the domain", body: `{"email":"a@b","password":"long enough","template":"neet"}`, want: "email address"},
		{name: "short password", body: `{"email":"a@b.com","password":"1234567","template":"neet"}`, want: "8 to 128"},
		{name: "long password", body: `{"email":"a@b.com","password":"` + strings.Repeat("a", 129) + `","template":"neet"}`, want: "8 to 128"},
		{name: "unknown template", body: `{"email":"a@b.com","password":"long enough","template":"jee"}`, want: "NEET or General"},
		{name: "no template", body: `{"email":"a@b.com","password":"long enough"}`, want: "NEET or General"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := send(t, core.LocalWebOrigin, http.MethodPost, "/v1/auth/signup", "", tt.body)
			if response.Code != http.StatusBadRequest || !strings.Contains(errorOf(t, response), tt.want) {
				t.Errorf("%d %s", response.Code, response.Body)
			}
			if response.Header().Get("Set-Cookie") != "" {
				t.Error("a failed signup must not set a cookie")
			}
		})
	}
}

func TestLogoutWithoutASessionClearsTheCookie(t *testing.T) {
	response := send(t, core.LocalWebOrigin, http.MethodPost, "/v1/auth/logout", "", "")
	cookie := response.Header().Get("Set-Cookie")
	if response.Code != http.StatusNoContent || !strings.Contains(cookie, "warden_session=;") || !strings.Contains(cookie, "Max-Age=0") {
		t.Errorf("%d %q", response.Code, cookie)
	}
}

func TestSampleWithoutOne(t *testing.T) {
	response := send(t, core.LocalWebOrigin, http.MethodGet, "/v1/sample", "", "")
	if response.Code != http.StatusNotFound || errorOf(t, response) == "" {
		t.Errorf("%d %s", response.Code, response.Body)
	}
}

func TestOrigins(t *testing.T) {
	const production = "https://warden.example"
	tests := []struct {
		name      string
		webOrigin string
		origin    string
		wantAllow string
		wantCode  int
	}{
		{name: "local website", webOrigin: core.LocalWebOrigin, origin: core.LocalWebOrigin, wantAllow: core.LocalWebOrigin, wantCode: http.StatusNoContent},
		{name: "local website on localhost", webOrigin: core.LocalWebOrigin, origin: "http://localhost:3000", wantAllow: "http://localhost:3000", wantCode: http.StatusNoContent},
		{name: "other site", webOrigin: core.LocalWebOrigin, origin: "https://evil.example", wantCode: http.StatusForbidden},
		{name: "production website", webOrigin: production, origin: production, wantAllow: production, wantCode: http.StatusNoContent},
		{name: "localhost in production", webOrigin: production, origin: "http://localhost:3000", wantCode: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := send(t, tt.webOrigin, http.MethodOptions, "/v1/me", tt.origin, "")
			if response.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", response.Code, tt.wantCode)
			}
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != tt.wantAllow {
				t.Errorf("allowed origin = %q, want %q", got, tt.wantAllow)
			}
			wantCredentials := ""
			if tt.wantAllow != "" {
				wantCredentials = "true"
			}
			if got := response.Header().Get("Access-Control-Allow-Credentials"); got != wantCredentials {
				t.Errorf("allow credentials = %q, want %q", got, wantCredentials)
			}
			if tt.wantCode == http.StatusNoContent && !strings.Contains(response.Header().Get("Access-Control-Allow-Methods"), "PATCH") {
				t.Errorf("methods = %q", response.Header().Get("Access-Control-Allow-Methods"))
			}
		})
	}
}

func TestOtherSitesCannotPost(t *testing.T) {
	// A plain form-style post skips the browser's preflight, so this must be refused outright.
	for _, path := range []string{"/v1/notes", "/v1/auth/login", "/v1/auth/signup"} {
		response := send(t, core.LocalWebOrigin, http.MethodPost, path, "https://evil.example", `{"url":"https://youtu.be/VjnHhmST8GY"}`)
		if response.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d", path, response.Code)
		}
	}
}

func TestSessionCookie(t *testing.T) {
	tests := []struct {
		host       string
		wantSecure bool
	}{
		{"127.0.0.1:8080", false},
		{"localhost:8080", false},
		{"api.warden.example", true},
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
			request.Host = tt.host
			cookie := sessionCookieFor(request, "token", 60)
			if cookie.Secure != tt.wantSecure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.Name != "warden_session" {
				t.Errorf("cookie = %+v", cookie)
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name, remote, header, want string
	}{
		{"direct", "203.0.113.5:4000", "", "203.0.113.5"},
		{"a stranger can't pick their address", "203.0.113.5:4000", "198.51.100.1", "203.0.113.5"},
		{"through the tunnel", "127.0.0.1:4000", "198.51.100.1", "198.51.100.1"},
		{"tunnel header that isn't an address", "127.0.0.1:4000", "nonsense", "127.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
			request.RemoteAddr = tt.remote
			if tt.header != "" {
				request.Header.Set("CF-Connecting-IP", tt.header)
			}
			if got := clientIP(request); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
