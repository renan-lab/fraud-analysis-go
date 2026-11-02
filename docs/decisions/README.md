# Architecture decision records

Append-only log of why the system is the way it is. Write a new ADR from [_template.md](_template.md) when a choice is expensive to reverse; never rewrite an accepted one, supersede it.

| ADR | Decision | Status |
|---|---|---|
| [0001](0001-record-architecture-decisions.md) | Record architecture decisions | accepted |
| [0002](0002-accept-duplicate-transactions-idempotently.md) | Accept duplicate transactions idempotently | accepted |
| [0003](0003-colocate-unit-tests-gate-integration-tests.md) | Colocate unit tests, gate integration tests | accepted |
| [0004](0004-use-franz-go-and-provision-topics-at-startup.md) | Use franz-go and provision topics at startup | accepted |
| [0005](0005-wrap-kafka-events-in-a-versioned-json-envelope.md) | Wrap Kafka events in a versioned JSON envelope | accepted |
| [0006](0006-problem-details-errors-and-global-rate-limit.md) | Problem details errors and a global rate limit | accepted |
| [0007](0007-expose-prometheus-metrics.md) | Expose Prometheus metrics | accepted |
| [0008](0008-add-optional-transaction-geolocation.md) | Add optional transaction geolocation | accepted |
| [0009](0009-use-go-redis-for-analysis-feature-state.md) | Use go-redis for analysis feature state with idempotent keys | accepted |
| [0010](0010-score-transactions-with-pure-rules-over-preloaded-features.md) | Score transactions with pure rules over preloaded features | accepted |
| [0011](0011-evaluate-decisions-by-rereading-analyzed-topic-against-ground-truth.md) | Evaluate decisions by rereading the analyzed topic against ground truth | accepted |
