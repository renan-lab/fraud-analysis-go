# Architecture

Last reviewed: 2026-11-02. Most of this document is **planned**: today only `cmd/api` with `GET /health`, `internal/config`, and `cmd/simulator` with `internal/simulator` exist. Update this file when a module boundary, binary, topic or data ownership changes.

## Overview

Simulated card transactions are sent to an ingestion API, published to Kafka, and analyzed asynchronously by a rule engine that scores each transaction and decides `approve`, `review` or `block`. Decisions are stored and exposed for querying.

Style: modular monolith. One Go module, several binaries, modules isolated by package boundaries and communicating through Kafka events.

## Runtime

```mermaid
flowchart LR
  sim["cmd/simulator (CLI)"] -->|"POST /transactions"| api["cmd/api: ingestion + alerts HTTP"]
  api -->|"publish transactions.received"| kafka[(Kafka)]
  kafka -->|consume| worker["cmd/worker: analysis + alerts consumer"]
  worker -->|"velocity counters"| redis[(Redis)]
  worker -->|"publish transactions.analyzed"| kafka
  worker -->|"persist decisions"| pg[(Postgres)]
  client[Client] -->|"GET /alerts"| api
  api -->|read| pg
```

Binaries (all in `cmd/`, each wires its modules in `main.go`):

| Binary | Runs | Talks to |
|---|---|---|
| `cmd/api` | `ingestion` HTTP handlers, `alerts` query handlers | Kafka (produce), Postgres (read) |
| `cmd/worker` | `analysis` consumer, `alerts` consumer | Kafka (consume/produce), Redis, Postgres |
| `cmd/simulator` | `simulator` event generator | `cmd/api` over HTTP only |

Topic names below are proposed; confirm before creating them.

| Topic | Producer | Consumer | Payload |
|---|---|---|---|
| `transactions.received` | `ingestion` | `analysis` | `events.TransactionReceived` |
| `transactions.analyzed` | `analysis` | `alerts` | `events.TransactionAnalyzed` |

## Ingestion HTTP contract

`POST /transactions` is the only contract `simulator` knows. Status: **proposed**; `internal/simulator` generates traffic against it and `ingestion` must implement it.

```json
{
  "transaction_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "account_id": "acc_3f9a1c2b7d4e",
  "card_id": "card_9b2c4d6e8f10",
  "amount": 12990,
  "currency": "BRL",
  "merchant": { "id": "mer_51a2b3c4d5e6", "name": "Grocery 5e6f", "mcc": "5411", "country": "BR" },
  "channel": "card_present",
  "device_id": "dev_0a1b2c3d4e5f",
  "ip": "177.34.10.200",
  "occurred_at": "2026-11-02T19:15:00Z"
}
```

| Field | Rule |
|---|---|
| `transaction_id` | UUID v4, lowercase. Idempotency key. |
| `account_id`, `card_id`, `device_id`, `merchant.id` | `^[A-Za-z0-9_-]{1,64}$` |
| `amount` | integer `> 0`, minor units of the card currency (cardholder billing amount). Floats and strings are rejected. |
| `currency` | ISO 4217, `^[A-Z]{3}$` |
| `merchant.name` | 1-128 printable characters, no control or format characters |
| `merchant.mcc` | `^[0-9]{4}$` |
| `merchant.country` | ISO 3166-1 alpha-2, `^[A-Z]{2}$` |
| `channel` | `card_present`, `ecommerce` or `contactless` |
| `ip` | IPv4 or IPv6 literal |
| `occurred_at` | RFC 3339; at most 5 minutes in the future (clock skew). Old events are accepted (late arrival). |

All fields are required; unknown fields are rejected. Responses:

| Status | When |
|---|---|
| `202 Accepted` | valid payload, including duplicates of an already accepted `transaction_id` (dedupe happens in consumers) |
| `400 Bad Request` | invalid JSON, wrong types, missing/unknown fields, any rule above violated |
| `413 Content Too Large` | body larger than 64 KiB |
| `415 Unsupported Media Type` | `Content-Type` is not `application/json` |

## Codemap

