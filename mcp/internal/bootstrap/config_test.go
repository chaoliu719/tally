package bootstrap_test

import (
	"testing"

	"tally/internal/bootstrap"
)

// clearConfigEnv resets the config-relevant environment variables around a
// test, since LoadConfig reads directly from the process environment.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"TALLY_MCP_TOKEN",
		"TALLY_CONFIRMATION_SECRET",
		"TALLY_OAUTH_SIGNING_SECRET",
		"TALLY_PUBLIC_BASE_URL",
		"TALLY_DB_PATH",
		"TALLY_LISTEN_ADDR",
	} {
		t.Setenv(key, "")
	}
}

// setRequiredEnv sets every required var to a valid placeholder, so a test
// can then clear exactly one and assert LoadConfig rejects it.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("TALLY_MCP_TOKEN", "token")
	t.Setenv("TALLY_CONFIRMATION_SECRET", "confirm-secret")
	t.Setenv("TALLY_OAUTH_SIGNING_SECRET", "oauth-secret")
	t.Setenv("TALLY_PUBLIC_BASE_URL", "https://tally.example.com")
}

func TestLoadConfigRequiresMCPToken(t *testing.T) {
	clearConfigEnv(t)
	setRequiredEnv(t)
	t.Setenv("TALLY_MCP_TOKEN", "")

	if _, err := bootstrap.LoadConfig(); err == nil {
		t.Fatal("expected an error when TALLY_MCP_TOKEN is not set")
	}
}

func TestLoadConfigRequiresConfirmationSecret(t *testing.T) {
	clearConfigEnv(t)
	setRequiredEnv(t)
	t.Setenv("TALLY_CONFIRMATION_SECRET", "")

	if _, err := bootstrap.LoadConfig(); err == nil {
		t.Fatal("expected an error when TALLY_CONFIRMATION_SECRET is not set")
	}
}

func TestLoadConfigRequiresOAuthSigningSecret(t *testing.T) {
	clearConfigEnv(t)
	setRequiredEnv(t)
	t.Setenv("TALLY_OAUTH_SIGNING_SECRET", "")

	if _, err := bootstrap.LoadConfig(); err == nil {
		t.Fatal("expected an error when TALLY_OAUTH_SIGNING_SECRET is not set")
	}
}

func TestLoadConfigRequiresPublicBaseURL(t *testing.T) {
	clearConfigEnv(t)
	setRequiredEnv(t)
	t.Setenv("TALLY_PUBLIC_BASE_URL", "")

	if _, err := bootstrap.LoadConfig(); err == nil {
		t.Fatal("expected an error when TALLY_PUBLIC_BASE_URL is not set")
	}
}

func TestLoadConfigSucceedsWithRequiredVars(t *testing.T) {
	clearConfigEnv(t)
	setRequiredEnv(t)

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.MCPToken != "token" {
		t.Errorf("MCPToken = %q, want %q", cfg.MCPToken, "token")
	}
	if cfg.ConfirmationSecret != "confirm-secret" {
		t.Errorf("ConfirmationSecret = %q, want %q", cfg.ConfirmationSecret, "confirm-secret")
	}
	// The three secrets are read from distinct variables, never shared.
	if cfg.OAuthSigningSecret != "oauth-secret" {
		t.Errorf("OAuthSigningSecret = %q, want %q", cfg.OAuthSigningSecret, "oauth-secret")
	}
	if cfg.OAuthSigningSecret == cfg.MCPToken || cfg.OAuthSigningSecret == cfg.ConfirmationSecret {
		t.Error("OAuthSigningSecret must come from its own variable, independent of the other two")
	}
	if cfg.DBPath != "./tally.db" {
		t.Errorf("DBPath = %q, want default", cfg.DBPath)
	}
	if cfg.ListenAddr != ":16355" {
		t.Errorf("ListenAddr = %q, want default", cfg.ListenAddr)
	}
}

func TestLoadConfigTrimsTrailingSlashFromBaseURL(t *testing.T) {
	clearConfigEnv(t)
	setRequiredEnv(t)
	t.Setenv("TALLY_PUBLIC_BASE_URL", "https://tally.example.com/")

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.PublicBaseURL != "https://tally.example.com" {
		t.Errorf("PublicBaseURL = %q, want trailing slash trimmed", cfg.PublicBaseURL)
	}
}
