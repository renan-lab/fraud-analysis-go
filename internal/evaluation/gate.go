package evaluation

import "fmt"

// Gates are optional limits a report must meet; a zero value disables a gate.
type Gates struct {
	MinRecall         float64
	MinSequenceRecall float64
	MaxFPR            float64
}

// Violation is one gate the report did not meet.
type Violation struct {
	Metric string  `json:"metric"`
	Got    float64 `json:"got"`
	Limit  float64 `json:"limit"`
}

func (v Violation) String() string {
	return fmt.Sprintf("%s = %.4f, want %s", v.Metric, v.Got, v.wantText())
}

func (v Violation) wantText() string {
	if v.Metric == metricFalsePositiveRate {
		return fmt.Sprintf("<= %.4f", v.Limit)
	}
	return fmt.Sprintf(">= %.4f", v.Limit)
}

const (
	metricRecall            = "recall"
	metricSequenceRecall    = "sequence_recall"
	metricFalsePositiveRate = "false_positive_rate"
)

// Check returns the violated gates, empty when the report passes.
//
//	if v := gates.Check(report); len(v) > 0 { os.Exit(1) }
func (g Gates) Check(r Report) []Violation {
	var out []Violation
	if g.MinRecall > 0 && r.Transactions.Recall < g.MinRecall {
		out = append(out, Violation{Metric: metricRecall, Got: r.Transactions.Recall, Limit: g.MinRecall})
	}
	if g.MinSequenceRecall > 0 && r.SequenceRecall < g.MinSequenceRecall {
		out = append(out, Violation{Metric: metricSequenceRecall, Got: r.SequenceRecall, Limit: g.MinSequenceRecall})
	}
	if g.MaxFPR > 0 && r.Transactions.FalsePositiveRate > g.MaxFPR {
		out = append(out, Violation{Metric: metricFalsePositiveRate, Got: r.Transactions.FalsePositiveRate, Limit: g.MaxFPR})
	}
	return out
}
