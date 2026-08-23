package tools_test

import (
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/tools"
)

func futureTime() int64 {
	return time.Now().Add(time.Hour).Unix()
}

// setupAccountAndCategory creates an account with the given initial balance
// and a usable category, returning their ids. Categories no longer have a
// level restriction, so a plain top-level category works fine here.
func setupAccountAndCategory(t *testing.T, session *mcp.ClientSession, initialBalance int64) (accountID, categoryID string) {
	t.Helper()

	var account tools.ManageAccountOutput
	callTool(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "create",
		Name:      "Cash Wallet",
		Type:      "cash",
		Currency:  "CNY",
		Balance:   initialBalance,
	}, &account)

	var category tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Groceries",
	}, &category)

	return account.Account.ID, category.Category.ID
}

func TestCreateTransactionExpenseUpdatesBalance(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		AccountID:  accountID,
		CategoryID: categoryID,
		Amount:     2500,
		Time:       futureTime(),
		Comment:    "groceries run",
	}, &created)

	if created.Transaction.Amount != 2500 {
		t.Errorf("Amount = %d, want 2500", created.Transaction.Amount)
	}
	if created.Transaction.Currency != "CNY" {
		t.Errorf("Currency = %q, want CNY", created.Transaction.Currency)
	}

	var got tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{ID: created.Transaction.ID}, &got)
	if got.Transaction.ID != created.Transaction.ID {
		t.Errorf("get_transaction id = %q, want %q", got.Transaction.ID, created.Transaction.ID)
	}

	var accounts tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &accounts)
	if len(accounts.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts.Accounts))
	}
	if accounts.Accounts[0].Balance != 10000-2500 {
		t.Errorf("balance after expense = %d, want %d", accounts.Accounts[0].Balance, 10000-2500)
	}
}

func TestCreateTransactionIncomeUpdatesBalance(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	var incomeCategory tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Salary",
	}, &incomeCategory)

	_ = categoryID // expense category unused in this test

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "income",
		AccountID:  accountID,
		CategoryID: incomeCategory.Category.ID,
		Amount:     5000,
		Time:       futureTime(),
	}, &tools.CreateTransactionOutput{})

	var accounts tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &accounts)
	if accounts.Accounts[0].Balance != 10000+5000 {
		t.Errorf("balance after income = %d, want %d", accounts.Accounts[0].Balance, 10000+5000)
	}
}

func TestCreateTransactionRejectsNonexistentAccount(t *testing.T) {
	session := newTestSession(t)
	_, categoryID := setupAccountAndCategory(t, session, 10000)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		AccountID:  "999999",
		CategoryID: categoryID,
		Amount:     100,
		Time:       futureTime(),
	})

	// The only transaction on record is the balance_adjustment that
	// setupAccountAndCategory's initial balance produced -- the rejected
	// expense above must not have been written.
	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{}, &list)
	if len(list.Transactions) != 1 {
		t.Fatalf("expected only the initial balance_adjustment transaction, got %d", len(list.Transactions))
	}
	if list.Transactions[0].Type != "balance_adjustment" {
		t.Fatalf("expected the recorded transaction to be balance_adjustment, got %q", list.Transactions[0].Type)
	}
}

func TestCreateTransactionRejectsNonexistentCategory(t *testing.T) {
	session := newTestSession(t)
	accountID, _ := setupAccountAndCategory(t, session, 10000)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		AccountID:  accountID,
		CategoryID: "999999",
		Amount:     100,
		Time:       futureTime(),
	})

	var accounts tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &accounts)
	if accounts.Accounts[0].Balance != 10000 {
		t.Fatalf("balance changed after rejected transaction: %d, want 10000", accounts.Accounts[0].Balance)
	}
}

// TestCreateTransactionAllowsTopLevelCategory verifies the former "must be a
// second-level category" restriction is gone: a plain top-level category
// (no parent) can now be referenced directly by create_transaction.
func TestCreateTransactionAllowsTopLevelCategory(t *testing.T) {
	session := newTestSession(t)
	accountID, _ := setupAccountAndCategory(t, session, 10000)

	var topLevel tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Housing",
	}, &topLevel)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		AccountID:  accountID,
		CategoryID: topLevel.Category.ID,
		Amount:     100,
		Time:       futureTime(),
	}, &tools.CreateTransactionOutput{})

	var accounts tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &accounts)
	if accounts.Accounts[0].Balance != 10000-100 {
		t.Fatalf("balance after expense against a top-level category = %d, want %d", accounts.Accounts[0].Balance, 10000-100)
	}
}

