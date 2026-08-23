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
	register(registerSourceTools)
}

func registerSourceTools(s *mcp.Server, deps Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_sources",
		Description: "List every source in the ledger (a source is where a transaction's money comes from or goes to), including its name.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ ListSourcesInput) (*mcp.CallToolResult, ListSourcesOutput, error) {
		return listSources(ctx, deps)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "manage_source",
		Description: "Create, update, or delete a source, selected via the operation field. " +
			"operation=create makes a new source. operation=update replaces name on an " +
			"existing source. " +
			"operation=delete is a two-step preview -> apply: call without confirmation_token to preview " +
			"(fails if the source is still referenced by any transaction), then call again with the " +
			"returned confirmation_token to actually delete.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ManageSourceInput) (*mcp.CallToolResult, ManageSourceOutput, error) {
		return manageSource(ctx, deps, in)
	})
}

// SourceInfo is the wire representation of a source returned by
// list_sources and manage_source.
type SourceInfo struct {
	ID   string `json:"id" jsonschema:"the source's unique id, as a decimal string"`
	Name string `json:"name" jsonschema:"the source's name"`
}

type ListSourcesInput struct{}

type ListSourcesOutput struct {
	Sources []SourceInfo `json:"sources" jsonschema:"every source in the ledger"`
}

func listSources(ctx context.Context, deps Deps) (*mcp.CallToolResult, ListSourcesOutput, error) {
	rows, err := deps.Q.ListSources(ctx)
	if err != nil {
		return nil, ListSourcesOutput{}, err
	}

	infos := make([]SourceInfo, 0, len(rows))
	for _, s := range rows {
		infos = append(infos, toSourceInfo(s))
	}

	return nil, ListSourcesOutput{Sources: infos}, nil
}

type ManageSourceInput struct {
	Operation string `json:"operation" jsonschema:"the operation to perform: create, update, or delete"`
	ID        string `json:"id,omitempty" jsonschema:"the source's id, as a decimal string; required for operation=update and operation=delete"`

	Name string `json:"name,omitempty" jsonschema:"the source's name; required for create and update"`

	ConfirmationToken string `json:"confirmation_token,omitempty" jsonschema:"for operation=delete only: omit to preview the deletion and receive a token, or supply the token from a prior preview to actually delete"`
}

type ManageSourceOutput struct {
	Source            SourceInfo `json:"source" jsonschema:"the source's current (or, for a completed delete, last known) info"`
	Status            string     `json:"status" jsonschema:"created, updated, pending_confirmation, or deleted"`
	ConfirmationToken string     `json:"confirmation_token,omitempty" jsonschema:"present when status is pending_confirmation; pass this back as confirmation_token with the same operation=delete call to actually delete"`
	ExpiresAt         int64      `json:"expires_at,omitempty" jsonschema:"unix seconds when confirmation_token expires; present when status is pending_confirmation"`
}

func manageSource(ctx context.Context, deps Deps, in ManageSourceInput) (*mcp.CallToolResult, ManageSourceOutput, error) {
	switch in.Operation {
	case "create":
		return createSource(ctx, deps, in)
	case "update":
		return updateSource(ctx, deps, in)
	case "delete":
		return deleteSource(ctx, deps, in)
	default:
		return nil, ManageSourceOutput{}, fmt.Errorf("missing or unsupported operation: %q (must be create, update, or delete)", in.Operation)
	}
}

