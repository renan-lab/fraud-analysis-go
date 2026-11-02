# 0011. Evaluate decisions by rereading the analyzed topic against ground truth

- Status: accepted
- Date: 2026-11-02

## Context

Tuning `ANALYSIS_*` needs numbers: which fraud scenarios are caught, how many legit purchases are flagged and why. The first evaluation was a set of throwaway Python scripts joining `truth.ndjson`, worker logs and `transactions.received`. The truth line format was a private struct of `simulator`, and modules may not import each other (ARCHITECTURE.md, Boundaries), so nothing else could read it in Go.

## Decision

- The truth file format is a contract package, `internal/groundtruth` (`Record`, `Label`, `StatusAccepted`), types only, like `internal/events`. `simulator` writes it (`simulator.Label` is an alias of `groundtruth.Label`); `evaluation` reads it. Its JSON shape is locked by a golden test.
- `cmd/evaluator` (module `internal/evaluation`) evaluates online: it joins the truth with `transactions.analyzed`, which already carries `card_id`, `occurred_at`, `decision`, `score` and `rule_results`. It does not read logs or `transactions.received`.
- The topic is read with `platform/kafka.TopicReader`: no consumer group, no commits, always from the oldest record. Events whose `transaction_id` is not in the truth (older runs) are ignored.
- The read stops when every accepted transaction has a decision and the topic then stays quiet for `--settle` (default 2 s), or when `--wait` (default 30 s) expires. The settle exists because stopping at the first complete moment hid redeliveries published right after, which the idempotency check must see.
- A transaction is positive when the decision is `review` or `block`. Besides per-transaction metrics, a fraud sequence (the fraud transactions of one scenario on one card) counts as detected if any of them was flagged.
- Optional gates (`--min-recall`, `--min-sequence-recall`, `--max-fpr`) make the command exit 1, for CI.

## Alternatives considered

- Offline evaluation (generate traffic in memory and run the rules without Kafka and Redis): faster for tuning, but it does not validate the pipeline (coverage, redelivery) and would couple `evaluation` to `simulator` and `analysis` internals.
- Consumer group for the evaluator: committed offsets would make the second run skip everything already read.
- Reading up to the high watermark: needs end offsets per partition (`kadm`, a new module) and still races with a worker that is catching up.
- `evaluation` declaring its own copy of the truth struct: two definitions of one file format drift silently.

## Consequences

- Adding or renaming a truth field is a contract change: update `internal/groundtruth`, its golden file and both modules.
- Evaluation needs the stack running (`make up` or `make infra` + `make run` + `make run-worker`) and a truth file from the same run.
- Each run rereads the whole `transactions.analyzed` topic; cost grows with topic retention, not with the run size.
- Per-transaction recall stays low by design (rules need evidence to accumulate); use sequence recall to judge whether attacks are stopped.
