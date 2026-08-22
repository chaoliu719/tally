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
// and a usable (second-level) expense category, returning their ids.
func setupAccountAndCategory(t *testing.T, session *mcp.ClientSession, initialBalance int64) (accountID, categoryID string) {
	t.Helper()

	var account tools.ManageAccountOutput
	callTool(t, session, "manage_account", tools.ManageAccountInput{
		Name:     "Cash Wallet",
		Type:     "cash",
		Currency: "CNY",
		Balance:  initialBalance,
	}, &account)

	var parent tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Name: "Food",
		Type: "expense",
	}, &parent)

	var child tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Name:     "Groceries",
		Type:     "expense",
		ParentID: parent.Category.ID,
	}, &child)

	return account.Account.ID, child.Category.ID
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

	// Reuse the expense category id as-is: the type of the transaction just
	// needs a valid second-level category id; create an income category too
	// for a semantically correct test.
	var parent tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Name: "Salary",
		Type: "income",
	}, &parent)
	var incomeCategory tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Name:     "Monthly Salary",
		Type:     "income",
		ParentID: parent.Category.ID,
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

func TestCreateTransactionRejectsPrimaryCategory(t *testing.T) {
	session := newTestSession(t)
	accountID, _ := setupAccountAndCategory(t, session, 10000)

	var primary tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Name: "Housing",
		Type: "expense",
	}, &primary)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		AccountID:  accountID,
		CategoryID: primary.Category.ID,
		Amount:     100,
		Time:       futureTime(),
	})

	var accounts tools.ListAccountsOutput
	callTool(t, session, "list_accounts", tools.ListAccountsInput{}, &accounts)
	if accounts.Accounts[0].Balance != 10000 {
		t.Fatalf("balance changed after rejected transaction: %d, want 10000", accounts.Accounts[0].Balance)
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
