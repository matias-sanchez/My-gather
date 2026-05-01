# Implementation Plan: Spec And Code Reconciliation

**Branch**: `009-spec-code-reconciliation` | **Date**: 2026-05-01 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/009-spec-code-reconciliation/spec.md`

## Summary

Reconcile Spec Kit state with the shipped repository by aligning active feature
pointers, correcting completed feature statuses, clarifying historical task
tracking, removing stale placeholders, and adding the missing archive-input CLI
tests. No runtime behavior change is intended except any correction required to
make the documented archive-input contract pass.

## Technical Context

**Language/Version**: Go version per current `go.mod` declaration
**Primary Dependencies**: Go standard library only
**Storage**: Checked-in Markdown spec artifacts plus Go tests
**Testing**: `go test ./cmd/my-gather`, then `go test ./...`
**Target Platform**: Static CLI binary for Linux and macOS targets
**Project Type**: CLI/report generator with Spec Kit documentation
**Performance Goals**: No production performance impact; test additions remain
small and deterministic
**Constraints**: English-only durable artifacts, no new dependencies, no
network access in shipped Go code, no duplicate archive-input path
**Scale/Scope**: One reconciliation feature, two targeted CLI tests, and
documentation-only status/task alignment across existing feature specs

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Single Static Binary | PASS | Test/docs only; no binary dependency change. |
| II. Read-Only Inputs | PASS | Archive tests use temporary test directories only. |
| III. Graceful Degradation | PASS | Tests pin documented non-panic input failures. |
| IV. Deterministic Output | PASS | No output generation changes planned. |
| V. Self-Contained HTML Reports | PASS | Report assets unchanged. |
| VI. Library-First Architecture | PASS | No package boundary change. |
| VII. Typed Errors | PASS | Existing archive error mapping is reused. |
| VIII. Reference Fixtures & Golden Tests | PASS | No collector parser or golden behavior change. |
| IX. Zero Network at Runtime | PASS | No runtime network code added. |
| X. Minimal Dependencies | PASS | No new dependencies. |
| XI. Reports Optimized for Humans Under Pressure | PASS | No report UX change. |
| XII. Pinned Go Version | PASS | No Go version change. |
| XIII. Canonical Code Path | PASS | Tests exercise the existing archive path. |
| XIV. English-Only Durable Artifacts | PASS | New durable text is English. |

## Project Structure

```text
AGENTS.md
CLAUDE.md
.specify/feature.json
cmd/my-gather/main_test.go
specs/001-ptstalk-report-mvp/
specs/002-report-feedback-button/
specs/003-feedback-backend-worker/
specs/006-observed-slowest-queries/
specs/007-advisor-intelligence/
specs/008-archive-input/
specs/009-spec-code-reconciliation/
```

## Design Decisions

- Treat the branch as an active Spec Kit reconciliation feature so automated
  agents have one unambiguous current target.
- Keep feature 001 as historical/in-progress because its task file explicitly
  tracks deferred quality work.
- Treat feature 002 tasks as historical because feature 003 superseded the
  submit path and later reconciliations document the shipped behavior.
- Mark features 006, 007, and 008 complete because their task files are fully
  checked and their implementation is already merged to `main`.
- Add focused tests in `cmd/my-gather/main_test.go` rather than introducing a
  separate archive test package, matching the existing CLI coverage pattern.

## Post-Design Constitution Check

No constitution violations were introduced.
