# 0012. Partition transactions.received by account_id

- Status: accepted
- Date: 2026-11-03

## Context

`analysis.Service.Analyze` loads the card and account state from Redis, evaluates the rules, then records the transaction ([ADR 0009](0009-use-go-redis-for-analysis-feature-state.md)). Load and Record are separate round trips. `transactions.received` was keyed by `transaction_id`, so two purchases of one account could land on different partitions. With several `cmd/worker` replicas in one group, they could be scored at the same time, and each Load could run before the other's Record. Both decisions would then ignore the other purchase, which silences `velocity`, `card_testing` and `impossible_travel` (and skews the account baseline) exactly when a card is used twice in quick succession.

A consumer group rebalance has the same effect on a smaller scale: by default franz-go can move a partition to another member while the old one is still handling a record from it.

## Decision

- `ingestion` keys `transactions.received` by `account_id`, superseding the key of [ADR 0005](0005-wrap-kafka-events-in-a-versioned-json-envelope.md) for that topic. Every event of one account goes to one partition, and a partition has one owner per group, so one worker scores an account's transactions sequentially, in publish order. `account_id` covers both the card keys and the account keys in Redis (a card belongs to one account).
- `transaction_id` stays the idempotency key, read from the payload. Duplicates of a transaction share the account, so they still share a partition.
- `platform/kafka.Consumer` uses `kgo.BlockRebalanceOnPoll`, polls at most 100 records (`PollRecords`) and calls `AllowRebalance` after each batch. A partition never moves mid-batch, and commits are only for partitions the member still owns. A blocked rebalance is logged as a warning.
- `transactions.analyzed` keeps `transaction_id` as its key. Its consumers (`evaluation`, planned `alerts`) upsert by transaction and keep no per-account state, and the key spreads decisions evenly. A future consumer that keeps per-account state from decisions (for example "block the account after N blocks") would need the same change on that topic, through a new ADR.
- `KAFKA_TOPIC_PARTITIONS` stays at 6. It is the upper bound on useful worker replicas.
- Records of one partition must keep being handled one at a time. Parallelizing inside a partition would bring the race back.

## Alternatives considered

- Key by `card_id`: protects the card rules, but the account keys (amount baseline, known devices and countries) would still race across cards of one account.
- Optimistic concurrency in Redis (`WATCH` + `MULTI/EXEC`, reload and retry on abort): correct regardless of partitioning, but each transaction pins a pool connection and pays extra round trips, retries pile up on the hot card during exactly the attacks the rules target, Redis Cluster would need hash tags for all keys of an account, and it does not order events. Kept as an option for defense in depth if something else ever writes these keys.
- Distributed lock per account (`SET NX PX`): a lease without a fencing token is unsafe when a GC pause or `ANALYSIS_PROCESS_TIMEOUT` outlives the TTL, and it adds latency, contention and lock ordering between card and account.
- Record before Load: at least one of two concurrent purchases sees the other, but only if the read window also looks past `occurred_at`, which changes the meaning of the velocity window, and `HSETNX` would mark a concurrently seen device as known.

## Consequences

- An account under attack concentrates its load on one partition. Watch consumer lag per partition.
- Changing the partition count remaps `hash(account_id) % N`. During the switch, old records on the previous partition and new ones on the next can be scored concurrently, so size the topic with headroom and drain the lag before resizing.
- Migration from the old key: stop producing, let the worker drain `transactions.received`, then deploy. Locally, `make clean` resets Kafka and Redis.
- A record stuck in retry (Redis or Kafka down) longer than the rebalance timeout (60 s) gets the member kicked from the group. Its next commit fails and the worker exits, but its last write to Redis may overlap with the new owner. Epoch fencing per partition in Redis (a Lua script that rejects writes from an older group generation) and aborting the retry loop when a rebalance is waiting are **planned**.
