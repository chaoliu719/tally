package tools

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/confirm"
	"tally/internal/currency"
	"tally/internal/store"
)

func init() {
	register(registerTransactionTools)
}

func registerTransactionTools(s *mcp.Server, deps Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_transaction",
		Description: "Record one income or expense transaction. Requires an existing category_id (any category in the ledger).",
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
		Name: "search_transactions",
		Description: "List transactions, optionally filtered by time range, source, category, and/or a case-insensitive comment keyword, sorted oldest first. Results are paginated: " +
			"each call returns at most limit transactions (default 50, max 200); if more match, the response includes next_cursor -- pass it back " +
			"as cursor on the next call to keep paging until next_cursor is no longer returned. With no filters, pages through every transaction in the ledger.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in SearchTransactionsInput) (*mcp.CallToolResult, SearchTransactionsOutput, error) {
		return searchTransactions(ctx, deps, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_transaction",
		Description: "Replace a transaction's fields (type, source, category, amount, currency, time, comment) by id. This is a full replacement, not a partial update -- all fields must be provided, and the same validation rules as create_transaction apply. Does not require confirmation.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateTransactionInput) (*mcp.CallToolResult, UpdateTransactionOutput, error) {
		return updateTransaction(ctx, deps, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "delete_transaction",
		Description: "Delete a transaction by id. This is a two-step preview -> apply: call without confirmation_token to preview " +
			"(returns the transaction and a token), then call again with the returned confirmation_token to actually delete. " +
			"Unlike source/category deletion, any existing transaction can be deleted -- there is no reference-count gate.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in DeleteTransactionInput) (*mcp.CallToolResult, DeleteTransactionOutput, error) {
		return deleteTransaction(ctx, deps, in)
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
	Type       string `json:"type" jsonschema:"the transaction's type: income or expense"`
	SourceID   string `json:"source_id" jsonschema:"the id of the source this transaction is from/to, as a decimal string"`
	CategoryID string `json:"category_id" jsonschema:"the id of the transaction's category, as a decimal string"`
	Amount     string `json:"amount" jsonschema:"the transaction amount, as a decimal string in the currency's major unit; how many fractional digits that allows varies by currency (e.g. \"50.00\" for CNY, \"5000\" for JPY, \"5.000\" for BHD). Positive for both income and expense"`
	Currency   string `json:"currency" jsonschema:"the transaction's currency, as an ISO 4217 code"`
	Time       int64  `json:"time" jsonschema:"when the transaction occurred, as unix seconds"`
	Comment    string `json:"comment,omitempty" jsonschema:"an optional note about the transaction"`
}

type CreateTransactionInput struct {
	LedgerID   string `json:"ledger_id" jsonschema:"the id of the ledger this transaction belongs to, as a decimal string"`
	Type       string `json:"type" jsonschema:"the transaction's type: income or expense"`
	SourceID   string `json:"source_id" jsonschema:"the id of the source this transaction is from/to, as a decimal string; must belong to the same ledger"`
	CategoryID string `json:"category_id" jsonschema:"the id of the transaction's category, as a decimal string; any existing category in the same ledger"`
	Amount     string `json:"amount" jsonschema:"the transaction amount, as a decimal string in the currency's major unit; how many fractional digits that allows varies by currency (e.g. \"50.00\" for CNY, \"5000\" for JPY, \"5.000\" for BHD); must be positive"`
	Currency   string `json:"currency" jsonschema:"the transaction's currency, as an ISO 4217 code, e.g. CNY, USD"`
	Time       int64  `json:"time" jsonschema:"when the transaction occurred, as unix seconds"`
	Comment    string `json:"comment,omitempty" jsonschema:"an optional note about the transaction"`
}

type CreateTransactionOutput struct {
	Transaction TransactionInfo `json:"transaction" jsonschema:"the newly recorded transaction"`
}

// validatedTransactionFields is the normalized, validated form of the fields
// shared by create_transaction and update_transaction: the ledger id, the
// source id, the resolved category_id, and the signed amount to store
// (income amounts are stored positive, expense negative, so SUM(amount) is
// directly the net total; the wire format keeps *Amount positive for both).
type validatedTransactionFields struct {
	LedgerID   int64
	SourceID   int64
	CategoryID int64
	Currency   string
	Amount     int64
}

// validateTransactionInput checks and normalizes the ledger_id/type/
// source_id/category_id/amount/currency/time rules shared by
// create_transaction and update_transaction: income/expense require an
// existing ledger, a source_id and category_id both belonging to that
// ledger, a supported currency code, and an amount string that is a valid,
// positive decimal number at or under that currency's standard precision.
// currency must be validated before amount can be parsed, since amount's
// allowed decimal precision depends on which currency it's in.
func validateTransactionInput(ctx context.Context, deps Deps, ledgerIDStr, txType, sourceIDStr, categoryIDStr, amountStr, currencyCode string, txTime int64) (validatedTransactionFields, error) {
	if !createableTransactionTypes[txType] {
		return validatedTransactionFields{}, fmt.Errorf("missing or unsupported transaction type: %q", txType)
	}

	if ledgerIDStr == "" {
		return validatedTransactionFields{}, fmt.Errorf("missing required field: ledger_id")
	}
	ledgerID, err := parseID(ledgerIDStr)
	if err != nil {
		return validatedTransactionFields{}, err
	}
	if _, err := deps.Q.GetLedger(ctx, ledgerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return validatedTransactionFields{}, fmt.Errorf("ledger %q not found", ledgerIDStr)
		}
		return validatedTransactionFields{}, err
	}

	if sourceIDStr == "" {
		return validatedTransactionFields{}, fmt.Errorf("missing required field: source_id")
	}
	sourceID, err := parseID(sourceIDStr)
	if err != nil {
		return validatedTransactionFields{}, err
	}

	if txTime <= 0 {
		return validatedTransactionFields{}, fmt.Errorf("missing required field: time")
	}

	if _, err := deps.Q.GetSource(ctx, store.GetSourceParams{ID: sourceID, LedgerID: ledgerID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return validatedTransactionFields{}, fmt.Errorf("source %q not found", sourceIDStr)
		}
		return validatedTransactionFields{}, err
	}

	if categoryIDStr == "" {
		return validatedTransactionFields{}, fmt.Errorf("missing required field: category_id")
	}
	categoryID, err := parseID(categoryIDStr)
	if err != nil {
		return validatedTransactionFields{}, err
	}
	if _, err := deps.Q.GetCategory(ctx, store.GetCategoryParams{ID: categoryID, LedgerID: ledgerID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return validatedTransactionFields{}, fmt.Errorf("category %q not found", categoryIDStr)
		}
		return validatedTransactionFields{}, err
	}

	if currencyCode == "" {
		return validatedTransactionFields{}, fmt.Errorf("missing required field: currency")
	}
	if !currency.Supported(currencyCode) {
		return validatedTransactionFields{}, fmt.Errorf("unsupported currency: %q", currencyCode)
	}

	if amountStr == "" {
		return validatedTransactionFields{}, fmt.Errorf("missing required field: amount")
	}
	amount, err := currency.ParseMajor(currencyCode, amountStr)
	if err != nil {
		return validatedTransactionFields{}, err
	}

	signedAmount := amount
	if txType == "expense" {
		signedAmount = -amount
	}

	return validatedTransactionFields{LedgerID: ledgerID, SourceID: sourceID, CategoryID: categoryID, Currency: currencyCode, Amount: signedAmount}, nil
}

func createTransaction(ctx context.Context, deps Deps, in CreateTransactionInput) (*mcp.CallToolResult, CreateTransactionOutput, error) {
	fields, err := validateTransactionInput(ctx, deps, in.LedgerID, in.Type, in.SourceID, in.CategoryID, in.Amount, in.Currency, in.Time)
	if err != nil {
		return nil, CreateTransactionOutput{}, err
	}

	now := time.Now().Unix()
	transaction, err := deps.Q.CreateTransaction(ctx, store.CreateTransactionParams{
		LedgerID:   fields.LedgerID,
		Type:       in.Type,
		SourceID:   fields.SourceID,
		CategoryID: fields.CategoryID,
		Currency:   fields.Currency,
		Amount:     fields.Amount,
		Time:       in.Time,
		Comment:    in.Comment,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		return nil, CreateTransactionOutput{}, err
	}

	info, err := toTransactionInfo(transaction)
	if err != nil {
		return nil, CreateTransactionOutput{}, err
	}
	return nil, CreateTransactionOutput{Transaction: info}, nil
}

type GetTransactionInput struct {
	ID       string `json:"id" jsonschema:"the transaction's unique id, as a decimal string"`
	LedgerID string `json:"ledger_id" jsonschema:"the id of the ledger this transaction belongs to, as a decimal string"`
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
	if in.LedgerID == "" {
		return nil, GetTransactionOutput{}, fmt.Errorf("missing required field: ledger_id")
	}
	ledgerID, err := parseID(in.LedgerID)
	if err != nil {
		return nil, GetTransactionOutput{}, err
	}

	transaction, err := deps.Q.GetTransaction(ctx, store.GetTransactionParams{ID: id, LedgerID: ledgerID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, GetTransactionOutput{}, fmt.Errorf("transaction %q not found", in.ID)
		}
		return nil, GetTransactionOutput{}, err
	}

	info, err := toTransactionInfo(transaction)
	if err != nil {
		return nil, GetTransactionOutput{}, err
	}
	return nil, GetTransactionOutput{Transaction: info}, nil
}

type UpdateTransactionInput struct {
	ID         string `json:"id" jsonschema:"the transaction's unique id, as a decimal string"`
	LedgerID   string `json:"ledger_id" jsonschema:"the id of the ledger this transaction belongs to, as a decimal string"`
	Type       string `json:"type" jsonschema:"the transaction's type: income or expense"`
	SourceID   string `json:"source_id" jsonschema:"the id of the source this transaction is from/to, as a decimal string; may differ from the transaction's current source_id to move it to another source in the same ledger"`
	CategoryID string `json:"category_id" jsonschema:"the id of the transaction's category, as a decimal string; any existing category in the same ledger"`
	Amount     string `json:"amount" jsonschema:"the transaction amount, as a decimal string in the currency's major unit; how many fractional digits that allows varies by currency (e.g. \"50.00\" for CNY, \"5000\" for JPY, \"5.000\" for BHD); must be positive"`
	Currency   string `json:"currency" jsonschema:"the transaction's currency, as an ISO 4217 code, e.g. CNY, USD"`
	Time       int64  `json:"time" jsonschema:"when the transaction occurred, as unix seconds"`
	Comment    string `json:"comment,omitempty" jsonschema:"an optional note about the transaction"`
}

type UpdateTransactionOutput struct {
	Transaction TransactionInfo `json:"transaction" jsonschema:"the transaction's updated info"`
}

// updateTransaction replaces every mutable field of an existing transaction.
// This is full-field-replacement semantics (like manage_source/
// manage_category's operation=update), reusing the exact validation rules
// create_transaction applies -- see validateTransactionInput. The
// transaction's ledger cannot be changed: ledger_id must match the
// transaction's current ledger, and source_id/category_id must belong to it.
func updateTransaction(ctx context.Context, deps Deps, in UpdateTransactionInput) (*mcp.CallToolResult, UpdateTransactionOutput, error) {
	if in.ID == "" {
		return nil, UpdateTransactionOutput{}, fmt.Errorf("missing required field: id")
	}
	id, err := parseID(in.ID)
	if err != nil {
		return nil, UpdateTransactionOutput{}, err
	}
	if in.LedgerID == "" {
		return nil, UpdateTransactionOutput{}, fmt.Errorf("missing required field: ledger_id")
	}
	ledgerID, err := parseID(in.LedgerID)
	if err != nil {
		return nil, UpdateTransactionOutput{}, err
	}

	if _, err := deps.Q.GetTransaction(ctx, store.GetTransactionParams{ID: id, LedgerID: ledgerID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, UpdateTransactionOutput{}, fmt.Errorf("transaction %q not found", in.ID)
		}
		return nil, UpdateTransactionOutput{}, err
	}

	fields, err := validateTransactionInput(ctx, deps, in.LedgerID, in.Type, in.SourceID, in.CategoryID, in.Amount, in.Currency, in.Time)
	if err != nil {
		return nil, UpdateTransactionOutput{}, err
	}

	updated, err := deps.Q.UpdateTransaction(ctx, store.UpdateTransactionParams{
		Type:       in.Type,
		SourceID:   fields.SourceID,
		CategoryID: fields.CategoryID,
		Currency:   fields.Currency,
		Amount:     fields.Amount,
		Time:       in.Time,
		Comment:    in.Comment,
		UpdatedAt:  time.Now().Unix(),
		ID:         id,
	})
	if err != nil {
		return nil, UpdateTransactionOutput{}, err
	}

	info, err := toTransactionInfo(updated)
	if err != nil {
		return nil, UpdateTransactionOutput{}, err
	}
	return nil, UpdateTransactionOutput{Transaction: info}, nil
}

// transactionDeletionRevisionFields is hashed to produce the revision
// embedded in a delete_transaction confirmation token (see design.md's
// "delete_transaction:走 preview → apply"). Unlike source/category
// deletion, there is no reference-count gate -- every field here exists
// purely to detect that the transaction was modified or replaced since the
// preview, not to decide whether deletion is allowed.
type transactionDeletionRevisionFields struct {
	Type       string
	SourceID   int64
	CategoryID int64
	Currency   string
	Amount     int64
	Time       int64
	Comment    string
}

func transactionDeletionRevision(t store.Transaction) string {
	fields := transactionDeletionRevisionFields{
		Type:       t.Type,
		SourceID:   t.SourceID,
		CategoryID: t.CategoryID,
		Currency:   t.Currency,
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
	LedgerID          string `json:"ledger_id" jsonschema:"the id of the ledger this transaction belongs to, as a decimal string"`
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
	if in.LedgerID == "" {
		return nil, DeleteTransactionOutput{}, fmt.Errorf("missing required field: ledger_id")
	}
	ledgerID, err := parseID(in.LedgerID)
	if err != nil {
		return nil, DeleteTransactionOutput{}, err
	}

	transaction, err := deps.Q.GetTransaction(ctx, store.GetTransactionParams{ID: id, LedgerID: ledgerID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, DeleteTransactionOutput{}, fmt.Errorf("transaction %q not found", in.ID)
		}
		return nil, DeleteTransactionOutput{}, err
	}

	info, err := toTransactionInfo(transaction)
	if err != nil {
		return nil, DeleteTransactionOutput{}, err
	}
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

	if _, err := q.GetTransaction(ctx, store.GetTransactionParams{ID: id, LedgerID: ledgerID}); err != nil {
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

const (
	searchTransactionsDefaultLimit = 50
	searchTransactionsMaxLimit     = 200
)

type SearchTransactionsInput struct {
	LedgerID   string `json:"ledger_id" jsonschema:"the id of the ledger to search transactions in, as a decimal string"`
	SourceID   string `json:"source_id,omitempty" jsonschema:"only include transactions from/to this source, as a decimal string"`
	CategoryID string `json:"category_id,omitempty" jsonschema:"only include transactions in this category, as a decimal string"`
	StartTime  int64  `json:"start_time,omitempty" jsonschema:"only include transactions at or after this unix time (seconds)"`
	EndTime    int64  `json:"end_time,omitempty" jsonschema:"only include transactions at or before this unix time (seconds)"`
	Keyword    string `json:"keyword,omitempty" jsonschema:"only include transactions whose comment contains this substring, case-insensitively; % and _ are matched literally, not as wildcards. Blank (empty or whitespace-only) is treated as not provided"`
	Limit      int64  `json:"limit,omitempty" jsonschema:"maximum number of transactions to return in this page; defaults to 50 when omitted, must be between 1 and 200 (requests over 200 are rejected, not truncated)"`
	Cursor     string `json:"cursor,omitempty" jsonschema:"opaque pagination cursor from a previous response's next_cursor; omit to fetch the first page. Must be paired with the exact same ledger_id/source_id/category_id/start_time/end_time/keyword filters used to obtain it"`
}

type SearchTransactionsOutput struct {
	Transactions []TransactionInfo `json:"transactions" jsonschema:"the matching transactions for this page"`
	NextCursor   string            `json:"next_cursor,omitempty" jsonschema:"present when more results exist beyond this page; pass it back as cursor to fetch the next page. Absent when this is the last page"`
}

func searchTransactions(ctx context.Context, deps Deps, in SearchTransactionsInput) (*mcp.CallToolResult, SearchTransactionsOutput, error) {
	if in.LedgerID == "" {
		return nil, SearchTransactionsOutput{}, fmt.Errorf("missing required field: ledger_id")
	}
	ledgerID, err := parseID(in.LedgerID)
	if err != nil {
		return nil, SearchTransactionsOutput{}, err
	}
	if _, err := deps.Q.GetLedger(ctx, ledgerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, SearchTransactionsOutput{}, fmt.Errorf("ledger %q not found", in.LedgerID)
		}
		return nil, SearchTransactionsOutput{}, err
	}

	params := store.SearchTransactionsParams{LedgerID: ledgerID}
	filter := searchTransactionsFilterFields{LedgerID: ledgerID}

	if in.SourceID != "" {
		id, err := parseID(in.SourceID)
		if err != nil {
			return nil, SearchTransactionsOutput{}, err
		}
		params.SourceID = id
		filter.SourceID = sql.NullInt64{Int64: id, Valid: true}
	}
	if in.CategoryID != "" {
		id, err := parseID(in.CategoryID)
		if err != nil {
			return nil, SearchTransactionsOutput{}, err
		}
		params.CategoryID = id
		filter.CategoryID = sql.NullInt64{Int64: id, Valid: true}
	}
	if in.StartTime > 0 {
		params.StartTime = in.StartTime
		filter.StartTime = sql.NullInt64{Int64: in.StartTime, Valid: true}
	}
	if in.EndTime > 0 {
		params.EndTime = in.EndTime
		filter.EndTime = sql.NullInt64{Int64: in.EndTime, Valid: true}
	}
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		params.Keyword = escapeLikeKeyword(keyword)
		filter.Keyword = keyword
	}

	limit := int64(searchTransactionsDefaultLimit)
	if in.Limit != 0 {
		if in.Limit < 0 || in.Limit > searchTransactionsMaxLimit {
			return nil, SearchTransactionsOutput{}, fmt.Errorf("limit must be between 1 and %d", searchTransactionsMaxLimit)
		}
		limit = in.Limit
	}

	if in.Cursor != "" {
		afterTime, afterID, err := decodeSearchTransactionsCursor(in.Cursor, filter)
		if err != nil {
			return nil, SearchTransactionsOutput{}, err
		}
		params.AfterTime = afterTime
		params.AfterID = sql.NullInt64{Int64: afterID, Valid: true}
	}

	// Ask for one extra row beyond the page size: if it comes back, there is
	// a next page, and the (limit)th row (the last one we actually return)
	// becomes the keyset position for next_cursor. This avoids a separate
	// COUNT(*) query to determine whether more results remain.
	params.Limit = limit + 1

	transactions, err := deps.Q.SearchTransactions(ctx, params)
	if err != nil {
		return nil, SearchTransactionsOutput{}, err
	}

	var nextCursor string
	if int64(len(transactions)) > limit {
		last := transactions[limit-1]
		nextCursor = encodeSearchTransactionsCursor(last.Time, last.ID, filter)
		transactions = transactions[:limit]
	}

	infos := make([]TransactionInfo, 0, len(transactions))
	for _, t := range transactions {
		info, err := toTransactionInfo(t)
		if err != nil {
			return nil, SearchTransactionsOutput{}, err
		}
		infos = append(infos, info)
	}

	return nil, SearchTransactionsOutput{Transactions: infos, NextCursor: nextCursor}, nil
}

func toTransactionInfo(t store.Transaction) (TransactionInfo, error) {
	amount, err := currency.FormatMajor(t.Currency, abs64(t.Amount))
	if err != nil {
		return TransactionInfo{}, err
	}
	return TransactionInfo{
		ID:         formatID(t.ID),
		Type:       t.Type,
		SourceID:   formatID(t.SourceID),
		CategoryID: formatID(t.CategoryID),
		Amount:     amount,
		Currency:   t.Currency,
		Time:       t.Time,
		Comment:    t.Comment,
	}, nil
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
