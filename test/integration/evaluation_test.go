//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/evaluation"
	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/config"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/ids"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

// TestEvaluatorReadsDecisionsFromTopic needs Kafka from `make infra`. It
// publishes decisions (plus one from another run and one redelivery after the
// set is complete) on a private topic and expects the evaluator to see the
// redelivery and stop once the topic settles, long before the wait.
func TestEvaluatorReadsDecisionsFromTopic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	brokers := config.Load().KafkaBrokers
	topic := newTestTopic(ctx, t, brokers)
	fraudID, legitID := ids.NewV4(), ids.NewV4()
	producer := newProducer(t, brokers)
	for _, ev := range []events.TransactionAnalyzed{
		newAnalyzedEvent(ids.NewV4(), events.DecisionBlock),
		newAnalyzedEvent(fraudID, events.DecisionBlock, "velocity"),
		newAnalyzedEvent(legitID, events.DecisionApprove),
		newAnalyzedEvent(legitID, events.DecisionApprove),
	} {
		publishAnalyzed(ctx, t, producer, topic, ev)
	}

	start := time.Now()
	report := evaluateTopic(ctx, t, brokers, topic, truthFor(fraudID, legitID))
	if elapsed := time.Since(start); elapsed > 25*time.Second {
		t.Errorf("evaluation took %v, want an early stop before the 30s wait", elapsed)
	}
	p := report.Pipeline
	if p.Accepted != 2 || p.Analyzed != 2 || p.Missing != 0 || p.Redelivered != 1 || len(p.DivergentIDs) != 0 {
		t.Errorf("pipeline = %+v, want 2 accepted and analyzed, 1 redelivered, none missing or divergent", p)
	}
	if report.Transactions.Recall != 1 || report.Transactions.FalsePositiveRate != 0 || report.SequenceRecall != 1 {
		t.Errorf("transactions = %+v, sequence recall %v; want recall 1, FPR 0, sequence recall 1", report.Transactions, report.SequenceRecall)
	}
}

func truthFor(fraudID, legitID string) string {
	return fmt.Sprintf(`{"transaction_id":%q,"mode":"fraud","scenario":"fraud/velocity","label":"fraud","expected_status":202,"status":202}
{"transaction_id":%q,"mode":"legit","scenario":"legit/purchase","label":"legit","expected_status":202,"status":202}
`, fraudID, legitID)
}

func evaluateTopic(ctx context.Context, t *testing.T, brokers []string, topic, truthNDJSON string) evaluation.Report {
	t.Helper()
	truth, err := evaluation.ReadTruth(strings.NewReader(truthNDJSON))
	if err != nil {
		t.Fatalf("ReadTruth: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	reader, err := kafka.NewTopicReader(brokers, topic, logger)
	if err != nil {
		t.Fatalf("NewTopicReader(%s): %v", topic, err)
	}
	t.Cleanup(reader.Close)
	limits := evaluation.ReadLimits{Wait: 30 * time.Second, Settle: 2 * time.Second}
	report, err := evaluation.NewEvaluator(reader, limits, logger).Evaluate(ctx, truth)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return report
}

func newAnalyzedEvent(txID, decision string, rules ...string) events.TransactionAnalyzed {
	results := []events.RuleResult{}
	for _, r := range rules {
		results = append(results, events.RuleResult{Rule: r, Score: 80, Reason: "integration"})
	}
	return events.TransactionAnalyzed{
		EventID: ids.NewV4(), EventType: events.EventTypeTransactionAnalyzed, SchemaVersion: events.TransactionAnalyzedSchemaVersion,
		AnalyzedAt: time.Now().UTC(), RequestID: ids.NewV4(), TransactionID: txID, AccountID: "acc_it", CardID: "card_it",
		Amount: 4590, Currency: "BRL", OccurredAt: time.Now().UTC(), Decision: decision, Score: 80 * len(rules), RuleResults: results,
	}
}

func publishAnalyzed(ctx context.Context, t *testing.T, producer *kafka.Producer, topic string, ev events.TransactionAnalyzed) {
	t.Helper()
	value, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := producer.Publish(ctx, kafka.Message{Topic: topic, Key: []byte(ev.TransactionID), Value: value}); err != nil {
		t.Fatalf("publish to %s: %v", topic, err)
	}
}
