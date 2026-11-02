package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

var fastRetry = RetryPolicy{Initial: time.Millisecond, Max: 2 * time.Millisecond}

func newTestConsumer(a TransactionAnalyzer) *Consumer {
	return NewConsumer(a, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second, fastRetry)
}

func receivedRecord(t *testing.T, ev events.TransactionReceived) kafka.Record {
	t.Helper()
	value, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal %+v: %v", ev, err)
	}
	return kafka.Record{Topic: events.TopicTransactionsReceived, Key: []byte(ev.Transaction.TransactionID), Value: value}
}

func TestHandleRecordSkipsPoisonRecords(t *testing.T) {
	t.Parallel()
	wrongVersion := newTestReceived()
	wrongVersion.SchemaVersion = 99
	noID := newTestReceived()
	noID.Transaction.TransactionID = ""
	tests := []struct {
		name string
		rec  kafka.Record
	}{
		{name: "invalid json", rec: kafka.Record{Value: []byte("{not json")}},
		{name: "unknown schema version", rec: receivedRecord(t, wrongVersion)},
		{name: "missing transaction id", rec: receivedRecord(t, noID)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			analyzer := &fakeAnalyzer{}
			if err := newTestConsumer(analyzer).HandleRecord(context.Background(), tt.rec); err != nil {
				t.Fatalf("HandleRecord(%s) = %v, want nil so the offset is committed", tt.name, err)
			}
			if analyzer.callCount() != 0 {
				t.Errorf("Analyze called %d times for a poison record, want 0", analyzer.callCount())
			}
		})
	}
}

func TestHandleRecordRetriesTransientErrors(t *testing.T) {
	t.Parallel()
	analyzer := &fakeAnalyzer{errs: []error{errors.New("redis down"), errors.New("kafka down")}}
	if err := newTestConsumer(analyzer).HandleRecord(context.Background(), receivedRecord(t, newTestReceived())); err != nil {
		t.Fatalf("HandleRecord = %v, want nil after retries", err)
	}
	if got := analyzer.callCount(); got != 3 {
		t.Errorf("Analyze called %d times, want 3 (2 failures + 1 success)", got)
	}
}

func TestHandleRecordGivesUpWhenContextDone(t *testing.T) {
	t.Parallel()
	errs := make([]error, 1000)
	for i := range errs {
		errs[i] = errors.New("redis down")
	}
	analyzer := &fakeAnalyzer{errs: errs}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := newTestConsumer(analyzer).HandleRecord(ctx, receivedRecord(t, newTestReceived()))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("HandleRecord = %v, want context.DeadlineExceeded so the record stays uncommitted", err)
	}
}

func TestSleepCtx(t *testing.T) {
	t.Parallel()
	if err := sleepCtx(context.Background(), time.Millisecond); err != nil {
		t.Errorf("sleepCtx(1ms) = %v, want nil", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepCtx(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("sleepCtx(cancelled, 1h) = %v, want context.Canceled", err)
	}
}

func TestRuleNames(t *testing.T) {
	t.Parallel()
	got := ruleNames([]RuleResult{{Rule: "a"}, {Rule: "b"}})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("ruleNames = %v, want [a b]", got)
	}
}
