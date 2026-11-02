# 0010. Score transactions with pure rules over preloaded features

- Status: accepted
- Date: 2026-11-02

## Context

The fraud engine could be a trained ML model or hand-written rules. The only labeled data is synthetic (`cmd/simulator --truth`), so a model would mostly relearn the simulator's generator, and serving a model from Go needs cgo or a sidecar. Decisions also need a human-readable reason. `ARCHITECTURE.md` first sketched rules as `Evaluate(ctx, tx)`, which lets every rule read Redis on its own.

## Decision

- Rules are deterministic Go code plus per-account statistics (median of recent amounts), no ML for now.
- A rule is pure: `Evaluate(tx Transaction, f Features) RuleResult`. `FeatureStore.Load` reads all state once before evaluation; `FeatureStore.Record` writes the transaction after it. Rules never do I/O.
- Scoring is additive: the service sums the scores of the rules that fired; `>= ANALYSIS_BLOCK_THRESHOLD` is `block`, `>= ANALYSIS_REVIEW_THRESHOLD` is `review`, else `approve`. Rule limits, windows and scores come from `ANALYSIS_*` env vars.
- Only rules that fired are kept in `TransactionAnalyzed.rule_results`.
- A future ML scorer (**planned**) is one more `Rule` (`rule_model.go`) reading the same `Features`, so the pipeline does not change.

## Alternatives considered

- ML first: see context; opaque decisions and heavy serving for synthetic data.
- Rules that query Redis themselves: N round trips per transaction, harder to test (every rule needs a fake), and evaluation order could leak writes into reads.
- Weighted model or max-of-scores instead of a sum: harder to explain than "these rules added up to N".
- Rules as CEL expressions in config: flexible, but a dependency and an indirection before there are many rules.

## Consequences

Rules are tested with plain table tests, no fakes. Adding a rule that needs new state means extending `Features` and the store, not only writing the rule. Thresholds and scores interact: changing one rule's score can move decisions of other scenarios, so tune them against the simulator's ground truth.
