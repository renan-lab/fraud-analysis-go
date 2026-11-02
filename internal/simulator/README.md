# simulator

## Purpose

Generates realistic and adversarial card transactions to exercise the ingestion API under real load. It knows only the public `POST /transactions` contract ([ARCHITECTURE.md](../../ARCHITECTURE.md#ingestion-http-contract)) and imports no other module.

## Entry points

- `go run ./cmd/simulator scenarios`: lists every scenario with its mode and expected status.
- `go run ./cmd/simulator run [flags]`: generates traffic; `run -h` lists flags. `--target=ndjson` (default) is a dry run to `--out`; `--target=http` posts to `SIMULATOR_TARGET_URL`.
- The report goes to stderr; `--truth=<file>` writes one ground-truth NDJSON line per send.

## Flow

`Population` (seeded customers) -> `Generator` (picks a mode by `--mix` weight, then a scenario) -> `Runner` (rate, ramp-up, burst, worker pool) -> `Sender` (HTTP or NDJSON) -> `Observer`s (`Report`, `TruthWriter`).

## Key files

- `catalog.go`, `population.go`: countries, merchant categories and customers with stable habits (home country, spend, local hours, devices, usual merchants).
- `scenario.go`: `Scenario` interface and `Catalog()`. One file per mode: `scenario_legit.go`, `scenario_fraud.go`, `scenario_malformed.go`, `scenario_attack.go`, `scenario_duplicate.go`, `scenario_timing.go`.
- `model.go`: `Transaction` (contract payload) and `Emission` (raw body, expected status, label, copies).
- `mix.go`: `--mix` parsing and `Generator`.
- `runner.go`: pacing, worker pool, shutdown; declares `Sender`, `EmissionSource`, `Observer`.
- `sender_http.go`, `sender_ndjson.go`, `report.go`, `truth.go`.

## Invariants and gotchas

- Expected statuses mirror the contract. If the contract in `ARCHITECTURE.md` changes, update the scenarios and `contractStatus` in `helpers_test.go`; `TestScenariosExpectStatusDefinedByContract` checks every scenario against it.
- `Generator` runs in one goroutine with one `*rand.Rand`: same `--seed` and flags give the same traffic, except timestamps taken from the wall clock.
- `--time-scale > 1` starts simulated time `--lookback` in the past and never passes wall time. `timing/future` and `timing/within_skew` shift from wall time, because the API checks skew against its own clock.
- Duplicates expect `202` (dedupe happens in consumers). `concurrent_retry` sends `Copies` identical requests, so the report's `sent` counts requests, not scenarios.
- Dry-run sends have status 0: latency and expectation checks are skipped.
- Sends use `context.WithoutCancel`: Ctrl+C stops new emissions but lets in-flight requests finish before the report prints.

## Configuration

- `SIMULATOR_TARGET_URL` (default `http://localhost:8080`), read by `internal/config`.
- Everything else is a `run` flag.

## Testing

`go test -race ./internal/simulator/... ./cmd/simulator/...`. HTTP is tested with `httptest`; the runner uses fakes (`recordingSender`, `staticSource`).

## Update when

A scenario, mode, flag, report field or ground-truth field is added or changed, or the ingestion contract changes.
