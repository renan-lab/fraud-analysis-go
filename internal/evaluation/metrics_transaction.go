package evaluation

import (
	"sort"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/groundtruth"
)

// TransactionStats scores each transaction on its own. A transaction is
// flagged (positive) when the decision is review or block.
type TransactionStats struct {
	TruePositive      int     `json:"true_positive"`
	FalseNegative     int     `json:"false_negative"`
	FalsePositive     int     `json:"false_positive"`
	TrueNegative      int     `json:"true_negative"`
	Precision         float64 `json:"precision"`
	Recall            float64 `json:"recall"`
	FalsePositiveRate float64 `json:"false_positive_rate"`
}

// ScenarioDecisions counts the decisions taken for one scenario.
type ScenarioDecisions struct {
	Scenario string            `json:"scenario"`
	Label    groundtruth.Label `json:"label"`
	Approve  int               `json:"approve"`
	Review   int               `json:"review"`
	Block    int               `json:"block"`
}

func flagged(decision string) bool {
	return decision != events.DecisionApprove
}

// transactionStats ignores labels other than fraud and legit: an accepted
// "reject" is an ingestion problem, already visible in the simulator report.
func transactionStats(txs []analyzedTransaction) TransactionStats {
	var s TransactionStats
	for _, tx := range txs {
		s.count(tx.Label, flagged(tx.Decision))
	}
	s.Precision = ratio(s.TruePositive, s.TruePositive+s.FalsePositive)
	s.Recall = ratio(s.TruePositive, s.TruePositive+s.FalseNegative)
	s.FalsePositiveRate = ratio(s.FalsePositive, s.FalsePositive+s.TrueNegative)
	return s
}

func (s *TransactionStats) count(label groundtruth.Label, positive bool) {
	switch {
	case label == groundtruth.LabelFraud && positive:
		s.TruePositive++
	case label == groundtruth.LabelFraud:
		s.FalseNegative++
	case label == groundtruth.LabelLegit && positive:
		s.FalsePositive++
	case label == groundtruth.LabelLegit:
		s.TrueNegative++
	}
}

type scenarioLabel struct {
	scenario string
	label    groundtruth.Label
}

// scenarioDecisions keys rows by scenario and label: a scenario can mix
// labels (impossible_travel sends a legit purchase at home before the fraud).
func scenarioDecisions(txs []analyzedTransaction) []ScenarioDecisions {
	rows := map[scenarioLabel]*ScenarioDecisions{}
	for _, tx := range txs {
		key := scenarioLabel{scenario: tx.Scenario, label: tx.Label}
		row, ok := rows[key]
		if !ok {
			row = &ScenarioDecisions{Scenario: tx.Scenario, Label: tx.Label}
			rows[key] = row
		}
		row.count(tx.Decision)
	}
	return sortScenarioRows(rows)
}

func sortScenarioRows(rows map[scenarioLabel]*ScenarioDecisions) []ScenarioDecisions {
	out := make([]ScenarioDecisions, 0, len(rows))
	for _, row := range rows {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Scenario != out[j].Scenario {
			return out[i].Scenario < out[j].Scenario
		}
		return out[i].Label < out[j].Label
	})
	return out
}

func (s *ScenarioDecisions) count(decision string) {
	switch decision {
	case events.DecisionBlock:
		s.Block++
	case events.DecisionReview:
		s.Review++
	default:
		s.Approve++
	}
}

// ratio returns 0 instead of NaN when there is nothing to divide, so JSON
// stays encodable and empty runs read as "nothing detected".
func ratio(num, den int) float64 {
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}
