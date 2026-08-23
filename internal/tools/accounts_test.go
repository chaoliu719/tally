package tools_test

import (
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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
		Operation: "create",
		Name:      "Cash Wallet",
		Type:      "cash",
		Currency:  "CNY",
		Balance:   10000,
	}, &created)

	if created.Status != "created" {
		t.Errorf("Status = %q, want %q", created.Status, "created")
	}
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
		Operation: "create",
		Type:      "cash",
		Currency:  "CNY",
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
		Operation: "create",
		Name:      "Bad Currency Account",
		Type:      "cash",
		Currency:  "NOTACURRENCY",
	})

	var list tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &list)
	if len(list.Accounts) != 0 {
		t.Fatalf("expected no account created, got %d", len(list.Accounts))
	}
}

// createTestAccount is a small helper shared by the update/delete tests
// below; it creates an account with the given initial balance and returns
// its id.
func createTestAccount(t *testing.T, session *mcp.ClientSession, balance int64) string {
	t.Helper()

	var created tools.ManageAccountOutput
	callTool(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "create",
		Name:      "Cash Wallet",
		Type:      "cash",
		Currency:  "CNY",
		Balance:   balance,
	}, &created)

	return created.Account.ID
}

func TestManageAccountUpdateHappyPath(t *testing.T) {
	session := newTestSession(t)
	accountID := createTestAccount(t, session, 10000)

	var updated tools.ManageAccountOutput
	callTool(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "update",
		ID:        accountID,
		Name:      "Renamed Wallet",
		Type:      "checking_account",
		Comment:   "moved to a bank",
	}, &updated)

	if updated.Status != "updated" {
		t.Errorf("Status = %q, want %q", updated.Status, "updated")
	}
	if updated.Account.Name != "Renamed Wallet" {
		t.Errorf("Name = %q, want %q", updated.Account.Name, "Renamed Wallet")
	}
	if updated.Account.Type != "checking_account" {
		t.Errorf("Type = %q, want %q", updated.Account.Type, "checking_account")
	}
	if updated.Account.Balance != 10000 {
		t.Errorf("Balance = %d, want unchanged 10000", updated.Account.Balance)
	}

	var list tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &list)
	if list.Accounts[0].Name != "Renamed Wallet" {
		t.Errorf("listed name = %q, want %q", list.Accounts[0].Name, "Renamed Wallet")
	}
	if list.Accounts[0].Type != "checking_account" {
		t.Errorf("listed type = %q, want %q", list.Accounts[0].Type, "checking_account")
	}
}

func TestManageAccountUpdateNotFound(t *testing.T) {
	session := newTestSession(t)

	callToolExpectError(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "update",
		ID:        "999999",
		Name:      "Ghost",
		Type:      "cash",
	})
}

func TestManageAccountUpdateMissingRequiredField(t *testing.T) {
	session := newTestSession(t)
	accountID := createTestAccount(t, session, 10000)

	callToolExpectError(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "update",
		ID:        accountID,
		Type:      "checking_account",
		// Name omitted
	})

	var list tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &list)
	if list.Accounts[0].Type != "cash" {
		t.Fatalf("account was modified despite missing required field: type = %q", list.Accounts[0].Type)
	}
}

func TestManageAccountUpdateRejectsCurrency(t *testing.T) {
	session := newTestSession(t)
	accountID := createTestAccount(t, session, 10000)

	callToolExpectError(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "update",
		ID:        accountID,
		Name:      "Cash Wallet",
		Type:      "cash",
		Currency:  "USD",
	})

	var list tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &list)
	if list.Accounts[0].Currency != "CNY" {
		t.Fatalf("currency changed despite update rejecting it: %q", list.Accounts[0].Currency)
	}
}

func TestManageAccountUpdateRejectsBalance(t *testing.T) {
	session := newTestSession(t)
	accountID := createTestAccount(t, session, 10000)

	callToolExpectError(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "update",
		ID:        accountID,
		Name:      "Cash Wallet",
		Type:      "cash",
		Balance:   5000,
	})

	var list tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &list)
	if list.Accounts[0].Balance != 10000 {
		t.Fatalf("balance changed despite update rejecting it: %d", list.Accounts[0].Balance)
	}
}

func TestManageAccountDeleteHappyPath(t *testing.T) {
	session := newTestSession(t)
	accountID := createTestAccount(t, session, 0)

	var preview tools.ManageAccountOutput
	callTool(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "delete",
		ID:        accountID,
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

	var list tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &list)
	if len(list.Accounts) != 1 {
		t.Fatalf("preview must not delete the account; got %d accounts", len(list.Accounts))
	}

	var applied tools.ManageAccountOutput
	callTool(t, session, "manage_account", tools.ManageAccountInput{
		Operation:         "delete",
		ID:                accountID,
		ConfirmationToken: preview.ConfirmationToken,
	}, &applied)

	if applied.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", applied.Status, "deleted")
	}

	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &list)
	if len(list.Accounts) != 0 {
		t.Fatalf("expected the account to be gone, got %d accounts", len(list.Accounts))
	}
}

func TestManageAccountDeleteBlockedByReferences(t *testing.T) {
	session := newTestSession(t)
	// A nonzero initial balance records a balance_adjustment transaction,
	// which alone should be enough to block deletion.
	accountID := createTestAccount(t, session, 10000)

	callToolExpectError(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "delete",
		ID:        accountID,
	})

	var list tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &list)
	if len(list.Accounts) != 1 {
		t.Fatalf("expected the account to survive a blocked delete, got %d accounts", len(list.Accounts))
	}
}

func TestManageAccountDeleteNotFound(t *testing.T) {
	session := newTestSession(t)

	callToolExpectError(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "delete",
		ID:        "999999",
	})
}

func TestManageAccountDeleteExpiredToken(t *testing.T) {
	session := newTestSession(t)
	accountID := createTestAccount(t, session, 0)

	expired := craftConfirmationToken(t, testConfirmSecret, "delete_account", accountID, "irrelevant-revision", time.Now().Add(-time.Minute).Unix())

	callToolExpectError(t, session, "manage_account", tools.ManageAccountInput{
		Operation:         "delete",
		ID:                accountID,
		ConfirmationToken: expired,
	})

	var list tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &list)
	if len(list.Accounts) != 1 {
		t.Fatalf("expired token must not delete the account, got %d accounts", len(list.Accounts))
	}
}

func TestManageAccountDeleteDriftedRevision(t *testing.T) {
	session := newTestSession(t)
	accountID := createTestAccount(t, session, 0)

	var preview tools.ManageAccountOutput
	callTool(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "delete",
		ID:        accountID,
	}, &preview)

	// The account's state changes after the preview but before apply -- the
	// stale token must be rejected rather than deleting a different account
	// than the one previewed.
	callTool(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "update",
		ID:        accountID,
		Name:      "Renamed After Preview",
		Type:      "cash",
	}, &tools.ManageAccountOutput{})

	callToolExpectError(t, session, "manage_account", tools.ManageAccountInput{
		Operation:         "delete",
		ID:                accountID,
		ConfirmationToken: preview.ConfirmationToken,
	})

	var list tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &list)
	if len(list.Accounts) != 1 {
		t.Fatalf("drifted-revision token must not delete the account, got %d accounts", len(list.Accounts))
	}
}
