package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	New(nil, webOrigin).ServeHTTP(response, request)
	return response
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

func TestCreateNoteRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "not a youtube link", body: `{"url":"https://example.com/video"}`},
		{name: "no link", body: `{}`},
		{name: "not json", body: `hello`},
		{name: "empty", body: ``},
		{name: "too big", body: `{"url":"` + strings.Repeat("a", maxBodyBytes) + `"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := send(t, core.LocalWebOrigin, http.MethodPost, "/v1/notes", "", tt.body)
			if response.Code != http.StatusBadRequest {
				t.Errorf("status = %d", response.Code)
			}
			var parsed struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &parsed); err != nil || parsed.Error == "" {
				t.Errorf("body = %s", response.Body)
			}
		})
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
			response := send(t, tt.webOrigin, http.MethodOptions, "/v1/notes", tt.origin, "")
			if response.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", response.Code, tt.wantCode)
			}
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != tt.wantAllow {
				t.Errorf("allowed origin = %q, want %q", got, tt.wantAllow)
			}
		})
	}
}

func TestOtherSitesCannotPost(t *testing.T) {
	// A plain form-style post skips the browser's preflight, so this must be refused outright.
	response := send(t, core.LocalWebOrigin, http.MethodPost, "/v1/notes", "https://evil.example", `{"url":"https://youtu.be/VjnHhmST8GY"}`)
	if response.Code != http.StatusForbidden {
		t.Errorf("status = %d", response.Code)
	}
}
