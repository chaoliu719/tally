// Package oauth implements tally-mcp's minimal, stateless OAuth 2.1
// authorization server: authorization-server metadata, an authorization-code
// + PKCE flow gated by the static MCP token, token issuance, and stateless
// dynamic client registration. It exists so tally-mcp can be added as a
// claude.ai custom connector; see
// openspec/changes/add-mcp-oauth-authorization/.
//
// Every credential this package mints — authorization codes, access tokens,
// client IDs — is a stateless HMAC-signed value in the same two-segment form
// as internal/confirm: base64url(JSON payload) + "." + base64url(HMAC-SHA256).
// There is no token store and no per-token revocation; rotating the signing
// secret invalidates everything at once.
package oauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

const (
	// authCodeTTL bounds how long an issued authorization code can be
	// exchanged at the token endpoint. Short, since PKCE already blocks
	// replay and the code is single-use in practice.
	authCodeTTL = 5 * time.Minute
	// accessTokenTTL is the lifetime of an issued access token. When it
	// elapses the client re-runs the authorization flow (no refresh token
	// in v1; see design D6).
	accessTokenTTL = time.Hour

	typAuthCode    = "code"
	typAccessToken = "at"
	typClientID    = "cid"
)

// authCodePayload is what an authorization code carries. It binds the code to
// the PKCE challenge, the exact redirect URI, and the resource, so the token
// endpoint can verify all three without any stored state.
type authCodePayload struct {
	Typ           string `json:"typ"`
	CodeChallenge string `json:"cc"`
	RedirectURI   string `json:"ru"`
	Resource      string `json:"res"`
	ExpiresAt     int64  `json:"exp"`
}

// accessTokenPayload is what an access token carries. Aud is the canonical
// resource URI of this MCP server; the resource-server middleware rejects any
// token whose Aud is not itself.
type accessTokenPayload struct {
	Typ       string `json:"typ"`
	Audience  string `json:"aud"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	JTI       string `json:"jti"`
}

// clientIDPayload is what a registered client_id encodes: the redirect URIs
// submitted at registration. /authorize verifies a request's redirect_uri
// against this list without consulting any registration store.
type clientIDPayload struct {
	Typ          string   `json:"typ"`
	RedirectURIs []string `json:"ru"`
	IssuedAt     int64    `json:"iat"`
}

// issueAuthCode signs an authorization code valid for authCodeTTL from now.
func issueAuthCode(secret, codeChallenge, redirectURI, resource string) string {
	return encode(secret, authCodePayload{
		Typ:           typAuthCode,
		CodeChallenge: codeChallenge,
		RedirectURI:   redirectURI,
		Resource:      resource,
		ExpiresAt:     time.Now().Add(authCodeTTL).Unix(),
	})
}

// verifyAuthCode checks an authorization code's signature, type, and
// expiry, and returns its bound fields. The caller is responsible for
// checking that redirectURI and resource match the token request, and that
// the PKCE verifier hashes to CodeChallenge.
func verifyAuthCode(secret, code string, now time.Time) (*authCodePayload, error) {
	var p authCodePayload
	if err := decode(secret, code, &p); err != nil {
		return nil, err
	}
	if p.Typ != typAuthCode {
		return nil, fmt.Errorf("token is not an authorization code")
	}
	if now.Unix() > p.ExpiresAt {
		return nil, fmt.Errorf("authorization code has expired")
	}
	return &p, nil
}

// issueAccessToken signs an access token bound to audience, valid for
// accessTokenTTL. jti is an opaque unique id (caller supplies randomness).
func issueAccessToken(secret, audience, jti string) (token string, expiresIn int) {
	now := time.Now()
	exp := now.Add(accessTokenTTL)
	return encode(secret, accessTokenPayload{
		Typ:       typAccessToken,
		Audience:  audience,
		IssuedAt:  now.Unix(),
		ExpiresAt: exp.Unix(),
		JTI:       jti,
	}), int(accessTokenTTL.Seconds())
}

// IssueAccessToken mints a signed access token bound to audience, valid for
// accessTokenTTL. It is symmetric with VerifyAccessToken; production
// issuance flows through the /token endpoint, but the resource-server
// package and tests mint tokens directly.
func IssueAccessToken(secret, audience string) string {
	tok, _ := issueAccessToken(secret, audience, newJTI())
	return tok
}

// VerifyAccessToken checks an access token's signature, type, audience, and
// expiry. wantAudience is this server's canonical resource URI. It returns
// the token's expiry time on success.
func VerifyAccessToken(secret, token, wantAudience string, now time.Time) (expiresAt time.Time, err error) {
	var p accessTokenPayload
	if err := decode(secret, token, &p); err != nil {
		return time.Time{}, err
	}
	if p.Typ != typAccessToken {
		return time.Time{}, fmt.Errorf("token is not an access token")
	}
	if p.Audience != wantAudience {
		return time.Time{}, fmt.Errorf("access token audience does not match this server")
	}
	if now.Unix() > p.ExpiresAt {
		return time.Time{}, fmt.Errorf("access token has expired")
	}
	return time.Unix(p.ExpiresAt, 0), nil
}

// issueClientID signs a client_id encoding the given redirect URIs.
func issueClientID(secret string, redirectURIs []string) string {
	return encode(secret, clientIDPayload{
		Typ:          typClientID,
		RedirectURIs: append([]string(nil), redirectURIs...),
		IssuedAt:     time.Now().Unix(),
	})
}

// verifyClientID checks a client_id's signature and type and returns the
// redirect URIs it was registered with. client_ids do not expire.
func verifyClientID(secret, clientID string) (*clientIDPayload, error) {
	var p clientIDPayload
	if err := decode(secret, clientID, &p); err != nil {
		return nil, err
	}
	if p.Typ != typClientID {
		return nil, fmt.Errorf("client_id is not a registered client")
	}
	return &p, nil
}

// encode marshals v and returns base64url(json) + "." + base64url(hmac).
func encode(secret string, v any) string {
	body, _ := json.Marshal(v)
	sig := sign(secret, body)
	return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// decode verifies the HMAC over a two-segment token and unmarshals the
// payload into v. It returns a descriptive error if the token is malformed
// or the signature does not match.
func decode(secret, token string, v any) error {
	bodyB64, sigB64, ok := splitToken(token)
	if !ok {
		return fmt.Errorf("malformed token")
	}
	body, err := base64.RawURLEncoding.DecodeString(bodyB64)
	if err != nil {
		return fmt.Errorf("malformed token: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("malformed token: %w", err)
	}
	if !hmac.Equal(sig, sign(secret, body)) {
		return fmt.Errorf("token signature is invalid")
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("malformed token: %w", err)
	}
	return nil
}

func splitToken(token string) (bodyB64, sigB64 string, ok bool) {
	for i := len(token) - 1; i >= 0; i-- {
		if token[i] == '.' {
			return token[:i], token[i+1:], true
		}
	}
	return "", "", false
}

func sign(secret string, body []byte) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return mac.Sum(nil)
}
