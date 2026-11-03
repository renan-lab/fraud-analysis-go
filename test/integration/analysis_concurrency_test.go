//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/renan-lab/fraud-analysis-go/internal/analysis"
	"github.com/renan-lab/fraud-analysis-go/internal/events"
	"github.com/renan-lab/fraud-analysis-go/internal/ingestion"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/config"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/ids"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/kafka"
)

// TestSameAccountIsSerializedAcrossWorkers needs Kafka and Redis from `make infra`.
// Two workers share a group on a 6-partition topic. An impossible-travel pair
// of one account must land on one partition, so a single worker scores it in
// order and the second purchase sees the first (regression for ADR 0012).
func TestSameAccountIsSerializedAcrossWorkers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	brokers := config.Load().KafkaBrokers
	inTopic, outTopic := newTestTopicWithPartitions(ctx, t, brokers, 6), newTestTopic(ctx, t, brokers)
	producer := newProducer(t, brokers)
	home, away := newTravelPair()
	publishViaIngestion(ctx, t, producer, inTopic, home, away)

	assertSamePartition(t, consumeN(ctx, t, brokers, inTopic, 2))
	stopWorkers := startWorkerPair(ctx, t, brokers, inTopic, analysis.NewKafkaPublisher(producer, outTopic))
	defer stopWorkers()

	decisions := decodeAnalyzed(t, consumeN(ctx, t, brokers, outTopic, 2))
	if got := decisions[away.Transaction.TransactionID]; !firedRule(got, "impossible_travel") {
		t.Fatalf("decision of %s = %+v, want impossible_travel against %s", away.Transaction.TransactionID, got, home.Transaction.TransactionID)
	}
}

// newTravelPair returns a purchase in Sao Paulo and one in Lisbon (~7,900 km)
// five minutes later, on the same account and card.
func newTravelPair() (events.TransactionReceived, events.TransactionReceived) {
	home, away := newReceivedEvent(), newReceivedEvent()
	away.Transaction.AccountID, away.Transaction.CardID = home.Transaction.AccountID, home.Transaction.CardID
	away.Transaction.DeviceID = home.Transaction.DeviceID
	home.Transaction.OccurredAt = home.Transaction.OccurredAt.Add(-10 * time.Minute)
	away.Transaction.OccurredAt = home.Transaction.OccurredAt.Add(5 * time.Minute)
	home.Transaction.Location = &events.Location{Lat: -23.5505, Lon: -46.6333}
	away.Transaction.Location = &events.Location{Lat: 38.7223, Lon: -9.1393}
	return home, away
}

// publishViaIngestion uses the ingestion publisher so the test exercises the
// production partition key.
func publishViaIngestion(ctx context.Context, t *testing.T, producer *kafka.Producer, topic string, evs ...events.TransactionReceived) {
	t.Helper()
	pub := ingestion.NewKafkaPublisher(producer, topic, prometheus.NewRegistry())
	for _, ev := range evs {
		if err := pub.PublishTransactionReceived(ctx, ev); err != nil {
			t.Fatalf("PublishTransactionReceived(%s) to %s: %v", ev.Transaction.TransactionID, topic, err)
		}
	}
}

func startWorkerPair(ctx context.Context, t *testing.T, brokers []string, topic string, pub analysis.AnalyzedPublisher) func() {
	t.Helper()
	group := "it-" + ids.NewV4()
	rules := []analysis.Rule{analysis.NewImpossibleTravelRule(analysis.TravelParams{MaxKMH: 900, MinKM: 500, Score: 80})}
	stopFirst := startAnalysisWorker(ctx, t, brokers, topic, group, rules, pub)
	stopSecond := startAnalysisWorker(ctx, t, brokers, topic, group, rules, pub)
	return func() {
		stopFirst()
		stopSecond()
	}
}

// consumeN reads topic from the start, without a group, until n records arrived.
func consumeN(ctx context.Context, t *testing.T, brokers []string, topic string, n int) []*kgo.Record {
	t.Helper()
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	if err != nil {
		t.Fatalf("consumer: %v", err)
	}
	defer client.Close()
	var recs []*kgo.Record
	for len(recs) < n {
		fetches := client.PollFetches(ctx)
		if err := ctx.Err(); err != nil {
			t.Fatalf("got %d records on %s before timeout, want %d: %v", len(recs), topic, n, err)
		}
		recs = append(recs, fetches.Records()...)
	}
	return recs
}

func assertSamePartition(t *testing.T, recs []*kgo.Record) {
	t.Helper()
	for _, r := range recs[1:] {
		if r.Partition != recs[0].Partition {
			t.Fatalf("key %s on partition %d, key %s on partition %d, want one partition per account",
				recs[0].Key, recs[0].Partition, r.Key, r.Partition)
		}
	}
}

func decodeAnalyzed(t *testing.T, recs []*kgo.Record) map[string]events.TransactionAnalyzed {
	t.Helper()
	byTx := make(map[string]events.TransactionAnalyzed, len(recs))
	for _, r := range recs {
		var ev events.TransactionAnalyzed
		if err := json.Unmarshal(r.Value, &ev); err != nil {
			t.Fatalf("decode %s: %v", r.Value, err)
		}
		byTx[ev.TransactionID] = ev
	}
	return byTx
}

func firedRule(ev events.TransactionAnalyzed, rule string) bool {
	for _, r := range ev.RuleResults {
		if r.Rule == rule {
			return true
		}
	}
	return false
}
