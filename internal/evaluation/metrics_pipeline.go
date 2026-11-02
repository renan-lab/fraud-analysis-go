package evaluation

import "sort"

// PipelineStats says whether every accepted transaction was analyzed, and
// analyzed consistently when Kafka redelivered it.
type PipelineStats struct {
	TruthLines        int            `json:"truth_lines"`
	NotAccepted       int            `json:"not_accepted"`
	Accepted          int            `json:"accepted"`
	Analyzed          int            `json:"analyzed"`
	Missing           int            `json:"missing"`
	MissingByScenario map[string]int `json:"missing_by_scenario"`
	Redelivered       int            `json:"redelivered"`
	DivergentIDs      []string       `json:"divergent_ids"`
}

func pipelineStats(r Results) PipelineStats {
	stats := PipelineStats{
		TruthLines:        r.Truth.Lines,
		NotAccepted:       r.Truth.NotAccepted,
		Accepted:          len(r.Truth.Expectations),
		MissingByScenario: map[string]int{},
		DivergentIDs:      divergentIDs(r.Observations),
	}
	for _, id := range r.Truth.Order {
		stats.countObservations(r.Truth.Expectations[id], len(r.Observations[id]))
	}
	return stats
}

func (s *PipelineStats) countObservations(exp Expectation, n int) {
	switch {
	case n == 0:
		s.Missing++
		s.MissingByScenario[exp.Scenario]++
	case n > 1:
		s.Analyzed++
		s.Redelivered++
	default:
		s.Analyzed++
	}
}

// divergentIDs lists transactions whose observations disagree on decision or
// score: a broken idempotency invariant (ARCHITECTURE.md, Invariants).
func divergentIDs(observations map[string][]Observation) []string {
	ids := []string{}
	for id, obs := range observations {
		if !consistent(obs) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func consistent(obs []Observation) bool {
	for _, o := range obs[1:] {
		if o.Decision != obs[0].Decision || o.Score != obs[0].Score {
			return false
		}
	}
	return true
}
