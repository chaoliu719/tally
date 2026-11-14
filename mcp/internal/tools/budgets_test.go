package tools_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/store"
	"tally/internal/tools"
)

func createTestBudget(t *testing.T, session *mcp.ClientSession, ledgerID, name string) string {
	t.Helper()

	var created tools.ManageBudgetOutput
	callTool(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      name,
	}, &created)

	return created.Budget.ID
}

func TestListBudgetsEmptyLedger(t *testing.T) {
	session, ledgerID := newTestSession(t)

	var out tools.ListBudgetsOutput
	callTool(t, session, "list_budgets", tools.ListBudgetsInput{LedgerID: ledgerID}, &out)

	// Delete any seeded budgets if present so we also verify an empty budgets list returns an empty slice, not nil.
	for _, b := range out.Budgets {
		var preview tools.ManageBudgetOutput
		callTool(t, session, "manage_budget", tools.ManageBudgetInput{
			LedgerID:  ledgerID,
			Operation: "delete",
			ID:        b.ID,
		}, &preview)
		callTool(t, session, "manage_budget", tools.ManageBudgetInput{
			LedgerID:          ledgerID,
			Operation:         "delete",
			ID:                b.ID,
			ConfirmationToken: preview.ConfirmationToken,
		}, &tools.ManageBudgetOutput{})
	}

	callTool(t, session, "list_budgets", tools.ListBudgetsInput{LedgerID: ledgerID}, &out)
	if out.Budgets == nil {
		t.Fatal("expected an empty slice, got nil")
	}
	if len(out.Budgets) != 0 {
		t.Fatalf("expected 0 budgets, got %d", len(out.Budgets))
	}
}

func TestManageBudgetCreate(t *testing.T) {
	session, ledgerID := newTestSession(t)

	var created tools.ManageBudgetOutput
	callTool(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      "旅行",
	}, &created)

	if created.Status != "created" {
		t.Errorf("Status = %q, want %q", created.Status, "created")
	}
	if created.Budget.Name != "旅行" {
		t.Errorf("Name = %q, want %q", created.Budget.Name, "旅行")
	}

	var list tools.ListBudgetsOutput
	callTool(t, session, "list_budgets", tools.ListBudgetsInput{LedgerID: ledgerID}, &list)

	var found bool
	for _, b := range list.Budgets {
		if b.ID == created.Budget.ID && b.Name == "旅行" {
			found = true
		}
	}
	if !found {
		t.Fatalf("created budget %+v not found in list_budgets: %+v", created.Budget, list.Budgets)
	}
}

func TestManageBudgetMissingRequiredField(t *testing.T) {
	session, ledgerID := newTestSession(t)

	callToolExpectError(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerID,
		Operation: "create",
	})
	callToolExpectError(t, session, "manage_budget", tools.ManageBudgetInput{
		Operation: "create",
		Name:      "旅行",
	})
}

func TestManageBudgetUpdateHappyPath(t *testing.T) {
	session, ledgerID := newTestSession(t)
	budgetID := createTestBudget(t, session, ledgerID, "旧预算")

	var updated tools.ManageBudgetOutput
	callTool(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerID,
		Operation: "update",
		ID:        budgetID,
		Name:      "新预算",
	}, &updated)

	if updated.Status != "updated" {
		t.Errorf("Status = %q, want %q", updated.Status, "updated")
	}
	if updated.Budget.Name != "新预算" {
		t.Errorf("Name = %q, want %q", updated.Budget.Name, "新预算")
	}

	var list tools.ListBudgetsOutput
	callTool(t, session, "list_budgets", tools.ListBudgetsInput{LedgerID: ledgerID}, &list)
	var found bool
	for _, b := range list.Budgets {
		if b.ID == budgetID && b.Name == "新预算" {
			found = true
		}
	}
	if !found {
		t.Errorf("updated budget not reflected in list_budgets: %+v", list.Budgets)
	}
}

