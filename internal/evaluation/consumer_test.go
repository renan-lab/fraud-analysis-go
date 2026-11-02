package evaluation

import (
	"context"
	"testing"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

func TestAnalyzedHandlerSkipsBadRecordsAndReportsCompletion(t *testing.T) {
	t.Parallel()
	c := NewCollector(newTruthSet(legitExp("t1")))
	watch := &quietWatch{}
	handle := newAnalyzedHandler(c, watch, discardLogger())
	bad := kafka.Record{Topic: events.TopicTransactionsAnalyzed, Value: []byte("{")}
	if err := handle(context.Background(), bad); err != nil {
		t.Fatalf("handle(undecodable) = %v, want nil (skipped)", err)
	}
	if watch.complete.Load() || watch.lastRecord.Load() == 0 {
		t.Fatalf("after undecodable record: complete=%v lastRecord=%d, want false and set", watch.complete.Load(), watch.lastRecord.Load())
	}
	if err := handle(context.Background(), analyzedRecord(t, analyzedEvent("t1", "card", 0, events.DecisionApprove))); err != nil {
		t.Fatalf("handle(last expected) = %v, want nil", err)
	}
	if !watch.complete.Load() {
		t.Errorf("after last expected record: complete = false, want true")
	}
}

func TestDecodeAnalyzed(t *testing.T) {
	t.Parallel()
	valid := analyzedEvent("t1", "card", 0, events.DecisionApprove)
	wrongType := valid
	wrongType.EventType = "transaction.received"
	noID := valid
	noID.TransactionID = ""
	tests := []struct {
		name    string
		ev      events.TransactionAnalyzed
		wantErr bool
	}{
		{name: "valid", ev: valid},
		{name: "wrong event type", ev: wrongType, wantErr: true},
		{name: "missing transaction id", ev: noID, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := decodeAnalyzed(analyzedRecord(t, tt.ev))
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeAnalyzed(%+v) error = %v, wantErr %v", tt.ev, err, tt.wantErr)
			}
		})
	}
}
