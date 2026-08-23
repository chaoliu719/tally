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
	register(registerCategoryTools)
}

func registerCategoryTools(s *mcp.Server, deps Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_categories",
		Description: "List every transaction category in the ledger, including its name and parent category id. A category with parent_id 0 is a top-level category; otherwise parent_id is the id of its parent category. Categories can nest to any depth and any existing category can be referenced by create_transaction.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ ListCategoriesInput) (*mcp.CallToolResult, ListCategoriesOutput, error) {
		return listCategories(ctx, deps)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "manage_category",
		Description: "Create, update, or delete a transaction category, selected via the operation field. " +
			"operation=create makes a new category; omit parent_id (or pass \"0\") for a top-level category, " +
			"or pass the id of any existing category to nest under it, to any depth. operation=update replaces " +
			"name/parent_id on an existing category (both required; moving a category cannot create a cycle). " +
			"operation=delete is a two-step preview -> apply: call without confirmation_token to preview (fails " +
			"if the category still has children or is referenced by any transaction), then call again with the " +
			"returned confirmation_token to actually delete.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ManageCategoryInput) (*mcp.CallToolResult, ManageCategoryOutput, error) {
		return manageCategory(ctx, deps, in)
	})
}

// topLevelParentID is the parent_id value that marks a category as
// top-level, matching the schema.sql default.
const topLevelParentID int64 = 0

// CategoryInfo is the wire representation of a transaction category returned
// by list_categories and manage_category.
type CategoryInfo struct {
	ID       string `json:"id" jsonschema:"the category's unique id, as a decimal string"`
	Name     string `json:"name" jsonschema:"the category's name"`
	ParentID string `json:"parent_id" jsonschema:"\"0\" for a top-level category; otherwise the id of this category's parent, as a decimal string"`
}

type ListCategoriesInput struct{}

type ListCategoriesOutput struct {
	Categories []CategoryInfo `json:"categories" jsonschema:"every transaction category in the ledger"`
}

func listCategories(ctx context.Context, deps Deps) (*mcp.CallToolResult, ListCategoriesOutput, error) {
	rows, err := deps.Q.ListCategories(ctx)
	if err != nil {
		return nil, ListCategoriesOutput{}, err
	}

	infos := make([]CategoryInfo, 0, len(rows))
	for _, c := range rows {
		infos = append(infos, toCategoryInfo(c))
	}

	return nil, ListCategoriesOutput{Categories: infos}, nil
}

type ManageCategoryInput struct {
	Operation string `json:"operation" jsonschema:"the operation to perform: create, update, or delete"`
	ID        string `json:"id,omitempty" jsonschema:"the category's id, as a decimal string; required for operation=update and operation=delete"`

	Name     string `json:"name,omitempty" jsonschema:"the category's name; required for create and update"`
	ParentID string `json:"parent_id,omitempty" jsonschema:"omit or pass \"0\" for a top-level category; otherwise the id (decimal string) of an existing category to nest this one under, to any depth. Used by create and update; for update, cannot equal the category's own id or any of its descendants"`

	ConfirmationToken string `json:"confirmation_token,omitempty" jsonschema:"for operation=delete only: omit to preview the deletion and receive a token, or supply the token from a prior preview to actually delete"`
}

type ManageCategoryOutput struct {
	Category          CategoryInfo `json:"category" jsonschema:"the category's current (or, for a completed delete, last known) info"`
	Status            string       `json:"status" jsonschema:"created, updated, pending_confirmation, or deleted"`
	ConfirmationToken string       `json:"confirmation_token,omitempty" jsonschema:"present when status is pending_confirmation; pass this back as confirmation_token with the same operation=delete call to actually delete"`
	ExpiresAt         int64        `json:"expires_at,omitempty" jsonschema:"unix seconds when confirmation_token expires; present when status is pending_confirmation"`
}

