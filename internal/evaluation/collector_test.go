package evaluation

import (
	"reflect"
	"testing"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
)

func TestCollectorIgnoresOtherRunsAndCompletes(t *testing.T) {
	t.Parallel()
	c := NewCollector(newTruthSet(legitExp("t1"), legitExp("t2")))
	if c.Observe(analyzedEvent("old-run", "card", 0, events.DecisionApprove)) {
		t.Errorf("Observe(old-run) = true, want false for a transaction outside the truth")
	}
	c.Observe(analyzedEvent("t1", "card", 0, events.DecisionApprove))
	c.Observe(analyzedEvent("t1", "card", 0, events.DecisionApprove))
	if c.Complete() {
		t.Fatalf("Complete() = true with t2 missing, want false")
	}
	c.Observe(analyzedEvent("t2", "card", 1, events.DecisionReview, "velocity"))
	if !c.Complete() {
		t.Fatalf("Complete() = false after t1 and t2, want true")
	}
	if got := len(c.Results().Observations["t1"]); got != 2 {
		t.Errorf("observations of t1 = %d, want 2 (redelivery kept)", got)
	}
}

func TestToObservationKeepsRuleNames(t *testing.T) {
	t.Parallel()
	got := toObservation(analyzedEvent("t1", "card_a", 2, events.DecisionBlock, "velocity", "card_testing"))
	if got.CardID != "card_a" || got.Decision != events.DecisionBlock || got.Score != 80 {
		t.Errorf("toObservation = %+v, want card_a/block/80", got)
	}
	if want := []string{"velocity", "card_testing"}; !reflect.DeepEqual(got.Rules, want) {
		t.Errorf("toObservation rules = %v, want %v", got.Rules, want)
	}
}
