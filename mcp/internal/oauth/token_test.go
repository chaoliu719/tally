package oauth

import (
	"strings"
	"testing"
	"time"
)

const testSecret = "test-signing-secret"

func TestAuthCodeRoundTrip(t *testing.T) {
	code := issueAuthCode(testSecret, "challenge-abc", "https://c.example/cb", "https://tally.test/mcp")

	p, err := verifyAuthCode(testSecret, code, time.Now())
	if err != nil {
		t.Fatalf("verifyAuthCode on a fresh code: %v", err)
	}
	if p.CodeChallenge != "challenge-abc" || p.RedirectURI != "https://c.example/cb" || p.Resource != "https://tally.test/mcp" {
		t.Fatalf("round-tripped payload lost a field: %+v", p)
	}
}

func TestAuthCodeRejectsTampered(t *testing.T) {
	code := issueAuthCode(testSecret, "cc", "https://c.example/cb", "res")
	tampered := flipSig(t, code)
	if _, err := verifyAuthCode(testSecret, tampered, time.Now()); err == nil {
		t.Fatal("expected tampered code to fail verification")
	}
}

func TestAuthCodeRejectsWrongSecret(t *testing.T) {
	code := issueAuthCode(testSecret, "cc", "https://c.example/cb", "res")
	if _, err := verifyAuthCode("other-secret", code, time.Now()); err == nil {
		t.Fatal("expected code signed with a different secret to fail")
	}
}

