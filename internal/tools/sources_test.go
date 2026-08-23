package tools_test

import (
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/tools"
)

func TestListSourcesEmptyLedger(t *testing.T) {
	session, ledgerID := newTestSession(t)

	var out tools.ListSourcesOutput
	callTool(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerID}, &out)

	if out.Sources == nil {
		t.Fatal("expected an empty slice, got nil")
	}
	if len(out.Sources) != 0 {
		t.Fatalf("expected 0 sources, got %d", len(out.Sources))
	}
}

func TestManageSourceCreate(t *testing.T) {
	session, ledgerID := newTestSession(t)

	var created tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      "Cash Wallet",
	}, &created)

	if created.Status != "created" {
		t.Errorf("Status = %q, want %q", created.Status, "created")
	}
	if created.Source.Name != "Cash Wallet" {
		t.Errorf("Name = %q, want %q", created.Source.Name, "Cash Wallet")
	}

	var list tools.ListSourcesOutput
	callTool(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerID}, &list)

	if len(list.Sources) != 1 {
		t.Fatalf("expected 1 source after creation, got %d", len(list.Sources))
	}
	if list.Sources[0].ID != created.Source.ID {
		t.Errorf("listed source id = %q, want %q", list.Sources[0].ID, created.Source.ID)
	}
}

func TestManageSourceMissingRequiredField(t *testing.T) {
	session, ledgerID := newTestSession(t)

	callToolExpectError(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "create",
	})

	var list tools.ListSourcesOutput
	callTool(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerID}, &list)
	if len(list.Sources) != 0 {
		t.Fatalf("expected no source created, got %d", len(list.Sources))
	}
}

// createTestSource is a small helper shared by the update/delete tests
// below; it creates a source and returns its id.
func createTestSource(t *testing.T, session *mcp.ClientSession, ledgerID string) string {
	t.Helper()

	var created tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      "Cash Wallet",
	}, &created)

	return created.Source.ID
}

func TestManageSourceUpdateHappyPath(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID := createTestSource(t, session, ledgerID)

	var updated tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "update",
		ID:        sourceID,
		Name:      "Renamed Wallet",
	}, &updated)

	if updated.Status != "updated" {
		t.Errorf("Status = %q, want %q", updated.Status, "updated")
	}
	if updated.Source.Name != "Renamed Wallet" {
		t.Errorf("Name = %q, want %q", updated.Source.Name, "Renamed Wallet")
	}

	var list tools.ListSourcesOutput
	callTool(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerID}, &list)
	if list.Sources[0].Name != "Renamed Wallet" {
		t.Errorf("listed name = %q, want %q", list.Sources[0].Name, "Renamed Wallet")
	}
}

func TestManageSourceUpdateNotFound(t *testing.T) {
	session, ledgerID := newTestSession(t)

	callToolExpectError(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "update",
		ID:        "999999",
		Name:      "Ghost",
	})
}

func TestManageSourceUpdateMissingRequiredField(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID := createTestSource(t, session, ledgerID)

	callToolExpectError(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "update",
		ID:        sourceID,
		// Name omitted
	})

	var list tools.ListSourcesOutput
	callTool(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerID}, &list)
	if list.Sources[0].Name != "Cash Wallet" {
		t.Fatalf("source was modified despite missing required field: name = %q", list.Sources[0].Name)
	}
}

func TestManageSourceDeleteHappyPath(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID := createTestSource(t, session, ledgerID)

	var preview tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "delete",
		ID:        sourceID,
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

	var list tools.ListSourcesOutput
	callTool(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerID}, &list)
	if len(list.Sources) != 1 {
		t.Fatalf("preview must not delete the source; got %d sources", len(list.Sources))
	}

	var applied tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:          ledgerID,
		Operation:         "delete",
		ID:                sourceID,
		ConfirmationToken: preview.ConfirmationToken,
	}, &applied)

	if applied.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", applied.Status, "deleted")
	}

	callTool(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerID}, &list)
	if len(list.Sources) != 0 {
		t.Fatalf("expected the source to be gone, got %d sources", len(list.Sources))
	}
}

func TestManageSourceDeleteBlockedByReferences(t *testing.T) {
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

	callToolExpectError(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "delete",
		ID:        sourceID,
	})

	var list tools.ListSourcesOutput
	callTool(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerID}, &list)
	if len(list.Sources) != 1 {
		t.Fatalf("expected the source to survive a blocked delete, got %d sources", len(list.Sources))
	}
}

