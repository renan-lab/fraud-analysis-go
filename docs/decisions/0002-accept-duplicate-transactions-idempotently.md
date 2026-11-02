# 0002. Accept duplicate transactions idempotently

- Status: accepted
- Date: 2026-11-02

## Context

Clients retry, and the simulator deliberately resends transactions (exact replays and concurrent retries). Kafka delivery is at-least-once, so consumers see duplicates anyway.

## Decision

`POST /transactions` answers `202 Accepted` for a duplicate `transaction_id`, the same as for the first request. Ingestion keeps no state to detect duplicates; consumers dedupe by `transaction_id` (see Invariants in `ARCHITECTURE.md`). The simulator's duplicate scenarios expect `202`.

## Alternatives considered

- `409 Conflict` from ingestion: needs a shared store (e.g. Redis) on the request path and still cannot replace consumer idempotency, since Kafka redelivers.

## Consequences

Ingestion stays stateless and simple. Every consumer must be idempotent, and tests must cover replays. A duplicate with a new `transaction_id` is a different transaction by contract (a possible double charge), which analysis rules may flag.
