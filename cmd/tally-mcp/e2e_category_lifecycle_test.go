package main

import (
	"testing"

	"tally/internal/tools"
)

// TestE2ECategoryLifecycle drives a category tree through the
// category-management capabilities added by the account-category-lifecycle
// change, all through main()'s real buildMux wiring: build a three-level
// nested tree, move a node to a different parent, then delete a leaf
// category.
func TestE2ECategoryLifecycle(t *testing.T) {
	session := newE2ESession(t)

	// Build a three-level tree: Expense -> Dining -> Delivery.
	var expense tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Expense",
	}, &expense)

	var dining tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Dining",
		ParentID:  expense.Category.ID,
	}, &dining)

	var delivery tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Delivery",
		ParentID:  dining.Category.ID,
	}, &delivery)

	var afterCreate tools.ListCategoriesOutput
	call(t, session, "list_categories", tools.ListCategoriesInput{}, &afterCreate)
	if len(afterCreate.Categories) != 3 {
		t.Fatalf("expected 3 categories after building the tree, got %d", len(afterCreate.Categories))
	}

	// Move Delivery out from under Dining, directly under Expense --
	// exercising arbitrary-depth nesting and the move-with-cycle-check path.
	var moved tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "update",
		ID:        delivery.Category.ID,
		Name:      "Delivery",
		ParentID:  expense.Category.ID,
	}, &moved)
	if moved.Status != "updated" {
		t.Fatalf("Status = %q, want %q", moved.Status, "updated")
	}

	var afterMove tools.ListCategoriesOutput
	call(t, session, "list_categories", tools.ListCategoriesInput{}, &afterMove)
	var foundMoved bool
	for _, c := range afterMove.Categories {
		if c.ID != delivery.Category.ID {
			continue
		}
		foundMoved = true
		if c.ParentID != expense.Category.ID {
			t.Fatalf("Delivery's parent_id = %q, want %q (Expense)", c.ParentID, expense.Category.ID)
		}
	}
	if !foundMoved {
		t.Fatalf("Delivery (id %q) missing from list_categories: %+v", delivery.Category.ID, afterMove.Categories)
	}

	// Delete the now-leaf Delivery category via preview -> apply.
	var preview tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "delete",
		ID:        delivery.Category.ID,
	}, &preview)
	if preview.Status != "pending_confirmation" {
		t.Fatalf("Status = %q, want %q", preview.Status, "pending_confirmation")
	}
	if preview.ConfirmationToken == "" {
		t.Fatal("expected a non-empty confirmation_token")
	}

	var applied tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{
		Operation:         "delete",
		ID:                delivery.Category.ID,
		ConfirmationToken: preview.ConfirmationToken,
	}, &applied)
	if applied.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", applied.Status, "deleted")
	}

	var final tools.ListCategoriesOutput
	call(t, session, "list_categories", tools.ListCategoriesInput{}, &final)
	if len(final.Categories) != 2 {
		t.Fatalf("expected 2 categories (Expense, Dining) after deleting Delivery, got %d: %+v", len(final.Categories), final.Categories)
	}
	for _, c := range final.Categories {
		if c.ID == delivery.Category.ID {
			t.Fatalf("Delivery (id %q) should have been deleted, still present: %+v", delivery.Category.ID, final.Categories)
		}
	}
}
