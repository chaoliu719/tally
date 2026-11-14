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

const (
	confirmActionBatchUpdateTransactionsPrefix = "batch_update_transactions:"
	batchUpdateTransactionsMaxItems            = batchDeleteTransactionsMaxIDs
)

// BatchUpdateTransactionsPatch specifies which reference fields to change in a
// batch_update_transactions call. At least one of CategoryID, SourceID, or
// BudgetID must be provided. Non-reference fields (amount, currency, time,
// type, merchant, comment) are not accepted here.
type BatchUpdateTransactionsPatch struct {
	CategoryID string `json:"category_id,omitempty" jsonschema:"new category_id to set on every target transaction, as a decimal string; omit to leave category_id unchanged"`
	SourceID   string `json:"source_id,omitempty" jsonschema:"new source_id to set on every target transaction, as a decimal string; omit to leave source_id unchanged"`
	BudgetID   string `json:"budget_id,omitempty" jsonschema:"new budget_id to set on every target transaction, as a decimal string (must be a real budget, not \"0\"); omit to leave budget_id unchanged"`
}

// BatchUpdateTransactionsFilter selects target transactions by AND-combining
// one or more conditions within the ledger.
type BatchUpdateTransactionsFilter struct {
	CategoryID string `json:"category_id,omitempty" jsonschema:"only match transactions in this category, as a decimal string"`
	SourceID   string `json:"source_id,omitempty" jsonschema:"only match transactions from/to this source, as a decimal string"`
	BudgetID   string `json:"budget_id,omitempty" jsonschema:"only match transactions with this budget, as a decimal string; pass \"0\" to match legacy unassigned transactions"`
	StartTime  string `json:"start_time,omitempty" jsonschema:"only match transactions at or after this local date-time (format YYYY-MM-DD HH:MM:SS, no timezone)"`
	EndTime    string `json:"end_time,omitempty" jsonschema:"only match transactions at or before this local date-time (format YYYY-MM-DD HH:MM:SS, no timezone)"`
}

type BatchUpdateTransactionsInput struct {
	LedgerID          string                         `json:"ledger_id" jsonschema:"the id of the ledger these transactions belong to, as a decimal string"`
	IDs               []string                       `json:"ids,omitempty" jsonschema:"target transactions by their unique ids, as decimal strings (at most 100 per call); mutually exclusive with filter"`
	Filter            *BatchUpdateTransactionsFilter `json:"filter,omitempty" jsonschema:"target transactions matching these AND-combined conditions (at most 100 matches per call); mutually exclusive with ids"`
	Patch             BatchUpdateTransactionsPatch   `json:"patch" jsonschema:"reference fields to update (category_id, source_id, and/or budget_id); at least one must be provided"`
	ConfirmationToken string                         `json:"confirmation_token,omitempty" jsonschema:"omit to preview the batch and receive a token covering it, or supply the token from a prior preview to apply the update"`
}

// BatchUpdateFieldDiff shows the before and after values of the patched
// reference fields for a single transaction.
type BatchUpdateFieldDiff struct {
	CategoryID string `json:"category_id,omitempty" jsonschema:"category_id value (present when patch modifies category_id)"`
	SourceID   string `json:"source_id,omitempty" jsonschema:"source_id value (present when patch modifies source_id)"`
	BudgetID   string `json:"budget_id,omitempty" jsonschema:"budget_id value (present when patch modifies budget_id)"`
}

// BatchUpdateTransactionItemResult reports the outcome for one target
// transaction in a batch_update_transactions call.
type BatchUpdateTransactionItemResult struct {
	ID          string                `json:"id" jsonschema:"the transaction's unique id, as a decimal string"`
	Status      string                `json:"status" jsonschema:"pending_confirmation, updated, not_found, or revision_changed"`
	Before      *BatchUpdateFieldDiff `json:"before,omitempty" jsonschema:"values of the patched fields before the update; absent when status is not_found"`
	After       *BatchUpdateFieldDiff `json:"after,omitempty" jsonschema:"values of the patched fields after the update; absent when status is not_found"`
	Transaction *TransactionInfo      `json:"transaction,omitempty" jsonschema:"the transaction's current (on preview or revision_changed) or updated (on updated) info; absent when status is not_found"`
}

