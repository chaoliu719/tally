package authn_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tally/internal/authn"
	"tally/internal/oauth"
)

const (
	staticToken = "correct-static-token"
	baseURL     = "https://tally.test"
	oauthSecret = "oauth-signing-secret"
)

func newMiddleware(t *testing.T, next http.Handler) http.Handler {
	t.Helper()
	srv := oauth.New(oauth.Config{
		SigningSecret: oauthSecret,
		BaseURL:       baseURL,
		StaticToken:   staticToken,
	})
	return authn.Middleware(staticToken, srv, next)
}

func TestMiddlewareStaticToken(t *testing.T) {
	cases := []struct {
		name       string
		header     string
		wantStatus int
		wantCalled bool
	}{
		{"missing header", "", http.StatusUnauthorized, false},
		{"wrong token", "Bearer wrong-token", http.StatusUnauthorized, false},
		{"correct static token", "Bearer " + staticToken, http.StatusOK, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			handler := newMiddleware(t, next)

			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if called != tc.wantCalled {
				t.Errorf("next called = %v, want %v", called, tc.wantCalled)
			}
		})
	}
}

// TestMiddlewareOAuthAccessToken checks the second channel: a valid access
// token this server's OAuth AS issued is accepted; a garbage token, a token
// signed with the wrong secret, and a token for another audience are not.
func TestMiddlewareOAuthAccessToken(t *testing.T) {
	good := oauth.IssueAccessToken(oauthSecret, baseURL+"/mcp")
	wrongSecret := oauth.IssueAccessToken("some-other-secret", baseURL+"/mcp")
	wrongAudience := oauth.IssueAccessToken(oauthSecret, "https://elsewhere.test/mcp")

	cases := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{"valid access token", good, http.StatusOK},
		{"token signed with wrong secret", wrongSecret, http.StatusUnauthorized},
		{"token for another audience", wrongAudience, http.StatusUnauthorized},
		{"not a token at all", "just-some-string", http.StatusUnauthorized},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})
			handler := newMiddleware(t, next)

			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if (rec.Code == http.StatusOK) != called {
				t.Errorf("next called = %v, but status = %d", called, rec.Code)
			}
		})
	}
}

// TestMiddlewareUnauthorizedCarriesWWWAuthenticate verifies a 401 points the
// client at the protected-resource metadata document (RFC 9728), and that no
// error message leaks the static token or the signing secret.
func TestMiddlewareUnauthorizedCarriesWWWAuthenticate(t *testing.T) {
	handler := newMiddleware(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer nope")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	wa := rec.Header().Get("WWW-Authenticate")
	if !strings.Contains(wa, "resource_metadata=") ||
		!strings.Contains(wa, baseURL+"/.well-known/oauth-protected-resource") {
		t.Errorf("WWW-Authenticate = %q, want it to carry the protected-resource metadata URL", wa)
	}
	if body := rec.Body.String(); strings.Contains(body, staticToken) || strings.Contains(body, oauthSecret) {
		t.Errorf("401 body leaks a secret: %q", body)
	}
}