func TestManageSourceDeleteTokenReplay(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID := createTestSource(t, session, ledgerID)

	var preview tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "delete",
		ID:        sourceID,
	}, &preview)

	var applied tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:          ledgerID,
		Operation:         "delete",
		ID:                sourceID,
		ConfirmationToken: preview.ConfirmationToken,
	}, &applied)
	if applied.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", applied.Status, "deleted")
	}

	// Reusing the same token to confirm the same delete a second time must
	// fail -- the source is already gone, so the live recheck at apply time
	// rejects it even though the token itself is still validly signed and
	// unexpired.
	callToolExpectError(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:          ledgerID,
		Operation:         "delete",
		ID:                sourceID,
		ConfirmationToken: preview.ConfirmationToken,
	})

	var list tools.ListSourcesOutput
	callTool(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerID}, &list)
	if len(list.Sources) != 0 {
		t.Fatalf("expected the source to remain deleted, got %d sources", len(list.Sources))
	}
}

func TestManageSourceDeleteNotFound(t *testing.T) {
	session, ledgerID := newTestSession(t)

	callToolExpectError(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "delete",
		ID:        "999999",
	})
}

func TestManageSourceDeleteExpiredToken(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID := createTestSource(t, session, ledgerID)

	expired := craftConfirmationToken(t, testConfirmSecret, "delete_source", sourceID, "irrelevant-revision", time.Now().Add(-time.Minute).Unix())

	callToolExpectError(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:          ledgerID,
		Operation:         "delete",
		ID:                sourceID,
		ConfirmationToken: expired,
	})

	var list tools.ListSourcesOutput
	callTool(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerID}, &list)
	if len(list.Sources) != 1 {
		t.Fatalf("expired token must not delete the source, got %d sources", len(list.Sources))
	}
}

func TestManageSourceDeleteDriftedRevision(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID := createTestSource(t, session, ledgerID)

	var preview tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "delete",
		ID:        sourceID,
	}, &preview)

	// The source's state changes after the preview but before apply -- the
	// stale token must be rejected rather than deleting a different source
	// than the one previewed.
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "update",
		ID:        sourceID,
		Name:      "Renamed After Preview",
	}, &tools.ManageSourceOutput{})

	callToolExpectError(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:          ledgerID,
		Operation:         "delete",
		ID:                sourceID,
		ConfirmationToken: preview.ConfirmationToken,
	})

	var list tools.ListSourcesOutput
	callTool(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerID}, &list)
	if len(list.Sources) != 1 {
		t.Fatalf("drifted-revision token must not delete the source, got %d sources", len(list.Sources))
	}
}

func TestListSourcesRejectsNonexistentLedger(t *testing.T) {
	session, _ := newTestSession(t)

	callToolExpectError(t, session, "list_sources", tools.ListSourcesInput{LedgerID: "999999"})
}

func TestListSourcesIsolatedByLedger(t *testing.T) {
	session, ledgerA := newTestSession(t)
	createTestSource(t, session, ledgerA)

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID

	var list tools.ListSourcesOutput
	callTool(t, session, "list_sources", tools.ListSourcesInput{LedgerID: ledgerB}, &list)
	if len(list.Sources) != 0 {
		t.Fatalf("expected ledgerB to have no sources, got %d (ledgerA's source leaked in)", len(list.Sources))
	}
}

func TestManageSourceCreateRejectsNonexistentLedger(t *testing.T) {
	session, _ := newTestSession(t)

	callToolExpectError(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  "999999",
		Operation: "create",
		Name:      "Cash Wallet",
	})
}

func TestManageSourceUpdateRejectsSourceFromAnotherLedger(t *testing.T) {
	session, ledgerA := newTestSession(t)
	sourceID := createTestSource(t, session, ledgerA)

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID

	// The source exists, but under ledgerA -- updating it while claiming
	// ledgerB must behave like updating a nonexistent source.
	callToolExpectError(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerB,
		Operation: "update",
		ID:        sourceID,
		Name:      "Hijacked",
	})
}

func TestManageSourceDeleteRejectsSourceFromAnotherLedger(t *testing.T) {
	session, ledgerA := newTestSession(t)
	sourceID := createTestSource(t, session, ledgerA)

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID

	callToolExpectError(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerB,
		Operation: "delete",
		ID:        sourceID,
	})
}
