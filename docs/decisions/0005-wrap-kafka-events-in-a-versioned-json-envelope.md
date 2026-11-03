# 0005. Wrap Kafka events in a versioned JSON envelope

- Status: accepted; the key of `transactions.received` is superseded by [0012](0012-partition-transactions-by-account-id.md)
- Date: 2026-11-02

## Context

`transactions.received` is the first contract between modules. Consumers need to trace an event back to the HTTP request, tell events apart from transactions (duplicates are published again, ADR 0002), and survive payload evolution.

## Decision

- Value: JSON `events.TransactionReceived` = `event_id` (UUID v4, new per publish), `event_type` (`transaction.received`), `schema_version` (integer, `1`), `received_at` (UTC), `request_id` and the validated `transaction` with the same field names as the HTTP contract.
- Key: `transaction_id`, so all events of a transaction land on the same partition, in order.
- Headers: `event_type`, `schema_version`, `request_id`, so consumers can route or skip without decoding the value.
- `schema_version` changes only on breaking changes; adding optional fields keeps it. `internal/events/testdata/transaction_received.golden.json` locks the wire format.

## Alternatives considered

- Plain JSON payload (HTTP body plus `received_at`): no way to version or trace events.
- Protobuf or Avro: smaller and schema-checked, but adds code generation or a schema registry for a single-module consumer.

## Consequences

Consumers dedupe by `transaction.transaction_id`, never by `event_id`. Every contract change must update the golden file and, if breaking, bump `schema_version`.
