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
		"TALLY_DB_PATH",
		"TALLY_LISTEN_ADDR",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadConfigRequiresMCPToken(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TALLY_CONFIRMATION_SECRET", "secret")

	if _, err := bootstrap.LoadConfig(); err == nil {
		t.Fatal("expected an error when TALLY_MCP_TOKEN is not set")
	}
}

func TestLoadConfigRequiresConfirmationSecret(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TALLY_MCP_TOKEN", "token")

	if _, err := bootstrap.LoadConfig(); err == nil {
		t.Fatal("expected an error when TALLY_CONFIRMATION_SECRET is not set")
	}
}

func TestLoadConfigSucceedsWithBothRequiredVars(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TALLY_MCP_TOKEN", "token")
	t.Setenv("TALLY_CONFIRMATION_SECRET", "secret")

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.MCPToken != "token" {
		t.Errorf("MCPToken = %q, want %q", cfg.MCPToken, "token")
	}
	if cfg.ConfirmationSecret != "secret" {
		t.Errorf("ConfirmationSecret = %q, want %q", cfg.ConfirmationSecret, "secret")
	}
	if cfg.DBPath != "./tally.db" {
		t.Errorf("DBPath = %q, want default", cfg.DBPath)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want default", cfg.ListenAddr)
	}
}
