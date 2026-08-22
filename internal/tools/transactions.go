package tools

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/store"
)

func init() {
	register(registerTransactionTools)
}

func registerTransactionTools(s *mcp.Server, deps Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_transaction",
		Description: "Record one income or expense transaction. The category must be a second-level category (see list_categories); the account's balance is updated accordingly.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in CreateTransactionInput) (*mcp.CallToolResult, CreateTransactionOutput, error) {
		return createTransaction(ctx, deps, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_transaction",
		Description: "Fetch one transaction by id, including its full details.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GetTransactionInput) (*mcp.CallToolResult, GetTransactionOutput, error) {
		return getTransaction(ctx, deps, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_transactions",
		Description: "List transactions, optionally filtered by time range, account, and/or category. With no filters, returns every transaction in the ledger.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in SearchTransactionsInput) (*mcp.CallToolResult, SearchTransactionsOutput, error) {
		return searchTransactions(ctx, deps, in)
	})
}

var createableTransactionTypes = map[string]bool{
	"income":  true,
	"expense": true,
}

// TransactionInfo is the wire representation of a transaction returned by
// create_transaction, get_transaction, and search_transactions.
type TransactionInfo struct {
	ID         string `json:"id" jsonschema:"the transaction's unique id, as a decimal string"`
	Type       string `json:"type" jsonschema:"the transaction's type: income, expense, or balance_adjustment (an account's initial balance, recorded automatically by manage_account)"`
	AccountID  string `json:"account_id" jsonschema:"the id of the account this transaction affects, as a decimal string"`
	CategoryID string `json:"category_id" jsonschema:"the id of the transaction's category, as a decimal string; \"0\" for a balance_adjustment transaction, which has no category"`
	Amount     int64  `json:"amount" jsonschema:"the transaction amount, in the account currency's smallest unit; how many decimal places that represents varies by currency (e.g. 2 for USD/CNY, 0 for JPY, 3 for BHD). Positive for income and expense; balance_adjustment amounts carry their own sign"`
	Currency   string `json:"currency" jsonschema:"the account's currency, as an ISO 4217 code"`
	Time       int64  `json:"time" jsonschema:"when the transaction occurred, as unix seconds"`
	Comment    string `json:"comment,omitempty" jsonschema:"an optional note about the transaction"`
}

type CreateTransactionInput struct {
	Type       string `json:"type" jsonschema:"the transaction's type: income or expense"`
	AccountID  string `json:"account_id" jsonschema:"the id of the account this transaction affects, as a decimal string"`
	CategoryID string `json:"category_id" jsonschema:"the id of the transaction's category (must be a second-level category), as a decimal string"`
	Amount     int64  `json:"amount" jsonschema:"the transaction amount, in the account currency's smallest unit; how many decimal places that represents varies by currency (e.g. 2 for USD/CNY, 0 for JPY, 3 for BHD); must be positive"`
	Time       int64  `json:"time" jsonschema:"when the transaction occurred, as unix seconds"`
	Comment    string `json:"comment,omitempty" jsonschema:"an optional note about the transaction"`
}

type CreateTransactionOutput struct {
	Transaction TransactionInfo `json:"transaction" jsonschema:"the newly recorded transaction"`
}

func createTransaction(ctx context.Context, deps Deps, in CreateTransactionInput) (*mcp.CallToolResult, CreateTransactionOutput, error) {
	if !createableTransactionTypes[in.Type] {
		return nil, CreateTransactionOutput{}, fmt.Errorf("missing or unsupported transaction type: %q", in.Type)
	}

	if in.AccountID == "" {
		return nil, CreateTransactionOutput{}, fmt.Errorf("missing required field: account_id")
	}
	accountID, err := parseID(in.AccountID)
	if err != nil {
		return nil, CreateTransactionOutput{}, err
	}

	if in.CategoryID == "" {
		return nil, CreateTransactionOutput{}, fmt.Errorf("missing required field: category_id")
	}
	categoryID, err := parseID(in.CategoryID)
	if err != nil {
		return nil, CreateTransactionOutput{}, err
	}

	if in.Amount <= 0 {
		return nil, CreateTransactionOutput{}, fmt.Errorf("missing required field: amount (must be positive)")
	}
	if in.Time <= 0 {
		return nil, CreateTransactionOutput{}, fmt.Errorf("missing required field: time")
	}

	account, err := deps.Q.GetAccount(ctx, accountID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, CreateTransactionOutput{}, fmt.Errorf("account %q not found", in.AccountID)
		}
		return nil, CreateTransactionOutput{}, err
	}

	category, err := deps.Q.GetCategory(ctx, categoryID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, CreateTransactionOutput{}, fmt.Errorf("category %q not found", in.CategoryID)
		}
		return nil, CreateTransactionOutput{}, err
	}
	if category.ParentID == topLevelParentID {
		return nil, CreateTransactionOutput{}, fmt.Errorf("category %q is a top-level category; create_transaction requires a second-level category", in.CategoryID)
	}

	// income/expense amounts are stored signed (income positive, expense
	// negative) so SUM(amount) is directly the account balance; the wire
	// format keeps both sides of CreateTransactionInput.Amount /
	// TransactionInfo.Amount positive, matching the tested contract.
	signedAmount := in.Amount
	if in.Type == "expense" {
		signedAmount = -in.Amount
	}

	transaction, err := deps.Q.CreateTransaction(ctx, store.CreateTransactionParams{
		Type:       in.Type,
		AccountID:  accountID,
		CategoryID: sql.NullInt64{Int64: categoryID, Valid: true},
		Amount:     signedAmount,
		Time:       in.Time,
		Comment:    in.Comment,
		CreatedAt:  time.Now().Unix(),
	})
	if err != nil {
		return nil, CreateTransactionOutput{}, err
	}

	return nil, CreateTransactionOutput{Transaction: toTransactionInfo(transaction, account.Currency)}, nil
}

