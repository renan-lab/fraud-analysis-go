package events

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestTransactionAnalyzedWireFormat locks the JSON shape alerts depends on.
// A diff here is a contract change: bump the schema version or add an ADR.
func TestTransactionAnalyzedWireFormat(t *testing.T) {
	t.Parallel()
	ev := TransactionAnalyzed{
		EventID:       "3d2c1b0a-9f8e-4d7c-8b6a-5f4e3d2c1b0a",
		EventType:     EventTypeTransactionAnalyzed,
		SchemaVersion: TransactionAnalyzedSchemaVersion,
		AnalyzedAt:    time.Date(2026, 11, 2, 19, 15, 2, 0, time.UTC),
		RequestID:     "5f1d2c3b-4a59-4687-9a1b-2c3d4e5f6a7b",
		TransactionID: "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		AccountID:     "acc_3f9a1c2b7d4e",
		CardID:        "card_9b2c4d6e8f10",
		Amount:        12990,
		Currency:      "BRL",
		OccurredAt:    time.Date(2026, 11, 2, 19, 15, 0, 0, time.UTC),
		Decision:      DecisionReview,
		Score:         60,
		RuleResults:   []RuleResult{{Rule: "amount_outlier", Score: 60, Reason: "amount 12990 is 18.2x the account median 713 (limit 10.0x)"}},
	}
	got, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		t.Fatalf("marshal %+v: %v", ev, err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "transaction_analyzed.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
		t.Fatalf("wire format changed:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