func createSource(ctx context.Context, deps Deps, in ManageSourceInput) (*mcp.CallToolResult, ManageSourceOutput, error) {
	if in.Name == "" {
		return nil, ManageSourceOutput{}, fmt.Errorf("missing required field: name")
	}

	now := time.Now().Unix()
	source, err := deps.Q.CreateSource(ctx, store.CreateSourceParams{
		Name:      in.Name,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return nil, ManageSourceOutput{}, err
	}

	return nil, ManageSourceOutput{Status: "created", Source: toSourceInfo(source)}, nil
}

func updateSource(ctx context.Context, deps Deps, in ManageSourceInput) (*mcp.CallToolResult, ManageSourceOutput, error) {
	if in.ID == "" {
		return nil, ManageSourceOutput{}, fmt.Errorf("missing required field: id")
	}
	id, err := parseID(in.ID)
	if err != nil {
		return nil, ManageSourceOutput{}, err
	}

	if in.Name == "" {
		return nil, ManageSourceOutput{}, fmt.Errorf("missing required field: name")
	}

	if _, err := deps.Q.GetSource(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ManageSourceOutput{}, fmt.Errorf("source %q not found", in.ID)
		}
		return nil, ManageSourceOutput{}, err
	}

	updated, err := deps.Q.UpdateSource(ctx, store.UpdateSourceParams{
		Name:      in.Name,
		UpdatedAt: time.Now().Unix(),
		ID:        id,
	})
	if err != nil {
		return nil, ManageSourceOutput{}, err
	}

	return nil, ManageSourceOutput{Status: "updated", Source: toSourceInfo(updated)}, nil
}

// sourceDeletionRevisionFields is hashed to produce the revision embedded
// in a source deletion's confirmation token (see design.md's "revision 覆盖
// 哪些字段"). TxCount is what actually gates whether the delete is allowed;
// Name guards against the previewed source having changed underneath the
// caller.
type sourceDeletionRevisionFields struct {
	Name    string
	TxCount int64
}

func sourceDeletionRevision(s store.Source, txCount int64) string {
	fields := sourceDeletionRevisionFields{
		Name:    s.Name,
		TxCount: txCount,
	}
	// Marshaling a fixed struct of plain strings/ints never fails.
	body, _ := json.Marshal(fields)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

const confirmActionDeleteSource = "delete_source"

func deleteSource(ctx context.Context, deps Deps, in ManageSourceInput) (*mcp.CallToolResult, ManageSourceOutput, error) {
	if in.ID == "" {
		return nil, ManageSourceOutput{}, fmt.Errorf("missing required field: id")
	}
	id, err := parseID(in.ID)
	if err != nil {
		return nil, ManageSourceOutput{}, err
	}

	source, err := deps.Q.GetSource(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ManageSourceOutput{}, fmt.Errorf("source %q not found", in.ID)
		}
		return nil, ManageSourceOutput{}, err
	}

	txCount, err := deps.Q.CountTransactionsBySource(ctx, id)
	if err != nil {
		return nil, ManageSourceOutput{}, err
	}

	info := toSourceInfo(source)
	revision := sourceDeletionRevision(source, txCount)

	if in.ConfirmationToken == "" {
		if txCount != 0 {
			return nil, ManageSourceOutput{}, fmt.Errorf("source %q is still referenced by %d transaction(s) and cannot be deleted", in.ID, txCount)
		}

		token, expiresAt := confirm.Issue(deps.ConfirmSecret, confirmActionDeleteSource, in.ID, revision)
		return nil, ManageSourceOutput{
			Status:            "pending_confirmation",
			Source:            info,
			ConfirmationToken: token,
			ExpiresAt:         expiresAt,
		}, nil
	}

	if err := confirm.Verify(deps.ConfirmSecret, in.ConfirmationToken, confirmActionDeleteSource, in.ID, revision, time.Now()); err != nil {
		return nil, ManageSourceOutput{}, err
	}

	tx, err := deps.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, ManageSourceOutput{}, err
	}
	defer tx.Rollback()

	q := deps.Q.WithTx(tx)

	liveTxCount, err := q.CountTransactionsBySource(ctx, id)
	if err != nil {
		return nil, ManageSourceOutput{}, err
	}
	if liveTxCount != 0 {
		return nil, ManageSourceOutput{}, fmt.Errorf("source %q is now referenced by %d transaction(s) and cannot be deleted; please preview again", in.ID, liveTxCount)
	}

	if err := q.DeleteSource(ctx, id); err != nil {
		return nil, ManageSourceOutput{}, err
	}

	if err := tx.Commit(); err != nil {
		return nil, ManageSourceOutput{}, err
	}

	return nil, ManageSourceOutput{Status: "deleted", Source: info}, nil
}

func toSourceInfo(s store.Source) SourceInfo {
	return SourceInfo{ID: formatID(s.ID), Name: s.Name}
}
