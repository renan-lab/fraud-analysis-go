// Package kafka wraps franz-go behind the small surface the project needs:
// synchronous publish, broker ping and topic provisioning.
package kafka

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Header is a Kafka record header.
type Header struct {
	Key   string
	Value string
}

// Message is one record to publish.
type Message struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers []Header
}

// Producer publishes records with acks=all and idempotent writes (the franz-go default).
type Producer struct {
	client *kgo.Client
}

// NewProducer connects lazily to brokers; nothing is dialed until the first request.
//
//	producer, err := kafka.NewProducer([]string{"localhost:29092"})
func NewProducer(brokers []string) (*Producer, error) {
	seeds, err := seedBrokers(brokers)
	if err != nil {
		return nil, err
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(seeds...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka client for brokers %v: %w", seeds, err)
	}
	return &Producer{client: client}, nil
}

func seedBrokers(brokers []string) ([]string, error) {
	seeds := make([]string, 0, len(brokers))
	for _, b := range brokers {
		if b = strings.TrimSpace(b); b != "" {
			seeds = append(seeds, b)
		}
	}
	if len(seeds) == 0 {
		return nil, fmt.Errorf("kafka brokers %q: want at least one host:port", brokers)
	}
	return seeds, nil
}

// Publish blocks until the broker acknowledges msg or ctx is done.
//
//	err := producer.Publish(ctx, kafka.Message{Topic: "transactions.received", Key: key, Value: body})
func (p *Producer) Publish(ctx context.Context, msg Message) error {
	if err := p.client.ProduceSync(ctx, toRecord(msg)).FirstErr(); err != nil {
		return fmt.Errorf("produce to topic %s: %w", msg.Topic, err)
	}
	return nil
}

func toRecord(msg Message) *kgo.Record {
	headers := make([]kgo.RecordHeader, 0, len(msg.Headers))
	for _, h := range msg.Headers {
		headers = append(headers, kgo.RecordHeader{Key: h.Key, Value: []byte(h.Value)})
	}
	return &kgo.Record{Topic: msg.Topic, Key: msg.Key, Value: msg.Value, Headers: headers}
}

// Ping succeeds when at least one broker answers a metadata request.
func (p *Producer) Ping(ctx context.Context) error {
	if err := p.client.Ping(ctx); err != nil {
		return fmt.Errorf("ping kafka: %w", err)
	}
	return nil
}

// EnsureTopic creates topic if it does not exist. An existing topic is left
// untouched, even if its partition count or replication factor differ.
//
//	err := producer.EnsureTopic(ctx, "transactions.received", 6, 1)
func (p *Producer) EnsureTopic(ctx context.Context, topic string, partitions, replicationFactor int) error {
	if partitions < 1 || partitions > math.MaxInt32 {
		return fmt.Errorf("topic %s partitions %d: want 1..%d", topic, partitions, math.MaxInt32)
	}
	if replicationFactor < 1 || replicationFactor > math.MaxInt16 {
		return fmt.Errorf("topic %s replication factor %d: want 1..%d", topic, replicationFactor, math.MaxInt16)
	}
	_, err := kadm.NewClient(p.client).CreateTopic(ctx, int32(partitions), int16(replicationFactor), nil, topic)
	if err != nil && !errors.Is(err, kerr.TopicAlreadyExists) {
		return fmt.Errorf("create topic %s: %w", topic, err)
	}
	return nil
}

// Close waits for buffered records until ctx is done, then closes the client.
func (p *Producer) Close(ctx context.Context) error {
	err := p.client.Flush(ctx)
	p.client.Close()
	if err != nil {
		return fmt.Errorf("flush kafka producer: %w", err)
	}
	return nil
}
