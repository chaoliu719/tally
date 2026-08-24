package tools_test

import (
	"testing"

	"tally/internal/currency"
	"tally/internal/tools"
)

// majorAmount formats minorUnits as the decimal-string amount the
// get_financial_summary wire format now uses, so existing test cases keep
// exercising the same underlying minor-unit values they always did (e.g.
// majorAmount(t, "CNY", 100) == "1.00").
func majorAmount(t *testing.T, code string, minorUnits int64) string {
	t.Helper()
	s, err := currency.FormatMajor(code, minorUnits)
	if err != nil {
		t.Fatalf("FormatMajor(%q, %d): %v", code, minorUnits, err)
	}
	return s
}

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
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	earlyTime := futureTime()
	lateTime := earlyTime + 100000

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: earlyTime,
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(200), Currency: "CNY", Time: lateTime,
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{
		LedgerID:  ledgerID,
		StartTime: earlyTime - 10,
		EndTime:   earlyTime + 10,
	}, &out)

	if len(out.TotalsByCurrency) != 1 {
		t.Fatalf("expected 1 currency total, got %d: %+v", len(out.TotalsByCurrency), out.TotalsByCurrency)
	}
	ct := out.TotalsByCurrency[0]
	if ct.Currency != "CNY" || ct.Income != majorAmount(t, "CNY", 0) || ct.Expense != majorAmount(t, "CNY", 100) || ct.Net != majorAmount(t, "CNY", -100) {
		t.Errorf("TotalsByCurrency[0] = %+v, want {CNY %q %q %q}", ct, majorAmount(t, "CNY", 0), majorAmount(t, "CNY", 100), majorAmount(t, "CNY", -100))
	}

	cs := findCategorySummary(t, out.ByCategory, categoryID)
	if cs.Expense != majorAmount(t, "CNY", 100) || cs.Income != majorAmount(t, "CNY", 0) {
		t.Errorf("ByCategory summary = %+v, want income=%q expense=%q", cs, majorAmount(t, "CNY", 0), majorAmount(t, "CNY", 100))
	}

	ss := findSourceSummary(t, out.BySource, sourceID)
	if ss.Expense != majorAmount(t, "CNY", 100) || ss.Income != majorAmount(t, "CNY", 0) {
		t.Errorf("BySource summary = %+v, want income=%q expense=%q", ss, majorAmount(t, "CNY", 0), majorAmount(t, "CNY", 100))
	}
}

func TestGetFinancialSummaryNoTimeRange(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "income", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(500), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(200), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{LedgerID: ledgerID}, &out)

	ct := findCurrencyTotals(t, out.TotalsByCurrency, "CNY")
	if ct.Income != majorAmount(t, "CNY", 500) || ct.Expense != majorAmount(t, "CNY", 200) || ct.Net != majorAmount(t, "CNY", 300) {
		t.Errorf("TotalsByCurrency = %+v, want income=%q expense=%q net=%q", ct, majorAmount(t, "CNY", 500), majorAmount(t, "CNY", 200), majorAmount(t, "CNY", 300))
	}

	cs := findCategorySummary(t, out.ByCategory, categoryID)
	if cs.Income != majorAmount(t, "CNY", 500) || cs.Expense != majorAmount(t, "CNY", 200) {
		t.Errorf("ByCategory summary = %+v, want income=%q expense=%q", cs, majorAmount(t, "CNY", 500), majorAmount(t, "CNY", 200))
	}

	ss := findSourceSummary(t, out.BySource, sourceID)
	if ss.Income != majorAmount(t, "CNY", 500) || ss.Expense != majorAmount(t, "CNY", 200) {
		t.Errorf("BySource summary = %+v, want income=%q expense=%q", ss, majorAmount(t, "CNY", 500), majorAmount(t, "CNY", 200))
	}
}

