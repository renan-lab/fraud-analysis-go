package analysis

import "testing"

func TestThresholdsDecide(t *testing.T) {
	t.Parallel()
	th := Thresholds{Review: 50, Block: 80}
	tests := []struct {
		score int
		want  Decision
	}{
		{score: 0, want: DecisionApprove},
		{score: 49, want: DecisionApprove},
		{score: 50, want: DecisionReview},
		{score: 79, want: DecisionReview},
		{score: 80, want: DecisionBlock},
		{score: 500, want: DecisionBlock},
	}
	for _, tt := range tests {
		if got := th.Decide(tt.score); got != tt.want {
			t.Errorf("%+v.Decide(%d) = %s, want %s", th, tt.score, got, tt.want)
		}
	}
}

func TestThresholdsValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		th      Thresholds
		wantErr bool
	}{
		{th: Thresholds{Review: 50, Block: 80}},
		{th: Thresholds{Review: 50, Block: 50}},
		{th: Thresholds{Review: 0, Block: 80}, wantErr: true},
		{th: Thresholds{Review: 80, Block: 50}, wantErr: true},
	}
	for _, tt := range tests {
		if err := tt.th.Validate(); (err != nil) != tt.wantErr {
			t.Errorf("%+v.Validate() = %v, wantErr %v", tt.th, err, tt.wantErr)
		}
	}
}

func TestEvaluateRulesSumsOnlyFiredRules(t *testing.T) {
	t.Parallel()
	rules := []Rule{staticRule{name: "a", score: 30}, staticRule{name: "quiet", score: 0}, staticRule{name: "b", score: 25}}
	total, fired := evaluateRules(rules, newTestTransaction(), Features{})
	if total != 55 {
		t.Errorf("evaluateRules total = %d, want 55", total)
	}
	if len(fired) != 2 || fired[0].Rule != "a" || fired[1].Rule != "b" {
		t.Errorf("evaluateRules fired = %+v, want [a b] in rule order", fired)
	}
}

func TestEvaluateRulesWithNothingFiredReturnsEmptySlice(t *testing.T) {
	t.Parallel()
	_, fired := evaluateRules([]Rule{staticRule{name: "quiet"}}, newTestTransaction(), Features{})
	if fired == nil || len(fired) != 0 {
		t.Errorf("evaluateRules fired = %#v, want non-nil empty slice (rule_results is never null)", fired)
	}
}
