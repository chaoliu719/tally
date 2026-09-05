package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/store"
	"tally/internal/widgets"
)

func init() {
	register(registerTimelineTools)
}

// registerTimelineTools wires the transaction timeline MCP App: the
// open_transaction_timeline tool (declares the widget via _meta.ui.resourceUri)
// and the ui:// resource that serves the widget HTML. The tool returns the
// most recent page of transactions plus a self-contained text summary; the
// widget fetches older pages itself via search_transactions (newest_first).
// See openspec change add-transaction-timeline-widget.
func registerTimelineTools(s *mcp.Server, deps Deps) {
	timelineURI := widgets.URI("timeline")

	s.AddResource(&mcp.Resource{
		Name:        "Transaction timeline",
		URI:         timelineURI,
		MIMEType:    widgets.MIMEType,
		Description: "Interactive, scrollable day-grouped view of a ledger's transactions, newest first.",
	}, func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		html, ok := widgets.HTML("timeline")
		if !ok {
			return nil, mcp.ResourceNotFoundError(req.Params.URI)
		}
		res := &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      timelineURI,
				MIMEType: widgets.MIMEType,
				Text:     html,
			}},
		}
		// The widget HTML (~380KB with the inlined ext-apps runtime) is
		// identical for everyone and only changes on deploy. Without a TTL
		// the SDK default is "immediately stale", so the host refetches the
		// whole payload on every panel open and every page reload -- the
		// ~10s "loading" the user sees. A few minutes of freshness kills the
		// repeated refetch within a working session while still letting a
		// new deploy propagate quickly.
		res.TTLMs = int(widgetResourceTTL / time.Millisecond)
		res.CacheScope = "public"
		return res, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "open_transaction_timeline",
		Description: "Open an interactive, scrollable timeline of a ledger's transactions -- grouped by day, newest first, " +
			"scroll down to load older transactions all the way back to the earliest one. Use this when the user wants to " +
			"browse or scroll through their transaction history rather than get a summary or a specific lookup. Read-only.",
		Meta: mcp.Meta{
			"ui": map[string]any{"resourceUri": timelineURI},
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in OpenTransactionTimelineInput) (*mcp.CallToolResult, OpenTransactionTimelineOutput, error) {
		return openTransactionTimeline(ctx, deps, in)
	})
}

type OpenTransactionTimelineInput struct {
	LedgerID string `json:"ledger_id" jsonschema:"the id of the ledger whose transactions to browse, as a decimal string"`
}

type OpenTransactionTimelineOutput struct {
	Total        int64             `json:"total" jsonschema:"total number of transactions in the ledger"`
	Transactions []TransactionInfo `json:"transactions" jsonschema:"the most recent page of transactions, newest first"`
	NextCursor   string            `json:"next_cursor,omitempty" jsonschema:"pass to search_transactions (newest_first=true) to load the next, older page"`
	Categories   []CategoryInfo    `json:"categories" jsonschema:"every transaction category in the ledger, so the widget can show category names and populate its filter without a second request"`
	Sources      []SourceInfo      `json:"sources" jsonschema:"every source in the ledger, so the widget can show source names and populate its filter without a second request"`
}

// timelineFirstPageSize is how many transactions the tool returns inline on
// the first paint. It is deliberately larger than the widget's on-screen
// page size: the widget drains the rest in the background and then filters
// and pages entirely locally, so a bigger first page just means fewer
// background round trips before that is done.
const timelineFirstPageSize = 200

// widgetResourceTTL is the freshness hint on the timeline widget's ui://
// resource read: how long a host may reuse a cached copy before refetching.
// Trade-off: larger = fewer ~380KB refetches, but a new deploy takes this
// long to reach an already-open client.
const widgetResourceTTL = 5 * time.Minute

