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
		SourceID:   sql.NullInt64{Int64: 7, Valid: true},
		CategoryID: sql.NullInt64{Int64: 3, Valid: true},
		StartTime:  sql.NullString{String: "2026-01-01 00:00:00", Valid: true},
	}

	cursor := encodeSearchTransactionsCursor("2026-01-02 03:04:05", 5, filter)
	if cursor == "" {
		t.Fatal("expected a non-empty cursor")
	}

	lastTime, lastID, err := decodeSearchTransactionsCursor(cursor, filter)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if lastTime != "2026-01-02 03:04:05" || lastID != 5 {
		t.Fatalf("decoded (last_time, last_id) = (%s, %d), want (2026-01-02 03:04:05, 5)", lastTime, lastID)
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
	issuedUnder := searchTransactionsFilterFields{SourceID: sql.NullInt64{Int64: 1, Valid: true}}
	cursor := encodeSearchTransactionsCursor("2026-01-01 00:00:00", 1, issuedUnder)

	differentFilter := searchTransactionsFilterFields{SourceID: sql.NullInt64{Int64: 2, Valid: true}}
	if _, _, err := decodeSearchTransactionsCursor(cursor, differentFilter); err == nil {
		t.Fatal("expected an error when the cursor's filter fingerprint doesn't match the current filters")
	}

	noFilter := searchTransactionsFilterFields{}
	if _, _, err := decodeSearchTransactionsCursor(cursor, noFilter); err == nil {
		t.Fatal("expected an error when the cursor was issued under a filter and the current request has none")
	}
}

// These cover add-transaction-keyword-search's design.md decision that
// Keyword (trimmed to its canonical value) participates in
// searchTransactionsFilterFingerprint exactly like the other filter fields:
// same keyword -> same fingerprint, different keyword (including switching
// to/from "no keyword") -> different fingerprint, and a cursor issued under
// one keyword is rejected when decoded under another.

func TestSearchTransactionsFilterFingerprintSameKeywordSameFingerprint(t *testing.T) {
	a := searchTransactionsFilterFields{LedgerID: 1, SourceID: sql.NullInt64{Int64: 7, Valid: true}, Keyword: "starbucks"}
	b := searchTransactionsFilterFields{LedgerID: 1, SourceID: sql.NullInt64{Int64: 7, Valid: true}, Keyword: "starbucks"}

	if searchTransactionsFilterFingerprint(a) != searchTransactionsFilterFingerprint(b) {
		t.Fatal("expected identical filters (including keyword) to produce the same fingerprint")
	}
}

func TestSearchTransactionsFilterFingerprintDiffersByKeyword(t *testing.T) {
	base := searchTransactionsFilterFields{LedgerID: 1}
	withKeywordA := searchTransactionsFilterFields{LedgerID: 1, Keyword: "coffee"}
	withKeywordB := searchTransactionsFilterFields{LedgerID: 1, Keyword: "tea"}

	if searchTransactionsFilterFingerprint(base) == searchTransactionsFilterFingerprint(withKeywordA) {
		t.Fatal("expected switching from no keyword to a keyword to change the fingerprint")
	}
	if searchTransactionsFilterFingerprint(withKeywordA) == searchTransactionsFilterFingerprint(base) {
		t.Fatal("expected switching from a keyword to no keyword to change the fingerprint")
	}
	if searchTransactionsFilterFingerprint(withKeywordA) == searchTransactionsFilterFingerprint(withKeywordB) {
		t.Fatal("expected two different keywords to produce different fingerprints")
	}
}

func TestSearchTransactionsCursorRejectsMismatchedKeyword(t *testing.T) {
	issuedWithKeyword := searchTransactionsFilterFields{LedgerID: 1, Keyword: "starbucks"}
	cursor := encodeSearchTransactionsCursor("2026-01-01 00:00:00", 1, issuedWithKeyword)

	// Decoding under no keyword at all must be rejected.
	noKeyword := searchTransactionsFilterFields{LedgerID: 1}
	if _, _, err := decodeSearchTransactionsCursor(cursor, noKeyword); err == nil {
		t.Fatal("expected an error decoding a keyword-issued cursor with no keyword in the current request")
	}

	// Decoding under a different keyword must also be rejected.
	differentKeyword := searchTransactionsFilterFields{LedgerID: 1, Keyword: "walmart"}
	if _, _, err := decodeSearchTransactionsCursor(cursor, differentKeyword); err == nil {
		t.Fatal("expected an error decoding a cursor issued under a different keyword")
	}

	// And the reverse: a cursor issued with no keyword must be rejected when
	// replayed with one.
	issuedWithoutKeyword := searchTransactionsFilterFields{LedgerID: 1}
	cursorNoKeyword := encodeSearchTransactionsCursor("2026-01-01 00:00:00", 1, issuedWithoutKeyword)
	if _, _, err := decodeSearchTransactionsCursor(cursorNoKeyword, searchTransactionsFilterFields{LedgerID: 1, Keyword: "starbucks"}); err == nil {
		t.Fatal("expected an error decoding a no-keyword-issued cursor with a keyword in the current request")
	}
}

// These cover add-transaction-timeline-filters's design.md D4: the cursor's
// filter fingerprint must be sensitive to IncludeDescendants, since it
// changes which rows a given category_id matches.

func TestSearchTransactionsFilterFingerprintDiffersByIncludeDescendants(t *testing.T) {
	exact := searchTransactionsFilterFields{LedgerID: 1, CategoryID: sql.NullInt64{Int64: 3, Valid: true}}
	withDescendants := searchTransactionsFilterFields{LedgerID: 1, CategoryID: sql.NullInt64{Int64: 3, Valid: true}, IncludeDescendants: true}

	if searchTransactionsFilterFingerprint(exact) == searchTransactionsFilterFingerprint(withDescendants) {
		t.Fatal("expected toggling include_descendants to change the fingerprint")
	}
}

func TestSearchTransactionsCursorRejectsMismatchedIncludeDescendants(t *testing.T) {
	issuedWithDescendants := searchTransactionsFilterFields{LedgerID: 1, CategoryID: sql.NullInt64{Int64: 3, Valid: true}, IncludeDescendants: true}
	cursor := encodeSearchTransactionsCursor("2026-01-01 00:00:00", 1, issuedWithDescendants)

	exactOnly := searchTransactionsFilterFields{LedgerID: 1, CategoryID: sql.NullInt64{Int64: 3, Valid: true}}
	if _, _, err := decodeSearchTransactionsCursor(cursor, exactOnly); err == nil {
		t.Fatal("expected an error decoding an include_descendants-issued cursor as an exact-match request")
	}
}
