package tools

import (
	"context"
	"fmt"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// searchResultLimit bounds how many transactions search_transactions returns
// in one call. v1 doesn't expose pagination; this is large enough to be
// effectively "all transactions" for a personal ledger.
const searchResultLimit = 100000

func init() {
	register(registerTransactionTools)
}

func registerTransactionTools(s *mcp.Server, deps Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_transaction",
		Description: "Record one income or expense transaction. The category must be a second-level category (see list_categories); the account's balance is updated accordingly.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in CreateTransactionInput) (*mcp.CallToolResult, CreateTransactionOutput, error) {
		return createTransaction(deps, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_transaction",
		Description: "Fetch one transaction by id, including its full details.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in GetTransactionInput) (*mcp.CallToolResult, GetTransactionOutput, error) {
		return getTransaction(deps, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_transactions",
		Description: "List transactions, optionally filtered by time range, account, and/or category. With no filters, returns every transaction in the ledger.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in SearchTransactionsInput) (*mcp.CallToolResult, SearchTransactionsOutput, error) {
		return searchTransactions(deps, in)
	})
}

var transactionTypeByName = map[string]models.TransactionType{
	"income":  models.TRANSACTION_TYPE_INCOME,
	"expense": models.TRANSACTION_TYPE_EXPENSE,
}

var transactionDbTypeNames = map[models.TransactionDbType]string{
	models.TRANSACTION_DB_TYPE_MODIFY_BALANCE: "modify_balance",
	models.TRANSACTION_DB_TYPE_INCOME:         "income",
	models.TRANSACTION_DB_TYPE_EXPENSE:        "expense",
	models.TRANSACTION_DB_TYPE_TRANSFER_OUT:   "transfer_out",
	models.TRANSACTION_DB_TYPE_TRANSFER_IN:    "transfer_in",
}

// TransactionInfo is the wire representation of a transaction returned by
// create_transaction, get_transaction, and search_transactions.
type TransactionInfo struct {
	ID         string `json:"id" jsonschema:"the transaction's unique id, as a decimal string"`
	Type       string `json:"type" jsonschema:"the transaction's type: income or expense"`
	AccountID  string `json:"account_id" jsonschema:"the id of the account this transaction affects, as a decimal string"`
	CategoryID string `json:"category_id" jsonschema:"the id of the transaction's category, as a decimal string"`
	Amount     int64  `json:"amount" jsonschema:"the transaction amount, in the account currency's smallest unit (e.g. cents)"`
	Currency   string `json:"currency" jsonschema:"the account's currency, as an ISO 4217 code"`
	Time       int64  `json:"time" jsonschema:"when the transaction occurred, as unix seconds"`
	Comment    string `json:"comment,omitempty" jsonschema:"an optional note about the transaction"`
}

type CreateTransactionInput struct {
	Type       string `json:"type" jsonschema:"the transaction's type: income or expense"`
	AccountID  string `json:"account_id" jsonschema:"the id of the account this transaction affects, as a decimal string"`
	CategoryID string `json:"category_id" jsonschema:"the id of the transaction's category (must be a second-level category), as a decimal string"`
	Amount     int64  `json:"amount" jsonschema:"the transaction amount, in the account currency's smallest unit (e.g. cents); must be positive"`
	Time       int64  `json:"time" jsonschema:"when the transaction occurred, as unix seconds"`
	Comment    string `json:"comment,omitempty" jsonschema:"an optional note about the transaction"`
}

type CreateTransactionOutput struct {
	Transaction TransactionInfo `json:"transaction" jsonschema:"the newly recorded transaction"`
}

func createTransaction(deps Deps, in CreateTransactionInput) (*mcp.CallToolResult, CreateTransactionOutput, error) {
	txType, ok := transactionTypeByName[in.Type]
	if !ok {
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

	dbType, err := txType.ToTransactionDbType()
	if err != nil {
		return nil, CreateTransactionOutput{}, err
	}

	ctx := core.NewNullContext()

	// Look up the account first so we can (a) reject a nonexistent account
	// with a clear error before touching anything else, and (b) report the
	// account's currency alongside the created transaction.
	account, err := services.Accounts.GetAccountByAccountId(ctx, deps.UID, accountID)
	if err != nil {
		return nil, CreateTransactionOutput{}, err
	}

	transaction := &models.Transaction{
		Uid:             deps.UID,
		Type:            dbType,
		CategoryId:      categoryID,
		TransactionTime: utils.GetMinTransactionTimeFromUnixTime(in.Time),
		AccountId:       accountID,
		Amount:          in.Amount,
		Comment:         in.Comment,
	}

	// CreateTransaction validates the category itself (existence, and that
	// it's a second-level category) via ezbookkeeping's own isCategoryValid;
	// we don't duplicate that check here.
	if err := services.Transactions.CreateTransaction(ctx, transaction, nil, nil); err != nil {
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

func getTransaction(deps Deps, in GetTransactionInput) (*mcp.CallToolResult, GetTransactionOutput, error) {
	if in.ID == "" {
		return nil, GetTransactionOutput{}, fmt.Errorf("missing required field: id")
	}
	id, err := parseID(in.ID)
	if err != nil {
		return nil, GetTransactionOutput{}, err
	}

	ctx := core.NewNullContext()

	transaction, err := services.Transactions.GetTransactionByTransactionId(ctx, deps.UID, id)
	if err != nil {
		return nil, GetTransactionOutput{}, err
	}

	currency, err := lookupCurrency(ctx, deps.UID, transaction.AccountId)
	if err != nil {
		return nil, GetTransactionOutput{}, err
	}

	return nil, GetTransactionOutput{Transaction: toTransactionInfo(transaction, currency)}, nil
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

func searchTransactions(deps Deps, in SearchTransactionsInput) (*mcp.CallToolResult, SearchTransactionsOutput, error) {
	var accountIds []int64
	if in.AccountID != "" {
		id, err := parseID(in.AccountID)
		if err != nil {
			return nil, SearchTransactionsOutput{}, err
		}
		accountIds = []int64{id}
	}

	var categoryIds []int64
	if in.CategoryID != "" {
		id, err := parseID(in.CategoryID)
		if err != nil {
			return nil, SearchTransactionsOutput{}, err
		}
		categoryIds = []int64{id}
	}

	var maxTime, minTime int64
	if in.EndTime > 0 {
		maxTime = utils.GetMaxTransactionTimeFromUnixTime(in.EndTime)
	}
	if in.StartTime > 0 {
		minTime = utils.GetMinTransactionTimeFromUnixTime(in.StartTime)
	}

	ctx := core.NewNullContext()

	transactions, err := services.Transactions.GetTransactionsByMaxTime(
		ctx, deps.UID, maxTime, minTime, 0, categoryIds, accountIds, nil, false,
		"", "", core.MATCH_MODE_DEFAULT, false, 1, searchResultLimit, false, true,
	)
	if err != nil {
		return nil, SearchTransactionsOutput{}, err
	}

	// Accounts get an implicit "modify balance" transaction when created with
	// a nonzero initial balance; that's bookkeeping plumbing, not something a
	// caller created via create_transaction, so it's excluded here.
	transactions = filterIncomeAndExpense(transactions)

	infos, err := toTransactionInfos(ctx, deps.UID, transactions)
	if err != nil {
		return nil, SearchTransactionsOutput{}, err
	}

	return nil, SearchTransactionsOutput{Transactions: infos}, nil
}

func filterIncomeAndExpense(transactions []*models.Transaction) []*models.Transaction {
	filtered := make([]*models.Transaction, 0, len(transactions))
	for _, t := range transactions {
		if t.Type == models.TRANSACTION_DB_TYPE_INCOME || t.Type == models.TRANSACTION_DB_TYPE_EXPENSE {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func lookupCurrency(ctx core.Context, uid int64, accountID int64) (string, error) {
	account, err := services.Accounts.GetAccountByAccountId(ctx, uid, accountID)
	if err != nil {
		return "", err
	}
	return account.Currency, nil
}

// toTransactionInfos resolves currencies for a batch of transactions with a
// single account lookup, instead of one query per transaction.
func toTransactionInfos(ctx core.Context, uid int64, transactions []*models.Transaction) ([]TransactionInfo, error) {
	accountIds := make([]int64, 0, len(transactions))
	seen := make(map[int64]bool, len(transactions))
	for _, t := range transactions {
		if !seen[t.AccountId] {
			seen[t.AccountId] = true
			accountIds = append(accountIds, t.AccountId)
		}
	}

	accountsByID := map[int64]*models.Account{}
	if len(accountIds) > 0 {
		var err error
		accountsByID, err = services.Accounts.GetAccountsByAccountIds(ctx, uid, accountIds)
		if err != nil {
			return nil, err
		}
	}

	infos := make([]TransactionInfo, 0, len(transactions))
	for _, t := range transactions {
		currency := ""
		if account, ok := accountsByID[t.AccountId]; ok {
			currency = account.Currency
		}
		infos = append(infos, toTransactionInfo(t, currency))
	}

	return infos, nil
}

func toTransactionInfo(t *models.Transaction, currency string) TransactionInfo {
	return TransactionInfo{
		ID:         formatID(t.TransactionId),
		Type:       transactionDbTypeNames[t.Type],
		AccountID:  formatID(t.AccountId),
		CategoryID: formatID(t.CategoryId),
		Amount:     t.Amount,
		Currency:   currency,
		Time:       utils.GetUnixTimeFromTransactionTime(t.TransactionTime),
		Comment:    t.Comment,
	}
}
