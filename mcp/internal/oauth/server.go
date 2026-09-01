package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// Config configures a Server.
type Config struct {
	// SigningSecret signs every credential (see token.go).
	SigningSecret string
	// BaseURL is the externally reachable origin, no trailing slash
	// (e.g. https://tally.liuchao.life).
	BaseURL string
	// StaticToken is the tally MCP token; it is the single login gate for
	// the /authorize step and is compared in constant time.
	StaticToken string
}

// Server is tally-mcp's embedded OAuth 2.1 authorization server.
type Server struct {
	secret  string
	baseURL string
	static  string
}

// New builds a Server. All Config fields are required.
func New(cfg Config) *Server {
	return &Server{
		secret:  cfg.SigningSecret,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		static:  cfg.StaticToken,
	}
}

// ResourceURI is the canonical URI of the MCP server (RFC 8707 / RFC 9728),
// which access tokens are bound to as their audience.
func (s *Server) ResourceURI() string { return s.baseURL + "/mcp" }

// ProtectedResourceMetadataURL is where the resource-server middleware points
// clients via WWW-Authenticate on a 401.
func (s *Server) ProtectedResourceMetadataURL() string {
	return s.baseURL + "/.well-known/oauth-protected-resource"
}

// Routes registers every OAuth and well-known endpoint on mux.
func (s *Server) Routes(mux *http.ServeMux) {
	mux.Handle("/.well-known/oauth-protected-resource",
		sdkauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
			Resource:               s.ResourceURI(),
			AuthorizationServers:   []string{s.baseURL},
			BearerMethodsSupported: []string{"header"},
			ResourceName:           "tally-mcp",
		}))
	mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleAuthServerMetadata)
	mux.HandleFunc("/authorize", s.handleAuthorize)
	mux.HandleFunc("/token", s.handleToken)
	mux.HandleFunc("/register", s.handleRegister)
}

// VerifyAccessToken checks token against this server's signing secret and
// resource URI. Exposed so the resource-server middleware can share it.
func (s *Server) VerifyAccessToken(token string, now time.Time) (expiresAt time.Time, err error) {
	return VerifyAccessToken(s.secret, token, s.ResourceURI(), now)
}

// StaticTokenMatches reports whether token equals the configured MCP token,
// in constant time.
func (s *Server) StaticTokenMatches(token string) bool {
	return subtle.ConstantTimeCompare([]byte(token), []byte(s.static)) == 1
}

// --- authorization-server metadata (RFC 8414) ---

// authServerMetadata is our own shape rather than oauthex.AuthServerMeta so
// we can omit jwks_uri (we sign with HMAC, there is no key set to publish).
type authServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

func (s *Server) handleAuthServerMetadata(w http.ResponseWriter, r *http.Request) {
	logReq(r, nil)
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, authServerMetadata{
		Issuer:                            s.baseURL,
		AuthorizationEndpoint:             s.baseURL + "/authorize",
		TokenEndpoint:                     s.baseURL + "/token",
		RegistrationEndpoint:              s.baseURL + "/register",
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
	})
}

// --- /authorize ---

// authorizeParams holds the validated query/form parameters common to the
// GET (render form) and POST (submit login) halves of /authorize.
type authorizeParams struct {
	ClientID      string
	RedirectURI   string
	State         string
	CodeChallenge string
	Resource      string
	Scope         string
}

func (s *Server) parseAuthorizeParams(values url.Values) (*authorizeParams, string) {
	if values.Get("response_type") != "code" {
		return nil, "response_type must be \"code\""
	}
	if values.Get("code_challenge_method") != "S256" {
		return nil, "code_challenge_method must be \"S256\""
	}
	cc := values.Get("code_challenge")
	if cc == "" {
		return nil, "code_challenge is required"
	}
	clientID := values.Get("client_id")
	redirectURI := values.Get("redirect_uri")
	client, err := verifyClientID(s.secret, clientID)
	if err != nil {
		return nil, "unknown client_id; register via the registration endpoint first"
	}
	if !containsExact(client.RedirectURIs, redirectURI) {
		return nil, "redirect_uri does not match a registered value for this client"
	}
	res := values.Get("resource")
	if res != "" && res != s.ResourceURI() {
		return nil, "resource does not identify this server"
	}
	return &authorizeParams{
		ClientID:      clientID,
		RedirectURI:   redirectURI,
		State:         values.Get("state"),
		CodeChallenge: cc,
		Resource:      res,
		Scope:         values.Get("scope"),
	}, ""
}

