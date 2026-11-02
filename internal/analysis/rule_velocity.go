package analysis

import (
	"fmt"
	"time"
)

// VelocityParams configures VelocityRule: more than Max transactions on one
// card within Window (the current one included) adds Score.
type VelocityParams struct {
	Window time.Duration
	Max    int
	Score  int
}

// VelocityRule flags bursts of purchases on the same card.
type VelocityRule struct {
	p VelocityParams
}

// NewVelocityRule builds a VelocityRule.
//
//	analysis.NewVelocityRule(analysis.VelocityParams{Window: 3 * time.Minute, Max: 6, Score: 80})
func NewVelocityRule(p VelocityParams) VelocityRule {
	return VelocityRule{p: p}
}

// Name implements Rule.
func (VelocityRule) Name() string { return "velocity" }

// Evaluate implements Rule.
func (r VelocityRule) Evaluate(tx Transaction, f Features) RuleResult {
	count := 1 + len(eventsSince(f.CardHistory, tx.OccurredAt.Add(-r.p.Window)))
	if count <= r.p.Max {
		return RuleResult{Rule: r.Name()}
	}
	return RuleResult{
		Rule:   r.Name(),
		Score:  r.p.Score,
		Reason: fmt.Sprintf("%d transactions on card within %s (limit %d)", count, r.p.Window, r.p.Max),
	}
}

// eventsSince returns the events at or after from; history is oldest first.
func eventsSince(history []CardEvent, from time.Time) []CardEvent {
	for i, ev := range history {
		if !ev.At.Before(from) {
			return history[i:]
		}
	}
	return nil
}
