package bootstrap

import (
	"database/sql"

	"tally/internal/store"
)

// InitDataStore opens (creating the file if it doesn't exist) the SQLite
// database at cfg.DBPath and applies tally's schema. It is safe to call on
// every startup: schema application is idempotent (`CREATE TABLE IF NOT
// EXISTS`), and there is no user or account to bootstrap -- the database
// itself is the single, implicit ledger.
func InitDataStore(cfg *Config) (*sql.DB, error) {
	return store.Open(cfg.DBPath)
}
