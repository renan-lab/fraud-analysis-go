package evaluation

import "github.com/renan-lab/fraud-analysis-go/internal/events"

// Collector keeps the decisions of the transactions in a TruthSet. It is not
// safe for concurrent use: TopicReader.Read calls the handler sequentially.
type Collector struct {
	truth        TruthSet
	observations map[string][]Observation
}

// NewCollector expects the transactions of truth.
//
//	c := evaluation.NewCollector(truth)
func NewCollector(truth TruthSet) *Collector {
	return &Collector{truth: truth, observations: map[string][]Observation{}}
}

// Observe records ev and reports whether it belongs to the truth. Events of
// other runs share the topic and are ignored.
func (c *Collector) Observe(ev events.TransactionAnalyzed) bool {
	if _, ok := c.truth.Expectations[ev.TransactionID]; !ok {
		return false
	}
	c.observations[ev.TransactionID] = append(c.observations[ev.TransactionID], toObservation(ev))
	return true
}

// Complete reports whether every expected transaction has a decision.
func (c *Collector) Complete() bool {
	return len(c.observations) == len(c.truth.Expectations)
}

// Results returns what was collected so far.
func (c *Collector) Results() Results {
	return Results{Truth: c.truth, Observations: c.observations}
}

func toObservation(ev events.TransactionAnalyzed) Observation {
	rules := make([]string, 0, len(ev.RuleResults))
	for _, r := range ev.RuleResults {
		rules = append(rules, r.Rule)
	}
	return Observation{CardID: ev.CardID, OccurredAt: ev.OccurredAt, Decision: ev.Decision, Score: ev.Score, Rules: rules}
}