type BatchUpdateTransactionsOutput struct {
	MatchedCount      int                                `json:"matched_count" jsonschema:"number of existing transactions in the ledger matched by this batch"`
	Results           []BatchUpdateTransactionItemResult `json:"results" jsonschema:"one result per target transaction"`
	ConfirmationToken string                             `json:"confirmation_token,omitempty" jsonschema:"present on preview; pass this back as confirmation_token to apply the batch update"`
	ExpiresAt         int64                              `json:"expires_at,omitempty" jsonschema:"unix seconds when confirmation_token expires; present on preview"`
}

type validatedBatchPatch struct {
	CategoryID sql.NullInt64
	SourceID   sql.NullInt64
	BudgetID   sql.NullInt64
}

func validateBatchPatch(ctx context.Context, deps Deps, ledgerID int64, p BatchUpdateTransactionsPatch) (validatedBatchPatch, error) {
	if p.CategoryID == "" && p.SourceID == "" && p.BudgetID == "" {
		return validatedBatchPatch{}, fmt.Errorf("patch is empty: at least one of category_id, source_id, or budget_id must be provided")
	}

	var out validatedBatchPatch
	if p.CategoryID != "" {
		catID, err := parseID(p.CategoryID)
		if err != nil {
			return validatedBatchPatch{}, err
		}
		if _, err := deps.Q.GetCategory(ctx, store.GetCategoryParams{ID: catID, LedgerID: ledgerID}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return validatedBatchPatch{}, fmt.Errorf("category %q not found", p.CategoryID)
			}
			return validatedBatchPatch{}, err
		}
		out.CategoryID = sql.NullInt64{Int64: catID, Valid: true}
	}
	if p.SourceID != "" {
		srcID, err := parseID(p.SourceID)
		if err != nil {
			return validatedBatchPatch{}, err
		}
		if _, err := deps.Q.GetSource(ctx, store.GetSourceParams{ID: srcID, LedgerID: ledgerID}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return validatedBatchPatch{}, fmt.Errorf("source %q not found", p.SourceID)
			}
			return validatedBatchPatch{}, err
		}
		out.SourceID = sql.NullInt64{Int64: srcID, Valid: true}
	}
	if p.BudgetID != "" {
		budID, err := parseID(p.BudgetID)
		if err != nil {
			return validatedBatchPatch{}, err
		}
		if budID <= 0 {
			return validatedBatchPatch{}, fmt.Errorf("invalid budget_id %q: must reference an existing budget", p.BudgetID)
		}
		if _, err := deps.Q.GetBudget(ctx, store.GetBudgetParams{ID: budID, LedgerID: ledgerID}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return validatedBatchPatch{}, fmt.Errorf("budget %q not found", p.BudgetID)
			}
			return validatedBatchPatch{}, err
		}
		out.BudgetID = sql.NullInt64{Int64: budID, Valid: true}
	}
	return out, nil
}

func batchUpdateConfirmAction(ledgerID int64, p BatchUpdateTransactionsPatch) string {
	type actionDescriptor struct {
		LedgerID   int64  `json:"ledger_id"`
		CategoryID string `json:"category_id,omitempty"`
		SourceID   string `json:"source_id,omitempty"`
		BudgetID   string `json:"budget_id,omitempty"`
	}
	body, _ := json.Marshal(actionDescriptor{
		LedgerID:   ledgerID,
		CategoryID: p.CategoryID,
		SourceID:   p.SourceID,
		BudgetID:   p.BudgetID,
	})
	sum := sha256.Sum256(body)
	return confirmActionBatchUpdateTransactionsPrefix + hex.EncodeToString(sum[:])
}

