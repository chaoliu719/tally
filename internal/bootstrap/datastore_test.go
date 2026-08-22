package bootstrap_test

import (
	"path/filepath"
	"testing"

	"tally/internal/bootstrap"
)

func TestInitDataStoreFreshFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tally.db")
	cfg := &bootstrap.Config{MCPToken: "unused", DBPath: dbPath}

	db, err := bootstrap.InitDataStore(cfg)
	if err != nil {
		t.Fatalf("InitDataStore failed: %v", err)
	}
	defer db.Close()

	var name string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='user'").Scan(&name)
	if err == nil {
		t.Fatal("expected no user table in the database")
	}
}

func TestInitDataStoreTwiceIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tally.db")
	cfg := &bootstrap.Config{MCPToken: "unused", DBPath: dbPath}

	db1, err := bootstrap.InitDataStore(cfg)
	if err != nil {
		t.Fatalf("first InitDataStore failed: %v", err)
	}
	db1.Close()

	db2, err := bootstrap.InitDataStore(cfg)
	if err != nil {
		t.Fatalf("second InitDataStore failed: %v", err)
	}
	defer db2.Close()
}
