package main

import (
	"testing"

	"tally/internal/tools"
)

// TestE2EAccountLifecycle drives two accounts through the account-management
// capabilities added by the account-category-lifecycle change, all through
// main()'s real buildMux wiring.
//
// The two capabilities under test -- update/delete and adjustment --
// use separate accounts here for clarity: per specs/account-management/
// spec.md, an account referenced by any transaction (adjustment
// included) can never be deleted directly. delete_transaction (added by the
// transaction-lifecycle change) can clear that reference first -- see
// TestE2ETransactionLifecycleUnblocksAccountAndCategoryDeletion for that
// path -- but this test keeps the two concerns apart: account A demonstrates
// the update -> preview-delete -> apply-delete chain on an account that
// never records a transaction, and account B demonstrates adjustment
// and is intentionally never deleted.
func TestE2EAccountLifecycle(t *testing.T) {
	session := newE2ESession(t)

	// Account A: create -> update -> preview delete -> apply delete.
	var accountA tools.ManageAccountOutput
	call(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "create",
		Name:      "Checking",
		Type:      "checking_account",
		Currency:  "CNY",
	}, &accountA)

	var updatedA tools.ManageAccountOutput
	call(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "update",
		ID:        accountA.Account.ID,
		Name:      "Checking (renamed)",
		Type:      "savings_account",
		Comment:   "renamed during e2e journey",
	}, &updatedA)
	if updatedA.Status != "updated" {
		t.Fatalf("Status = %q, want %q", updatedA.Status, "updated")
	}

	var afterUpdate tools.ListAccountsOutput
	call(t, session, "list_accounts", tools.ListAccountsInput{}, &afterUpdate)
	if len(afterUpdate.Accounts) != 1 {
		t.Fatalf("expected 1 account (only A exists so far), got %d", len(afterUpdate.Accounts))
	}
	if got := afterUpdate.Accounts[0]; got.Name != "Checking (renamed)" || got.Type != "savings_account" {
		t.Fatalf("account A after update = %+v, want name %q type %q", got, "Checking (renamed)", "savings_account")
	}

	var previewA tools.ManageAccountOutput
	call(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "delete",
		ID:        accountA.Account.ID,
	}, &previewA)
	if previewA.Status != "pending_confirmation" {
		t.Fatalf("Status = %q, want %q", previewA.Status, "pending_confirmation")
	}
	if previewA.ConfirmationToken == "" {
		t.Fatal("expected a non-empty confirmation_token")
	}

	var appliedA tools.ManageAccountOutput
	call(t, session, "manage_account", tools.ManageAccountInput{
		Operation:         "delete",
		ID:                accountA.Account.ID,
		ConfirmationToken: previewA.ConfirmationToken,
	}, &appliedA)
	if appliedA.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", appliedA.Status, "deleted")
	}

	// Account B: create -> record an adjustment -> verify the balance
	// via list_accounts. Never deleted -- the adjustment transaction
	// now references it, which correctly and permanently blocks deletion.
	var accountB tools.ManageAccountOutput
	call(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "create",
		Name:      "Savings",
		Type:      "savings_account",
		Currency:  "CNY",
	}, &accountB)

	var adjustment tools.CreateTransactionOutput
	call(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:      "adjustment",
		AccountID: accountB.Account.ID,
		Amount:    -500,
		Time:      futureTime(),
		Comment:   "correcting an overstated balance",
	}, &adjustment)

	var afterAdjustment tools.ListAccountsOutput
	call(t, session, "list_accounts", tools.ListAccountsInput{}, &afterAdjustment)

	var foundB bool
	for _, a := range afterAdjustment.Accounts {
		if a.ID != accountB.Account.ID {
			continue
		}
		foundB = true
		if a.Balance != -500 {
			t.Fatalf("account B balance after adjustment = %d, want -500", a.Balance)
		}
	}
	if !foundB {
		t.Fatalf("account B (id %q) missing from list_accounts: %+v", accountB.Account.ID, afterAdjustment.Accounts)
	}

	// Account A must remain deleted; only account B should be left.
	if len(afterAdjustment.Accounts) != 1 {
		t.Fatalf("expected only account B to remain, got %d accounts: %+v", len(afterAdjustment.Accounts), afterAdjustment.Accounts)
	}
}
