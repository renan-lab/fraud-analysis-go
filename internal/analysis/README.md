# analysis

## Purpose

Scores every transaction from `transactions.received` and publishes `approve`, `review` or `block` to `transactions.analyzed`. Deterministic rules plus a per-account statistical baseline, no ML ([ADR 0010](../../docs/decisions/0010-score-transactions-with-pure-rules-over-preloaded-features.md)). State lives in Redis ([ADR 0009](../../docs/decisions/0009-use-go-redis-for-analysis-feature-state.md)).

## Entry points

- `go run ./cmd/worker` (needs `make infra`). It creates both topics, pings Redis, and joins consumer group `WORKER_CONSUMER_GROUP`.
- `Consumer.HandleRecord` is the `kafka.RecordHandler`; `Service.Analyze` is the business entry point.

## Flow

`consumer.go` decodes the record -> `FeatureStore.Load` (Redis reads, excludes the current `transaction_id`) -> every `Rule.Evaluate` (pure) -> `evaluateRules` sums fired scores -> `Thresholds.Decide` -> `FeatureStore.Record` (one `MULTI/EXEC`) -> `KafkaPublisher` -> `platform/kafka.Consumer` commits the offset.

## Key files

- `rule.go`: `Rule` interface. One file per rule: `rule_velocity.go`, `rule_card_testing.go`, `rule_impossible_travel.go`, `rule_amount_outlier.go`, `rule_new_device_foreign.go`.
- `model.go`: `Transaction`, `Features`, `CardEvent`, `RuleResult`, `Analysis`, `Decision`.
- `scoring.go`: `Thresholds`, `evaluateRules`. `stats.go` (median), `geo.go` (haversine).
- `featurestore_redis.go`: `RedisFeatureStore`; `featurestore_keys.go`: key names and member encodings.
- `service.go`: `Service`, `FeatureStore`, `AnalyzedPublisher`. `publisher.go`, `consumer.go`.
- Rule set wiring: `cmd/worker/rules.go`.

## Invariants and gotchas

- Rules never do I/O or read the clock. A new signal that needs state means extending `Features`, `Load` and `Record`, not querying Redis from the rule.
- Redelivery safety: `Load` skips the current `transaction_id`, `Record` only uses `ZADD` and `HSETNX` with `transaction_id` in the member/value, and the amounts ZSET keeps `BaselineSize + 1` entries. A replay gets the same decision unless another event for the same card/account was recorded in between.
- Windows use `occurred_at` (event time), not arrival time. `Load` reads card history in `[occurred_at - ANALYSIS_HISTORY_RETENTION, occurred_at]`, so a late event is compared with what happened before it.
- A device or country is "known" only when the hash stores a different txid. The first transaction of an account always scores new device + new country (45 by default), which stays below review on purpose.
- `amount_outlier` stays silent until the account has `ANALYSIS_OUTLIER_MIN_SAMPLES` past amounts. `impossible_travel` stays silent without a location on both sides.
- Undecodable records (bad JSON, unknown `event_type`/`schema_version`, no `transaction_id`) are logged and committed (no DLQ yet). Any other error is retried with backoff (100 ms doubling to 5 s) and never committed. On SIGINT the attempt in progress finishes within `ANALYSIS_PROCESS_TIMEOUT`.
- Malformed Redis members are skipped on read: less history instead of a stuck partition.
- Scores interact: tune `ANALYSIS_*` against `cmd/simulator --truth` output, not one rule at a time.

## Configuration

`WORKER_CONSUMER_GROUP`, `REDIS_ADDR`, `KAFKA_BROKERS`, `KAFKA_TOPIC_*` and every `ANALYSIS_*` var, documented with defaults in `.env.example` and loaded into `config.AnalysisConfig`.

## Testing

`go test -race ./internal/analysis/... ./cmd/worker/...`. Rules, scoring, stats and geo use plain table tests. The service uses `fakeFeatureStore` and `fakeAnalyzedPublisher`, the consumer uses `fakeAnalyzer`, and the store uses `fakeRedisCommands` for reads. Real Redis idempotency and the full Kafka -> Redis -> Kafka path: `go test -race -tags=integration ./test/integration/...`.

## Update when

A rule, threshold, env var, Redis key or member format, `Features` field or the `transactions.analyzed` payload changes.
