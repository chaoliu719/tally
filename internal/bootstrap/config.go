package bootstrap

import (
	"fmt"
	"os"
)

const (
	envMCPToken        = "TALLY_MCP_TOKEN"
	envDefaultCurrency = "TALLY_DEFAULT_CURRENCY"
	envDBPath          = "TALLY_DB_PATH"
	envListenAddr      = "TALLY_LISTEN_ADDR"

	defaultCurrency   = "CNY"
	defaultDBPath     = "./tally.db"
	defaultListenAddr = ":8080"
)

// Config holds the runtime configuration for the tally-mcp process, loaded
// entirely from environment variables (no ini file, unlike upstream
// ezbookkeeping).
type Config struct {
	MCPToken        string
	DefaultCurrency string
	DBPath          string
	ListenAddr      string
}

// LoadConfig reads configuration from environment variables. It returns an
// error if TALLY_MCP_TOKEN is not set; callers should treat that as fatal.
func LoadConfig() (*Config, error) {
	token := os.Getenv(envMCPToken)
	if token == "" {
		return nil, fmt.Errorf("%s environment variable is required but not set", envMCPToken)
	}

	currency := os.Getenv(envDefaultCurrency)
	if currency == "" {
		currency = defaultCurrency
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
		MCPToken:        token,
		DefaultCurrency: currency,
		DBPath:          dbPath,
		ListenAddr:      listenAddr,
	}, nil
}
