package evaluation

import (
	"reflect"
	"testing"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
)

func TestPipelineStats(t *testing.T) {
	t.Parallel()
	truth := newTruthSet(legitExp("t1"), legitExp("t2"), fraudExp("t3", "fraud/velocity"), legitExp("t4"))
	truth.NotAccepted = 5
	approve := toObservation(analyzedEvent("x", "card", 0, events.DecisionApprove))
	review := toObservation(analyzedEvent("x", "card", 0, events.DecisionReview, "velocity"))
	r := Results{Truth: truth, Observations: map[string][]Observation{
		"t1": {approve},
		"t2": {approve, approve},
		"t4": {approve, review},
	}}
	got := pipelineStats(r)
	if got.Accepted != 4 || got.Analyzed != 3 || got.Missing != 1 || got.Redelivered != 2 || got.NotAccepted != 5 {
		t.Errorf("pipelineStats = %+v, want accepted 4, analyzed 3, missing 1, redelivered 2, not accepted 5", got)
	}
	if want := map[string]int{"fraud/velocity": 1}; !reflect.DeepEqual(got.MissingByScenario, want) {
		t.Errorf("MissingByScenario = %v, want %v", got.MissingByScenario, want)
	}
	if want := []string{"t4"}; !reflect.DeepEqual(got.DivergentIDs, want) {
		t.Errorf("DivergentIDs = %v, want %v", got.DivergentIDs, want)
	}
}

func TestConsistent(t *testing.T) {
	t.Parallel()
	a := Observation{Decision: events.DecisionReview, Score: 60}
	tests := []struct {
		name string
		obs  []Observation
		want bool
	}{
		{name: "single", obs: []Observation{a}, want: true},
		{name: "same decision and score", obs: []Observation{a, a}, want: true},
		{name: "different score", obs: []Observation{a, {Decision: events.DecisionReview, Score: 70}}, want: false},
		{name: "different decision", obs: []Observation{a, {Decision: events.DecisionBlock, Score: 60}}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := consistent(tt.obs); got != tt.want {
				t.Errorf("consistent(%+v) = %v, want %v", tt.obs, got, tt.want)
			}
		})
	}
}
