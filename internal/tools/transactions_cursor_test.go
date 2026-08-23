package tools

import (
	"database/sql"
	"encoding/base64"
	"testing"
)

// These are white-box unit tests of the search_transactions cursor
// encode/decode helpers (see transactions_cursor.go and design.md's "cursor
// 编码"). End-to-end behavior of the same scenarios through the
// search_transactions MCP tool is covered separately in transactions_test.go
// (package tools_test).

func TestSearchTransactionsCursorRoundTrip(t *testing.T) {
	filter := searchTransactionsFilterFields{
		AccountID:  sql.NullInt64{Int64: 7, Valid: true},
		CategoryID: sql.NullInt64{Int64: 3, Valid: true},
		StartTime:  sql.NullInt64{Int64: 1000, Valid: true},
	}

	cursor := encodeSearchTransactionsCursor(1234, 5, filter)
	if cursor == "" {
		t.Fatal("expected a non-empty cursor")
	}

	lastTime, lastID, err := decodeSearchTransactionsCursor(cursor, filter)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if lastTime != 1234 || lastID != 5 {
		t.Fatalf("decoded (last_time, last_id) = (%d, %d), want (1234, 5)", lastTime, lastID)
	}
}

func TestSearchTransactionsCursorRejectsCorruptedInput(t *testing.T) {
	filter := searchTransactionsFilterFields{}

	if _, _, err := decodeSearchTransactionsCursor("not valid base64url!!", filter); err == nil {
		t.Fatal("expected an error for a cursor that isn't valid base64url")
	}

	// Valid base64url, but the decoded bytes aren't valid JSON.
	garbage := base64.RawURLEncoding.EncodeToString([]byte("this is not json"))
	if _, _, err := decodeSearchTransactionsCursor(garbage, filter); err == nil {
		t.Fatal("expected an error for a cursor whose payload isn't valid JSON")
	}
}

func TestSearchTransactionsCursorRejectsMismatchedFilter(t *testing.T) {
	issuedUnder := searchTransactionsFilterFields{AccountID: sql.NullInt64{Int64: 1, Valid: true}}
	cursor := encodeSearchTransactionsCursor(100, 1, issuedUnder)

	differentFilter := searchTransactionsFilterFields{AccountID: sql.NullInt64{Int64: 2, Valid: true}}
	if _, _, err := decodeSearchTransactionsCursor(cursor, differentFilter); err == nil {
		t.Fatal("expected an error when the cursor's filter fingerprint doesn't match the current filters")
	}

	noFilter := searchTransactionsFilterFields{}
	if _, _, err := decodeSearchTransactionsCursor(cursor, noFilter); err == nil {
		t.Fatal("expected an error when the cursor was issued under a filter and the current request has none")
	}
}
