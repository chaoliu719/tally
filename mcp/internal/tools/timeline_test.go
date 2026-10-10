package tools_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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

// timelineFixture is a ledger with the transactions the scope tests slice up:
// a Food category with a Dining child, a Travel category, two sources, and
// rows hugging both edges of September 2026.
type timelineFixture struct {
	session              *mcp.ClientSession
	ledgerID             string
	food, dining, travel string
	cash, card           string
}

func newTimelineFixture(t *testing.T) timelineFixture {
	t.Helper()
	session, ledgerID := newTestSession(t)
	f := timelineFixture{session: session, ledgerID: ledgerID}

	cat := func(name, parent string) string {
		var out tools.ManageCategoryOutput
		callTool(t, session, "manage_category", tools.ManageCategoryInput{
			LedgerID: ledgerID, Operation: "create", Name: name, ParentID: parent,
		}, &out)
		return out.Category.ID
	}
	src := func(name string) string {
		var out tools.ManageSourceOutput
		callTool(t, session, "manage_source", tools.ManageSourceInput{
			LedgerID: ledgerID, Operation: "create", Name: name,
		}, &out)
		return out.Source.ID
	}
	f.food = cat("Food", "")
	f.dining = cat("Dining", f.food)
	f.travel = cat("Travel", "")
	f.cash, f.card = src("Cash"), src("Card")

	f.add(t, "expense", f.cash, f.food, "CNY", "10.00", "2026-08-31 23:59:59", "last of August")
	f.add(t, "expense", f.cash, f.food, "CNY", "20.00", "2026-09-01 00:00:00", "first of September")
	f.add(t, "income", f.card, f.travel, "CNY", "50.00", "2026-09-15 12:00:00", "mid September")
	f.add(t, "expense", f.card, f.dining, "USD", "3.00", "2026-09-30 23:59:59", "last of September")
	f.add(t, "expense", f.cash, f.food, "CNY", "40.00", "2026-10-01 00:00:00", "first of October")
	return f
}

func (f timelineFixture) add(t *testing.T, typ, source, category, cur, amount, at, comment string) {
	t.Helper()
	callTool(t, f.session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: f.ledgerID, Type: typ, SourceID: source, CategoryID: category,
		Currency: cur, Amount: amount, Time: at, Comment: comment,
	}, &tools.CreateTransactionOutput{})
}

func (f timelineFixture) open(t *testing.T, in tools.OpenTransactionTimelineInput) (tools.OpenTransactionTimelineOutput, string) {
	t.Helper()
	in.LedgerID = f.ledgerID
	var out tools.OpenTransactionTimelineOutput
	res := callTool(t, f.session, "open_transaction_timeline", in, &out)
	return out, contentText(res)
}

func comments(out tools.OpenTransactionTimelineOutput) []string {
	var got []string
	for _, tx := range out.Transactions {
		got = append(got, tx.Comment)
	}
	return got
}

// TestOpenTransactionTimelineScopeByDate: start_date / end_date are inclusive
// of the whole day, and nothing outside them is loaded or counted.
func TestOpenTransactionTimelineScopeByDate(t *testing.T) {
	f := newTimelineFixture(t)

	out, summary := f.open(t, tools.OpenTransactionTimelineInput{StartDate: "2026-09-01", EndDate: "2026-09-30"})
	if out.Total != 3 || len(out.Transactions) != 3 {
		t.Fatalf("september: total=%d rows=%d, want 3/3 (%v)", out.Total, len(out.Transactions), comments(out))
	}
	// newest first, both edge instants included, neighbours excluded.
	if got := strings.Join(comments(out), "|"); got != "last of September|mid September|first of September" {
		t.Errorf("september rows = %q", got)
	}
	if out.Scope.StartDate != "2026-09-01" || out.Scope.EndDate != "2026-09-30" ||
		out.Scope.StartTime != "2026-09-01 00:00:00" || out.Scope.EndTime != "2026-09-30 23:59:59" {
		t.Errorf("scope echo = %+v", out.Scope)
	}
	if !strings.Contains(summary, "3 transaction(s) in scope") || !strings.Contains(summary, "2026-09-01 to 2026-09-30") {
		t.Errorf("summary = %q", summary)
	}

	// Only a start: no upper bound.
	out, _ = f.open(t, tools.OpenTransactionTimelineInput{StartDate: "2026-09-30"})
	if got := strings.Join(comments(out), "|"); got != "first of October|last of September" || out.Total != 2 {
		t.Errorf("start only: total=%d rows=%q", out.Total, got)
	}
	// Only an end: no lower bound.
	out, _ = f.open(t, tools.OpenTransactionTimelineInput{EndDate: "2026-09-01"})
	if got := strings.Join(comments(out), "|"); got != "first of September|last of August" || out.Total != 2 {
		t.Errorf("end only: total=%d rows=%q", out.Total, got)
	}
	// No scope at all: the whole ledger, exactly as before.
	out, summary = f.open(t, tools.OpenTransactionTimelineInput{})
	if out.Total != 5 || len(out.Transactions) != 5 || out.Scope != (tools.TimelineScope{}) {
		t.Errorf("unscoped: total=%d rows=%d scope=%+v", out.Total, len(out.Transactions), out.Scope)
	}
	if !strings.Contains(summary, "the whole ledger") {
		t.Errorf("unscoped summary = %q", summary)
	}
}

