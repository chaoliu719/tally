package tools_test

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/tools"
)

func TestListCategoriesEmptyLedger(t *testing.T) {
	session := newTestSession(t)

	var out tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{}, &out)

	if out.Categories == nil {
		t.Fatal("expected an empty slice, got nil")
	}
	if len(out.Categories) != 0 {
		t.Fatalf("expected 0 categories, got %d", len(out.Categories))
	}
}

func TestManageCategoryCreateTopLevel(t *testing.T) {
	session := newTestSession(t)

	var created tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Food",
	}, &created)

	if created.Status != "created" {
		t.Errorf("Status = %q, want %q", created.Status, "created")
	}
	if created.Category.Name != "Food" {
		t.Errorf("Name = %q, want %q", created.Category.Name, "Food")
	}
	if created.Category.ParentID != "0" {
		t.Errorf("ParentID = %q, want %q", created.Category.ParentID, "0")
	}

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{}, &list)

	if len(list.Categories) != 1 {
		t.Fatalf("expected 1 category after creation, got %d", len(list.Categories))
	}
	if list.Categories[0].ID != created.Category.ID {
		t.Errorf("listed category id = %q, want %q", list.Categories[0].ID, created.Category.ID)
	}
}

func TestManageCategoryCreateNested(t *testing.T) {
	session := newTestSession(t)

	parentID := createTestCategory(t, session, "Food", "")
	childID := createTestCategory(t, session, "Groceries", parentID)

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{}, &list)
	if len(list.Categories) != 2 {
		t.Fatalf("expected 2 categories after creation, got %d", len(list.Categories))
	}

	_ = childID
}

// TestManageCategoryAllowsArbitraryDepth verifies the two-level restriction
// is gone: a category can be created under a category that already has a
// parent itself (a former "second-level" category), and so on to any depth.
func TestManageCategoryAllowsArbitraryDepth(t *testing.T) {
	session := newTestSession(t)

	topID := createTestCategory(t, session, "Food", "")
	midID := createTestCategory(t, session, "Groceries", topID)
	leafID := createTestCategory(t, session, "Supermarket", midID)

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{}, &list)
	if len(list.Categories) != 3 {
		t.Fatalf("expected 3 categories after creation, got %d", len(list.Categories))
	}

	var leaf tools.CategoryInfo
	for _, c := range list.Categories {
		if c.ID == leafID {
			leaf = c
		}
	}
	if leaf.ParentID != midID {
		t.Errorf("leaf ParentID = %q, want %q", leaf.ParentID, midID)
	}
}

func TestManageCategoryRejectsNonexistentParent(t *testing.T) {
	session := newTestSession(t)

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Groceries",
		ParentID:  "999999",
	})

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{}, &list)
	if len(list.Categories) != 0 {
		t.Fatalf("expected no category created, got %d", len(list.Categories))
	}
}

func TestManageCategoryMissingRequiredField(t *testing.T) {
	session := newTestSession(t)

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
	})

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{}, &list)
	if len(list.Categories) != 0 {
		t.Fatalf("expected no category created, got %d", len(list.Categories))
	}
}

// createTestCategory is a small helper shared by the tests in this file; it
// creates a category (top-level if parentID is "") and returns its id.
func createTestCategory(t *testing.T, session *mcp.ClientSession, name, parentID string) string {
	t.Helper()

	var created tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      name,
		ParentID:  parentID,
	}, &created)

	return created.Category.ID
}

func TestManageCategoryUpdateName(t *testing.T) {
	session := newTestSession(t)
	id := createTestCategory(t, session, "Food", "")

	var updated tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "update",
		ID:        id,
		Name:      "Groceries & Food",
	}, &updated)

	if updated.Status != "updated" {
		t.Errorf("Status = %q, want %q", updated.Status, "updated")
	}
	if updated.Category.Name != "Groceries & Food" {
		t.Errorf("Name = %q, want %q", updated.Category.Name, "Groceries & Food")
	}
	if updated.Category.ParentID != "0" {
		t.Errorf("ParentID = %q, want %q (unchanged)", updated.Category.ParentID, "0")
	}
}