func TestCreateTransactionMissingRequiredField(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:      "expense",
		AccountID: accountID,
		// CategoryID omitted
		Amount: 100,
		Time:   futureTime(),
	})
	_ = categoryID

	var accounts tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &accounts)
	if accounts.Accounts[0].Balance != 10000 {
		t.Fatalf("balance changed after rejected transaction: %d, want 10000", accounts.Accounts[0].Balance)
	}
}

func TestCreateTransactionBalanceAdjustmentPositive(t *testing.T) {
	session := newTestSession(t)
	accountID, _ := setupAccountAndCategory(t, session, 10000)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:      "balance_adjustment",
		AccountID: accountID,
		Amount:    500,
		Time:      futureTime(),
	}, &created)

	if created.Transaction.Amount != 500 {
		t.Errorf("Amount = %d, want 500", created.Transaction.Amount)
	}
	if created.Transaction.CategoryID != "0" {
		t.Errorf("CategoryID = %q, want %q", created.Transaction.CategoryID, "0")
	}

	var accounts tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &accounts)
	if accounts.Accounts[0].Balance != 10000+500 {
		t.Errorf("balance after positive adjustment = %d, want %d", accounts.Accounts[0].Balance, 10000+500)
	}
}

func TestCreateTransactionBalanceAdjustmentNegative(t *testing.T) {
	session := newTestSession(t)
	accountID, _ := setupAccountAndCategory(t, session, 10000)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:      "balance_adjustment",
		AccountID: accountID,
		Amount:    -500,
		Time:      futureTime(),
	}, &tools.CreateTransactionOutput{})

	var accounts tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &accounts)
	if accounts.Accounts[0].Balance != 10000-500 {
		t.Errorf("balance after negative adjustment = %d, want %d", accounts.Accounts[0].Balance, 10000-500)
	}
}

func TestCreateTransactionBalanceAdjustmentRejectsCategory(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "balance_adjustment",
		AccountID:  accountID,
		CategoryID: categoryID,
		Amount:     500,
		Time:       futureTime(),
	})

	var accounts tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &accounts)
	if accounts.Accounts[0].Balance != 10000 {
		t.Fatalf("balance changed after rejected transaction: %d, want 10000", accounts.Accounts[0].Balance)
	}
}

func TestCreateTransactionBalanceAdjustmentRejectsZeroAmount(t *testing.T) {
	session := newTestSession(t)
	accountID, _ := setupAccountAndCategory(t, session, 10000)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:      "balance_adjustment",
		AccountID: accountID,
		Amount:    0,
		Time:      futureTime(),
	})

	var accounts tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &accounts)
	if accounts.Accounts[0].Balance != 10000 {
		t.Fatalf("balance changed after rejected transaction: %d, want 10000", accounts.Accounts[0].Balance)
	}
}

func TestCreateTransactionBalanceAdjustmentRejectsNonexistentAccount(t *testing.T) {
	session := newTestSession(t)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:      "balance_adjustment",
		AccountID: "999999",
		Amount:    500,
		Time:      futureTime(),
	})
}

func TestGetTransactionNotFound(t *testing.T) {
	session := newTestSession(t)

	callToolExpectError(t, session, "get_transaction", tools.GetTransactionInput{ID: "999999"})
}

func TestGetTransactionBalanceAdjustment(t *testing.T) {
	session := newTestSession(t)
	accountID, _ := setupAccountAndCategory(t, session, 10000)

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{}, &list)
	if len(list.Transactions) != 1 {
		t.Fatalf("expected 1 transaction (the initial balance_adjustment), got %d", len(list.Transactions))
	}
	adjustmentID := list.Transactions[0].ID

	var got tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{ID: adjustmentID}, &got)
	if got.Transaction.Type != "balance_adjustment" {
		t.Fatalf("Type = %q, want balance_adjustment", got.Transaction.Type)
	}
	if got.Transaction.AccountID != accountID {
		t.Fatalf("AccountID = %q, want %q", got.Transaction.AccountID, accountID)
	}
	if got.Transaction.Amount != 10000 {
		t.Fatalf("Amount = %d, want 10000", got.Transaction.Amount)
	}
}

