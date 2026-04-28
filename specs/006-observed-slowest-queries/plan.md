# Implementation Plan: Observed Slowest Processlist Queries

**Branch**: `006-observed-slowest-queries` | **Date**: 2026-04-28 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `specs/006-observed-slowest-queries/spec.md`

## Summary

Add a bounded "Slowest observed queries" summary to the existing Processlist
subview by grouping active processlist rows with query text into stable
fingerprints. The implementation extends the existing processlist parser,
typed model, render merge path, embedded payload, DB template, client-side
report script, and Advisor rules without adding a new collector, dependency,
or top-level report section.

## Technical Context

**Language/Version**: Go 1.26.2 per current toolchain and `go.mod`  
**Primary Dependencies**: Standard library plus existing embedded uPlot assets  
**Storage**: N/A; reads pt-stalk files and writes one user-selected HTML output  
**Testing**: `go test ./...`, focused parser/render/model tests  
**Target Platform**: Static CLI binary for Linux and macOS targets  
**Project Type**: CLI plus importable Go packages  
**Performance Goals**: Process a multi-snapshot processlist-heavy incident
collection without materially increasing report size; bound rendered query
summaries to top entries; filter already-rendered rows in the browser without
re-parsing report payloads
**Constraints**: Read-only input tree, deterministic output, self-contained
HTML, no runtime network, no new dependency  
**Scale/Scope**: Existing `-processlist` collector only; no slow-query-log or
pt-query-digest parser in this feature

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Single Static Binary | PASS | Standard library only; no runtime dependency. |
| II. Read-Only Inputs | PASS | Parser reads processlist files only; no input-tree writes. |
| III. Graceful Degradation | PASS | Malformed optional fields are ignored while valid row data remains. |
| IV. Deterministic Output | PASS | Stable fingerprinting, sorting, and bounded lists. |
| V. Self-Contained HTML Reports | PASS | Rendered in existing embedded HTML/CSS/JS path. |
| VI. Library-First Architecture | PASS | Changes remain in `model/`, `parse/`, and `render/`; CLI stays thin. |
| VII. Typed Errors | PASS | No new branchable errors expected. |
| VIII. Reference Fixtures & Golden Tests | PASS | Adds focused tests and updates render golden output intentionally. |
| IX. Zero Network at Runtime | PASS | No network code. |
| X. Minimal Dependencies | PASS | No new direct dependency. |
| XI. Reports Optimized for Humans Under Pressure | PASS | Adds a bounded table, collapse controls, filters, and Advisor guidance inside the existing report flow. |
| XII. Pinned Go Version | PASS | No Go version change. |
| XIII. Canonical Code Path | PASS | Extends the existing processlist path; no duplicate parser. |
| XIV. English-Only Durable Artifacts | PASS | All durable artifacts are English. |

## Project Structure

### Documentation (this feature)

```text
specs/006-observed-slowest-queries/
├── plan.md
├── spec.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── packages.md
│   └── ui.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
model/
├── model.go
└── processlist_query_test.go

parse/
├── processlist.go
└── processlist_test.go

render/
├── concat.go
├── payloads.go
├── summaries.go
├── templates/db.html.tmpl
├── assets/app.js
├── assets/app.css
├── db_test.go
└── render_test.go

findings/
├── rules_queryshape.go
└── findings_test.go
```

**Structure Decision**: Extend the existing processlist parser/model/render
pipeline in place. Extend the existing Query Shape Advisor subsystem rather
than adding a separate diagnostic surface. This preserves the canonical path
for all processlist and Advisor data.

## Complexity Tracking

No constitution violations.
