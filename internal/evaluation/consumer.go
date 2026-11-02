package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

// recordReader is the slice of kafka.TopicReader the evaluator needs.
type recordReader interface {
	Read(ctx context.Context, handle kafka.RecordHandler) error
}

// newAnalyzedHandler feeds transactions.analyzed records to c and reports
// activity to watch, which decides when to stop. Undecodable records are
// logged and skipped: an audit must not fail because of one bad message.
func newAnalyzedHandler(c *Collector, watch *quietWatch, logger *slog.Logger) kafka.RecordHandler {
	return func(_ context.Context, rec kafka.Record) error {
		defer func() { watch.recordSeen(c.Complete()) }()
		ev, err := decodeAnalyzed(rec)
		if err != nil {
			logger.Warn("skipping undecodable record", "topic", rec.Topic, "partition", rec.Partition,
				"offset", rec.Offset, "key", string(rec.Key), "error", err)
			return nil
		}
		c.Observe(ev)
		return nil
	}
}

func decodeAnalyzed(rec kafka.Record) (events.TransactionAnalyzed, error) {
	var ev events.TransactionAnalyzed
	if err := json.Unmarshal(rec.Value, &ev); err != nil {
		return ev, fmt.Errorf("decode %s value: %w; want events.TransactionAnalyzed JSON", rec.Topic, err)
	}
	if ev.EventType != events.EventTypeTransactionAnalyzed || ev.TransactionID == "" {
		return ev, fmt.Errorf("event_type %q transaction_id %q: want %q with a transaction_id",
			ev.EventType, ev.TransactionID, events.EventTypeTransactionAnalyzed)
	}
	return ev, nil
}
