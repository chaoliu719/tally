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

func TestAggregatedBalanceMatchesTransactionSum(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	q := New(db)

	now := time.Now().Unix()

	account, err := q.CreateAccount(ctx, CreateAccountParams{
		Name: "Cash", Type: "cash", Currency: "CNY", CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	category, err := q.CreateCategory(ctx, CreateCategoryParams{
		Name: "Food", ParentID: 0, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}
	sub, err := q.CreateCategory(ctx, CreateCategoryParams{
		Name: "Groceries", ParentID: category.ID, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateCategory (sub) failed: %v", err)
	}

	if _, err := q.CreateTransaction(ctx, CreateTransactionParams{
		Type: "balance_adjustment", AccountID: account.ID, CategoryID: sql.NullInt64{}, Amount: 10000, Time: now, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateTransaction (balance_adjustment) failed: %v", err)
	}
	if _, err := q.CreateTransaction(ctx, CreateTransactionParams{
		Type: "expense", AccountID: account.ID, CategoryID: sql.NullInt64{Int64: sub.ID, Valid: true}, Amount: -2500, Time: now, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateTransaction (expense) failed: %v", err)
	}

	balance, err := q.GetAccountBalance(ctx, account.ID)
	if err != nil {
		t.Fatalf("GetAccountBalance failed: %v", err)
	}
	if balance != 10000-2500 {
		t.Fatalf("balance = %d, want %d", balance, 10000-2500)
	}

	rows, err := q.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("ListAccounts failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 account, got %d", len(rows))
	}
	if rows[0].Balance != 10000-2500 {
		t.Fatalf("ListAccounts balance = %d, want %d", rows[0].Balance, 10000-2500)
	}
}

func TestCheckConstraintRejectsIncomeExpenseWithoutCategory(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	q := New(db)

	now := time.Now().Unix()
	account, err := q.CreateAccount(ctx, CreateAccountParams{
		Name: "Cash", Type: "cash", Currency: "CNY", CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	_, err = q.CreateTransaction(ctx, CreateTransactionParams{
		Type: "expense", AccountID: account.ID, CategoryID: sql.NullInt64{}, Amount: -100, Time: now, CreatedAt: now, UpdatedAt: now,
	})
	if err == nil {
		t.Fatal("expected CHECK constraint violation for expense without category_id, got nil error")
	}
}

func TestCheckConstraintRejectsBalanceAdjustmentWithCategory(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	q := New(db)

	now := time.Now().Unix()
	account, err := q.CreateAccount(ctx, CreateAccountParams{
		Name: "Cash", Type: "cash", Currency: "CNY", CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}
	category, err := q.CreateCategory(ctx, CreateCategoryParams{
		Name: "Food", ParentID: 0, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	_, err = q.CreateTransaction(ctx, CreateTransactionParams{
		Type: "balance_adjustment", AccountID: account.ID, CategoryID: sql.NullInt64{Int64: category.ID, Valid: true}, Amount: 100, Time: now, CreatedAt: now, UpdatedAt: now,
	})
	if err == nil {
		t.Fatal("expected CHECK constraint violation for balance_adjustment with category_id, got nil error")
	}
}
