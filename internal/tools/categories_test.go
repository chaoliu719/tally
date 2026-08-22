package tools_test

import (
	"testing"

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
		Name: "Food",
		Type: "expense",
	}, &created)

	if created.Category.Name != "Food" {
		t.Errorf("Name = %q, want %q", created.Category.Name, "Food")
	}
	if created.Category.Type != "expense" {
		t.Errorf("Type = %q, want %q", created.Category.Type, "expense")
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

func TestManageCategoryCreateSecondLevel(t *testing.T) {
	session := newTestSession(t)

	var parent tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Name: "Food",
		Type: "expense",
	}, &parent)

	var child tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Name:     "Groceries",
		Type:     "expense",
		ParentID: parent.Category.ID,
	}, &child)

	if child.Category.ParentID != parent.Category.ID {
		t.Errorf("ParentID = %q, want %q", child.Category.ParentID, parent.Category.ID)
	}

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{}, &list)
	if len(list.Categories) != 2 {
		t.Fatalf("expected 2 categories after creation, got %d", len(list.Categories))
	}
}

func TestManageCategoryRejectsThirdLevel(t *testing.T) {
	session := newTestSession(t)

	var parent tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Name: "Food",
		Type: "expense",
	}, &parent)

	var child tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Name:     "Groceries",
		Type:     "expense",
		ParentID: parent.Category.ID,
	}, &child)

	// Attempting to create a category under the second-level "Groceries"
	// category must be rejected -- only two levels are supported.
	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		Name:     "Supermarket",
		Type:     "expense",
		ParentID: child.Category.ID,
	})

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{}, &list)
	if len(list.Categories) != 2 {
		t.Fatalf("expected no third-level category to be created, got %d categories", len(list.Categories))
	}
}

func TestManageCategoryRejectsNonexistentParent(t *testing.T) {
	session := newTestSession(t)

	callToolExpectError(t, session, "manage_category", tools.ManageCategoryInput{
		Name:     "Groceries",
		Type:     "expense",
		ParentID: "999999",
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
		Type: "expense",
	})

	var list tools.ListCategoriesOutput
	callTool(t, session, "list_categories", tools.ListCategoriesInput{}, &list)
	if len(list.Categories) != 0 {
		t.Fatalf("expected no category created, got %d", len(list.Categories))
	}
}
