package tools

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/confirm"
	"tally/internal/store"
)

func init() {
	register(registerTransactionTools)
}

func registerTransactionTools(s *mcp.Server, deps Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_transaction",
		Description: "Record one income, expense, or balance_adjustment transaction; the account's balance is updated accordingly. income/expense require an existing category_id (any category in the ledger). balance_adjustment is the formal way to correct an account's balance: it takes a signed, nonzero amount and must not have a category_id.",
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

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_transaction",
		Description: "Replace a transaction's fields (type, account, category, amount, time, comment) by id. This is a full replacement, not a partial update -- all fields must be provided, and the same income/expense/balance_adjustment validation rules as create_transaction apply. Does not require confirmation.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateTransactionInput) (*mcp.CallToolResult, UpdateTransactionOutput, error) {
		return updateTransaction(ctx, deps, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "delete_transaction",
		Description: "Delete a transaction by id. This is a two-step preview -> apply: call without confirmation_token to preview " +
			"(returns the transaction and a token), then call again with the returned confirmation_token to actually delete. " +
			"Unlike account/category deletion, any existing transaction can be deleted -- there is no reference-count gate.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in DeleteTransactionInput) (*mcp.CallToolResult, DeleteTransactionOutput, error) {
		return deleteTransaction(ctx, deps, in)
	})
}

var createableTransactionTypes = map[string]bool{
	"income":             true,
	"expense":            true,
	"balance_adjustment": true,
}

// TransactionInfo is the wire representation of a transaction returned by
// create_transaction, get_transaction, and search_transactions.
type TransactionInfo struct {
	ID         string `json:"id" jsonschema:"the transaction's unique id, as a decimal string"`
	Type       string `json:"type" jsonschema:"the transaction's type: income, expense, or balance_adjustment"`
	AccountID  string `json:"account_id" jsonschema:"the id of the account this transaction affects, as a decimal string"`
	CategoryID string `json:"category_id" jsonschema:"the id of the transaction's category, as a decimal string; \"0\" for a balance_adjustment transaction, which has no category"`
	Amount     int64  `json:"amount" jsonschema:"the transaction amount, in the account currency's smallest unit; how many decimal places that represents varies by currency (e.g. 2 for USD/CNY, 0 for JPY, 3 for BHD). Positive for income and expense; balance_adjustment amounts carry their own sign"`
	Currency   string `json:"currency" jsonschema:"the account's currency, as an ISO 4217 code"`
	Time       int64  `json:"time" jsonschema:"when the transaction occurred, as unix seconds"`
	Comment    string `json:"comment,omitempty" jsonschema:"an optional note about the transaction"`
}

type CreateTransactionInput struct {
	Type       string `json:"type" jsonschema:"the transaction's type: income, expense, or balance_adjustment"`
	AccountID  string `json:"account_id" jsonschema:"the id of the account this transaction affects, as a decimal string"`
	CategoryID string `json:"category_id,omitempty" jsonschema:"the id of the transaction's category, as a decimal string; required for income/expense (any existing category), must be omitted for balance_adjustment"`
	Amount     int64  `json:"amount" jsonschema:"the transaction amount, in the account currency's smallest unit; how many decimal places that represents varies by currency (e.g. 2 for USD/CNY, 0 for JPY, 3 for BHD); must be positive for income/expense, nonzero (positive or negative) for balance_adjustment"`
	Time       int64  `json:"time" jsonschema:"when the transaction occurred, as unix seconds"`
	Comment    string `json:"comment,omitempty" jsonschema:"an optional note about the transaction"`
}

type CreateTransactionOutput struct {
	Transaction TransactionInfo `json:"transaction" jsonschema:"the newly recorded transaction"`
}

// validatedTransactionFields is the normalized, validated form of the fields
// shared by create_transaction and update_transaction: the account id, the
// resolved category_id (NULL for balance_adjustment), and the signed amount
// to store (income/expense amounts are stored signed -- income positive,
// expense negative -- so SUM(amount) is directly the account balance; the
// wire format keeps both sides of *Amount positive for income/expense).
type validatedTransactionFields struct {
	AccountID  int64
	CategoryID sql.NullInt64
	Amount     int64
}

