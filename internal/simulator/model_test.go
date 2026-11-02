package simulator

import (
	"net/http"
	"testing"
)

func TestNewUUIDIsVersion4AndUnique(t *testing.T) {
	t.Parallel()
	r := newTestRand(1)
	seen := map[string]bool{}
	for range 1000 {
		id := newUUID(r)
		if !uuidV4Pattern.MatchString(id) || seen[id] {
			t.Fatalf("newUUID() = %q, want unique UUID v4", id)
		}
		seen[id] = true
	}
}

func TestNewIDHasPrefixAndPattern(t *testing.T) {
	t.Parallel()
	got := newID(newTestRand(1), "acc")
	if len(got) != len("acc_")+12 || got[:4] != "acc_" || !idPattern.MatchString(got) {
		t.Errorf("newID(acc) = %q, want acc_ followed by 12 hex chars", got)
	}
}

func TestAcceptedEmissionCarriesTransaction(t *testing.T) {
	t.Parallel()
	tx := Transaction{TransactionID: "id-1", Amount: 100, OccurredAt: testNow}
	e, err := acceptedEmission(ModeFraud, "fraud/x", LabelFraud, tx)
	if err != nil {
		t.Fatalf("acceptedEmission() error = %v", err)
	}
	if e.TransactionID != "id-1" || e.ExpectedStatus != http.StatusAccepted || e.ContentType != contentTypeJSON || e.Label != LabelFraud {
		t.Errorf("acceptedEmission() = %+v, want id-1, 202, JSON, fraud", e)
	}
	if got := decodeTransaction(t, e); got.Amount != 100 {
		t.Errorf("decoded amount = %d, want 100", got.Amount)
	}
}

func TestEmissionCopiesDefaultsToOne(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ copies, want int }{{0, 1}, {1, 1}, {4, 4}} {
		if got := (Emission{Copies: tt.copies}).copies(); got != tt.want {
			t.Errorf("Emission{Copies: %d}.copies() = %d, want %d", tt.copies, got, tt.want)
		}
	}
}