func TestSearchTransactionsNoFilter(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 100, Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 200, Time: futureTime() + 3600,
	}, &tools.CreateTransactionOutput{})

	// 2 expenses plus the balance_adjustment that setupAccountAndCategory's
	// initial balance produced -- no filter means no hiding either.
	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{}, &out)
	if len(out.Transactions) != 3 {
		t.Fatalf("expected 3 transactions, got %d", len(out.Transactions))
	}

	var sawBalanceAdjustment bool
	for _, txn := range out.Transactions {
		if txn.Type == "balance_adjustment" {
			sawBalanceAdjustment = true
		}
	}
	if !sawBalanceAdjustment {
		t.Fatal("expected the account's initial balance_adjustment transaction to be searchable")
	}
}

func TestSearchTransactionsTimeRange(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	earlyTime := futureTime()
	lateTime := earlyTime + 100000

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 100, Time: earlyTime,
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 200, Time: lateTime,
	}, &tools.CreateTransactionOutput{})

	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{
		StartTime: earlyTime - 10,
		EndTime:   earlyTime + 10,
	}, &out)

	if len(out.Transactions) != 1 {
		t.Fatalf("expected 1 transaction in range, got %d", len(out.Transactions))
	}
	if out.Transactions[0].Amount != 100 {
		t.Errorf("Amount = %d, want 100", out.Transactions[0].Amount)
	}
}

// balanceOf returns the balance of the account with the given id from a
// list_accounts result, failing the test if it's not present.
func balanceOf(t *testing.T, accounts tools.ListAccountsOutput, accountID string) int64 {
	t.Helper()
	for _, a := range accounts.Accounts {
		if a.ID == accountID {
			return a.Balance
		}
	}
	t.Fatalf("account %q not found in %+v", accountID, accounts.Accounts)
	return 0
}

func TestUpdateTransactionHappyPath(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		AccountID:  accountID,
		CategoryID: categoryID,
		Amount:     2500,
		Time:       futureTime(),
		Comment:    "original",
	}, &created)

	var otherCategory tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Dining",
	}, &otherCategory)

	var updated tools.UpdateTransactionOutput
	callTool(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		Type:       "expense",
		AccountID:  accountID,
		CategoryID: otherCategory.Category.ID,
		Amount:     4000,
		Time:       futureTime(),
		Comment:    "revised",
	}, &updated)

	if updated.Transaction.Amount != 4000 {
		t.Errorf("Amount = %d, want 4000", updated.Transaction.Amount)
	}
	if updated.Transaction.CategoryID != otherCategory.Category.ID {
		t.Errorf("CategoryID = %q, want %q", updated.Transaction.CategoryID, otherCategory.Category.ID)
	}
	if updated.Transaction.Comment != "revised" {
		t.Errorf("Comment = %q, want %q", updated.Transaction.Comment, "revised")
	}

	var got tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{ID: created.Transaction.ID}, &got)
	if got.Transaction.Amount != 4000 {
		t.Errorf("get_transaction Amount = %d, want 4000", got.Transaction.Amount)
	}

	var accounts tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &accounts)
	if got := balanceOf(t, accounts, accountID); got != 10000-4000 {
		t.Errorf("balance after update = %d, want %d", got, 10000-4000)
	}
}

func TestUpdateTransactionChangesAccount(t *testing.T) {
	session := newTestSession(t)
	accountA, categoryID := setupAccountAndCategory(t, session, 10000)

	var accountB tools.ManageAccountOutput
	callTool(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "create",
		Name:      "Second Wallet",
		Type:      "cash",
		Currency:  "CNY",
		Balance:   5000,
	}, &accountB)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		AccountID:  accountA,
		CategoryID: categoryID,
		Amount:     2000,
		Time:       futureTime(),
	}, &created)

	callTool(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		Type:       "expense",
		AccountID:  accountB.Account.ID,
		CategoryID: categoryID,
		Amount:     2000,
		Time:       futureTime(),
	}, &tools.UpdateTransactionOutput{})

	var accounts tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &accounts)

	if got := balanceOf(t, accounts, accountA); got != 10000 {
		t.Errorf("account A balance after moving its expense away = %d, want %d (expense no longer counted)", got, 10000)
	}
	if got := balanceOf(t, accounts, accountB.Account.ID); got != 5000-2000 {
		t.Errorf("account B balance after receiving the moved expense = %d, want %d", got, 5000-2000)
	}
}