func TestGetFinancialSummaryEmptyRange(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{
		LedgerID:  ledgerID,
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
	session, ledgerID := newTestSession(t)

	var source tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "create", Name: "Wallet",
	}, &source)

	var category tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "create", Name: "Misc",
	}, &category)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "income", SourceID: source.Source.ID, CategoryID: category.Category.ID, Amount: cnyAmount(1000), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: source.Source.ID, CategoryID: category.Category.ID, Amount: cnyAmount(300), Currency: "USD", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{LedgerID: ledgerID}, &out)

	if len(out.TotalsByCurrency) != 2 {
		t.Fatalf("expected 2 currency totals, got %d: %+v", len(out.TotalsByCurrency), out.TotalsByCurrency)
	}

	cny := findCurrencyTotals(t, out.TotalsByCurrency, "CNY")
	if cny.Income != majorAmount(t, "CNY", 1000) || cny.Expense != majorAmount(t, "CNY", 0) || cny.Net != majorAmount(t, "CNY", 1000) {
		t.Errorf("CNY totals = %+v, want income=%q expense=%q net=%q", cny, majorAmount(t, "CNY", 1000), majorAmount(t, "CNY", 0), majorAmount(t, "CNY", 1000))
	}

	usd := findCurrencyTotals(t, out.TotalsByCurrency, "USD")
	if usd.Income != majorAmount(t, "USD", 0) || usd.Expense != majorAmount(t, "USD", 300) || usd.Net != majorAmount(t, "USD", -300) {
		t.Errorf("USD totals = %+v, want income=%q expense=%q net=%q", usd, majorAmount(t, "USD", 0), majorAmount(t, "USD", 300), majorAmount(t, "USD", -300))
	}
}

func TestGetFinancialSummaryByCategory(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryA := setupSourceAndCategory(t, session, ledgerID)

	var categoryB tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "create", Name: "Dining",
	}, &categoryB)

	// A third category with no transactions must not appear in the result.
	var categoryC tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "create", Name: "Unused",
	}, &categoryC)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryA, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryB.Category.ID, Amount: cnyAmount(250), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{LedgerID: ledgerID}, &out)

	if len(out.ByCategory) != 2 {
		t.Fatalf("expected 2 category summaries, got %d: %+v", len(out.ByCategory), out.ByCategory)
	}

	csA := findCategorySummary(t, out.ByCategory, categoryA)
	if csA.Expense != majorAmount(t, "CNY", 100) {
		t.Errorf("categoryA expense = %q, want %q", csA.Expense, majorAmount(t, "CNY", 100))
	}
	csB := findCategorySummary(t, out.ByCategory, categoryB.Category.ID)
	if csB.Expense != majorAmount(t, "CNY", 250) {
		t.Errorf("categoryB expense = %q, want %q", csB.Expense, majorAmount(t, "CNY", 250))
	}

	for _, cs := range out.ByCategory {
		if cs.CategoryID == categoryC.Category.ID {
			t.Fatalf("unused category %q unexpectedly present in ByCategory", categoryC.Category.ID)
		}
	}
}

func TestGetFinancialSummaryBySource(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceA, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var sourceB tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "create", Name: "Second Wallet",
	}, &sourceB)

	// A third source with no transactions must not appear in the result.
	var sourceC tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "create", Name: "Unused Wallet",
	}, &sourceC)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceA, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceB.Source.ID, CategoryID: categoryID, Amount: cnyAmount(250), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{LedgerID: ledgerID}, &out)

	if len(out.BySource) != 2 {
		t.Fatalf("expected 2 source summaries, got %d: %+v", len(out.BySource), out.BySource)
	}

	ssA := findSourceSummary(t, out.BySource, sourceA)
	if ssA.Expense != majorAmount(t, "CNY", 100) {
		t.Errorf("sourceA expense = %q, want %q", ssA.Expense, majorAmount(t, "CNY", 100))
	}
	ssB := findSourceSummary(t, out.BySource, sourceB.Source.ID)
	if ssB.Expense != majorAmount(t, "CNY", 250) {
		t.Errorf("sourceB expense = %q, want %q", ssB.Expense, majorAmount(t, "CNY", 250))
	}

	for _, ss := range out.BySource {
		if ss.SourceID == sourceC.Source.ID {
			t.Fatalf("unused source %q unexpectedly present in BySource", sourceC.Source.ID)
		}
	}
}

func TestGetFinancialSummaryRejectsNonexistentLedger(t *testing.T) {
	session, _ := newTestSession(t)

	callToolExpectError(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{LedgerID: "999999"})
}

