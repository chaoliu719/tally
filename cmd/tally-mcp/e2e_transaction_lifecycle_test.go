package main

import (
	"testing"

	"tally/internal/tools"
)

// TestE2ETransactionLifecycleUnblocksAccountAndCategoryDeletion is the
// journey the transaction-lifecycle change exists to unblock: before
// delete_transaction existed, an account or category referenced by any
// transaction (adjustment included) could never be deleted, because
// there was no way to clear that reference. This test drives an account and
// a category into exactly that blocked state, then uses delete_transaction
// to clear every referencing transaction and confirms manage_account/
// manage_category's own preview -> apply delete now completes successfully.
func TestE2ETransactionLifecycleUnblocksAccountAndCategoryDeletion(t *testing.T) {
	session := newE2ESession(t)

	var account tools.ManageAccountOutput
	call(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "create",
		Name:      "Checking",
		Type:      "checking_account",
		Currency:  "CNY",
		Balance:   5000, // records an initial adjustment transaction
	}, &account)

	var category tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Groceries",
	}, &category)

	var expense tools.CreateTransactionOutput
	call(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		AccountID:  account.Account.ID,
		CategoryID: category.Category.ID,
		Amount:     1000,
		Time:       futureTime(),
	}, &expense)

	// Both are referenced by transactions right now, so both deletes must be
	// rejected -- this is the deadlock this change fixes.
	callExpectError(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "delete",
		ID:        account.Account.ID,
	})
	callExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "delete",
		ID:        category.Category.ID,
	})

	// Clear the category's only reference (the expense) via delete_transaction
	// preview -> apply, then the category becomes deletable.
	var expensePreview tools.DeleteTransactionOutput
	call(t, session, "delete_transaction", tools.DeleteTransactionInput{ID: expense.Transaction.ID}, &expensePreview)
	if expensePreview.Status != "pending_confirmation" {
		t.Fatalf("expense preview Status = %q, want %q", expensePreview.Status, "pending_confirmation")
	}
	var expenseApplied tools.DeleteTransactionOutput
	call(t, session, "delete_transaction", tools.DeleteTransactionInput{
		ID:                expense.Transaction.ID,
		ConfirmationToken: expensePreview.ConfirmationToken,
	}, &expenseApplied)
	if expenseApplied.Status != "deleted" {
		t.Fatalf("expense apply Status = %q, want %q", expenseApplied.Status, "deleted")
	}

	var categoryPreview tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "delete",
		ID:        category.Category.ID,
	}, &categoryPreview)
	if categoryPreview.Status != "pending_confirmation" {
		t.Fatalf("category delete preview Status = %q, want %q", categoryPreview.Status, "pending_confirmation")
	}
	var categoryApplied tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{
		Operation:         "delete",
		ID:                category.Category.ID,
		ConfirmationToken: categoryPreview.ConfirmationToken,
	}, &categoryApplied)
	if categoryApplied.Status != "deleted" {
		t.Fatalf("category delete apply Status = %q, want %q", categoryApplied.Status, "deleted")
	}

	var categoriesAfter tools.ListCategoriesOutput
	call(t, session, "list_categories", tools.ListCategoriesInput{}, &categoriesAfter)
	if len(categoriesAfter.Categories) != 0 {
		t.Fatalf("expected the category to be gone, got %d: %+v", len(categoriesAfter.Categories), categoriesAfter.Categories)
	}

	// The account still has its initial adjustment transaction, so it
	// remains blocked until that is cleared too.
	callExpectError(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "delete",
		ID:        account.Account.ID,
	})

	var remaining tools.SearchTransactionsOutput
	call(t, session, "search_transactions", tools.SearchTransactionsInput{AccountID: account.Account.ID}, &remaining)
	if len(remaining.Transactions) != 1 {
		t.Fatalf("expected only the initial adjustment left on the account, got %d: %+v", len(remaining.Transactions), remaining.Transactions)
	}
	adjustmentID := remaining.Transactions[0].ID

	var adjustmentPreview tools.DeleteTransactionOutput
	call(t, session, "delete_transaction", tools.DeleteTransactionInput{ID: adjustmentID}, &adjustmentPreview)
	var adjustmentApplied tools.DeleteTransactionOutput
	call(t, session, "delete_transaction", tools.DeleteTransactionInput{
		ID:                adjustmentID,
		ConfirmationToken: adjustmentPreview.ConfirmationToken,
	}, &adjustmentApplied)
	if adjustmentApplied.Status != "deleted" {
		t.Fatalf("adjustment apply Status = %q, want %q", adjustmentApplied.Status, "deleted")
	}

	// Every referencing transaction is now gone -- the account's own
	// preview -> apply delete must complete successfully.
	var accountPreview tools.ManageAccountOutput
	call(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "delete",
		ID:        account.Account.ID,
	}, &accountPreview)
	if accountPreview.Status != "pending_confirmation" {
		t.Fatalf("account delete preview Status = %q, want %q", accountPreview.Status, "pending_confirmation")
	}
	var accountApplied tools.ManageAccountOutput
	call(t, session, "manage_account", tools.ManageAccountInput{
		Operation:         "delete",
		ID:                account.Account.ID,
		ConfirmationToken: accountPreview.ConfirmationToken,
	}, &accountApplied)
	if accountApplied.Status != "deleted" {
		t.Fatalf("account delete apply Status = %q, want %q", accountApplied.Status, "deleted")
	}

	var accountsAfter tools.ListAccountsOutput
	call(t, session, "list_accounts", tools.ListAccountsInput{}, &accountsAfter)
	if len(accountsAfter.Accounts) != 0 {
		t.Fatalf("expected the account to be gone, got %d: %+v", len(accountsAfter.Accounts), accountsAfter.Accounts)
	}
}