func buildBatchUpdateDiffs(t store.Transaction, p BatchUpdateTransactionsPatch) (before, after BatchUpdateFieldDiff) {
	if p.CategoryID != "" {
		before.CategoryID = formatID(t.CategoryID)
		after.CategoryID = p.CategoryID
	}
	if p.SourceID != "" {
		before.SourceID = formatID(t.SourceID)
		after.SourceID = p.SourceID
	}
	if p.BudgetID != "" {
		before.BudgetID = formatID(t.BudgetID)
		after.BudgetID = p.BudgetID
	}
	return before, after
}

func batchUpdateTransactions(ctx context.Context, deps Deps, in BatchUpdateTransactionsInput) (*mcp.CallToolResult, BatchUpdateTransactionsOutput, error) {
	if in.LedgerID == "" {
		return nil, BatchUpdateTransactionsOutput{}, fmt.Errorf("missing required field: ledger_id")
	}
	ledgerID, err := parseID(in.LedgerID)
	if err != nil {
		return nil, BatchUpdateTransactionsOutput{}, err
	}
	if _, err := deps.Q.GetLedger(ctx, ledgerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, BatchUpdateTransactionsOutput{}, fmt.Errorf("ledger %q not found", in.LedgerID)
		}
		return nil, BatchUpdateTransactionsOutput{}, err
	}

	patch, err := validateBatchPatch(ctx, deps, ledgerID, in.Patch)
	if err != nil {
		return nil, BatchUpdateTransactionsOutput{}, err
	}

	hasIDs := len(in.IDs) > 0
	hasFilter := in.Filter != nil
	if hasIDs && hasFilter {
		return nil, BatchUpdateTransactionsOutput{}, fmt.Errorf("specify either ids or filter, not both")
	}
	if !hasIDs && !hasFilter {
		return nil, BatchUpdateTransactionsOutput{}, fmt.Errorf("missing target transactions: specify either ids or filter")
	}

	if in.ConfirmationToken == "" {
		return previewBatchUpdateTransactions(ctx, deps, ledgerID, in, patch)
	}
	return confirmBatchUpdateTransactions(ctx, deps, ledgerID, in, patch)
}

func resolveFilterTransactions(ctx context.Context, deps Deps, ledgerID int64, f *BatchUpdateTransactionsFilter) ([]store.Transaction, error) {
	if f.CategoryID == "" && f.SourceID == "" && f.BudgetID == "" && f.StartTime == "" && f.EndTime == "" {
		return nil, fmt.Errorf("filter is empty: at least one filter condition must be provided")
	}
	params := store.SearchTransactionsParams{
		LedgerID: ledgerID,
		Limit:    batchUpdateTransactionsMaxItems + 1,
	}
	if f.SourceID != "" {
		id, err := parseID(f.SourceID)
		if err != nil {
			return nil, err
		}
		params.SourceID = id
	}
	if f.CategoryID != "" {
		id, err := parseID(f.CategoryID)
		if err != nil {
			return nil, err
		}
		catJSON, _ := json.Marshal([]int64{id})
		params.CategoryIdsJson = string(catJSON)
	}
	if f.BudgetID != "" {
		id, err := parseID(f.BudgetID)
		if err != nil {
			return nil, err
		}
		params.BudgetID = sql.NullInt64{Int64: id, Valid: true}
	}
	if f.StartTime != "" {
		if err := validateLocalDateTime(f.StartTime); err != nil {
			return nil, err
		}
		params.StartTime = f.StartTime
	}
	if f.EndTime != "" {
		if err := validateLocalDateTime(f.EndTime); err != nil {
			return nil, err
		}
		params.EndTime = f.EndTime
	}

	rows, err := deps.Q.SearchTransactions(ctx, params)
	if err != nil {
		return nil, err
	}
	if len(rows) > batchUpdateTransactionsMaxItems {
		return nil, fmt.Errorf("too many matching transactions: matched more than max %d per call", batchUpdateTransactionsMaxItems)
	}
	return rows, nil
}

