package main

import (
	"testing"

	"tally/internal/tools"
)

// TestE2ELedgerIsolation drives two ledgers, each with a same-named source,
// category, and transaction, all through main()'s real buildMux wiring, and
// verifies they never leak into each other's listings, searches, or
// summaries -- and that a transaction in one ledger cannot reference a
// source/category from the other. It also drives list_ledgers, since it has
// no per-ledger scope of its own and this is the test with more than one
// ledger to list.
func TestE2ELedgerIsolation(t *testing.T) {
	session, ledgerA := newE2ESession(t)

	var ledgerBOut tools.ManageLedgerOutput
	call(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "create",
		Name:      "Company",
	}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID

	// list_ledgers: unlike every other list_* tool, this one isn't scoped to
	// a single ledger, so its journey coverage lives here alongside the
	// other cross-ledger assertions -- both ledgers must show up.
	var ledgers tools.ListLedgersOutput
	call(t, session, "list_ledgers", tools.ListLedgersInput{}, &ledgers)
	var foundA, foundB bool
	for _, l := range ledgers.Ledgers {
		foundA = foundA || l.ID == ledgerA
		foundB = foundB || l.ID == ledgerB
	}
	if !foundA || !foundB {
		t.Fatalf("list_ledgers = %+v, want both ledgerA (%q) and ledgerB (%q)", ledgers.Ledgers, ledgerA, ledgerB)
	}

	// Same-named source, category, and transaction amount in both ledgers.
	var sourceA, sourceB tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerA, Operation: "create", Name: "Cash"}, &sourceA)
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerB, Operation: "create", Name: "Cash"}, &sourceB)

	var categoryA, categoryB tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{LedgerID: ledgerA, Operation: "create", Name: "Food"}, &categoryA)
	call(t, session, "manage_category", tools.ManageCategoryInput{LedgerID: ledgerB, Operation: "create", Name: "Food"}, &categoryB)

	var txnA, txnB tools.CreateTransactionOutput
	call(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerA, Type: "expense", SourceID: sourceA.Source.ID, CategoryID: categoryA.Category.ID,
		Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &txnA)
	call(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerB, Type: "expense", SourceID: sourceB.Source.ID, CategoryID: categoryB.Category.ID,
		Amount: cnyAmount(999), Currency: "CNY", Time: futureTime(),
	}, &txnB)

	// list_sources/list_categories: each ledger sees only its own row, even
	// though the names collide.
	var sourcesA, sourcesB tools.ListSourcesOutput
	call(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerA}, &sourcesA)
	call(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerB}, &sourcesB)
	if len(sourcesA.Sources) != 1 || sourcesA.Sources[0].ID != sourceA.Source.ID {
		t.Fatalf("ledgerA sources = %+v, want only sourceA", sourcesA.Sources)
	}
	if len(sourcesB.Sources) != 1 || sourcesB.Sources[0].ID != sourceB.Source.ID {
		t.Fatalf("ledgerB sources = %+v, want only sourceB", sourcesB.Sources)
	}

	var categoriesA, categoriesB tools.ListCategoriesOutput
	call(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerA}, &categoriesA)
	call(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerB}, &categoriesB)
	if len(categoriesA.Categories) != 1 || categoriesA.Categories[0].ID != categoryA.Category.ID {
		t.Fatalf("ledgerA categories = %+v, want only categoryA", categoriesA.Categories)
	}
	if len(categoriesB.Categories) != 1 || categoriesB.Categories[0].ID != categoryB.Category.ID {
		t.Fatalf("ledgerB categories = %+v, want only categoryB", categoriesB.Categories)
	}

	// search_transactions: each ledger sees only its own transaction.
	var txnsA, txnsB tools.SearchTransactionsOutput
	call(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerA}, &txnsA)
	call(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerB}, &txnsB)
	if len(txnsA.Transactions) != 1 || txnsA.Transactions[0].Amount != cnyAmount(100) {
		t.Fatalf("ledgerA transactions = %+v, want a single 100-amount transaction", txnsA.Transactions)
	}
	if len(txnsB.Transactions) != 1 || txnsB.Transactions[0].Amount != cnyAmount(999) {
		t.Fatalf("ledgerB transactions = %+v, want a single 999-amount transaction", txnsB.Transactions)
	}

	// get_financial_summary: each ledger's summary reflects only its own
	// activity, despite both being in CNY over the same time window.
	var summaryA tools.GetFinancialSummaryOutput
	call(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{LedgerID: ledgerA}, &summaryA)
	if len(summaryA.TotalsByCurrency) != 1 || summaryA.TotalsByCurrency[0].Expense != 100 {
		t.Fatalf("ledgerA summary = %+v, want expense=100 only", summaryA.TotalsByCurrency)
	}

	// A transaction in ledgerA cannot reference ledgerB's source or category.
	callExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerA, Type: "expense", SourceID: sourceB.Source.ID, CategoryID: categoryA.Category.ID,
		Amount: cnyAmount(50), Currency: "CNY", Time: futureTime(),
	})
	callExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerA, Type: "expense", SourceID: sourceA.Source.ID, CategoryID: categoryB.Category.ID,
		Amount: cnyAmount(50), Currency: "CNY", Time: futureTime(),
	})

	// get_transaction under the wrong ledger behaves like "not found".
	callExpectError(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerB, ID: txnA.Transaction.ID})

	// Neither ledger can be deleted while it still holds a source, category,
	// and transaction.
	callExpectError(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "delete", ID: ledgerA})
	callExpectError(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "delete", ID: ledgerB})
}
