package tools_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"tally/internal/confirm"
	"tally/internal/store"
	"tally/internal/tools"
)

func TestBatchUpdateTransactionsOnlyBudgetAndCombinedPatch(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceA, categoryA := setupSourceAndCategory(t, session, ledgerID)
	sourceB := createTestSource(t, session, ledgerID)
	categoryB := createTestCategory(t, session, ledgerID, "Dining", "")
	budgetA := firstBudgetID(t, session, ledgerID)
	budgetB := createTestBudget(t, session, ledgerID, "专项")

	var t1, t2 tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceA, CategoryID: categoryA, BudgetID: budgetA,
		Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(), Merchant: "M1", Comment: "C1",
	}, &t1)
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceA, CategoryID: categoryA, BudgetID: budgetA,
		Amount: cnyAmount(200), Currency: "CNY", Time: futureTime(), Merchant: "M2", Comment: "C2",
	}, &t2)

	// Scenario 1: patch only budget_id
	var preview1 tools.BatchUpdateTransactionsOutput
	callTool(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID: ledgerID,
		IDs:      []string{t1.Transaction.ID, t2.Transaction.ID},
		Patch:    tools.BatchUpdateTransactionsPatch{BudgetID: budgetB},
	}, &preview1)
	if preview1.MatchedCount != 2 || preview1.ConfirmationToken == "" {
		t.Fatalf("unexpected preview1: %+v", preview1)
	}
	if preview1.Results[0].Before.BudgetID != budgetA || preview1.Results[0].After.BudgetID != budgetB {
		t.Fatalf("unexpected diff in preview1: %+v -> %+v", preview1.Results[0].Before, preview1.Results[0].After)
	}

	var confirmed1 tools.BatchUpdateTransactionsOutput
	callTool(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID:          ledgerID,
		IDs:               []string{t1.Transaction.ID, t2.Transaction.ID},
		Patch:             tools.BatchUpdateTransactionsPatch{BudgetID: budgetB},
		ConfirmationToken: preview1.ConfirmationToken,
	}, &confirmed1)
	for _, r := range confirmed1.Results {
		if r.Status != "updated" || r.Transaction.BudgetID != budgetB || r.Transaction.CategoryID != categoryA || r.Transaction.SourceID != sourceA {
			t.Fatalf("unexpected updated item: %+v", r)
		}
	}

	// Scenario 2: patch category_id and source_id simultaneously, leaving budget_id unchanged
	var preview2 tools.BatchUpdateTransactionsOutput
	callTool(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID: ledgerID,
		IDs:      []string{t1.Transaction.ID, t2.Transaction.ID},
		Patch:    tools.BatchUpdateTransactionsPatch{CategoryID: categoryB, SourceID: sourceB},
	}, &preview2)

	var confirmed2 tools.BatchUpdateTransactionsOutput
	callTool(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID:          ledgerID,
		IDs:               []string{t1.Transaction.ID, t2.Transaction.ID},
		Patch:             tools.BatchUpdateTransactionsPatch{CategoryID: categoryB, SourceID: sourceB},
		ConfirmationToken: preview2.ConfirmationToken,
	}, &confirmed2)
	for _, r := range confirmed2.Results {
		if r.Status != "updated" || r.Transaction.CategoryID != categoryB || r.Transaction.SourceID != sourceB || r.Transaction.BudgetID != budgetB {
			t.Fatalf("unexpected updated item in scenario 2: %+v", r)
		}
	}
}

func TestBatchUpdateTransactionsRejectsEmptyPatchAndNonReferenceFields(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceA, categoryA := setupSourceAndCategory(t, session, ledgerID)
	var t1 tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceA, CategoryID: categoryA,
		Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &t1)

	// Empty patch is rejected.
	callToolExpectError(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID: ledgerID,
		IDs:      []string{t1.Transaction.ID},
		Patch:    tools.BatchUpdateTransactionsPatch{},
	})

	// Attempting to batch-modify amount in patch is rejected (JSON schema / unknown field rejection).
	callToolExpectError(t, session, "batch_update_transactions", map[string]any{
		"ledger_id": ledgerID,
		"ids":       []string{t1.Transaction.ID},
		"patch": map[string]any{
			"amount": "999.00",
		},
	})
}

