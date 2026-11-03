package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

// MessageProducer is the slice of platform/kafka.Producer the publisher needs.
type MessageProducer interface {
	Publish(ctx context.Context, msg kafka.Message) error
}

// KafkaPublisher implements AnalyzedPublisher on top of Kafka.
type KafkaPublisher struct {
	producer MessageProducer
	topic    string
}

// NewKafkaPublisher publishes to topic.
//
//	pub := analysis.NewKafkaPublisher(producer, events.TopicTransactionsAnalyzed)
func NewKafkaPublisher(producer MessageProducer, topic string) *KafkaPublisher {
	return &KafkaPublisher{producer: producer, topic: topic}
}

// PublishTransactionAnalyzed sends ev keyed by transaction_id: its consumers
// upsert by transaction and keep no per-account state, so they need no
// per-account ordering, and the key spreads decisions evenly (ADR 0012).
func (p *KafkaPublisher) PublishTransactionAnalyzed(ctx context.Context, ev events.TransactionAnalyzed) error {
	value, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("encode event %s: %w", ev.EventID, err)
	}
	msg := kafka.Message{
		Topic: p.topic,
		Key:   []byte(ev.TransactionID),
		Value: value,
		Headers: []kafka.Header{
			{Key: events.HeaderEventType, Value: ev.EventType},
			{Key: events.HeaderSchemaVersion, Value: strconv.Itoa(ev.SchemaVersion)},
			{Key: events.HeaderRequestID, Value: ev.RequestID},
		},
	}
	if err := p.producer.Publish(ctx, msg); err != nil {
		return fmt.Errorf("publish event %s: %w", ev.EventID, err)
	}
	return nil
}
