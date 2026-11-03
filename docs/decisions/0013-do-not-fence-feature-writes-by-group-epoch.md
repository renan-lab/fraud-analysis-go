# 0013. Do not fence feature writes by group epoch

- Status: accepted
- Date: 2026-11-03

## Context

[ADR 0012](0012-partition-transactions-by-account-id.md) accepts that a record stuck in retry (Redis or Kafka down) longer than the rebalance timeout (60 s) gets the worker kicked from the group, and that its last write to Redis may overlap with the new owner of the partition. It listed two fixes as planned: epoch fencing per partition in Redis and aborting the retry loop when a rebalance is waiting.

The overlap is bounded today:

- `platform/kafka.Consumer` commits synchronously after each record. The kicked worker fails its next commit and exits, so it handles at most one record after losing the partition.
- The new owner starts at the committed offset, so it records that same transaction before any later one of the account, and its `Load` of later transactions sees it.
- `RedisFeatureStore.Record` only uses `ZADD` with `transaction_id` in the member, `HSETNX`, trims by score or rank and `EXPIRE`. Writing a transaction again, interleaved with writes of later transactions of the same account, leaves the same state.
- `Load` excludes the transaction being analyzed and reads only up to its `occurred_at`, so later transactions do not leak into the zombie's decision through the card history or the amounts.

## Decision

- Neither epoch fencing nor aborting the retry loop on a waiting rebalance is planned. The overlap described in ADR 0012 stays accepted.
- It is safe only while two invariants hold:
  - Feature writes in `Record` are idempotent and commutative. No overwrites (`SET`, `HSET`), no counters (`INCR`, `HINCRBY`), no value computed from `Load` and written back.
  - Offsets of `transactions.received` are committed synchronously, one record at a time, right after the handler succeeds. No batch, async or `AutoCommitMarks` commits.

## Alternatives considered

- Epoch fencing per partition: a Lua script stores the group generation per partition and rejects `Record` calls from an older one. Needs hash tags so card, account and fence keys share a Redis Cluster slot, leaks the partition and generation into `analysis`, runs a script on every write, and still lets the zombie publish if it records before the new owner bumps the epoch.
- Yield on `OnPartitionsCallbackBlocked`: cancel the batch, rewind the uncommitted partitions with `SetOffsets` and allow the rebalance. Removes the 60 s stall, but franz-go does not redeliver on its own, so a wrong rewind silently skips records on partitions the member keeps.
- Exit the process when a rebalance is waiting during a retry: leaves the group at once and the new owner restarts from the committed offset, at the cost of a container restart per overlapping rebalance.
- Kafka transactions (exactly-once) to stop the duplicate decision; heavier than the problem.

## Consequences

- A kicked worker may publish a second decision, possibly different, for one transaction to `transactions.analyzed`. Consumers already upsert by `transaction_id` ([ADR 0002](0002-accept-duplicate-transactions-idempotently.md), [ADR 0009](0009-use-go-redis-for-analysis-feature-state.md)).
- A worker stuck in retry still holds rebalances (deploy, scale) for up to 60 s, is then kicked, exits and is restarted by Compose.
- Revisit this decision, and fence or yield, when any invariant above breaks, when records of one partition are handled concurrently, or when something other than `analysis` writes the feature keys.