func TestBatchUpdateTransactionsByFilterAndBackfillUnassigned(t *testing.T) {
	session, ledgerID, q := newTestSessionWithStore(t)
	sourceA, categoryA := setupSourceAndCategory(t, session, ledgerID)
	categoryB := createTestCategory(t, session, ledgerID, "OtherCat", "")
	budgetA := firstBudgetID(t, session, ledgerID)
	budgetB := createTestBudget(t, session, ledgerID, "回填目标")

	// Insert two legacy unassigned transactions (budget_id = 0) under categoryA and one assigned transaction under categoryB.
	lID, _ := strconv.ParseInt(ledgerID, 10, 64)
	sID, _ := strconv.ParseInt(sourceA, 10, 64)
	cA, _ := strconv.ParseInt(categoryA, 10, 64)
	now := time.Now().Unix()

	legacy1, err := q.CreateTransaction(context.Background(), store.CreateTransactionParams{
		LedgerID: lID, Type: "expense", SourceID: sID, CategoryID: cA, BudgetID: 0,
		Currency: "CNY", Amount: -100, Time: futureTime(), CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("insert legacy1: %v", err)
	}
	legacy2, err := q.CreateTransaction(context.Background(), store.CreateTransactionParams{
		LedgerID: lID, Type: "expense", SourceID: sID, CategoryID: cA, BudgetID: 0,
		Currency: "CNY", Amount: -200, Time: futureTime(), CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("insert legacy2: %v", err)
	}
	_ = legacy1
	_ = legacy2

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceA, CategoryID: categoryB, BudgetID: budgetA,
		Amount: cnyAmount(300), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	// Filter by budget_id = "0" to backfill unassigned transactions to budgetB.
	var preview tools.BatchUpdateTransactionsOutput
	callTool(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID: ledgerID,
		Filter:   &tools.BatchUpdateTransactionsFilter{BudgetID: "0"},
		Patch:    tools.BatchUpdateTransactionsPatch{BudgetID: budgetB},
	}, &preview)
	if preview.MatchedCount != 2 {
		t.Fatalf("MatchedCount = %d, want 2", preview.MatchedCount)
	}

	var confirmed tools.BatchUpdateTransactionsOutput
	callTool(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID:          ledgerID,
		Filter:            &tools.BatchUpdateTransactionsFilter{BudgetID: "0"},
		Patch:             tools.BatchUpdateTransactionsPatch{BudgetID: budgetB},
		ConfirmationToken: preview.ConfirmationToken,
	}, &confirmed)
	if confirmed.MatchedCount != 2 {
		t.Fatalf("confirmed MatchedCount = %d, want 2", confirmed.MatchedCount)
	}

	// Verify no unassigned transactions remain.
	var remaining tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID: ledgerID,
		BudgetID: "0",
	}, &remaining)
	if len(remaining.Transactions) != 0 {
		t.Fatalf("expected 0 unassigned transactions after backfill, got %d", len(remaining.Transactions))
	}
}

func TestBatchUpdateTransactionsPreviewRejectsInvalidAndCrossLedgerReferences(t *testing.T) {
	session, ledgerA := newTestSession(t)
	sourceA, categoryA := setupSourceAndCategory(t, session, ledgerA)
	var tA tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerA, Type: "expense", SourceID: sourceA, CategoryID: categoryA,
		Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &tA)

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Ledger B"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID
	sourceB, categoryB := setupSourceAndCategory(t, session, ledgerB)
	budgetB := firstBudgetID(t, session, ledgerB)
	var tB tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerB, Type: "expense", SourceID: sourceB, CategoryID: categoryB, BudgetID: budgetB,
		Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &tB)

	// Nonexistent budget in patch -> rejected on preview
	callToolExpectError(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID: ledgerA,
		IDs:      []string{tA.Transaction.ID},
		Patch:    tools.BatchUpdateTransactionsPatch{BudgetID: "999999"},
	})

	// Cross-ledger category in patch -> rejected on preview
	callToolExpectError(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID: ledgerA,
		IDs:      []string{tA.Transaction.ID},
		Patch:    tools.BatchUpdateTransactionsPatch{CategoryID: categoryB},
	})

	// Cross-ledger source in patch -> rejected on preview
	callToolExpectError(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID: ledgerA,
		IDs:      []string{tA.Transaction.ID},
		Patch:    tools.BatchUpdateTransactionsPatch{SourceID: sourceB},
	})

	// Cross-ledger budget in patch -> rejected on preview
	callToolExpectError(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID: ledgerA,
		IDs:      []string{tA.Transaction.ID},
		Patch:    tools.BatchUpdateTransactionsPatch{BudgetID: budgetB},
	})

	// Transaction ID belonging to another ledger is reported as not_found in preview
	budgetA := firstBudgetID(t, session, ledgerA)
	var preview tools.BatchUpdateTransactionsOutput
	callTool(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID: ledgerA,
		IDs:      []string{tA.Transaction.ID, tB.Transaction.ID},
		Patch:    tools.BatchUpdateTransactionsPatch{BudgetID: budgetA},
	}, &preview)
	if preview.MatchedCount != 1 || preview.Results[1].Status != "not_found" {
		t.Fatalf("expected cross-ledger tx ID to be reported as not_found, got %+v", preview)
	}
}

