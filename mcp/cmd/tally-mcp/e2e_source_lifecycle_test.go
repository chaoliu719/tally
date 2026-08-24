package main

import (
	"testing"

	"tally/internal/tools"
)

// TestE2ESourceLifecycle drives two sources through the source-management
// capabilities, all through main()'s real buildMux wiring.
//
// Source A demonstrates the update -> preview-delete -> apply-delete chain
// on a source that never records a transaction. Source B records a
// transaction against it and is intentionally never deleted -- per
// specs/source-management/spec.md, a source referenced by any transaction
// can never be deleted directly. delete_transaction can clear that
// reference first -- see TestE2ETransactionLifecycleUnblocksSourceAndCategoryDeletion
// for that path -- but this test keeps the two concerns apart.
func TestE2ESourceLifecycle(t *testing.T) {
	session, ledgerID := newE2ESession(t)

	// Source A: create -> update -> preview delete -> apply delete.
	var sourceA tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID,
		Operation: "create",
		Name:      "Checking",
	}, &sourceA)

	var updatedA tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID,
		Operation: "update",
		ID:        sourceA.Source.ID,
		Name:      "Checking (renamed)",
	}, &updatedA)
	if updatedA.Status != "updated" {
		t.Fatalf("Status = %q, want %q", updatedA.Status, "updated")
	}

	var afterUpdate tools.ListSourcesOutput
	call(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerID}, &afterUpdate)
	if len(afterUpdate.Sources) != 1 {
		t.Fatalf("expected 1 source (only A exists so far), got %d", len(afterUpdate.Sources))
	}
	if got := afterUpdate.Sources[0]; got.Name != "Checking (renamed)" {
		t.Fatalf("source A after update = %+v, want name %q", got, "Checking (renamed)")
	}

	var previewA tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID,
		Operation: "delete",
		ID:        sourceA.Source.ID,
	}, &previewA)
	if previewA.Status != "pending_confirmation" {
		t.Fatalf("Status = %q, want %q", previewA.Status, "pending_confirmation")
	}
	if previewA.ConfirmationToken == "" {
		t.Fatal("expected a non-empty confirmation_token")
	}

	var appliedA tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID,
		Operation:         "delete",
		ID:                sourceA.Source.ID,
		ConfirmationToken: previewA.ConfirmationToken,
	}, &appliedA)
	if appliedA.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", appliedA.Status, "deleted")
	}

	// Source B: create -> record a transaction against it -> never deleted --
	// the transaction now references it, which correctly and permanently
	// blocks deletion.
	var sourceB tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID,
		Operation: "create",
		Name:      "Savings",
	}, &sourceB)

	var category tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{LedgerID: ledgerID,
		Operation: "create",
		Name:      "Misc",
	}, &category)

	var expense tools.CreateTransactionOutput
	call(t, session, "create_transaction", tools.CreateTransactionInput{LedgerID: ledgerID,
		Type:       "expense",
		SourceID:   sourceB.Source.ID,
		CategoryID: category.Category.ID,
		Amount:     cnyAmount(500),
		Currency:   "CNY",
		Time:       futureTime(),
		Comment:    "keeps source B referenced",
	}, &expense)

	callExpectError(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID,
		Operation: "delete",
		ID:        sourceB.Source.ID,
	})

	var afterExpense tools.ListSourcesOutput
	call(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerID}, &afterExpense)

	var foundB bool
	for _, s := range afterExpense.Sources {
		if s.ID == sourceB.Source.ID {
			foundB = true
		}
	}
	if !foundB {
		t.Fatalf("source B (id %q) missing from list_sources: %+v", sourceB.Source.ID, afterExpense.Sources)
	}

	// Source A must remain deleted; only source B should be left.
	if len(afterExpense.Sources) != 1 {
		t.Fatalf("expected only source B to remain, got %d sources: %+v", len(afterExpense.Sources), afterExpense.Sources)
	}
}
