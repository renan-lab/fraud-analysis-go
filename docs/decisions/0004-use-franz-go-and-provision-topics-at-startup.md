# 0004. Use franz-go and provision topics at startup

- Status: accepted
- Date: 2026-11-02

## Context

`cmd/api` must publish to Kafka and answer `202` only after the broker acknowledged the event. The stdlib has no Kafka client: the protocol is a binary TCP protocol with leader discovery, record batches, CRC32C, compression, acks and idempotent producer sequences. The local broker auto-creates topics, but with broker defaults (1 partition) and only on first use.

## Decision

- Use `github.com/twmb/franz-go` (`kgo`) for producing and `kadm` for admin calls, wrapped by `internal/platform/kafka`. Modules see only `kafka.Message` and a small interface, never `kgo` types.
- Producer: `acks=all`, idempotent writes (franz-go default), default 10 ms linger. `ProduceSync` blocks the HTTP request until the ack or the context deadline (`INGEST_PUBLISH_TIMEOUT`).
- `cmd/api` creates `transactions.received` at startup with `KAFKA_TOPIC_PARTITIONS` and `KAFKA_TOPIC_REPLICATION_FACTOR`, ignoring "already exists", and exits if Kafka does not answer within 15 s.

## Alternatives considered

- `segmentio/kafka-go`: simpler API, but no idempotent producer and slower maintenance.
- `confluent-kafka-go`: mature, but needs cgo (librdkafka), which complicates the distroless image and cross-compiling.
- Kafka REST Proxy with `net/http`: one more container and hides the client behavior this project wants to learn.
- Relying on broker auto-create: partition count would silently be the broker default.

## Consequences

Startup depends on Kafka being reachable. An existing topic keeps its partition count even if the env var changes; changing partitions requires a manual `kafka-topics.sh --alter`. Changing client library means rewriting only `internal/platform/kafka`.
