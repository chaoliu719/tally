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
		Description: "List transactions, optionally filtered by time range, source, category, and/or a case-insensitive comment keyword. Sorted oldest first by default, or newest first when newest_first is true. Results are paginated: " +
			"each call returns at most limit transactions (default 50, max 200); if more match, the response includes next_cursor -- pass it back " +
			"as cursor on the next call (with the same filters and the same newest_first value) to keep paging until next_cursor is no longer returned. With no filters, pages through every transaction in the ledger.",
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

	mcp.AddTool(s, &mcp.Tool{
		Name: "batch_delete_transactions",
		Description: fmt.Sprintf("Delete a batch of transactions by id, in one ledger. This is a two-step preview -> confirm: "+
			"call without confirmation_token to preview (returns each id's current transaction info, or a not_found marker, "+
			"plus one token covering the whole batch), then call again with the returned confirmation_token to actually delete. "+
			"Deletion is best-effort per item: an id that no longer exists or changed since the preview is reported individually "+
			"(status not_found or revision_changed) without blocking the rest of the batch from being deleted. "+
			"At most %d ids per call -- previewing more than that is rejected outright.", batchDeleteTransactionsMaxIDs),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in BatchDeleteTransactionsInput) (*mcp.CallToolResult, BatchDeleteTransactionsOutput, error) {
		return batchDeleteTransactions(ctx, deps, in)
	})
}

var createableTransactionTypes = map[string]bool{
	"income":  true,
	"expense": true,
}

// localDateTimeLayout is the only format tally['s transactions.time field
// accepts: a naive local date-time, no timezone marker. It is never parsed
// into a time.Time for arithmetic -- only validated against this layout and
// otherwise stored/compared/returned as a plain string (see design.md's D3).
const localDateTimeLayout = "2006-01-02 15:04:05"

func validateLocalDateTime(s string) error {
	if _, err := time.Parse(localDateTimeLayout, s); err != nil {
		return fmt.Errorf("invalid time %q: expected format YYYY-MM-DD HH:MM:SS", s)
	}
	return nil
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
	Time       string `json:"time" jsonschema:"when the transaction occurred, as a local date-time string in the form YYYY-MM-DD HH:MM:SS (24-hour, zero-padded); no timezone -- stored and returned exactly as given, never converted"`
	Comment    string `json:"comment,omitempty" jsonschema:"an optional note about the transaction"`
}

