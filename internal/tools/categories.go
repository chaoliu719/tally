package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func init() {
	register(registerCategoryTools)
}

func registerCategoryTools(s *mcp.Server, deps Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_categories",
		Description: "List every transaction category in the ledger, including its name, type (income, expense, or transfer), and parent category id. A category with parent_id 0 is a top-level (primary) category, used only for grouping; it cannot be used in create_transaction. A category with a non-zero parent_id is a second-level category and can be used in create_transaction.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ ListCategoriesInput) (*mcp.CallToolResult, ListCategoriesOutput, error) {
		return listCategories(deps)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "manage_category",
		Description: "Create a new transaction category. Categories are two levels deep, matching ezbookkeeping: omit parent_id (or pass 0) to create a top-level category for grouping; pass the id of an existing top-level category to create a second-level category under it. Only second-level categories can be used in create_transaction.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in ManageCategoryInput) (*mcp.CallToolResult, ManageCategoryOutput, error) {
		return manageCategory(deps, in)
	})
}

var categoryTypeByName = map[string]models.TransactionCategoryType{
	"income":   models.CATEGORY_TYPE_INCOME,
	"expense":  models.CATEGORY_TYPE_EXPENSE,
	"transfer": models.CATEGORY_TYPE_TRANSFER,
}

var categoryTypeNames = func() map[models.TransactionCategoryType]string {
	names := make(map[models.TransactionCategoryType]string, len(categoryTypeByName))
	for name, t := range categoryTypeByName {
		names[t] = name
	}
	return names
}()

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

func listCategories(deps Deps) (*mcp.CallToolResult, ListCategoriesOutput, error) {
	ctx := core.NewNullContext()

	// categoryType=0 means "all types", parentCategoryId=-1 means "all parents".
	categories, err := services.TransactionCategories.GetAllCategoriesByUid(ctx, deps.UID, 0, -1)
	if err != nil {
		return nil, ListCategoriesOutput{}, err
	}

	infos := make([]CategoryInfo, 0, len(categories))
	for _, c := range categories {
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

func manageCategory(deps Deps, in ManageCategoryInput) (*mcp.CallToolResult, ManageCategoryOutput, error) {
	if in.Name == "" {
		return nil, ManageCategoryOutput{}, fmt.Errorf("missing required field: name")
	}

	categoryType, ok := categoryTypeByName[in.Type]
	if !ok {
		return nil, ManageCategoryOutput{}, fmt.Errorf("missing or unsupported category type: %q", in.Type)
	}

	var parentID int64
	if in.ParentID != "" {
		var err error
		parentID, err = parseID(in.ParentID)
		if err != nil {
			return nil, ManageCategoryOutput{}, err
		}
	}

	ctx := core.NewNullContext()

	var displayOrder int32

	if parentID == models.LevelOneTransactionCategoryParentId {
		maxOrder, err := services.TransactionCategories.GetMaxDisplayOrder(ctx, deps.UID, categoryType)
		if err != nil {
			return nil, ManageCategoryOutput{}, err
		}
		displayOrder = maxOrder + 1
	} else {
		parent, err := services.TransactionCategories.GetCategoryByCategoryId(ctx, deps.UID, parentID)
		if err != nil {
			if errors.Is(err, errs.ErrTransactionCategoryNotFound) {
				return nil, ManageCategoryOutput{}, errs.ErrParentTransactionCategoryNotFound
			}
			return nil, ManageCategoryOutput{}, err
		}
		if parent.ParentCategoryId != models.LevelOneTransactionCategoryParentId {
			return nil, ManageCategoryOutput{}, errs.ErrCannotAddToSecondaryTransactionCategory
		}

		maxOrder, err := services.TransactionCategories.GetMaxSubCategoryDisplayOrder(ctx, deps.UID, categoryType, parentID)
		if err != nil {
			return nil, ManageCategoryOutput{}, err
		}
		displayOrder = maxOrder + 1
	}

	category := &models.TransactionCategory{
		Uid:              deps.UID,
		Type:             categoryType,
		ParentCategoryId: parentID,
		Name:             in.Name,
		DisplayOrder:     displayOrder,
		Icon:             1,
		IconType:         core.ICON_TYPE_SYSTEM,
		Color:            "000000",
	}

	if err := services.TransactionCategories.CreateCategory(ctx, category); err != nil {
		return nil, ManageCategoryOutput{}, err
	}

	return nil, ManageCategoryOutput{Category: toCategoryInfo(category)}, nil
}

func toCategoryInfo(c *models.TransactionCategory) CategoryInfo {
	return CategoryInfo{
		ID:       formatID(c.CategoryId),
		Name:     c.Name,
		Type:     categoryTypeNames[c.Type],
		ParentID: formatID(c.ParentCategoryId),
	}
}
