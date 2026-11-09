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

	if err := ensureTransactionMerchantColumn(context.Background(), db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// ensureTransactionMerchantColumn adds the merchant column to databases that
// were created before it existed. schema.sql only runs CREATE TABLE IF NOT
// EXISTS, which leaves an existing transactions table untouched, and tally has
// no migration framework. This single additive migration is applied
// idempotently at startup: it only inspects the table and adds the column when
// it is missing, never dropping or rewriting existing rows.
func ensureTransactionMerchantColumn(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(transactions)`)
	if err != nil {
		return fmt.Errorf("inspect transactions columns: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			colType    string
			notNull    int
			defaultV   sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultV, &primaryKey); err != nil {
			return fmt.Errorf("inspect transactions columns: %w", err)
		}
		if name == "merchant" {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect transactions columns: %w", err)
	}

	if _, err := db.ExecContext(ctx, `ALTER TABLE transactions ADD COLUMN merchant TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("add transactions.merchant column: %w", err)
	}
	return nil
}
