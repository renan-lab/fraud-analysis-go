package analysis

import (
	"testing"
	"time"
)

func TestVelocityRule(t *testing.T) {
	t.Parallel()
	rule := NewVelocityRule(VelocityParams{Window: 3 * time.Minute, Max: 6, Score: 80})
	tests := []struct {
		name      string
		history   []CardEvent
		wantScore int
	}{
		{name: "no history", history: nil, wantScore: 0},
		{name: "at limit counting current", history: repeatEvents(5, 10*time.Second, 5000, ChannelCardPresent), wantScore: 0},
		{name: "above limit", history: repeatEvents(6, 10*time.Second, 5000, ChannelCardPresent), wantScore: 80},
		{name: "burst outside window", history: repeatEvents(10, 30*time.Minute, 5000, ChannelCardPresent), wantScore: 0},
		{name: "event exactly at window start counts", history: append(
			[]CardEvent{cardEvent(3*time.Minute, 1, ChannelEcommerce, nil)},
			repeatEvents(5, 10*time.Second, 5000, ChannelCardPresent)...), wantScore: 80},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := rule.Evaluate(newTestTransaction(), Features{CardHistory: tt.history})
			if got.Score != tt.wantScore {
				t.Errorf("Evaluate(history of %d) = %+v, want score %d", len(tt.history), got, tt.wantScore)
			}
			if got.Rule != "velocity" || got.Fired() != (got.Reason != "") {
				t.Errorf("Evaluate = %+v, want rule velocity with reason only when fired", got)
			}
		})
	}
}

func TestEventsSince(t *testing.T) {
	t.Parallel()
	history := repeatEvents(4, time.Minute, 1, ChannelCardPresent)
	tests := []struct {
		from time.Time
		want int
	}{
		{from: testNow.Add(-10 * time.Minute), want: 4},
		{from: testNow.Add(-2 * time.Minute), want: 2},
		{from: testNow, want: 0},
	}
	for _, tt := range tests {
		if got := len(eventsSince(history, tt.from)); got != tt.want {
			t.Errorf("eventsSince(from %s) = %d events, want %d", tt.from, got, tt.want)
		}
	}
}
