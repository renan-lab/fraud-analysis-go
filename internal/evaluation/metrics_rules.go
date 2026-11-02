package evaluation

import (
	"sort"

	"github.com/renan-lab/fraud-analysis-go/internal/groundtruth"
)

// RuleFirings counts how often a rule fired on fraud and on legit
// transactions; a rule with many legit firings is the one to tune.
type RuleFirings struct {
	Rule  string `json:"rule"`
	Fraud int    `json:"fraud"`
	Legit int    `json:"legit"`
}

func ruleFirings(txs []analyzedTransaction) []RuleFirings {
	byRule := map[string]*RuleFirings{}
	for _, tx := range txs {
		for _, name := range tx.Rules {
			row, ok := byRule[name]
			if !ok {
				row = &RuleFirings{Rule: name}
				byRule[name] = row
			}
			row.count(tx.Label)
		}
	}
	out := make([]RuleFirings, 0, len(byRule))
	for _, row := range byRule {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rule < out[j].Rule })
	return out
}

func (r *RuleFirings) count(label groundtruth.Label) {
	switch label {
	case groundtruth.LabelFraud:
		r.Fraud++
	case groundtruth.LabelLegit:
		r.Legit++
	}
}
