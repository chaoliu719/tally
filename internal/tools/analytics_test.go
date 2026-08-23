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

// findAccountSummary returns the AccountSummary entry for accountID, failing
// the test if it's not present.
func findAccountSummary(t *testing.T, rows []tools.AccountSummary, accountID string) tools.AccountSummary {
	t.Helper()
	for _, as := range rows {
		if as.AccountID == accountID {
			return as
		}
	}
	t.Fatalf("no AccountSummary for account %q in %+v", accountID, rows)
	return tools.AccountSummary{}
}

func TestGetFinancialSummaryTimeRange(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 0)

	earlyTime := futureTime()
	lateTime := earlyTime + 100000

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 100, Time: earlyTime,
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 200, Time: lateTime,
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

	as := findAccountSummary(t, out.ByAccount, accountID)
	if as.Expense != 100 || as.Income != 0 {
		t.Errorf("ByAccount summary = %+v, want income=0 expense=100", as)
	}
}

func TestGetFinancialSummaryNoTimeRange(t *testing.T) {
	session := newTestSession(t)
	// Nonzero initial balance triggers an implicit balance_adjustment
	// transaction (see manage_account's operation=create), which this test
	// deliberately includes since it queries with no time filter.
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "income", AccountID: accountID, CategoryID: categoryID, Amount: 500, Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 200, Time: futureTime(),
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

	as := findAccountSummary(t, out.ByAccount, accountID)
	if as.Income != 500 || as.Expense != 200 {
		t.Errorf("ByAccount summary = %+v, want income=500 expense=200", as)
	}

	ba := findBalanceAdjustment(t, out.BalanceAdjustmentByCurrency, "CNY")
	if ba.Amount != 10000 {
		t.Errorf("BalanceAdjustmentByCurrency amount = %d, want 10000", ba.Amount)
	}
}

func TestGetFinancialSummaryEmptyRange(t *testing.T) {
	session := newTestSession(t)
	accountID, categoryID := setupAccountAndCategory(t, session, 10000)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 100, Time: futureTime(),
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
	if len(out.ByAccount) != 0 {
		t.Errorf("ByAccount = %+v, want empty", out.ByAccount)
	}
	if len(out.BalanceAdjustmentByCurrency) != 0 {
		t.Errorf("BalanceAdjustmentByCurrency = %+v, want empty", out.BalanceAdjustmentByCurrency)
	}
}

func TestGetFinancialSummaryMultiCurrency(t *testing.T) {
	session := newTestSession(t)

	var accountCNY tools.ManageAccountOutput
	callTool(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "create", Name: "CNY Wallet", Type: "cash", Currency: "CNY",
	}, &accountCNY)

	var accountUSD tools.ManageAccountOutput
	callTool(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "create", Name: "USD Wallet", Type: "cash", Currency: "USD",
	}, &accountUSD)

	var category tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create", Name: "Misc",
	}, &category)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "income", AccountID: accountCNY.Account.ID, CategoryID: category.Category.ID, Amount: 1000, Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountUSD.Account.ID, CategoryID: category.Category.ID, Amount: 300, Time: futureTime(),
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
	accountID, categoryA := setupAccountAndCategory(t, session, 0)

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
		Type: "expense", AccountID: accountID, CategoryID: categoryA, Amount: 100, Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryB.Category.ID, Amount: 250, Time: futureTime(),
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