// validateTransactionInput checks and normalizes the type/account_id/
// category_id/amount/time rules shared by create_transaction and
// update_transaction: income/expense require an existing category_id and a
// positive amount; balance_adjustment forbids a category_id and requires a
// nonzero amount. It returns the resolved account (for its currency)
// alongside the normalized fields.
func validateTransactionInput(ctx context.Context, deps Deps, txType, accountIDStr, categoryIDStr string, amount, txTime int64) (store.Account, validatedTransactionFields, error) {
	if !createableTransactionTypes[txType] {
		return store.Account{}, validatedTransactionFields{}, fmt.Errorf("missing or unsupported transaction type: %q", txType)
	}

	if accountIDStr == "" {
		return store.Account{}, validatedTransactionFields{}, fmt.Errorf("missing required field: account_id")
	}
	accountID, err := parseID(accountIDStr)
	if err != nil {
		return store.Account{}, validatedTransactionFields{}, err
	}

	if txTime <= 0 {
		return store.Account{}, validatedTransactionFields{}, fmt.Errorf("missing required field: time")
	}

	account, err := deps.Q.GetAccount(ctx, accountID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.Account{}, validatedTransactionFields{}, fmt.Errorf("account %q not found", accountIDStr)
		}
		return store.Account{}, validatedTransactionFields{}, err
	}

	var categoryID sql.NullInt64
	var signedAmount int64

	if txType == "balance_adjustment" {
		if categoryIDStr != "" {
			return store.Account{}, validatedTransactionFields{}, fmt.Errorf("balance_adjustment transactions cannot specify a category_id")
		}
		if amount == 0 {
			return store.Account{}, validatedTransactionFields{}, fmt.Errorf("missing required field: amount (must be nonzero for balance_adjustment)")
		}
		signedAmount = amount
	} else {
		if categoryIDStr == "" {
			return store.Account{}, validatedTransactionFields{}, fmt.Errorf("missing required field: category_id")
		}
		catID, err := parseID(categoryIDStr)
		if err != nil {
			return store.Account{}, validatedTransactionFields{}, err
		}
		if _, err := deps.Q.GetCategory(ctx, catID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return store.Account{}, validatedTransactionFields{}, fmt.Errorf("category %q not found", categoryIDStr)
			}
			return store.Account{}, validatedTransactionFields{}, err
		}
		categoryID = sql.NullInt64{Int64: catID, Valid: true}

		if amount <= 0 {
			return store.Account{}, validatedTransactionFields{}, fmt.Errorf("missing required field: amount (must be positive)")
		}
		signedAmount = amount
		if txType == "expense" {
			signedAmount = -amount
		}
	}

	return account, validatedTransactionFields{AccountID: accountID, CategoryID: categoryID, Amount: signedAmount}, nil
}

