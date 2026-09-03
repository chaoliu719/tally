package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const (
	srvSecret  = "srv-signing-secret"
	srvBaseURL = "https://tally.test"
	srvToken   = "the-static-mcp-token"
	clientCB   = "https://claude.ai/api/mcp/auth_callback"
)

func newTestServer() (*Server, *http.ServeMux) {
	s := New(Config{SigningSecret: srvSecret, BaseURL: srvBaseURL, StaticToken: srvToken})
	mux := http.NewServeMux()
	s.Routes(mux)
	return s, mux
}

func pkcePair() (verifier, challenge string) {
	verifier = "a-sufficiently-long-random-pkce-code-verifier-string-1234567890"
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

func do(mux *http.ServeMux, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestProtectedResourceMetadata(t *testing.T) {
	_, mux := newTestServer()
	rec := do(mux, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (must be unauthenticated)", rec.Code)
	}
	var meta struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if meta.Resource != srvBaseURL+"/mcp" {
		t.Errorf("resource = %q, want %q", meta.Resource, srvBaseURL+"/mcp")
	}
	if len(meta.AuthorizationServers) != 1 || meta.AuthorizationServers[0] != srvBaseURL {
		t.Errorf("authorization_servers = %v, want [%q]", meta.AuthorizationServers, srvBaseURL)
	}
}

func TestAuthServerMetadata(t *testing.T) {
	_, mux := newTestServer()
	rec := do(mux, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var m authServerMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if m.Issuer != srvBaseURL {
		t.Errorf("issuer = %q, want %q", m.Issuer, srvBaseURL)
	}
	if m.AuthorizationEndpoint != srvBaseURL+"/authorize" || m.TokenEndpoint != srvBaseURL+"/token" || m.RegistrationEndpoint != srvBaseURL+"/register" {
		t.Errorf("endpoints wrong: %+v", m)
	}
	if len(m.CodeChallengeMethodsSupported) != 1 || m.CodeChallengeMethodsSupported[0] != "S256" {
		t.Errorf("code_challenge_methods_supported = %v, want [S256]", m.CodeChallengeMethodsSupported)
	}
	if !strings.Contains(rec.Body.String(), "\"response_types_supported\"") {
		t.Error("metadata is missing required response_types_supported")
	}
	gt := strings.Join(m.GrantTypesSupported, ",")
	if !strings.Contains(gt, "authorization_code") || !strings.Contains(gt, "refresh_token") {
		t.Errorf("grant_types_supported = %v, want both authorization_code and refresh_token", m.GrantTypesSupported)
	}
	if strings.Contains(rec.Body.String(), "jwks_uri") {
		t.Error("metadata should not advertise jwks_uri (HMAC signing, no key set)")
	}
}

// registerClient drives POST /register and returns the issued client_id.
func registerClient(t *testing.T, mux *http.ServeMux, redirectURIs ...string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"redirect_uris": redirectURIs})
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := do(mux, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("/register status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("/register body not JSON: %v", err)
	}
	if resp.ClientID == "" {
		t.Fatal("/register returned an empty client_id")
	}
	if resp.ClientSecret != "" {
		t.Fatalf("/register issued a client_secret %q; public clients get none", resp.ClientSecret)
	}
	return resp.ClientID
}

func TestRegisterRejectsBadRedirectURI(t *testing.T) {
	_, mux := newTestServer()
	for _, bad := range []string{"http://evil.example/cb", "https://c.example/cb#frag", "ftp://x/y"} {
		body, _ := json.Marshal(map[string]any{"redirect_uris": []string{bad}})
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := do(mux, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("/register with %q: status = %d, want 400", bad, rec.Code)
		}
	}
}

func TestRegisterAcceptsLocalhostHTTP(t *testing.T) {
	_, mux := newTestServer()
	registerClient(t, mux, "http://localhost:8080/cb")
}

func authorizeQuery(clientID, redirectURI, challenge, state string) url.Values {
	return url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
		"resource":              {srvBaseURL + "/mcp"},
	}
}

func TestAuthorizeGETValidRendersForm(t *testing.T) {
	_, mux := newTestServer()
	cid := registerClient(t, mux, clientCB)
	_, challenge := pkcePair()

	req := httptest.NewRequest(http.MethodGet, "/authorize?"+authorizeQuery(cid, clientCB, challenge, "xyz").Encode(), nil)
	rec := do(mux, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), `name="tally_token"`) {
		t.Error("login form is missing the tally_token field")
	}
}

