package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strconv"
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
	nowStr := time.Now().Format("2006-01-02 15:04:05")

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
		LedgerID: ledger.ID, Type: "income", SourceID: source.ID, CategoryID: category.ID, Currency: "CNY", Amount: 10000, Time: nowStr, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateTransaction (income) failed: %v", err)
	}
	expense, err := q.CreateTransaction(ctx, CreateTransactionParams{
		LedgerID: ledger.ID, Type: "expense", SourceID: source.ID, CategoryID: sub.ID, Currency: "CNY", Amount: -2500, Time: nowStr, CreatedAt: now, UpdatedAt: now,
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

func TestSummarizeTimelineScope(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	q := New(db)
	now := time.Now().Unix()

	ledger, err := q.CreateLedger(ctx, CreateLedgerParams{Name: "Personal", CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("CreateLedger failed: %v", err)
	}
	mkSource := func(name string) int64 {
		s, err := q.CreateSource(ctx, CreateSourceParams{LedgerID: ledger.ID, Name: name, CreatedAt: now, UpdatedAt: now})
		if err != nil {
			t.Fatalf("CreateSource failed: %v", err)
		}
		return s.ID
	}
	mkCategory := func(name string, parent int64) int64 {
		c, err := q.CreateCategory(ctx, CreateCategoryParams{LedgerID: ledger.ID, Name: name, ParentID: parent, CreatedAt: now, UpdatedAt: now})
		if err != nil {
			t.Fatalf("CreateCategory failed: %v", err)
		}
		return c.ID
	}
	cash, card := mkSource("Cash"), mkSource("Card")
	food, groceries, travel := mkCategory("Food", 0), mkCategory("Groceries", 0), mkCategory("Travel", 0)
	_ = groceries

	add := func(typ string, src, cat int64, cur string, amount int64, at string) {
		t.Helper()
		if _, err := q.CreateTransaction(ctx, CreateTransactionParams{
			LedgerID: ledger.ID, Type: typ, SourceID: src, CategoryID: cat, Currency: cur,
			Amount: amount, Time: at, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("CreateTransaction failed: %v", err)
		}
	}
	// expense amounts are stored negative, income positive.
	add("expense", cash, food, "CNY", -1000, "2026-08-31 23:59:59")  // before September
	add("expense", cash, food, "CNY", -2000, "2026-09-01 00:00:00")  // first instant of September
	add("income", card, travel, "CNY", 5000, "2026-09-15 12:00:00")  // mid September
	add("expense", card, travel, "USD", -300, "2026-09-30 23:59:59") // last instant of September
	add("expense", cash, food, "CNY", -4000, "2026-10-01 00:00:00")  // after September

	sum := func(p SummarizeTimelineScopeParams) (count int64, rows []SummarizeTimelineScopeRow) {
		t.Helper()
		p.LedgerID = ledger.ID
		rows, err := q.SummarizeTimelineScope(ctx, p)
		if err != nil {
			t.Fatalf("SummarizeTimelineScope failed: %v", err)
		}
		for _, r := range rows {
			count += r.Count
		}
		return count, rows
	}

	// No scope at all: every transaction in the ledger, grouped by currency.
	count, rows := sum(SummarizeTimelineScopeParams{})
	if count != 5 || len(rows) != 2 {
		t.Fatalf("unscoped: count=%d groups=%d, want 5/2", count, len(rows))
	}

	// September only: both boundary instants are included, neighbours are not.
	count, rows = sum(SummarizeTimelineScopeParams{StartTime: "2026-09-01 00:00:00", EndTime: "2026-09-30 23:59:59"})
	if count != 3 || len(rows) != 2 {
		t.Fatalf("september: count=%d groups=%d, want 3/2", count, len(rows))
	}
	// rows are ordered by currency: CNY then USD.
	if rows[0].Currency != "CNY" || rows[0].Count != 2 || rows[0].Income != 5000 || rows[0].Expense != 2000 {
		t.Errorf("september CNY = %+v, want count 2, income 5000, expense 2000", rows[0])
	}
	if rows[1].Currency != "USD" || rows[1].Count != 1 || rows[1].Income != 0 || rows[1].Expense != 300 {
		t.Errorf("september USD = %+v, want count 1, income 0, expense 300", rows[1])
	}

	// Only a lower bound.
	count, _ = sum(SummarizeTimelineScopeParams{StartTime: "2026-09-30 00:00:00"})
	if count != 2 {
		t.Errorf("start only: count=%d, want 2", count)
	}
	// Only an upper bound.
	count, _ = sum(SummarizeTimelineScopeParams{EndTime: "2026-09-01 00:00:00"})
	if count != 2 {
		t.Errorf("end only: count=%d, want 2", count)
	}

	// Source scope.
	count, _ = sum(SummarizeTimelineScopeParams{SourceID: card})
	if count != 2 {
		t.Errorf("source=card: count=%d, want 2", count)
	}

	// Category set scope (a JSON array of ids, as search_transactions passes).
	count, _ = sum(SummarizeTimelineScopeParams{CategoryIdsJson: `[` + itoa(food) + `]`})
	if count != 3 {
		t.Errorf("category=food: count=%d, want 3", count)
	}
	count, _ = sum(SummarizeTimelineScopeParams{CategoryIdsJson: `[` + itoa(food) + `,` + itoa(travel) + `]`})
	if count != 5 {
		t.Errorf("category=food+travel: count=%d, want 5", count)
	}

	// Combined: September, Card source, Travel category.
	count, _ = sum(SummarizeTimelineScopeParams{
		SourceID: card, CategoryIdsJson: `[` + itoa(travel) + `]`,
		StartTime: "2026-09-01 00:00:00", EndTime: "2026-09-30 23:59:59",
	})
	if count != 2 {
		t.Errorf("combined: count=%d, want 2", count)
	}

	// An empty scope returns no rows at all.
	count, rows = sum(SummarizeTimelineScopeParams{StartTime: "2027-01-01 00:00:00"})
	if count != 0 || len(rows) != 0 {
		t.Errorf("empty scope: count=%d groups=%d, want 0/0", count, len(rows))
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
