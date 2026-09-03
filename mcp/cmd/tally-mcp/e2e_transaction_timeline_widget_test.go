package main

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/tools"
	"tally/internal/widgets"
)

// TestE2ETransactionTimelineWidgetResource is the spike check for
// add-transaction-timeline-widget (task 2.2): over real HTTP wiring, the
// open_transaction_timeline tool advertises the widget via _meta.ui and the
// ui:// resource reads back as a self-contained mcp-app HTML document.
func TestE2ETransactionTimelineWidgetResource(t *testing.T) {
	session, ledgerID := newE2ESession(t)

	var wallet tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID, Operation: "create", Name: "Checking"}, &wallet)
	var cat tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{LedgerID: ledgerID, Operation: "create", Name: "Dining"}, &cat)
	call(t, session, "create_transaction", tools.CreateTransactionInput{LedgerID: ledgerID,
		Type: "expense", SourceID: wallet.Source.ID, CategoryID: cat.Category.ID,
		Amount: cnyAmount(1234), Currency: "CNY", Time: futureTime(), Comment: "STARBUCKS",
	}, &tools.CreateTransactionOutput{})

	// The tool result carries the widget reference in _meta.ui.resourceUri and
	// a degradation-friendly text summary + JSON payload.
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "open_transaction_timeline",
		Arguments: tools.OpenTransactionTimelineInput{LedgerID: ledgerID},
	})
	if err != nil || res.IsError {
		t.Fatalf("open_transaction_timeline failed: err=%v isError=%v", err, res.IsError)
	}
	wantURI := widgets.URI("timeline")

	// Read the widget resource itself.
	rr, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: wantURI})
	if err != nil {
		t.Fatalf("ReadResource(%s): %v", wantURI, err)
	}
	if len(rr.Contents) != 1 {
		t.Fatalf("resource contents = %d, want 1", len(rr.Contents))
	}
	c := rr.Contents[0]
	if c.MIMEType != widgets.MIMEType {
		t.Errorf("mime = %q, want %q", c.MIMEType, widgets.MIMEType)
	}
	if !strings.Contains(c.Text, "<!doctype html>") {
		t.Errorf("resource text is not an HTML document")
	}
	if strings.Contains(c.Text, widgets.BundlePlaceholder) {
		t.Errorf("ext-apps bundle placeholder was not substituted")
	}
	if !strings.Contains(c.Text, "globalThis.ExtApps={") {
		t.Errorf("ext-apps browser runtime not spliced in")
	}
	// The document must not pull in anything over the network: no external
	// script/style/font/image references. (The vendored ext-apps bundle
	// contains URL *string literals* -- schema ids, protocol version URLs --
	// that are never fetched, so scan for fetch-shaped markup, not "http".)
	for _, bad := range []string{`src="http`, `src='http`, `href="http`, `href='http`, "<link ", "@import", "//esm.sh", "//unpkg", "//cdn"} {
		if strings.Contains(c.Text, bad) {
			t.Errorf("widget HTML pulls an external resource (%q); must be self-contained", bad)
		}
	}
}
