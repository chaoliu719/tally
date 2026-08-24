package tools

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// searchTransactionsFilterFields is hashed to produce the filter_fingerprint
// embedded in a search_transactions cursor (see design.md's "cursor 编码:
// base64url(JSON)"). It captures exactly the filter parameters that affect
// which rows are eligible for keyset pagination -- ledger_id/source_id/
// category_id/start_time/end_time/keyword -- so a cursor issued under one
// set of filters is rejected if replayed against a different set (see the
// spec's "cursor 无效或已不匹配当前筛选条件" scenario). Keyword stores the
// trimmed, canonical keyword value; empty means "not provided", matching how
// searchTransactions treats a blank keyword (see add-transaction-keyword-
// search's design.md).
type searchTransactionsFilterFields struct {
	LedgerID   int64
	SourceID   sql.NullInt64
	CategoryID sql.NullInt64
	StartTime  sql.NullInt64
	EndTime    sql.NullInt64
	Keyword    string
}

func searchTransactionsFilterFingerprint(f searchTransactionsFilterFields) string {
	// Marshaling a fixed struct of plain nullable ints never fails.
	body, _ := json.Marshal(f)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// searchTransactionsCursor is the decoded content of an opaque
// search_transactions pagination cursor: the keyset position `(last_time,
// last_id)` of the last row returned on the previous page, plus a
// fingerprint of the filters that page was issued under. Per design.md's
// Non-Goals, it is intentionally not signed -- it is a correctness token,
// not an authorization token, so a forged or corrupted cursor can at worst
// produce a wrong or rejected query, never a permission bypass or data
// change.
type searchTransactionsCursor struct {
	LastTime    int64  `json:"last_time"`
	LastID      int64  `json:"last_id"`
	Fingerprint string `json:"filter_fingerprint"`
}

// encodeSearchTransactionsCursor builds the opaque cursor string for the
// last row of a page, to be returned as next_cursor.
func encodeSearchTransactionsCursor(lastTime, lastID int64, filter searchTransactionsFilterFields) string {
	c := searchTransactionsCursor{
		LastTime:    lastTime,
		LastID:      lastID,
		Fingerprint: searchTransactionsFilterFingerprint(filter),
	}
	// Marshaling a fixed struct of plain ints/strings never fails.
	body, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(body)
}

// decodeSearchTransactionsCursor decodes cursor and checks that it was
// issued under the same filters as the current request (filter), returning
// the keyset position to resume from. It returns an error if the cursor
// cannot be parsed, or if its fingerprint doesn't match the current filters.
func decodeSearchTransactionsCursor(cursor string, filter searchTransactionsFilterFields) (lastTime, lastID int64, err error) {
	body, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid cursor")
	}
	var c searchTransactionsCursor
	if err := json.Unmarshal(body, &c); err != nil {
		return 0, 0, fmt.Errorf("invalid cursor")
	}
	if c.Fingerprint == "" || c.Fingerprint != searchTransactionsFilterFingerprint(filter) {
		return 0, 0, fmt.Errorf("cursor does not match the current filters; fetch a fresh first page")
	}
	return c.LastTime, c.LastID, nil
}
