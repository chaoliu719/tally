package main

import (
	"testing"
	"time"

	"tally/internal/tools"
)

// TestE2ETransactionKeywordSearchFindsPrecedent drives the journey
// add-transaction-keyword-search exists for (see its proposal.md's "Why"):
// before recording a new transaction, an Agent searches search_transactions
// by a keyword drawn from the raw bill text to see how the same merchant was
// recorded before -- what source/category it landed under -- without
// pulling the whole ledger through the model. This test records a handful
// of precedent transactions plus noise, then confirms a keyword search over
// real HTTP wiring surfaces exactly the matching precedent, case-
// insensitively, combined with a source filter, and continues to behave
// correctly across a page boundary.
func TestE2ETransactionKeywordSearchFindsPrecedent(t *testing.T) {
	session, ledgerID := newE2ESession(t)

	var wallet tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID,
		Operation: "create",
		Name:      "Checking",
	}, &wallet)

	var dining tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{LedgerID: ledgerID,
		Operation: "create",
		Name:      "Dining",
	}, &dining)

	var groceries tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{LedgerID: ledgerID,
		Operation: "create",
		Name:      "Groceries",
	}, &groceries)

	// A precedent: the same merchant, recorded weeks apart, previously filed
	// under Dining. The exact bill text ("STARBUCKS #4821 TAIPEI") was kept
	// verbatim in comment per the comment discipline convention.
	var precedent tools.CreateTransactionOutput
	call(t, session, "create_transaction", tools.CreateTransactionInput{LedgerID: ledgerID,
		Type:       "expense",
		SourceID:   wallet.Source.ID,
		CategoryID: dining.Category.ID,
		Amount:     cnyAmount(3200),
		Currency:   "CNY",
		Time:       futureTime(),
		Comment:    "STARBUCKS #4821 TAIPEI",
	}, &precedent)

	// Unrelated noise that must not surface for an unrelated keyword.
	call(t, session, "create_transaction", tools.CreateTransactionInput{LedgerID: ledgerID,
		Type:       "expense",
		SourceID:   wallet.Source.ID,
		CategoryID: groceries.Category.ID,
		Amount:     cnyAmount(8800),
		Currency:   "CNY",
		Time:       futureTimeOffset(10 * time.Second),
		Comment:    "Walmart weekly groceries",
	}, &tools.CreateTransactionOutput{})

	// Before recording a brand-new bill for the same merchant, the Agent
	// searches by a lowercase substring of the raw bill text -- matching
	// must be case-insensitive.
	var found tools.SearchTransactionsOutput
	call(t, session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID: ledgerID,
		Keyword:  "starbucks",
	}, &found)
	if len(found.Transactions) != 1 {
		t.Fatalf("expected exactly 1 precedent for keyword \"starbucks\", got %d: %+v", len(found.Transactions), found.Transactions)
	}
	if found.Transactions[0].ID != precedent.Transaction.ID {
		t.Fatalf("found transaction id = %q, want the precedent %q", found.Transactions[0].ID, precedent.Transaction.ID)
	}
	if found.Transactions[0].CategoryID != dining.Category.ID {
		t.Fatalf("precedent CategoryID = %q, want %q (Dining)", found.Transactions[0].CategoryID, dining.Category.ID)
	}

	// Combined with source_id, still resolves to the same precedent.
	var foundWithSource tools.SearchTransactionsOutput
	call(t, session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID: ledgerID,
		SourceID: wallet.Source.ID,
		Keyword:  "starbucks",
	}, &foundWithSource)
	if len(foundWithSource.Transactions) != 1 || foundWithSource.Transactions[0].ID != precedent.Transaction.ID {
		t.Fatalf("keyword+source_id search = %+v, want exactly the precedent %q", foundWithSource.Transactions, precedent.Transaction.ID)
	}

	// Paging through a keyword-filtered search with a page size of 1 must
	// still terminate with no next_cursor once the only match is returned.
	var paged tools.SearchTransactionsOutput
	call(t, session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID: ledgerID,
		Keyword:  "starbucks",
		Limit:    1,
	}, &paged)
	if len(paged.Transactions) != 1 || paged.NextCursor != "" {
		t.Fatalf("paged keyword search = %+v, next_cursor = %q, want 1 result and no next_cursor", paged.Transactions, paged.NextCursor)
	}

	// A keyword with no precedent returns an empty result, not an error.
	var empty tools.SearchTransactionsOutput
	call(t, session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID: ledgerID,
		Keyword:  "costco",
	}, &empty)
	if len(empty.Transactions) != 0 {
		t.Fatalf("expected no precedent for keyword \"costco\", got %+v", empty.Transactions)
	}
}
