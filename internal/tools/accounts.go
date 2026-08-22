package tools

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/currency"
	"tally/internal/store"
)

func init() {
	register(registerAccountTools)
}

func registerAccountTools(s *mcp.Server, deps Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_accounts",
		Description: "List every account in the ledger, including its name, type, currency, and current balance.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ ListAccountsInput) (*mcp.CallToolResult, ListAccountsOutput, error) {
		return listAccounts(ctx, deps)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "manage_account",
		Description: "Create a new account in the ledger.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ManageAccountInput) (*mcp.CallToolResult, ManageAccountOutput, error) {
		return manageAccount(ctx, deps, in)
	})
}

// accountTypes is the set of account types tally accepts. It's a plain TEXT
// column in the schema (see design.md) -- validation happens here in Go,
// not as a database CHECK constraint.
var accountTypes = map[string]bool{
	"cash":                   true,
	"checking_account":       true,
	"credit_card":            true,
	"virtual":                true,
	"debt":                   true,
	"receivables":            true,
	"investment":             true,
	"savings_account":        true,
	"certificate_of_deposit": true,
}

// AccountInfo is the wire representation of an account returned by
// list_accounts and manage_account.
type AccountInfo struct {
	ID       string `json:"id" jsonschema:"the account's unique id, as a decimal string"`
	Name     string `json:"name" jsonschema:"the account's name"`
	Type     string `json:"type" jsonschema:"the account's type, e.g. cash, checking_account, credit_card"`
	Currency string `json:"currency" jsonschema:"the account's currency, as an ISO 4217 code, e.g. CNY, USD"`
	Balance  int64  `json:"balance" jsonschema:"the account's current balance, in the currency's smallest unit; how many decimal places that represents varies by currency (e.g. 2 for USD/CNY, 0 for JPY, 3 for BHD)"`
}

type ListAccountsInput struct{}

type ListAccountsOutput struct {
	Accounts []AccountInfo `json:"accounts" jsonschema:"every account in the ledger"`
}

func listAccounts(ctx context.Context, deps Deps) (*mcp.CallToolResult, ListAccountsOutput, error) {
	rows, err := deps.Q.ListAccounts(ctx)
	if err != nil {
		return nil, ListAccountsOutput{}, err
	}

	infos := make([]AccountInfo, 0, len(rows))
	for _, a := range rows {
		infos = append(infos, AccountInfo{
			ID:       formatID(a.ID),
			Name:     a.Name,
			Type:     a.Type,
			Currency: a.Currency,
			Balance:  a.Balance,
		})
	}

	return nil, ListAccountsOutput{Accounts: infos}, nil
}

type ManageAccountInput struct {
	Name     string `json:"name" jsonschema:"the account's name"`
	Type     string `json:"type" jsonschema:"the account's type: cash, checking_account, credit_card, virtual, debt, receivables, investment, savings_account, or certificate_of_deposit"`
	Currency string `json:"currency" jsonschema:"the account's currency, as an ISO 4217 code, e.g. CNY, USD"`
	Balance  int64  `json:"balance,omitempty" jsonschema:"the account's initial balance, in the currency's smallest unit; how many decimal places that represents varies by currency (e.g. 2 for USD/CNY, 0 for JPY, 3 for BHD); defaults to 0"`
	Comment  string `json:"comment,omitempty" jsonschema:"an optional note about the account"`
}

type ManageAccountOutput struct {
	Account AccountInfo `json:"account" jsonschema:"the newly created account"`
}

func manageAccount(ctx context.Context, deps Deps, in ManageAccountInput) (*mcp.CallToolResult, ManageAccountOutput, error) {
	if in.Name == "" {
		return nil, ManageAccountOutput{}, fmt.Errorf("missing required field: name")
	}
	if !accountTypes[in.Type] {
		return nil, ManageAccountOutput{}, fmt.Errorf("missing or unsupported account type: %q", in.Type)
	}
	if in.Currency == "" {
		return nil, ManageAccountOutput{}, fmt.Errorf("missing required field: currency")
	}
	if !currency.Supported(in.Currency) {
		return nil, ManageAccountOutput{}, fmt.Errorf("unsupported currency: %q", in.Currency)
	}

	tx, err := deps.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, ManageAccountOutput{}, err
	}
	defer tx.Rollback()

	q := deps.Q.WithTx(tx)
	now := time.Now().Unix()

	account, err := q.CreateAccount(ctx, store.CreateAccountParams{
		Name:      in.Name,
		Type:      in.Type,
		Currency:  in.Currency,
		Comment:   in.Comment,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return nil, ManageAccountOutput{}, err
	}

	// A nonzero initial balance is recorded as a balance_adjustment
	// transaction in the same DB transaction as the account row, so a
	// concurrent list_accounts can never observe the account with the
	// account's balance (computed as SUM(amount)) still at zero.
	if in.Balance != 0 {
		if _, err := q.CreateTransaction(ctx, store.CreateTransactionParams{
			Type:       "balance_adjustment",
			AccountID:  account.ID,
			CategoryID: sql.NullInt64{},
			Amount:     in.Balance,
			Time:       now,
			CreatedAt:  now,
		}); err != nil {
			return nil, ManageAccountOutput{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, ManageAccountOutput{}, err
	}

	return nil, ManageAccountOutput{Account: AccountInfo{
		ID:       formatID(account.ID),
		Name:     account.Name,
		Type:     account.Type,
		Currency: account.Currency,
		Balance:  in.Balance,
	}}, nil
}
