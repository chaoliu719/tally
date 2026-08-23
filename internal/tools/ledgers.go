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
	register(registerLedgerTools)
}

func registerLedgerTools(s *mcp.Server, deps Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_ledgers",
		Description: "List every ledger, including its name and comment. A ledger is a top-level isolation container: sources, categories, and transactions each belong to exactly one ledger and are never shared across ledgers.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ ListLedgersInput) (*mcp.CallToolResult, ListLedgersOutput, error) {
		return listLedgers(ctx, deps)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "manage_ledger",
		Description: "Create, update, or delete a ledger, selected via the operation field. " +
			"operation=create makes a new ledger with no sources or categories yet. operation=update replaces " +
			"name and/or comment on an existing ledger (at least one must be provided). " +
			"operation=delete is a two-step preview -> apply: call without confirmation_token to preview " +
			"(fails if the ledger still has any sources, categories, or transactions), then call again with the " +
			"returned confirmation_token to actually delete.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ManageLedgerInput) (*mcp.CallToolResult, ManageLedgerOutput, error) {
		return manageLedger(ctx, deps, in)
	})
}

// LedgerInfo is the wire representation of a ledger returned by list_ledgers
// and manage_ledger.
type LedgerInfo struct {
	ID      string `json:"id" jsonschema:"the ledger's unique id, as a decimal string"`
	Name    string `json:"name" jsonschema:"the ledger's name"`
	Comment string `json:"comment,omitempty" jsonschema:"an optional note about the ledger"`
}

type ListLedgersInput struct{}

type ListLedgersOutput struct {
	Ledgers []LedgerInfo `json:"ledgers" jsonschema:"every ledger in the system"`
}

func listLedgers(ctx context.Context, deps Deps) (*mcp.CallToolResult, ListLedgersOutput, error) {
	rows, err := deps.Q.ListLedgers(ctx)
	if err != nil {
		return nil, ListLedgersOutput{}, err
	}

	infos := make([]LedgerInfo, 0, len(rows))
	for _, l := range rows {
		infos = append(infos, toLedgerInfo(l))
	}

	return nil, ListLedgersOutput{Ledgers: infos}, nil
}

type ManageLedgerInput struct {
	Operation string `json:"operation" jsonschema:"the operation to perform: create, update, or delete"`
	ID        string `json:"id,omitempty" jsonschema:"the ledger's id, as a decimal string; required for operation=update and operation=delete"`

	Name    string `json:"name,omitempty" jsonschema:"the ledger's name; required for create, optional for update"`
	Comment string `json:"comment,omitempty" jsonschema:"an optional note about the ledger; for update, only changed when non-empty"`

	ConfirmationToken string `json:"confirmation_token,omitempty" jsonschema:"for operation=delete only: omit to preview the deletion and receive a token, or supply the token from a prior preview to actually delete"`
}

type ManageLedgerOutput struct {
	Ledger            LedgerInfo `json:"ledger" jsonschema:"the ledger's current (or, for a completed delete, last known) info"`
	Status            string     `json:"status" jsonschema:"created, updated, pending_confirmation, or deleted"`
	ConfirmationToken string     `json:"confirmation_token,omitempty" jsonschema:"present when status is pending_confirmation; pass this back as confirmation_token with the same operation=delete call to actually delete"`
	ExpiresAt         int64      `json:"expires_at,omitempty" jsonschema:"unix seconds when confirmation_token expires; present when status is pending_confirmation"`
}

func manageLedger(ctx context.Context, deps Deps, in ManageLedgerInput) (*mcp.CallToolResult, ManageLedgerOutput, error) {
	switch in.Operation {
	case "create":
		return createLedger(ctx, deps, in)
	case "update":
		return updateLedger(ctx, deps, in)
	case "delete":
		return deleteLedger(ctx, deps, in)
	default:
		return nil, ManageLedgerOutput{}, fmt.Errorf("missing or unsupported operation: %q (must be create, update, or delete)", in.Operation)
	}
}

func createLedger(ctx context.Context, deps Deps, in ManageLedgerInput) (*mcp.CallToolResult, ManageLedgerOutput, error) {
	if in.Name == "" {
		return nil, ManageLedgerOutput{}, fmt.Errorf("missing required field: name")
	}

	now := time.Now().Unix()
	ledger, err := deps.Q.CreateLedger(ctx, store.CreateLedgerParams{
		Name:      in.Name,
		Comment:   in.Comment,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return nil, ManageLedgerOutput{}, err
	}

	return nil, ManageLedgerOutput{Status: "created", Ledger: toLedgerInfo(ledger)}, nil
}

func updateLedger(ctx context.Context, deps Deps, in ManageLedgerInput) (*mcp.CallToolResult, ManageLedgerOutput, error) {
	if in.ID == "" {
		return nil, ManageLedgerOutput{}, fmt.Errorf("missing required field: id")
	}
	id, err := parseID(in.ID)
	if err != nil {
		return nil, ManageLedgerOutput{}, err
	}

	if in.Name == "" && in.Comment == "" {
		return nil, ManageLedgerOutput{}, fmt.Errorf("missing required field: name or comment (at least one must be provided)")
	}

	existing, err := deps.Q.GetLedger(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ManageLedgerOutput{}, fmt.Errorf("ledger %q not found", in.ID)
		}
		return nil, ManageLedgerOutput{}, err
	}

	newName := existing.Name
	if in.Name != "" {
		newName = in.Name
	}
	newComment := existing.Comment
	if in.Comment != "" {
		newComment = in.Comment
	}

	updated, err := deps.Q.UpdateLedger(ctx, store.UpdateLedgerParams{
		Name:      newName,
		Comment:   newComment,
		UpdatedAt: time.Now().Unix(),
		ID:        id,
	})
	if err != nil {
		return nil, ManageLedgerOutput{}, err
	}

	return nil, ManageLedgerOutput{Status: "updated", Ledger: toLedgerInfo(updated)}, nil
}

