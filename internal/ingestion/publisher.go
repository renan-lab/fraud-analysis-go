package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

// MessageProducer is the slice of platform/kafka.Producer the publisher needs.
type MessageProducer interface {
	Publish(ctx context.Context, msg kafka.Message) error
}

// KafkaPublisher implements Publisher on top of Kafka.
type KafkaPublisher struct {
	producer MessageProducer
	topic    string
	duration *prometheus.HistogramVec
}

// NewKafkaPublisher publishes to topic and registers
// ingestion_publish_duration_seconds{result} on reg.
//
//	pub := ingestion.NewKafkaPublisher(producer, events.TopicTransactionsReceived, registry)
func NewKafkaPublisher(producer MessageProducer, topic string, reg prometheus.Registerer) *KafkaPublisher {
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ingestion_publish_duration_seconds",
		Help:    "Time to get a broker acknowledgement for transactions.received, by result.",
		Buckets: prometheus.DefBuckets,
	}, []string{"result"})
	reg.MustRegister(duration)
	return &KafkaPublisher{producer: producer, topic: topic, duration: duration}
}

// PublishTransactionReceived sends ev keyed by transaction_id, so every event
// of one transaction lands on the same partition, in order.
func (p *KafkaPublisher) PublishTransactionReceived(ctx context.Context, ev events.TransactionReceived) error {
	msg, err := p.toMessage(ev)
	if err != nil {
		return err
	}
	start := time.Now()
	err = p.producer.Publish(ctx, msg)
	p.duration.WithLabelValues(publishResult(err)).Observe(time.Since(start).Seconds())
	if err != nil {
		return fmt.Errorf("publish event %s: %w", ev.EventID, err)
	}
	return nil
}

func (p *KafkaPublisher) toMessage(ev events.TransactionReceived) (kafka.Message, error) {
	value, err := json.Marshal(ev)
	if err != nil {
		return kafka.Message{}, fmt.Errorf("encode event %s: %w", ev.EventID, err)
	}
	return kafka.Message{
		Topic: p.topic,
		Key:   []byte(ev.Transaction.TransactionID),
		Value: value,
		Headers: []kafka.Header{
			{Key: events.HeaderEventType, Value: ev.EventType},
			{Key: events.HeaderSchemaVersion, Value: strconv.Itoa(ev.SchemaVersion)},
			{Key: events.HeaderRequestID, Value: ev.RequestID},
		},
	}, nil
}

func publishResult(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
