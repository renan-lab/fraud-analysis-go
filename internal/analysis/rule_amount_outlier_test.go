package analysis

import "testing"

func TestAmountOutlierRule(t *testing.T) {
	t.Parallel()
	rule := NewAmountOutlierRule(OutlierParams{Ratio: 10, MinSamples: 5, Score: 60})
	baseline := []int64{4000, 5000, 6000, 5000, 5000}
	tests := []struct {
		name      string
		amounts   []int64
		amount    int64
		wantScore int
	}{
		{name: "not enough samples", amounts: baseline[:4], amount: 500000, wantScore: 0},
		{name: "normal amount", amounts: baseline, amount: 7000, wantScore: 0},
		{name: "just below ratio", amounts: baseline, amount: 49999, wantScore: 0},
		{name: "exactly ratio", amounts: baseline, amount: 50000, wantScore: 60},
		{name: "far above", amounts: baseline, amount: 200000, wantScore: 60},
		{name: "zero median", amounts: []int64{0, 0, 0, 0, 0}, amount: 100, wantScore: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tx := newTestTransaction()
			tx.Amount = tt.amount
			got := rule.Evaluate(tx, Features{AccountAmounts: tt.amounts})
			if got.Score != tt.wantScore {
				t.Errorf("Evaluate(amount %d, baseline %v) = %+v, want score %d", tt.amount, tt.amounts, got, tt.wantScore)
			}
		})
	}
}
