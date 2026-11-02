# AGENTS.md

## Project

Portfolio project for learning Go: fraud analysis of financial transactions. Simulated transaction events hit an ingestion API, are published to Kafka, and are analyzed asynchronously by a rule engine. Results and alerts are stored and exposed for querying.

Current stage: bootstrap only (`GET /health`). Business rules are not defined yet. Treat everything marked **planned** as direction, not existing code.

## Stack

- Go 1.27, stdlib first (`net/http` with method-based routing, `log/slog`, `testing`)
- PostgreSQL 17, Redis 8, Kafka 4 (KRaft, no Zookeeper)
- Docker Compose for local infra

## Architecture

Modular monolith: one Go module, binaries `cmd/api`, `cmd/worker` and `cmd/simulator`, and modules `ingestion`, `analysis`, `alerts` and `simulator` under `internal/` that communicate only through Kafka events.

Read `ARCHITECTURE.md` before creating packages, binaries, topics or tables, or moving code between modules. It holds the codemap, boundaries and invariants.

## Commands

Run from the repository root. Targets live in `Makefile`; if `make` is unavailable, run the underlying command shown there.

- Setup env: `make env` (copies `.env.example` to `.env`)
- Run API locally: `make run` (needs infra up: `make infra`)
- Full stack in Docker: `make up` / `make down` / `make logs` / `make ps`
- Tests: `make test` (`go test -race ./...`); single test: `go test -race -run TestName ./internal/<pkg>/...`
- Format: `make fmt`; static analysis: `make vet`
- Health check: `curl localhost:8080/health`

Ports: API `8080`, Postgres `5432`, Redis `6379`, Kafka `29092` from host / `kafka:9092` inside Compose.

## Conventions

Detailed rules live in `.cursor/.rules/` (do not duplicate them here):

- `coding-standards.mdc`: style, package layout, errors, logging
- `testing.mdc`: test conventions
- `git-commits.mdc`: Conventional Commits in English

Non-obvious points:

- Prefer the stdlib. Add a third-party library only when the stdlib does not cover it well (e.g. Postgres, Kafka, Redis drivers).
- Config comes only from env vars via `internal/config`. Never read `os.Getenv` elsewhere. Add new vars to `.env.example`.
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
5. Update this file if commands changed, and `ARCHITECTURE.md` if structure or module boundaries changed.