// TestUpdateTransactionChangesTypeUpdatesBalance verifies that changing a
// transaction's type (here income -> expense on the same amount) is
// reflected in the account's balance, which is always SUM(amount) computed
// at query time rather than a stored field.
func TestUpdateTransactionChangesTypeUpdatesBalance(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "income",
		AccountID:  accountID,
		CategoryID: categoryID,
		Amount:     1000,
		Time:       futureTime(),
	}, &created)

	var accountsAfterCreate tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &accountsAfterCreate)
	if got := balanceOf(t, accountsAfterCreate, accountID); got != 10000+1000 {
		t.Fatalf("balance after income = %d, want %d", got, 10000+1000)
	}

	callTool(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		Type:       "expense",
		AccountID:  accountID,
		CategoryID: categoryID,
		Amount:     1000,
		Time:       futureTime(),
	}, &tools.UpdateTransactionOutput{})

	var accountsAfterUpdate tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &accountsAfterUpdate)
	if got := balanceOf(t, accountsAfterUpdate, accountID); got != 10000-1000 {
		t.Fatalf("balance after changing income to expense = %d, want %d", got, 10000-1000)
	}
}

func TestUpdateTransactionMissingRequiredField(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 100, Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:   created.Transaction.ID,
		Type: "expense",
		// AccountID omitted
		CategoryID: categoryID,
		Amount:     100,
		Time:       futureTime(),
	})

	var got tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{ID: created.Transaction.ID}, &got)
	if got.Transaction.Amount != 100 {
		t.Fatalf("transaction changed after rejected update: Amount = %d, want 100", got.Transaction.Amount)
	}
}

func TestUpdateTransactionRejectsNonexistentAccount(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 100, Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		Type:       "expense",
		AccountID:  "999999",
		CategoryID: categoryID,
		Amount:     100,
		Time:       futureTime(),
	})
}

func TestUpdateTransactionRejectsNonexistentCategory(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 100, Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		Type:       "expense",
		AccountID:  accountID,
		CategoryID: "999999",
		Amount:     100,
		Time:       futureTime(),
	})
}

func TestUpdateTransactionBalanceAdjustmentRejectsCategory(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "balance_adjustment", AccountID: accountID, Amount: 300, Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		Type:       "balance_adjustment",
		AccountID:  accountID,
		CategoryID: categoryID,
		Amount:     300,
		Time:       futureTime(),
	})
}

func TestUpdateTransactionBalanceAdjustmentRejectsZeroAmount(t *testing.T) {
	session := newTestSession(t)
	accountID, _ := setupAccountAndCategory(t, session, 10000)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "balance_adjustment", AccountID: accountID, Amount: 300, Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:        created.Transaction.ID,
		Type:      "balance_adjustment",
		AccountID: accountID,
		Amount:    0,
		Time:      futureTime(),
	})
}

func TestUpdateTransactionIncomeExpenseRejectsNonPositiveAmount(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 100, Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		Type:       "expense",
		AccountID:  accountID,
		CategoryID: categoryID,
		Amount:     0,
		Time:       futureTime(),
	})
}

func TestUpdateTransactionNotFound(t *testing.T) {
	session := newTestSession(t)
	_, categoryID := setupAccountAndCategory(t, session, 10000)
	accountID, _ := setupAccountAndCategory(t, session, 10000)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         "999999",
		Type:       "expense",
		AccountID:  accountID,
		CategoryID: categoryID,
		Amount:     100,
		Time:       futureTime(),
	})
}

