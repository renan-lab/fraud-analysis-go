package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
)

func TestKafkaPublisherBuildsMessage(t *testing.T) {
	t.Parallel()
	producer := &fakeProducer{}
	pub := NewKafkaPublisher(producer, "topic-x", prometheus.NewRegistry())
	svc := NewService(pub, fixedClock{testNow}, sequentialIDs(), 0)
	ev := svc.newEvent(validTransaction(), "req-1")
	if err := pub.PublishTransactionReceived(context.Background(), ev); err != nil {
		t.Fatalf("PublishTransactionReceived: %v", err)
	}
	if len(producer.messages) != 1 {
		t.Fatalf("produced %d messages, want 1", len(producer.messages))
	}
	msg := producer.messages[0]
	if msg.Topic != "topic-x" || string(msg.Key) != ev.Transaction.TransactionID {
		t.Fatalf("message topic %q key %q, want topic-x keyed by transaction_id", msg.Topic, msg.Key)
	}
	headers := map[string]string{}
	for _, h := range msg.Headers {
		headers[h.Key] = h.Value
	}
	if headers[events.HeaderEventType] != "transaction.received" || headers[events.HeaderSchemaVersion] != "1" || headers[events.HeaderRequestID] != "req-1" {
		t.Fatalf("headers = %v, want event_type, schema_version and request_id", headers)
	}
	var decoded events.TransactionReceived
	if err := json.Unmarshal(msg.Value, &decoded); err != nil || decoded.EventID != ev.EventID {
		t.Fatalf("value %s (err %v), want JSON envelope with event_id %s", msg.Value, err, ev.EventID)
	}
}

func TestKafkaPublisherWrapsProducerError(t *testing.T) {
	t.Parallel()
	brokerErr := errors.New("leader not available")
	pub := NewKafkaPublisher(&fakeProducer{err: brokerErr}, "topic-x", prometheus.NewRegistry())
	err := pub.PublishTransactionReceived(context.Background(), events.TransactionReceived{EventID: "e1"})
	if !errors.Is(err, brokerErr) {
		t.Fatalf("error = %v, want wrapping %v", err, brokerErr)
	}
}
