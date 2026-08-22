package tools_test

import (
	"testing"

	"tally/internal/tools"
)

func TestListAccountsEmptyLedger(t *testing.T) {
	session := newTestSession(t)

	var out tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &out)

	if out.Accounts == nil {
		t.Fatal("expected an empty slice, got nil")
	}
	if len(out.Accounts) != 0 {
		t.Fatalf("expected 0 accounts, got %d", len(out.Accounts))
	}
}

func TestManageAccountCreate(t *testing.T) {
	session := newTestSession(t)

	var created tools.ManageAccountOutput
	callTool(t, session, "manage_account", tools.ManageAccountInput{
		Name:     "Cash Wallet",
		Type:     "cash",
		Currency: "CNY",
		Balance:  10000,
	}, &created)

	if created.Account.Name != "Cash Wallet" {
		t.Errorf("Name = %q, want %q", created.Account.Name, "Cash Wallet")
	}
	if created.Account.Balance != 10000 {
		t.Errorf("Balance = %d, want 10000", created.Account.Balance)
	}

	var list tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &list)

	if len(list.Accounts) != 1 {
		t.Fatalf("expected 1 account after creation, got %d", len(list.Accounts))
	}
	if list.Accounts[0].ID != created.Account.ID {
		t.Errorf("listed account id = %q, want %q", list.Accounts[0].ID, created.Account.ID)
	}
	if list.Accounts[0].Balance != 10000 {
		t.Errorf("listed account balance = %d, want 10000", list.Accounts[0].Balance)
	}
}

func TestManageAccountMissingRequiredField(t *testing.T) {
	session := newTestSession(t)

	callToolExpectError(t, session, "manage_account", tools.ManageAccountInput{
		Type:     "cash",
		Currency: "CNY",
	})

	var list tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &list)
	if len(list.Accounts) != 0 {
		t.Fatalf("expected no account created, got %d", len(list.Accounts))
	}
}

func TestManageAccountUnsupportedCurrency(t *testing.T) {
	session := newTestSession(t)

	callToolExpectError(t, session, "manage_account", tools.ManageAccountInput{
		Name:     "Bad Currency Account",
		Type:     "cash",
		Currency: "NOTACURRENCY",
	})

	var list tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &list)
	if len(list.Accounts) != 0 {
		t.Fatalf("expected no account created, got %d", len(list.Accounts))
	}
}
