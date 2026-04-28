# Implementation Plan: Richer Processlist Metrics

**Branch**: `005-richer-processlist-metrics` | **Date**: 2026-04-28 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `specs/005-richer-processlist-metrics/spec.md`

## Summary

Extend the existing Processlist subview so support engineers can distinguish
idle connection pressure from active workload pressure, see long-running and
high-row work, and know whether query text is present in captured rows. The
feature reuses the existing `-processlist` parser, typed model, embedded chart
payload, and Database Usage section. It adds no new runtime dependencies, no
new top-level report section, and no network behavior.

## Technical Context

**Language/Version**: Go 1.24, pinned in `go.mod`.  
**Primary Dependencies**: Standard library only. Existing embedded JavaScript
chart asset is reused. No new Go module or browser dependency.  
**Storage**: N/A. Reads pt-stalk input files and writes one HTML output.  
**Testing**: `go test ./...`, focused parser tests, render payload/template
tests, determinism coverage through the existing suite.  
**Target Platform**: Existing My-gather targets: `linux/amd64`,
`linux/arm64`, `darwin/amd64`, `darwin/arm64`.  
**Project Type**: Single Go CLI with importable `parse/`, `model/`, and
`render/` packages.  
**Performance Goals**: Preserve current successful generation for the observed
396 MB, 20-snapshot processlist-heavy incident collection; new metrics are
computed in the existing line scan with no second pass over input files.  
**Constraints**: Single self-contained HTML file, deterministic output, no
runtime network, no writes under the input directory, graceful degradation for
missing or malformed optional fields.  
**Scale/Scope**: Existing processlist parser and report subview only. No full
SQL listing, no new raw query table, no new top-level section.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Pre-design status | Rationale |
|-----------|-------------------|-----------|
| I. Single Static Binary | PASS | Go-only change; no dynamic runtime dependency. |
| II. Read-Only Inputs | PASS | Parser continues to read `-processlist` through existing read-only paths. |
| III. Graceful Degradation | PASS | Optional malformed numeric fields are ignored without failing the row; age falls back from invalid `Time_ms` to valid `Time` when available. |
| IV. Deterministic Output | PASS | New maps/slices must use existing stable ordering and deterministic JSON emission. |
| V. Self-Contained HTML Reports | PASS | New metrics are embedded in the existing report payload and rendered by bundled JS. |
| VI. Library-First Architecture | PASS | Changes stay in `parse/`, `model/`, and `render/`; CLI remains thin. |
| VII. Typed Errors | PASS | No new branchable fatal error is introduced. |
| VIII. Reference Fixtures & Golden Tests | PASS | Existing processlist fixture/golden is updated, and targeted tests cover synthetic edge cases. |
| IX. Zero Network at Runtime | PASS | No network package or browser fetch is introduced. |
| X. Minimal Dependencies | PASS | No new dependency. |
| XI. Reports Optimized for Humans Under Pressure | PASS | Adds high-signal processlist summaries inside the existing DB section instead of adding a new section. |
| XII. Pinned Go Version | PASS | Go version unchanged. |
| XIII. Canonical Code Path | PASS | Extends the existing processlist parser/model/render path; no duplicate parser or fallback path. |
| XIV. English-Only Durable Artifacts | PASS | All new checked-in artifacts are English. |

**Pre-design gate: PASS.** No Complexity Tracking entry is required.

## Project Structure

### Documentation (this feature)

```text
specs/005-richer-processlist-metrics/
├── spec.md
├── plan.md
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
└── model.go                  # Processlist sample metric fields

parse/
├── processlist.go            # Parse Time/Time_ms, Rows_examined, Rows_sent, Info
└── processlist_test.go       # Parser and golden coverage

render/
├── build_view.go             # Attach summary metrics to template view
├── concat.go                 # Preserve metrics across merged snapshots
├── payloads.go               # Embed richer processlist payload
├── summaries.go              # Processlist summary callouts
├── view.go                   # Template view fields
├── templates/db.html.tmpl    # Processlist summary strip
├── assets/app.js             # Activity/metric chart behavior
├── render_test.go            # Embedded payload coverage
└── db_test.go                # Markup coverage

testdata/
└── golden/
    └── processlist.example2.2026_04_21_16_51_41.json
```

**Structure Decision**: Extend the existing processlist pipeline in place.
This keeps one canonical code path for the `-processlist` collector and keeps
the new output in the existing Database Usage Processlist subview.

## Phase 0 Research

See [research.md](./research.md).

## Phase 1 Design

See [data-model.md](./data-model.md), [contracts/packages.md](./contracts/packages.md),
[contracts/ui.md](./contracts/ui.md), and [quickstart.md](./quickstart.md).

## Post-design Constitution Check

| Principle | Post-design status | Rationale |
|-----------|--------------------|-----------|
| I | PASS | No build or runtime dependency change. |
| II | PASS | No writes inside input tree. |
| III | PASS | Missing optional fields are represented as zero metrics, not fatal errors. |
| IV | PASS | New summary and payload arrays are derived from sorted sample order. |
| V | PASS | HTML remains single-file and offline. |
| VI | PASS | Exported model fields receive godoc in `model/model.go`. |
| VII | PASS | No new branchable error path. |
| VIII | PASS | Parser golden and targeted tests are part of the task list. |
| IX | PASS | No network code. |
| X | PASS | No dependency. |
| XI | PASS | Metrics are compact callouts plus an in-place chart dimension. |
| XII | PASS | Go 1.24 unchanged. |
| XIII | PASS | Existing processlist parser is extended in place. |
| XIV | PASS | Artifacts are English. |

## Complexity Tracking

No constitution violations or complexity exceptions.
