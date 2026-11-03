# ingestion

## Purpose

Implements `POST /transactions` ([contract](../../ARCHITECTURE.md#ingestion-http-contract)): rejects anything off-contract and publishes each valid transaction to `transactions.received` as an `events.TransactionReceived` envelope, answering `202` only after Kafka acknowledged it. Keeps no state and imports no other module.

## Entry points

- `NewHandler(service, clock, logger)`: the `http.Handler` mounted by `cmd/api/routes.go` behind rate limit, metrics, request ID, access log and recover middlewares.
- `NewService(publisher, clock, newID, publishTimeout)` and `NewKafkaPublisher(producer, topic, registry)`: wired in `cmd/api`.

## Flow

`Handler` -> `decodeTransaction` (415, 413, strict JSON -> 400) -> `transactionRequest.toTransaction` (field rules -> 400 listing every violation) -> `Service.Ingest` (envelope + publish timeout) -> `Publisher` -> `KafkaPublisher` -> `platform/kafka` (`ProduceSync`, acks=all). Publish error or timeout -> `ErrPublishFailed` -> 503.

## Key files

- `model.go`: domain `Transaction`, raw `transactionRequest` (pointer fields), `Clock`.
- `decode.go`: content type, body limit, strict decoding, decode error classification.
- `validate.go`: contract rules, one `FieldError` per violation.
- `handler.go`: HTTP status and problem+json mapping.
- `service.go`: `Publisher` interface, envelope building, `ErrPublishFailed`.
- `publisher.go`: Kafka adapter (key, headers, `ingestion_publish_duration_seconds`).

## Invariants and gotchas

- Rules mirror the contract table in `ARCHITECTURE.md` and `contractStatus` in `internal/simulator/helpers_test.go`; change all three together.
- `transactionRequest` uses pointers so a missing field ("is required") differs from a zero value (`amount: 0` -> "must be > 0"). `Amount *int64` makes floats, strings and int64 overflow fail at decode time.
- `location` is the only optional field: absent or `null` -> `Transaction.Location == nil`, and the event omits it (`omitempty` on a pointer). Once the object is sent, `lat` and `lon` are both required. Coordinates are `*float64`, so a string fails at decode time ("a number in decimal degrees") and out-of-range values fail in `validate.go`. `0,0` is a valid point and accepted.
- `occurred_at` skew (5 min) is checked against the API clock; any past timestamp is accepted. It is stored in UTC.
- The body limit is enforced twice: `Content-Length` up front and `http.MaxBytesReader` while streaming (chunked bodies). Trailing data is read through the same limit.
- Error details echo at most 64 runes of the offending value; problem bodies are JSON-encoded, so hostile input cannot break the response.
- Duplicates are published again on purpose ([ADR 0002](../../docs/decisions/0002-accept-duplicate-transactions-idempotently.md)). A client disconnect cancels the publish context; the event may or may not land, and a retry is safe.
- Events are keyed by `account_id`, not `transaction_id`: one account maps to one partition, so a single analysis worker scores its transactions in order ([ADR 0012](../../docs/decisions/0012-partition-transactions-by-account-id.md)). Do not change the key without revisiting that ADR.
- `Service` never imports `net/http` or Kafka packages; `publisher.go` is the only file touching `platform/kafka`.

## Configuration

`INGEST_PUBLISH_TIMEOUT` (default `5s`, keep below the server `WriteTimeout` of 10 s), `INGEST_RATE_LIMIT_RPS` (default `0` = off), `INGEST_RATE_LIMIT_BURST` (default `100`), `KAFKA_BROKERS`, `KAFKA_TOPIC_PARTITIONS`, `KAFKA_TOPIC_REPLICATION_FACTOR`. All read by `internal/platform/config`.

## Testing

- Unit: `go test -race ./internal/ingestion/...`. `fakePublisher`, `blockingPublisher` and `fakeProducer` in `helpers_test.go`; handler tests cover every malformed and attack scenario of the simulator.
- Integration (needs `make infra`): `go test -race -tags=integration ./test/integration/...` posts through the handler and consumes the record from a per-test topic.
- End to end: run the API, then `go run ./cmd/simulator run --target=http`; the report must show `expectation violations: none` (with the rate limit off).

## Update when

The HTTP contract, a validation rule, the event envelope, a problem `type`, or an ingestion env var changes.
