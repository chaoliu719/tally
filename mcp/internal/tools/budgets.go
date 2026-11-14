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
	register(registerBudgetTools)
}

func registerBudgetTools(s *mcp.Server, deps Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_budgets",
		Description: "List every budget in a ledger (a budget labels what kind of money a transaction represents in your financial structure, e.g. 固定, 专项, 突发, 日常), including its name.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListBudgetsInput) (*mcp.CallToolResult, ListBudgetsOutput, error) {
		return listBudgets(ctx, deps, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "manage_budget",
		Description: "Create, update, or delete a budget, selected via the operation field. " +
			"operation=create makes a new budget. operation=update replaces name on an " +
			"existing budget. " +
			"operation=delete is a two-step preview -> apply: call without confirmation_token to preview " +
			"(fails if the budget is still referenced by any transaction), then call again with the " +
			"returned confirmation_token to actually delete.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ManageBudgetInput) (*mcp.CallToolResult, ManageBudgetOutput, error) {
		return manageBudget(ctx, deps, in)
	})
}

// BudgetInfo is the wire representation of a budget returned by
// list_budgets and manage_budget.
type BudgetInfo struct {
	ID   string `json:"id" jsonschema:"the budget's unique id, as a decimal string"`
	Name string `json:"name" jsonschema:"the budget's name"`
}

type ListBudgetsInput struct {
	LedgerID string `json:"ledger_id" jsonschema:"the id of the ledger to list budgets from, as a decimal string"`
}

type ListBudgetsOutput struct {
	Budgets []BudgetInfo `json:"budgets" jsonschema:"every budget in the ledger"`
}

func listBudgets(ctx context.Context, deps Deps, in ListBudgetsInput) (*mcp.CallToolResult, ListBudgetsOutput, error) {
	if in.LedgerID == "" {
		return nil, ListBudgetsOutput{}, fmt.Errorf("missing required field: ledger_id")
	}
	ledgerID, err := parseID(in.LedgerID)
	if err != nil {
		return nil, ListBudgetsOutput{}, err
	}
	if _, err := deps.Q.GetLedger(ctx, ledgerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ListBudgetsOutput{}, fmt.Errorf("ledger %q not found", in.LedgerID)
		}
		return nil, ListBudgetsOutput{}, err
	}

	rows, err := deps.Q.ListBudgets(ctx, ledgerID)
	if err != nil {
		return nil, ListBudgetsOutput{}, err
	}

	infos := make([]BudgetInfo, 0, len(rows))
	for _, b := range rows {
		infos = append(infos, toBudgetInfo(b))
	}

	return nil, ListBudgetsOutput{Budgets: infos}, nil
}

type ManageBudgetInput struct {
	Operation string `json:"operation" jsonschema:"the operation to perform: create, update, or delete"`
	LedgerID  string `json:"ledger_id" jsonschema:"the id of the ledger this budget belongs to, as a decimal string"`
	ID        string `json:"id,omitempty" jsonschema:"the budget's id, as a decimal string; required for operation=update and operation=delete"`

	Name string `json:"name,omitempty" jsonschema:"the budget's name; required for create and update"`

	ConfirmationToken string `json:"confirmation_token,omitempty" jsonschema:"for operation=delete only: omit to preview the deletion and receive a token, or supply the token from a prior preview to actually delete"`
}

type ManageBudgetOutput struct {
	Budget            BudgetInfo `json:"budget" jsonschema:"the budget's current (or, for a completed delete, last known) info"`
	Status            string     `json:"status" jsonschema:"created, updated, pending_confirmation, or deleted"`
	ConfirmationToken string     `json:"confirmation_token,omitempty" jsonschema:"present when status is pending_confirmation; pass this back as confirmation_token with the same operation=delete call to actually delete"`
	ExpiresAt         int64      `json:"expires_at,omitempty" jsonschema:"unix seconds when confirmation_token expires; present when status is pending_confirmation"`
}

func manageBudget(ctx context.Context, deps Deps, in ManageBudgetInput) (*mcp.CallToolResult, ManageBudgetOutput, error) {
	switch in.Operation {
	case "create":
		return createBudget(ctx, deps, in)
	case "update":
		return updateBudget(ctx, deps, in)
	case "delete":
		return deleteBudget(ctx, deps, in)
	default:
		return nil, ManageBudgetOutput{}, fmt.Errorf("missing or unsupported operation: %q (must be create, update, or delete)", in.Operation)
	}
}

func createBudget(ctx context.Context, deps Deps, in ManageBudgetInput) (*mcp.CallToolResult, ManageBudgetOutput, error) {
	if in.Name == "" {
		return nil, ManageBudgetOutput{}, fmt.Errorf("missing required field: name")
	}
	if in.LedgerID == "" {
		return nil, ManageBudgetOutput{}, fmt.Errorf("missing required field: ledger_id")
	}
	ledgerID, err := parseID(in.LedgerID)
	if err != nil {
		return nil, ManageBudgetOutput{}, err
	}
	if _, err := deps.Q.GetLedger(ctx, ledgerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ManageBudgetOutput{}, fmt.Errorf("ledger %q not found", in.LedgerID)
		}
		return nil, ManageBudgetOutput{}, err
	}

	now := time.Now().Unix()
	budget, err := deps.Q.CreateBudget(ctx, store.CreateBudgetParams{
		LedgerID:  ledgerID,
		Name:      in.Name,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return nil, ManageBudgetOutput{}, err
	}

	return nil, ManageBudgetOutput{Status: "created", Budget: toBudgetInfo(budget)}, nil
}

