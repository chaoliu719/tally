package tools

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/confirm"
	"tally/internal/currency"
	"tally/internal/store"
)

func init() {
	register(registerAccountTools)
}

func registerAccountTools(s *mcp.Server, deps Deps) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_accounts",
		Description: "List every account in the ledger, including its name, type, currency, and current balance.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ ListAccountsInput) (*mcp.CallToolResult, ListAccountsOutput, error) {
		return listAccounts(ctx, deps)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "manage_account",
		Description: "Create, update, or delete an account, selected via the operation field. " +
			"operation=create makes a new account. operation=update replaces name/type/comment on an " +
			"existing account (all three required; currency and balance cannot be changed). " +
			"operation=delete is a two-step preview -> apply: call without confirmation_token to preview " +
			"(fails if the account is still referenced by any transaction), then call again with the " +
			"returned confirmation_token to actually delete.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ManageAccountInput) (*mcp.CallToolResult, ManageAccountOutput, error) {
		return manageAccount(ctx, deps, in)
	})
}

// accountTypes is the set of account types tally accepts. It's a plain TEXT
// column in the schema (see design.md) -- validation happens here in Go,
// not as a database CHECK constraint.
var accountTypes = map[string]bool{
	"cash":                   true,
	"checking_account":       true,
	"credit_card":            true,
	"virtual":                true,
	"debt":                   true,
	"receivables":            true,
	"investment":             true,
	"savings_account":        true,
	"certificate_of_deposit": true,
}

// AccountInfo is the wire representation of an account returned by
// list_accounts and manage_account.
type AccountInfo struct {
	ID       string `json:"id" jsonschema:"the account's unique id, as a decimal string"`
	Name     string `json:"name" jsonschema:"the account's name"`
	Type     string `json:"type" jsonschema:"the account's type, e.g. cash, checking_account, credit_card"`
	Currency string `json:"currency" jsonschema:"the account's currency, as an ISO 4217 code, e.g. CNY, USD"`
	Balance  int64  `json:"balance" jsonschema:"the account's current balance, in the currency's smallest unit; how many decimal places that represents varies by currency (e.g. 2 for USD/CNY, 0 for JPY, 3 for BHD)"`
}

type ListAccountsInput struct{}

type ListAccountsOutput struct {
	Accounts []AccountInfo `json:"accounts" jsonschema:"every account in the ledger"`
}

func listAccounts(ctx context.Context, deps Deps) (*mcp.CallToolResult, ListAccountsOutput, error) {
	rows, err := deps.Q.ListAccounts(ctx)
	if err != nil {
		return nil, ListAccountsOutput{}, err
	}

	infos := make([]AccountInfo, 0, len(rows))
	for _, a := range rows {
		infos = append(infos, AccountInfo{
			ID:       formatID(a.ID),
			Name:     a.Name,
			Type:     a.Type,
			Currency: a.Currency,
			Balance:  a.Balance,
		})
	}

	return nil, ListAccountsOutput{Accounts: infos}, nil
}

type ManageAccountInput struct {
	Operation string `json:"operation" jsonschema:"the operation to perform: create, update, or delete"`
	ID        string `json:"id,omitempty" jsonschema:"the account's id, as a decimal string; required for operation=update and operation=delete"`

	Name    string `json:"name,omitempty" jsonschema:"the account's name; required for create and update"`
	Type    string `json:"type,omitempty" jsonschema:"the account's type: cash, checking_account, credit_card, virtual, debt, receivables, investment, savings_account, or certificate_of_deposit; required for create and update"`
	Comment string `json:"comment,omitempty" jsonschema:"an optional note about the account; used by create and update"`

	Currency string `json:"currency,omitempty" jsonschema:"the account's currency, as an ISO 4217 code, e.g. CNY, USD; required for create, and cannot be provided for update"`
	Balance  int64  `json:"balance,omitempty" jsonschema:"the account's initial balance, in the currency's smallest unit; only used by create (defaults to 0); cannot be provided for update"`

	ConfirmationToken string `json:"confirmation_token,omitempty" jsonschema:"for operation=delete only: omit to preview the deletion and receive a token, or supply the token from a prior preview to actually delete"`
}

