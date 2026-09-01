// Package authn authenticates requests to the MCP HTTP endpoint. A request
// is authorized if it carries EITHER the configured static bearer token OR a
// valid OAuth access token issued by this server's own authorization server
// (see internal/oauth). On failure it returns 401 with a WWW-Authenticate
// header pointing at the protected-resource metadata, so OAuth clients can
// discover the authorization server.
package authn

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"time"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"

	"tally/internal/oauth"
)

// Middleware wraps next with the dual-channel credential check described in
// the package doc. staticToken is the tally MCP token; oauthSrv verifies
// OAuth access tokens and supplies the resource-metadata URL.
func Middleware(staticToken string, oauthSrv *oauth.Server, next http.Handler) http.Handler {
	verifier := func(_ context.Context, token string, _ *http.Request) (*sdkauth.TokenInfo, error) {
		// Static token first: a constant-time compare, no allocation.
		if subtle.ConstantTimeCompare([]byte(token), []byte(staticToken)) == 1 {
			return &sdkauth.TokenInfo{}, nil
		}
		// Otherwise it must be an access token this server issued.
		expiresAt, err := oauthSrv.VerifyAccessToken(token, time.Now())
		if err != nil {
			return nil, fmt.Errorf("%w: %v", sdkauth.ErrInvalidToken, err)
		}
		return &sdkauth.TokenInfo{Expiration: expiresAt}, nil
	}

	opts := &sdkauth.RequireBearerTokenOptions{
		ResourceMetadataURL: oauthSrv.ProtectedResourceMetadataURL(),
		// The static-token branch returns no Expiration; the OAuth branch
		// verifies expiry itself before returning. Either way the verifier
		// is authoritative, so opt out of the middleware's own exp check.
		AllowMissingExpiration: true,
	}

	return sdkauth.RequireBearerToken(verifier, opts)(next)
}
