package evaluation

import (
	"reflect"
	"testing"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
)

func TestSequenceStats(t *testing.T) {
	t.Parallel()
	velocity := fraudExp("", "fraud/velocity")
	cardTesting := fraudExp("", "fraud/card_testing")
	txs := []analyzedTransaction{
		analyzedTx(velocity, analyzedEvent("v1", "card_a", 0, events.DecisionApprove)),
		analyzedTx(velocity, analyzedEvent("v2", "card_a", 1, events.DecisionBlock, "velocity")),
		analyzedTx(velocity, analyzedEvent("v3", "card_b", 0, events.DecisionApprove)),
		analyzedTx(cardTesting, analyzedEvent("c1", "card_a", 5, events.DecisionReview, "card_testing")),
		analyzedTx(legitExp("l1"), analyzedEvent("l1", "card_c", 0, events.DecisionReview, "velocity")),
	}
	got := sequenceStats(txs)
	want := []SequenceStats{
		{Scenario: "fraud/card_testing", Total: 1, Detected: 1},
		{Scenario: "fraud/velocity", Total: 2, Detected: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sequenceStats = %+v, want %+v", got, want)
	}
	if r := sequenceRecall(got); r != 2.0/3 {
		t.Errorf("sequenceRecall(%+v) = %v, want %v", got, r, 2.0/3)
	}
}

func TestSequenceRecallOfNoSequencesIsZero(t *testing.T) {
	t.Parallel()
	if got := sequenceRecall(nil); got != 0 {
		t.Errorf("sequenceRecall(nil) = %v, want 0", got)
	}
}
