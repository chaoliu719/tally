package confirm_test

import (
	"testing"
	"time"

	"tally/internal/confirm"
)

const batchAction = "batch_delete_transactions"

func batchItems() []confirm.Item {
	return []confirm.Item{
		{ID: "10", Revision: "rev-10"},
		{ID: "11", Revision: "rev-11"},
		{ID: "12", Revision: "rev-12"},
	}
}

func TestIssueBatchVerifyBatchRoundTrip(t *testing.T) {
	items := batchItems()
	token, expiresAt := confirm.IssueBatch(secret, batchAction, items)
	if token == "" {
		t.Fatal("expected a non-empty token")
	}
	if expiresAt <= time.Now().Unix() {
		t.Fatalf("expiresAt = %d, want in the future", expiresAt)
	}

	if err := confirm.VerifyBatch(secret, token, batchAction, items, time.Now()); err != nil {
		t.Fatalf("VerifyBatch failed on a freshly issued token: %v", err)
	}
}

func TestVerifyBatchRejectsExpiredToken(t *testing.T) {
	items := batchItems()
	token, _ := confirm.IssueBatch(secret, batchAction, items)

	future := time.Now().Add(confirm.TTL + time.Minute)
	if err := confirm.VerifyBatch(secret, token, batchAction, items, future); err == nil {
		t.Fatal("expected an error for an expired token")
	}
}

func TestVerifyBatchRejectsWrongAction(t *testing.T) {
	items := batchItems()
	token, _ := confirm.IssueBatch(secret, batchAction, items)

	if err := confirm.VerifyBatch(secret, token, "batch_delete_categories", items, time.Now()); err == nil {
		t.Fatal("expected an error for a mismatched action")
	}
}

func TestVerifyBatchRejectsTamperedSignature(t *testing.T) {
	items := batchItems()
	token, _ := confirm.IssueBatch(secret, batchAction, items)

	tampered := tamperSignature(t, token)

	if err := confirm.VerifyBatch(secret, tampered, batchAction, items, time.Now()); err == nil {
		t.Fatal("expected an error for a tampered signature")
	}
}

func TestVerifyBatchRejectsExtraID(t *testing.T) {
	items := batchItems()
	token, _ := confirm.IssueBatch(secret, batchAction, items)

	withExtra := append(append([]confirm.Item{}, items...), confirm.Item{ID: "999", Revision: "rev-999"})
	if err := confirm.VerifyBatch(secret, token, batchAction, withExtra, time.Now()); err == nil {
		t.Fatal("expected an error when confirming with an id not in the token")
	}
}

func TestVerifyBatchRejectsMissingID(t *testing.T) {
	items := batchItems()
	token, _ := confirm.IssueBatch(secret, batchAction, items)

	fewer := items[:len(items)-1]
	if err := confirm.VerifyBatch(secret, token, batchAction, fewer, time.Now()); err == nil {
		t.Fatal("expected an error when confirming with fewer ids than the token")
	}
}

// TestVerifyBatchIgnoresPerItemRevisionMismatch verifies that VerifyBatch
// itself does not reject on a per-item revision mismatch -- that check is
// the caller's responsibility (see design.md), so best-effort deletion can
// report each item individually instead of failing the whole batch.
func TestVerifyBatchIgnoresPerItemRevisionMismatch(t *testing.T) {
	items := batchItems()
	token, _ := confirm.IssueBatch(secret, batchAction, items)

	changed := append([]confirm.Item{}, items...)
	changed[0].Revision = "some-other-revision"

	if err := confirm.VerifyBatch(secret, token, batchAction, changed, time.Now()); err != nil {
		t.Fatalf("VerifyBatch should ignore per-item revision mismatches, got: %v", err)
	}
}

func TestBatchItemsReturnsIssuedItems(t *testing.T) {
	items := batchItems()
	token, _ := confirm.IssueBatch(secret, batchAction, items)

	got, err := confirm.BatchItems(token)
	if err != nil {
		t.Fatalf("BatchItems failed: %v", err)
	}
	if len(got) != len(items) {
		t.Fatalf("BatchItems returned %d items, want %d", len(got), len(items))
	}
	byID := make(map[string]string, len(got))
	for _, item := range got {
		byID[item.ID] = item.Revision
	}
	for _, want := range items {
		if byID[want.ID] != want.Revision {
			t.Fatalf("BatchItems: id %q revision = %q, want %q", want.ID, byID[want.ID], want.Revision)
		}
	}
}