func updateBudget(ctx context.Context, deps Deps, in ManageBudgetInput) (*mcp.CallToolResult, ManageBudgetOutput, error) {
	if in.ID == "" {
		return nil, ManageBudgetOutput{}, fmt.Errorf("missing required field: id")
	}
	id, err := parseID(in.ID)
	if err != nil {
		return nil, ManageBudgetOutput{}, err
	}

	if in.Name == "" {
		return nil, ManageBudgetOutput{}, fmt.Errorf("missing required field: name")
	}
	if in.LedgerID == "" {
		return nil, ManageBudgetOutput{}, fmt.Errorf("missing required field: ledger_id")
	}
	ledgerID, err := parseID(in.LedgerID)
	if err != nil {
		return nil, ManageBudgetOutput{}, err
	}

	if _, err := deps.Q.GetBudget(ctx, store.GetBudgetParams{ID: id, LedgerID: ledgerID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ManageBudgetOutput{}, fmt.Errorf("budget %q not found", in.ID)
		}
		return nil, ManageBudgetOutput{}, err
	}

	updated, err := deps.Q.UpdateBudget(ctx, store.UpdateBudgetParams{
		Name:      in.Name,
		UpdatedAt: time.Now().Unix(),
		ID:        id,
	})
	if err != nil {
		return nil, ManageBudgetOutput{}, err
	}

	return nil, ManageBudgetOutput{Status: "updated", Budget: toBudgetInfo(updated)}, nil
}

type budgetDeletionRevisionFields struct {
	Name    string
	TxCount int64
}

func budgetDeletionRevision(b store.Budget, txCount int64) string {
	fields := budgetDeletionRevisionFields{
		Name:    b.Name,
		TxCount: txCount,
	}
	body, _ := json.Marshal(fields)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

const confirmActionDeleteBudget = "delete_budget"

func deleteBudget(ctx context.Context, deps Deps, in ManageBudgetInput) (*mcp.CallToolResult, ManageBudgetOutput, error) {
	if in.ID == "" {
		return nil, ManageBudgetOutput{}, fmt.Errorf("missing required field: id")
	}
	id, err := parseID(in.ID)
	if err != nil {
		return nil, ManageBudgetOutput{}, err
	}
	if in.LedgerID == "" {
		return nil, ManageBudgetOutput{}, fmt.Errorf("missing required field: ledger_id")
	}
	ledgerID, err := parseID(in.LedgerID)
	if err != nil {
		return nil, ManageBudgetOutput{}, err
	}

	budget, err := deps.Q.GetBudget(ctx, store.GetBudgetParams{ID: id, LedgerID: ledgerID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ManageBudgetOutput{}, fmt.Errorf("budget %q not found", in.ID)
		}
		return nil, ManageBudgetOutput{}, err
	}

	txCount, err := deps.Q.CountTransactionsByBudget(ctx, id)
	if err != nil {
		return nil, ManageBudgetOutput{}, err
	}

	info := toBudgetInfo(budget)
	revision := budgetDeletionRevision(budget, txCount)

	if in.ConfirmationToken == "" {
		if txCount != 0 {
			return nil, ManageBudgetOutput{}, fmt.Errorf("budget %q is still referenced by %d transaction(s) and cannot be deleted", in.ID, txCount)
		}

		token, expiresAt := confirm.Issue(deps.ConfirmSecret, confirmActionDeleteBudget, in.ID, revision)
		return nil, ManageBudgetOutput{
			Status:            "pending_confirmation",
			Budget:            info,
			ConfirmationToken: token,
			ExpiresAt:         expiresAt,
		}, nil
	}

	if err := confirm.Verify(deps.ConfirmSecret, in.ConfirmationToken, confirmActionDeleteBudget, in.ID, revision, time.Now()); err != nil {
		return nil, ManageBudgetOutput{}, err
	}

	tx, err := deps.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, ManageBudgetOutput{}, err
	}
	defer tx.Rollback()

	q := deps.Q.WithTx(tx)

	liveTxCount, err := q.CountTransactionsByBudget(ctx, id)
	if err != nil {
		return nil, ManageBudgetOutput{}, err
	}
	if liveTxCount != 0 {
		return nil, ManageBudgetOutput{}, fmt.Errorf("budget %q is now referenced by %d transaction(s) and cannot be deleted; please preview again", in.ID, liveTxCount)
	}

	if err := q.DeleteBudget(ctx, id); err != nil {
		return nil, ManageBudgetOutput{}, err
	}

	if err := tx.Commit(); err != nil {
		return nil, ManageBudgetOutput{}, err
	}

	return nil, ManageBudgetOutput{Status: "deleted", Budget: info}, nil
}

func toBudgetInfo(b store.Budget) BudgetInfo {
	return BudgetInfo{ID: formatID(b.ID), Name: b.Name}
}
