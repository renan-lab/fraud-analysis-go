package evaluation

import (
	"reflect"
	"testing"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
)

func TestRuleFirings(t *testing.T) {
	t.Parallel()
	txs := []analyzedTransaction{
		analyzedTx(fraudExp("f1", "fraud/velocity"), analyzedEvent("f1", "c", 0, events.DecisionBlock, "velocity", "card_testing")),
		analyzedTx(legitExp("l1"), analyzedEvent("l1", "c", 1, events.DecisionReview, "velocity")),
		analyzedTx(legitExp("l2"), analyzedEvent("l2", "c", 2, events.DecisionApprove)),
	}
	want := []RuleFirings{
		{Rule: "card_testing", Fraud: 1},
		{Rule: "velocity", Fraud: 1, Legit: 1},
	}
	if got := ruleFirings(txs); !reflect.DeepEqual(got, want) {
		t.Errorf("ruleFirings = %+v, want %+v", got, want)
	}
}
