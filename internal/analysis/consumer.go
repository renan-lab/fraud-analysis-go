package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

// TransactionAnalyzer is the slice of Service the consumer needs.
type TransactionAnalyzer interface {
	Analyze(ctx context.Context, ev events.TransactionReceived) (Analysis, error)
}

// RetryPolicy is the exponential backoff between attempts on transient errors.
type RetryPolicy struct {
	Initial time.Duration
	Max     time.Duration
}

// DefaultRetryPolicy waits 100 ms, doubling up to 5 s.
var DefaultRetryPolicy = RetryPolicy{Initial: 100 * time.Millisecond, Max: 5 * time.Second}

// Consumer turns transactions.received records into Analyze calls.
type Consumer struct {
	analyzer       TransactionAnalyzer
	logger         *slog.Logger
	processTimeout time.Duration
	retry          RetryPolicy
}

// NewConsumer bounds each attempt by processTimeout and retries transient
// failures with retry.
//
//	c := analysis.NewConsumer(svc, logger, 10*time.Second, analysis.DefaultRetryPolicy)
func NewConsumer(analyzer TransactionAnalyzer, logger *slog.Logger, processTimeout time.Duration, retry RetryPolicy) *Consumer {
	return &Consumer{analyzer: analyzer, logger: logger, processTimeout: processTimeout, retry: retry}
}

// HandleRecord implements kafka.RecordHandler. A record that cannot be decoded
// is logged and skipped (returns nil, so its offset is committed): retrying
// would never succeed and would block the partition. Transient errors are
// retried until success or ctx is done; only then is an error returned, and
// the record stays uncommitted.
func (c *Consumer) HandleRecord(ctx context.Context, rec kafka.Record) error {
	ev, err := decodeReceived(rec)
	if err != nil {
		c.logger.Error("skipping undecodable record", "topic", rec.Topic, "partition", rec.Partition,
			"offset", rec.Offset, "key", string(rec.Key), "error", err)
		return nil
	}
	return c.analyzeWithRetry(ctx, ev)
}

func (c *Consumer) analyzeWithRetry(ctx context.Context, ev events.TransactionReceived) error {
	wait := c.retry.Initial
	for attempt := 1; ; attempt++ {
		err := c.analyzeOnce(ctx, ev)
		if err == nil {
			return nil
		}
		c.logger.Warn("analysis attempt failed", "transaction_id", ev.Transaction.TransactionID,
			"attempt", attempt, "retry_in", wait.String(), "error", err)
		if err := sleepCtx(ctx, wait); err != nil {
			return fmt.Errorf("give up on %s after %d attempts: %w", ev.Transaction.TransactionID, attempt, err)
		}
		wait = min(2*wait, c.retry.Max)
	}
}

// analyzeOnce detaches from ctx cancellation so an attempt in progress during
// shutdown finishes (bounded by processTimeout) instead of being cut midway.
func (c *Consumer) analyzeOnce(ctx context.Context, ev events.TransactionReceived) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.processTimeout)
	defer cancel()
	a, err := c.analyzer.Analyze(ctx, ev)
	if err != nil {
		return err
	}
	c.logger.Info("transaction analyzed", "transaction_id", a.Transaction.ID,
		"decision", string(a.Decision), "score", a.Score, "rules", ruleNames(a.Results))
	return nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func ruleNames(results []RuleResult) []string {
	names := make([]string, 0, len(results))
	for _, r := range results {
		names = append(names, r.Rule)
	}
	return names
}

// decodeReceived parses and sanity-checks a transactions.received value.
func decodeReceived(rec kafka.Record) (events.TransactionReceived, error) {
	var ev events.TransactionReceived
	if err := json.Unmarshal(rec.Value, &ev); err != nil {
		return events.TransactionReceived{}, fmt.Errorf("decode %d-byte value: %w", len(rec.Value), err)
	}
	if ev.EventType != events.EventTypeTransactionReceived || ev.SchemaVersion != events.TransactionReceivedSchemaVersion {
		return events.TransactionReceived{}, fmt.Errorf("event %q v%d: want %q v%d",
			ev.EventType, ev.SchemaVersion, events.EventTypeTransactionReceived, events.TransactionReceivedSchemaVersion)
	}
	if ev.Transaction.TransactionID == "" || ev.Transaction.OccurredAt.IsZero() {
		return events.TransactionReceived{}, fmt.Errorf("event %s: want transaction_id and occurred_at set", ev.EventID)
	}
	return ev, nil
}
