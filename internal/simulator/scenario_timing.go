package simulator

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"
)

// timingScenarios shifts future timestamps from wall time, because the API's
// skew check compares against its own clock, not the simulated one.
func timingScenarios(pop *Population, wall Clock) []Scenario {
	fromWall := func(time.Time) time.Time { return wall.Now() }
	fromSim := func(now time.Time) time.Time { return now }
	return []Scenario{
		scenarioSpec{
			name:        "timing/out_of_order",
			mode:        ModeTiming,
			description: "two purchases of the same card sent newest first; expects 202 for both",
			next:        pop.outOfOrder,
		},
		shiftedSpec(pop, "timing/future", "occurred_at 1-24h ahead of wall time; expects 400", fromWall,
			func(r *rand.Rand) time.Duration { return time.Duration(1+r.IntN(24)) * time.Hour }, http.StatusBadRequest),
		shiftedSpec(pop, "timing/within_skew", "occurred_at 1-3 min ahead of wall time, inside the 5 min skew; expects 202", fromWall,
			func(r *rand.Rand) time.Duration { return time.Duration(1+r.IntN(3)) * time.Minute }, http.StatusAccepted),
		shiftedSpec(pop, "timing/stale", "occurred_at 7-60 days ago (late arrival); expects 202", fromSim,
			func(r *rand.Rand) time.Duration { return -time.Duration(7+r.IntN(54)) * 24 * time.Hour }, http.StatusAccepted),
	}
}

func (p *Population) outOfOrder(r *rand.Rand, now time.Time) ([]Emission, error) {
	c := p.randomCustomer(r)
	earlier := p.legitPurchase(r, c, now.Add(-time.Duration(1+r.IntN(5))*time.Minute))
	later := p.legitPurchase(r, c, now)
	return encodeSequence(ModeTiming, "timing/out_of_order", []labeledTx{
		{tx: later, label: LabelLegit},
		{tx: earlier, label: LabelLegit},
	})
}

// shiftedSpec sends a legit purchase with occurred_at moved by shift(r).
func shiftedSpec(pop *Population, name, description string, base func(time.Time) time.Time, shift func(*rand.Rand) time.Duration, status int) Scenario {
	return scenarioSpec{
		name:        name,
		mode:        ModeTiming,
		description: description,
		next: func(r *rand.Rand, now time.Time) ([]Emission, error) {
			tx := pop.legitPurchase(r, pop.randomCustomer(r), base(now).Add(shift(r)))
			if status == http.StatusAccepted {
				return single(acceptedEmission(ModeTiming, name, LabelLegit, tx))
			}
			body, err := json.Marshal(tx)
			if err != nil {
				return nil, fmt.Errorf("encode transaction %s for %s: %w", tx.TransactionID, name, err)
			}
			return []Emission{rejectedEmission(ModeTiming, name, tx.TransactionID, status, body)}, nil
		},
	}
}
