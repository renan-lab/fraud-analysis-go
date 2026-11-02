# 0001. Record architecture decisions

- Status: accepted
- Date: 2026-11-02

## Context

Humans and AI agents work on this repository across sessions. The reasons behind expensive-to-reverse choices get lost, and agents tend to "tidy away" things that look odd but are intentional.

## Decision

Record such choices as numbered Markdown ADRs in `docs/decisions/`, from `_template.md`, indexed in `README.md`. Accepted ADRs are append-only: a change of mind is a new ADR that supersedes the old one. The rule `.cursor/rules/documentation.mdc` says when to write one.

## Alternatives considered

- Only the "Open decisions" list in `ARCHITECTURE.md`: keeps what is undecided, not why something was decided.
- Commit messages: not discoverable and too short for alternatives and consequences.

## Consequences

A future reader (or agent) can find why something is the way it is before changing it. Writing an ADR becomes part of finishing a change that makes such a decision.