func TestAuthorizeGETRejectsMissingPKCE(t *testing.T) {
	_, mux := newTestServer()
	cid := registerClient(t, mux, clientCB)

	q := authorizeQuery(cid, clientCB, "", "xyz")
	q.Del("code_challenge")
	rec := do(mux, httptest.NewRequest(http.MethodGet, "/authorize?"+q.Encode(), nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for missing code_challenge", rec.Code)
	}
}

func TestAuthorizeGETRejectsUnregisteredRedirectURI(t *testing.T) {
	_, mux := newTestServer()
	cid := registerClient(t, mux, clientCB)
	_, challenge := pkcePair()

	rec := do(mux, httptest.NewRequest(http.MethodGet,
		"/authorize?"+authorizeQuery(cid, "https://attacker.example/cb", challenge, "xyz").Encode(), nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a redirect_uri not registered to this client", rec.Code)
	}
}

func TestAuthorizePOSTWrongTokenReRendersForm(t *testing.T) {
	_, mux := newTestServer()
	cid := registerClient(t, mux, clientCB)
	_, challenge := pkcePair()

	form := authorizeQuery(cid, clientCB, challenge, "xyz")
	form.Set("tally_token", "not-the-token")
	req := httptest.NewRequest(http.MethodPost, "/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := do(mux, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if rec.Header().Get("Location") != "" {
		t.Error("a failed login must not redirect to the client")
	}
	if !strings.Contains(rec.Body.String(), `name="tally_token"`) {
		t.Error("expected the login form to be re-rendered")
	}
}

// fullFlow runs register -> authorize(POST, correct token) -> token, and
// returns the issued access and refresh tokens.
func fullFlow(t *testing.T, mux *http.ServeMux) (accessToken, refreshToken string) {
	t.Helper()
	cid := registerClient(t, mux, clientCB)
	verifier, challenge := pkcePair()

	form := authorizeQuery(cid, clientCB, challenge, "the-state")
	form.Set("tally_token", srvToken)
	areq := httptest.NewRequest(http.MethodPost, "/authorize", strings.NewReader(form.Encode()))
	areq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	arec := do(mux, areq)

	if arec.Code != http.StatusFound {
		t.Fatalf("/authorize status = %d, want 302; body = %s", arec.Code, arec.Body.String())
	}
	loc, err := url.Parse(arec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Location header not a URL: %v", err)
	}
	if loc.Query().Get("state") != "the-state" {
		t.Errorf("state = %q, want it echoed back unchanged", loc.Query().Get("state"))
	}
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatal("redirect carried no code")
	}

	tform := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {clientCB},
		"client_id":     {cid},
		"resource":      {srvBaseURL + "/mcp"},
	}
	treq := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(tform.Encode()))
	treq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	trec := do(mux, treq)
	if trec.Code != http.StatusOK {
		t.Fatalf("/token status = %d, want 200; body = %s", trec.Code, trec.Body.String())
	}
	var tok struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(trec.Body.Bytes(), &tok); err != nil {
		t.Fatalf("/token body not JSON: %v", err)
	}
	if tok.TokenType != "Bearer" || tok.AccessToken == "" || tok.ExpiresIn <= 0 {
		t.Fatalf("/token response malformed: %+v", tok)
	}
	if tok.RefreshToken == "" {
		t.Fatal("/token response carried no refresh_token")
	}
	return tok.AccessToken, tok.RefreshToken
}

// refreshGrant drives POST /token with grant_type=refresh_token.
func refreshGrant(mux *http.ServeMux, refreshToken string) *httptest.ResponseRecorder {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return do(mux, req)
}

func TestFullAuthorizationCodeFlow(t *testing.T) {
	s, mux := newTestServer()
	at, _ := fullFlow(t, mux)

	if _, err := s.VerifyAccessToken(at, time.Now()); err != nil {
		t.Fatalf("the access token from a full flow does not verify: %v", err)
	}
}

