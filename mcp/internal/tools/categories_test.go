package tools_test

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/tools"
)

func TestListCategoriesEmptyLedger(t *testing.T) {
	session, ledgerID := newTestSession(t)

	var out tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerID}, &out)

	if out.Categories == nil {
		t.Fatal("expected an empty slice, got nil")
	}
	if len(out.Categories) != 0 {
		t.Fatalf("expected 0 categories, got %d", len(out.Categories))
	}
}

func TestManageCategoryCreateTopLevel(t *testing.T) {
	session, ledgerID := newTestSession(t)

	var created tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
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
	callTool(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerID}, &list)

	if len(list.Categories) != 1 {
		t.Fatalf("expected 1 category after creation, got %d", len(list.Categories))
	}
	if list.Categories[0].ID != created.Category.ID {
		t.Errorf("listed category id = %q, want %q", list.Categories[0].ID, created.Category.ID)
	}
}

func TestManageCategoryCreateNested(t *testing.T) {
	session, ledgerID := newTestSession(t)

	parentID := createTestCategory(t, session, ledgerID, "Food", "")
	childID := createTestCategory(t, session, ledgerID, "Groceries", parentID)

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerID}, &list)
	if len(list.Categories) != 2 {
		t.Fatalf("expected 2 categories after creation, got %d", len(list.Categories))
	}

	_ = childID
}

// TestManageCategoryAllowsArbitraryDepth verifies the two-level restriction
// is gone: a category can be created under a category that already has a
// parent itself (a former "second-level" category), and so on to any depth.
func TestManageCategoryAllowsArbitraryDepth(t *testing.T) {
	session, ledgerID := newTestSession(t)

	topID := createTestCategory(t, session, ledgerID, "Food", "")
	midID := createTestCategory(t, session, ledgerID, "Groceries", topID)
	leafID := createTestCategory(t, session, ledgerID, "Supermarket", midID)

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerID}, &list)
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
	session, ledgerID := newTestSession(t)

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      "Groceries",
		ParentID:  "999999",
	})

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerID}, &list)
	if len(list.Categories) != 0 {
		t.Fatalf("expected no category created, got %d", len(list.Categories))
	}
}

func TestManageCategoryMissingRequiredField(t *testing.T) {
	session, ledgerID := newTestSession(t)

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "create",
	})

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerID}, &list)
	if len(list.Categories) != 0 {
		t.Fatalf("expected no category created, got %d", len(list.Categories))
	}
}

// createTestCategory is a small helper shared by the tests in this file; it
// creates a category (top-level if parentID is "") and returns its id.
func createTestCategory(t *testing.T, session *mcp.ClientSession, ledgerID, name, parentID string) string {
	t.Helper()

	var created tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      name,
		ParentID:  parentID,
	}, &created)

	return created.Category.ID
}

func TestManageCategoryUpdateName(t *testing.T) {
	session, ledgerID := newTestSession(t)
	id := createTestCategory(t, session, ledgerID, "Food", "")

	var updated tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
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
	session, ledgerID := newTestSession(t)
	oldParent := createTestCategory(t, session, ledgerID, "Food", "")
	newParent := createTestCategory(t, session, ledgerID, "Household", "")
	child := createTestCategory(t, session, ledgerID, "Groceries", oldParent)

	var updated tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "update",
		ID:        child,
		Name:      "Groceries",
		ParentID:  newParent,
	}, &updated)

	if updated.Category.ParentID != newParent {
		t.Errorf("ParentID = %q, want %q", updated.Category.ParentID, newParent)
	}

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerID}, &list)
	for _, c := range list.Categories {
		if c.ID == child && c.ParentID != newParent {
			t.Errorf("listed ParentID = %q, want %q", c.ParentID, newParent)
		}
	}
}

func TestManageCategoryUpdateNotFound(t *testing.T) {
	session, ledgerID := newTestSession(t)

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "update",
		ID:        "999999",
		Name:      "Ghost",
	})
}

func TestManageCategoryUpdateMissingRequiredField(t *testing.T) {
	session, ledgerID := newTestSession(t)
	id := createTestCategory(t, session, ledgerID, "Food", "")

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "update",
		ID:        id,
		// Name omitted
	})

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerID}, &list)
	if list.Categories[0].Name != "Food" {
		t.Fatalf("category was modified despite missing required field: name = %q", list.Categories[0].Name)
	}
}

func TestManageCategoryUpdateRejectsSelfReference(t *testing.T) {
	session, ledgerID := newTestSession(t)
	id := createTestCategory(t, session, ledgerID, "Food", "")

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "update",
		ID:        id,
		Name:      "Food",
		ParentID:  id,
	})
}

func TestManageCategoryUpdateRejectsCycle(t *testing.T) {
	session, ledgerID := newTestSession(t)
	top := createTestCategory(t, session, ledgerID, "Food", "")
	mid := createTestCategory(t, session, ledgerID, "Groceries", top)
	leaf := createTestCategory(t, session, ledgerID, "Supermarket", mid)

	// Moving "Food" (top) under its own grandchild "Supermarket" (leaf)
	// would create a cycle.
	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "update",
		ID:        top,
		Name:      "Food",
		ParentID:  leaf,
	})

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerID}, &list)
	for _, c := range list.Categories {
		if c.ID == top && c.ParentID != "0" {
			t.Fatalf("cycle-forming move was not rejected: top's ParentID = %q", c.ParentID)
		}
	}
}