func manageCategory(ctx context.Context, deps Deps, in ManageCategoryInput) (*mcp.CallToolResult, ManageCategoryOutput, error) {
	switch in.Operation {
	case "create":
		return createCategory(ctx, deps, in)
	case "update":
		return updateCategory(ctx, deps, in)
	case "delete":
		return deleteCategory(ctx, deps, in)
	default:
		return nil, ManageCategoryOutput{}, fmt.Errorf("missing or unsupported operation: %q (must be create, update, or delete)", in.Operation)
	}
}

// parseParentID parses a manage_category parent_id field, treating an empty
// string as topLevelParentID (0), matching schema.sql's default.
func parseParentID(s string) (int64, error) {
	if s == "" {
		return topLevelParentID, nil
	}
	return parseID(s)
}

func createCategory(ctx context.Context, deps Deps, in ManageCategoryInput) (*mcp.CallToolResult, ManageCategoryOutput, error) {
	if in.Name == "" {
		return nil, ManageCategoryOutput{}, fmt.Errorf("missing required field: name")
	}

	parentID, err := parseParentID(in.ParentID)
	if err != nil {
		return nil, ManageCategoryOutput{}, err
	}

	if parentID != topLevelParentID {
		if _, err := deps.Q.GetCategory(ctx, parentID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ManageCategoryOutput{}, fmt.Errorf("parent category %q not found", in.ParentID)
			}
			return nil, ManageCategoryOutput{}, err
		}
	}

	now := time.Now().Unix()
	category, err := deps.Q.CreateCategory(ctx, store.CreateCategoryParams{
		Name:      in.Name,
		ParentID:  parentID,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return nil, ManageCategoryOutput{}, err
	}

	return nil, ManageCategoryOutput{Status: "created", Category: toCategoryInfo(category)}, nil
}

func updateCategory(ctx context.Context, deps Deps, in ManageCategoryInput) (*mcp.CallToolResult, ManageCategoryOutput, error) {
	if in.ID == "" {
		return nil, ManageCategoryOutput{}, fmt.Errorf("missing required field: id")
	}
	id, err := parseID(in.ID)
	if err != nil {
		return nil, ManageCategoryOutput{}, err
	}

	if in.Name == "" {
		return nil, ManageCategoryOutput{}, fmt.Errorf("missing required field: name")
	}

	newParentID, err := parseParentID(in.ParentID)
	if err != nil {
		return nil, ManageCategoryOutput{}, err
	}

	if _, err := deps.Q.GetCategory(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ManageCategoryOutput{}, fmt.Errorf("category %q not found", in.ID)
		}
		return nil, ManageCategoryOutput{}, err
	}

	if newParentID != topLevelParentID {
		if newParentID == id {
			return nil, ManageCategoryOutput{}, fmt.Errorf("category %q cannot be its own parent", in.ID)
		}

		if _, err := deps.Q.GetCategory(ctx, newParentID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ManageCategoryOutput{}, fmt.Errorf("parent category %q not found", in.ParentID)
			}
			return nil, ManageCategoryOutput{}, err
		}

		descendantIDs, err := deps.Q.ListCategoryDescendantIDs(ctx, id)
		if err != nil {
			return nil, ManageCategoryOutput{}, err
		}
		for _, descendantID := range descendantIDs {
			if descendantID == newParentID {
				return nil, ManageCategoryOutput{}, fmt.Errorf("cannot move category %q under %q: that would create a cycle", in.ID, in.ParentID)
			}
		}
	}

	updated, err := deps.Q.UpdateCategory(ctx, store.UpdateCategoryParams{
		Name:      in.Name,
		ParentID:  newParentID,
		UpdatedAt: time.Now().Unix(),
		ID:        id,
	})
	if err != nil {
		return nil, ManageCategoryOutput{}, err
	}

	return nil, ManageCategoryOutput{Status: "updated", Category: toCategoryInfo(updated)}, nil
}

