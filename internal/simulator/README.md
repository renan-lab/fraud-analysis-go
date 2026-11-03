# simulator

## Purpose

Generates realistic and adversarial card transactions to exercise the ingestion API under real load. It knows only the public `POST /transactions` contract ([ARCHITECTURE.md](../../ARCHITECTURE.md#ingestion-http-contract)) and imports no other module.

## Entry points

- `go run ./cmd/simulator scenarios`: lists every scenario with its mode and expected status.
- `go run ./cmd/simulator run [flags]`: generates traffic; `run -h` lists flags. `--target=ndjson` (default) is a dry run to `--out`; `--target=http` posts to `SIMULATOR_TARGET_URL`.
- The report goes to stderr; `--truth=<file>` writes one ground-truth NDJSON line per send. The line format is `groundtruth.Record` (`internal/groundtruth`), read back by `cmd/evaluator`.

## Flow

`Population` (seeded customers) -> `Generator` (picks a mode by `--mix` weight, then a scenario) -> `Runner` (rate, ramp-up, burst, worker pool, causal order inside each scenario's batch) -> `Sender` (HTTP or NDJSON) -> `Observer`s (`Report`, `TruthWriter`).

## Key files

- `catalog.go`, `population.go`: countries (with reference-city coordinates), merchant categories and customers with stable habits (home country, spend, local hours, devices, usual merchants).
- `scenario.go`: `Scenario` interface and `Catalog()`. One file per mode: `scenario_legit.go`, `scenario_fraud.go`, `scenario_malformed.go`, `scenario_attack.go`, `scenario_duplicate.go`, `scenario_timing.go`.
- `model.go`: `Transaction` (contract payload) and `Emission` (raw body, expected status, label, copies). `Label` is an alias of `groundtruth.Label`.
- `mix.go`: `--mix` parsing and `Generator`.
- `runner.go`: pacing, worker pool, shutdown; declares `Sender`, `EmissionSource`, `Observer`. `runner_queue.go`: batch queue and the per-leg dependencies (`job`, `legSignal`).
- `sender_http.go`, `sender_ndjson.go`, `report.go`, `truth.go`.

## Invariants and gotchas

- Expected statuses mirror the contract. If the contract in `ARCHITECTURE.md` changes, update the scenarios and `contractStatus` in `helpers_test.go`; `TestScenariosExpectStatusDefinedByContract` checks every scenario against it.
- `Generator` runs in one goroutine with one `*rand.Rand`: same `--seed` and flags give the same traffic, except timestamps taken from the wall clock.
- `--time-scale > 1` starts simulated time `--lookback` in the past and never passes wall time. `timing/future` and `timing/within_skew` shift from wall time, because the API checks skew against its own clock.
- `429` (rate limit) and `503` (Kafka not acknowledging) depend on load and broker health, not on the payload, so no scenario expects them and `contractStatus` ignores them. They show up as expectation violations: run load tests with `INGEST_RATE_LIMIT_RPS=0` (the default) or a limit above `--rate`.
- Duplicates expect `202` (dedupe happens in consumers). `concurrent_retry` sends `Copies` identical requests, so the report's `sent` counts requests, not scenarios.
- `location` follows the contract meaning: in-person channels use the merchant country's reference city, `ecommerce` uses `customer.deviceCountry` and omits the block in 30% of purchases. Points get up to 0.25 degrees of seeded jitter (about 30 km), rounded to 4 decimals. Fraud scenarios with a new foreign device (`card_testing`, `foreign_new_device`) move `deviceCountry` to the fraud origin; `impossible_travel` and `cloned_card_concurrent` make both legs card present so both carry coordinates.
- Causal order: the simulator compresses time (legs minutes apart in `occurred_at` go out one tick apart), so without care a later leg could overtake an earlier one on the wire, which real traffic never does. An emission therefore waits until every earlier emission of its batch (one `Scenario.Next`) got a response, even an error. `Emission.Parallel` opts out for concurrency that is real: parallel legs wait only for the last sequential leg before them. `card_testing` fires its probes in parallel and the cash-out after all of them; `cloned_card_concurrent` sends both legs together. Batches never wait for each other, and pacing, `--rate`, `--count` and `Copies` are unchanged.
- The pool cannot deadlock: jobs are queued FIFO and only wait on earlier legs of their batch, which some worker already took (also with `--concurrency=1`).
- `fraud/cloned_card_concurrent` lets either leg reach the API first. `impossible_travel` only compares with earlier events ([ADR 0009](../../docs/decisions/0009-use-go-redis-for-analysis-feature-state.md)), so on a card with no earlier located purchase the far leg is missed whenever it lands first. Customers are reused across scenarios, so most cards already have a located purchase at home and the far leg is caught anyway; misses show up mostly early in a run. Do not "fix" the order in the simulator: the concurrency is the point.
- Dry-run sends have status 0: latency and expectation checks are skipped.
- Sends use `context.WithoutCancel`: Ctrl+C stops new emissions but lets in-flight requests finish before the report prints.

## Configuration

- `SIMULATOR_TARGET_URL` (default `http://localhost:8080`), read by `internal/platform/config`.
- Everything else is a `run` flag.

## Testing

`go test -race ./internal/simulator/... ./cmd/simulator/...`. HTTP is tested with `httptest`; the runner uses fakes (`recordingSender`, `staticSource`).

## Update when

A scenario, mode, flag, report field or ground-truth field is added or changed (a truth field change also touches `internal/groundtruth` and `internal/evaluation`), or the ingestion contract changes.