func openTransactionTimeline(ctx context.Context, deps Deps, in OpenTransactionTimelineInput) (*mcp.CallToolResult, OpenTransactionTimelineOutput, error) {
	if in.LedgerID == "" {
		return nil, OpenTransactionTimelineOutput{}, fmt.Errorf("missing required field: ledger_id")
	}
	ledgerID, err := parseID(in.LedgerID)
	if err != nil {
		return nil, OpenTransactionTimelineOutput{}, err
	}
	if _, err := deps.Q.GetLedger(ctx, ledgerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, OpenTransactionTimelineOutput{}, fmt.Errorf("ledger %q not found", in.LedgerID)
		}
		return nil, OpenTransactionTimelineOutput{}, err
	}

	stats, err := deps.Q.TransactionStats(ctx, ledgerID)
	if err != nil {
		return nil, OpenTransactionTimelineOutput{}, err
	}

	filter := searchTransactionsFilterFields{LedgerID: ledgerID, NewestFirst: true}
	rows, err := deps.Q.SearchTransactionsDesc(ctx, store.SearchTransactionsDescParams{
		LedgerID: ledgerID,
		Limit:    timelineFirstPageSize + 1,
	})
	if err != nil {
		return nil, OpenTransactionTimelineOutput{}, err
	}

	var nextCursor string
	if len(rows) > timelineFirstPageSize {
		last := rows[timelineFirstPageSize-1]
		nextCursor = encodeSearchTransactionsCursor(last.Time, last.ID, filter)
		rows = rows[:timelineFirstPageSize]
	}

	infos := make([]TransactionInfo, 0, len(rows))
	for _, t := range rows {
		info, err := toTransactionInfo(t)
		if err != nil {
			return nil, OpenTransactionTimelineOutput{}, err
		}
		infos = append(infos, info)
	}

	catRows, err := deps.Q.ListCategories(ctx, ledgerID)
	if err != nil {
		return nil, OpenTransactionTimelineOutput{}, err
	}
	categories := make([]CategoryInfo, 0, len(catRows))
	for _, c := range catRows {
		categories = append(categories, toCategoryInfo(c))
	}

	srcRows, err := deps.Q.ListSources(ctx, ledgerID)
	if err != nil {
		return nil, OpenTransactionTimelineOutput{}, err
	}
	sources := make([]SourceInfo, 0, len(srcRows))
	for _, s := range srcRows {
		sources = append(sources, toSourceInfo(s))
	}

	out := OpenTransactionTimelineOutput{
		Total:        stats.Count,
		Transactions: infos,
		NextCursor:   nextCursor,
		Categories:   categories,
		Sources:      sources,
	}

	// Text content doubles as the graceful-degradation answer for hosts that
	// don't render the widget: a summary (total count, date span) plus the
	// JSON payload the widget parses for its first paint.
	var summary string
	if stats.Count == 0 {
		summary = fmt.Sprintf("Ledger %s has no transactions yet.", in.LedgerID)
	} else {
		summary = fmt.Sprintf("Ledger %s has %d transaction(s), from %s to %s. Showing the %d most recent below",
			in.LedgerID, stats.Count,
			formatLocalDate(nullableString(stats.EarliestTime)),
			formatLocalDate(nullableString(stats.LatestTime)),
			len(infos))
		if nextCursor != "" {
			summary += "; the rest load as you scroll the panel (or page search_transactions with newest_first=true)."
		} else {
			summary += " (the whole ledger)."
		}
	}
	payload, _ := json.Marshal(out)
	result := &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: summary + "\n\n" + string(payload)}},
	}
	return result, out, nil
}

// nullableString pulls a string value out of a nullable SQL aggregate result
// (MIN/MAX time), returning "" when the column was NULL. The SQL driver may
// hand back either a string or a []byte for a TEXT column, depending on the
// scan path, so both are handled.
func nullableString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return ""
	}
}

// formatLocalDate renders a "YYYY-MM-DD HH:MM:SS" local date-time string
// (tally stores no timezone anywhere -- see design.md) as just its date
// part, used only for the human-readable degradation summary. No timezone
// conversion happens here or anywhere else: the stored string already is
// the date it displays.
func formatLocalDate(s string) string {
	if len(s) < len("2006-01-02") {
		return "?"
	}
	return s[:len("2006-01-02")]
}