type ManageAccountOutput struct {
	Account           AccountInfo `json:"account" jsonschema:"the account's current (or, for a completed delete, last known) info"`
	Status            string      `json:"status" jsonschema:"created, updated, pending_confirmation, or deleted"`
	ConfirmationToken string      `json:"confirmation_token,omitempty" jsonschema:"present when status is pending_confirmation; pass this back as confirmation_token with the same operation=delete call to actually delete"`
	ExpiresAt         int64       `json:"expires_at,omitempty" jsonschema:"unix seconds when confirmation_token expires; present when status is pending_confirmation"`
}

func manageAccount(ctx context.Context, deps Deps, in ManageAccountInput) (*mcp.CallToolResult, ManageAccountOutput, error) {
	switch in.Operation {
	case "create":
		return createAccount(ctx, deps, in)
	case "update":
		return updateAccount(ctx, deps, in)
	case "delete":
		return deleteAccount(ctx, deps, in)
	default:
		return nil, ManageAccountOutput{}, fmt.Errorf("missing or unsupported operation: %q (must be create, update, or delete)", in.Operation)
	}
}

func createAccount(ctx context.Context, deps Deps, in ManageAccountInput) (*mcp.CallToolResult, ManageAccountOutput, error) {
	if in.Name == "" {
		return nil, ManageAccountOutput{}, fmt.Errorf("missing required field: name")
	}
	if !accountTypes[in.Type] {
		return nil, ManageAccountOutput{}, fmt.Errorf("missing or unsupported account type: %q", in.Type)
	}
	if in.Currency == "" {
		return nil, ManageAccountOutput{}, fmt.Errorf("missing required field: currency")
	}
	if !currency.Supported(in.Currency) {
		return nil, ManageAccountOutput{}, fmt.Errorf("unsupported currency: %q", in.Currency)
	}

	tx, err := deps.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, ManageAccountOutput{}, err
	}
	defer tx.Rollback()

	q := deps.Q.WithTx(tx)
	now := time.Now().Unix()

	account, err := q.CreateAccount(ctx, store.CreateAccountParams{
		Name:      in.Name,
		Type:      in.Type,
		Currency:  in.Currency,
		Comment:   in.Comment,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return nil, ManageAccountOutput{}, err
	}

	// A nonzero initial balance is recorded as an adjustment
	// transaction in the same DB transaction as the account row, so a
	// concurrent list_accounts can never observe the account with the
	// account's balance (computed as SUM(amount)) still at zero.
	if in.Balance != 0 {
		if _, err := q.CreateTransaction(ctx, store.CreateTransactionParams{
			Type:       "adjustment",
			AccountID:  account.ID,
			CategoryID: sql.NullInt64{},
			Amount:     in.Balance,
			Time:       now,
			CreatedAt:  now,
			UpdatedAt:  now,
		}); err != nil {
			return nil, ManageAccountOutput{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, ManageAccountOutput{}, err
	}

	return nil, ManageAccountOutput{
		Status: "created",
		Account: AccountInfo{
			ID:       formatID(account.ID),
			Name:     account.Name,
			Type:     account.Type,
			Currency: account.Currency,
			Balance:  in.Balance,
		},
	}, nil
}

func updateAccount(ctx context.Context, deps Deps, in ManageAccountInput) (*mcp.CallToolResult, ManageAccountOutput, error) {
	if in.ID == "" {
		return nil, ManageAccountOutput{}, fmt.Errorf("missing required field: id")
	}
	id, err := parseID(in.ID)
	if err != nil {
		return nil, ManageAccountOutput{}, err
	}

	if in.Name == "" {
		return nil, ManageAccountOutput{}, fmt.Errorf("missing required field: name")
	}
	if !accountTypes[in.Type] {
		return nil, ManageAccountOutput{}, fmt.Errorf("missing or unsupported account type: %q", in.Type)
	}
	if in.Currency != "" {
		return nil, ManageAccountOutput{}, fmt.Errorf("currency cannot be changed via operation=update")
	}
	if in.Balance != 0 {
		return nil, ManageAccountOutput{}, fmt.Errorf("balance cannot be changed via operation=update")
	}

	if _, err := deps.Q.GetAccount(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ManageAccountOutput{}, fmt.Errorf("account %q not found", in.ID)
		}
		return nil, ManageAccountOutput{}, err
	}

	updated, err := deps.Q.UpdateAccount(ctx, store.UpdateAccountParams{
		Name:      in.Name,
		Type:      in.Type,
		Comment:   in.Comment,
		UpdatedAt: time.Now().Unix(),
		ID:        id,
	})
	if err != nil {
		return nil, ManageAccountOutput{}, err
	}

	balance, err := deps.Q.GetAccountBalance(ctx, id)
	if err != nil {
		return nil, ManageAccountOutput{}, err
	}

	return nil, ManageAccountOutput{
		Status: "updated",
		Account: AccountInfo{
			ID:       formatID(updated.ID),
			Name:     updated.Name,
			Type:     updated.Type,
			Currency: updated.Currency,
			Balance:  balance,
		},
	}, nil
}

// accountDeletionRevisionFields is hashed to produce the revision embedded
// in an account deletion's confirmation token (see design.md's "revision 覆盖
// 哪些字段"). TxCount is what actually gates whether the delete is allowed;
// the rest guards against the previewed account having changed underneath
// the caller.
type accountDeletionRevisionFields struct {
	Name     string
	Type     string
	Comment  string
	Currency string
	TxCount  int64
}

func accountDeletionRevision(a store.Account, txCount int64) string {
	fields := accountDeletionRevisionFields{
		Name:     a.Name,
		Type:     a.Type,
		Comment:  a.Comment,
		Currency: a.Currency,
		TxCount:  txCount,
	}
	// Marshaling a fixed struct of plain strings/ints never fails.
	body, _ := json.Marshal(fields)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

const confirmActionDeleteAccount = "delete_account"

func deleteAccount(ctx context.Context, deps Deps, in ManageAccountInput) (*mcp.CallToolResult, ManageAccountOutput, error) {
	if in.ID == "" {
		return nil, ManageAccountOutput{}, fmt.Errorf("missing required field: id")
	}
	id, err := parseID(in.ID)
	if err != nil {
		return nil, ManageAccountOutput{}, err
	}

	account, err := deps.Q.GetAccount(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ManageAccountOutput{}, fmt.Errorf("account %q not found", in.ID)
		}
		return nil, ManageAccountOutput{}, err
	}

	txCount, err := deps.Q.CountTransactionsByAccount(ctx, id)
	if err != nil {
		return nil, ManageAccountOutput{}, err
	}

	balance, err := deps.Q.GetAccountBalance(ctx, id)
	if err != nil {
		return nil, ManageAccountOutput{}, err
	}

	info := AccountInfo{
		ID:       formatID(account.ID),
		Name:     account.Name,
		Type:     account.Type,
		Currency: account.Currency,
		Balance:  balance,
	}
	revision := accountDeletionRevision(account, txCount)

	if in.ConfirmationToken == "" {
		if txCount != 0 {
			return nil, ManageAccountOutput{}, fmt.Errorf("account %q is still referenced by %d transaction(s) and cannot be deleted", in.ID, txCount)
		}

		token, expiresAt := confirm.Issue(deps.ConfirmSecret, confirmActionDeleteAccount, in.ID, revision)
		return nil, ManageAccountOutput{
			Status:            "pending_confirmation",
			Account:           info,
			ConfirmationToken: token,
			ExpiresAt:         expiresAt,
		}, nil
	}

	if err := confirm.Verify(deps.ConfirmSecret, in.ConfirmationToken, confirmActionDeleteAccount, in.ID, revision, time.Now()); err != nil {
		return nil, ManageAccountOutput{}, err
	}

	tx, err := deps.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, ManageAccountOutput{}, err
	}
	defer tx.Rollback()

	q := deps.Q.WithTx(tx)

	liveTxCount, err := q.CountTransactionsByAccount(ctx, id)
	if err != nil {
		return nil, ManageAccountOutput{}, err
	}
	if liveTxCount != 0 {
		return nil, ManageAccountOutput{}, fmt.Errorf("account %q is now referenced by %d transaction(s) and cannot be deleted; please preview again", in.ID, liveTxCount)
	}

	if err := q.DeleteAccount(ctx, id); err != nil {
		return nil, ManageAccountOutput{}, err
	}

	if err := tx.Commit(); err != nil {
		return nil, ManageAccountOutput{}, err
	}

	return nil, ManageAccountOutput{Status: "deleted", Account: info}, nil
}
