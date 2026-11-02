# 0007. Expose Prometheus metrics

- Status: accepted
- Date: 2026-11-02

## Context

Load tests with the simulator need server-side numbers (request rate by status, latency, Kafka ack time) to compare with the client-side report.

## Decision

`cmd/api` serves `GET /metrics` in Prometheus text format using `github.com/prometheus/client_golang` with a private registry (no global state). Metrics: `http_requests_total{route,method,status}`, `http_request_duration_seconds{route,method}`, `ingestion_publish_duration_seconds{result}`, plus Go runtime and process collectors. Route labels are fixed strings set at wiring time, never raw URL paths.

## Alternatives considered

- `expvar` (stdlib): no dependency, but JSON at `/debug/vars` without histograms or labels, not scrapeable by Prometheus without an exporter.
- OpenTelemetry: more general, much larger dependency surface for one binary.

## Consequences

New metrics are registered on the registry passed through constructors. `/metrics` is unauthenticated; do not expose the port publicly.
