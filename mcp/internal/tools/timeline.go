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
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      timelineURI,
				MIMEType: widgets.MIMEType,
				Text:     html,
			}},
		}, nil
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
}

const timelineFirstPageSize = 50

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

	out := OpenTransactionTimelineOutput{Total: stats.Count, Transactions: infos, NextCursor: nextCursor}

	// Text content doubles as the graceful-degradation answer for hosts that
	// don't render the widget: a summary (total count, date span) plus the
	// JSON payload the widget parses for its first paint.
	var summary string
	if stats.Count == 0 {
		summary = fmt.Sprintf("Ledger %s has no transactions yet.", in.LedgerID)
	} else {
		summary = fmt.Sprintf("Ledger %s has %d transaction(s), from %s to %s. Showing the %d most recent below",
			in.LedgerID, stats.Count,
			formatUnixDate(nullableUnix(stats.EarliestTime)),
			formatUnixDate(nullableUnix(stats.LatestTime)),
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

// nullableUnix pulls an int64 unix seconds value out of a nullable SQL
// aggregate result (MIN/MAX time), returning 0 when the column was NULL.
func nullableUnix(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	default:
		return 0
	}
}

// formatUnixDate renders unix seconds as a plain calendar date. tally does no
// timezone handling anywhere (see config.yaml), so this is UTC and only used
// for the human-readable degradation summary; the widget itself groups by the
// host's local date.
func formatUnixDate(ts int64) string {
	if ts == 0 {
		return "?"
	}
	return time.Unix(ts, 0).UTC().Format("2006-01-02") + " UTC"
}
