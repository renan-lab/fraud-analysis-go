package evaluation

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

func TestEvaluateStopsSoonAfterEveryTransactionHasADecision(t *testing.T) {
	t.Parallel()
	reader := &fakeRecordReader{records: []kafka.Record{
		analyzedRecord(t, analyzedEvent("t1", "card", 0, events.DecisionApprove)),
		analyzedRecord(t, analyzedEvent("t2", "card", 1, events.DecisionBlock, "velocity")),
	}}
	start := time.Now()
	report, err := NewEvaluator(reader, testLimits, discardLogger()).Evaluate(context.Background(), newTruthSet(legitExp("t1"), fraudExp("t2", "fraud/velocity")))
	if err != nil {
		t.Fatalf("Evaluate error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Evaluate took %v, want a stop after the 10ms settle, well before the 1m wait", elapsed)
	}
	if report.Pipeline.Analyzed != 2 || report.Pipeline.Missing != 0 || report.Transactions.Recall != 1 {
		t.Errorf("report = %+v, want 2 analyzed, 0 missing, recall 1", report.Pipeline)
	}
}

// TestEvaluateSeesRedeliveryAfterCompletion is the regression test for
// stopping at the first complete moment, which hid late divergent decisions.
func TestEvaluateSeesRedeliveryAfterCompletion(t *testing.T) {
	t.Parallel()
	reader := &fakeRecordReader{records: []kafka.Record{
		analyzedRecord(t, analyzedEvent("t1", "card", 0, events.DecisionApprove)),
		analyzedRecord(t, analyzedEvent("t1", "card", 0, events.DecisionReview, "velocity")),
	}}
	report, err := NewEvaluator(reader, testLimits, discardLogger()).Evaluate(context.Background(), newTruthSet(legitExp("t1")))
	if err != nil {
		t.Fatalf("Evaluate error = %v", err)
	}
	if want := []string{"t1"}; report.Pipeline.Redelivered != 1 || !reflect.DeepEqual(report.Pipeline.DivergentIDs, want) {
		t.Errorf("pipeline = %+v, want t1 redelivered and divergent", report.Pipeline)
	}
}

func TestEvaluateReportsMissingWhenWaitExpires(t *testing.T) {
	t.Parallel()
	reader := &fakeRecordReader{records: []kafka.Record{analyzedRecord(t, analyzedEvent("t1", "card", 0, events.DecisionApprove))}}
	limits := ReadLimits{Wait: 20 * time.Millisecond, Settle: time.Millisecond}
	report, err := NewEvaluator(reader, limits, discardLogger()).Evaluate(context.Background(), newTruthSet(legitExp("t1"), legitExp("t2")))
	if err != nil {
		t.Fatalf("Evaluate error = %v", err)
	}
	if report.Pipeline.Missing != 1 || report.Pipeline.MissingByScenario["legit/purchase"] != 1 {
		t.Errorf("pipeline = %+v, want t2 missing under legit/purchase", report.Pipeline)
	}
}

func TestEvaluateReturnsReaderError(t *testing.T) {
	t.Parallel()
	boom := errors.New("broker gone")
	_, err := NewEvaluator(&fakeRecordReader{err: boom}, testLimits, discardLogger()).Evaluate(context.Background(), newTruthSet(legitExp("t1")))
	if !errors.Is(err, boom) {
		t.Fatalf("Evaluate error = %v, want %v", err, boom)
	}
}

func TestEvaluateSkipsReadingForEmptyTruth(t *testing.T) {
	t.Parallel()
	reader := &fakeRecordReader{}
	if _, err := NewEvaluator(reader, testLimits, discardLogger()).Evaluate(context.Background(), newTruthSet()); err != nil {
		t.Fatalf("Evaluate(empty truth) error = %v", err)
	}
	if reader.reads != 0 {
		t.Errorf("reads = %d, want 0 for an empty truth", reader.reads)
	}
}
