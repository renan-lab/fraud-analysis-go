//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/analysis"
	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/config"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/ids"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

// TestAnalysisPipelinePublishesDecision needs Kafka and Redis from `make infra`.
// It wires the worker pieces on private topics: received -> Redis -> analyzed.
func TestAnalysisPipelinePublishesDecision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	brokers := config.Load().KafkaBrokers
	inTopic, outTopic := newTestTopic(ctx, t, brokers), newTestTopic(ctx, t, brokers)
	producer := newProducer(t, brokers)

	ev := newReceivedEvent()
	publishReceived(ctx, t, producer, inTopic, ev)
	stopWorker := startAnalysisWorker(ctx, t, brokers, inTopic, analysis.NewKafkaPublisher(producer, outTopic))
	defer stopWorker()

	rec := consumeOne(ctx, t, brokers, outTopic)
	var got events.TransactionAnalyzed
	if err := json.Unmarshal(rec.Value, &got); err != nil {
		t.Fatalf("decode %s: %v", rec.Value, err)
	}
	if got.TransactionID != ev.Transaction.TransactionID || got.RequestID != ev.RequestID {
		t.Fatalf("analyzed = %+v, want transaction %s request %s", got, ev.Transaction.TransactionID, ev.RequestID)
	}
	// A brand-new account trips new_device_foreign (device + country = 45),
	// below the default review threshold.
	if got.Decision != events.DecisionApprove || got.Score != 45 {
		t.Fatalf("analyzed decision = %s/%d (%+v), want approve/45", got.Decision, got.Score, got.RuleResults)
	}
}

func newReceivedEvent() events.TransactionReceived {
	suffix := ids.NewV4()[:12]
	return events.TransactionReceived{
		EventID: ids.NewV4(), EventType: events.EventTypeTransactionReceived,
		SchemaVersion: events.TransactionReceivedSchemaVersion, ReceivedAt: time.Now().UTC(), RequestID: ids.NewV4(),
		Transaction: events.Transaction{
			TransactionID: ids.NewV4(), AccountID: "acc_it_" + suffix, CardID: "card_it_" + suffix,
			Amount: 4590, Currency: "BRL",
			Merchant: events.Merchant{ID: "mer_it", Name: "Integration Shop", MCC: "5411", Country: "BR"},
			Channel:  analysis.ChannelCardPresent, DeviceID: "dev_it_" + suffix, IP: "10.0.0.1",
			OccurredAt: time.Now().UTC().Truncate(time.Second),
		},
	}
}

func publishReceived(ctx context.Context, t *testing.T, producer *kafka.Producer, topic string, ev events.TransactionReceived) {
	t.Helper()
	value, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	msg := kafka.Message{Topic: topic, Key: []byte(ev.Transaction.TransactionID), Value: value}
	if err := producer.Publish(ctx, msg); err != nil {
		t.Fatalf("publish to %s: %v", topic, err)
	}
}

// startAnalysisWorker runs the consumer in a goroutine; the returned func stops it and waits.
func startAnalysisWorker(ctx context.Context, t *testing.T, brokers []string, topic string, pub analysis.AnalyzedPublisher) func() {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := analysis.NewService(analysis.ServiceDeps{
		Store: newFeatureStore(ctx, t), Publisher: pub, Rules: defaultRules(),
		Thresholds: analysis.Thresholds{Review: 50, Block: 80}, Clock: analysis.SystemClock{}, NewID: ids.NewV4,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	consumer, err := kafka.NewConsumer(kafka.ConsumerConfig{Brokers: brokers, Group: "it-" + ids.NewV4(), Topics: []string{topic}}, logger)
	if err != nil {
		t.Fatalf("NewConsumer: %v", err)
	}
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	handler := analysis.NewConsumer(svc, logger, 10*time.Second, analysis.DefaultRetryPolicy)
	go func() { done <- consumer.Run(runCtx, handler.HandleRecord) }()
	return func() {
		stop()
		if err := <-done; err != nil {
			t.Errorf("consumer.Run: %v", err)
		}
		consumer.Close()
	}
}

func defaultRules() []analysis.Rule {
	return []analysis.Rule{
		analysis.NewVelocityRule(analysis.VelocityParams{Window: 3 * time.Minute, Max: 6, Score: 80}),
		analysis.NewNewDeviceForeignRule(analysis.DeviceCountryParams{RiskyMCCs: []string{"7995"}, NewDeviceScore: 20, NewCountryScore: 25, RiskyMCCScore: 25}),
	}
}