func TestBatchUpdateTransactionsBestEffortAndExpiredToken(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceA, categoryA := setupSourceAndCategory(t, session, ledgerID)
	budgetA := firstBudgetID(t, session, ledgerID)
	budgetB := createTestBudget(t, session, ledgerID, "新预算")

	var t1, t2, t3 tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceA, CategoryID: categoryA, BudgetID: budgetA,
		Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &t1)
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceA, CategoryID: categoryA, BudgetID: budgetA,
		Amount: cnyAmount(200), Currency: "CNY", Time: futureTime(),
	}, &t2)
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: sourceA, CategoryID: categoryA, BudgetID: budgetA,
		Amount: cnyAmount(300), Currency: "CNY", Time: futureTime(),
	}, &t3)

	var preview tools.BatchUpdateTransactionsOutput
	callTool(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID: ledgerID,
		IDs:      []string{t1.Transaction.ID, t2.Transaction.ID, t3.Transaction.ID},
		Patch:    tools.BatchUpdateTransactionsPatch{BudgetID: budgetB},
	}, &preview)

	// Modify t2 and delete t3 before confirming.
	callTool(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID: t2.Transaction.ID, LedgerID: ledgerID, Type: "expense", SourceID: sourceA, CategoryID: categoryA, BudgetID: budgetA,
		Amount: cnyAmount(999), Currency: "CNY", Time: futureTime(),
	}, &tools.UpdateTransactionOutput{})

	var delPrev tools.DeleteTransactionOutput
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{LedgerID: ledgerID, ID: t3.Transaction.ID}, &delPrev)
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{
		LedgerID: ledgerID, ID: t3.Transaction.ID, ConfirmationToken: delPrev.ConfirmationToken,
	}, &tools.DeleteTransactionOutput{})

	var confirmed tools.BatchUpdateTransactionsOutput
	callTool(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID:          ledgerID,
		IDs:               []string{t1.Transaction.ID, t2.Transaction.ID, t3.Transaction.ID},
		Patch:             tools.BatchUpdateTransactionsPatch{BudgetID: budgetB},
		ConfirmationToken: preview.ConfirmationToken,
	}, &confirmed)

	byID := map[string]string{}
	for _, r := range confirmed.Results {
		byID[r.ID] = r.Status
	}
	if byID[t1.Transaction.ID] != "updated" {
		t.Errorf("t1 status = %q, want updated", byID[t1.Transaction.ID])
	}
	if byID[t2.Transaction.ID] != "revision_changed" {
		t.Errorf("t2 status = %q, want revision_changed", byID[t2.Transaction.ID])
	}
	if byID[t3.Transaction.ID] != "not_found" {
		t.Errorf("t3 status = %q, want not_found", byID[t3.Transaction.ID])
	}

	// Expired or tampered token is rejected.
	callToolExpectError(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID:          ledgerID,
		IDs:               []string{t1.Transaction.ID},
		Patch:             tools.BatchUpdateTransactionsPatch{BudgetID: budgetB},
		ConfirmationToken: "tampered.token",
	})

	expired := craftExpiredBatchToken(t, testConfirmSecret, preview.ConfirmationToken)
	callToolExpectError(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID:          ledgerID,
		IDs:               []string{t1.Transaction.ID, t2.Transaction.ID, t3.Transaction.ID},
		Patch:             tools.BatchUpdateTransactionsPatch{BudgetID: budgetB},
		ConfirmationToken: expired,
	})
}

func TestBatchUpdateTransactionsRejectsOverMaxLimit(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceA, categoryA := setupSourceAndCategory(t, session, ledgerID)
	budgetA := firstBudgetID(t, session, ledgerID)
	budgetB := createTestBudget(t, session, ledgerID, "目标")

	// Over 100 IDs rejected.
	ids := make([]string, 101)
	for i := range ids {
		ids[i] = "1"
	}
	callToolExpectError(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID: ledgerID,
		IDs:      ids,
		Patch:    tools.BatchUpdateTransactionsPatch{BudgetID: budgetB},
	})

	// Over 100 filter matches rejected.
	createNExpenses(t, session, ledgerID, sourceA, categoryA, 101, futureUnix())
	_ = budgetA
	callToolExpectError(t, session, "batch_update_transactions", tools.BatchUpdateTransactionsInput{
		LedgerID: ledgerID,
		Filter:   &tools.BatchUpdateTransactionsFilter{CategoryID: categoryA},
		Patch:    tools.BatchUpdateTransactionsPatch{BudgetID: budgetB},
	})
}

func craftExpiredBatchToken(t *testing.T, secret, validToken string) string {
	t.Helper()
	var split int
	for i := len(validToken) - 1; i >= 0; i-- {
		if validToken[i] == '.' {
			split = i
			break
		}
	}
	raw, err := base64.RawURLEncoding.DecodeString(validToken[:split])
	if err != nil {
		t.Fatalf("decode token body: %v", err)
	}
	var p struct {
		Action    string         `json:"action"`
		Items     []confirm.Item `json:"items"`
		ExpiresAt int64          `json:"expires_at"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal token body: %v", err)
	}
	p.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	body, _ := json.Marshal(p)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(sig)
}