func TestRefreshTokenGrantRotates(t *testing.T) {
	s, mux := newTestServer()
	_, rt := fullFlow(t, mux)

	rec := refreshGrant(mux, rt)
	if rec.Code != http.StatusOK {
		t.Fatalf("/token refresh: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var tok struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tok); err != nil {
		t.Fatalf("refresh body not JSON: %v", err)
	}
	if tok.TokenType != "Bearer" || tok.AccessToken == "" {
		t.Fatalf("refresh response malformed: %+v", tok)
	}
	if tok.RefreshToken == "" || tok.RefreshToken == rt {
		t.Fatalf("refresh must rotate the refresh token; got %q (old %q)", tok.RefreshToken, rt)
	}
	if _, err := s.VerifyAccessToken(tok.AccessToken, time.Now()); err != nil {
		t.Fatalf("access token from a refresh does not verify: %v", err)
	}
	// The rotated refresh token works for a further refresh.
	if rec2 := refreshGrant(mux, tok.RefreshToken); rec2.Code != http.StatusOK {
		t.Fatalf("second refresh: status = %d, body = %s", rec2.Code, rec2.Body.String())
	}
}

func TestRefreshTokenGrantRejections(t *testing.T) {
	_, mux := newTestServer()

	// Signed for another server.
	otherRT, _ := issueRefreshToken(srvSecret, "https://someone-else.test/mcp", "j")
	// An access token presented as a refresh token.
	accessAsRefresh := IssueAccessToken(srvSecret, srvBaseURL+"/mcp")
	// Signed with a different secret.
	wrongSecretRT, _ := issueRefreshToken("some-other-secret", srvBaseURL+"/mcp", "j")

	for name, rt := range map[string]string{
		"empty":        "",
		"garbage":      "not-a-token",
		"wrong-aud":    otherRT,
		"access-token": accessAsRefresh,
		"wrong-secret": wrongSecretRT,
	} {
		rec := refreshGrant(mux, rt)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "invalid_grant") {
			t.Errorf("%s: body = %s, want invalid_grant", name, rec.Body.String())
		}
	}
}

func TestTokenRejectsUnknownGrantType(t *testing.T) {
	_, mux := newTestServer()
	form := url.Values{"grant_type": {"client_credentials"}}
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := do(mux, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unsupported_grant_type") {
		t.Fatalf("status = %d body = %s, want 400 unsupported_grant_type", rec.Code, rec.Body.String())
	}
}

func TestTokenRejectsBadPKCEVerifier(t *testing.T) {
	_, mux := newTestServer()
	cid := registerClient(t, mux, clientCB)
	_, challenge := pkcePair()

	form := authorizeQuery(cid, clientCB, challenge, "s")
	form.Set("tally_token", srvToken)
	areq := httptest.NewRequest(http.MethodPost, "/authorize", strings.NewReader(form.Encode()))
	areq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	arec := do(mux, areq)
	loc, _ := url.Parse(arec.Header().Get("Location"))
	code := loc.Query().Get("code")

	tform := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {"the-WRONG-verifier"},
		"redirect_uri":  {clientCB},
	}
	treq := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(tform.Encode()))
	treq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	trec := do(mux, treq)

	if trec.Code != http.StatusBadRequest {
		t.Fatalf("/token with a bad verifier: status = %d, want 400", trec.Code)
	}
	if !strings.Contains(trec.Body.String(), "invalid_grant") {
		t.Errorf("expected an invalid_grant error, got %s", trec.Body.String())
	}
}

func TestTokenRejectsRedirectURIMismatch(t *testing.T) {
	_, mux := newTestServer()
	cid := registerClient(t, mux, clientCB, "https://claude.ai/other/cb")
	verifier, challenge := pkcePair()

	form := authorizeQuery(cid, clientCB, challenge, "s")
	form.Set("tally_token", srvToken)
	areq := httptest.NewRequest(http.MethodPost, "/authorize", strings.NewReader(form.Encode()))
	areq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	arec := do(mux, areq)
	loc, _ := url.Parse(arec.Header().Get("Location"))
	code := loc.Query().Get("code")

	tform := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {"https://claude.ai/other/cb"}, // registered, but not the one the code was issued for
	}
	treq := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(tform.Encode()))
	treq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	trec := do(mux, treq)
	if trec.Code != http.StatusBadRequest {
		t.Fatalf("/token with a mismatched redirect_uri: status = %d, want 400", trec.Code)
	}
}
