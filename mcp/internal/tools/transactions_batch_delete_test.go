package tools_test

import (
	"context"
	"testing"

	"tally/internal/tools"
)

// TestBatchDeleteTransactionsToolIsRegisteredAlongsideDeleteTransaction
// covers tasks.md's 2.4 verification: batch_delete_transactions must show up
// in the tools list next to delete_transaction, not replace it.
func TestBatchDeleteTransactionsToolIsRegisteredAlongsideDeleteTransaction(t *testing.T) {
	session, _ := newTestSession(t)

	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	var sawBatch, sawSingle bool
	for _, tool := range res.Tools {
		switch tool.Name {
		case "batch_delete_transactions":
			sawBatch = true
		case "delete_transaction":
			sawSingle = true
		}
	}
	if !sawBatch {
		t.Error("expected batch_delete_transactions to be registered")
	}
	if !sawSingle {
		t.Error("expected delete_transaction to still be registered")
	}
}

func TestBatchDeleteTransactionsPreviewMixedFoundAndNotFound(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var t1, t2 tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &t1)
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(200), Currency: "CNY", Time: futureTime(),
	}, &t2)

	var preview tools.BatchDeleteTransactionsOutput
	callTool(t, session, "batch_delete_transactions", tools.BatchDeleteTransactionsInput{
		LedgerID: ledgerID,
		IDs:      []string{t1.Transaction.ID, t2.Transaction.ID, "999999"},
	}, &preview)

	if preview.ConfirmationToken == "" {
		t.Fatal("expected a non-empty confirmation_token")
	}
	if len(preview.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(preview.Results))
	}
	if preview.Results[0].Status != "pending_confirmation" || preview.Results[0].Transaction == nil {
		t.Errorf("result[0] = %+v, want pending_confirmation with transaction info", preview.Results[0])
	}
	if preview.Results[1].Status != "pending_confirmation" || preview.Results[1].Transaction == nil {
		t.Errorf("result[1] = %+v, want pending_confirmation with transaction info", preview.Results[1])
	}
	if preview.Results[2].Status != "not_found" || preview.Results[2].Transaction != nil {
		t.Errorf("result[2] = %+v, want not_found with no transaction info", preview.Results[2])
	}

	// Preview must not delete anything.
	callTool(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: t1.Transaction.ID}, &tools.GetTransactionOutput{})
	callTool(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: t2.Transaction.ID}, &tools.GetTransactionOutput{})
}

func TestBatchDeleteTransactionsConfirmHappyPath(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var t1, t2 tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &t1)
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(200), Currency: "CNY", Time: futureTime(),
	}, &t2)

	var preview tools.BatchDeleteTransactionsOutput
	callTool(t, session, "batch_delete_transactions", tools.BatchDeleteTransactionsInput{
		LedgerID: ledgerID,
		IDs:      []string{t1.Transaction.ID, t2.Transaction.ID},
	}, &preview)

	var confirmed tools.BatchDeleteTransactionsOutput
	callTool(t, session, "batch_delete_transactions", tools.BatchDeleteTransactionsInput{
		LedgerID:          ledgerID,
		IDs:               []string{t1.Transaction.ID, t2.Transaction.ID},
		ConfirmationToken: preview.ConfirmationToken,
	}, &confirmed)

	if len(confirmed.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(confirmed.Results))
	}
	for _, r := range confirmed.Results {
		if r.Status != "deleted" {
			t.Errorf("result for %q = %q, want deleted", r.ID, r.Status)
		}
	}

	callToolExpectError(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: t1.Transaction.ID})
	callToolExpectError(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: t2.Transaction.ID})
}

// TestBatchDeleteTransactionsBestEffortAfterOneDeletedBeforeConfirm covers
// spec.md's "确认前有一项已被删除" scenario: previewing 3, deleting one via
// delete_transaction before confirming the batch, then confirming must still
// delete the other two and report the missing one as not_found rather than
// failing the whole call.
func TestBatchDeleteTransactionsBestEffortAfterOneDeletedBeforeConfirm(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var t1, t2, t3 tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &t1)
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(200), Currency: "CNY", Time: futureTime(),
	}, &t2)
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(300), Currency: "CNY", Time: futureTime(),
	}, &t3)

	var preview tools.BatchDeleteTransactionsOutput
	callTool(t, session, "batch_delete_transactions", tools.BatchDeleteTransactionsInput{
		LedgerID: ledgerID,
		IDs:      []string{t1.Transaction.ID, t2.Transaction.ID, t3.Transaction.ID},
	}, &preview)

	// Delete t2 independently, out from under the batch's preview.
	var singlePreview tools.DeleteTransactionOutput
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{LedgerID: ledgerID, ID: t2.Transaction.ID}, &singlePreview)
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{
		LedgerID: ledgerID, ID: t2.Transaction.ID, ConfirmationToken: singlePreview.ConfirmationToken,
	}, &tools.DeleteTransactionOutput{})

	var confirmed tools.BatchDeleteTransactionsOutput
	callTool(t, session, "batch_delete_transactions", tools.BatchDeleteTransactionsInput{
		LedgerID:          ledgerID,
		IDs:               []string{t1.Transaction.ID, t2.Transaction.ID, t3.Transaction.ID},
		ConfirmationToken: preview.ConfirmationToken,
	}, &confirmed)

	byID := map[string]string{}
	for _, r := range confirmed.Results {
		byID[r.ID] = r.Status
	}
	if byID[t1.Transaction.ID] != "deleted" {
		t.Errorf("t1 status = %q, want deleted", byID[t1.Transaction.ID])
	}
	if byID[t2.Transaction.ID] != "not_found" {
		t.Errorf("t2 status = %q, want not_found", byID[t2.Transaction.ID])
	}
	if byID[t3.Transaction.ID] != "deleted" {
		t.Errorf("t3 status = %q, want deleted", byID[t3.Transaction.ID])
	}

	callToolExpectError(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: t1.Transaction.ID})
	callToolExpectError(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: t3.Transaction.ID})
}