// categoryDeletionRevisionFields is hashed to produce the revision embedded
// in a category deletion's confirmation token (see design.md's "revision 覆盖
// 哪些字段"). ChildCount and TxCount are what actually gate whether the
// delete is allowed; Name/ParentID guard against the previewed category
// having changed underneath the caller.
type categoryDeletionRevisionFields struct {
	Name       string
	ParentID   int64
	ChildCount int64
	TxCount    int64
}

func categoryDeletionRevision(c store.Category, childCount, txCount int64) string {
	fields := categoryDeletionRevisionFields{
		Name:       c.Name,
		ParentID:   c.ParentID,
		ChildCount: childCount,
		TxCount:    txCount,
	}
	// Marshaling a fixed struct of plain strings/ints never fails.
	body, _ := json.Marshal(fields)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

const confirmActionDeleteCategory = "delete_category"

func deleteCategory(ctx context.Context, deps Deps, in ManageCategoryInput) (*mcp.CallToolResult, ManageCategoryOutput, error) {
	if in.ID == "" {
		return nil, ManageCategoryOutput{}, fmt.Errorf("missing required field: id")
	}
	id, err := parseID(in.ID)
	if err != nil {
		return nil, ManageCategoryOutput{}, err
	}

	category, err := deps.Q.GetCategory(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ManageCategoryOutput{}, fmt.Errorf("category %q not found", in.ID)
		}
		return nil, ManageCategoryOutput{}, err
	}

	childCount, err := deps.Q.CountChildCategories(ctx, id)
	if err != nil {
		return nil, ManageCategoryOutput{}, err
	}
	txCount, err := deps.Q.CountTransactionsByCategory(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		return nil, ManageCategoryOutput{}, err
	}

	info := toCategoryInfo(category)
	revision := categoryDeletionRevision(category, childCount, txCount)

	if in.ConfirmationToken == "" {
		if childCount != 0 {
			return nil, ManageCategoryOutput{}, fmt.Errorf("category %q still has %d child categor(y/ies) and cannot be deleted", in.ID, childCount)
		}
		if txCount != 0 {
			return nil, ManageCategoryOutput{}, fmt.Errorf("category %q is still referenced by %d transaction(s) and cannot be deleted", in.ID, txCount)
		}

		token, expiresAt := confirm.Issue(deps.ConfirmSecret, confirmActionDeleteCategory, in.ID, revision)
		return nil, ManageCategoryOutput{
			Status:            "pending_confirmation",
			Category:          info,
			ConfirmationToken: token,
			ExpiresAt:         expiresAt,
		}, nil
	}

	if err := confirm.Verify(deps.ConfirmSecret, in.ConfirmationToken, confirmActionDeleteCategory, in.ID, revision, time.Now()); err != nil {
		return nil, ManageCategoryOutput{}, err
	}

	tx, err := deps.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, ManageCategoryOutput{}, err
	}
	defer tx.Rollback()

	q := deps.Q.WithTx(tx)

	liveChildCount, err := q.CountChildCategories(ctx, id)
	if err != nil {
		return nil, ManageCategoryOutput{}, err
	}
	liveTxCount, err := q.CountTransactionsByCategory(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		return nil, ManageCategoryOutput{}, err
	}
	if liveChildCount != 0 || liveTxCount != 0 {
		return nil, ManageCategoryOutput{}, fmt.Errorf("category %q can no longer be deleted (children=%d, transactions=%d); please preview again", in.ID, liveChildCount, liveTxCount)
	}

	if err := q.DeleteCategory(ctx, id); err != nil {
		return nil, ManageCategoryOutput{}, err
	}

	if err := tx.Commit(); err != nil {
		return nil, ManageCategoryOutput{}, err
	}

	return nil, ManageCategoryOutput{Status: "deleted", Category: info}, nil
}

func toCategoryInfo(c store.Category) CategoryInfo {
	return CategoryInfo{
		ID:       formatID(c.ID),
		Name:     c.Name,
		ParentID: formatID(c.ParentID),
	}
}