func TestDeleteTransactionHappyPath(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 2500, Time: futureTime(),
	}, &created)

	var preview tools.DeleteTransactionOutput
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{ID: created.Transaction.ID}, &preview)
	if preview.Status != "pending_confirmation" {
		t.Fatalf("Status = %q, want %q", preview.Status, "pending_confirmation")
	}
	if preview.ConfirmationToken == "" {
		t.Fatal("expected a non-empty confirmation_token")
	}

	// Preview must not delete anything.
	var stillThere tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{ID: created.Transaction.ID}, &stillThere)

	var applied tools.DeleteTransactionOutput
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{
		ID:                created.Transaction.ID,
		ConfirmationToken: preview.ConfirmationToken,
	}, &applied)
	if applied.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", applied.Status, "deleted")
	}

	callToolExpectError(t, session, "get_transaction", tools.GetTransactionInput{ID: created.Transaction.ID})

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{}, &list)
	for _, txn := range list.Transactions {
		if txn.ID == created.Transaction.ID {
			t.Fatalf("deleted transaction %q still present in search_transactions", created.Transaction.ID)
		}
	}

	var accounts tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &accounts)
	if got := balanceOf(t, accounts, accountID); got != 10000 {
		t.Errorf("balance after deleting the expense = %d, want %d", got, 10000)
	}
}

func TestDeleteTransactionNotFound(t *testing.T) {
	session := newTestSession(t)

	callToolExpectError(t, session, "delete_transaction", tools.DeleteTransactionInput{ID: "999999"})
	callToolExpectError(t, session, "delete_transaction", tools.DeleteTransactionInput{ID: "999999", ConfirmationToken: "irrelevant"})
}

func TestDeleteTransactionExpiredToken(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 100, Time: futureTime(),
	}, &created)

	expired := craftConfirmationToken(t, testConfirmSecret, "delete_transaction", created.Transaction.ID, "irrelevant-revision", time.Now().Add(-time.Minute).Unix())

	callToolExpectError(t, session, "delete_transaction", tools.DeleteTransactionInput{
		ID:                created.Transaction.ID,
		ConfirmationToken: expired,
	})

	// The transaction must survive the expired token.
	callTool(t, session, "get_transaction", tools.GetTransactionInput{ID: created.Transaction.ID}, &tools.GetTransactionOutput{})
}

func TestDeleteTransactionDriftedRevisionAfterUpdate(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 100, Time: futureTime(),
	}, &created)

	var preview tools.DeleteTransactionOutput
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{ID: created.Transaction.ID}, &preview)

	// The transaction changes after the preview but before apply -- the stale
	// token must be rejected rather than deleting a transaction whose state no
	// longer matches what was previewed.
	callTool(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		Type:       "expense",
		AccountID:  accountID,
		CategoryID: categoryID,
		Amount:     200,
		Time:       futureTime(),
	}, &tools.UpdateTransactionOutput{})

	callToolExpectError(t, session, "delete_transaction", tools.DeleteTransactionInput{
		ID:                created.Transaction.ID,
		ConfirmationToken: preview.ConfirmationToken,
	})

	callTool(t, session, "get_transaction", tools.GetTransactionInput{ID: created.Transaction.ID}, &tools.GetTransactionOutput{})
}

func TestDeleteTransactionTokenReplay(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 100, Time: futureTime(),
	}, &created)

	var preview tools.DeleteTransactionOutput
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{ID: created.Transaction.ID}, &preview)

	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{
		ID:                created.Transaction.ID,
		ConfirmationToken: preview.ConfirmationToken,
	}, &tools.DeleteTransactionOutput{})

	// Reusing the same token to confirm the same delete a second time must
	// fail -- the transaction is already gone.
	callToolExpectError(t, session, "delete_transaction", tools.DeleteTransactionInput{
		ID:                created.Transaction.ID,
		ConfirmationToken: preview.ConfirmationToken,
	})
}

func TestSearchTransactionsEmptyResult(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 100, Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{
		StartTime: futureTime() + 1000000,
		EndTime:   futureTime() + 1100000,
	}, &out)

	if out.Transactions == nil {
		t.Fatal("expected an empty slice, got nil")
	}
	if len(out.Transactions) != 0 {
		t.Fatalf("expected 0 transactions, got %d", len(out.Transactions))
	}
}
