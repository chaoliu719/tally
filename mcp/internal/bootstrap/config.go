package bootstrap

import (
	"fmt"
	"os"
)

const (
	envMCPToken           = "TALLY_MCP_TOKEN"
	envConfirmationSecret = "TALLY_CONFIRMATION_SECRET"
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
	DBPath             string
	ListenAddr         string
}

// LoadConfig reads configuration from environment variables. It returns an
// error if TALLY_MCP_TOKEN or TALLY_CONFIRMATION_SECRET is not set; callers
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
		DBPath:             dbPath,
		ListenAddr:         listenAddr,
	}, nil
}
