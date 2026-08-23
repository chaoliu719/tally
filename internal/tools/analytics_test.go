package tools_test

import (
	"testing"

	"tally/internal/tools"
)

// findCurrencyTotals returns the CurrencyTotals entry for currency, failing
// the test if it's not present.
func findCurrencyTotals(t *testing.T, totals []tools.CurrencyTotals, currency string) tools.CurrencyTotals {
	t.Helper()
	for _, ct := range totals {
		if ct.Currency == currency {
			return ct
		}
	}
	t.Fatalf("no CurrencyTotals for currency %q in %+v", currency, totals)
	return tools.CurrencyTotals{}
}

// findCategorySummary returns the CategorySummary entry for categoryID,
// failing the test if it's not present.
func findCategorySummary(t *testing.T, rows []tools.CategorySummary, categoryID string) tools.CategorySummary {
	t.Helper()
	for _, cs := range rows {
		if cs.CategoryID == categoryID {
			return cs
		}
	}
	t.Fatalf("no CategorySummary for category %q in %+v", categoryID, rows)
	return tools.CategorySummary{}
}

// findSourceSummary returns the SourceSummary entry for sourceID, failing
// the test if it's not present.
func findSourceSummary(t *testing.T, rows []tools.SourceSummary, sourceID string) tools.SourceSummary {
	t.Helper()
	for _, ss := range rows {
		if ss.SourceID == sourceID {
			return ss
		}
	}
	t.Fatalf("no SourceSummary for source %q in %+v", sourceID, rows)
	return tools.SourceSummary{}
}

func TestGetFinancialSummaryTimeRange(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	earlyTime := futureTime()
	lateTime := earlyTime + 100000

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 100, Currency: "CNY", Time: earlyTime,
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 200, Currency: "CNY", Time: lateTime,
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{
		StartTime: earlyTime - 10,
		EndTime:   earlyTime + 10,
	}, &out)

	if len(out.TotalsByCurrency) != 1 {
		t.Fatalf("expected 1 currency total, got %d: %+v", len(out.TotalsByCurrency), out.TotalsByCurrency)
	}
	ct := out.TotalsByCurrency[0]
	if ct.Currency != "CNY" || ct.Income != 0 || ct.Expense != 100 || ct.Net != -100 {
		t.Errorf("TotalsByCurrency[0] = %+v, want {CNY 0 100 -100}", ct)
	}

	cs := findCategorySummary(t, out.ByCategory, categoryID)
	if cs.Expense != 100 || cs.Income != 0 {
		t.Errorf("ByCategory summary = %+v, want income=0 expense=100", cs)
	}

	ss := findSourceSummary(t, out.BySource, sourceID)
	if ss.Expense != 100 || ss.Income != 0 {
		t.Errorf("BySource summary = %+v, want income=0 expense=100", ss)
	}
}

func TestGetFinancialSummaryNoTimeRange(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "income", SourceID: sourceID, CategoryID: categoryID, Amount: 500, Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 200, Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{}, &out)

	ct := findCurrencyTotals(t, out.TotalsByCurrency, "CNY")
	if ct.Income != 500 || ct.Expense != 200 || ct.Net != 300 {
		t.Errorf("TotalsByCurrency = %+v, want income=500 expense=200 net=300", ct)
	}

	cs := findCategorySummary(t, out.ByCategory, categoryID)
	if cs.Income != 500 || cs.Expense != 200 {
		t.Errorf("ByCategory summary = %+v, want income=500 expense=200", cs)
	}

	ss := findSourceSummary(t, out.BySource, sourceID)
	if ss.Income != 500 || ss.Expense != 200 {
		t.Errorf("BySource summary = %+v, want income=500 expense=200", ss)
	}
}

