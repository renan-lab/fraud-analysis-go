package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// commitTimeout bounds one offset commit. Commits use a context detached from
// shutdown, so a message processed during Ctrl+C is still committed.
const commitTimeout = 5 * time.Second

// Record is one consumed message.
type Record struct {
	Topic     string
	Partition int32
	Offset    int64
	Key       []byte
	Value     []byte
	Headers   []Header
}

// RecordHandler processes one record. A nil return commits its offset; an
// error stops Consumer.Run without committing, so the record is redelivered.
type RecordHandler func(ctx context.Context, rec Record) error

// Consumer reads topics as a member of a consumer group and commits offsets
// manually, one record at a time, only after the handler succeeded.
type Consumer struct {
	client *kgo.Client
	logger *slog.Logger
}

// ConsumerConfig names the brokers, group and topics of a Consumer.
type ConsumerConfig struct {
	Brokers []string
	Group   string
	Topics  []string
}

// NewConsumer joins cfg.Group lazily; nothing is dialed until Run polls.
// A group with no committed offset starts at the oldest record.
//
//	c, err := kafka.NewConsumer(kafka.ConsumerConfig{Brokers: brokers, Group: "analysis", Topics: []string{"transactions.received"}}, logger)
func NewConsumer(cfg ConsumerConfig, logger *slog.Logger) (*Consumer, error) {
	if err := validateConsumerConfig(cfg); err != nil {
		return nil, err
	}
	seeds, err := seedBrokers(cfg.Brokers)
	if err != nil {
		return nil, err
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(seeds...),
		kgo.ConsumerGroup(cfg.Group),
		kgo.ConsumeTopics(cfg.Topics...),
		kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka consumer for group %s: %w", cfg.Group, err)
	}
	return &Consumer{client: client, logger: logger}, nil
}

func validateConsumerConfig(cfg ConsumerConfig) error {
	if strings.TrimSpace(cfg.Group) == "" {
		return fmt.Errorf("consumer group %q: want a non-empty name", cfg.Group)
	}
	if len(cfg.Topics) == 0 {
		return fmt.Errorf("consumer topics %q: want at least one topic", cfg.Topics)
	}
	return nil
}

// Run polls and hands records to handle in partition order until ctx is done
// (returns nil) or handle fails (returns its error). Fetch errors are logged
// and retried by the client, they never stop Run.
//
//	err := consumer.Run(ctx, func(ctx context.Context, rec kafka.Record) error { return nil })
func (c *Consumer) Run(ctx context.Context, handle RecordHandler) error {
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil || fetches.IsClientClosed() {
			return nil
		}
		fetches.EachError(func(topic string, partition int32, err error) {
			c.logger.Warn("kafka fetch error", "topic", topic, "partition", partition, "error", err)
		})
		if err := c.handleFetches(ctx, fetches, handle); err != nil {
			return err
		}
	}
}

func (c *Consumer) handleFetches(ctx context.Context, fetches kgo.Fetches, handle RecordHandler) error {
	for iter := fetches.RecordIter(); !iter.Done(); {
		rec := iter.Next()
		if err := handle(ctx, toConsumed(rec)); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("handle %s/%d@%d: %w", rec.Topic, rec.Partition, rec.Offset, err)
		}
		if err := c.commit(ctx, rec); err != nil {
			return err
		}
	}
	return nil
}

func (c *Consumer) commit(ctx context.Context, rec *kgo.Record) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), commitTimeout)
	defer cancel()
	if err := c.client.CommitRecords(ctx, rec); err != nil {
		return fmt.Errorf("commit %s/%d@%d: %w", rec.Topic, rec.Partition, rec.Offset, err)
	}
	return nil
}

func toConsumed(rec *kgo.Record) Record {
	headers := make([]Header, 0, len(rec.Headers))
	for _, h := range rec.Headers {
		headers = append(headers, Header{Key: h.Key, Value: string(h.Value)})
	}
	return Record{
		Topic: rec.Topic, Partition: rec.Partition, Offset: rec.Offset,
		Key: rec.Key, Value: rec.Value, Headers: headers,
	}
}

// HeaderValue returns the value of the first header named key, or "".
func (r Record) HeaderValue(key string) string {
	for _, h := range r.Headers {
		if h.Key == key {
			return h.Value
		}
	}
	return ""
}

// Close leaves the group and closes the client. Offsets are committed per
// record by Run, so nothing is pending here.
func (c *Consumer) Close() {
	c.client.Close()
}
