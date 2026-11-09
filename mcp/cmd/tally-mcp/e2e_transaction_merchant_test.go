package main

import (
	"testing"

	"tally/internal/tools"
)

// TestE2ETransactionMerchantSeparateFromComment exercises the merchant field
// end to end over real HTTP wiring: a transaction records the bank's raw
// merchant name in merchant and the user's own note in comment, both come back
// intact, and keyword search finds it through either field.
func TestE2ETransactionMerchantSeparateFromComment(t *testing.T) {
	session, ledgerID := newE2ESession(t)

	var wallet tools.ManageSourceOutput
	call(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID,
		Operation: "create",
		Name:      "Checking",
	}, &wallet)

	var dining tools.ManageCategoryOutput
	call(t, session, "manage_category", tools.ManageCategoryInput{LedgerID: ledgerID,
		Operation: "create",
		Name:      "Dining",
	}, &dining)

	var created tools.CreateTransactionOutput
	call(t, session, "create_transaction", tools.CreateTransactionInput{LedgerID: ledgerID,
		Type:       "expense",
		SourceID:   wallet.Source.ID,
		CategoryID: dining.Category.ID,
		Amount:     cnyAmount(3200),
		Currency:   "CNY",
		Time:       futureTime(),
		Merchant:   "STARBUCKS #4821 TAIPEI",
		Comment:    "和同事喝咖啡",
	}, &created)

	if created.Transaction.Merchant != "STARBUCKS #4821 TAIPEI" {
		t.Fatalf("Merchant = %q, want the raw bank merchant name", created.Transaction.Merchant)
	}
	if created.Transaction.Comment != "和同事喝咖啡" {
		t.Fatalf("Comment = %q, want the user's own note", created.Transaction.Comment)
	}

	var got tools.GetTransactionOutput
	call(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: created.Transaction.ID}, &got)
	if got.Transaction.Merchant != "STARBUCKS #4821 TAIPEI" || got.Transaction.Comment != "和同事喝咖啡" {
		t.Fatalf("get_transaction = merchant %q / comment %q, want both preserved", got.Transaction.Merchant, got.Transaction.Comment)
	}

	// Search by the merchant name, with a case and substring that only exists
	// in merchant.
	var byMerchant tools.SearchTransactionsOutput
	call(t, session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID: ledgerID,
		Keyword:  "4821",
	}, &byMerchant)
	if len(byMerchant.Transactions) != 1 || byMerchant.Transactions[0].ID != created.Transaction.ID {
		t.Fatalf("keyword 4821 = %+v, want only the merchant-named transaction", byMerchant.Transactions)
	}

	// Search by a word that only exists in the user's note.
	var byNote tools.SearchTransactionsOutput
	call(t, session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID: ledgerID,
		Keyword:  "咖啡",
	}, &byNote)
	if len(byNote.Transactions) != 1 || byNote.Transactions[0].ID != created.Transaction.ID {
		t.Fatalf("keyword 咖啡 = %+v, want only the commented transaction", byNote.Transactions)
	}
}
