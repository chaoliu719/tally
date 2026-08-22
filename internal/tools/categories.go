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
	register(registerCategoryTools)
}

func registerCategoryTools(s *mcp.Server, deps Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_categories",
		Description: "List every transaction category in the ledger, including its name, type (income, expense, or transfer), and parent category id. A category with parent_id 0 is a top-level (primary) category, used only for grouping; it cannot be used in create_transaction. A category with a non-zero parent_id is a second-level category and can be used in create_transaction.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ ListCategoriesInput) (*mcp.CallToolResult, ListCategoriesOutput, error) {
		return listCategories(ctx, deps)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "manage_category",
		Description: "Create a new transaction category. Categories are two levels deep: omit parent_id (or pass 0) to create a top-level category for grouping; pass the id of an existing top-level category to create a second-level category under it. Only second-level categories can be used in create_transaction.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ManageCategoryInput) (*mcp.CallToolResult, ManageCategoryOutput, error) {
		return manageCategory(ctx, deps, in)
	})
}

var categoryTypes = map[string]bool{
	"income":   true,
	"expense":  true,
	"transfer": true,
}

// topLevelParentID is the parent_id value that marks a category as
// top-level, matching the schema.sql default.
const topLevelParentID int64 = 0

// CategoryInfo is the wire representation of a transaction category returned
// by list_categories and manage_category.
type CategoryInfo struct {
	ID       string `json:"id" jsonschema:"the category's unique id, as a decimal string"`
	Name     string `json:"name" jsonschema:"the category's name"`
	Type     string `json:"type" jsonschema:"the category's type: income, expense, or transfer"`
	ParentID string `json:"parent_id" jsonschema:"\"0\" for a top-level category; otherwise the id of the top-level category this second-level category belongs to, as a decimal string"`
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
	Name     string `json:"name" jsonschema:"the category's name"`
	Type     string `json:"type" jsonschema:"the category's type: income, expense, or transfer"`
	ParentID string `json:"parent_id,omitempty" jsonschema:"omit or pass \"0\" to create a top-level category; pass the id (decimal string) of an existing top-level category to create a second-level category under it"`
}

type ManageCategoryOutput struct {
	Category CategoryInfo `json:"category" jsonschema:"the newly created category"`
}

func manageCategory(ctx context.Context, deps Deps, in ManageCategoryInput) (*mcp.CallToolResult, ManageCategoryOutput, error) {
	if in.Name == "" {
		return nil, ManageCategoryOutput{}, fmt.Errorf("missing required field: name")
	}
	if !categoryTypes[in.Type] {
		return nil, ManageCategoryOutput{}, fmt.Errorf("missing or unsupported category type: %q", in.Type)
	}

	parentID := topLevelParentID
	if in.ParentID != "" {
		var err error
		parentID, err = parseID(in.ParentID)
		if err != nil {
			return nil, ManageCategoryOutput{}, err
		}
	}

	if parentID != topLevelParentID {
		parent, err := deps.Q.GetCategory(ctx, parentID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ManageCategoryOutput{}, fmt.Errorf("parent category %q not found", in.ParentID)
			}
			return nil, ManageCategoryOutput{}, err
		}
		if parent.ParentID != topLevelParentID {
			return nil, ManageCategoryOutput{}, fmt.Errorf("category tree only supports two levels: cannot create a category under second-level category %q", in.ParentID)
		}
	}

	now := time.Now().Unix()
	category, err := deps.Q.CreateCategory(ctx, store.CreateCategoryParams{
		Name:      in.Name,
		Type:      in.Type,
		ParentID:  parentID,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return nil, ManageCategoryOutput{}, err
	}

	return nil, ManageCategoryOutput{Category: toCategoryInfo(category)}, nil
}

func toCategoryInfo(c store.Category) CategoryInfo {
	return CategoryInfo{
		ID:       formatID(c.ID),
		Name:     c.Name,
		Type:     c.Type,
		ParentID: formatID(c.ParentID),
	}
}
