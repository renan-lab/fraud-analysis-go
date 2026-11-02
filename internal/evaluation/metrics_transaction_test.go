package evaluation

import (
	"reflect"
	"testing"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/groundtruth"
)

func TestTransactionStats(t *testing.T) {
	t.Parallel()
	velocity := fraudExp("f", "fraud/velocity")
	txs := []analyzedTransaction{
		analyzedTx(velocity, analyzedEvent("f1", "c", 0, events.DecisionBlock, "velocity")),
		analyzedTx(velocity, analyzedEvent("f2", "c", 1, events.DecisionReview, "velocity")),
		analyzedTx(velocity, analyzedEvent("f3", "c", 2, events.DecisionApprove)),
		analyzedTx(legitExp("l1"), analyzedEvent("l1", "c", 3, events.DecisionReview, "amount_outlier")),
		analyzedTx(legitExp("l2"), analyzedEvent("l2", "c", 4, events.DecisionApprove)),
		analyzedTx(legitExp("l3"), analyzedEvent("l3", "c", 5, events.DecisionApprove)),
		analyzedTx(legitExp("l4"), analyzedEvent("l4", "c", 6, events.DecisionApprove)),
		analyzedTx(Expectation{Label: groundtruth.LabelReject}, analyzedEvent("r", "c", 7, events.DecisionBlock)),
	}
	got := transactionStats(txs)
	want := TransactionStats{TruePositive: 2, FalseNegative: 1, FalsePositive: 1, TrueNegative: 3,
		Precision: 2.0 / 3, Recall: 2.0 / 3, FalsePositiveRate: 0.25}
	if got != want {
		t.Errorf("transactionStats = %+v, want %+v", got, want)
	}
}

func TestScenarioDecisions(t *testing.T) {
	t.Parallel()
	txs := []analyzedTransaction{
		analyzedTx(legitExp("l1"), analyzedEvent("l1", "c", 0, events.DecisionApprove)),
		analyzedTx(fraudExp("f1", "fraud/velocity"), analyzedEvent("f1", "c", 1, events.DecisionBlock)),
		analyzedTx(fraudExp("f2", "fraud/velocity"), analyzedEvent("f2", "c", 2, events.DecisionReview)),
		analyzedTx(Expectation{Scenario: "fraud/impossible_travel", Label: groundtruth.LabelLegit}, analyzedEvent("h", "c", 3, events.DecisionApprove)),
		analyzedTx(fraudExp("a", "fraud/impossible_travel"), analyzedEvent("a", "c", 4, events.DecisionBlock)),
	}
	want := []ScenarioDecisions{
		{Scenario: "fraud/impossible_travel", Label: groundtruth.LabelFraud, Block: 1},
		{Scenario: "fraud/impossible_travel", Label: groundtruth.LabelLegit, Approve: 1},
		{Scenario: "fraud/velocity", Label: groundtruth.LabelFraud, Review: 1, Block: 1},
		{Scenario: "legit/purchase", Label: groundtruth.LabelLegit, Approve: 1},
	}
	if got := scenarioDecisions(txs); !reflect.DeepEqual(got, want) {
		t.Errorf("scenarioDecisions = %+v, want %+v", got, want)
	}
}

func TestRatio(t *testing.T) {
	t.Parallel()
	tests := []struct {
		num, den int
		want     float64
	}{
		{num: 1, den: 4, want: 0.25},
		{num: 0, den: 0, want: 0},
	}
	for _, tt := range tests {
		if got := ratio(tt.num, tt.den); got != tt.want {
			t.Errorf("ratio(%d, %d) = %v, want %v", tt.num, tt.den, got, tt.want)
		}
	}
}