func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		logReq(r, nil)
		p, msg := s.parseAuthorizeParams(r.URL.Query())
		if msg != "" {
			http.Error(w, "invalid authorization request: "+msg, http.StatusBadRequest)
			return
		}
		s.renderLogin(w, p, "")
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "malformed form", http.StatusBadRequest)
			return
		}
		logReq(r, map[string]string{"has_tally_token": boolStr(r.PostForm.Get("tally_token") != "")})
		p, msg := s.parseAuthorizeParams(r.PostForm)
		if msg != "" {
			http.Error(w, "invalid authorization request: "+msg, http.StatusBadRequest)
			return
		}
		if !s.StaticTokenMatches(r.PostForm.Get("tally_token")) {
			w.WriteHeader(http.StatusUnauthorized)
			s.renderLogin(w, p, "That token is not correct.")
			return
		}
		code := issueAuthCode(s.secret, p.CodeChallenge, p.RedirectURI, s.ResourceURI())
		redirect, err := url.Parse(p.RedirectURI)
		if err != nil {
			http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
			return
		}
		q := redirect.Query()
		q.Set("code", code)
		if p.State != "" {
			q.Set("state", p.State)
		}
		redirect.RawQuery = q.Encode()
		http.Redirect(w, r, redirect.String(), http.StatusFound)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