func TestGetFinancialSummaryByAccount(t *testing.T) {
	session := newTestSession(t)
	accountA, categoryID := setupAccountAndCategory(t, session, 0)

	var accountB tools.ManageAccountOutput
	callTool(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "create", Name: "Second Wallet", Type: "cash", Currency: "CNY",
	}, &accountB)

	// A third account with no transactions must not appear in the result.
	var accountC tools.ManageAccountOutput
	callTool(t, session, "manage_account", tools.ManageAccountInput{
		Operation: "create", Name: "Unused Wallet", Type: "cash", Currency: "CNY",
	}, &accountC)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountA, CategoryID: categoryID, Amount: 100, Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountB.Account.ID, CategoryID: categoryID, Amount: 250, Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{}, &out)

	if len(out.ByAccount) != 2 {
		t.Fatalf("expected 2 account summaries, got %d: %+v", len(out.ByAccount), out.ByAccount)
	}

	asA := findAccountSummary(t, out.ByAccount, accountA)
	if asA.Expense != 100 {
		t.Errorf("accountA expense = %d, want 100", asA.Expense)
	}
	asB := findAccountSummary(t, out.ByAccount, accountB.Account.ID)
	if asB.Expense != 250 {
		t.Errorf("accountB expense = %d, want 250", asB.Expense)
	}

	for _, as := range out.ByAccount {
		if as.AccountID == accountC.Account.ID {
			t.Fatalf("unused account %q unexpectedly present in ByAccount", accountC.Account.ID)
		}
	}
}

// findBalanceAdjustment returns the BalanceAdjustmentTotal entry for
// currency, failing the test if it's not present.
func findBalanceAdjustment(t *testing.T, rows []tools.BalanceAdjustmentTotal, currency string) tools.BalanceAdjustmentTotal {
	t.Helper()
	for _, ba := range rows {
		if ba.Currency == currency {
			return ba
		}
	}
	t.Fatalf("no BalanceAdjustmentTotal for currency %q in %+v", currency, rows)
	return tools.BalanceAdjustmentTotal{}
}

func TestGetFinancialSummaryBalanceAdjustmentPresent(t *testing.T) {
	session := newTestSession(t)
	// Zero initial balance so the only balance_adjustment transaction is the
	// one this test creates explicitly, inside the queried time range.
	accountID, categoryID := setupAccountAndCategory(t, session, 0)

	rangeStart := futureTime()
	rangeEnd := rangeStart + 1000

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 100, Time: rangeStart + 10,
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "balance_adjustment", AccountID: accountID, Amount: -50, Time: rangeStart + 20,
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{
		StartTime: rangeStart,
		EndTime:   rangeEnd,
	}, &out)

	ba := findBalanceAdjustment(t, out.BalanceAdjustmentByCurrency, "CNY")
	if ba.Amount != -50 {
		t.Errorf("BalanceAdjustmentByCurrency amount = %d, want -50", ba.Amount)
	}

	// The balance_adjustment must not leak into totals/by_category/by_account.
	ct := findCurrencyTotals(t, out.TotalsByCurrency, "CNY")
	if ct.Income != 0 || ct.Expense != 100 || ct.Net != -100 {
		t.Errorf("TotalsByCurrency = %+v, want income=0 expense=100 net=-100 (balance_adjustment excluded)", ct)
	}
	if len(out.ByCategory) != 1 || out.ByCategory[0].Income != 0 || out.ByCategory[0].Expense != 100 {
		t.Errorf("ByCategory = %+v, want a single entry with income=0 expense=100", out.ByCategory)
	}
	if len(out.ByAccount) != 1 || out.ByAccount[0].Income != 0 || out.ByAccount[0].Expense != 100 {
		t.Errorf("ByAccount = %+v, want a single entry with income=0 expense=100", out.ByAccount)
	}
}

func TestGetFinancialSummaryBalanceAdjustmentAbsent(t *testing.T) {
	session := newTestSession(t)
	// Zero initial balance means no implicit balance_adjustment transaction.
	accountID, categoryID := setupAccountAndCategory(t, session, 0)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", AccountID: accountID, CategoryID: categoryID, Amount: 100, Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.GetFinancialSummaryOutput
	callTool(t, session, "get_financial_summary", tools.GetFinancialSummaryInput{}, &out)

	if len(out.BalanceAdjustmentByCurrency) != 0 {
		t.Errorf("BalanceAdjustmentByCurrency = %+v, want empty", out.BalanceAdjustmentByCurrency)
	}
}