func TestManageBudgetUpdateNotFound(t *testing.T) {
	session, ledgerID := newTestSession(t)

	callToolExpectError(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerID,
		Operation: "update",
		ID:        "999999",
		Name:      "Ghost",
	})
}

func TestManageBudgetUpdateMissingRequiredField(t *testing.T) {
	session, ledgerID := newTestSession(t)
	budgetID := createTestBudget(t, session, ledgerID, "日常")

	callToolExpectError(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerID,
		Operation: "update",
		ID:        budgetID,
	})
}

func TestManageBudgetDeleteHappyPath(t *testing.T) {
	session, ledgerID := newTestSession(t)
	budgetID := createTestBudget(t, session, ledgerID, "待删预算")

	var preview tools.ManageBudgetOutput
	callTool(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerID,
		Operation: "delete",
		ID:        budgetID,
	}, &preview)

	if preview.Status != "pending_confirmation" {
		t.Fatalf("Status = %q, want %q", preview.Status, "pending_confirmation")
	}
	if preview.ConfirmationToken == "" {
		t.Fatal("expected a non-empty confirmation_token")
	}
	if preview.ExpiresAt <= time.Now().Unix() {
		t.Fatalf("ExpiresAt = %d, want in the future", preview.ExpiresAt)
	}

	var applied tools.ManageBudgetOutput
	callTool(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:          ledgerID,
		Operation:         "delete",
		ID:                budgetID,
		ConfirmationToken: preview.ConfirmationToken,
	}, &applied)

	if applied.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", applied.Status, "deleted")
	}

	var list tools.ListBudgetsOutput
	callTool(t, session, "list_budgets", tools.ListBudgetsInput{LedgerID: ledgerID}, &list)
	for _, b := range list.Budgets {
		if b.ID == budgetID {
			t.Fatalf("expected budget %q to be deleted, still in list: %+v", budgetID, list.Budgets)
		}
	}
}

func TestManageBudgetDeleteBlockedByReferences(t *testing.T) {
	session, ledgerID, q := newTestSessionWithStore(t)
	budgetIDStr := createTestBudget(t, session, ledgerID, "被引用预算")
	sourceIDStr := createTestSource(t, session, ledgerID)
	categoryIDStr := createTestCategory(t, session, ledgerID, "Food", "")

	lID, _ := strconv.ParseInt(ledgerID, 10, 64)
	sID, _ := strconv.ParseInt(sourceIDStr, 10, 64)
	cID, _ := strconv.ParseInt(categoryIDStr, 10, 64)
	bID, _ := strconv.ParseInt(budgetIDStr, 10, 64)
	now := time.Now().Unix()

	if _, err := q.CreateTransaction(context.Background(), store.CreateTransactionParams{
		LedgerID:   lID,
		Type:       "expense",
		SourceID:   sID,
		CategoryID: cID,
		BudgetID:   bID,
		Currency:   "CNY",
		Amount:     -100,
		Time:       futureTime(),
		CreatedAt:  now,
		UpdatedAt:  now,
	}); err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	callToolExpectError(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerID,
		Operation: "delete",
		ID:        budgetIDStr,
	})
}

func TestListBudgetsRejectsNonexistentLedger(t *testing.T) {
	session, _ := newTestSession(t)
	callToolExpectError(t, session, "list_budgets", tools.ListBudgetsInput{LedgerID: "999999"})
}

func TestManageBudgetCreateRejectsNonexistentLedger(t *testing.T) {
	session, _ := newTestSession(t)
	callToolExpectError(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  "999999",
		Operation: "create",
		Name:      "日常",
	})
}

func TestManageBudgetRejectsCrossLedger(t *testing.T) {
	session, ledgerA := newTestSession(t)
	budgetID := createTestBudget(t, session, ledgerA, "A的预算")

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID

	callToolExpectError(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerB,
		Operation: "update",
		ID:        budgetID,
		Name:      "Hijacked",
	})

	callToolExpectError(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerB,
		Operation: "delete",
		ID:        budgetID,
	})
}
