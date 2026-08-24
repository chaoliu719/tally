package store

import (
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
