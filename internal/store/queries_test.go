package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := Open(filepath.Join(t.TempDir(), "tally.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestTransactionsReferenceSourceAndCategory(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	q := New(db)

	now := time.Now().Unix()

	ledger, err := q.CreateLedger(ctx, CreateLedgerParams{
		Name: "Personal", CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateLedger failed: %v", err)
	}

	source, err := q.CreateSource(ctx, CreateSourceParams{
		LedgerID: ledger.ID, Name: "Cash", CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateSource failed: %v", err)
	}

	category, err := q.CreateCategory(ctx, CreateCategoryParams{
		LedgerID: ledger.ID, Name: "Food", ParentID: 0, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}
	sub, err := q.CreateCategory(ctx, CreateCategoryParams{
		LedgerID: ledger.ID, Name: "Groceries", ParentID: category.ID, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateCategory (sub) failed: %v", err)
	}

	income, err := q.CreateTransaction(ctx, CreateTransactionParams{
		LedgerID: ledger.ID, Type: "income", SourceID: source.ID, CategoryID: category.ID, Currency: "CNY", Amount: 10000, Time: now, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateTransaction (income) failed: %v", err)
	}
	expense, err := q.CreateTransaction(ctx, CreateTransactionParams{
		LedgerID: ledger.ID, Type: "expense", SourceID: source.ID, CategoryID: sub.ID, Currency: "CNY", Amount: -2500, Time: now, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateTransaction (expense) failed: %v", err)
	}

	sources, err := q.ListSources(ctx, ledger.ID)
	if err != nil {
		t.Fatalf("ListSources failed: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}

	count, err := q.CountTransactionsBySource(ctx, source.ID)
	if err != nil {
		t.Fatalf("CountTransactionsBySource failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("CountTransactionsBySource = %d, want 2", count)
	}

	got, err := q.GetTransaction(ctx, GetTransactionParams{ID: income.ID, LedgerID: ledger.ID})
	if err != nil {
		t.Fatalf("GetTransaction (income) failed: %v", err)
	}
	if got.SourceID != source.ID || got.CategoryID != category.ID || got.Currency != "CNY" || got.Amount != 10000 {
		t.Fatalf("GetTransaction (income) = %+v, unexpected", got)
	}

	got, err = q.GetTransaction(ctx, GetTransactionParams{ID: expense.ID, LedgerID: ledger.ID})
	if err != nil {
		t.Fatalf("GetTransaction (expense) failed: %v", err)
	}
	if got.SourceID != source.ID || got.CategoryID != sub.ID || got.Amount != -2500 {
		t.Fatalf("GetTransaction (expense) = %+v, unexpected", got)
	}
}
