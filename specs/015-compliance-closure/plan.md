# Implementation Plan: Compliance Closure

**Branch**: `015-compliance-closure` | **Date**: 2026-05-05 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/015-compliance-closure/spec.md`

## Summary

Close the confirmed findings from the post-014 multi-agent audit: historical
spec drift, unsupported-version render state, InnoDB formatting ownership,
Spec Kit extension metadata drift, stale agent/skill guidance, and high-signal
Worker route test gaps.

## Technical Context

**Language/Version**: Go 1.26.2, TypeScript 5.7, Bash, Markdown
**Primary Dependencies**: Existing Go standard library and current Worker dev dependencies
**Storage**: N/A
**Testing**: `go test`, `go vet`, Vitest with Workers pool, Spec Kit scripts
**Target Platform**: local repository and GitHub CI
**Project Type**: Go CLI with TypeScript feedback Worker and Spec Kit tooling
**Performance Goals**: no runtime performance regression
**Constraints**: one canonical path per behavior, no hidden fallbacks, no new product features, no source file over 1000 lines
**Scale/Scope**: confirmed audit findings and targeted tests only

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Static Binary | PASS | No CGO or release build change. |
| II. Preserve Inputs | PASS | No input write path added. |
| III. Graceful Degradation | PASS | Unsupported versions become clearer report states. |
| IV. Deterministic Output | PASS | Rendering remains deterministic; tests cover output text. |
| V. Self-Contained HTML | PASS | No external report asset or fetch added. |
| VI. Library-First | PASS | Package boundaries remain intact. |
| VII. Typed Errors | PASS | No new branchable untyped errors. |
| VIII. Fixtures and Goldens | PASS | Parser/render behavior is covered by focused tests. |
| IX. Zero Network | PASS | No Go runtime network path added; Worker route tests cover existing exception. |
| X. Minimal Dependencies | PASS | No new dependency. |
| XI. Human Pressure Optimization | PASS | Unsupported state improves triage clarity. |
| XII. Pinned Go Version | PASS | No Go version change. |
| XIII. Canonical Code Path | PASS | Moves formatting through `reportutil` and aligns metadata pointers. |
| XIV. English-Only | PASS | All new artifacts are English. |
| XV. Bounded Source Size | PASS | Source-size gate remains in force. |

**Canonical Path Audit (Principle XIII)**:
- Canonical owner/path for touched behavior: `render` report state, `reportutil`
  integer formatting, `.specify/feature.json`, `.specify/extensions.yml`, and
  repo-local Spec Kit skills.
- Replaced or retired paths: stale historical text and stale metadata only.
- External degradation paths: unsupported pt-stalk versions are visible report
  states.
- Review check: run source-size, canonical-path, and validation commands.

## Project Structure

```text
specs/015-compliance-closure/
├── spec.md
├── plan.md
├── research.md
├── quickstart.md
├── contracts/audit.md
└── tasks.md
```

## Complexity Tracking

No constitution exceptions are required.
