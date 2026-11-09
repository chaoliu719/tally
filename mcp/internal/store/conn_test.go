package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenCreatesSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tally.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	for _, table := range []string{"sources", "categories", "transactions"} {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Fatalf("table %q not found: %v", table, err)
		}
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tally.db")

	db1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first Open failed: %v", err)
	}
	db1.Close()

	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second Open on the same file failed: %v", err)
	}
	defer db2.Close()
}

func TestOpenAddsMerchantColumnToExistingDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tally.db")

	// Build a database with the pre-merchant transactions schema and one row,
	// the way an already-deployed tally database looks before this change.
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw database: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE transactions (
		id           INTEGER PRIMARY KEY,
		ledger_id    INTEGER NOT NULL,
		type         TEXT NOT NULL,
		source_id    INTEGER NOT NULL,
		category_id  INTEGER NOT NULL,
		currency     TEXT NOT NULL,
		amount       INTEGER NOT NULL,
		time         TEXT NOT NULL,
		comment      TEXT NOT NULL DEFAULT '',
		created_at   INTEGER NOT NULL,
		updated_at   INTEGER NOT NULL
	)`); err != nil {
		t.Fatalf("create legacy transactions table: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO transactions
		(id, ledger_id, type, source_id, category_id, currency, amount, time, comment, created_at, updated_at)
		VALUES (1, 1, 'expense', 1, 1, 'CNY', -3200, '2026-01-02 10:00:00', '星巴克 拿铁一杯', 1, 1)`); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	raw.Close()

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	var merchant, comment string
	if err := db.QueryRow(`SELECT merchant, comment FROM transactions WHERE id = 1`).Scan(&merchant, &comment); err != nil {
		t.Fatalf("read migrated row: %v", err)
	}
	if merchant != "" {
		t.Errorf("merchant = %q, want empty string after migration", merchant)
	}
	if comment != "星巴克 拿铁一杯" {
		t.Errorf("comment = %q, want the legacy comment unchanged", comment)
	}
}

func TestOpenSetsWALJournalMode(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tally.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode failed: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want %q", mode, "wal")
	}
}
