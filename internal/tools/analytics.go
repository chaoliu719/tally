package tools

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/store"
)

func init() {
	register(registerAnalyticsTools)
}

func registerAnalyticsTools(s *mcp.Server, deps Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "get_financial_summary",
		Description: "Aggregate income, expense, and net totals over an optional time range within one ledger, grouped by " +
			"currency, and broken down by category and by source. With no time range, summarizes the " +
			"ledger's entire history.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GetFinancialSummaryInput) (*mcp.CallToolResult, GetFinancialSummaryOutput, error) {
		return getFinancialSummary(ctx, deps, in)
	})
}

type GetFinancialSummaryInput struct {
	LedgerID  string `json:"ledger_id" jsonschema:"the id of the ledger to summarize, as a decimal string"`
	StartTime int64  `json:"start_time,omitempty" jsonschema:"only include transactions at or after this unix time (seconds); omit for no lower bound"`
	EndTime   int64  `json:"end_time,omitempty" jsonschema:"only include transactions at or before this unix time (seconds); omit for no upper bound"`
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

// SourceSummary is the income/expense subtotal for one source in one
// currency. A source with transactions in multiple currencies appears as
// multiple rows, one per currency.
type SourceSummary struct {
	SourceID string `json:"source_id" jsonschema:"the source's unique id, as a decimal string"`
	Currency string `json:"currency" jsonschema:"the ISO 4217 currency code this subtotal is denominated in"`
	Income   int64  `json:"income" jsonschema:"total income on this source and currency, over the requested time range"`
	Expense  int64  `json:"expense" jsonschema:"total expense on this source and currency (as a positive number), over the requested time range"`
}

type GetFinancialSummaryOutput struct {
	TotalsByCurrency []CurrencyTotals  `json:"totals_by_currency" jsonschema:"total income/expense/net for the requested time range, one entry per currency that had any income/expense activity"`
	ByCategory       []CategorySummary `json:"by_category" jsonschema:"income/expense subtotals broken down by category (and currency); only categories with matching transactions appear"`
	BySource         []SourceSummary   `json:"by_source" jsonschema:"income/expense subtotals broken down by source (and currency); only sources with matching transactions appear"`
}

func getFinancialSummary(ctx context.Context, deps Deps, in GetFinancialSummaryInput) (*mcp.CallToolResult, GetFinancialSummaryOutput, error) {
	if in.LedgerID == "" {
		return nil, GetFinancialSummaryOutput{}, fmt.Errorf("missing required field: ledger_id")
	}
	ledgerID, err := parseID(in.LedgerID)
	if err != nil {
		return nil, GetFinancialSummaryOutput{}, err
	}
	if _, err := deps.Q.GetLedger(ctx, ledgerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, GetFinancialSummaryOutput{}, fmt.Errorf("ledger %q not found", in.LedgerID)
		}
		return nil, GetFinancialSummaryOutput{}, err
	}

	currencyParams := store.SummarizeTransactionsByCurrencyParams{LedgerID: ledgerID}
	categoryParams := store.SummarizeTransactionsByCategoryParams{LedgerID: ledgerID}
	sourceParams := store.SummarizeTransactionsBySourceParams{LedgerID: ledgerID}

	if in.StartTime > 0 {
		currencyParams.StartTime = in.StartTime
		categoryParams.StartTime = in.StartTime
		sourceParams.StartTime = in.StartTime
	}
	if in.EndTime > 0 {
		currencyParams.EndTime = in.EndTime
		categoryParams.EndTime = in.EndTime
		sourceParams.EndTime = in.EndTime
	}

	currencyRows, err := deps.Q.SummarizeTransactionsByCurrency(ctx, currencyParams)
	if err != nil {
		return nil, GetFinancialSummaryOutput{}, err
	}

	categoryRows, err := deps.Q.SummarizeTransactionsByCategory(ctx, categoryParams)
	if err != nil {
		return nil, GetFinancialSummaryOutput{}, err
	}

	sourceRows, err := deps.Q.SummarizeTransactionsBySource(ctx, sourceParams)
	if err != nil {
		return nil, GetFinancialSummaryOutput{}, err
	}

	totals := make([]CurrencyTotals, 0, len(currencyRows))
	for _, r := range currencyRows {
		if r.Income != 0 || r.Expense != 0 {
			totals = append(totals, CurrencyTotals{
				Currency: r.Currency,
				Income:   r.Income,
				Expense:  r.Expense,
				Net:      r.Income - r.Expense,
			})
		}
	}

	byCategory := make([]CategorySummary, 0, len(categoryRows))
	for _, r := range categoryRows {
		byCategory = append(byCategory, CategorySummary{
			CategoryID: formatID(r.CategoryID),
			Currency:   r.Currency,
			Income:     r.Income,
			Expense:    r.Expense,
		})
	}

	bySource := make([]SourceSummary, 0, len(sourceRows))
	for _, r := range sourceRows {
		bySource = append(bySource, SourceSummary{
			SourceID: formatID(r.SourceID),
			Currency: r.Currency,
			Income:   r.Income,
			Expense:  r.Expense,
		})
	}

	return nil, GetFinancialSummaryOutput{
		TotalsByCurrency: totals,
		ByCategory:       byCategory,
		BySource:         bySource,
	}, nil
}