type CreateTransactionInput struct {
	LedgerID   string `json:"ledger_id" jsonschema:"the id of the ledger this transaction belongs to, as a decimal string"`
	Type       string `json:"type" jsonschema:"the transaction's type: income or expense"`
	SourceID   string `json:"source_id" jsonschema:"the id of the source this transaction is from/to, as a decimal string; must belong to the same ledger"`
	CategoryID string `json:"category_id" jsonschema:"the id of the transaction's category, as a decimal string; any existing category in the same ledger"`
	Amount     string `json:"amount" jsonschema:"the transaction amount, as a decimal string in the currency's major unit; how many fractional digits that allows varies by currency (e.g. \"50.00\" for CNY, \"5000\" for JPY, \"5.000\" for BHD); must be positive"`
	Currency   string `json:"currency" jsonschema:"the transaction's currency, as an ISO 4217 code, e.g. CNY, USD"`
	Time       string `json:"time" jsonschema:"when the transaction occurred, as a local date-time string in the form YYYY-MM-DD HH:MM:SS (24-hour, zero-padded); no timezone -- stored and returned exactly as given, never converted"`
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
func validateTransactionInput(ctx context.Context, deps Deps, ledgerIDStr, txType, sourceIDStr, categoryIDStr, amountStr, currencyCode, txTime string) (validatedTransactionFields, error) {
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

	if txTime == "" {
		return validatedTransactionFields{}, fmt.Errorf("missing required field: time")
	}
	if err := validateLocalDateTime(txTime); err != nil {
		return validatedTransactionFields{}, err
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
	Time       string `json:"time" jsonschema:"when the transaction occurred, as a local date-time string in the form YYYY-MM-DD HH:MM:SS (24-hour, zero-padded); no timezone -- stored and returned exactly as given, never converted"`
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
	Time       string
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
	confirmActionBatchDeleteTransactions = "batch_delete_transactions"
	batchDeleteTransactionsMaxIDs        = 100
)

type BatchDeleteTransactionsInput struct {
	LedgerID          string   `json:"ledger_id" jsonschema:"the id of the ledger these transactions belong to, as a decimal string"`
	IDs               []string `json:"ids" jsonschema:"the transactions' unique ids, as decimal strings; at most 100 per call"`
	ConfirmationToken string   `json:"confirmation_token,omitempty" jsonschema:"omit to preview the batch and receive a token covering it, or supply the token from a prior preview to actually delete"`
}

// BatchDeleteTransactionItemResult reports the outcome for one id in a
// batch_delete_transactions call, whether previewed or confirmed.
type BatchDeleteTransactionItemResult struct {
	ID          string           `json:"id" jsonschema:"the transaction's unique id, as a decimal string"`
	Status      string           `json:"status" jsonschema:"pending_confirmation, deleted, not_found, or revision_changed"`
	Transaction *TransactionInfo `json:"transaction,omitempty" jsonschema:"the transaction's info (or, for a completed delete, its last known info); absent when status is not_found"`
}

type BatchDeleteTransactionsOutput struct {
	Results           []BatchDeleteTransactionItemResult `json:"results" jsonschema:"one result per requested id, in the same order as the request"`
	ConfirmationToken string                             `json:"confirmation_token,omitempty" jsonschema:"present on preview; pass this back as confirmation_token to actually delete the batch"`
	ExpiresAt         int64                              `json:"expires_at,omitempty" jsonschema:"unix seconds when confirmation_token expires; present on preview"`
}

func batchDeleteTransactions(ctx context.Context, deps Deps, in BatchDeleteTransactionsInput) (*mcp.CallToolResult, BatchDeleteTransactionsOutput, error) {
	if in.LedgerID == "" {
		return nil, BatchDeleteTransactionsOutput{}, fmt.Errorf("missing required field: ledger_id")
	}
	ledgerID, err := parseID(in.LedgerID)
	if err != nil {
		return nil, BatchDeleteTransactionsOutput{}, err
	}
	if len(in.IDs) == 0 {
		return nil, BatchDeleteTransactionsOutput{}, fmt.Errorf("missing required field: ids")
	}
	if len(in.IDs) > batchDeleteTransactionsMaxIDs {
		return nil, BatchDeleteTransactionsOutput{}, fmt.Errorf("too many ids: got %d, max is %d per call", len(in.IDs), batchDeleteTransactionsMaxIDs)
	}

	ids := make([]int64, len(in.IDs))
	for i, idStr := range in.IDs {
		id, err := parseID(idStr)
		if err != nil {
			return nil, BatchDeleteTransactionsOutput{}, err
		}
		ids[i] = id
	}

	if in.ConfirmationToken == "" {
		return previewBatchDeleteTransactions(ctx, deps, ledgerID, in.IDs, ids)
	}
	return confirmBatchDeleteTransactions(ctx, deps, ledgerID, in.IDs, ids, in.ConfirmationToken)
}

func previewBatchDeleteTransactions(ctx context.Context, deps Deps, ledgerID int64, idStrs []string, ids []int64) (*mcp.CallToolResult, BatchDeleteTransactionsOutput, error) {
	results := make([]BatchDeleteTransactionItemResult, len(ids))
	// items covers every requested id, including not-found ones (with an
	// empty revision) -- the token's id set must match the full batch the
	// caller previewed, so a later confirm resending the same ids list
	// verifies cleanly. A not-found id's empty revision is never compared:
	// confirmDeleteOneTransaction re-queries first and reports not_found
	// before it would look at revision.
	items := make([]confirm.Item, len(ids))

	for i, id := range ids {
		idStr := idStrs[i]
		transaction, err := deps.Q.GetTransaction(ctx, store.GetTransactionParams{ID: id, LedgerID: ledgerID})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				results[i] = BatchDeleteTransactionItemResult{ID: idStr, Status: "not_found"}
				items[i] = confirm.Item{ID: idStr}
				continue
			}
			return nil, BatchDeleteTransactionsOutput{}, err
		}

		info, err := toTransactionInfo(transaction)
		if err != nil {
			return nil, BatchDeleteTransactionsOutput{}, err
		}
		results[i] = BatchDeleteTransactionItemResult{ID: idStr, Status: "pending_confirmation", Transaction: &info}
		items[i] = confirm.Item{ID: idStr, Revision: transactionDeletionRevision(transaction)}
	}

	token, expiresAt := confirm.IssueBatch(deps.ConfirmSecret, confirmActionBatchDeleteTransactions, items)
	return nil, BatchDeleteTransactionsOutput{
		Results:           results,
		ConfirmationToken: token,
		ExpiresAt:         expiresAt,
	}, nil
}

func confirmBatchDeleteTransactions(ctx context.Context, deps Deps, ledgerID int64, idStrs []string, ids []int64, token string) (*mcp.CallToolResult, BatchDeleteTransactionsOutput, error) {
	wantItems := make([]confirm.Item, len(idStrs))
	for i, idStr := range idStrs {
		wantItems[i] = confirm.Item{ID: idStr}
	}
	if err := confirm.VerifyBatch(deps.ConfirmSecret, token, confirmActionBatchDeleteTransactions, wantItems, time.Now()); err != nil {
		return nil, BatchDeleteTransactionsOutput{}, err
	}

	tokenItems, err := confirm.BatchItems(token)
	if err != nil {
		return nil, BatchDeleteTransactionsOutput{}, err
	}
	expectedRevision := make(map[string]string, len(tokenItems))
	for _, item := range tokenItems {
		expectedRevision[item.ID] = item.Revision
	}

	results := make([]BatchDeleteTransactionItemResult, len(ids))
	for i, id := range ids {
		idStr := idStrs[i]
		result, err := confirmDeleteOneTransaction(ctx, deps, ledgerID, id, idStr, expectedRevision[idStr])
		if err != nil {
			return nil, BatchDeleteTransactionsOutput{}, err
		}
		results[i] = result
	}

	return nil, BatchDeleteTransactionsOutput{Results: results}, nil
}

