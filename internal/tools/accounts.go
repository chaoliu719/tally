package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/validators"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func init() {
	register(registerAccountTools)
}

func registerAccountTools(s *mcp.Server, deps Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_accounts",
		Description: "List every account in the ledger, including its name, type, currency, and current balance.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ ListAccountsInput) (*mcp.CallToolResult, ListAccountsOutput, error) {
		return listAccounts(deps)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "manage_account",
		Description: "Create a new account in the ledger.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in ManageAccountInput) (*mcp.CallToolResult, ManageAccountOutput, error) {
		return manageAccount(deps, in)
	})
}

// accountCategoryByName / accountCategoryNames provide the string<->enum
// mapping for models.AccountCategory used on the MCP wire format, since the
// numeric enum values are not meaningful to a tool caller.
var accountCategoryByName = map[string]models.AccountCategory{
	"cash":                   models.ACCOUNT_CATEGORY_CASH,
	"checking_account":       models.ACCOUNT_CATEGORY_CHECKING_ACCOUNT,
	"credit_card":            models.ACCOUNT_CATEGORY_CREDIT_CARD,
	"virtual":                models.ACCOUNT_CATEGORY_VIRTUAL,
	"debt":                   models.ACCOUNT_CATEGORY_DEBT,
	"receivables":            models.ACCOUNT_CATEGORY_RECEIVABLES,
	"investment":             models.ACCOUNT_CATEGORY_INVESTMENT,
	"savings_account":        models.ACCOUNT_CATEGORY_SAVINGS_ACCOUNT,
	"certificate_of_deposit": models.ACCOUNT_CATEGORY_CERTIFICATE_OF_DEPOSIT,
}

var accountCategoryNames = func() map[models.AccountCategory]string {
	names := make(map[models.AccountCategory]string, len(accountCategoryByName))
	for name, category := range accountCategoryByName {
		names[category] = name
	}
	return names
}()

// AccountInfo is the wire representation of an account returned by
// list_accounts and manage_account.
type AccountInfo struct {
	ID       string `json:"id" jsonschema:"the account's unique id, as a decimal string"`
	Name     string `json:"name" jsonschema:"the account's name"`
	Type     string `json:"type" jsonschema:"the account's type, e.g. cash, checking_account, credit_card"`
	Currency string `json:"currency" jsonschema:"the account's currency, as an ISO 4217 code, e.g. CNY, USD"`
	Balance  int64  `json:"balance" jsonschema:"the account's current balance, in the currency's smallest unit (e.g. cents)"`
}

type ListAccountsInput struct{}

type ListAccountsOutput struct {
	Accounts []AccountInfo `json:"accounts" jsonschema:"every account in the ledger"`
}

func listAccounts(deps Deps) (*mcp.CallToolResult, ListAccountsOutput, error) {
	ctx := core.NewNullContext()

	accounts, err := services.Accounts.GetAllAccountsByUid(ctx, deps.UID)
	if err != nil {
		return nil, ListAccountsOutput{}, err
	}

	infos := make([]AccountInfo, 0, len(accounts))
	for _, a := range accounts {
		infos = append(infos, toAccountInfo(a))
	}

	return nil, ListAccountsOutput{Accounts: infos}, nil
}

type ManageAccountInput struct {
	Name     string `json:"name" jsonschema:"the account's name"`
	Type     string `json:"type" jsonschema:"the account's type: cash, checking_account, credit_card, virtual, debt, receivables, investment, savings_account, or certificate_of_deposit"`
	Currency string `json:"currency" jsonschema:"the account's currency, as an ISO 4217 code, e.g. CNY, USD"`
	Balance  int64  `json:"balance,omitempty" jsonschema:"the account's initial balance, in the currency's smallest unit (e.g. cents); defaults to 0"`
	Comment  string `json:"comment,omitempty" jsonschema:"an optional note about the account"`
}

type ManageAccountOutput struct {
	Account AccountInfo `json:"account" jsonschema:"the newly created account"`
}

func manageAccount(deps Deps, in ManageAccountInput) (*mcp.CallToolResult, ManageAccountOutput, error) {
	if in.Name == "" {
		return nil, ManageAccountOutput{}, fmt.Errorf("missing required field: name")
	}

	category, ok := accountCategoryByName[in.Type]
	if !ok {
		return nil, ManageAccountOutput{}, fmt.Errorf("missing or unsupported account type: %q", in.Type)
	}

	if in.Currency == "" {
		return nil, ManageAccountOutput{}, fmt.Errorf("missing required field: currency")
	}
	if !validators.AllCurrencyNames[in.Currency] {
		return nil, ManageAccountOutput{}, fmt.Errorf("unsupported currency: %q", in.Currency)
	}

	ctx := core.NewNullContext()

	maxOrder, err := services.Accounts.GetMaxDisplayOrder(ctx, deps.UID, category)
	if err != nil {
		return nil, ManageAccountOutput{}, err
	}

	account := &models.Account{
		Uid:          deps.UID,
		Name:         in.Name,
		Category:     category,
		Type:         models.ACCOUNT_TYPE_SINGLE_ACCOUNT,
		DisplayOrder: maxOrder + 1,
		Icon:         1,
		IconType:     core.ICON_TYPE_SYSTEM,
		Color:        "000000",
		Currency:     in.Currency,
		Balance:      in.Balance,
		Comment:      in.Comment,
	}

	if err := services.Accounts.CreateAccounts(ctx, account, 0, nil, nil, time.UTC); err != nil {
		return nil, ManageAccountOutput{}, err
	}

	return nil, ManageAccountOutput{Account: toAccountInfo(account)}, nil
}

func toAccountInfo(a *models.Account) AccountInfo {
	return AccountInfo{
		ID:       formatID(a.AccountId),
		Name:     a.Name,
		Type:     accountCategoryNames[a.Category],
		Currency: a.Currency,
		Balance:  a.Balance,
	}
}
