package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/bootstrap"
	"tally/internal/tools"
)

func futureTime() int64 {
	return time.Now().Add(time.Hour).Unix()
}

// TestEndToEndMinimalLoop drives the exact loop described in proposal.md,
// starting from a brand-new, empty SQLite file: start the server -> create
// an account -> create a (two-level) category -> record an expense -> find
// it via search -> fetch it via get, with the account balance deducted.
// Every step goes through a real MCP tool call over authenticated HTTP,
// using the same buildMux wiring main() uses -- nothing touches the
// database directly.
func TestEndToEndMinimalLoop(t *testing.T) {
	const token = "e2e-test-token"

	dbPath := filepath.Join(t.TempDir(), "tally-e2e.db")
	cfg := &bootstrap.Config{
		MCPToken:           token,
		ConfirmationSecret: "e2e-test-confirmation-secret",
		DBPath:             dbPath,
	}

	db, err := bootstrap.InitDataStore(cfg)
	if err != nil {
		t.Fatalf("InitDataStore failed: %v", err)
	}
	defer db.Close()

	mux := buildMux(cfg, db)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-client", Version: "1.0.0"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint: httpServer.URL + "/mcp",
		HTTPClient: &http.Client{
			Transport: &bearerRoundTripper{token: token, base: http.DefaultTransport},
		},
	}

	ctx := context.Background()
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("initialize handshake failed: %v", err)
	}
	defer session.Close()

	call := func(name string, args any, out any) {
		t.Helper()
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("CallTool(%s) protocol error: %v", name, err)
		}
		if res.IsError {
			var text string
			for _, c := range res.Content {
				if tc, ok := c.(*mcp.TextContent); ok {
					text += tc.Text
				}
			}
			t.Fatalf("CallTool(%s) tool error: %s", name, text)
		}
		if out != nil {
			raw, err := json.Marshal(res.StructuredContent)
			if err != nil {
				t.Fatalf("marshal structured content: %v", err)
			}
			if err := json.Unmarshal(raw, out); err != nil {
				t.Fatalf("unmarshal into %T: %v", out, err)
			}
		}
	}

	// 1. manage_account: create the account.
	var account tools.ManageAccountOutput
	call("manage_account", tools.ManageAccountInput{
		Operation: "create",
		Name:      "Checking",
		Type:      "checking_account",
		Currency:  "CNY",
		Balance:   50000,
	}, &account)

	// 2. manage_category: create a top-level category, then a second-level
	// one under it (the only kind create_transaction can reference).
	var topCategory tools.ManageCategoryOutput
	call("manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Food",
	}, &topCategory)

	var category tools.ManageCategoryOutput
	call("manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Groceries",
		ParentID:  topCategory.Category.ID,
	}, &category)

	// 3. create_transaction: record one expense.
	var transaction tools.CreateTransactionOutput
	call("create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		AccountID:  account.Account.ID,
		CategoryID: category.Category.ID,
		Amount:     3000,
		Time:       futureTime(),
		Comment:    "weekly groceries",
	}, &transaction)

	// 4. search_transactions: find both the expense and the balance_adjustment
	// that manage_account's nonzero initial balance produced -- neither is
	// hidden from search (see design.md's "交易可见性" decision).
	var searchResult tools.SearchTransactionsOutput
	call("search_transactions", tools.SearchTransactionsInput{}, &searchResult)
	if len(searchResult.Transactions) != 2 {
		t.Fatalf("expected 2 transactions from search, got %d", len(searchResult.Transactions))
	}

	var sawExpense, sawBalanceAdjustment bool
	for _, txn := range searchResult.Transactions {
		switch {
		case txn.ID == transaction.Transaction.ID:
			sawExpense = true
		case txn.Type == "balance_adjustment":
			sawBalanceAdjustment = true
		}
	}
	if !sawExpense {
		t.Fatalf("search result missing the recorded expense (id %q): %+v", transaction.Transaction.ID, searchResult.Transactions)
	}
	if !sawBalanceAdjustment {
		t.Fatalf("search result missing the account-creation balance_adjustment: %+v", searchResult.Transactions)
	}

	// 5. get_transaction: fetch it by id, and confirm the account balance
	// was deducted.
	var fetched tools.GetTransactionOutput
	call("get_transaction", tools.GetTransactionInput{ID: transaction.Transaction.ID}, &fetched)
	if fetched.Transaction.Amount != 3000 {
		t.Fatalf("fetched amount = %d, want 3000", fetched.Transaction.Amount)
	}

	var accounts tools.ListAccountsOutput
	call("list_accounts", tools.ListAccountsInput{}, &accounts)
	if len(accounts.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts.Accounts))
	}
	const wantBalance = 50000 - 3000
	if accounts.Accounts[0].Balance != wantBalance {
		t.Fatalf("balance after expense = %d, want %d", accounts.Accounts[0].Balance, wantBalance)
	}
}
