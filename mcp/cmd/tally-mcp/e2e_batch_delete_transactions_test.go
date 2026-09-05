package main

import (
	"testing"

	"tally/internal/tools"
)

// TestE2EBatchDeleteTransactionsPreviewThenConfirm drives
// batch_delete_transactions through the real wiring (HTTP + SQLite): preview
// a batch, confirm it, and verify each item's reported result matches
// reality via get_transaction.
func TestE2EBatchDeleteTransactionsPreviewThenConfirm(t *testing.T) {
	session, ledgerID := newE2ESession(t)

	var source tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID,
		Operation: "create",
		Name:      "Checking",
	}, &source)

	var category tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{LedgerID: ledgerID,
		Operation: "create",
		Name:      "Groceries",
	}, &category)

	var t1, t2 tools.CreateTransactionOutput
	call(t, session, "create_transaction", tools.CreateTransactionInput{LedgerID: ledgerID,
		Type:       "expense",
		SourceID:   source.Source.ID,
		CategoryID: category.Category.ID,
		Amount:     cnyAmount(1000),
		Currency:   "CNY",
		Time:       futureTime(),
	}, &t1)
	call(t, session, "create_transaction", tools.CreateTransactionInput{LedgerID: ledgerID,
		Type:       "expense",
		SourceID:   source.Source.ID,
		CategoryID: category.Category.ID,
		Amount:     cnyAmount(2000),
		Currency:   "CNY",
		Time:       futureTime(),
	}, &t2)

	var preview tools.BatchDeleteTransactionsOutput
	call(t, session, "batch_delete_transactions", tools.BatchDeleteTransactionsInput{
		LedgerID: ledgerID,
		IDs:      []string{t1.Transaction.ID, t2.Transaction.ID, "999999"},
	}, &preview)
	if preview.ConfirmationToken == "" {
		t.Fatal("expected a non-empty confirmation_token from preview")
	}
	if len(preview.Results) != 3 {
		t.Fatalf("preview Results len = %d, want 3", len(preview.Results))
	}

	var confirmed tools.BatchDeleteTransactionsOutput
	call(t, session, "batch_delete_transactions", tools.BatchDeleteTransactionsInput{
		LedgerID:          ledgerID,
		IDs:               []string{t1.Transaction.ID, t2.Transaction.ID, "999999"},
		ConfirmationToken: preview.ConfirmationToken,
	}, &confirmed)

	byID := make(map[string]string, len(confirmed.Results))
	for _, r := range confirmed.Results {
		byID[r.ID] = r.Status
	}
	if byID[t1.Transaction.ID] != "deleted" {
		t.Errorf("t1 status = %q, want deleted", byID[t1.Transaction.ID])
	}
	if byID[t2.Transaction.ID] != "deleted" {
		t.Errorf("t2 status = %q, want deleted", byID[t2.Transaction.ID])
	}
	if byID["999999"] != "not_found" {
		t.Errorf("999999 status = %q, want not_found", byID["999999"])
	}

	callExpectError(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: t1.Transaction.ID})
	callExpectError(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: t2.Transaction.ID})
}
