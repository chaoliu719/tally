package confirm_test

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"tally/internal/confirm"
)

const (
	secret   = "test-secret"
	action   = "delete_account"
	id       = "42"
	revision = "abc123"
)

func TestIssueVerifyRoundTrip(t *testing.T) {
	token, expiresAt := confirm.Issue(secret, action, id, revision)
	if token == "" {
		t.Fatal("expected a non-empty token")
	}
	if expiresAt <= time.Now().Unix() {
		t.Fatalf("expiresAt = %d, want in the future", expiresAt)
	}

	if err := confirm.Verify(secret, token, action, id, revision, time.Now()); err != nil {
		t.Fatalf("Verify failed on a freshly issued token: %v", err)
	}
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	token, _ := confirm.Issue(secret, action, id, revision)

	tampered := tamperSignature(t, token)

	err := confirm.Verify(secret, tampered, action, id, revision, time.Now())
	if err == nil {
		t.Fatal("expected an error for a tampered signature")
	}
}

func TestVerifyRejectsWrongAction(t *testing.T) {
	token, _ := confirm.Issue(secret, action, id, revision)

	err := confirm.Verify(secret, token, "delete_category", id, revision, time.Now())
	if err == nil {
		t.Fatal("expected an error for a mismatched action")
	}
}

func TestVerifyRejectsWrongID(t *testing.T) {
	token, _ := confirm.Issue(secret, action, id, revision)

	err := confirm.Verify(secret, token, action, "999", revision, time.Now())
	if err == nil {
		t.Fatal("expected an error for a mismatched id")
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	token, _ := confirm.Issue(secret, action, id, revision)

	future := time.Now().Add(confirm.TTL + time.Minute)
	err := confirm.Verify(secret, token, action, id, revision, future)
	if err == nil {
		t.Fatal("expected an error for an expired token")
	}
}

func TestVerifyRejectsMismatchedRevision(t *testing.T) {
	token, _ := confirm.Issue(secret, action, id, revision)

	err := confirm.Verify(secret, token, action, id, "different-revision", time.Now())
	if err == nil {
		t.Fatal("expected an error for a mismatched revision")
	}
}

// TestVerifyErrorsAreDistinct verifies each failure mode returns a
// distinguishable, descriptive error rather than one shared generic message.
func TestVerifyErrorsAreDistinct(t *testing.T) {
	token, _ := confirm.Issue(secret, action, id, revision)
	tampered := tamperSignature(t, token)
	future := time.Now().Add(confirm.TTL + time.Minute)

	cases := map[string]error{
		"tampered":    confirm.Verify(secret, tampered, action, id, revision, time.Now()),
		"wrongAction": confirm.Verify(secret, token, "delete_category", id, revision, time.Now()),
		"wrongID":     confirm.Verify(secret, token, action, "999", revision, time.Now()),
		"expired":     confirm.Verify(secret, token, action, id, revision, future),
		"revision":    confirm.Verify(secret, token, action, id, "different-revision", time.Now()),
	}

	seen := map[string]bool{}
	for name, err := range cases {
		if err == nil {
			t.Fatalf("%s: expected an error", name)
		}
		msg := err.Error()
		if seen[msg] {
			t.Fatalf("%s: error message %q is not distinct from another failure mode", name, msg)
		}
		seen[msg] = true
	}
}

// tamperSignature flips a bit in the token's signature bytes and
// re-encodes it, guaranteeing the decoded signature actually changes.
// (Flipping the trailing base64url character isn't reliable: the final
// character of a 32-byte HMAC only encodes 4 significant bits, so some
// flips there land on unused padding bits and decode to the same bytes.)
func tamperSignature(t *testing.T, token string) string {
	t.Helper()

	i := strings.LastIndexByte(token, '.')
	if i < 0 {
		t.Fatalf("token %q has no signature segment", token)
	}
	bodyB64, sigB64 := token[:i], token[i+1:]

	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	sig[0] ^= 0xFF

	return bodyB64 + "." + base64.RawURLEncoding.EncodeToString(sig)
}
