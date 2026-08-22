package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Open opens (creating the file if it doesn't exist) the SQLite database at
// path, applies tally's schema, and returns a ready-to-use connection pool.
//
// WAL mode lets readers and writers proceed without blocking each other;
// busy_timeout turns a writer-writer conflict into "wait and retry" instead
// of an immediate SQLITE_BUSY error. Both are set via DSN query parameters,
// which modernc.org/sqlite applies to every new pooled connection, not just
// the first one opened.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if _, err := db.ExecContext(context.Background(), schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return db, nil
}
