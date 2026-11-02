package evaluation

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
)

// sampleReport has one caught fraud, one missed fraud on the same card, a
// flagged legit after it and one missing transaction.
func sampleReport() Report {
	truth := newTruthSet(fraudExp("f1", "fraud/velocity"), fraudExp("f2", "fraud/velocity"), legitExp("l1"), legitExp("l2"))
	c := NewCollector(truth)
	c.Observe(analyzedEvent("f1", "card_a", 0, events.DecisionApprove))
	c.Observe(analyzedEvent("f2", "card_a", 1, events.DecisionBlock, "velocity"))
	c.Observe(analyzedEvent("l1", "card_a", 2, events.DecisionReview, "velocity"))
	return BuildReport(c.Results())
}

func TestBuildReportFillsEverySection(t *testing.T) {
	t.Parallel()
	r := sampleReport()
	if r.Pipeline.Missing != 1 || r.Transactions.TruePositive != 1 || r.Transactions.FalseNegative != 1 {
		t.Errorf("report pipeline/transactions = %+v / %+v, want 1 missing, 1 TP, 1 FN", r.Pipeline, r.Transactions)
	}
	if r.SequenceRecall != 1 || len(r.Scenarios) != 2 || len(r.Rules) != 1 {
		t.Errorf("report = %+v, want sequence recall 1, 2 scenarios, 1 rule", r)
	}
	if len(r.FalsePositiveCauses) != 1 || r.FalsePositiveCauses[0].Cause != CauseFraudBeforeOnCard {
		t.Errorf("FalsePositiveCauses = %+v, want one %s", r.FalsePositiveCauses, CauseFraudBeforeOnCard)
	}
}

func TestWriteTextHasEverySection(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := sampleReport().WriteText(&buf); err != nil {
		t.Fatalf("WriteText error = %v", err)
	}
	for _, want := range []string{"== Pipeline", "== Per transaction", "== Decisions per scenario", "== Fraud sequences", "== Rule firings", "== Flagged legit", "legit/purchase=1"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("WriteText output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestWriteJSONRoundTrips(t *testing.T) {
	t.Parallel()
	want := sampleReport()
	var buf bytes.Buffer
	if err := want.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON error = %v", err)
	}
	var got Report
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal %s: %v", buf.Bytes(), err)
	}
	if got.Transactions != want.Transactions || got.SequenceRecall != want.SequenceRecall {
		t.Errorf("round trip = %+v, want %+v", got.Transactions, want.Transactions)
	}
}

func TestFormatCounts(t *testing.T) {
	t.Parallel()
	in := map[string]int{"b": 2, "a": 1}
	if got := formatCounts(in); got != "a=1 b=2" {
		t.Errorf("formatCounts(%v) = %q, want %q", in, got, "a=1 b=2")
	}
}
