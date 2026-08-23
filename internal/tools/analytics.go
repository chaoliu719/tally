package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/store"
)

func init() {
	register(registerAnalyticsTools)
}

func registerAnalyticsTools(s *mcp.Server, deps Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "get_financial_summary",
		Description: "Aggregate income, expense, and net totals over an optional time range, grouped by " +
			"currency, and broken down by category and by account. adjustment transactions are not " +
			"real income/expense activity, so they're excluded from these totals and reported separately, " +
			"grouped by currency. With no time range, summarizes the ledger's entire history.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GetFinancialSummaryInput) (*mcp.CallToolResult, GetFinancialSummaryOutput, error) {
		return getFinancialSummary(ctx, deps, in)
	})
}

type GetFinancialSummaryInput struct {
	StartTime int64 `json:"start_time,omitempty" jsonschema:"only include transactions at or after this unix time (seconds); omit for no lower bound"`
	EndTime   int64 `json:"end_time,omitempty" jsonschema:"only include transactions at or before this unix time (seconds); omit for no upper bound"`
}

// CurrencyTotals is the income/expense/net summary for one currency, over
// the requested time range. Net is income minus expense.
type CurrencyTotals struct {
	Currency string `json:"currency" jsonschema:"the ISO 4217 currency code this total is denominated in"`
	Income   int64  `json:"income" jsonschema:"total income in this currency's smallest unit, over the requested time range"`
	Expense  int64  `json:"expense" jsonschema:"total expense (as a positive number) in this currency's smallest unit, over the requested time range"`
	Net      int64  `json:"net" jsonschema:"income minus expense, in this currency's smallest unit"`
}

// CategorySummary is the income/expense subtotal for one category in one
// currency. A category with transactions in multiple currencies appears as
// multiple rows, one per currency.
type CategorySummary struct {
	CategoryID string `json:"category_id" jsonschema:"the category's unique id, as a decimal string"`
	Currency   string `json:"currency" jsonschema:"the ISO 4217 currency code this subtotal is denominated in"`
	Income     int64  `json:"income" jsonschema:"total income in this category and currency, over the requested time range"`
	Expense    int64  `json:"expense" jsonschema:"total expense (as a positive number) in this category and currency, over the requested time range"`
}

// AccountSummary is the income/expense subtotal for one account. Unlike
// CategorySummary, an account has exactly one currency, so there's at most
// one row per account.
type AccountSummary struct {
	AccountID string `json:"account_id" jsonschema:"the account's unique id, as a decimal string"`
	Currency  string `json:"currency" jsonschema:"the account's currency, as an ISO 4217 code"`
	Income    int64  `json:"income" jsonschema:"total income on this account, over the requested time range"`
	Expense   int64  `json:"expense" jsonschema:"total expense on this account (as a positive number), over the requested time range"`
}

// AdjustmentTotal is the total of adjustment transactions in
// one currency. It's reported separately from CurrencyTotals because
// adjustment isn't real income/expense activity (see
// get_financial_summary's description).
type AdjustmentTotal struct {
	Currency string `json:"currency" jsonschema:"the ISO 4217 currency code this total is denominated in"`
	Amount   int64  `json:"amount" jsonschema:"total adjustment amount in this currency's smallest unit, over the requested time range; carries its own sign"`
}

type GetFinancialSummaryOutput struct {
	TotalsByCurrency            []CurrencyTotals         `json:"totals_by_currency" jsonschema:"total income/expense/net for the requested time range, one entry per currency that had any income/expense activity"`
	ByCategory                  []CategorySummary        `json:"by_category" jsonschema:"income/expense subtotals broken down by category (and currency); only categories with matching transactions appear"`
	ByAccount                   []AccountSummary         `json:"by_account" jsonschema:"income/expense subtotals broken down by account; only accounts with matching transactions appear"`
	AdjustmentByCurrency []AdjustmentTotal `json:"adjustment_by_currency" jsonschema:"adjustment totals, one entry per currency that had any adjustment transactions; not counted in totals_by_currency, by_category, or by_account"`
}

func getFinancialSummary(ctx context.Context, deps Deps, in GetFinancialSummaryInput) (*mcp.CallToolResult, GetFinancialSummaryOutput, error) {
	var currencyParams store.SummarizeTransactionsByCurrencyParams
	var categoryParams store.SummarizeTransactionsByCategoryParams
	var accountParams store.SummarizeTransactionsByAccountParams

	if in.StartTime > 0 {
		currencyParams.StartTime = in.StartTime
		categoryParams.StartTime = in.StartTime
		accountParams.StartTime = in.StartTime
	}
	if in.EndTime > 0 {
		currencyParams.EndTime = in.EndTime
		categoryParams.EndTime = in.EndTime
		accountParams.EndTime = in.EndTime
	}

	currencyRows, err := deps.Q.SummarizeTransactionsByCurrency(ctx, currencyParams)
	if err != nil {
		return nil, GetFinancialSummaryOutput{}, err
	}

	categoryRows, err := deps.Q.SummarizeTransactionsByCategory(ctx, categoryParams)
	if err != nil {
		return nil, GetFinancialSummaryOutput{}, err
	}

	accountRows, err := deps.Q.SummarizeTransactionsByAccount(ctx, accountParams)
	if err != nil {
		return nil, GetFinancialSummaryOutput{}, err
	}

	totals := make([]CurrencyTotals, 0, len(currencyRows))
	adjustments := make([]AdjustmentTotal, 0, len(currencyRows))
	for _, r := range currencyRows {
		if r.Income != 0 || r.Expense != 0 {
			totals = append(totals, CurrencyTotals{
				Currency: r.Currency,
				Income:   r.Income,
				Expense:  r.Expense,
				Net:      r.Income - r.Expense,
			})
		}
		if r.Adjustment != 0 {
			adjustments = append(adjustments, AdjustmentTotal{
				Currency: r.Currency,
				Amount:   r.Adjustment,
			})
		}
	}

	byCategory := make([]CategorySummary, 0, len(categoryRows))
	for _, r := range categoryRows {
		byCategory = append(byCategory, CategorySummary{
			CategoryID: formatID(r.CategoryID.Int64),
			Currency:   r.Currency,
			Income:     r.Income,
			Expense:    r.Expense,
		})
	}

	byAccount := make([]AccountSummary, 0, len(accountRows))
	for _, r := range accountRows {
		byAccount = append(byAccount, AccountSummary{
			AccountID: formatID(r.AccountID),
			Currency:  r.Currency,
			Income:    r.Income,
			Expense:   r.Expense,
		})
	}

	return nil, GetFinancialSummaryOutput{
		TotalsByCurrency:            totals,
		ByCategory:                  byCategory,
		ByAccount:                   byAccount,
		AdjustmentByCurrency: adjustments,
	}, nil
}
