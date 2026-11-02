# 0009. Use go-redis for analysis feature state with idempotent keys

- Status: accepted
- Date: 2026-11-02

## Context

The analysis rules need per-card and per-account history (recent purchases, last location, amount baseline, known devices and countries) that survives worker restarts and is shared by every worker replica. Kafka delivers at least once, so the same transaction can be processed twice, and a replay must not double count or change the decision. The stdlib has no Redis client.

## Decision

- Use `github.com/redis/go-redis/v9`, wrapped by `internal/platform/redis`. Modules see only `redis.Client`, `redis.Batch` and `redis.ZMember`, never go-redis types.
- All keys written by `analysis` start with `analysis:` (the module owns that namespace, like a Postgres schema):
  - `analysis:card:{card_id}:history`: ZSET, score = `occurred_at` in Unix ms, member = `txid|amount|channel|lat|lon` (lat/lon empty when unknown). Trimmed by event time to `ANALYSIS_HISTORY_RETENTION`.
  - `analysis:account:{account_id}:amounts`: ZSET, member = `txid|amount`, keeps the newest `ANALYSIS_BASELINE_SIZE + 1` entries.
  - `analysis:account:{account_id}:devices` and `...:countries`: HASH, `HSETNX field -> txid` (first transaction that used it).
- Members always contain `transaction_id`, so `ZADD` and `HSETNX` are no-ops on replay.
- Reads exclude the transaction being analyzed: a device or country is known only when the stored txid differs from the current one. Writes run after evaluation, in one `MULTI/EXEC`.

## Alternatives considered

- `valkey-io/valkey-go` or `rueidis`: faster, but a less familiar API and smaller example base for a learning project.
- In-memory state: lost on restart and not shared between replicas.
- Postgres for features: works, but range queries over sliding windows are the use case sorted sets were made for.

## Consequences

Rules see the same state on a replay, so decisions are repeatable. The trade-off: an out-of-order event recorded between two deliveries can still change a replayed decision. Changing a member format needs a migration or a key prefix bump. Malformed members are skipped on read instead of blocking the partition. Changing client library means rewriting only `internal/platform/redis`.
