# 0006. Problem details errors and a global rate limit

- Status: accepted
- Date: 2026-11-02

## Context

Clients (and the simulator report) need machine-readable reasons for rejections, with every invalid field listed. The ingestion endpoint also needs a cheap guard against floods before the body is read.

## Decision

- Every error response is `application/problem+json` (RFC 9457): `type` (relative URI such as `/problems/validation`), `title`, `status`, `detail`, `instance`, plus extensions `request_id` and `errors: [{field, detail}]`. Validation collects all violations, not only the first.
- `POST /transactions` has a process-wide token bucket (`golang.org/x/time/rate`) configured by `INGEST_RATE_LIMIT_RPS` and `INGEST_RATE_LIMIT_BURST`; `0` disables it (default, so simulator load tests are not throttled). Excess requests get `429` with `Retry-After`.

## Alternatives considered

- Plain `{"error": "..."}` body: ad hoc shape, no standard media type.
- Per-IP limiter: needs a map with eviction and trust rules for proxies; the only client today is the simulator.
- Hand-written token bucket: `x/time/rate` is maintained by the Go team and already correct under concurrency.

## Consequences

`429` and `503` depend on load and broker health, not on the payload, so simulator scenarios never expect them and report them as violations. The limit is per process: N replicas allow N times the configured rate.
