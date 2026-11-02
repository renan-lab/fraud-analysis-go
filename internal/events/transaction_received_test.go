package events

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestTransactionReceivedWireFormat locks the JSON shape consumers depend on.
// A diff here is a contract change: bump the schema version or add an ADR.
func TestTransactionReceivedWireFormat(t *testing.T) {
	t.Parallel()
	ev := TransactionReceived{
		EventID:       "0b6f3c1e-2d4a-4f5b-8c7d-9e0f1a2b3c4d",
		EventType:     EventTypeTransactionReceived,
		SchemaVersion: TransactionReceivedSchemaVersion,
		ReceivedAt:    time.Date(2026, 11, 2, 19, 15, 1, 0, time.UTC),
		RequestID:     "5f1d2c3b-4a59-4687-9a1b-2c3d4e5f6a7b",
		Transaction: Transaction{
			TransactionID: "7c9e6679-7425-40de-944b-e07fc1f90ae7",
			AccountID:     "acc_3f9a1c2b7d4e",
			CardID:        "card_9b2c4d6e8f10",
			Amount:        12990,
			Currency:      "BRL",
			Merchant:      Merchant{ID: "mer_51a2b3c4d5e6", Name: "Grocery 5e6f", MCC: "5411", Country: "BR"},
			Channel:       "card_present",
			DeviceID:      "dev_0a1b2c3d4e5f",
			IP:            "177.34.10.200",
			OccurredAt:    time.Date(2026, 11, 2, 19, 15, 0, 0, time.UTC),
			Location:      &Location{Lat: -23.5505, Lon: -46.6333},
		},
	}
	got, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		t.Fatalf("marshal %+v: %v", ev, err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "transaction_received.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
		t.Fatalf("wire format changed:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestTransactionOmitsMissingLocation keeps location optional on the wire:
// consumers must see no key at all, never {"lat":0,"lon":0}.
func TestTransactionOmitsMissingLocation(t *testing.T) {
	t.Parallel()
	got, err := json.Marshal(Transaction{TransactionID: "7c9e6679-7425-40de-944b-e07fc1f90ae7"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if bytes.Contains(got, []byte(`"location"`)) {
		t.Fatalf("marshal(no location) = %s, want no location key", got)
	}
}