var loginTmpl = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="zh">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>tally · 授权</title>
<style>
 body{font-family:system-ui,-apple-system,"PingFang SC",sans-serif;max-width:26rem;margin:12vh auto;padding:0 1.25rem;color:#1a1a1a;background:#fafafa}
 h1{font-size:1.15rem;margin:0 0 .25rem}
 p{color:#555;font-size:.9rem;line-height:1.5}
 input[type=password]{width:100%;box-sizing:border-box;padding:.6rem .7rem;font-size:1rem;border:1px solid #ccc;border-radius:6px}
 button{margin-top:.9rem;width:100%;padding:.6rem;font-size:1rem;border:0;border-radius:6px;background:#1a1a1a;color:#fff;cursor:pointer}
 .err{color:#b00020;font-size:.85rem;margin:.5rem 0 0}
 @media(prefers-color-scheme:dark){body{background:#141414;color:#eee}p{color:#aaa}input[type=password]{background:#222;border-color:#444;color:#eee}button{background:#eee;color:#141414}}
</style>
</head>
<body>
<h1>授权 tally 连接器</h1>
<p>粘贴你的 tally token（与 <code>TALLY_MCP_TOKEN</code> 相同的值）以授权这次连接。</p>
{{if .Error}}<p class="err">{{.Error}}</p>{{end}}
<form method="post" action="/authorize">
 <input type="hidden" name="response_type" value="code">
 <input type="hidden" name="code_challenge_method" value="S256">
 <input type="hidden" name="client_id" value="{{.P.ClientID}}">
 <input type="hidden" name="redirect_uri" value="{{.P.RedirectURI}}">
 <input type="hidden" name="state" value="{{.P.State}}">
 <input type="hidden" name="code_challenge" value="{{.P.CodeChallenge}}">
 <input type="hidden" name="resource" value="{{.P.Resource}}">
 <input type="hidden" name="scope" value="{{.P.Scope}}">
 <input type="password" name="tally_token" placeholder="tally token" autofocus autocomplete="off" required>
 <button type="submit">授权</button>
</form>
</body>
</html>`))

func (s *Server) renderLogin(w http.ResponseWriter, p *authorizeParams, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = loginTmpl.Execute(w, struct {
		P     *authorizeParams
		Error string
	}{P: p, Error: errMsg})
}

// --- /token ---

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type oauthError struct {
	ErrorCode   string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, oauthError{ErrorCode: "invalid_request", Description: "malformed form"})
		return
	}
	logReq(r, map[string]string{
		"grant_type":        r.PostForm.Get("grant_type"),
		"has_code":          boolStr(r.PostForm.Get("code") != ""),
		"has_code_verifier": boolStr(r.PostForm.Get("code_verifier") != ""),
	})

	if r.PostForm.Get("grant_type") != "authorization_code" {
		writeJSON(w, http.StatusBadRequest, oauthError{ErrorCode: "unsupported_grant_type"})
		return
	}
	code := r.PostForm.Get("code")
	verifier := r.PostForm.Get("code_verifier")
	redirectURI := r.PostForm.Get("redirect_uri")
	resource := r.PostForm.Get("resource")

	payload, err := verifyAuthCode(s.secret, code, time.Now())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, oauthError{ErrorCode: "invalid_grant", Description: err.Error()})
		return
	}
	if payload.RedirectURI != redirectURI {
		writeJSON(w, http.StatusBadRequest, oauthError{ErrorCode: "invalid_grant", Description: "redirect_uri mismatch"})
		return
	}
	if resource != "" && payload.Resource != resource {
		writeJSON(w, http.StatusBadRequest, oauthError{ErrorCode: "invalid_grant", Description: "resource mismatch"})
		return
	}
	if !pkceMatches(verifier, payload.CodeChallenge) {
		writeJSON(w, http.StatusBadRequest, oauthError{ErrorCode: "invalid_grant", Description: "PKCE verification failed"})
		return
	}

	token, expiresIn := issueAccessToken(s.secret, s.ResourceURI(), newJTI())
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
	})
}

// pkceMatches reports whether base64url(sha256(verifier)) == challenge
// (PKCE "S256"), comparing in constant time.
func pkceMatches(verifier, challenge string) bool {
	if verifier == "" || challenge == "" {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(want), []byte(challenge)) == 1
}

// --- /register (RFC 7591, stateless) ---

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var meta oauthex.ClientRegistrationMetadata
	if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
		writeJSON(w, http.StatusBadRequest, oauthex.ClientRegistrationError{
			ErrorCode: "invalid_client_metadata", ErrorDescription: "body is not valid JSON",
		})
		return
	}
	logReq(r, map[string]string{"redirect_uris": strings.Join(meta.RedirectURIs, " ")})

	if len(meta.RedirectURIs) == 0 {
		writeJSON(w, http.StatusBadRequest, oauthex.ClientRegistrationError{
			ErrorCode: "invalid_redirect_uri", ErrorDescription: "at least one redirect_uri is required",
		})
		return
	}
	for _, u := range meta.RedirectURIs {
		if !validRedirectURI(u) {
			writeJSON(w, http.StatusBadRequest, oauthex.ClientRegistrationError{
				ErrorCode:        "invalid_redirect_uri",
				ErrorDescription: fmt.Sprintf("redirect_uri %q must be https or http://localhost and contain no fragment", u),
			})
			return
		}
	}

	clientID := issueClientID(s.secret, meta.RedirectURIs)
	resp := &oauthex.ClientRegistrationResponse{
		ClientRegistrationMetadata: meta,
		ClientID:                   clientID,
		ClientIDIssuedAt:           time.Now(),
	}
	resp.TokenEndpointAuthMethod = "none"
	writeJSON(w, http.StatusCreated, resp)
}

// validRedirectURI enforces the OAuth 2.1 rule: HTTPS everywhere, except
// loopback may be http. No fragment component.
func validRedirectURI(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Fragment != "" || strings.Contains(raw, "#") {
		return false
	}
	switch u.Scheme {
	case "https":
		return u.Host != ""
	case "http":
		host := u.Hostname()
		return host == "localhost" || host == "127.0.0.1" || host == "::1"
	default:
		return false
	}
}

// --- helpers ---

func containsExact(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func newJTI() string {
	var b [16]byte
	// A zero jti still yields a valid (if less unique) token, and crypto/rand
	// does not fail on any platform tally runs on, so the error is ignored.
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

// logReq emits a one-line record of an OAuth endpoint hit for the first-run
// verification against claude.ai (tasks 1.1/1.2). It never logs token values.
func logReq(r *http.Request, extra map[string]string) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "oauth %s %s", r.Method, r.URL.Path)
	if q := r.URL.RawQuery; q != "" {
		if vals, err := url.ParseQuery(q); err == nil {
			keys := make([]string, 0, len(vals))
			for k := range vals {
				keys = append(keys, k)
			}
			fmt.Fprintf(&sb, " query=[%s]", strings.Join(keys, ","))
		}
	}
	for k, v := range extra {
		fmt.Fprintf(&sb, " %s=%s", k, v)
	}
	log.Print(sb.String())
}
