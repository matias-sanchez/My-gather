# Implementation Plan: Advisor Intelligence

**Branch**: `007-advisor-intelligence` | **Date**: 2026-04-29 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `specs/007-advisor-intelligence/spec.md`

## Summary

Enhance the existing Advisor so it behaves more like an expert MySQL triage
assistant. The implementation will extend the current Rosetta-Stone-derived
`findings` registry with explicit diagnostic categories, richer evidence
bundles, correlation metadata, and a documented coverage map. It will improve
the rendered Advisor summary and cards without adding a new collector, runtime
service, dependency, or report section.

## Technical Context

**Language/Version**: Go version per current `go.mod` declaration (`go 1.24`)  
**Primary Dependencies**: Standard library plus existing embedded report assets  
**Storage**: N/A; reads pt-stalk files and writes one user-selected HTML output  
**Testing**: `go test ./...`, focused `findings`, `render`, and golden tests  
**Target Platform**: Static CLI binary for Linux and macOS targets  
**Project Type**: CLI plus importable Go packages  
**Performance Goals**: Advisor evaluation remains linear in registered rules
and report-sized inputs; generated HTML remains self-contained and bounded;
the top incident-driver summary is visible within the existing Advisor section  
**Constraints**: Read-only input tree, deterministic output, self-contained
HTML, zero network at runtime, no new dependency, no duplicate Advisor engine  
**Scale/Scope**: Existing MySQL pt-stalk report data and current Advisor
surface; no new collector parser and no external knowledge lookup at render
time

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Single Static Binary | PASS | Standard library only; no runtime install step. |
| II. Read-Only Inputs | PASS | Advisor consumes parsed report data only; no input-tree writes. |
| III. Graceful Degradation | PASS | Rules skip or downgrade when inputs are absent; no panics on partial captures. |
| IV. Deterministic Output | PASS | Registry ordering, finding ordering, evidence ordering, and rendered text are deterministic. |
| V. Self-Contained HTML Reports | PASS | Advisor renders through existing embedded assets; no external fetch. |
| VI. Library-First Architecture | PASS | Changes stay in `findings/`, `model/`, and `render/`; CLI remains thin. |
| VII. Typed Errors | PASS | No new branchable runtime errors expected; unsupported evidence remains structured finding state. |
| VIII. Reference Fixtures & Golden Tests | PASS | Existing report fixtures and Advisor goldens cover output drift; new scenarios add focused tests. |
| IX. Zero Network at Runtime | PASS | No network code or source lookup at runtime. |
| X. Minimal Dependencies | PASS | No new direct dependency. |
| XI. Reports Optimized for Humans Under Pressure | PASS | Adds a compact incident-driver summary and richer findings within the existing Advisor section. |
| XII. Pinned Go Version | PASS | No Go version change. |
| XIII. Canonical Code Path | PASS | Extends the current native Advisor registry; no second rule engine or fallback path. |
| XIV. English-Only Durable Artifacts | PASS | All planned artifacts and UI copy are English. |

## Project Structure

### Documentation (this feature)

```text
specs/007-advisor-intelligence/
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
└── tasks.md              # Created later by /speckit-tasks
```

### Source Code (repository root)

```text
findings/
├── findings.go
├── register.go
├── inputs.go
├── rules_*.go
├── rules_quality_test.go
├── findings_golden_test.go
└── findings_test.go

model/
└── model.go                  # Existing report inputs consumed by findings

render/
├── advisor_view.go
├── build_view.go
├── templates/advisor.html.tmpl
├── assets/app.css
├── render_test.go
└── db_test.go

testdata/
└── golden/
    └── findings.example2.json
```

**Structure Decision**: Extend the native Advisor registry and render path in
place. Add structured metadata to `findings.Finding` and registry definitions
only where it improves user-visible evidence, coverage traceability, and
deterministic prioritization. Preserve current report section placement and
avoid creating a parallel rules catalogue or runtime knowledge loader.

## Complexity Tracking

No constitution violations.

## Phase 0: Research

Research is captured in [research.md](./research.md). All planning unknowns are
resolved there:

- How Rosetta Stone topics map into deterministic Advisor coverage.
- How evidence strength and diagnostic categories should be represented.
- How correlated findings should be prioritized without a second rule engine.
- How to improve the Advisor UI while preserving the existing report spine.
- How to test rule quality, sparse captures, and deterministic output.

## Phase 1: Design

Design artifacts generated for this phase:

- [data-model.md](./data-model.md) defines Advisor Signal, Diagnostic Finding,
  Evidence Bundle, Recommendation, Correlation, and Expert Coverage Map.
- [contracts/packages.md](./contracts/packages.md) defines package-level
  contracts for `findings`, `model`, and `render`.
- [contracts/ui.md](./contracts/ui.md) defines the rendered Advisor contract.
- [quickstart.md](./quickstart.md) defines validation commands and manual
  review checks.

## Constitution Check - Post-Design

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Single Static Binary | PASS | Design adds no runtime dependency or install step. |
| II. Read-Only Inputs | PASS | Design reads parsed model data only. |
| III. Graceful Degradation | PASS | Missing evidence is represented as skipped, downgraded, or low-confidence findings. |
| IV. Deterministic Output | PASS | Contracts require stable ordering for findings, evidence, recommendations, and coverage rows. |
| V. Self-Contained HTML Reports | PASS | UI contract keeps Advisor fully inline in the generated report. |
| VI. Library-First Architecture | PASS | Contracts keep Advisor logic inside importable packages. |
| VII. Typed Errors | PASS | No new branchable error surface is planned. |
| VIII. Reference Fixtures & Golden Tests | PASS | Quickstart requires focused tests plus Advisor golden review. |
| IX. Zero Network at Runtime | PASS | Coverage source is checked in or embedded as durable metadata, never loaded remotely. |
| X. Minimal Dependencies | PASS | Standard library remains sufficient. |
| XI. Reports Optimized for Humans Under Pressure | PASS | UI contract emphasizes top drivers and collapsible detail. |
| XII. Pinned Go Version | PASS | No Go version change. |
| XIII. Canonical Code Path | PASS | Existing registry remains the only dispatch path. |
| XIV. English-Only Durable Artifacts | PASS | Design artifacts and planned UI copy are English. |
