package evaluation

import (
	"sort"

	"github.com/renan-lab/fraud-analysis-go/internal/groundtruth"
)

// SequenceStats counts fraud sequences of one scenario. A sequence is every
// fraud-labeled transaction of the scenario on one card; it is detected when
// at least one of them was flagged. Rules need evidence to accumulate (the
// first purchases of a velocity burst pass by design), so this is the number
// that says whether an attack was stopped.
type SequenceStats struct {
	Scenario string `json:"scenario"`
	Total    int    `json:"total"`
	Detected int    `json:"detected"`
}

type sequenceKey struct {
	scenario string
	cardID   string
}

func sequenceStats(txs []analyzedTransaction) []SequenceStats {
	detected := map[sequenceKey]bool{}
	for _, tx := range txs {
		if tx.Label != groundtruth.LabelFraud {
			continue
		}
		key := sequenceKey{scenario: tx.Scenario, cardID: tx.CardID}
		detected[key] = detected[key] || flagged(tx.Decision)
	}
	return sumSequences(detected)
}

func sumSequences(detected map[sequenceKey]bool) []SequenceStats {
	byScenario := map[string]*SequenceStats{}
	for key, caught := range detected {
		row, ok := byScenario[key.scenario]
		if !ok {
			row = &SequenceStats{Scenario: key.scenario}
			byScenario[key.scenario] = row
		}
		row.Total++
		if caught {
			row.Detected++
		}
	}
	out := make([]SequenceStats, 0, len(byScenario))
	for _, row := range byScenario {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Scenario < out[j].Scenario })
	return out
}

// sequenceRecall is detected over total across every scenario.
func sequenceRecall(stats []SequenceStats) float64 {
	var total, detected int
	for _, s := range stats {
		total += s.Total
		detected += s.Detected
	}
	return ratio(detected, total)
}
