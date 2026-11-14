package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

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

	if err := ensureTransactionBudgetColumn(context.Background(), db); err != nil {
		db.Close()
		return nil, err
	}

	if err := ensureInitialBudgetsSeeded(context.Background(), db); err != nil {
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

// SeedBudgetNames is the canonical set of budgets seeded for every ledger:
// both existing ledgers during the one-time user_version 0 -> 1 migration,
// and new ledgers when created via manage_ledger (operation="create").
var SeedBudgetNames = []string{"固定", "专项", "突发", "日常"}

// ensureTransactionBudgetColumn adds the budget_id column (default 0, meaning
// "unassigned") to databases created before the budget dimension existed, and
// ensures its index exists.
func ensureTransactionBudgetColumn(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(transactions)`)
	if err != nil {
		return fmt.Errorf("inspect transactions columns: %w", err)
	}
	defer rows.Close()

	hasBudgetID := false
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
		if name == "budget_id" {
			hasBudgetID = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect transactions columns: %w", err)
	}

	if !hasBudgetID {
		if _, err := db.ExecContext(ctx, `ALTER TABLE transactions ADD COLUMN budget_id INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add transactions.budget_id column: %w", err)
		}
	}

	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_transactions_budget_time ON transactions(budget_id, time)`); err != nil {
		return fmt.Errorf("create idx_transactions_budget_time: %w", err)
	}
	return nil
}

// ensureInitialBudgetsSeeded seeds the four default budgets (固定, 专项, 突发, 日常)
// for every existing ledger exactly once, gated by PRAGMA user_version 0 -> 1.
// Because it checks user_version rather than whether budgets is empty, deleting
// or renaming seeded budgets later will not cause them to be recreated on the
// next startup.
func ensureInitialBudgetsSeeded(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	if version >= 1 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin seed budgets tx: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `SELECT id FROM ledgers ORDER BY id`)
	if err != nil {
		return fmt.Errorf("list ledgers for budget seed: %w", err)
	}
	var ledgerIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan ledger id for budget seed: %w", err)
		}
		ledgerIDs = append(ledgerIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("list ledgers for budget seed: %w", err)
	}

	now := time.Now().Unix()
	for _, ledgerID := range ledgerIDs {
		for _, name := range SeedBudgetNames {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO budgets (ledger_id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
				ledgerID, name, now, now,
			); err != nil {
				return fmt.Errorf("seed budget %q for ledger %d: %w", name, ledgerID, err)
			}
		}
	}

	if _, err := tx.ExecContext(ctx, `PRAGMA user_version = 1`); err != nil {
		return fmt.Errorf("set user_version = 1: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit seed budgets tx: %w", err)
	}
	return nil
}
