package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/platform/redis"
)

// fakeRedisCommands serves canned reads and counts write batches. Real write
// semantics (ZADD/HSETNX idempotency) are covered by test/integration.
type fakeRedisCommands struct {
	zsets   map[string][]redis.ZMember
	hashes  map[string]map[string]string
	readErr error
	batches []*redis.Batch
}

func (f *fakeRedisCommands) ZRangeByScore(_ context.Context, key string, min, max float64) ([]redis.ZMember, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	var out []redis.ZMember
	for _, m := range f.zsets[key] {
		if m.Score >= min && m.Score <= max {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeRedisCommands) HGet(_ context.Context, key, field string) (string, bool, error) {
	v, ok := f.hashes[key][field]
	return v, ok, nil
}

func (f *fakeRedisCommands) Exec(_ context.Context, b *redis.Batch) error {
	f.batches = append(f.batches, b)
	return nil
}

func newTestStore(t *testing.T, cmds RedisCommands) *RedisFeatureStore {
	t.Helper()
	store, err := NewRedisFeatureStore(cmds, StoreParams{HistoryRetention: time.Hour, BaselineSize: 3, ProfileTTL: 24 * time.Hour})
	if err != nil {
		t.Fatalf("NewRedisFeatureStore: %v", err)
	}
	return store
}

func millisAgo(d time.Duration) float64 {
	return unixMillis(testNow.Add(-d))
}

func TestRedisFeatureStoreLoadExcludesCurrentTransaction(t *testing.T) {
	t.Parallel()
	tx := newTestTransaction()
	cmds := &fakeRedisCommands{
		zsets: map[string][]redis.ZMember{
			cardHistoryKey(tx.CardID): {
				{Member: "too-old|100|ecommerce||", Score: millisAgo(2 * time.Hour)},
				{Member: "a|300|ecommerce||", Score: millisAgo(10 * time.Minute)},
				{Member: "garbage", Score: millisAgo(5 * time.Minute)},
				{Member: tx.ID + "|5000|card_present|-23.5505|-46.6333", Score: millisAgo(0)},
			},
			accountAmountsKey(tx.AccountID): {
				{Member: "a|100", Score: millisAgo(4 * time.Hour)},
				{Member: "b|200", Score: millisAgo(3 * time.Hour)},
				{Member: "c|300", Score: millisAgo(2 * time.Hour)},
				{Member: "d|400", Score: millisAgo(time.Hour)},
				{Member: tx.ID + "|5000", Score: millisAgo(0)},
			},
		},
		hashes: map[string]map[string]string{
			devicesKey(tx.AccountID):   {tx.DeviceID: tx.ID},
			countriesKey(tx.AccountID): {tx.MerchantCountry: "a"},
		},
	}
	f, err := newTestStore(t, cmds).Load(context.Background(), tx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(f.CardHistory) != 1 || f.CardHistory[0].TransactionID != "a" || f.CardHistory[0].Amount != 300 {
		t.Errorf("CardHistory = %+v, want only event a (no self, no garbage, no expired)", f.CardHistory)
	}
	if got := f.AccountAmounts; len(got) != 3 || got[0] != 200 || got[2] != 400 {
		t.Errorf("AccountAmounts = %v, want newest 3 without self [200 300 400]", got)
	}
	if f.DeviceKnown {
		t.Errorf("DeviceKnown = true, want false: device was first stored by the current transaction")
	}
	if !f.CountryKnown {
		t.Errorf("CountryKnown = false, want true: country was first stored by transaction a")
	}
}

func TestRedisFeatureStoreLoadWrapsReadError(t *testing.T) {
	t.Parallel()
	boom := errors.New("connection refused")
	_, err := newTestStore(t, &fakeRedisCommands{readErr: boom}).Load(context.Background(), newTestTransaction())
	if !errors.Is(err, boom) {
		t.Fatalf("Load error = %v, want wrapping %v", err, boom)
	}
}

func TestRedisFeatureStoreRecordSendsOneBatch(t *testing.T) {
	t.Parallel()
	cmds := &fakeRedisCommands{}
	if err := newTestStore(t, cmds).Record(context.Background(), newTestTransaction()); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(cmds.batches) != 1 {
		t.Fatalf("Record sent %d batches, want 1", len(cmds.batches))
	}
	if got := cmds.batches[0].Len(); got != 10 {
		t.Fatalf("Record batch has %d commands, want 10 (3 card history + 7 account profile)", got)
	}
}

func TestStoreParamsValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		p       StoreParams
		wantErr bool
	}{
		{p: StoreParams{HistoryRetention: time.Hour, BaselineSize: 1, ProfileTTL: time.Hour}},
		{p: StoreParams{HistoryRetention: 0, BaselineSize: 1, ProfileTTL: time.Hour}, wantErr: true},
		{p: StoreParams{HistoryRetention: time.Hour, BaselineSize: 0, ProfileTTL: time.Hour}, wantErr: true},
		{p: StoreParams{HistoryRetention: time.Hour, BaselineSize: 1, ProfileTTL: 0}, wantErr: true},
	}
	for _, tt := range tests {
		if err := tt.p.Validate(); (err != nil) != tt.wantErr {
			t.Errorf("%+v.Validate() = %v, wantErr %v", tt.p, err, tt.wantErr)
		}
	}
}

func TestCardEventRoundTrip(t *testing.T) {
	t.Parallel()
	withLoc := newTestTransaction()
	noLoc := newTestTransaction()
	noLoc.Location = nil
	for _, tx := range []Transaction{withLoc, noLoc} {
		ev, ok := decodeCardEvent(redis.ZMember{Member: encodeCardEvent(tx), Score: unixMillis(tx.OccurredAt)})
		if !ok {
			t.Fatalf("decodeCardEvent(encodeCardEvent(%+v)) not ok", tx)
		}
		if ev.TransactionID != tx.ID || ev.Amount != tx.Amount || ev.Channel != tx.Channel || !ev.At.Equal(tx.OccurredAt) {
			t.Errorf("round trip = %+v, want fields of %+v", ev, tx)
		}
		if (ev.Location == nil) != (tx.Location == nil) || (ev.Location != nil && *ev.Location != *tx.Location) {
			t.Errorf("round trip location = %v, want %v", ev.Location, tx.Location)
		}
	}
}

func TestDecodeCardEventRejectsMalformed(t *testing.T) {
	t.Parallel()
	for _, member := range []string{"", "a|b", "|100|ecommerce||", "a|x|ecommerce||", "a|100|ecommerce|1|", "a|100|ecommerce|x|y"} {
		if _, ok := decodeCardEvent(redis.ZMember{Member: member}); ok {
			t.Errorf("decodeCardEvent(%q) ok, want rejected", member)
		}
	}
}

func TestDecodeAmount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		member     string
		wantID     string
		wantAmount int64
		wantOK     bool
	}{
		{member: "a|1500", wantID: "a", wantAmount: 1500, wantOK: true},
		{member: "a", wantOK: false},
		{member: "|1500", wantOK: false},
		{member: "a|x", wantOK: false},
	}
	for _, tt := range tests {
		id, amount, ok := decodeAmount(tt.member)
		if ok != tt.wantOK || id != tt.wantID || amount != tt.wantAmount {
			t.Errorf("decodeAmount(%q) = %q, %d, %v, want %q, %d, %v", tt.member, id, amount, ok, tt.wantID, tt.wantAmount, tt.wantOK)
		}
	}
}
