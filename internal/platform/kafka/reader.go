package kafka

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
)

// TopicReader reads every partition of one topic from the oldest record,
// outside any consumer group and without committing offsets, so each run sees
// the whole topic again. Use it for audits and replays, never for processing
// that must happen once (use Consumer).
type TopicReader struct {
	client *kgo.Client
	logger *slog.Logger
}

// NewTopicReader creates a reader for topic; nothing is dialed until Read.
//
//	r, err := kafka.NewTopicReader(brokers, events.TopicTransactionsAnalyzed, logger)
func NewTopicReader(brokers []string, topic string, logger *slog.Logger) (*TopicReader, error) {
	if topic == "" {
		return nil, fmt.Errorf("reader topic %q: want a non-empty name", topic)
	}
	seeds, err := seedBrokers(brokers)
	if err != nil {
		return nil, err
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(seeds...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka reader for topic %s: %w", topic, err)
	}
	return &TopicReader{client: client, logger: logger}, nil
}

// Read hands records to handle in partition order until ctx is done (returns
// nil) or handle fails (returns its error). It never reports "end of topic":
// the caller decides when to stop by cancelling ctx.
//
//	err := r.Read(ctx, func(ctx context.Context, rec kafka.Record) error { return nil })
func (r *TopicReader) Read(ctx context.Context, handle RecordHandler) error {
	for {
		fetches := r.client.PollFetches(ctx)
		if ctx.Err() != nil || fetches.IsClientClosed() {
			return nil
		}
		fetches.EachError(func(topic string, partition int32, err error) {
			r.logger.Warn("kafka fetch error", "topic", topic, "partition", partition, "error", err)
		})
		if err := handleUncommitted(ctx, fetches, handle); err != nil {
			return err
		}
	}
}

func handleUncommitted(ctx context.Context, fetches kgo.Fetches, handle RecordHandler) error {
	for iter := fetches.RecordIter(); !iter.Done(); {
		rec := iter.Next()
		if err := handle(ctx, toConsumed(rec)); err != nil {
			return fmt.Errorf("handle %s/%d@%d: %w", rec.Topic, rec.Partition, rec.Offset, err)
		}
	}
	return nil
}

// Close closes the client; there are no offsets to commit.
func (r *TopicReader) Close() {
	r.client.Close()
}
