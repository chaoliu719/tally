package main

import (
	"testing"

	"tally/internal/tools"
)

func TestE2EBudgetLifecycle(t *testing.T) {
	session, ledgerID := newE2ESession(t)

	// Clear any initial budgets in ledger A to test empty list behaviour first.
	var initial tools.ListBudgetsOutput
	call(t, session, "list_budgets", tools.ListBudgetsInput{LedgerID: ledgerID}, &initial)
	for _, b := range initial.Budgets {
		var prev tools.ManageBudgetOutput
		call(t, session, "manage_budget", tools.ManageBudgetInput{
			LedgerID:  ledgerID,
			Operation: "delete",
			ID:        b.ID,
		}, &prev)
		call(t, session, "manage_budget", tools.ManageBudgetInput{
			LedgerID:          ledgerID,
			Operation:         "delete",
			ID:                b.ID,
			ConfirmationToken: prev.ConfirmationToken,
		}, &tools.ManageBudgetOutput{})
	}

	var emptyList tools.ListBudgetsOutput
	call(t, session, "list_budgets", tools.ListBudgetsInput{LedgerID: ledgerID}, &emptyList)
	if emptyList.Budgets == nil || len(emptyList.Budgets) != 0 {
		t.Fatalf("expected empty budgets slice, got %+v", emptyList.Budgets)
	}

	// Budget A: create -> update -> preview delete -> apply delete.
	var budgetA tools.ManageBudgetOutput
	call(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      "专项",
	}, &budgetA)

	var updatedA tools.ManageBudgetOutput
	call(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerID,
		Operation: "update",
		ID:        budgetA.Budget.ID,
		Name:      "专项(改名)",
	}, &updatedA)
	if updatedA.Status != "updated" {
		t.Fatalf("Status = %q, want %q", updatedA.Status, "updated")
	}

	var afterUpdate tools.ListBudgetsOutput
	call(t, session, "list_budgets", tools.ListBudgetsInput{LedgerID: ledgerID}, &afterUpdate)
	if len(afterUpdate.Budgets) != 1 || afterUpdate.Budgets[0].Name != "专项(改名)" {
		t.Fatalf("list_budgets after update = %+v, want [专项(改名)]", afterUpdate.Budgets)
	}

	// Cross-ledger rejection.
	var ledgerBOut tools.ManageLedgerOutput
	call(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "create",
		Name:      "Other Ledger",
	}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID

	callExpectError(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerB,
		Operation: "update",
		ID:        budgetA.Budget.ID,
		Name:      "Cross Ledger Hijack",
	})
	callExpectError(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerB,
		Operation: "delete",
		ID:        budgetA.Budget.ID,
	})

	// Delete Budget A.
	var previewA tools.ManageBudgetOutput
	call(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerID,
		Operation: "delete",
		ID:        budgetA.Budget.ID,
	}, &previewA)
	if previewA.Status != "pending_confirmation" || previewA.ConfirmationToken == "" {
		t.Fatalf("unexpected preview output: %+v", previewA)
	}

	var appliedA tools.ManageBudgetOutput
	call(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:          ledgerID,
		Operation:         "delete",
		ID:                budgetA.Budget.ID,
		ConfirmationToken: previewA.ConfirmationToken,
	}, &appliedA)
	if appliedA.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", appliedA.Status, "deleted")
	}

	// Budget B: create -> referenced by transaction -> cannot be deleted.
	var budgetB tools.ManageBudgetOutput
	call(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      "日常",
	}, &budgetB)

	var source tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      "Savings",
	}, &source)

	var category tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      "Misc",
	}, &category)

	var expense tools.CreateTransactionOutput
	call(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerID,
		Type:       "expense",
		SourceID:   source.Source.ID,
		CategoryID: category.Category.ID,
		BudgetID:   budgetB.Budget.ID,
		Amount:     cnyAmount(500),
		Currency:   "CNY",
		Time:       futureTime(),
		Comment:    "keeps budget B referenced",
	}, &expense)

	callExpectError(t, session, "manage_budget", tools.ManageBudgetInput{
		LedgerID:  ledgerID,
		Operation: "delete",
		ID:        budgetB.Budget.ID,
	})

	var afterExpense tools.ListBudgetsOutput
	call(t, session, "list_budgets", tools.ListBudgetsInput{LedgerID: ledgerID}, &afterExpense)
	if len(afterExpense.Budgets) != 1 || afterExpense.Budgets[0].ID != budgetB.Budget.ID {
		t.Fatalf("expected only budget B to remain, got %+v", afterExpense.Budgets)
	}
}
