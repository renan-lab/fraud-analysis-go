package analysis

import (
	"testing"
	"time"
)

func TestImpossibleTravelRule(t *testing.T) {
	t.Parallel()
	rule := NewImpossibleTravelRule(TravelParams{MaxKMH: 900, MinKM: 500, Score: 80})
	tests := []struct {
		name      string
		history   []CardEvent
		current   *Location
		wantScore int
	}{
		{name: "no history", current: &saoPaulo, wantScore: 0},
		{name: "current without location", history: []CardEvent{cardEvent(20*time.Minute, 1, ChannelCardPresent, &lisbon)}, current: nil, wantScore: 0},
		{name: "history without location", history: []CardEvent{cardEvent(20*time.Minute, 1, ChannelEcommerce, nil)}, current: &saoPaulo, wantScore: 0},
		{name: "lisbon 20 minutes ago", history: []CardEvent{cardEvent(20*time.Minute, 1, ChannelCardPresent, &lisbon)}, current: &saoPaulo, wantScore: 80},
		{name: "lisbon 12 hours ago is a flight", history: []CardEvent{cardEvent(12*time.Hour, 1, ChannelCardPresent, &lisbon)}, current: &saoPaulo, wantScore: 0},
		{name: "campinas 5 minutes ago is below min distance", history: []CardEvent{cardEvent(5*time.Minute, 1, ChannelCardPresent, &campinas)}, current: &saoPaulo, wantScore: 0},
		{name: "same instant far away", history: []CardEvent{cardEvent(0, 1, ChannelCardPresent, &lisbon)}, current: &saoPaulo, wantScore: 80},
		{name: "newest located event wins", history: []CardEvent{
			cardEvent(30*time.Minute, 1, ChannelCardPresent, &lisbon),
			cardEvent(10*time.Minute, 1, ChannelCardPresent, &campinas),
			cardEvent(5*time.Minute, 1, ChannelEcommerce, nil),
		}, current: &saoPaulo, wantScore: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tx := newTestTransaction()
			tx.Location = tt.current
			got := rule.Evaluate(tx, Features{CardHistory: tt.history})
			if got.Score != tt.wantScore {
				t.Errorf("Evaluate(%s) = %+v, want score %d", tt.name, got, tt.wantScore)
			}
		})
	}
}
