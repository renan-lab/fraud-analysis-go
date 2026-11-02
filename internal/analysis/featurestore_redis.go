package analysis

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/renan-lab/fraud-analysis-go/internal/platform/redis"
)

// RedisCommands is the slice of platform/redis.Client the feature store needs.
type RedisCommands interface {
	ZRangeByScore(ctx context.Context, key string, min, max float64) ([]redis.ZMember, error)
	HGet(ctx context.Context, key, field string) (string, bool, error)
	Exec(ctx context.Context, b *redis.Batch) error
}

// StoreParams configures RedisFeatureStore.
type StoreParams struct {
	// HistoryRetention is how far back card history is read and kept.
	HistoryRetention time.Duration
	// BaselineSize is how many past amounts per account feed the median.
	BaselineSize int
	// ProfileTTL expires the amounts, devices and countries of idle accounts.
	ProfileTTL time.Duration
}

// Validate rejects params that would drop all history.
func (p StoreParams) Validate() error {
	if p.HistoryRetention <= 0 || p.ProfileTTL <= 0 || p.BaselineSize < 1 {
		return fmt.Errorf("store params %+v: want positive retention and ttl, baseline size >= 1", p)
	}
	return nil
}

// RedisFeatureStore keeps card and account state in Redis with idempotent
// writes keyed by transaction_id (key layout in ADR 0009).
type RedisFeatureStore struct {
	redis RedisCommands
	p     StoreParams
}

// NewRedisFeatureStore validates p and returns a store on top of cmds.
//
//	store, err := analysis.NewRedisFeatureStore(redisClient, analysis.StoreParams{HistoryRetention: 24 * time.Hour, BaselineSize: 50, ProfileTTL: 2160 * time.Hour})
func NewRedisFeatureStore(cmds RedisCommands, p StoreParams) (*RedisFeatureStore, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &RedisFeatureStore{redis: cmds, p: p}, nil
}

// Load reads the features of tx, excluding tx itself, so a redelivered
// transaction sees the same features as its first delivery.
func (s *RedisFeatureStore) Load(ctx context.Context, tx Transaction) (Features, error) {
	history, err := s.cardHistory(ctx, tx)
	if err != nil {
		return Features{}, err
	}
	amounts, err := s.accountAmounts(ctx, tx)
	if err != nil {
		return Features{}, err
	}
	deviceKnown, err := s.known(ctx, devicesKey(tx.AccountID), tx.DeviceID, tx.ID)
	if err != nil {
		return Features{}, err
	}
	countryKnown, err := s.known(ctx, countriesKey(tx.AccountID), tx.MerchantCountry, tx.ID)
	if err != nil {
		return Features{}, err
	}
	return Features{CardHistory: history, AccountAmounts: amounts, DeviceKnown: deviceKnown, CountryKnown: countryKnown}, nil
}

func (s *RedisFeatureStore) cardHistory(ctx context.Context, tx Transaction) ([]CardEvent, error) {
	at := unixMillis(tx.OccurredAt)
	members, err := s.redis.ZRangeByScore(ctx, cardHistoryKey(tx.CardID), at-float64(s.p.HistoryRetention.Milliseconds()), at)
	if err != nil {
		return nil, fmt.Errorf("load card history of %s: %w", tx.CardID, err)
	}
	history := make([]CardEvent, 0, len(members))
	for _, m := range members {
		ev, ok := decodeCardEvent(m)
		if ok && ev.TransactionID != tx.ID {
			history = append(history, ev)
		}
	}
	return history, nil
}

func (s *RedisFeatureStore) accountAmounts(ctx context.Context, tx Transaction) ([]int64, error) {
	members, err := s.redis.ZRangeByScore(ctx, accountAmountsKey(tx.AccountID), math.Inf(-1), unixMillis(tx.OccurredAt))
	if err != nil {
		return nil, fmt.Errorf("load amounts of %s: %w", tx.AccountID, err)
	}
	amounts := make([]int64, 0, len(members))
	for _, m := range members {
		id, amount, ok := decodeAmount(m.Member)
		if ok && id != tx.ID {
			amounts = append(amounts, amount)
		}
	}
	return newest(amounts, s.p.BaselineSize), nil
}

// known reports whether field was first stored by another transaction.
func (s *RedisFeatureStore) known(ctx context.Context, key, field, txID string) (bool, error) {
	firstTxID, ok, err := s.redis.HGet(ctx, key, field)
	if err != nil {
		return false, fmt.Errorf("load %s %s: %w", key, field, err)
	}
	return ok && firstTxID != txID, nil
}

// Record stores tx in one MULTI/EXEC. Every write is a no-op when repeated.
func (s *RedisFeatureStore) Record(ctx context.Context, tx Transaction) error {
	b := redis.NewBatch()
	s.recordCard(b, tx)
	s.recordAccount(b, tx)
	if err := s.redis.Exec(ctx, b); err != nil {
		return fmt.Errorf("record transaction %s: %w", tx.ID, err)
	}
	return nil
}

func (s *RedisFeatureStore) recordCard(b *redis.Batch, tx Transaction) {
	key := cardHistoryKey(tx.CardID)
	at := unixMillis(tx.OccurredAt)
	b.ZAdd(key, at, encodeCardEvent(tx))
	b.ZRemRangeByScore(key, math.Inf(-1), at-float64(s.p.HistoryRetention.Milliseconds())-1)
	b.Expire(key, s.p.HistoryRetention)
}

// recordAccount keeps BaselineSize+1 amounts so that, on a replay, the
// baseline still has BaselineSize entries after excluding the replayed one.
func (s *RedisFeatureStore) recordAccount(b *redis.Batch, tx Transaction) {
	amountsKey := accountAmountsKey(tx.AccountID)
	b.ZAdd(amountsKey, unixMillis(tx.OccurredAt), encodeAmount(tx))
	b.ZKeepHighest(amountsKey, s.p.BaselineSize+1)
	b.Expire(amountsKey, s.p.ProfileTTL)
	b.HSetNX(devicesKey(tx.AccountID), tx.DeviceID, tx.ID)
	b.Expire(devicesKey(tx.AccountID), s.p.ProfileTTL)
	b.HSetNX(countriesKey(tx.AccountID), tx.MerchantCountry, tx.ID)
	b.Expire(countriesKey(tx.AccountID), s.p.ProfileTTL)
}

func newest(amounts []int64, n int) []int64 {
	if len(amounts) <= n {
		return amounts
	}
	return amounts[len(amounts)-n:]
}

func unixMillis(t time.Time) float64 {
	return float64(t.UnixMilli())
}
