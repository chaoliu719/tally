// Package confirm implements the stateless, HMAC-signed preview -> apply
// confirmation tokens used by destructive write operations (see
// openspec/changes/account-category-lifecycle/design.md). A token is
// base64url(JSON payload) + "." + base64url(HMAC-SHA256(payload, secret)) --
// two segments, no JWT header/algorithm negotiation, since the algorithm is
// fixed and there's no other consumer to negotiate with.
package confirm

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// TTL is how long a confirmation token remains valid after it's issued.
const TTL = 15 * time.Minute

type payload struct {
	Action    string `json:"action"`
	ID        string `json:"id"`
	Revision  string `json:"revision"`
	ExpiresAt int64  `json:"expires_at"`
}

// Issue signs a new confirmation token binding action, id, and revision,
// valid for TTL from now. It returns the token and its expiry (unix
// seconds).
func Issue(secret, action, id, revision string) (token string, expiresAt int64) {
	expiresAt = time.Now().Add(TTL).Unix()

	p := payload{
		Action:    action,
		ID:        id,
		Revision:  revision,
		ExpiresAt: expiresAt,
	}

	// Marshaling a fixed struct with no user-controlled map ordering never
	// fails, so the error is deliberately ignored here.
	body, _ := json.Marshal(p)
	sig := sign(secret, body)

	return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(sig), expiresAt
}

// Verify checks a confirmation token against the action, id, and current
// revision the caller expects, at time now. It checks, in order: the
// signature, that the token was issued for wantAction and wantID, that it
// hasn't expired, and that its revision still matches currentRevision. It
// returns a descriptive error on the first failing check.
func Verify(secret, token, wantAction, wantID, currentRevision string, now time.Time) error {
	bodyB64, sigB64, ok := splitToken(token)
	if !ok {
		return fmt.Errorf("malformed confirmation token")
	}

	body, err := base64.RawURLEncoding.DecodeString(bodyB64)
	if err != nil {
		return fmt.Errorf("malformed confirmation token: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("malformed confirmation token: %w", err)
	}

	if !hmac.Equal(sig, sign(secret, body)) {
		return fmt.Errorf("confirmation token signature is invalid")
	}

	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		return fmt.Errorf("malformed confirmation token: %w", err)
	}

	if p.Action != wantAction {
		return fmt.Errorf("confirmation token was issued for a different action")
	}
	if p.ID != wantID {
		return fmt.Errorf("confirmation token was issued for a different resource")
	}
	if now.Unix() > p.ExpiresAt {
		return fmt.Errorf("confirmation token has expired; please preview again")
	}
	if p.Revision != currentRevision {
		return fmt.Errorf("resource state has changed since the token was issued; please preview again")
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
