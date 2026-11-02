// Package redis wraps go-redis behind the small surface the project needs:
// sorted-set range reads, hash reads and atomic write batches.
package redis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Client is a Redis connection pool.
type Client struct {
	rdb *goredis.Client
}

// ZMember is one sorted-set entry.
type ZMember struct {
	Member string
	Score  float64
}

// NewClient connects lazily to addr; nothing is dialed until the first command.
//
//	client, err := redis.NewClient("localhost:6379")
func NewClient(addr string) (*Client, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("redis addr %q: want host:port", addr)
	}
	return &Client{rdb: goredis.NewClient(&goredis.Options{Addr: addr})}, nil
}

// Ping succeeds when the server answers PING.
func (c *Client) Ping(ctx context.Context) error {
	if err := c.rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	return nil
}

// ZRangeByScore returns members of key with min <= score <= max, lowest score
// first. Use math.Inf for open bounds.
//
//	members, err := client.ZRangeByScore(ctx, "k", math.Inf(-1), 1700000000000)
func (c *Client) ZRangeByScore(ctx context.Context, key string, min, max float64) ([]ZMember, error) {
	zs, err := c.rdb.ZRangeByScoreWithScores(ctx, key, &goredis.ZRangeBy{
		Min: formatScore(min), Max: formatScore(max),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("zrangebyscore %s: %w", key, err)
	}
	out := make([]ZMember, 0, len(zs))
	for _, z := range zs {
		member, _ := z.Member.(string)
		out = append(out, ZMember{Member: member, Score: z.Score})
	}
	return out, nil
}

// HGet returns the value of field in hash key and whether it exists.
func (c *Client) HGet(ctx context.Context, key, field string) (string, bool, error) {
	v, err := c.rdb.HGet(ctx, key, field).Result()
	if errors.Is(err, goredis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("hget %s %s: %w", key, field, err)
	}
	return v, true, nil
}

// Exec runs every command of b inside one MULTI/EXEC. An empty batch is a no-op.
func (c *Client) Exec(ctx context.Context, b *Batch) error {
	if b.Len() == 0 {
		return nil
	}
	_, err := c.rdb.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
		for _, cmd := range b.cmds {
			cmd(ctx, pipe)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("exec batch of %d commands: %w", b.Len(), err)
	}
	return nil
}

// Close releases the connection pool.
func (c *Client) Close() error {
	if err := c.rdb.Close(); err != nil {
		return fmt.Errorf("close redis: %w", err)
	}
	return nil
}

// formatScore writes infinities the way Redis range arguments expect them.
func formatScore(f float64) string {
	switch {
	case math.IsInf(f, -1):
		return "-inf"
	case math.IsInf(f, 1):
		return "+inf"
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// Batch collects write commands for Client.Exec.
type Batch struct {
	cmds []func(ctx context.Context, pipe goredis.Pipeliner)
}

// NewBatch returns an empty batch.
//
//	b := redis.NewBatch()
//	b.ZAdd("k", 1, "member")
//	err := client.Exec(ctx, b)
func NewBatch() *Batch {
	return &Batch{}
}

// Len is the number of queued commands.
func (b *Batch) Len() int {
	return len(b.cmds)
}

// ZAdd adds member with score; re-adding the same member only updates its score.
func (b *Batch) ZAdd(key string, score float64, member string) {
	b.add(func(ctx context.Context, p goredis.Pipeliner) {
		p.ZAdd(ctx, key, goredis.Z{Score: score, Member: member})
	})
}

// ZRemRangeByScore removes members with min <= score <= max.
func (b *Batch) ZRemRangeByScore(key string, min, max float64) {
	b.add(func(ctx context.Context, p goredis.Pipeliner) {
		p.ZRemRangeByScore(ctx, key, formatScore(min), formatScore(max))
	})
}

// ZKeepHighest keeps only the n members with the highest scores.
func (b *Batch) ZKeepHighest(key string, n int) {
	b.add(func(ctx context.Context, p goredis.Pipeliner) { p.ZRemRangeByRank(ctx, key, 0, int64(-n-1)) })
}

// HSetNX sets field to value only if field does not exist yet.
func (b *Batch) HSetNX(key, field, value string) {
	b.add(func(ctx context.Context, p goredis.Pipeliner) { p.HSetNX(ctx, key, field, value) })
}

// Expire sets the time to live of key.
func (b *Batch) Expire(key string, ttl time.Duration) {
	b.add(func(ctx context.Context, p goredis.Pipeliner) { p.Expire(ctx, key, ttl) })
}

func (b *Batch) add(cmd func(ctx context.Context, pipe goredis.Pipeliner)) {
	b.cmds = append(b.cmds, cmd)
}
