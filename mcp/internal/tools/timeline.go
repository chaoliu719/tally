package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/currency"
	"tally/internal/store"
)

func init() {
	register(registerTimelineTools)
}

// registerTimelineTools wires the transaction timeline MCP App: the
// open_transaction_timeline tool (declares the widget via _meta.ui.resourceUri)
// and the ui:// resource that serves the widget HTML. The tool returns the
// most recent page of the requested scope plus a self-contained text summary;
// the widget fetches the older pages of that same scope itself via
// search_transactions (newest_first). See openspec changes
// add-transaction-timeline-widget and scope-timeline-widget-by-range.
func registerTimelineTools(s *mcp.Server, deps Deps) {
	registerWidgetResource(s, "timeline", "Transaction timeline",
		"Interactive, scrollable day-grouped view of a ledger's transactions within a chosen scope, newest first.")

	mcp.AddTool(s, &mcp.Tool{
		Name: "open_transaction_timeline",
		Description: "Open an interactive, scrollable timeline of a ledger's transactions -- grouped by day, newest first. " +
			"Use this when the user wants to browse or scroll through their transactions rather than get a summary or a " +
			"specific lookup. The optional start_date / end_date / category_id / source_id fix the scope when the panel " +
			"opens: only transactions inside the scope are loaded, nothing outside it is, and the scope cannot be widened " +
			"from the panel -- to look at a different period, category or source, call this tool again with a new scope. " +
			"A parameter you leave out does not restrict anything, so passing only ledger_id opens the ledger's entire " +
			"history. The text result is only a summary (scope, transaction count, income and expense totals per currency); " +
			"it does not list transactions. Read-only.",
		Meta: widgetToolMeta("timeline"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in OpenTransactionTimelineInput) (*mcp.CallToolResult, OpenTransactionTimelineOutput, error) {
		return openTransactionTimeline(ctx, deps, in)
	})
}

type OpenTransactionTimelineInput struct {
	LedgerID   string `json:"ledger_id" jsonschema:"the id of the ledger whose transactions to browse, as a decimal string"`
	StartDate  string `json:"start_date,omitempty" jsonschema:"only load transactions on or after this local date (format YYYY-MM-DD, no timezone), the whole day included; omit for no lower bound"`
	EndDate    string `json:"end_date,omitempty" jsonschema:"only load transactions on or before this local date (format YYYY-MM-DD, no timezone), the whole day included; omit for no upper bound"`
	CategoryID string `json:"category_id,omitempty" jsonschema:"only load transactions in this category and all of its descendant categories, as a decimal string; omit for every category"`
	SourceID   string `json:"source_id,omitempty" jsonschema:"only load transactions from/to this source, as a decimal string; omit for every source"`
}

// TimelineScope is the scope an open_transaction_timeline call was opened
// with, echoed back so the widget can show it and can page the rest of the
// scope with search_transactions using exactly the filters the first-page
// cursor was issued under (the cursor fingerprints them). StartTime / EndTime
// are the fixed-width local date-times the dates were turned into; the widget
// passes them through untouched rather than rebuilding them.
type TimelineScope struct {
	StartDate          string `json:"start_date,omitempty" jsonschema:"the start_date the timeline was opened with, if any"`
	EndDate            string `json:"end_date,omitempty" jsonschema:"the end_date the timeline was opened with, if any"`
	StartTime          string `json:"start_time,omitempty" jsonschema:"start_date as the local date-time to pass to search_transactions as start_time"`
	EndTime            string `json:"end_time,omitempty" jsonschema:"end_date as the local date-time to pass to search_transactions as end_time"`
	CategoryID         string `json:"category_id,omitempty" jsonschema:"the category the timeline was opened with, if any"`
	SourceID           string `json:"source_id,omitempty" jsonschema:"the source the timeline was opened with, if any"`
	IncludeDescendants bool   `json:"include_descendants,omitempty" jsonschema:"true when category_id is set: pass it to search_transactions as include_descendants"`
}

type OpenTransactionTimelineOutput struct {
	Total        int64             `json:"total" jsonschema:"number of transactions in the scope the timeline was opened with (the whole ledger when no scope was given)"`
	Scope        TimelineScope     `json:"scope" jsonschema:"the scope the timeline was opened with; pass its filters, unchanged, to search_transactions (newest_first=true) to load the rest of the scope"`
	Transactions []TransactionInfo `json:"transactions" jsonschema:"the most recent page of transactions in the scope, newest first"`
	NextCursor   string            `json:"next_cursor,omitempty" jsonschema:"pass to search_transactions (newest_first=true, with the scope's filters) to load the next, older page"`
	Categories   []CategoryInfo    `json:"categories" jsonschema:"every transaction category in the ledger, so the widget can show category names and populate its filter without a second request"`
	Sources      []SourceInfo      `json:"sources" jsonschema:"every source in the ledger, so the widget can show source names and populate its filter without a second request"`
}

// timelineFirstPageSize is how many transactions the tool returns inline on
// the first paint. It is deliberately larger than the widget's on-screen
// page size: the widget drains the rest of the scope in the background and
// then filters and pages entirely locally, so a bigger first page just means
// fewer background round trips before that is done.
const timelineFirstPageSize = 200

const dateOnlyLayout = "2006-01-02"

// parseTimelineDate validates a YYYY-MM-DD date. The date is only checked and
// echoed, never interpreted in any timezone (tally stores none).
func parseTimelineDate(field, s string) error {
	if _, err := time.Parse(dateOnlyLayout, s); err != nil {
		return fmt.Errorf("invalid %s %q: expected format YYYY-MM-DD", field, s)
	}
	return nil
}

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

	// Resolve the scope. The same values feed the first-page query, the
	// statistics query and the cursor fingerprint, so the three always agree.
	filter := searchTransactionsFilterFields{LedgerID: ledgerID, NewestFirst: true}
	pageParams := store.SearchTransactionsDescParams{LedgerID: ledgerID, Limit: timelineFirstPageSize + 1}
	statParams := store.SummarizeTimelineScopeParams{LedgerID: ledgerID}
	scope := TimelineScope{StartDate: in.StartDate, EndDate: in.EndDate}
	var scopeParts []string

	if in.StartDate != "" {
		if err := parseTimelineDate("start_date", in.StartDate); err != nil {
			return nil, OpenTransactionTimelineOutput{}, err
		}
	}
	if in.EndDate != "" {
		if err := parseTimelineDate("end_date", in.EndDate); err != nil {
			return nil, OpenTransactionTimelineOutput{}, err
		}
	}
	if in.StartDate != "" && in.EndDate != "" && in.StartDate > in.EndDate {
		return nil, OpenTransactionTimelineOutput{}, fmt.Errorf("start_date %s is after end_date %s", in.StartDate, in.EndDate)
	}
	// Dates are fixed-width, so comparing the strings is comparing the days;
	// the whole of end_date is in scope, hence 23:59:59.
	if in.StartDate != "" {
		scope.StartTime = in.StartDate + " 00:00:00"
		pageParams.StartTime, statParams.StartTime = scope.StartTime, scope.StartTime
		filter.StartTime = sql.NullString{String: scope.StartTime, Valid: true}
	}
	if in.EndDate != "" {
		scope.EndTime = in.EndDate + " 23:59:59"
		pageParams.EndTime, statParams.EndTime = scope.EndTime, scope.EndTime
		filter.EndTime = sql.NullString{String: scope.EndTime, Valid: true}
	}
	switch {
	case in.StartDate != "" && in.EndDate != "":
		scopeParts = append(scopeParts, fmt.Sprintf("%s to %s", in.StartDate, in.EndDate))
	case in.StartDate != "":
		scopeParts = append(scopeParts, "from "+in.StartDate)
	case in.EndDate != "":
		scopeParts = append(scopeParts, "up to "+in.EndDate)
	}

	if in.CategoryID != "" {
		id, err := parseID(in.CategoryID)
		if err != nil {
			return nil, OpenTransactionTimelineOutput{}, err
		}
		cat, err := deps.Q.GetCategory(ctx, store.GetCategoryParams{ID: id, LedgerID: ledgerID})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, OpenTransactionTimelineOutput{}, fmt.Errorf("category %q not found in ledger %q", in.CategoryID, in.LedgerID)
			}
			return nil, OpenTransactionTimelineOutput{}, err
		}
		descendants, err := deps.Q.ListCategoryDescendantIDs(ctx, id)
		if err != nil {
			return nil, OpenTransactionTimelineOutput{}, err
		}
		categoryIDsJSON, err := json.Marshal(append([]int64{id}, descendants...))
		if err != nil {
			return nil, OpenTransactionTimelineOutput{}, err
		}
		pageParams.CategoryIdsJson, statParams.CategoryIdsJson = string(categoryIDsJSON), string(categoryIDsJSON)
		filter.CategoryID = sql.NullInt64{Int64: id, Valid: true}
		filter.IncludeDescendants = true
		scope.CategoryID = in.CategoryID
		scope.IncludeDescendants = true
		scopeParts = append(scopeParts, fmt.Sprintf("category %q (including subcategories)", cat.Name))
	}
	if in.SourceID != "" {
		id, err := parseID(in.SourceID)
		if err != nil {
			return nil, OpenTransactionTimelineOutput{}, err
		}
		src, err := deps.Q.GetSource(ctx, store.GetSourceParams{ID: id, LedgerID: ledgerID})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, OpenTransactionTimelineOutput{}, fmt.Errorf("source %q not found in ledger %q", in.SourceID, in.LedgerID)
			}
			return nil, OpenTransactionTimelineOutput{}, err
		}
		pageParams.SourceID, statParams.SourceID = id, id
		filter.SourceID = sql.NullInt64{Int64: id, Valid: true}
		scope.SourceID = in.SourceID
		scopeParts = append(scopeParts, fmt.Sprintf("source %q", src.Name))
	}

	statRows, err := deps.Q.SummarizeTimelineScope(ctx, statParams)
	if err != nil {
		return nil, OpenTransactionTimelineOutput{}, err
	}
	var total int64
	for _, r := range statRows {
		total += r.Count
	}

	rows, err := deps.Q.SearchTransactionsDesc(ctx, pageParams)
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
		Total:        total,
		Scope:        scope,
		Transactions: infos,
		NextCursor:   nextCursor,
		Categories:   categories,
		Sources:      sources,
	}

	summary, err := timelineSummary(in.LedgerID, scope, scopeParts, total, statRows)
	if err != nil {
		return nil, OpenTransactionTimelineOutput{}, err
	}
	// The text is only a summary: the transactions themselves travel in the
	// structured result for the widget, so browsing them never lands in the
	// agent's conversation context.
	result := &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: summary}},
	}
	return result, out, nil
}