// ledgerDeletionRevisionFields is hashed to produce the revision embedded in
// a ledger deletion's confirmation token (see sourceDeletionRevisionFields/
// categoryDeletionRevisionFields for the same pattern). SourceCount/
// CategoryCount/TxCount are what actually gate whether the delete is
// allowed; Name/Comment guard against the previewed ledger having changed
// underneath the caller.
type ledgerDeletionRevisionFields struct {
	Name          string
	Comment       string
	SourceCount   int64
	CategoryCount int64
	TxCount       int64
}

func ledgerDeletionRevision(l store.Ledger, sourceCount, categoryCount, txCount int64) string {
	fields := ledgerDeletionRevisionFields{
		Name:          l.Name,
		Comment:       l.Comment,
		SourceCount:   sourceCount,
		CategoryCount: categoryCount,
		TxCount:       txCount,
	}
	// Marshaling a fixed struct of plain strings/ints never fails.
	body, _ := json.Marshal(fields)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

const confirmActionDeleteLedger = "delete_ledger"

func deleteLedger(ctx context.Context, deps Deps, in ManageLedgerInput) (*mcp.CallToolResult, ManageLedgerOutput, error) {
	if in.ID == "" {
		return nil, ManageLedgerOutput{}, fmt.Errorf("missing required field: id")
	}
	id, err := parseID(in.ID)
	if err != nil {
		return nil, ManageLedgerOutput{}, err
	}

	ledger, err := deps.Q.GetLedger(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ManageLedgerOutput{}, fmt.Errorf("ledger %q not found", in.ID)
		}
		return nil, ManageLedgerOutput{}, err
	}

	sourceCount, err := deps.Q.CountSourcesByLedger(ctx, id)
	if err != nil {
		return nil, ManageLedgerOutput{}, err
	}
	categoryCount, err := deps.Q.CountCategoriesByLedger(ctx, id)
	if err != nil {
		return nil, ManageLedgerOutput{}, err
	}
	txCount, err := deps.Q.CountTransactionsByLedger(ctx, id)
	if err != nil {
		return nil, ManageLedgerOutput{}, err
	}

	info := toLedgerInfo(ledger)
	revision := ledgerDeletionRevision(ledger, sourceCount, categoryCount, txCount)

	if in.ConfirmationToken == "" {
		if sourceCount != 0 || categoryCount != 0 || txCount != 0 {
			return nil, ManageLedgerOutput{}, fmt.Errorf("ledger %q is not empty (sources=%d, categories=%d, transactions=%d) and cannot be deleted", in.ID, sourceCount, categoryCount, txCount)
		}

		token, expiresAt := confirm.Issue(deps.ConfirmSecret, confirmActionDeleteLedger, in.ID, revision)
		return nil, ManageLedgerOutput{
			Status:            "pending_confirmation",
			Ledger:            info,
			ConfirmationToken: token,
			ExpiresAt:         expiresAt,
		}, nil
	}

	if err := confirm.Verify(deps.ConfirmSecret, in.ConfirmationToken, confirmActionDeleteLedger, in.ID, revision, time.Now()); err != nil {
		return nil, ManageLedgerOutput{}, err
	}

	tx, err := deps.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, ManageLedgerOutput{}, err
	}
	defer tx.Rollback()

	q := deps.Q.WithTx(tx)

	liveSourceCount, err := q.CountSourcesByLedger(ctx, id)
	if err != nil {
		return nil, ManageLedgerOutput{}, err
	}
	liveCategoryCount, err := q.CountCategoriesByLedger(ctx, id)
	if err != nil {
		return nil, ManageLedgerOutput{}, err
	}
	liveTxCount, err := q.CountTransactionsByLedger(ctx, id)
	if err != nil {
		return nil, ManageLedgerOutput{}, err
	}
	if liveSourceCount != 0 || liveCategoryCount != 0 || liveTxCount != 0 {
		return nil, ManageLedgerOutput{}, fmt.Errorf("ledger %q can no longer be deleted (sources=%d, categories=%d, transactions=%d); please preview again", in.ID, liveSourceCount, liveCategoryCount, liveTxCount)
	}

	if err := q.DeleteLedger(ctx, id); err != nil {
		return nil, ManageLedgerOutput{}, err
	}

	if err := tx.Commit(); err != nil {
		return nil, ManageLedgerOutput{}, err
	}

	return nil, ManageLedgerOutput{Status: "deleted", Ledger: info}, nil
}

func toLedgerInfo(l store.Ledger) LedgerInfo {
	return LedgerInfo{ID: formatID(l.ID), Name: l.Name, Comment: l.Comment}
}
