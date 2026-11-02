package evaluation

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/groundtruth"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

var testStart = time.Date(2026, 11, 2, 19, 15, 0, 0, time.UTC)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTruthSet builds a TruthSet from expectations, keeping their order.
func newTruthSet(exps ...Expectation) TruthSet {
	set := TruthSet{Expectations: map[string]Expectation{}, Lines: len(exps)}
	for _, e := range exps {
		set.Expectations[e.TransactionID] = e
		set.Order = append(set.Order, e.TransactionID)
	}
	return set
}

func fraudExp(id, scenario string) Expectation {
	return Expectation{TransactionID: id, Scenario: scenario, Label: groundtruth.LabelFraud}
}

func legitExp(id string) Expectation {
	return Expectation{TransactionID: id, Scenario: "legit/purchase", Label: groundtruth.LabelLegit}
}

// analyzedEvent is a decision for id on card at testStart+minute.
func analyzedEvent(id, card string, minute int, decision string, rules ...string) events.TransactionAnalyzed {
	results := make([]events.RuleResult, 0, len(rules))
	for _, r := range rules {
		results = append(results, events.RuleResult{Rule: r, Score: 40, Reason: "test"})
	}
	return events.TransactionAnalyzed{
		EventType: events.EventTypeTransactionAnalyzed, TransactionID: id, CardID: card,
		OccurredAt: testStart.Add(time.Duration(minute) * time.Minute), Decision: decision, Score: 40 * len(rules), RuleResults: results,
	}
}

// analyzedTx is the joined form used by metrics tests.
func analyzedTx(exp Expectation, ev events.TransactionAnalyzed) analyzedTransaction {
	return analyzedTransaction{Expectation: exp, Observation: toObservation(ev)}
}

func analyzedRecord(t *testing.T, ev events.TransactionAnalyzed) kafka.Record {
	t.Helper()
	value, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal %+v: %v", ev, err)
	}
	return kafka.Record{Topic: events.TopicTransactionsAnalyzed, Key: []byte(ev.TransactionID), Value: value}
}

// testLimits settle fast so tests that reach completion end in milliseconds.
var testLimits = ReadLimits{Wait: time.Minute, Settle: 10 * time.Millisecond}

// fakeRecordReader replays records, then blocks until ctx is done, like a
// real reader waiting for new messages. err, when set, is returned at once.
type fakeRecordReader struct {
	records []kafka.Record
	err     error
	reads   int
}

func (f *fakeRecordReader) Read(ctx context.Context, handle kafka.RecordHandler) error {
	f.reads++
	if f.err != nil {
		return f.err
	}
	for _, rec := range f.records {
		if err := handle(ctx, rec); err != nil {
			return err
		}
	}
	<-ctx.Done()
	return nil
}