type GetTransactionInput struct {
	ID string `json:"id" jsonschema:"the transaction's unique id, as a decimal string"`
}

type GetTransactionOutput struct {
	Transaction TransactionInfo `json:"transaction" jsonschema:"the requested transaction"`
}

func getTransaction(ctx context.Context, deps Deps, in GetTransactionInput) (*mcp.CallToolResult, GetTransactionOutput, error) {
	if in.ID == "" {
		return nil, GetTransactionOutput{}, fmt.Errorf("missing required field: id")
	}
	id, err := parseID(in.ID)
	if err != nil {
		return nil, GetTransactionOutput{}, err
	}

	transaction, err := deps.Q.GetTransaction(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, GetTransactionOutput{}, fmt.Errorf("transaction %q not found", in.ID)
		}
		return nil, GetTransactionOutput{}, err
	}

	account, err := deps.Q.GetAccount(ctx, transaction.AccountID)
	if err != nil {
		return nil, GetTransactionOutput{}, err
	}

	return nil, GetTransactionOutput{Transaction: toTransactionInfo(transaction, account.Currency)}, nil
}

type SearchTransactionsInput struct {
	AccountID  string `json:"account_id,omitempty" jsonschema:"only include transactions on this account, as a decimal string"`
	CategoryID string `json:"category_id,omitempty" jsonschema:"only include transactions in this category, as a decimal string"`
	StartTime  int64  `json:"start_time,omitempty" jsonschema:"only include transactions at or after this unix time (seconds)"`
	EndTime    int64  `json:"end_time,omitempty" jsonschema:"only include transactions at or before this unix time (seconds)"`
}

type SearchTransactionsOutput struct {
	Transactions []TransactionInfo `json:"transactions" jsonschema:"the matching transactions"`
}

func searchTransactions(ctx context.Context, deps Deps, in SearchTransactionsInput) (*mcp.CallToolResult, SearchTransactionsOutput, error) {
	params := store.SearchTransactionsParams{}

	if in.AccountID != "" {
		id, err := parseID(in.AccountID)
		if err != nil {
			return nil, SearchTransactionsOutput{}, err
		}
		params.AccountID = id
	}
	if in.CategoryID != "" {
		id, err := parseID(in.CategoryID)
		if err != nil {
			return nil, SearchTransactionsOutput{}, err
		}
		params.CategoryID = id
	}
	if in.StartTime > 0 {
		params.StartTime = in.StartTime
	}
	if in.EndTime > 0 {
		params.EndTime = in.EndTime
	}

	transactions, err := deps.Q.SearchTransactions(ctx, params)
	if err != nil {
		return nil, SearchTransactionsOutput{}, err
	}

	infos, err := toTransactionInfos(ctx, deps, transactions)
	if err != nil {
		return nil, SearchTransactionsOutput{}, err
	}

	return nil, SearchTransactionsOutput{Transactions: infos}, nil
}

// toTransactionInfos resolves currencies for a batch of transactions with one
// account lookup per distinct account, instead of one query per transaction.
func toTransactionInfos(ctx context.Context, deps Deps, transactions []store.Transaction) ([]TransactionInfo, error) {
	currencyByAccount := map[int64]string{}
	infos := make([]TransactionInfo, 0, len(transactions))

	for _, t := range transactions {
		curr, ok := currencyByAccount[t.AccountID]
		if !ok {
			account, err := deps.Q.GetAccount(ctx, t.AccountID)
			if err != nil {
				return nil, err
			}
			curr = account.Currency
			currencyByAccount[t.AccountID] = curr
		}
		infos = append(infos, toTransactionInfo(t, curr))
	}

	return infos, nil
}

func toTransactionInfo(t store.Transaction, currencyCode string) TransactionInfo {
	amount := t.Amount
	if t.Type != "balance_adjustment" {
		amount = abs64(amount)
	}

	categoryID := "0"
	if t.CategoryID.Valid {
		categoryID = formatID(t.CategoryID.Int64)
	}

	return TransactionInfo{
		ID:         formatID(t.ID),
		Type:       t.Type,
		AccountID:  formatID(t.AccountID),
		CategoryID: categoryID,
		Amount:     amount,
		Currency:   currencyCode,
		Time:       t.Time,
		Comment:    t.Comment,
	}
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
