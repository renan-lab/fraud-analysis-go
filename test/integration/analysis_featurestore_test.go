//go:build integration

package integration

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/analysis"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/config"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/ids"
	"github.com/renan-lab/fraud-analysis-go/internal/platform/redis"
)

// TestRedisFeatureStoreReplayIsIdempotent needs Redis from `make infra`
// (REDIS_ADDR, default localhost:6379). Each run uses fresh account/card IDs.
func TestRedisFeatureStoreReplayIsIdempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := newFeatureStore(ctx, t)
	suffix := ids.NewV4()[:12]
	base := time.Now().UTC().Truncate(time.Millisecond)

	first := newAnalysisTransaction(suffix, base.Add(-time.Minute), 4000)
	recordAll(ctx, t, store, first)
	second := newAnalysisTransaction(suffix, base, 5000)

	before := loadFeatures(ctx, t, store, second)
	recordAll(ctx, t, store, second, second)
	after := loadFeatures(ctx, t, store, second)

	if !reflect.DeepEqual(before, after) {
		t.Fatalf("features changed after recording the same transaction twice:\nbefore %+v\nafter  %+v", before, after)
	}
	if len(after.CardHistory) != 1 || after.CardHistory[0].TransactionID != first.ID {
		t.Fatalf("CardHistory = %+v, want only %s", after.CardHistory, first.ID)
	}
	if !after.DeviceKnown || !after.CountryKnown {
		t.Fatalf("features = %+v, want device and country known from %s", after, first.ID)
	}
}

func newFeatureStore(ctx context.Context, t *testing.T) *analysis.RedisFeatureStore {
	t.Helper()
	addr := config.Load().RedisAddr
	client, err := redis.NewClient(addr)
	if err != nil {
		t.Fatalf("redis.NewClient(%s): %v", addr, err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping redis at %s: %v (is `make infra` up?)", addr, err)
	}
	store, err := analysis.NewRedisFeatureStore(client, analysis.StoreParams{
		HistoryRetention: time.Hour, BaselineSize: 10, ProfileTTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewRedisFeatureStore: %v", err)
	}
	return store
}

func newAnalysisTransaction(suffix string, at time.Time, amount int64) analysis.Transaction {
	return analysis.Transaction{
		ID: ids.NewV4(), AccountID: "acc_it_" + suffix, CardID: "card_it_" + suffix,
		Amount: amount, Currency: "BRL", MerchantMCC: "5411", MerchantCountry: "BR",
		Channel: analysis.ChannelCardPresent, DeviceID: "dev_it", OccurredAt: at,
		Location: &analysis.Location{Lat: -23.5505, Lon: -46.6333},
	}
}

func recordAll(ctx context.Context, t *testing.T, store *analysis.RedisFeatureStore, txs ...analysis.Transaction) {
	t.Helper()
	for _, tx := range txs {
		if err := store.Record(ctx, tx); err != nil {
			t.Fatalf("Record(%s): %v", tx.ID, err)
		}
	}
}

func loadFeatures(ctx context.Context, t *testing.T, store *analysis.RedisFeatureStore, tx analysis.Transaction) analysis.Features {
	t.Helper()
	f, err := store.Load(ctx, tx)
	if err != nil {
		t.Fatalf("Load(%s): %v", tx.ID, err)
	}
	return f
}
