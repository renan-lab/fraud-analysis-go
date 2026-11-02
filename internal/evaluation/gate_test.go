package evaluation

import (
	"reflect"
	"testing"
)

func TestGatesCheck(t *testing.T) {
	t.Parallel()
	report := Report{Transactions: TransactionStats{Recall: 0.5, FalsePositiveRate: 0.07}, SequenceRecall: 0.9}
	tests := []struct {
		name  string
		gates Gates
		want  []string
	}{
		{name: "disabled", gates: Gates{}},
		{name: "all pass", gates: Gates{MinRecall: 0.5, MinSequenceRecall: 0.9, MaxFPR: 0.1}},
		{name: "all fail", gates: Gates{MinRecall: 0.6, MinSequenceRecall: 0.95, MaxFPR: 0.05},
			want: []string{"recall = 0.5000, want >= 0.6000", "sequence_recall = 0.9000, want >= 0.9500", "false_positive_rate = 0.0700, want <= 0.0500"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, v := range tt.gates.Check(report) {
				got = append(got, v.String())
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Check(%+v) = %q, want %q", tt.gates, got, tt.want)
			}
		})
	}
}