// TestBatchDeleteTransactionsBestEffortAfterOneModifiedBeforeConfirm covers
// spec.md's "有一项自预览后发生了变化" scenario.
func TestBatchDeleteTransactionsBestEffortAfterOneModifiedBeforeConfirm(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var t1, t2 tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &t1)
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(200), Currency: "CNY", Time: futureTime(),
	}, &t2)

	var preview tools.BatchDeleteTransactionsOutput
	callTool(t, session, "batch_delete_transactions", tools.BatchDeleteTransactionsInput{
		LedgerID: ledgerID,
		IDs:      []string{t1.Transaction.ID, t2.Transaction.ID},
	}, &preview)

	callTool(t, session, "update_transaction", tools.UpdateTransactionInput{
		LedgerID: ledgerID, ID: t2.Transaction.ID, Type: "expense", SourceID: sourceID, CategoryID: categoryID,
		Amount: cnyAmount(999), Currency: "CNY", Time: futureTime(),
	}, &tools.UpdateTransactionOutput{})

	var confirmed tools.BatchDeleteTransactionsOutput
	callTool(t, session, "batch_delete_transactions", tools.BatchDeleteTransactionsInput{
		LedgerID:          ledgerID,
		IDs:               []string{t1.Transaction.ID, t2.Transaction.ID},
		ConfirmationToken: preview.ConfirmationToken,
	}, &confirmed)

	byID := map[string]string{}
	for _, r := range confirmed.Results {
		byID[r.ID] = r.Status
	}
	if byID[t1.Transaction.ID] != "deleted" {
		t.Errorf("t1 status = %q, want deleted", byID[t1.Transaction.ID])
	}
	if byID[t2.Transaction.ID] != "revision_changed" {
		t.Errorf("t2 status = %q, want revision_changed", byID[t2.Transaction.ID])
	}

	// t2 must survive, unmodified further, since it was not deleted.
	var got tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: t2.Transaction.ID}, &got)
	if got.Transaction.Amount != cnyAmount(999) {
		t.Errorf("t2 Amount = %q, want %q", got.Transaction.Amount, cnyAmount(999))
	}
}

func TestBatchDeleteTransactionsExpiredOrTamperedTokenRejectsWholeConfirm(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var t1 tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &t1)

	callToolExpectError(t, session, "batch_delete_transactions", tools.BatchDeleteTransactionsInput{
		LedgerID:          ledgerID,
		IDs:               []string{t1.Transaction.ID},
		ConfirmationToken: "not-a-real-token",
	})

	// Nothing should have been deleted.
	callTool(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: t1.Transaction.ID}, &tools.GetTransactionOutput{})
}

func TestBatchDeleteTransactionsRejectsOverMaxIDs(t *testing.T) {
	session, ledgerID := newTestSession(t)

	ids := make([]string, 101)
	for i := range ids {
		ids[i] = "1"
	}

	callToolExpectError(t, session, "batch_delete_transactions", tools.BatchDeleteTransactionsInput{
		LedgerID: ledgerID,
		IDs:      ids,
	})
}

func TestBatchDeleteTransactionsConfirmRejectsMismatchedIDSet(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var t1, t2 tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &t1)
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(200), Currency: "CNY", Time: futureTime(),
	}, &t2)

	var preview tools.BatchDeleteTransactionsOutput
	callTool(t, session, "batch_delete_transactions", tools.BatchDeleteTransactionsInput{
		LedgerID: ledgerID,
		IDs:      []string{t1.Transaction.ID},
	}, &preview)

	// Confirming with a different (larger) id set than what was previewed
	// must be rejected -- the token only authorizes the exact batch it was
	// issued for.
	callToolExpectError(t, session, "batch_delete_transactions", tools.BatchDeleteTransactionsInput{
		LedgerID:          ledgerID,
		IDs:               []string{t1.Transaction.ID, t2.Transaction.ID},
		ConfirmationToken: preview.ConfirmationToken,
	})

	callTool(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: t1.Transaction.ID}, &tools.GetTransactionOutput{})
}
