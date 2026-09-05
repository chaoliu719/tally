package main

import (
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