// TestGetFinancialSummaryIsolatedByLedger verifies that summarizing one
// ledger never mixes in another ledger's transactions, even when both hold
// activity in the same currency over the same time range.
func TestGetFinancialSummaryIsolatedByLedger(t *testing.T) {
	session, ledgerA := newTestSession(t)
	sourceA, categoryA := setupSourceAndCategory(t, session, ledgerA)
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerA,
		Type:     "expense", SourceID: sourceA, CategoryID: categoryA, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID
	sourceB, categoryB := setupSourceAndCategory(t, session, ledgerB)
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerB,
		Type:     "expense", SourceID: sourceB, CategoryID: categoryB, Amount: cnyAmount(999), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{LedgerID: ledgerA}, &out)

	ct := findCurrencyTotals(t, out.TotalsByCurrency, "CNY")
	if ct.Expense != majorAmount(t, "CNY", 100) {
		t.Errorf("TotalsByCurrency expense = %q, want %q (ledgerB's 999 must not be included)", ct.Expense, majorAmount(t, "CNY", 100))
	}
	if len(out.BySource) != 1 || len(out.ByCategory) != 1 {
		t.Fatalf("expected exactly ledgerA's source/category in the breakdown, got BySource=%+v ByCategory=%+v", out.BySource, out.ByCategory)
	}
}

// TestGetFinancialSummaryNetExpenseIsNegative covers spec.md's "净支出场景
// 净额带负号" scenario: when expense exceeds income, net is a decimal string
// carrying a leading minus sign, while income/expense themselves stay
// non-negative.
func TestGetFinancialSummaryNetExpenseIsNegative(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "income", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(3000), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(5000), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{LedgerID: ledgerID}, &out)

	ct := findCurrencyTotals(t, out.TotalsByCurrency, "CNY")
	if ct.Income != majorAmount(t, "CNY", 3000) {
		t.Errorf("Income = %q, want %q", ct.Income, majorAmount(t, "CNY", 3000))
	}
	if ct.Expense != majorAmount(t, "CNY", 5000) {
		t.Errorf("Expense = %q, want %q", ct.Expense, majorAmount(t, "CNY", 5000))
	}
	if ct.Net != "-20.00" {
		t.Errorf("Net = %q, want %q (net expense must carry a minus sign)", ct.Net, "-20.00")
	}
}

// TestGetFinancialSummaryNonCNYCurrencyPrecision covers spec.md's precision
// guarantee for get_financial_summary's income/expense/net fields on
// non-CNY currencies -- they must reflect that currency's own standard
// precision, not silently degrade to two decimal places.
func TestGetFinancialSummaryNonCNYCurrencyPrecision(t *testing.T) {
	session, _ := newTestSession(t)

	cases := []struct {
		name          string
		currency      string
		incomeAmount  string
		expenseAmount string
		wantIncome    string
		wantExpense   string
		wantNet       string
	}{
		{"JPY zero-decimal", "JPY", "8000", "5000", "8000", "5000", "3000"},
		{"BHD three-decimal", "BHD", "3.000", "5.000", "3.000", "5.000", "-2.000"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var ledgerOut tools.ManageLedgerOutput
			callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Precision " + c.name}, &ledgerOut)
			ledgerID := ledgerOut.Ledger.ID
			sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

			callTool(t, session, "create_transaction", tools.CreateTransactionInput{
				LedgerID: ledgerID,
				Type:     "income", SourceID: sourceID, CategoryID: categoryID, Amount: c.incomeAmount, Currency: c.currency, Time: futureTime(),
			}, &tools.CreateTransactionOutput{})
			callTool(t, session, "create_transaction", tools.CreateTransactionInput{
				LedgerID: ledgerID,
				Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: c.expenseAmount, Currency: c.currency, Time: futureTime(),
			}, &tools.CreateTransactionOutput{})

			var out tools.GetFinancialSummaryOutput
			callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{LedgerID: ledgerID}, &out)

			ct := findCurrencyTotals(t, out.TotalsByCurrency, c.currency)
			if ct.Income != c.wantIncome {
				t.Errorf("Income = %q, want %q", ct.Income, c.wantIncome)
			}
			if ct.Expense != c.wantExpense {
				t.Errorf("Expense = %q, want %q", ct.Expense, c.wantExpense)
			}
			if ct.Net != c.wantNet {
				t.Errorf("Net = %q, want %q", ct.Net, c.wantNet)
			}
		})
	}
}
