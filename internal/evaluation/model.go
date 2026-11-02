// Package evaluation measures the analysis pipeline against the simulator's
// ground truth: it joins a truth file (internal/groundtruth) with the
// decisions published on transactions.analyzed and reports coverage,
// idempotency and detection quality.
package evaluation

import (
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/groundtruth"
)

// Expectation is the ground truth of one accepted transaction.
type Expectation struct {
	TransactionID string
	Scenario      string
	Label         groundtruth.Label
}

// TruthSet is a truth file reduced to accepted transactions, deduplicated by
// transaction_id (the first accepted line wins). Order keeps file order so
// reports are stable.
type TruthSet struct {
	Expectations map[string]Expectation
	Order        []string
	Lines        int
	NotAccepted  int
}

// Observation is one decision event seen for a transaction.
type Observation struct {
	CardID     string
	OccurredAt time.Time
	Decision   string
	Score      int
	Rules      []string
}

// Results is everything one read collected: the truth and every observation
// of the expected transactions, keyed by transaction_id.
type Results struct {
	Truth        TruthSet
	Observations map[string][]Observation
}

// analyzedTransaction joins an expectation with its first observation; the
// pipeline check guarantees later observations agree or reports them.
type analyzedTransaction struct {
	Expectation
	Observation
	position int
}

// analyzed returns the expected transactions that have a decision, in truth
// file order.
func (r Results) analyzed() []analyzedTransaction {
	out := make([]analyzedTransaction, 0, len(r.Observations))
	for i, id := range r.Truth.Order {
		obs := r.Observations[id]
		if len(obs) == 0 {
			continue
		}
		out = append(out, analyzedTransaction{Expectation: r.Truth.Expectations[id], Observation: obs[0], position: i})
	}
	return out
}