func createTransaction(ctx context.Context, deps Deps, in CreateTransactionInput) (*mcp.CallToolResult, CreateTransactionOutput, error) {
	account, fields, err := validateTransactionInput(ctx, deps, in.Type, in.AccountID, in.CategoryID, in.Amount, in.Time)
	if err != nil {
		return nil, CreateTransactionOutput{}, err
	}

	now := time.Now().Unix()
	transaction, err := deps.Q.CreateTransaction(ctx, store.CreateTransactionParams{
		Type:       in.Type,
		AccountID:  fields.AccountID,
		CategoryID: fields.CategoryID,
		Amount:     fields.Amount,
		Time:       in.Time,
		Comment:    in.Comment,
		CreatedAt:  now,
		UpdatedAt:  now,
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

type UpdateTransactionInput struct {
	ID         string `json:"id" jsonschema:"the transaction's unique id, as a decimal string"`
	Type       string `json:"type" jsonschema:"the transaction's type: income, expense, or balance_adjustment"`
	AccountID  string `json:"account_id" jsonschema:"the id of the account this transaction affects, as a decimal string; may differ from the transaction's current account_id to move it to another account"`
	CategoryID string `json:"category_id,omitempty" jsonschema:"the id of the transaction's category, as a decimal string; required for income/expense (any existing category), must be omitted for balance_adjustment"`
	Amount     int64  `json:"amount" jsonschema:"the transaction amount, in the account currency's smallest unit; how many decimal places that represents varies by currency (e.g. 2 for USD/CNY, 0 for JPY, 3 for BHD); must be positive for income/expense, nonzero (positive or negative) for balance_adjustment"`
	Time       int64  `json:"time" jsonschema:"when the transaction occurred, as unix seconds"`
	Comment    string `json:"comment,omitempty" jsonschema:"an optional note about the transaction"`
}

type UpdateTransactionOutput struct {
	Transaction TransactionInfo `json:"transaction" jsonschema:"the transaction's updated info"`
}

// updateTransaction replaces every mutable field of an existing transaction.
// This is full-field-replacement semantics (like manage_account/
// manage_category's operation=update), reusing the exact validation rules
// create_transaction applies -- see validateTransactionInput.
func updateTransaction(ctx context.Context, deps Deps, in UpdateTransactionInput) (*mcp.CallToolResult, UpdateTransactionOutput, error) {
	if in.ID == "" {
		return nil, UpdateTransactionOutput{}, fmt.Errorf("missing required field: id")
	}
	id, err := parseID(in.ID)
	if err != nil {
		return nil, UpdateTransactionOutput{}, err
	}

	if _, err := deps.Q.GetTransaction(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, UpdateTransactionOutput{}, fmt.Errorf("transaction %q not found", in.ID)
		}
		return nil, UpdateTransactionOutput{}, err
	}

	account, fields, err := validateTransactionInput(ctx, deps, in.Type, in.AccountID, in.CategoryID, in.Amount, in.Time)
	if err != nil {
		return nil, UpdateTransactionOutput{}, err
	}

	updated, err := deps.Q.UpdateTransaction(ctx, store.UpdateTransactionParams{
		Type:       in.Type,
		AccountID:  fields.AccountID,
		CategoryID: fields.CategoryID,
		Amount:     fields.Amount,
		Time:       in.Time,
		Comment:    in.Comment,
		UpdatedAt:  time.Now().Unix(),
		ID:         id,
	})
	if err != nil {
		return nil, UpdateTransactionOutput{}, err
	}

	return nil, UpdateTransactionOutput{Transaction: toTransactionInfo(updated, account.Currency)}, nil
}

// transactionDeletionRevisionFields is hashed to produce the revision
// embedded in a delete_transaction confirmation token (see design.md's
// "delete_transaction:走 preview → apply"). Unlike account/category
// deletion, there is no reference-count gate -- every field here exists
// purely to detect that the transaction was modified or replaced since the
// preview, not to decide whether deletion is allowed.
type transactionDeletionRevisionFields struct {
	Type       string
	AccountID  int64
	CategoryID sql.NullInt64
	Amount     int64
	Time       int64
	Comment    string
}

func transactionDeletionRevision(t store.Transaction) string {
	fields := transactionDeletionRevisionFields{
		Type:       t.Type,
		AccountID:  t.AccountID,
		CategoryID: t.CategoryID,
		Amount:     t.Amount,
		Time:       t.Time,
		Comment:    t.Comment,
	}
	// Marshaling a fixed struct of plain strings/ints never fails.
	body, _ := json.Marshal(fields)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

const confirmActionDeleteTransaction = "delete_transaction"

type DeleteTransactionInput struct {
	ID                string `json:"id" jsonschema:"the transaction's unique id, as a decimal string"`
	ConfirmationToken string `json:"confirmation_token,omitempty" jsonschema:"omit to preview the deletion and receive a token, or supply the token from a prior preview to actually delete"`
}

type DeleteTransactionOutput struct {
	Transaction       TransactionInfo `json:"transaction" jsonschema:"the transaction's info (or, for a completed delete, its last known info)"`
	Status            string          `json:"status" jsonschema:"pending_confirmation or deleted"`
	ConfirmationToken string          `json:"confirmation_token,omitempty" jsonschema:"present when status is pending_confirmation; pass this back as confirmation_token to actually delete"`
	ExpiresAt         int64           `json:"expires_at,omitempty" jsonschema:"unix seconds when confirmation_token expires; present when status is pending_confirmation"`
}

func deleteTransaction(ctx context.Context, deps Deps, in DeleteTransactionInput) (*mcp.CallToolResult, DeleteTransactionOutput, error) {
	if in.ID == "" {
		return nil, DeleteTransactionOutput{}, fmt.Errorf("missing required field: id")
	}
	id, err := parseID(in.ID)
	if err != nil {
		return nil, DeleteTransactionOutput{}, err
	}

	transaction, err := deps.Q.GetTransaction(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, DeleteTransactionOutput{}, fmt.Errorf("transaction %q not found", in.ID)
		}
		return nil, DeleteTransactionOutput{}, err
	}

	account, err := deps.Q.GetAccount(ctx, transaction.AccountID)
	if err != nil {
		return nil, DeleteTransactionOutput{}, err
	}

	info := toTransactionInfo(transaction, account.Currency)
	revision := transactionDeletionRevision(transaction)

	if in.ConfirmationToken == "" {
		token, expiresAt := confirm.Issue(deps.ConfirmSecret, confirmActionDeleteTransaction, in.ID, revision)
		return nil, DeleteTransactionOutput{
			Status:            "pending_confirmation",
			Transaction:       info,
			ConfirmationToken: token,
			ExpiresAt:         expiresAt,
		}, nil
	}

	if err := confirm.Verify(deps.ConfirmSecret, in.ConfirmationToken, confirmActionDeleteTransaction, in.ID, revision, time.Now()); err != nil {
		return nil, DeleteTransactionOutput{}, err
	}

	tx, err := deps.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, DeleteTransactionOutput{}, err
	}
	defer tx.Rollback()

	q := deps.Q.WithTx(tx)

	if _, err := q.GetTransaction(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, DeleteTransactionOutput{}, fmt.Errorf("transaction %q no longer exists; please preview again", in.ID)
		}
		return nil, DeleteTransactionOutput{}, err
	}

	if err := q.DeleteTransaction(ctx, id); err != nil {
		return nil, DeleteTransactionOutput{}, err
	}

	if err := tx.Commit(); err != nil {
		return nil, DeleteTransactionOutput{}, err
	}

	return nil, DeleteTransactionOutput{Status: "deleted", Transaction: info}, nil
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
