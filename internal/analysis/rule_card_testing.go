package analysis

import (
	"fmt"
	"time"
)

// CardTestingParams configures CardTestingRule: at least MinCount ecommerce
// charges <= SmallAmount within Window add Score, plus CashoutScore when the
// current charge is larger than SmallAmount.
type CardTestingParams struct {
	Window       time.Duration
	SmallAmount  int64
	MinCount     int
	Score        int
	CashoutScore int
}

// CardTestingRule flags a run of tiny online charges that validate a stolen
// card, usually followed by a large purchase.
type CardTestingRule struct {
	p CardTestingParams
}

// NewCardTestingRule builds a CardTestingRule.
//
//	analysis.NewCardTestingRule(analysis.CardTestingParams{Window: 10 * time.Minute, SmallAmount: 1000, MinCount: 4, Score: 60, CashoutScore: 30})
func NewCardTestingRule(p CardTestingParams) CardTestingRule {
	return CardTestingRule{p: p}
}

// Name implements Rule.
func (CardTestingRule) Name() string { return "card_testing" }

// Evaluate implements Rule.
func (r CardTestingRule) Evaluate(tx Transaction, f Features) RuleResult {
	small := r.countSmallOnline(eventsSince(f.CardHistory, tx.OccurredAt.Add(-r.p.Window)))
	if small < r.p.MinCount {
		return RuleResult{Rule: r.Name()}
	}
	res := RuleResult{
		Rule:   r.Name(),
		Score:  r.p.Score,
		Reason: fmt.Sprintf("%d ecommerce charges <= %d within %s (limit %d)", small, r.p.SmallAmount, r.p.Window, r.p.MinCount),
	}
	if tx.Amount > r.p.SmallAmount {
		res.Score += r.p.CashoutScore
		res.Reason += fmt.Sprintf(", followed by %d", tx.Amount)
	}
	return res
}

func (r CardTestingRule) countSmallOnline(history []CardEvent) int {
	n := 0
	for _, ev := range history {
		if ev.Channel == ChannelEcommerce && ev.Amount <= r.p.SmallAmount {
			n++
		}
	}
	return n
}
