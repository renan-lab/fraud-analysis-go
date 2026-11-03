# AGENTS.md

## Project

Portfolio project for learning Go: fraud analysis of financial transactions. Simulated transaction events hit an ingestion API, are published to Kafka, and are analyzed asynchronously by a rule engine. Results and alerts are stored and exposed for querying.

Current stage: `cmd/api` implements ingestion (`POST /transactions` validates and publishes to Kafka `transactions.received`) plus `GET /health`, `/ready`, `/metrics`; `cmd/worker` runs `analysis` (rules + per-account baseline over Redis state, publishes `transactions.analyzed`); `cmd/simulator` (transaction traffic generator) is implemented; `cmd/evaluator` (`evaluation`) scores the decisions on `transactions.analyzed` against the simulator's ground truth. `alerts` (persistence and queries) is not built yet. Treat everything marked **planned** as direction, not existing code.

## Stack

- Go 1.27, stdlib first (`net/http` with method-based routing, `log/slog`, `testing`)
- Dependencies: `twmb/franz-go` (Kafka), `prometheus/client_golang` (metrics), `golang.org/x/time/rate` (rate limit), `redis/go-redis/v9` (Redis); see ADRs 0004, 0006, 0007, 0009
- PostgreSQL 17, Redis 8, Kafka 4 (KRaft, no Zookeeper)
- Docker Compose for local infra

## Architecture

Modular monolith: one Go module, binaries `cmd/api`, `cmd/worker`, `cmd/simulator` and `cmd/evaluator`, and modules `ingestion`, `analysis`, `alerts`, `simulator` and `evaluation` under `internal/` that communicate only through Kafka events (`internal/events`) and the ground-truth file (`internal/groundtruth`).

Read `ARCHITECTURE.md` before creating packages, binaries, topics or tables, or moving code between modules. It holds the codemap, boundaries and invariants.

## Commands

Run from the repository root. Targets live in `Makefile`; if `make` is unavailable, run the underlying command shown there.

- Setup env: `make env` (copies `.env.example` to `.env`)
- Run API locally: `make run` (needs infra up: `make infra`)
- Run worker locally: `make run-worker` (`go run ./cmd/worker`, needs `make infra`); in Docker it is the Compose service `worker` (same image as `app`, entrypoint `/worker`). It creates `transactions.received` and `transactions.analyzed`, pings Redis, and exits if either is unreachable within 15 s. Env vars: `WORKER_CONSUMER_GROUP`, `REDIS_ADDR` and the `ANALYSIS_*` rule settings; defaults and meaning in `.env.example`. A new group starts at the oldest record of `transactions.received`. Scale with `docker compose up -d --scale worker=N` (useful up to `KAFKA_TOPIC_PARTITIONS`); one worker owns each account at a time because the topic is keyed by `account_id` (ADR 0012). Records published with the old `transaction_id` key need `make clean` (wipes Kafka and Redis volumes) or a drained topic.
- Full stack in Docker: `make up` / `make down` / `make logs` / `make ps`
- Tests: `make test` (`go test -race ./...`); single test: `go test -race -run TestName ./internal/<pkg>/...`
- Integration tests (need `make infra`): `go test -race -tags=integration ./test/integration/...`. E2E (planned): `go test -race -tags=e2e ./test/e2e/...`
- Format: `make fmt`; static analysis: `make vet`
- Health/readiness/metrics: `curl localhost:8080/health`, `curl localhost:8080/ready` (503 when Kafka is unreachable), `curl localhost:8080/metrics`
- `cmd/api` needs Kafka at startup (creates `transactions.received`, exits after 15 s if unreachable). Env vars: `KAFKA_BROKERS`, `KAFKA_TOPIC_PARTITIONS`, `KAFKA_TOPIC_REPLICATION_FACTOR`, `INGEST_PUBLISH_TIMEOUT`, `INGEST_RATE_LIMIT_RPS` (0 = off), `INGEST_RATE_LIMIT_BURST`; defaults in `.env.example`.
- Simulator: `go run ./cmd/simulator scenarios` lists scenarios; `go run ./cmd/simulator run -h` lists flags. Dry run to stdout: `go run ./cmd/simulator run --count=50`. Against the API (`SIMULATOR_TARGET_URL`): `go run ./cmd/simulator run --target=http --rate=50 --duration=1m --truth=truth.ndjson`. Same `--seed` and flags reproduce the same traffic (except wall-clock timestamps).
- Evaluator (stack up, after a simulator run with `--truth`): `make evaluate` (`TRUTH=<file>`, default `truth.ndjson`) or `go run ./cmd/evaluator --truth=truth.ndjson` prints coverage, idempotency, per-transaction and per-sequence metrics, rule firings and false positive causes; `-h` lists flags. `--format=json` for scripts; `--min-recall`, `--min-sequence-recall`, `--max-fpr` exit 1 when violated. It rereads `transactions.analyzed` from the start without a consumer group (never commits) and stops when every accepted transaction has a decision and the topic is quiet for `--settle` (2s), or after `--wait` (30s).
- `-race` needs cgo and a C compiler (`gcc`); without it, `go test ./...` runs the tests without the race detector.

Ports: API `8080`, Postgres `5432`, Redis `6379`, Kafka `29092` from host / `kafka:9092` inside Compose.

## Conventions

Detailed rules live in `.cursor/rules/` (do not duplicate them here):

- `coding-standards.mdc`: style, package layout, errors, logging
- `testing.mdc`: test conventions
- `documentation.mdc`: which docs to update with each feature change
- `git-commits.mdc`: Conventional Commits in English

## Docs index

| Topic | Document |
|---|---|
| Setup and usage (Portuguese, for humans) | `README.md` |
| System map, HTTP contract, boundaries, invariants | `ARCHITECTURE.md` |
| Module context (flow, key files, gotchas) | `internal/<module>/README.md` |
| Why things are the way they are | `docs/decisions/` (ADRs) |

Non-obvious points:

- Prefer the stdlib. Add a third-party library only when the stdlib does not cover it well (e.g. Postgres, Kafka, Redis drivers).
- Config comes only from env vars via `internal/platform/config`. Never read `os.Getenv` elsewhere. Add new vars to `.env.example`.
- Never commit `.env` or real credentials.

## Learning mode

This is a learning project. After implementing, add a short note (1-3 bullets) explaining the Go idioms or particularities used (e.g. why an interface is defined on the consumer side, how `context` cancellation flows).

## Ask before

- Adding dependencies (`go get`)
- Creating commits
- Starting or stopping Docker containers
- Editing `docker-compose.yml`, `Dockerfile` or `Makefile`

## Before you finish

1. `make fmt`
2. `make vet`
3. `make test`
4. New behavior has a test; bug fixes have a regression test.
5. Docs updated in the same change, following `.cursor/rules/documentation.mdc`.
