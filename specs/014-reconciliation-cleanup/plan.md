# Implementation Plan: Reconciliation Cleanup

**Branch**: `014-reconciliation-cleanup` | **Date**: 2026-05-05 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/014-reconciliation-cleanup/spec.md`

## Summary

Close the confirmed findings from the post-PR #47 multi-agent audit with a
small reconciliation pass: strengthen netstat malformed timestamp diagnostics,
remove Spec Kit git-extension fallback helper paths, and align stale feature
001 documentation with current shipped behavior.

## Technical Context

**Language/Version**: Go 1.26.2, TypeScript 5.7, Bash, PowerShell, Markdown
**Primary Dependencies**: Existing Go standard library and current Worker dev dependencies
**Storage**: N/A
**Testing**: `go test`, `go vet`, Vitest with Workers pool, Spec Kit scripts
**Target Platform**: local repository and GitHub CI
**Project Type**: Go CLI with TypeScript feedback Worker and Spec Kit tooling
**Performance Goals**: no runtime performance change outside parser diagnostics
**Constraints**: no duplicate canonical paths, no hidden fallback behavior, no new product features, no source file over 1000 lines
**Scale/Scope**: confirmed post-PR #47 reconciliation findings only

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Static Binary | PASS | No CGO or release build change. |
| II. Preserve Inputs | PASS | No input writes added. |
| III. Graceful Degradation | PASS | Malformed netstat `TS` headers become observable diagnostics. |
| IV. Deterministic Output | PASS | Parser boundary handling remains deterministic. |
| V. Self-Contained HTML | PASS | No report asset or network change. |
| VI. Library-First | PASS | Changes remain inside parse/model/render/tooling owners. |
| VII. Typed Errors | PASS | No new branchable untyped errors. |
| VIII. Fixtures and Goldens | PASS | Parser behavior is covered with focused tests; no new collector suffix. |
| IX. Zero Network | PASS | No Go runtime network path added. |
| X. Minimal Dependencies | PASS | No new dependency. |
| XI. Human Pressure Optimization | PASS | No report UX expansion. |
| XII. Pinned Go Version | PASS | No Go version change. |
| XIII. Canonical Code Path | PASS | Removes fallback helper paths and stale parallel interpretations. |
| XIV. English-Only | PASS | All artifacts are English. |
| XV. Bounded Source Size | PASS | Source-size gate remains in force. |

**Canonical Path Audit (Principle XIII)**:
- Canonical owner/path for touched behavior: Spec Kit common helper scripts,
  netstat timestamp parser diagnostics, and current feature 001 text.
- Replaced or retired paths: duplicate fallback helper loading in git feature
  scripts and stale feature 001 wording.
- External degradation paths, if any: none.
- Review check: grep retired paths and run full validation.

## Project Structure

```text
specs/014-reconciliation-cleanup/
├── spec.md
├── plan.md
├── research.md
├── quickstart.md
├── contracts/audit.md
└── tasks.md
```

## Complexity Tracking

No constitution exceptions are required.
