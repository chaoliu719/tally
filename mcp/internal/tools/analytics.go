package tools

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/currency"
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
	Income   string `json:"income" jsonschema:"total income, as a decimal string in this currency's major unit, over the requested time range"`
	Expense  string `json:"expense" jsonschema:"total expense (as a positive decimal string), in this currency's major unit, over the requested time range"`
	Net      string `json:"net" jsonschema:"income minus expense, as a decimal string in this currency's major unit; negative (net expense) values are prefixed with a minus sign, e.g. \"-50.00\""`
}

// CategorySummary is the income/expense subtotal for one category in one
// currency. A category with transactions in multiple currencies appears as
// multiple rows, one per currency.
type CategorySummary struct {
	CategoryID string `json:"category_id" jsonschema:"the category's unique id, as a decimal string"`
	Currency   string `json:"currency" jsonschema:"the ISO 4217 currency code this subtotal is denominated in"`
	Income     string `json:"income" jsonschema:"total income in this category and currency, as a decimal string in the currency's major unit"`
	Expense    string `json:"expense" jsonschema:"total expense (as a positive decimal string) in this category and currency, in the currency's major unit"`
}

// SourceSummary is the income/expense subtotal for one source in one
// currency. A source with transactions in multiple currencies appears as
// multiple rows, one per currency.
type SourceSummary struct {
	SourceID string `json:"source_id" jsonschema:"the source's unique id, as a decimal string"`
	Currency string `json:"currency" jsonschema:"the ISO 4217 currency code this subtotal is denominated in"`
	Income   string `json:"income" jsonschema:"total income on this source and currency, as a decimal string in the currency's major unit"`
	Expense  string `json:"expense" jsonschema:"total expense on this source and currency (as a positive decimal string), in the currency's major unit"`
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
			income, err := currency.FormatMajor(r.Currency, r.Income)
			if err != nil {
				return nil, GetFinancialSummaryOutput{}, err
			}
			expense, err := currency.FormatMajor(r.Currency, r.Expense)
			if err != nil {
				return nil, GetFinancialSummaryOutput{}, err
			}
			net, err := currency.FormatMajor(r.Currency, r.Income-r.Expense)
			if err != nil {
				return nil, GetFinancialSummaryOutput{}, err
			}
			totals = append(totals, CurrencyTotals{
				Currency: r.Currency,
				Income:   income,
				Expense:  expense,
				Net:      net,
			})
		}
	}

	byCategory := make([]CategorySummary, 0, len(categoryRows))
	for _, r := range categoryRows {
		income, err := currency.FormatMajor(r.Currency, r.Income)
		if err != nil {
			return nil, GetFinancialSummaryOutput{}, err
		}
		expense, err := currency.FormatMajor(r.Currency, r.Expense)
		if err != nil {
			return nil, GetFinancialSummaryOutput{}, err
		}
		byCategory = append(byCategory, CategorySummary{
			CategoryID: formatID(r.CategoryID),
			Currency:   r.Currency,
			Income:     income,
			Expense:    expense,
		})
	}

	bySource := make([]SourceSummary, 0, len(sourceRows))
	for _, r := range sourceRows {
		income, err := currency.FormatMajor(r.Currency, r.Income)
		if err != nil {
			return nil, GetFinancialSummaryOutput{}, err
		}
		expense, err := currency.FormatMajor(r.Currency, r.Expense)
		if err != nil {
			return nil, GetFinancialSummaryOutput{}, err
		}
		bySource = append(bySource, SourceSummary{
			SourceID: formatID(r.SourceID),
			Currency: r.Currency,
			Income:   income,
			Expense:  expense,
		})
	}

	return nil, GetFinancialSummaryOutput{
		TotalsByCurrency: totals,
		ByCategory:       byCategory,
		BySource:         bySource,
	}, nil
}
