package ingestion

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func decodeRequest(t *testing.T, payload map[string]any) transactionRequest {
	t.Helper()
	var req transactionRequest
	if err := json.Unmarshal([]byte(mustJSON(t, payload)), &req); err != nil {
		t.Fatalf("unmarshal %v: %v", payload, err)
	}
	return req
}

func TestToTransactionAcceptsContractExample(t *testing.T) {
	t.Parallel()
	tx, errs := decodeRequest(t, validPayload()).toTransaction(testNow)
	if len(errs) > 0 {
		t.Fatalf("toTransaction(valid) errors = %+v, want none", errs)
	}
	want := Transaction{
		TransactionID: "7c9e6679-7425-40de-944b-e07fc1f90ae7", AccountID: "acc_3f9a1c2b7d4e", CardID: "card_9b2c4d6e8f10",
		Amount: 12990, Currency: "BRL", Merchant: Merchant{ID: "mer_51a2b3c4d5e6", Name: "Grocery 5e6f", MCC: "5411", Country: "BR"},
		Channel: ChannelCardPresent, DeviceID: "dev_0a1b2c3d4e5f", IP: "177.34.10.200", OccurredAt: testNow.Add(-5 * time.Minute),
	}
	if tx != want {
		t.Fatalf("toTransaction(valid) = %+v, want %+v", tx, want)
	}
}

func TestToTransactionRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		field string
		value any
		ok    bool
	}{
		{"uppercase uuid", "transaction_id", "7C9E6679-7425-40DE-944B-E07FC1F90AE7", false},
		{"uuid v1", "transaction_id", "7c9e6679-7425-10de-944b-e07fc1f90ae7", false},
		{"sql injection id", "account_id", "acc_1' OR '1'='1", false},
		{"id too long", "card_id", strings.Repeat("a", 65), false},
		{"id max length", "device_id", strings.Repeat("a", 64), true},
		{"zero amount", "amount", 0, false},
		{"negative amount", "amount", -10, false},
		{"lowercase currency", "currency", "brl", false},
		{"long currency", "currency", "REAL", false},
		{"unknown channel", "channel", "telepathy", false},
		{"ecommerce", "channel", "ecommerce", true},
		{"ipv6", "ip", "2804:14c::1", true},
		{"hostname ip", "ip", "example.com", false},
		{"non rfc3339", "occurred_at", "02/11/2026 19:15", false},
		{"offset timestamp", "occurred_at", "2026-11-02T16:15:00-03:00", true},
		{"within skew", "occurred_at", "2026-11-02T19:19:59Z", true},
		{"beyond skew", "occurred_at", "2026-11-02T19:20:01Z", false},
		{"stale", "occurred_at", "2026-09-01T00:00:00Z", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := validPayload()
			p[tt.field] = tt.value
			_, errs := decodeRequest(t, p).toTransaction(testNow)
			if tt.ok != (len(errs) == 0) {
				t.Fatalf("%s=%v: errors = %+v, want ok=%v", tt.field, tt.value, errs, tt.ok)
			}
			if !tt.ok && errs[0].Field != tt.field {
				t.Fatalf("%s=%v: error field = %q, want %q", tt.field, tt.value, errs[0].Field, tt.field)
			}
		})
	}
}

func TestToTransactionMerchantRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		field string
		value string
		ok    bool
	}{
		{"xss id", "id", "<script>alert(1)</script>", false},
		{"control chars name", "name", "Shop\x00\x1b[31m", false},
		{"rtl override name", "name", "Shop\u202egnp.exe", false},
		{"empty name", "name", "", false},
		{"name too long", "name", strings.Repeat("A", 129), false},
		{"accented name", "name", "Padaria São João", true},
		{"command injection mcc", "mcc", "5411; rm -rf /", false},
		{"lowercase country", "country", "br", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := validPayload()
			p["merchant"].(map[string]any)[tt.field] = tt.value
			_, errs := decodeRequest(t, p).toTransaction(testNow)
			if tt.ok != (len(errs) == 0) {
				t.Fatalf("merchant.%s=%q: errors = %+v, want ok=%v", tt.field, tt.value, errs, tt.ok)
			}
			if !tt.ok && errs[0].Field != "merchant."+tt.field {
				t.Fatalf("merchant.%s: error field = %q", tt.field, errs[0].Field)
			}
		})
	}
}

func TestToTransactionReportsEveryMissingField(t *testing.T) {
	t.Parallel()
	_, errs := transactionRequest{}.toTransaction(testNow)
	if len(errs) != 10 {
		t.Fatalf("empty request: %d errors %+v, want 10 (one per required field)", len(errs), errs)
	}
	for _, e := range errs {
		if e.Detail != "is required" {
			t.Errorf("field %s detail = %q, want %q", e.Field, e.Detail, "is required")
		}
	}
}

func TestEchoTruncatesLongValues(t *testing.T) {
	t.Parallel()
	got := echo(strings.Repeat("é", 100))
	if want := strings.Repeat("é", maxEchoedRunes) + "..."; got != want {
		t.Fatalf("echo(100 runes) = %q, want %q", got, want)
	}
	if got := echo("short"); got != "short" {
		t.Fatalf("echo(short) = %q, want unchanged", got)
	}
}
