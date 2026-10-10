package main

import (
	"fmt"
	"testing"
	"time"

	"tally/internal/tools"
)

// TestE2ETransactionTimelineNewestFirstPaging drives the journey the
// transaction timeline widget needs (add-transaction-timeline-widget): over
// real HTTP wiring, page search_transactions with newest_first=true from the
// latest transaction back to the earliest, one page at a time, and confirm
// the walk covers every transaction exactly once in descending time order
// and terminates with no next_cursor once the earliest row is returned.
func TestE2ETransactionTimelineNewestFirstPaging(t *testing.T) {
	session, ledgerID := newE2ESession(t)

	var wallet tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID,
		Operation: "create", Name: "Checking",
	}, &wallet)

	var cat tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{LedgerID: ledgerID,
		Operation: "create", Name: "Dining",
	}, &cat)

	const n = 7
	for i := 0; i < n; i++ {
		call(t, session, "create_transaction", tools.CreateTransactionInput{LedgerID: ledgerID,
			Type:       "expense",
			SourceID:   wallet.Source.ID,
			CategoryID: cat.Category.ID,
			Amount:     cnyAmount(int64(100 + i)),
			Currency:   "CNY",
			Time:       futureTimeOffset(time.Duration(i) * time.Hour),
		}, &tools.CreateTransactionOutput{})
	}

	var collected []tools.TransactionInfo
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 20 {
			t.Fatal("newest-first pagination did not terminate")
		}
		var page tools.SearchTransactionsOutput
		call(t, session, "search_transactions", tools.SearchTransactionsInput{
			LedgerID:    ledgerID,
			Limit:       3,
			NewestFirst: true,
			Cursor:      cursor,
		}, &page)
		collected = append(collected, page.Transactions...)
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}

	if len(collected) != n {
		t.Fatalf("collected %d transactions across pages, want %d", len(collected), n)
	}
	seen := map[string]bool{}
	for i, txn := range collected {
		if seen[txn.ID] {
			t.Fatalf("transaction %q returned on more than one page", txn.ID)
		}
		seen[txn.ID] = true
		if i > 0 && collected[i-1].Time < txn.Time {
			t.Fatalf("not newest-first at index %d: %s then %s", i, collected[i-1].Time, txn.Time)
		}
	}
	latest := futureTimeOffset(time.Duration(n-1) * time.Hour)
	earliest := futureTimeOffset(0)
	if collected[0].Time != latest {
		t.Fatalf("first row time = %s, want the latest transaction %s", collected[0].Time, latest)
	}
	if collected[n-1].Time != earliest {
		t.Fatalf("last row time = %s, want the earliest transaction %s", collected[n-1].Time, earliest)
	}
}

// TestE2ETransactionTimelineScopedDrain walks a scoped timeline the way the
// widget does (scope-timeline-widget-by-range): take the first page from
// open_transaction_timeline, then page the rest with search_transactions
// (newest_first, limit 100) repeating the scope's filters verbatim from the
// result. The union must be exactly the transactions inside the scope --
// including both edge instants of the range -- and nothing outside it, with
// the count the tool reported.
func TestE2ETransactionTimelineScopedDrain(t *testing.T) {
	session, ledgerID := newE2ESession(t)

	var wallet tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID,
		Operation: "create", Name: "Checking",
	}, &wallet)
	var cat tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{LedgerID: ledgerID,
		Operation: "create", Name: "Dining",
	}, &cat)

	add := func(at string) {
		call(t, session, "create_transaction", tools.CreateTransactionInput{LedgerID: ledgerID,
			Type: "expense", SourceID: wallet.Source.ID, CategoryID: cat.Category.ID,
			Amount: cnyAmount(100), Currency: "CNY", Time: at,
		}, &tools.CreateTransactionOutput{})
	}
	// Outside the scope on both sides, hugging the edges...
	add("2026-08-31 23:59:59")
	add("2026-10-01 00:00:00")
	// ...and 230 September rows: more than the 200 the first page carries and
	// more than one 100-row drain page, so the cursor handoff is exercised.
	const inScope = 230
	for i := 0; i < inScope-2; i++ {
		add(fmt.Sprintf("2026-09-%02d %02d:%02d:00", 2+i%28, i%24, i%60))
	}
	add("2026-09-01 00:00:00")
	add("2026-09-30 23:59:59")

	var first tools.OpenTransactionTimelineOutput
	call(t, session, "open_transaction_timeline", tools.OpenTransactionTimelineInput{
		LedgerID: ledgerID, StartDate: "2026-09-01", EndDate: "2026-09-30",
	}, &first)
	if first.Total != inScope {
		t.Fatalf("total = %d, want %d (the scope, not the whole ledger)", first.Total, inScope)
	}
	if len(first.Transactions) != 200 || first.NextCursor == "" {
		t.Fatalf("first page rows=%d cursor=%q, want 200 and a cursor", len(first.Transactions), first.NextCursor)
	}

	collected := append([]tools.TransactionInfo(nil), first.Transactions...)
	cursor := first.NextCursor
	for pages := 0; cursor != ""; pages++ {
		if pages > 10 {
			t.Fatal("scoped drain did not terminate")
		}
		var page tools.SearchTransactionsOutput
		call(t, session, "search_transactions", tools.SearchTransactionsInput{
			LedgerID: ledgerID, NewestFirst: true, Limit: 100, Cursor: cursor,
			StartTime: first.Scope.StartTime, EndTime: first.Scope.EndTime,
		}, &page)
		collected = append(collected, page.Transactions...)
		cursor = page.NextCursor
	}

	if len(collected) != inScope {
		t.Fatalf("drained %d transactions, want exactly the %d in scope", len(collected), inScope)
	}
	seen := map[string]bool{}
	for i, tx := range collected {
		if seen[tx.ID] {
			t.Fatalf("transaction %s returned twice", tx.ID)
		}
		seen[tx.ID] = true
		if tx.Time < "2026-09-01 00:00:00" || tx.Time > "2026-09-30 23:59:59" {
			t.Fatalf("row %d at %s is outside the September scope", i, tx.Time)
		}
		if i > 0 && collected[i-1].Time < tx.Time {
			t.Fatalf("not newest-first at index %d: %s then %s", i, collected[i-1].Time, tx.Time)
		}
	}
	if collected[0].Time != "2026-09-30 23:59:59" || collected[inScope-1].Time != "2026-09-01 00:00:00" {
		t.Errorf("scope edges: first=%s last=%s, want both edge instants present",
			collected[0].Time, collected[inScope-1].Time)
	}
}
