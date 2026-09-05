package tools_test

import (
	"testing"

	"tally/internal/tools"
)

// TestOpenTransactionTimelineCarriesLookups checks that open_transaction_timeline
// returns the ledger's categories and sources inline, with parent_id wired up,
// so the widget can render names and populate its filter without a second call
// (see change timeline-widget-local-filtering).
func TestOpenTransactionTimelineCarriesLookups(t *testing.T) {
	session, ledgerID := newTestSession(t)

	var parent tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID: ledgerID, Operation: "create", Name: "Food",
	}, &parent)
	var child tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID: ledgerID, Operation: "create", Name: "Dining", ParentID: parent.Category.ID,
	}, &child)
	var src tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID: ledgerID, Operation: "create", Name: "Checking",
	}, &src)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID, Type: "expense", SourceID: src.Source.ID,
		CategoryID: child.Category.ID, Amount: cnyAmount(1234), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.OpenTransactionTimelineOutput
	callTool(t, session, "open_transaction_timeline", tools.OpenTransactionTimelineInput{LedgerID: ledgerID}, &out)

	if len(out.Categories) != 2 {
		t.Fatalf("Categories = %d, want 2", len(out.Categories))
	}
	byID := map[string]tools.CategoryInfo{}
	for _, c := range out.Categories {
		byID[c.ID] = c
	}
	if got := byID[parent.Category.ID].ParentID; got != "0" {
		t.Errorf("parent category parent_id = %q, want \"0\"", got)
	}
	if got := byID[child.Category.ID].ParentID; got != parent.Category.ID {
		t.Errorf("child category parent_id = %q, want %q", got, parent.Category.ID)
	}
	if len(out.Sources) != 1 || out.Sources[0].Name != "Checking" {
		t.Errorf("Sources = %+v, want one \"Checking\"", out.Sources)
	}
}

// TestOpenTransactionTimelineEmptyLedgerLookups: an empty ledger still returns
// non-nil (empty) lookup slices, and no transactions.
func TestOpenTransactionTimelineEmptyLedgerLookups(t *testing.T) {
	session, ledgerID := newTestSession(t)

	var out tools.OpenTransactionTimelineOutput
	callTool(t, session, "open_transaction_timeline", tools.OpenTransactionTimelineInput{LedgerID: ledgerID}, &out)

	if out.Categories == nil || out.Sources == nil {
		t.Fatalf("lookup slices must be non-nil: categories=%v sources=%v", out.Categories, out.Sources)
	}
	if len(out.Transactions) != 0 || out.NextCursor != "" {
		t.Errorf("empty ledger: rows=%d cursor=%q", len(out.Transactions), out.NextCursor)
	}
}
