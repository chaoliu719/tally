// Package authn implements bearer token authentication for the MCP HTTP endpoint.
package authn

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

const bearerPrefix = "Bearer "

// Middleware wraps next with a check that the request carries an
// Authorization: Bearer <token> header matching token, comparing it in
// constant time to avoid leaking the token via a timing side channel. On
// failure it writes a 401 response and never calls next, so no tool logic
// downstream is reached.
func Middleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAuthorized(r, token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isAuthorized(r *http.Request, token string) bool {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, bearerPrefix) {
		return false
	}

	provided := strings.TrimPrefix(header, bearerPrefix)

	return subtle.ConstantTimeCompare([]byte(provided), []byte(token)) == 1
}
