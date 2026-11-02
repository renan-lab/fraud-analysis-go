package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
)

func TestKafkaPublisherKeysByTransactionAndSetsHeaders(t *testing.T) {
	t.Parallel()
	producer := &fakeMessageProducer{}
	ev := events.TransactionAnalyzed{
		EventID: "e1", EventType: events.EventTypeTransactionAnalyzed, SchemaVersion: 1,
		RequestID: "r1", TransactionID: "tx1", Decision: events.DecisionBlock, RuleResults: []events.RuleResult{},
	}
	if err := NewKafkaPublisher(producer, "topic.analyzed").PublishTransactionAnalyzed(context.Background(), ev); err != nil {
		t.Fatalf("PublishTransactionAnalyzed: %v", err)
	}
	if len(producer.messages) != 1 {
		t.Fatalf("published %d messages, want 1", len(producer.messages))
	}
	msg := producer.messages[0]
	if msg.Topic != "topic.analyzed" || string(msg.Key) != "tx1" {
		t.Errorf("message topic %q key %q, want topic.analyzed/tx1", msg.Topic, msg.Key)
	}
	want := map[string]string{events.HeaderEventType: events.EventTypeTransactionAnalyzed, events.HeaderSchemaVersion: "1", events.HeaderRequestID: "r1"}
	for _, h := range msg.Headers {
		if want[h.Key] != h.Value {
			t.Errorf("header %s = %q, want %q", h.Key, h.Value, want[h.Key])
		}
	}
	var decoded events.TransactionAnalyzed
	if err := json.Unmarshal(msg.Value, &decoded); err != nil || decoded.Decision != events.DecisionBlock {
		t.Errorf("value %s decoded to %+v (err %v), want decision block", msg.Value, decoded, err)
	}
}

func TestKafkaPublisherWrapsProducerError(t *testing.T) {
	t.Parallel()
	boom := errors.New("not acknowledged")
	err := NewKafkaPublisher(&fakeMessageProducer{err: boom}, "t").PublishTransactionAnalyzed(context.Background(), events.TransactionAnalyzed{EventID: "e1"})
	if !errors.Is(err, boom) {
		t.Fatalf("PublishTransactionAnalyzed error = %v, want wrapping %v", err, boom)
	}
}
