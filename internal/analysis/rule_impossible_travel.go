package analysis

import (
	"fmt"
	"math"
)

// TravelParams configures ImpossibleTravelRule: a distance of at least MinKM
// from the card's previous location at an implied speed above MaxKMH adds Score.
type TravelParams struct {
	MaxKMH float64
	MinKM  float64
	Score  int
}

// ImpossibleTravelRule flags a card used in two places too far apart for the
// time between them. MinKM ignores GPS jitter and nearby cities.
type ImpossibleTravelRule struct {
	p TravelParams
}

// NewImpossibleTravelRule builds an ImpossibleTravelRule.
//
//	analysis.NewImpossibleTravelRule(analysis.TravelParams{MaxKMH: 900, MinKM: 500, Score: 80})
func NewImpossibleTravelRule(p TravelParams) ImpossibleTravelRule {
	return ImpossibleTravelRule{p: p}
}

// Name implements Rule.
func (ImpossibleTravelRule) Name() string { return "impossible_travel" }

// Evaluate implements Rule. Without a location on tx or in the card history
// the rule cannot judge and stays silent.
func (r ImpossibleTravelRule) Evaluate(tx Transaction, f Features) RuleResult {
	prev, ok := lastLocated(f.CardHistory)
	if tx.Location == nil || !ok {
		return RuleResult{Rule: r.Name()}
	}
	km := haversineKM(*prev.Location, *tx.Location)
	elapsed := tx.OccurredAt.Sub(prev.At)
	kmh := math.Inf(1)
	if elapsed > 0 {
		kmh = km / elapsed.Hours()
	}
	if km < r.p.MinKM || kmh <= r.p.MaxKMH {
		return RuleResult{Rule: r.Name()}
	}
	return RuleResult{
		Rule:   r.Name(),
		Score:  r.p.Score,
		Reason: fmt.Sprintf("%.0f km from previous purchase in %s (%.0f km/h, limit %.0f)", km, elapsed, kmh, r.p.MaxKMH),
	}
}

// lastLocated returns the newest event that carries a location.
func lastLocated(history []CardEvent) (CardEvent, bool) {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Location != nil {
			return history[i], true
		}
	}
	return CardEvent{}, false
}
