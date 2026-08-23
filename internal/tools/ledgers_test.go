package tools_test

import (
	"testing"
	"time"

	"tally/internal/tools"
)

func TestListLedgersReturnsTheDefaultLedger(t *testing.T) {
	session, ledgerID := newTestSession(t)

	var out tools.ListLedgersOutput
	callTool(t, session, "list_ledgers", tools.ListLedgersInput{}, &out)

	if len(out.Ledgers) != 1 {
		t.Fatalf("expected 1 ledger, got %d", len(out.Ledgers))
	}
	if out.Ledgers[0].ID != ledgerID {
		t.Errorf("listed ledger id = %q, want %q", out.Ledgers[0].ID, ledgerID)
	}
}

func TestManageLedgerCreate(t *testing.T) {
	session, _ := newTestSession(t)

	var created tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "create",
		Name:      "Company",
		Comment:   "business expenses",
	}, &created)

	if created.Status != "created" {
		t.Errorf("Status = %q, want %q", created.Status, "created")
	}
	if created.Ledger.Name != "Company" {
		t.Errorf("Name = %q, want %q", created.Ledger.Name, "Company")
	}
	if created.Ledger.Comment != "business expenses" {
		t.Errorf("Comment = %q, want %q", created.Ledger.Comment, "business expenses")
	}

	var list tools.ListLedgersOutput
	callTool(t, session, "list_ledgers", tools.ListLedgersInput{}, &list)
	if len(list.Ledgers) != 2 {
		t.Fatalf("expected 2 ledgers (default + created), got %d", len(list.Ledgers))
	}
}

func TestManageLedgerCreateMissingRequiredField(t *testing.T) {
	session, _ := newTestSession(t)

	callToolExpectError(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "create",
	})

	var list tools.ListLedgersOutput
	callTool(t, session, "list_ledgers", tools.ListLedgersInput{}, &list)
	if len(list.Ledgers) != 1 {
		t.Fatalf("expected only the default ledger, got %d", len(list.Ledgers))
	}
}

func TestManageLedgerUpdateNameAndComment(t *testing.T) {
	session, ledgerID := newTestSession(t)

	var updated tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "update",
		ID:        ledgerID,
		Name:      "Renamed Ledger",
		Comment:   "updated comment",
	}, &updated)

	if updated.Status != "updated" {
		t.Errorf("Status = %q, want %q", updated.Status, "updated")
	}
	if updated.Ledger.Name != "Renamed Ledger" {
		t.Errorf("Name = %q, want %q", updated.Ledger.Name, "Renamed Ledger")
	}
	if updated.Ledger.Comment != "updated comment" {
		t.Errorf("Comment = %q, want %q", updated.Ledger.Comment, "updated comment")
	}
}

func TestManageLedgerUpdatePartialOnlyName(t *testing.T) {
	session, ledgerID := newTestSession(t)

	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "update",
		ID:        ledgerID,
		Name:      "",
		Comment:   "just a comment",
	}, &tools.ManageLedgerOutput{})

	var updated tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "update",
		ID:        ledgerID,
		Name:      "Only Name Changed",
	}, &updated)

	if updated.Ledger.Name != "Only Name Changed" {
		t.Errorf("Name = %q, want %q", updated.Ledger.Name, "Only Name Changed")
	}
	if updated.Ledger.Comment != "just a comment" {
		t.Errorf("Comment = %q, want %q (unchanged since omitted)", updated.Ledger.Comment, "just a comment")
	}
}

func TestManageLedgerUpdateMissingBothFields(t *testing.T) {
	session, ledgerID := newTestSession(t)

	callToolExpectError(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "update",
		ID:        ledgerID,
	})
}

func TestManageLedgerUpdateNotFound(t *testing.T) {
	session, _ := newTestSession(t)

	callToolExpectError(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "update",
		ID:        "999999",
		Name:      "Ghost",
	})
}

func TestManageLedgerDeleteEmptyLedger(t *testing.T) {
	session, _ := newTestSession(t)

	var created tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "create",
		Name:      "Throwaway",
	}, &created)
	ledgerID := created.Ledger.ID

	var preview tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "delete",
		ID:        ledgerID,
	}, &preview)

	if preview.Status != "pending_confirmation" {
		t.Fatalf("Status = %q, want %q", preview.Status, "pending_confirmation")
	}
	if preview.ConfirmationToken == "" {
		t.Fatal("expected a non-empty confirmation_token")
	}
	if preview.ExpiresAt <= time.Now().Unix() {
		t.Fatalf("ExpiresAt = %d, want in the future", preview.ExpiresAt)
	}

	var applied tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation:         "delete",
		ID:                ledgerID,
		ConfirmationToken: preview.ConfirmationToken,
	}, &applied)

	if applied.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", applied.Status, "deleted")
	}

	var list tools.ListLedgersOutput
	callTool(t, session, "list_ledgers", tools.ListLedgersInput{}, &list)
	if len(list.Ledgers) != 1 {
		t.Fatalf("expected only the original default ledger to remain, got %d", len(list.Ledgers))
	}
}

func TestManageLedgerDeleteBlockedBySources(t *testing.T) {
	session, ledgerID := newTestSession(t)
	createTestSource(t, session, ledgerID)

	callToolExpectError(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "delete",
		ID:        ledgerID,
	})

	var list tools.ListLedgersOutput
	callTool(t, session, "list_ledgers", tools.ListLedgersInput{}, &list)
	if len(list.Ledgers) != 1 {
		t.Fatalf("expected the ledger to survive a blocked delete, got %d ledgers", len(list.Ledgers))
	}
}

func TestManageLedgerDeleteBlockedByCategories(t *testing.T) {
	session, ledgerID := newTestSession(t)
	createTestCategory(t, session, ledgerID, "Food", "")

	callToolExpectError(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "delete",
		ID:        ledgerID,
	})
}

func TestManageLedgerDeleteBlockedByTransactions(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID := createTestSource(t, session, ledgerID)
	categoryID := createTestCategory(t, session, ledgerID, "Food", "")

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     100,
		Currency:   "CNY",
		Time:       futureTime(),
	}, &tools.CreateTransactionOutput{})

	callToolExpectError(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "delete",
		ID:        ledgerID,
	})
}

func TestManageLedgerDeleteNotFound(t *testing.T) {
	session, _ := newTestSession(t)

	callToolExpectError(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "delete",
		ID:        "999999",
	})
}

func TestManageLedgerDeleteTokenReplay(t *testing.T) {
	session, _ := newTestSession(t)

	var created tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Throwaway"}, &created)
	ledgerID := created.Ledger.ID

	var preview tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation: "delete",
		ID:        ledgerID,
	}, &preview)

	var applied tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation:         "delete",
		ID:                ledgerID,
		ConfirmationToken: preview.ConfirmationToken,
	}, &applied)
	if applied.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", applied.Status, "deleted")
	}

	callToolExpectError(t, session, "manage_ledger", tools.ManageLedgerInput{
		Operation:         "delete",
		ID:                ledgerID,
		ConfirmationToken: preview.ConfirmationToken,
	})
}
