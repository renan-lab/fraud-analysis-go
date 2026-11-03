# evaluation

## Purpose

Measures the analysis pipeline against the simulator's ground truth: did every accepted transaction get a decision, were redeliveries decided the same way, and how well do the rules separate fraud from legit. Reads the truth file (`internal/groundtruth`) and `transactions.analyzed`; imports no other module ([ADR 0011](../../docs/decisions/0011-evaluate-decisions-by-rereading-analyzed-topic-against-ground-truth.md)).

## Entry points

- `go run ./cmd/evaluator --truth=truth.ndjson` after `simulator run --target=http --truth=truth.ndjson`, with the stack up. `-h` lists flags.
- `ReadTruth`, `NewEvaluator(...).Evaluate`, `Report.WriteText` / `WriteJSON`, `Gates.Check`.

## Flow

`ReadTruth` (accepted, deduplicated by `transaction_id`) -> `Evaluator.Evaluate` -> `kafka.TopicReader.Read` (no group, from the oldest record) -> `newAnalyzedHandler` decodes -> `Collector.Observe` -> `quietWatch` cancels the read once complete and quiet for `Settle`, or `Wait` expires -> `BuildReport` (pure) -> writer -> `Gates.Check`.

## Key files

- `truth.go`, `collector.go`, `consumer.go`, `quiet.go`, `service.go`: reading and stop rule.
- `metrics_pipeline.go` (coverage, redelivery, divergence), `metrics_transaction.go` (confusion matrix, decisions per scenario and label), `metrics_sequence.go`, `metrics_rules.go`, `metrics_fpcause.go`.
- `report.go` (`Report`, `BuildReport`), `report_text.go`, `report_json.go`, `gate.go`.

## Invariants and gotchas

- Positive means `review` or `block`. Only `fraud` and `legit` labels enter the confusion matrix.
- Metrics use the first decision of a transaction; later ones only feed the divergence check (decision or score differs = broken idempotency).
- The topic holds every run: events outside the truth are ignored, so a stale topic is harmless, but the truth must come from a run whose events are still retained.
- Truth lines without `transaction_id` or with a status other than 202 are only counted (`not accepted`); the simulator report already checks their statuses.
- A fraud sequence is (scenario, `card_id`) over fraud-labeled transactions. The first purchases of a burst pass by design, so per-transaction recall is lower than sequence recall.
- `fraud-before-on-card`: a legit transaction flagged after a fraud on the same card (by `occurred_at`). The simulator still labels it legit; read these as label semantics, `clean-card` as real misfires.
- A scenario can mix labels (`impossible_travel` and `cloned_card_concurrent` send a legit purchase at home), so the scenario table has one row per scenario and label.
- `missing > 0` with a running worker usually means `--wait` was too short or the worker uses another topic.

## Configuration

`KAFKA_BROKERS` (via `internal/platform/config`). Everything else is a flag: `--truth`, `--wait` (30s), `--settle` (2s), `--format` (`text`|`json`), `--min-recall`, `--min-sequence-recall`, `--max-fpr` (0 = gate off). A failed gate prints `gate failed: ...` on stderr and exits 1.

## Testing

`go test -race ./internal/evaluation/... ./cmd/evaluator/...`. The reader is faked by `fakeRecordReader` (replays records, then blocks like a real topic); fixtures in `testdata/`. Against real Kafka: `go test -race -tags=integration -run TestEvaluatorReadsDecisionsFromTopic ./test/integration/...`.

## Update when

A metric, flag, gate or stop rule changes, or the `groundtruth.Record` or `transactions.analyzed` payload changes.
