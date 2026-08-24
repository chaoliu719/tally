package main

import (
	"testing"

	"tally/internal/tools"
)

// TestE2EMinimalLoop drives the exact loop described in proposal.md,
// starting from a brand-new, empty SQLite file: start the server -> create
// a source -> create a (two-level) category -> record an expense -> find
// it via search -> fetch it via get.
// Every step goes through a real MCP tool call over authenticated HTTP,
// using the same buildMux wiring main() uses -- nothing touches the
// database directly.
func TestE2EMinimalLoop(t *testing.T) {
	session, ledgerID := newE2ESession(t)

	// 1. manage_source: create the source.
	var source tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID,
		Operation: "create",
		Name:      "Checking",
	}, &source)

	// 2. manage_category: create a top-level category, then a nested one
	// under it.
	var topCategory tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{LedgerID: ledgerID,
		Operation: "create",
		Name:      "Food",
	}, &topCategory)

	var category tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{LedgerID: ledgerID,
		Operation: "create",
		Name:      "Groceries",
		ParentID:  topCategory.Category.ID,
	}, &category)

	// 3. create_transaction: record one expense.
	var transaction tools.CreateTransactionOutput
	call(t, session, "create_transaction", tools.CreateTransactionInput{LedgerID: ledgerID,
		Type:       "expense",
		SourceID:   source.Source.ID,
		CategoryID: category.Category.ID,
		Amount:     cnyAmount(3000),
		Currency:   "CNY",
		Time:       futureTime(),
		Comment:    "weekly groceries",
	}, &transaction)

	// 4. search_transactions: find the expense.
	var searchResult tools.SearchTransactionsOutput
	call(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID}, &searchResult)
	if len(searchResult.Transactions) != 1 {
		t.Fatalf("expected 1 transaction from search, got %d", len(searchResult.Transactions))
	}
	if searchResult.Transactions[0].ID != transaction.Transaction.ID {
		t.Fatalf("search result missing the recorded expense (id %q): %+v", transaction.Transaction.ID, searchResult.Transactions)
	}

	// 5. get_transaction: fetch it by id.
	var fetched tools.GetTransactionOutput
	call(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: transaction.Transaction.ID}, &fetched)
	if fetched.Transaction.Amount != cnyAmount(3000) {
		t.Fatalf("fetched amount = %q, want %q", fetched.Transaction.Amount, cnyAmount(3000))
	}

	var sources tools.ListSourcesOutput
	call(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerID}, &sources)
	if len(sources.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources.Sources))
	}
}