func previewBatchUpdateTransactions(ctx context.Context, deps Deps, ledgerID int64, in BatchUpdateTransactionsInput, _ validatedBatchPatch) (*mcp.CallToolResult, BatchUpdateTransactionsOutput, error) {
	action := batchUpdateConfirmAction(ledgerID, in.Patch)

	if len(in.IDs) > 0 {
		if len(in.IDs) > batchUpdateTransactionsMaxItems {
			return nil, BatchUpdateTransactionsOutput{}, fmt.Errorf("too many ids: got %d, max is %d per call", len(in.IDs), batchUpdateTransactionsMaxItems)
		}
		ids := make([]int64, len(in.IDs))
		for i, idStr := range in.IDs {
			id, err := parseID(idStr)
			if err != nil {
				return nil, BatchUpdateTransactionsOutput{}, err
			}
			ids[i] = id
		}

		results := make([]BatchUpdateTransactionItemResult, len(ids))
		items := make([]confirm.Item, len(ids))
		matchedCount := 0

		for i, id := range ids {
			idStr := in.IDs[i]
			transaction, err := deps.Q.GetTransaction(ctx, store.GetTransactionParams{ID: id, LedgerID: ledgerID})
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					results[i] = BatchUpdateTransactionItemResult{ID: idStr, Status: "not_found"}
					items[i] = confirm.Item{ID: idStr}
					continue
				}
				return nil, BatchUpdateTransactionsOutput{}, err
			}
			matchedCount++
			info, err := toTransactionInfo(transaction)
			if err != nil {
				return nil, BatchUpdateTransactionsOutput{}, err
			}
			before, after := buildBatchUpdateDiffs(transaction, in.Patch)
			results[i] = BatchUpdateTransactionItemResult{
				ID:          idStr,
				Status:      "pending_confirmation",
				Before:      &before,
				After:       &after,
				Transaction: &info,
			}
			items[i] = confirm.Item{ID: idStr, Revision: transactionDeletionRevision(transaction)}
		}

		token, expiresAt := confirm.IssueBatch(deps.ConfirmSecret, action, items)
		return nil, BatchUpdateTransactionsOutput{
			MatchedCount:      matchedCount,
			Results:           results,
			ConfirmationToken: token,
			ExpiresAt:         expiresAt,
		}, nil
	}

	rows, err := resolveFilterTransactions(ctx, deps, ledgerID, in.Filter)
	if err != nil {
		return nil, BatchUpdateTransactionsOutput{}, err
	}

	results := make([]BatchUpdateTransactionItemResult, len(rows))
	items := make([]confirm.Item, len(rows))
	for i, transaction := range rows {
		idStr := formatID(transaction.ID)
		info, err := toTransactionInfo(transaction)
		if err != nil {
			return nil, BatchUpdateTransactionsOutput{}, err
		}
		before, after := buildBatchUpdateDiffs(transaction, in.Patch)
		results[i] = BatchUpdateTransactionItemResult{
			ID:          idStr,
			Status:      "pending_confirmation",
			Before:      &before,
			After:       &after,
			Transaction: &info,
		}
		items[i] = confirm.Item{ID: idStr, Revision: transactionDeletionRevision(transaction)}
	}

	token, expiresAt := confirm.IssueBatch(deps.ConfirmSecret, action, items)
	return nil, BatchUpdateTransactionsOutput{
		MatchedCount:      len(rows),
		Results:           results,
		ConfirmationToken: token,
		ExpiresAt:         expiresAt,
	}, nil
}