```
cmd/api/               HTTP entrypoint (exists)
cmd/worker/            Kafka consumers entrypoint
cmd/simulator/         CLI that generates simulated transactions (exists)
internal/ingestion/    validates incoming transactions, publishes transactions.received
internal/analysis/     consumes transactions.received, runs rules, publishes transactions.analyzed
internal/alerts/       consumes transactions.analyzed, persists decisions, serves queries
internal/simulator/    builds realistic/fraudulent/adversarial transaction scenarios (exists)
internal/events/       shared event contracts (types + topic names), no logic
internal/platform/     shared infrastructure: config, logging, postgres, kafka, redis
```

`internal/config` exists today and moves to `internal/platform/config` when `platform` is created.

Each module is a single package with files per layer:

- `handler.go` (HTTP) or `consumer.go` (Kafka): decode input, validate shape, call the service, map errors to responses.
- `service.go`: business logic. Depends on small interfaces declared in this package.
- `repository.go` / `publisher.go`: Postgres, Redis or Kafka adapters implementing those interfaces.
- `model.go`: domain types of the module.

In `analysis`, each rule lives in its own file `rule_<name>.go` and implements:

```go
type Rule interface {
	Name() string
	Evaluate(ctx context.Context, tx Transaction) (RuleResult, error)
}
```

`RuleResult` carries the rule name, a score and a human-readable reason. The service sums the scores of all rules and maps the total to a decision using thresholds from config. Rule parameters (limits, windows) come from config, never hardcoded.

`simulator` has no HTTP handler or repository, so its files are split by role instead:

- `population.go` / `catalog.go`: customers with stable habits (home country, spend, hours, devices, usual merchants) built from `--seed`.
- `scenario_<mode>.go`: each scenario implements `Scenario` and returns one or more `Emission`s (raw body + expected status + ground-truth label). Modes: `legit`, `fraud`, `malformed`, `attack`, `duplicate`, `timing`.
- `mix.go`: `Generator` picks a mode by `--mix` weight, then a scenario of that mode. Single goroutine, so output is reproducible per seed.
- `runner.go`: paces emissions (rate, ramp-up, burst) into a worker pool calling a `Sender` (`sender_http.go` or dry-run `sender_ndjson.go`); `report.go` and `truth.go` observe outcomes.

## Boundaries

1. Modules never import each other. Only `cmd/*` imports modules; modules share data only through `internal/events` contracts sent over Kafka.
2. `internal/events` and `internal/platform` never import modules.
3. Inside a module, dependencies go downward: handler/consumer -> service -> repository/publisher. Services never import `net/http`, Kafka or database client packages; they depend on interfaces.
4. Each module owns its own Postgres schema (e.g. `alerts`). No module reads or writes another module's tables. `ingestion` and `analysis` own no tables initially.
5. `simulator` only knows the public HTTP contract of the ingestion API.

## Invariants

- Kafka delivery is at-least-once. Every consumer MUST be idempotent, keyed by `transaction_id`. Commit offsets only after processing succeeds.
- `transaction_id` is the Kafka message key and the idempotency key. `alerts` upserts by `transaction_id`.
- Velocity counters in Redis must tolerate redelivery: use sorted sets with `transaction_id` as member and the event timestamp as score, so reprocessing does not double count.
- Ingestion responds `202 Accepted` only after the event is published to Kafka; publish failure returns `503`. Analysis results are never returned by the ingestion endpoint.
- Money is `int64` in minor units (cents) plus an ISO 4217 currency code. Never use floats for amounts.
- All timestamps are UTC (`time.Time`, RFC 3339 in JSON).
- Rule evaluation is deterministic for the same input and Redis state; rules have no side effects besides reading Redis.

## Cross-cutting concerns

- Config: env vars only, loaded once in `main.go` through `platform/config` and passed down via constructors.
- Logging: `log/slog` JSON. Include `transaction_id` on every log line related to a transaction.
- Context: every I/O call receives a `context.Context`; the root context is cancelled on SIGINT/SIGTERM.
- Shutdown: `cmd/api` drains HTTP with `Server.Shutdown`; `cmd/worker` stops fetching, finishes in-flight messages and commits offsets before exiting.
- Errors: wrapped with `%w`; handlers and consumers are the only places that translate errors into HTTP status codes or retry/skip decisions.

## Open decisions

- Postgres migrations tool and Kafka/Postgres/Redis client libraries.
- Retry and dead-letter strategy for messages that keep failing.
- Initial rule set and score thresholds.
- Compose services for `cmd/worker` (today only `app`, which runs `cmd/api`).
