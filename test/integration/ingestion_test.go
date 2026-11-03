//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/ingestion"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/config"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/httpmw"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/ids"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

// TestPostTransactionPublishesEnvelope needs Kafka from `make infra`
// (KAFKA_BROKERS, default localhost:29092). Each run uses its own topic.
func TestPostTransactionPublishesEnvelope(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	brokers := config.Load().KafkaBrokers
	topic := newTestTopic(ctx, t, brokers)

	srv := httptest.NewServer(newIngestionHandler(t, brokers, topic))
	defer srv.Close()
	txID := ids.NewV4()
	resp := postTransaction(t, srv.URL, txID)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST /transactions status = %d, want 202", resp.StatusCode)
	}

	rec := consumeOne(ctx, t, brokers, topic)
	if string(rec.Key) != "acc_it" {
		t.Fatalf("record key = %q, want account_id %q", rec.Key, "acc_it")
	}
	assertHeaders(t, rec, resp.Header.Get(httpmw.HeaderRequestID))
	var ev events.TransactionReceived
	if err := json.Unmarshal(rec.Value, &ev); err != nil {
		t.Fatalf("decode record value %s: %v", rec.Value, err)
	}
	if ev.Transaction.TransactionID != txID || ev.EventType != events.EventTypeTransactionReceived || ev.EventID == "" {
		t.Fatalf("event = %+v, want transaction.received for %s with event_id", ev, txID)
	}
}

func TestEnsureTopicIsIdempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	brokers := config.Load().KafkaBrokers
	topic := newTestTopic(ctx, t, brokers)
	producer := newProducer(t, brokers)
	if err := producer.EnsureTopic(ctx, topic, 1, 1); err != nil {
		t.Fatalf("second EnsureTopic(%s) = %v, want nil for existing topic", topic, err)
	}
}

func newProducer(t *testing.T, brokers []string) *kafka.Producer {
	t.Helper()
	producer, err := kafka.NewProducer(brokers)
	if err != nil {
		t.Fatalf("NewProducer(%v): %v", brokers, err)
	}
	t.Cleanup(func() { _ = producer.Close(context.Background()) })
	return producer
}

// newTestTopic creates a unique single-partition topic and deletes it when the test ends.
func newTestTopic(ctx context.Context, t *testing.T, brokers []string) string {
	t.Helper()
	return newTestTopicWithPartitions(ctx, t, brokers, 1)
}

// newTestTopicWithPartitions creates a unique topic with n partitions and
// deletes it when the test ends.
func newTestTopicWithPartitions(ctx context.Context, t *testing.T, brokers []string, n int) string {
	t.Helper()
	topic := "it.transactions.received." + ids.NewV4()
	if err := newProducer(t, brokers).EnsureTopic(ctx, topic, n, 1); err != nil {
		t.Fatalf("EnsureTopic(%s) on %v: %v (is `make infra` up?)", topic, brokers, err)
	}
	t.Cleanup(func() { deleteTopic(t, brokers, topic) })
	return topic
}

func deleteTopic(t *testing.T, brokers []string, topic string) {
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Logf("delete topic %s: %v", topic, err)
		return
	}
	defer client.Close()
	if _, err := kadm.NewClient(client).DeleteTopic(context.Background(), topic); err != nil {
		t.Logf("delete topic %s: %v", topic, err)
	}
}

func newIngestionHandler(t *testing.T, brokers []string, topic string) http.Handler {
	t.Helper()
	pub := ingestion.NewKafkaPublisher(newProducer(t, brokers), topic, prometheus.NewRegistry())
	svc := ingestion.NewService(pub, ingestion.SystemClock{}, ids.NewV4, 10*time.Second)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpmw.RequestID(ingestion.NewHandler(svc, ingestion.SystemClock{}, logger))
}

func postTransaction(t *testing.T, baseURL, txID string) *http.Response {
	t.Helper()
	body := `{"transaction_id":"` + txID + `","account_id":"acc_it","card_id":"card_it","amount":12990,"currency":"BRL",` +
		`"merchant":{"id":"mer_it","name":"Integration Shop","mcc":"5411","country":"BR"},"channel":"ecommerce",` +
		`"device_id":"dev_it","ip":"10.0.0.1","occurred_at":"` + time.Now().UTC().Format(time.RFC3339) + `"}`
	resp, err := http.Post(baseURL+"/transactions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /transactions: %v", err)
	}
	_ = resp.Body.Close()
	return resp
}

func consumeOne(ctx context.Context, t *testing.T, brokers []string, topic string) *kgo.Record {
	t.Helper()
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	if err != nil {
		t.Fatalf("consumer: %v", err)
	}
	defer client.Close()
	for {
		fetches := client.PollFetches(ctx)
		if err := ctx.Err(); err != nil {
			t.Fatalf("no record on %s before timeout: %v", topic, err)
		}
		if recs := fetches.Records(); len(recs) > 0 {
			return recs[0]
		}
	}
}

func assertHeaders(t *testing.T, rec *kgo.Record, requestID string) {
	t.Helper()
	got := map[string]string{}
	for _, h := range rec.Headers {
		got[h.Key] = string(h.Value)
	}
	want := map[string]string{
		events.HeaderEventType:     events.EventTypeTransactionReceived,
		events.HeaderSchemaVersion: "1",
		events.HeaderRequestID:     requestID,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("header %s = %q, want %q (all headers %v)", k, got[k], v, got)
		}
	}
}