func confirmBatchUpdateTransactions(ctx context.Context, deps Deps, ledgerID int64, in BatchUpdateTransactionsInput, patch validatedBatchPatch) (*mcp.CallToolResult, BatchUpdateTransactionsOutput, error) {
	action := batchUpdateConfirmAction(ledgerID, in.Patch)

	tokenItems, err := confirm.BatchItems(in.ConfirmationToken)
	if err != nil {
		return nil, BatchUpdateTransactionsOutput{}, err
	}

	var idStrs []string
	if len(in.IDs) > 0 {
		if len(in.IDs) > batchUpdateTransactionsMaxItems {
			return nil, BatchUpdateTransactionsOutput{}, fmt.Errorf("too many ids: got %d, max is %d per call", len(in.IDs), batchUpdateTransactionsMaxItems)
		}
		idStrs = in.IDs
	} else {
		if _, err := resolveFilterTransactions(ctx, deps, ledgerID, in.Filter); err != nil {
			return nil, BatchUpdateTransactionsOutput{}, err
		}
		idStrs = make([]string, len(tokenItems))
		for i, item := range tokenItems {
			idStrs[i] = item.ID
		}
	}

	wantItems := make([]confirm.Item, len(idStrs))
	for i, idStr := range idStrs {
		wantItems[i] = confirm.Item{ID: idStr}
	}
	if err := confirm.VerifyBatch(deps.ConfirmSecret, in.ConfirmationToken, action, wantItems, time.Now()); err != nil {
		return nil, BatchUpdateTransactionsOutput{}, err
	}

	expectedRevision := make(map[string]string, len(tokenItems))
	for _, item := range tokenItems {
		expectedRevision[item.ID] = item.Revision
	}

	results := make([]BatchUpdateTransactionItemResult, len(idStrs))
	matchedCount := 0
	for i, idStr := range idStrs {
		id, err := parseID(idStr)
		if err != nil {
			return nil, BatchUpdateTransactionsOutput{}, err
		}
		res, err := confirmUpdateOneTransaction(ctx, deps, ledgerID, id, idStr, expectedRevision[idStr], in.Patch, patch)
		if err != nil {
			return nil, BatchUpdateTransactionsOutput{}, err
		}
		if res.Status == "updated" {
			matchedCount++
		}
		results[i] = res
	}

	return nil, BatchUpdateTransactionsOutput{
		MatchedCount: matchedCount,
		Results:      results,
	}, nil
}

func confirmUpdateOneTransaction(ctx context.Context, deps Deps, ledgerID, id int64, idStr, wantRevision string, rawPatch BatchUpdateTransactionsPatch, patch validatedBatchPatch) (BatchUpdateTransactionItemResult, error) {
	tx, err := deps.DB.BeginTx(ctx, nil)
	if err != nil {
		return BatchUpdateTransactionItemResult{}, err
	}
	defer tx.Rollback()

	q := deps.Q.WithTx(tx)

	transaction, err := q.GetTransaction(ctx, store.GetTransactionParams{ID: id, LedgerID: ledgerID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BatchUpdateTransactionItemResult{ID: idStr, Status: "not_found"}, nil
		}
		return BatchUpdateTransactionItemResult{}, err
	}

	if transactionDeletionRevision(transaction) != wantRevision {
		info, err := toTransactionInfo(transaction)
		if err != nil {
			return BatchUpdateTransactionItemResult{}, err
		}
		return BatchUpdateTransactionItemResult{ID: idStr, Status: "revision_changed", Transaction: &info}, nil
	}

	before, after := buildBatchUpdateDiffs(transaction, rawPatch)

	newCatID := transaction.CategoryID
	if patch.CategoryID.Valid {
		newCatID = patch.CategoryID.Int64
	}
	newSrcID := transaction.SourceID
	if patch.SourceID.Valid {
		newSrcID = patch.SourceID.Int64
	}
	newBudID := transaction.BudgetID
	if patch.BudgetID.Valid {
		newBudID = patch.BudgetID.Int64
	}

	updated, err := q.UpdateTransaction(ctx, store.UpdateTransactionParams{
		Type:       transaction.Type,
		SourceID:   newSrcID,
		CategoryID: newCatID,
		BudgetID:   newBudID,
		Currency:   transaction.Currency,
		Amount:     transaction.Amount,
		Time:       transaction.Time,
		Merchant:   transaction.Merchant,
		Comment:    transaction.Comment,
		UpdatedAt:  time.Now().Unix(),
		ID:         id,
	})
	if err != nil {
		return BatchUpdateTransactionItemResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return BatchUpdateTransactionItemResult{}, err
	}

	info, err := toTransactionInfo(updated)
	if err != nil {
		return BatchUpdateTransactionItemResult{}, err
	}

	return BatchUpdateTransactionItemResult{
		ID:          idStr,
		Status:      "updated",
		Before:      &before,
		After:       &after,
		Transaction: &info,
	}, nil
}
