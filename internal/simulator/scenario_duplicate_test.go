package simulator

import (
	"bytes"
	"net/http"
	"testing"
)

func TestEmissionHistoryKeepsOnlyAcceptedAndWraps(t *testing.T) {
	t.Parallel()
	h := newEmissionHistory(2)
	h.record(Emission{Scenario: "rejected", ExpectedStatus: http.StatusBadRequest})
	for _, name := range []string{"a", "b", "c"} {
		h.record(Emission{Scenario: name, ExpectedStatus: http.StatusAccepted})
	}
	var got []string
	for _, e := range h.items {
		got = append(got, e.Scenario)
	}
	if len(got) != 2 || got[0] != "c" || got[1] != "b" {
		t.Errorf("history after a,b,c with size 2 = %v, want [c b]", got)
	}
}

func TestDuplicateScenariosReplayHistory(t *testing.T) {
	t.Parallel()
	pop := newTestPopulation(t, 20, 10)
	r := newTestRand(21)
	original, err := acceptedEmission(ModeFraud, "fraud/x", LabelFraud, pop.legitPurchase(r, pop.randomCustomer(r), testNow))
	if err != nil {
		t.Fatalf("acceptedEmission() error = %v", err)
	}
	history := newEmissionHistory(1)
	history.record(original)
	for _, s := range duplicateScenarios(pop, history) {
		emissions, err := s.Next(r, testNow)
		if err != nil || len(emissions) != 1 {
			t.Fatalf("%s.Next() = %d emissions, %v; want 1, nil", s.Name(), len(emissions), err)
		}
		assertDuplicate(t, s.Name(), original, emissions[0])
	}
}

func assertDuplicate(t *testing.T, name string, original, dup Emission) {
	t.Helper()
	if dup.Mode != ModeDuplicate || dup.Label != original.Label {
		t.Errorf("%s: mode %q label %q, want duplicate and original label %q", name, dup.Mode, dup.Label, original.Label)
	}
	switch name {
	case "duplicate/exact_replay":
		if !bytes.Equal(dup.Body, original.Body) || dup.copies() != 1 {
			t.Errorf("%s: body differs or copies=%d, want identical single send", name, dup.copies())
		}
	case "duplicate/concurrent_retry":
		if !bytes.Equal(dup.Body, original.Body) || dup.copies() < 3 {
			t.Errorf("%s: copies=%d, want identical body sent >= 3 times", name, dup.copies())
		}
	case "duplicate/same_content_new_id":
		got, want := decodeTransaction(t, dup), decodeTransaction(t, original)
		if got.TransactionID == want.TransactionID || got.Amount != want.Amount || got.CardID != want.CardID {
			t.Errorf("%s: got %+v, want same purchase as %+v with a new id", name, got, want)
		}
	}
}

func TestDuplicateWithEmptyHistoryEmitsOriginalFirst(t *testing.T) {
	t.Parallel()
	pop := newTestPopulation(t, 22, 10)
	s := duplicateScenarios(pop, newEmissionHistory(4))[0]
	emissions, err := s.Next(newTestRand(23), testNow)
	if err != nil || len(emissions) != 2 {
		t.Fatalf("%s.Next() = %d emissions, %v; want original + replay", s.Name(), len(emissions), err)
	}
	if emissions[0].Mode != ModeLegit || !bytes.Equal(emissions[0].Body, emissions[1].Body) {
		t.Errorf("first emission mode %q, want legit original replayed by the second", emissions[0].Mode)
	}
}