func TestAuthCodeRejectsExpired(t *testing.T) {
	code := issueAuthCode(testSecret, "cc", "https://c.example/cb", "res")
	future := time.Now().Add(authCodeTTL + time.Minute)
	if _, err := verifyAuthCode(testSecret, code, future); err == nil {
		t.Fatal("expected an expired code to fail")
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	tok := IssueAccessToken(testSecret, "https://tally.test/mcp")
	exp, err := VerifyAccessToken(testSecret, tok, "https://tally.test/mcp", time.Now())
	if err != nil {
		t.Fatalf("VerifyAccessToken on a fresh token: %v", err)
	}
	if time.Until(exp) <= 0 || time.Until(exp) > accessTokenTTL+time.Minute {
		t.Fatalf("expiry %v is not within the expected window", exp)
	}
}

func TestAccessTokenRejectsWrongAudience(t *testing.T) {
	tok := IssueAccessToken(testSecret, "https://tally.test/mcp")
	if _, err := VerifyAccessToken(testSecret, tok, "https://someone-else.test/mcp", time.Now()); err == nil {
		t.Fatal("expected a token for another audience to be rejected")
	}
}

func TestAccessTokenRejectsExpired(t *testing.T) {
	tok := IssueAccessToken(testSecret, "https://tally.test/mcp")
	future := time.Now().Add(accessTokenTTL + time.Minute)
	if _, err := VerifyAccessToken(testSecret, tok, "https://tally.test/mcp", future); err == nil {
		t.Fatal("expected an expired access token to be rejected")
	}
}

func TestAccessTokenRejectsRotatedSecret(t *testing.T) {
	tok := IssueAccessToken(testSecret, "https://tally.test/mcp")
	if _, err := VerifyAccessToken("rotated-secret", tok, "https://tally.test/mcp", time.Now()); err == nil {
		t.Fatal("expected a token signed with the pre-rotation secret to be rejected")
	}
}

func TestRefreshTokenRoundTrip(t *testing.T) {
	tok, expiresIn := issueRefreshToken(testSecret, "https://tally.test/mcp", "jti-1")
	if expiresIn <= 0 {
		t.Fatalf("expiresIn = %d, want > 0", expiresIn)
	}
	exp, err := verifyRefreshToken(testSecret, tok, "https://tally.test/mcp", time.Now())
	if err != nil {
		t.Fatalf("verifyRefreshToken on a fresh token: %v", err)
	}
	if time.Until(exp) <= 0 || time.Until(exp) > refreshTokenTTL+time.Minute {
		t.Fatalf("expiry %v is not within the expected window", exp)
	}
}

func TestRefreshTokenRejections(t *testing.T) {
	good := func() string {
		tok, _ := issueRefreshToken(testSecret, "https://tally.test/mcp", "jti")
		return tok
	}
	now := time.Now()

	if _, err := verifyRefreshToken(testSecret, flipSig(t, good()), "https://tally.test/mcp", now); err == nil {
		t.Error("tampered refresh token verified")
	}
	if _, err := verifyRefreshToken("rotated-secret", good(), "https://tally.test/mcp", now); err == nil {
		t.Error("refresh token signed with the pre-rotation secret verified")
	}
	if _, err := verifyRefreshToken(testSecret, good(), "https://someone-else.test/mcp", now); err == nil {
		t.Error("refresh token for another audience verified")
	}
	future := now.Add(refreshTokenTTL + time.Minute)
	if _, err := verifyRefreshToken(testSecret, good(), "https://tally.test/mcp", future); err == nil {
		t.Error("expired refresh token verified")
	}
	// An access token must not verify as a refresh token, and vice versa.
	at := IssueAccessToken(testSecret, "https://tally.test/mcp")
	if _, err := verifyRefreshToken(testSecret, at, "https://tally.test/mcp", now); err == nil {
		t.Error("an access token verified as a refresh token")
	}
	if _, err := VerifyAccessToken(testSecret, good(), "https://tally.test/mcp", now); err == nil {
		t.Error("a refresh token verified as an access token")
	}
}

func TestTokensAreNotCrossUsable(t *testing.T) {
	// An access token must not verify as an authorization code, and vice versa.
	at := IssueAccessToken(testSecret, "https://tally.test/mcp")
	if _, err := verifyAuthCode(testSecret, at, time.Now()); err == nil {
		t.Error("an access token verified as an authorization code")
	}

	code := issueAuthCode(testSecret, "cc", "https://c.example/cb", "https://tally.test/mcp")
	if _, err := VerifyAccessToken(testSecret, code, "https://tally.test/mcp", time.Now()); err == nil {
		t.Error("an authorization code verified as an access token")
	}

	cid := issueClientID(testSecret, []string{"https://c.example/cb"})
	if _, err := VerifyAccessToken(testSecret, cid, "https://tally.test/mcp", time.Now()); err == nil {
		t.Error("a client_id verified as an access token")
	}
}

func TestClientIDRoundTrip(t *testing.T) {
	uris := []string{"https://c.example/cb", "https://c.example/cb2"}
	cid := issueClientID(testSecret, uris)

	p, err := verifyClientID(testSecret, cid)
	if err != nil {
		t.Fatalf("verifyClientID on a fresh client_id: %v", err)
	}
	if len(p.RedirectURIs) != 2 || p.RedirectURIs[0] != uris[0] || p.RedirectURIs[1] != uris[1] {
		t.Fatalf("round-tripped redirect URIs lost: %+v", p.RedirectURIs)
	}
}

func TestClientIDRejectsTampered(t *testing.T) {
	cid := issueClientID(testSecret, []string{"https://c.example/cb"})
	if _, err := verifyClientID(testSecret, flipSig(t, cid)); err == nil {
		t.Fatal("expected a tampered client_id to fail")
	}
}

func TestDecodeRejectsMalformed(t *testing.T) {
	var p authCodePayload
	for _, bad := range []string{"", "no-dot", "a.b.c.d", "!!!.???"} {
		if err := decode(testSecret, bad, &p); err == nil {
			t.Errorf("decode(%q) succeeded, want error", bad)
		}
	}
}

// flipSig flips a bit in a token's signature segment so the decoded bytes
// genuinely change (see internal/confirm's tamperSignature for why the
// trailing base64 char is not reliable).
func flipSig(t *testing.T, token string) string {
	t.Helper()
	i := strings.LastIndexByte(token, '.')
	if i < 0 {
		t.Fatalf("token %q has no signature segment", token)
	}
	body, sig := token[:i], []byte(token[i+1:])
	sig[0] ^= 0x7F
	return body + "." + string(sig)
}