// timelineSummary renders the text a host without MCP Apps support shows
// instead of the panel -- and what the agent reads on hosts that have it: the
// scope, how many transactions it holds and, per currency, the income and
// expense totals. Currencies are never added together. It deliberately lists
// no individual transaction.
func timelineSummary(ledgerID string, scope TimelineScope, scopeParts []string, total int64, stats []store.SummarizeTimelineScopeRow) (string, error) {
	scoped := len(scopeParts) > 0
	scopeText := "the whole ledger"
	if scoped {
		scopeText = strings.Join(scopeParts, ", ")
	}

	var b strings.Builder
	if total == 0 {
		if scoped {
			fmt.Fprintf(&b, "Ledger %s has no transactions in scope (%s).", ledgerID, scopeText)
		} else {
			fmt.Fprintf(&b, "Ledger %s has no transactions yet.", ledgerID)
		}
		return b.String(), nil
	}

	fmt.Fprintf(&b, "Ledger %s, scope: %s. %d transaction(s) in scope.", ledgerID, scopeText, total)
	for _, r := range stats {
		income, err := currency.FormatMajor(r.Currency, r.Income)
		if err != nil {
			return "", err
		}
		expense, err := currency.FormatMajor(r.Currency, r.Expense)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "\n%s: income %s, expense %s", r.Currency, income, expense)
	}
	b.WriteString("\nTransactions are not listed here: the timeline panel shows them. To read them in the conversation, ")
	b.WriteString("page search_transactions with newest_first=true and the same filters")
	var args []string
	if scope.StartTime != "" {
		args = append(args, fmt.Sprintf("start_time=%q", scope.StartTime))
	}
	if scope.EndTime != "" {
		args = append(args, fmt.Sprintf("end_time=%q", scope.EndTime))
	}
	if scope.CategoryID != "" {
		args = append(args, "category_id="+scope.CategoryID, "include_descendants=true")
	}
	if scope.SourceID != "" {
		args = append(args, "source_id="+scope.SourceID)
	}
	if len(args) > 0 {
		fmt.Fprintf(&b, " (%s)", strings.Join(args, ", "))
	}
	b.WriteString(".")
	return b.String(), nil
}