func TestManageCategoryUpdateRejectsNonexistentParent(t *testing.T) {
	session, ledgerID := newTestSession(t)
	id := createTestCategory(t, session, ledgerID, "Food", "")

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "update",
		ID:        id,
		Name:      "Food",
		ParentID:  "999999",
	})
}

func TestManageCategoryDeleteHappyPath(t *testing.T) {
	session, ledgerID := newTestSession(t)
	id := createTestCategory(t, session, ledgerID, "Food", "")

	var preview tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
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
		LedgerID:          ledgerID,
		Operation:         "delete",
		ID:                id,
		ConfirmationToken: preview.ConfirmationToken,
	}, &applied)

	if applied.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", applied.Status, "deleted")
	}

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerID}, &list)
	if len(list.Categories) != 0 {
		t.Fatalf("expected the category to be gone, got %d categories", len(list.Categories))
	}
}

func TestManageCategoryDeleteBlockedByChildren(t *testing.T) {
	session, ledgerID := newTestSession(t)
	parent := createTestCategory(t, session, ledgerID, "Food", "")
	createTestCategory(t, session, ledgerID, "Groceries", parent)

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "delete",
		ID:        parent,
	})

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerID}, &list)
	if len(list.Categories) != 2 {
		t.Fatalf("expected both categories to survive a blocked delete, got %d", len(list.Categories))
	}
}

func TestManageCategoryDeleteBlockedByReferences(t *testing.T) {
	session, ledgerID := newTestSession(t)
	categoryID := createTestCategory(t, session, ledgerID, "Food", "")
	sourceID := createTestSource(t, session, ledgerID)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     cnyAmount(100),
		Currency:   "CNY",
		Time:       futureTime(),
	}, &tools.CreateTransactionOutput{})

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "delete",
		ID:        categoryID,
	})

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerID}, &list)
	if len(list.Categories) != 1 {
		t.Fatalf("expected the category to survive a blocked delete, got %d", len(list.Categories))
	}
}

func TestManageCategoryDeleteTokenReplay(t *testing.T) {
	session, ledgerID := newTestSession(t)
	id := createTestCategory(t, session, ledgerID, "Food", "")

	var preview tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "delete",
		ID:        id,
	}, &preview)

	var applied tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:          ledgerID,
		Operation:         "delete",
		ID:                id,
		ConfirmationToken: preview.ConfirmationToken,
	}, &applied)
	if applied.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", applied.Status, "deleted")
	}

	// Reusing the same token to confirm the same delete a second time must
	// fail -- the category is already gone, so the live recheck at apply
	// time rejects it even though the token itself is still validly signed
	// and unexpired.
	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:          ledgerID,
		Operation:         "delete",
		ID:                id,
		ConfirmationToken: preview.ConfirmationToken,
	})

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerID}, &list)
	if len(list.Categories) != 0 {
		t.Fatalf("expected the category to remain deleted, got %d categories", len(list.Categories))
	}
}

func TestManageCategoryDeleteNotFound(t *testing.T) {
	session, ledgerID := newTestSession(t)

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "delete",
		ID:        "999999",
	})
}

func TestListCategoriesIsolatedByLedger(t *testing.T) {
	session, ledgerA := newTestSession(t)
	createTestCategory(t, session, ledgerA, "Food", "")

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{LedgerID: ledgerB}, &list)
	if len(list.Categories) != 0 {
		t.Fatalf("expected ledgerB to have no categories, got %d (ledgerA's category leaked in)", len(list.Categories))
	}
}

func TestManageCategoryCreateRejectsParentFromAnotherLedger(t *testing.T) {
	session, ledgerA := newTestSession(t)

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID
	parentInB := createTestCategory(t, session, ledgerB, "Food", "")

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerA,
		Operation: "create",
		Name:      "Groceries",
		ParentID:  parentInB,
	})
}

func TestManageCategoryUpdateRejectsCategoryFromAnotherLedger(t *testing.T) {
	session, ledgerA := newTestSession(t)
	id := createTestCategory(t, session, ledgerA, "Food", "")

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerB,
		Operation: "update",
		ID:        id,
		Name:      "Hijacked",
	})
}

func TestManageCategoryUpdateRejectsParentFromAnotherLedger(t *testing.T) {
	session, ledgerA := newTestSession(t)
	id := createTestCategory(t, session, ledgerA, "Food", "")

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID
	parentInB := createTestCategory(t, session, ledgerB, "Household", "")

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerA,
		Operation: "update",
		ID:        id,
		Name:      "Food",
		ParentID:  parentInB,
	})
}

func TestManageCategoryDeleteRejectsCategoryFromAnotherLedger(t *testing.T) {
	session, ledgerA := newTestSession(t)
	id := createTestCategory(t, session, ledgerA, "Food", "")

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerB,
		Operation: "delete",
		ID:        id,
	})
}
