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

	for _, table := range []string{"sources", "categories", "budgets", "transactions"} {
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

func TestOpenAddsBudgetColumnAndSeedsExistingLedgersOnce(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tally.db")

	// Build a pre-budget database with one ledger and one transaction.
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw database: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE ledgers (
		id          INTEGER PRIMARY KEY,
		name        TEXT NOT NULL,
		comment     TEXT NOT NULL DEFAULT '',
		created_at  INTEGER NOT NULL,
		updated_at  INTEGER NOT NULL
	)`); err != nil {
		t.Fatalf("create legacy ledgers table: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO ledgers (id, name, comment, created_at, updated_at) VALUES (1, 'Personal', '', 1, 1)`); err != nil {
		t.Fatalf("insert legacy ledger: %v", err)
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
		merchant     TEXT NOT NULL DEFAULT '',
		comment      TEXT NOT NULL DEFAULT '',
		created_at   INTEGER NOT NULL,
		updated_at   INTEGER NOT NULL
	)`); err != nil {
		t.Fatalf("create legacy transactions table: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO transactions
		(id, ledger_id, type, source_id, category_id, currency, amount, time, merchant, comment, created_at, updated_at)
		VALUES (1, 1, 'expense', 1, 1, 'CNY', -3200, '2026-01-02 10:00:00', '星巴克', '拿铁一杯', 1, 1)`); err != nil {
		t.Fatalf("insert legacy transaction: %v", err)
	}
	raw.Close()

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	var budgetID int64
	if err := db.QueryRow(`SELECT budget_id FROM transactions WHERE id = 1`).Scan(&budgetID); err != nil {
		t.Fatalf("read migrated budget_id: %v", err)
	}
	if budgetID != 0 {
		t.Errorf("budget_id = %d, want 0 (unassigned) after migration", budgetID)
	}

	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 1 {
		t.Errorf("user_version = %d, want 1", version)
	}

	rows, err := db.Query(`SELECT name FROM budgets WHERE ledger_id = 1 ORDER BY id`)
	if err != nil {
		t.Fatalf("query seeded budgets: %v", err)
	}
	var gotNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan budget name: %v", err)
		}
		gotNames = append(gotNames, name)
	}
	rows.Close()

	wantNames := []string{"固定", "专项", "突发", "日常"}
	if len(gotNames) != len(wantNames) {
		t.Fatalf("seeded budgets = %v, want %v", gotNames, wantNames)
	}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Errorf("seeded budget[%d] = %q, want %q", i, gotNames[i], wantNames[i])
		}
	}

	// Reopening does not duplicate seeded budgets.
	db.Close()
	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second Open failed: %v", err)
	}
	var count int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM budgets WHERE ledger_id = 1`).Scan(&count); err != nil {
		t.Fatalf("count budgets after second Open: %v", err)
	}
	if count != 4 {
		t.Fatalf("budget count after second Open = %d, want 4", count)
	}

	// Deleting all seeded budgets and reopening must NOT re-seed them.
	if _, err := db2.Exec(`DELETE FROM budgets WHERE ledger_id = 1`); err != nil {
		t.Fatalf("delete all budgets: %v", err)
	}
	db2.Close()

	db3, err := Open(dbPath)
	if err != nil {
		t.Fatalf("third Open failed: %v", err)
	}
	defer db3.Close()
	if err := db3.QueryRow(`SELECT COUNT(*) FROM budgets WHERE ledger_id = 1`).Scan(&count); err != nil {
		t.Fatalf("count budgets after third Open: %v", err)
	}
	if count != 0 {
		t.Fatalf("budget count after deleting all and reopening = %d, want 0", count)
	}
}