func TestManageCategoryUpdateMove(t *testing.T) {
	session := newTestSession(t)
	oldParent := createTestCategory(t, session, "Food", "")
	newParent := createTestCategory(t, session, "Household", "")
	child := createTestCategory(t, session, "Groceries", oldParent)

	var updated tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "update",
		ID:        child,
		Name:      "Groceries",
		ParentID:  newParent,
	}, &updated)

	if updated.Category.ParentID != newParent {
		t.Errorf("ParentID = %q, want %q", updated.Category.ParentID, newParent)
	}

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{}, &list)
	for _, c := range list.Categories {
		if c.ID == child && c.ParentID != newParent {
			t.Errorf("listed ParentID = %q, want %q", c.ParentID, newParent)
		}
	}
}

func TestManageCategoryUpdateNotFound(t *testing.T) {
	session := newTestSession(t)

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "update",
		ID:        "999999",
		Name:      "Ghost",
	})
}

func TestManageCategoryUpdateMissingRequiredField(t *testing.T) {
	session := newTestSession(t)
	id := createTestCategory(t, session, "Food", "")

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "update",
		ID:        id,
		// Name omitted
	})

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{}, &list)
	if list.Categories[0].Name != "Food" {
		t.Fatalf("category was modified despite missing required field: name = %q", list.Categories[0].Name)
	}
}

func TestManageCategoryUpdateRejectsSelfReference(t *testing.T) {
	session := newTestSession(t)
	id := createTestCategory(t, session, "Food", "")

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "update",
		ID:        id,
		Name:      "Food",
		ParentID:  id,
	})
}

func TestManageCategoryUpdateRejectsCycle(t *testing.T) {
	session := newTestSession(t)
	top := createTestCategory(t, session, "Food", "")
	mid := createTestCategory(t, session, "Groceries", top)
	leaf := createTestCategory(t, session, "Supermarket", mid)

	// Moving "Food" (top) under its own grandchild "Supermarket" (leaf)
	// would create a cycle.
	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "update",
		ID:        top,
		Name:      "Food",
		ParentID:  leaf,
	})

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{}, &list)
	for _, c := range list.Categories {
		if c.ID == top && c.ParentID != "0" {
			t.Fatalf("cycle-forming move was not rejected: top's ParentID = %q", c.ParentID)
		}
	}
}

func TestManageCategoryUpdateRejectsNonexistentParent(t *testing.T) {
	session := newTestSession(t)
	id := createTestCategory(t, session, "Food", "")

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "update",
		ID:        id,
		Name:      "Food",
		ParentID:  "999999",
	})
}

func TestManageCategoryDeleteHappyPath(t *testing.T) {
	session := newTestSession(t)
	id := createTestCategory(t, session, "Food", "")

	var preview tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "delete",
		ID:        id,
	}, &preview)

	if preview.Status != "pending_confirmation" {
		t.Fatalf("Status = %q, want %q", preview.Status, "pending_confirmation")
	}
	if preview.ConfirmationToken == "" {
		t.Fatal("expected a non-empty confirmation_token")
	}

	var applied tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation:         "delete",
		ID:                id,
		ConfirmationToken: preview.ConfirmationToken,
	}, &applied)

	if applied.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", applied.Status, "deleted")
	}

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{}, &list)
	if len(list.Categories) != 0 {
		t.Fatalf("expected the category to be gone, got %d categories", len(list.Categories))
	}
}

func TestManageCategoryDeleteBlockedByChildren(t *testing.T) {
	session := newTestSession(t)
	parent := createTestCategory(t, session, "Food", "")
	createTestCategory(t, session, "Groceries", parent)

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "delete",
		ID:        parent,
	})

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{}, &list)
	if len(list.Categories) != 2 {
		t.Fatalf("expected both categories to survive a blocked delete, got %d", len(list.Categories))
	}
}

func TestManageCategoryDeleteBlockedByReferences(t *testing.T) {
	session := newTestSession(t)
	categoryID := createTestCategory(t, session, "Food", "")
	accountID := createTestAccount(t, session, 10000)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		AccountID:  accountID,
		CategoryID: categoryID,
		Amount:     100,
		Time:       futureTime(),
	}, &tools.CreateTransactionOutput{})

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "delete",
		ID:        categoryID,
	})

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{}, &list)
	if len(list.Categories) != 1 {
		t.Fatalf("expected the category to survive a blocked delete, got %d", len(list.Categories))
	}
}

func TestManageCategoryDeleteNotFound(t *testing.T) {
	session := newTestSession(t)

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "delete",
		ID:        "999999",
	})
}