// TestOpenTransactionTimelineScopeByCategoryAndSource: a category scope pulls
// in its descendants, a source scope is exact, and they combine with a date.
func TestOpenTransactionTimelineScopeByCategoryAndSource(t *testing.T) {
	f := newTimelineFixture(t)

	// Food has the Dining child: Food rows (3) + the Dining row (1).
	out, _ := f.open(t, tools.OpenTransactionTimelineInput{CategoryID: f.food})
	if out.Total != 4 || !out.Scope.IncludeDescendants || out.Scope.CategoryID != f.food {
		t.Errorf("food: total=%d scope=%+v, want 4 rows incl. descendants", out.Total, out.Scope)
	}
	// A leaf category is an exact match.
	out, _ = f.open(t, tools.OpenTransactionTimelineInput{CategoryID: f.dining})
	if out.Total != 1 {
		t.Errorf("dining: total=%d, want 1", out.Total)
	}
	out, summary := f.open(t, tools.OpenTransactionTimelineInput{SourceID: f.card})
	if out.Total != 2 || out.Scope.SourceID != f.card {
		t.Errorf("card: total=%d scope=%+v, want 2", out.Total, out.Scope)
	}
	if !strings.Contains(summary, `source "Card"`) {
		t.Errorf("source summary = %q", summary)
	}
	// September + Food(+Dining) + Card: only the Dining row.
	out, _ = f.open(t, tools.OpenTransactionTimelineInput{
		StartDate: "2026-09-01", EndDate: "2026-09-30", CategoryID: f.food, SourceID: f.card,
	})
	if got := strings.Join(comments(out), "|"); got != "last of September" || out.Total != 1 {
		t.Errorf("combined: total=%d rows=%q", out.Total, got)
	}
}

// TestOpenTransactionTimelineRejectsBadScope: bad scopes fail loudly instead
// of silently loading something else.
func TestOpenTransactionTimelineRejectsBadScope(t *testing.T) {
	f := newTimelineFixture(t)

	var other tools.ManageLedgerOutput
	callTool(t, f.session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other"}, &other)
	var otherCat tools.ManageCategoryOutput
	callTool(t, f.session, "manage_category", tools.ManageCategoryInput{
		LedgerID: other.Ledger.ID, Operation: "create", Name: "Elsewhere",
	}, &otherCat)
	var otherSrc tools.ManageSourceOutput
	callTool(t, f.session, "manage_source", tools.ManageSourceInput{
		LedgerID: other.Ledger.ID, Operation: "create", Name: "Elsewhere",
	}, &otherSrc)

	cases := []struct {
		name string
		in   tools.OpenTransactionTimelineInput
		want string
	}{
		{"unpadded start", tools.OpenTransactionTimelineInput{StartDate: "2026-9-1"}, "start_date"},
		{"impossible end", tools.OpenTransactionTimelineInput{EndDate: "2026-02-30"}, "end_date"},
		{"with time of day", tools.OpenTransactionTimelineInput{StartDate: "2026-09-01 00:00:00"}, "start_date"},
		{"start after end", tools.OpenTransactionTimelineInput{StartDate: "2026-09-30", EndDate: "2026-09-01"}, "after"},
		{"unknown category", tools.OpenTransactionTimelineInput{CategoryID: "999999"}, "category"},
		{"category of another ledger", tools.OpenTransactionTimelineInput{CategoryID: otherCat.Category.ID}, "category"},
		{"non-numeric category", tools.OpenTransactionTimelineInput{CategoryID: "food"}, "invalid id"},
		{"unknown source", tools.OpenTransactionTimelineInput{SourceID: "999999"}, "source"},
		{"source of another ledger", tools.OpenTransactionTimelineInput{SourceID: otherSrc.Source.ID}, "source"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.in.LedgerID = f.ledgerID
			res := callToolExpectError(t, f.session, "open_transaction_timeline", c.in)
			if msg := contentText(res); !strings.Contains(msg, c.want) {
				t.Errorf("error = %q, want it to mention %q", msg, c.want)
			}
		})
	}
}