// confirmDeleteOneTransaction re-queries and deletes a single transaction as
// part of a batch confirm, in its own transaction, mirroring
// deleteTransaction's own begin/check/delete/commit sequence. It reports
// not_found or revision_changed instead of erroring, so one item's failure
// never blocks the rest of the batch (see design.md's best-effort decision).
func confirmDeleteOneTransaction(ctx context.Context, deps Deps, ledgerID, id int64, idStr, wantRevision string) (BatchDeleteTransactionItemResult, error) {
	tx, err := deps.DB.BeginTx(ctx, nil)
	if err != nil {
		return BatchDeleteTransactionItemResult{}, err
	}
	defer tx.Rollback()

	q := deps.Q.WithTx(tx)

	transaction, err := q.GetTransaction(ctx, store.GetTransactionParams{ID: id, LedgerID: ledgerID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BatchDeleteTransactionItemResult{ID: idStr, Status: "not_found"}, nil
		}
		return BatchDeleteTransactionItemResult{}, err
	}

	info, err := toTransactionInfo(transaction)
	if err != nil {
		return BatchDeleteTransactionItemResult{}, err
	}

	if transactionDeletionRevision(transaction) != wantRevision {
		return BatchDeleteTransactionItemResult{ID: idStr, Status: "revision_changed", Transaction: &info}, nil
	}

	if err := q.DeleteTransaction(ctx, id); err != nil {
		return BatchDeleteTransactionItemResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return BatchDeleteTransactionItemResult{}, err
	}

	return BatchDeleteTransactionItemResult{ID: idStr, Status: "deleted", Transaction: &info}, nil
}

const (
	searchTransactionsDefaultLimit = 50
	searchTransactionsMaxLimit     = 200
)

type SearchTransactionsInput struct {
	LedgerID    string `json:"ledger_id" jsonschema:"the id of the ledger to search transactions in, as a decimal string"`
	SourceID    string `json:"source_id,omitempty" jsonschema:"only include transactions from/to this source, as a decimal string"`
	CategoryID  string `json:"category_id,omitempty" jsonschema:"only include transactions in this category, as a decimal string"`
	StartTime   string `json:"start_time,omitempty" jsonschema:"only include transactions at or after this local date-time (format YYYY-MM-DD HH:MM:SS, no timezone)"`
	EndTime     string `json:"end_time,omitempty" jsonschema:"only include transactions at or before this local date-time (format YYYY-MM-DD HH:MM:SS, no timezone)"`
	Keyword     string `json:"keyword,omitempty" jsonschema:"only include transactions whose comment contains this substring, case-insensitively; % and _ are matched literally, not as wildcards. Blank (empty or whitespace-only) is treated as not provided"`
	Limit       int64  `json:"limit,omitempty" jsonschema:"maximum number of transactions to return in this page; defaults to 50 when omitted, must be between 1 and 200 (requests over 200 are rejected, not truncated)"`
	Cursor      string `json:"cursor,omitempty" jsonschema:"opaque pagination cursor from a previous response's next_cursor; omit to fetch the first page. Must be paired with the exact same ledger_id/source_id/category_id/start_time/end_time/keyword filters and the same newest_first value used to obtain it"`
	NewestFirst bool   `json:"newest_first,omitempty" jsonschema:"when false (default), results are ordered oldest-first and next_cursor pages toward later transactions; when true, results are ordered newest-first and next_cursor pages toward earlier transactions. Must match the value the cursor was issued under"`
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
	filter := searchTransactionsFilterFields{LedgerID: ledgerID, NewestFirst: in.NewestFirst}

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
	if in.StartTime != "" {
		if err := validateLocalDateTime(in.StartTime); err != nil {
			return nil, SearchTransactionsOutput{}, err
		}
		params.StartTime = in.StartTime
		filter.StartTime = sql.NullString{String: in.StartTime, Valid: true}
	}
	if in.EndTime != "" {
		if err := validateLocalDateTime(in.EndTime); err != nil {
			return nil, SearchTransactionsOutput{}, err
		}
		params.EndTime = in.EndTime
		filter.EndTime = sql.NullString{String: in.EndTime, Valid: true}
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

	var transactions []store.Transaction
	if in.NewestFirst {
		transactions, err = deps.Q.SearchTransactionsDesc(ctx, store.SearchTransactionsDescParams{
			LedgerID:   params.LedgerID,
			SourceID:   params.SourceID,
			CategoryID: params.CategoryID,
			StartTime:  params.StartTime,
			EndTime:    params.EndTime,
			Keyword:    params.Keyword,
			AfterTime:  params.AfterTime,
			AfterID:    params.AfterID,
			Limit:      params.Limit,
		})
	} else {
		transactions, err = deps.Q.SearchTransactions(ctx, params)
	}
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
