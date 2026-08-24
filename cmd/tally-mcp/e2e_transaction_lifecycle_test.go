package main

import (
	"testing"

	"tally/internal/tools"
)

// TestE2ETransactionLifecycleUnblocksSourceAndCategoryDeletion is the
// journey the transaction-lifecycle change exists to unblock: before
// delete_transaction existed, a source or category referenced by any
// transaction could never be deleted, because there was no way to clear
// that reference. This test drives a source and a category into exactly
// that blocked state, then uses delete_transaction to clear the referencing
// transaction and confirms manage_source/manage_category's own preview ->
// apply delete now completes successfully. It also drives update_transaction
// through the same real wiring, since no other e2e journey touches it.
func TestE2ETransactionLifecycleUnblocksSourceAndCategoryDeletion(t *testing.T) {
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

	var expense tools.CreateTransactionOutput
	call(t, session, "create_transaction", tools.CreateTransactionInput{LedgerID: ledgerID,
		Type:       "expense",
		SourceID:   source.Source.ID,
		CategoryID: category.Category.ID,
		Amount:     1000,
		Currency:   "CNY",
		Time:       futureTime(),
	}, &expense)

	// update_transaction: full-field replacement (a different amount and
	// comment), still referencing the same source/category. This is the only
	// e2e journey that drives update_transaction through the real wiring.
	var updated tools.UpdateTransactionOutput
	call(t, session, "update_transaction", tools.UpdateTransactionInput{LedgerID: ledgerID,
		ID:         expense.Transaction.ID,
		Type:       "expense",
		SourceID:   source.Source.ID,
		CategoryID: category.Category.ID,
		Amount:     1500,
		Currency:   "CNY",
		Time:       futureTime(),
		Comment:    "updated amount",
	}, &updated)
	if updated.Transaction.Amount != 1500 {
		t.Fatalf("updated amount = %d, want 1500", updated.Transaction.Amount)
	}

	// Both are referenced by the expense right now, so both deletes must be
	// rejected -- this is the deadlock this change fixes.
	callExpectError(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID,
		Operation: "delete",
		ID:        source.Source.ID,
	})
	callExpectError(t, session, "manage_category", tools.ManageCategoryInput{LedgerID: ledgerID,
		Operation: "delete",
		ID:        category.Category.ID,
	})

	// Clear the only reference (the expense) via delete_transaction
	// preview -> apply, then both become deletable.
	var expensePreview tools.DeleteTransactionOutput
	call(t, session, "delete_transaction", tools.DeleteTransactionInput{LedgerID: ledgerID, ID: expense.Transaction.ID}, &expensePreview)
	if expensePreview.Status != "pending_confirmation" {
		t.Fatalf("expense preview Status = %q, want %q", expensePreview.Status, "pending_confirmation")
	}
	var expenseApplied tools.DeleteTransactionOutput
	call(t, session, "delete_transaction", tools.DeleteTransactionInput{LedgerID: ledgerID,
		ID:                expense.Transaction.ID,
		ConfirmationToken: expensePreview.ConfirmationToken,
	}, &expenseApplied)
	if expenseApplied.Status != "deleted" {
		t.Fatalf("expense apply Status = %q, want %q", expenseApplied.Status, "deleted")
	}

	var categoryPreview tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{LedgerID: ledgerID,
		Operation: "delete",
		ID:        category.Category.ID,
	}, &categoryPreview)
	if categoryPreview.Status != "pending_confirmation" {
		t.Fatalf("category delete preview Status = %q, want %q", categoryPreview.Status, "pending_confirmation")
	}
	var categoryApplied tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{LedgerID: ledgerID,
		Operation:         "delete",
		ID:                category.Category.ID,
		ConfirmationToken: categoryPreview.ConfirmationToken,
	}, &categoryApplied)
	if categoryApplied.Status != "deleted" {
		t.Fatalf("category delete apply Status = %q, want %q", categoryApplied.Status, "deleted")
	}

	var categoriesAfter tools.ListCategoriesOutput
	call(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerID}, &categoriesAfter)
	if len(categoriesAfter.Categories) != 0 {
		t.Fatalf("expected the category to be gone, got %d: %+v", len(categoriesAfter.Categories), categoriesAfter.Categories)
	}

	// Every referencing transaction is now gone -- the source's own
	// preview -> apply delete must complete successfully.
	var sourcePreview tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID,
		Operation: "delete",
		ID:        source.Source.ID,
	}, &sourcePreview)
	if sourcePreview.Status != "pending_confirmation" {
		t.Fatalf("source delete preview Status = %q, want %q", sourcePreview.Status, "pending_confirmation")
	}
	var sourceApplied tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID,
		Operation:         "delete",
		ID:                source.Source.ID,
		ConfirmationToken: sourcePreview.ConfirmationToken,
	}, &sourceApplied)
	if sourceApplied.Status != "deleted" {
		t.Fatalf("source delete apply Status = %q, want %q", sourceApplied.Status, "deleted")
	}

	var sourcesAfter tools.ListSourcesOutput
	call(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerID}, &sourcesAfter)
	if len(sourcesAfter.Sources) != 0 {
		t.Fatalf("expected the source to be gone, got %d: %+v", len(sourcesAfter.Sources), sourcesAfter.Sources)
	}
}