// TestOpenTransactionTimelineCursorPagesTheScope: the first page's cursor,
// replayed with the scope's own filters, pages exactly the rest of the scope
// -- never anything outside it -- and is rejected under different filters.
func TestOpenTransactionTimelineCursorPagesTheScope(t *testing.T) {
	f := newTimelineFixture(t)
	// 205 more September rows push the scope past one first page (200).
	for i := 0; i < 205; i++ {
		f.add(t, "expense", f.cash, f.food, "CNY", "1.00", fmt.Sprintf("2026-09-10 %02d:%02d:00", i/60, i%60), "bulk")
	}

	out, _ := f.open(t, tools.OpenTransactionTimelineInput{StartDate: "2026-09-01", EndDate: "2026-09-30"})
	if out.Total != 208 || len(out.Transactions) != 200 || out.NextCursor == "" {
		t.Fatalf("first page: total=%d rows=%d cursor=%q, want 208/200/non-empty", out.Total, len(out.Transactions), out.NextCursor)
	}

	args := tools.SearchTransactionsInput{
		LedgerID: f.ledgerID, NewestFirst: true, Limit: 100, Cursor: out.NextCursor,
		StartTime: out.Scope.StartTime, EndTime: out.Scope.EndTime,
	}
	var rest tools.SearchTransactionsOutput
	callTool(t, f.session, "search_transactions", args, &rest)
	if len(rest.Transactions) != 8 || rest.NextCursor != "" {
		t.Fatalf("second page: rows=%d cursor=%q, want 8/empty", len(rest.Transactions), rest.NextCursor)
	}
	for _, tx := range rest.Transactions {
		if tx.Time < out.Scope.StartTime || tx.Time > out.Scope.EndTime {
			t.Errorf("paged row %s at %s is outside the scope", tx.ID, tx.Time)
		}
	}

	// The cursor is bound to the scope's filters: dropping one is refused.
	args.EndTime = ""
	callToolExpectError(t, f.session, "search_transactions", args)

	// Same for a category scope: the descendants flag is part of the filters.
	cout, _ := f.open(t, tools.OpenTransactionTimelineInput{CategoryID: f.food})
	if cout.Scope.CategoryID != f.food || !cout.Scope.IncludeDescendants || cout.NextCursor == "" {
		t.Fatalf("category scope = %+v cursor=%q", cout.Scope, cout.NextCursor)
	}
	var crest tools.SearchTransactionsOutput
	callTool(t, f.session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID: f.ledgerID, NewestFirst: true, Limit: 100, Cursor: cout.NextCursor,
		CategoryID: cout.Scope.CategoryID, IncludeDescendants: cout.Scope.IncludeDescendants,
	}, &crest)
	if len(crest.Transactions) != int(cout.Total)-200 {
		t.Errorf("category second page rows=%d, want %d", len(crest.Transactions), int(cout.Total)-200)
	}
}

// TestOpenTransactionTimelineSummaryIsOnlyASummary: the text lists no
// transaction, splits totals per currency without adding them, and says so
// plainly for an empty scope.
func TestOpenTransactionTimelineSummaryIsOnlyASummary(t *testing.T) {
	f := newTimelineFixture(t)

	_, summary := f.open(t, tools.OpenTransactionTimelineInput{StartDate: "2026-09-01", EndDate: "2026-09-30"})
	for _, leaked := range []string{"mid September", "first of September", "last of September", "STARBUCKS", `"transactions"`, "2026-09-15 12:00:00"} {
		if strings.Contains(summary, leaked) {
			t.Errorf("summary leaks transaction detail %q: %q", leaked, summary)
		}
	}
	if strings.Contains(summary, "{") {
		t.Errorf("summary carries a JSON payload: %q", summary)
	}
	// September: CNY income 50.00 / expense 20.00, USD expense 3.00.
	for _, want := range []string{"CNY: income 50.00, expense 20.00", "USD: income 0.00, expense 3.00"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary missing %q: %q", want, summary)
		}
	}
	if !strings.Contains(summary, `start_time="2026-09-01 00:00:00"`) || !strings.Contains(summary, `end_time="2026-09-30 23:59:59"`) {
		t.Errorf("summary does not hand back the search_transactions filters: %q", summary)
	}

	// A scope with nothing in it says so and names the scope.
	out, empty := f.open(t, tools.OpenTransactionTimelineInput{StartDate: "2027-01-01", EndDate: "2027-01-31"})
	if out.Total != 0 || len(out.Transactions) != 0 || out.NextCursor != "" {
		t.Errorf("empty scope: total=%d rows=%d cursor=%q", out.Total, len(out.Transactions), out.NextCursor)
	}
	if !strings.Contains(empty, "no transactions in scope") || !strings.Contains(empty, "2027-01-01 to 2027-01-31") {
		t.Errorf("empty-scope summary = %q", empty)
	}
	// Lookups still ride along so the panel can render an empty scope.
	if len(out.Categories) != 3 || len(out.Sources) != 2 {
		t.Errorf("empty scope lookups: categories=%d sources=%d", len(out.Categories), len(out.Sources))
	}
}
