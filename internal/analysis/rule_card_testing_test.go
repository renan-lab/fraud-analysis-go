package analysis

import (
	"testing"
	"time"
)

func TestCardTestingRule(t *testing.T) {
	t.Parallel()
	rule := NewCardTestingRule(CardTestingParams{Window: 10 * time.Minute, SmallAmount: 1000, MinCount: 4, Score: 60, CashoutScore: 30})
	tests := []struct {
		name      string
		history   []CardEvent
		amount    int64
		wantScore int
	}{
		{name: "no history", amount: 300, wantScore: 0},
		{name: "three small charges", history: repeatEvents(3, 10*time.Second, 300, ChannelEcommerce), amount: 300, wantScore: 0},
		{name: "four small charges then small", history: repeatEvents(4, 10*time.Second, 300, ChannelEcommerce), amount: 300, wantScore: 60},
		{name: "four small charges then cashout", history: repeatEvents(4, 10*time.Second, 300, ChannelEcommerce), amount: 250000, wantScore: 90},
		{name: "small charges in person ignored", history: repeatEvents(6, 10*time.Second, 300, ChannelCardPresent), amount: 300, wantScore: 0},
		{name: "large online charges ignored", history: repeatEvents(6, 10*time.Second, 5000, ChannelEcommerce), amount: 300, wantScore: 0},
		{name: "small charges outside window", history: repeatEvents(6, 5*time.Minute, 300, ChannelEcommerce), amount: 300, wantScore: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tx := newTestTransaction()
			tx.Amount = tt.amount
			got := rule.Evaluate(tx, Features{CardHistory: tt.history})
			if got.Score != tt.wantScore {
				t.Errorf("Evaluate(amount %d, history %d) = %+v, want score %d", tt.amount, len(tt.history), got, tt.wantScore)
			}
		})
	}
}