func TestGetFinancialSummaryEmptyRange(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 100, Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{
		StartTime: futureTime() + 1000000,
		EndTime:   futureTime() + 1100000,
	}, &out)

	if len(out.TotalsByCurrency) != 0 {
		t.Errorf("TotalsByCurrency = %+v, want empty", out.TotalsByCurrency)
	}
	if len(out.ByCategory) != 0 {
		t.Errorf("ByCategory = %+v, want empty", out.ByCategory)
	}
	if len(out.BySource) != 0 {
		t.Errorf("BySource = %+v, want empty", out.BySource)
	}
}

func TestGetFinancialSummaryMultiCurrency(t *testing.T) {
	session := newTestSession(t)

	var source tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		Operation: "create", Name: "Wallet",
	}, &source)

	var category tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create", Name: "Misc",
	}, &category)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "income", SourceID: source.Source.ID, CategoryID: category.Category.ID, Amount: 1000, Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: source.Source.ID, CategoryID: category.Category.ID, Amount: 300, Currency: "USD", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{}, &out)

	if len(out.TotalsByCurrency) != 2 {
		t.Fatalf("expected 2 currency totals, got %d: %+v", len(out.TotalsByCurrency), out.TotalsByCurrency)
	}

	cny := findCurrencyTotals(t, out.TotalsByCurrency, "CNY")
	if cny.Income != 1000 || cny.Expense != 0 || cny.Net != 1000 {
		t.Errorf("CNY totals = %+v, want income=1000 expense=0 net=1000", cny)
	}

	usd := findCurrencyTotals(t, out.TotalsByCurrency, "USD")
	if usd.Income != 0 || usd.Expense != 300 || usd.Net != -300 {
		t.Errorf("USD totals = %+v, want income=0 expense=300 net=-300", usd)
	}
}

func TestGetFinancialSummaryByCategory(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryA := setupSourceAndCategory(t, session)

	var categoryB tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create", Name: "Dining",
	}, &categoryB)

	// A third category with no transactions must not appear in the result.
	var categoryC tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create", Name: "Unused",
	}, &categoryC)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryA, Amount: 100, Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryB.Category.ID, Amount: 250, Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{}, &out)

	if len(out.ByCategory) != 2 {
		t.Fatalf("expected 2 category summaries, got %d: %+v", len(out.ByCategory), out.ByCategory)
	}

	csA := findCategorySummary(t, out.ByCategory, categoryA)
	if csA.Expense != 100 {
		t.Errorf("categoryA expense = %d, want 100", csA.Expense)
	}
	csB := findCategorySummary(t, out.ByCategory, categoryB.Category.ID)
	if csB.Expense != 250 {
		t.Errorf("categoryB expense = %d, want 250", csB.Expense)
	}

	for _, cs := range out.ByCategory {
		if cs.CategoryID == categoryC.Category.ID {
			t.Fatalf("unused category %q unexpectedly present in ByCategory", categoryC.Category.ID)
		}
	}
}

func TestGetFinancialSummaryBySource(t *testing.T) {
	session := newTestSession(t)
	sourceA, categoryID := setupSourceAndCategory(t, session)

	var sourceB tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		Operation: "create", Name: "Second Wallet",
	}, &sourceB)

	// A third source with no transactions must not appear in the result.
	var sourceC tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		Operation: "create", Name: "Unused Wallet",
	}, &sourceC)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceA, CategoryID: categoryID, Amount: 100, Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceB.Source.ID, CategoryID: categoryID, Amount: 250, Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{}, &out)

	if len(out.BySource) != 2 {
		t.Fatalf("expected 2 source summaries, got %d: %+v", len(out.BySource), out.BySource)
	}

	ssA := findSourceSummary(t, out.BySource, sourceA)
	if ssA.Expense != 100 {
		t.Errorf("sourceA expense = %d, want 100", ssA.Expense)
	}
	ssB := findSourceSummary(t, out.BySource, sourceB.Source.ID)
	if ssB.Expense != 250 {
		t.Errorf("sourceB expense = %d, want 250", ssB.Expense)
	}

	for _, ss := range out.BySource {
		if ss.SourceID == sourceC.Source.ID {
			t.Fatalf("unused source %q unexpectedly present in BySource", sourceC.Source.ID)
		}
	}
}
