package bootstrap

import (
	"fmt"
	"os"
	"strings"
)

const (
	envMCPToken           = "TALLY_MCP_TOKEN"
	envConfirmationSecret = "TALLY_CONFIRMATION_SECRET"
	envOAuthSigningSecret = "TALLY_OAUTH_SIGNING_SECRET"
	envPublicBaseURL      = "TALLY_PUBLIC_BASE_URL"
	envDBPath             = "TALLY_DB_PATH"
	envListenAddr         = "TALLY_LISTEN_ADDR"

	defaultDBPath     = "./tally.db"
	defaultListenAddr = ":16355"
)

// Config holds the runtime configuration for the tally-mcp process, loaded
// entirely from environment variables.
type Config struct {
	MCPToken           string
	ConfirmationSecret string
	// OAuthSigningSecret signs/verifies the OAuth authorization codes,
	// access tokens, and client IDs (see internal/oauth). Independent of
	// MCPToken and ConfirmationSecret.
	OAuthSigningSecret string
	// PublicBaseURL is the externally reachable origin of this server
	// (e.g. https://tally.liuchao.life), with no trailing slash. It anchors
	// the OAuth issuer, the well-known metadata URLs, and the canonical
	// resource URI (PublicBaseURL + "/mcp") that access tokens are bound to.
	PublicBaseURL string
	DBPath        string
	ListenAddr    string
}

// LoadConfig reads configuration from environment variables. It returns an
// error if any required variable (TALLY_MCP_TOKEN, TALLY_CONFIRMATION_SECRET,
// TALLY_OAUTH_SIGNING_SECRET, TALLY_PUBLIC_BASE_URL) is not set; callers
// should treat that as fatal.
func LoadConfig() (*Config, error) {
	token := os.Getenv(envMCPToken)
	if token == "" {
		return nil, fmt.Errorf("%s environment variable is required but not set", envMCPToken)
	}

	confirmationSecret := os.Getenv(envConfirmationSecret)
	if confirmationSecret == "" {
		return nil, fmt.Errorf("%s environment variable is required but not set", envConfirmationSecret)
	}

	oauthSigningSecret := os.Getenv(envOAuthSigningSecret)
	if oauthSigningSecret == "" {
		return nil, fmt.Errorf("%s environment variable is required but not set", envOAuthSigningSecret)
	}

	publicBaseURL := strings.TrimRight(os.Getenv(envPublicBaseURL), "/")
	if publicBaseURL == "" {
		return nil, fmt.Errorf("%s environment variable is required but not set", envPublicBaseURL)
	}

	dbPath := os.Getenv(envDBPath)
	if dbPath == "" {
		dbPath = defaultDBPath
	}

	listenAddr := os.Getenv(envListenAddr)
	if listenAddr == "" {
		listenAddr = defaultListenAddr
	}

	return &Config{
		MCPToken:           token,
		ConfirmationSecret: confirmationSecret,
		OAuthSigningSecret: oauthSigningSecret,
		PublicBaseURL:      publicBaseURL,
		DBPath:             dbPath,
		ListenAddr:         listenAddr,
	}, nil
}
