# 0003. Colocate unit tests, gate integration tests

- Status: accepted
- Date: 2026-11-02

## Context

Some layouts put tests in a separate tree. In Go, a test in another directory is another package: it sees only exported identifiers, and `go test ./internal/<module>` does not run it.

## Decision

Unit tests live next to the code as `<file>_test.go` in the same package, with fixtures in `testdata/`. Tests that cross modules or need real infra live in `test/integration/` and `test/e2e/`, behind the `integration` and `e2e` build tags. Details in `.cursor/rules/testing.mdc`.

## Alternatives considered

- Black-box `package <module>_test` by default with `export_test.go`: stricter, but most current tests exercise unexported scenario and report logic.
- A separate `tests/` folder per module: would force exporting internals only for tests.

## Consequences

`go test ./...` stays fast and needs no infra. Integration tests can only use what another module could use, so they also check module boundaries.
