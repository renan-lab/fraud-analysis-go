package evaluation

import (
	"reflect"
	"testing"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
)

func TestFalsePositiveCauses(t *testing.T) {
	t.Parallel()
	travel := fraudExp("f1", "fraud/impossible_travel")
	txs := []analyzedTransaction{
		// card_a: a legit flagged before the fraud is clean, after it is not.
		analyzedTx(legitExp("a3"), analyzedEvent("a3", "card_a", 10, events.DecisionReview, "impossible_travel")),
		analyzedTx(travel, analyzedEvent("a2", "card_a", 5, events.DecisionBlock, "impossible_travel")),
		analyzedTx(legitExp("a1"), analyzedEvent("a1", "card_a", 0, events.DecisionReview, "velocity")),
		// card_b: clean card, two flagged legit with the same rules.
		analyzedTx(legitExp("b1"), analyzedEvent("b1", "card_b", 0, events.DecisionReview, "amount_outlier")),
		analyzedTx(legitExp("b2"), analyzedEvent("b2", "card_b", 1, events.DecisionBlock, "amount_outlier")),
		analyzedTx(legitExp("b3"), analyzedEvent("b3", "card_b", 2, events.DecisionApprove)),
	}
	want := []FalsePositiveCause{
		{Cause: CauseCleanCard, Rules: "amount_outlier", Count: 2},
		{Cause: CauseCleanCard, Rules: "velocity", Count: 1},
		{Cause: CauseFraudBeforeOnCard, Rules: "impossible_travel", Count: 1},
	}
	if got := falsePositiveCauses(txs); !reflect.DeepEqual(got, want) {
		t.Errorf("falsePositiveCauses = %+v, want %+v", got, want)
	}
}

func TestRuleSet(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   []string
		want string
	}{
		{in: nil, want: noRules},
		{in: []string{"velocity", "card_testing"}, want: "velocity+card_testing"},
	}
	for _, tt := range tests {
		if got := ruleSet(tt.in); got != tt.want {
			t.Errorf("ruleSet(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
