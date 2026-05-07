# Implementation Plan: Top CPU Processes Caption

**Branch**: `018-top-cpu-caption` | **Date**: 2026-05-07 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/018-top-cpu-caption/spec.md`

## Summary

Add a single-sentence caption immediately above the "Top CPU
processes" chart in the OS Usage section so readers see the chart's
two non-obvious rules — top-3 only, and `mysqld` is always pinned —
before they interpret the curves. The mysqld pin already exists in
`render/concat.go::concatTop`; this feature adds the caption and a
regression test that locks the pin invariant in place.

## Technical Context

**Language/Version**: Go 1.26.2, HTML template (`html/template`)
**Primary Dependencies**: Existing Go standard library only
**Storage**: N/A
**Testing**: `go test ./...`, including the existing
`render.TestConcatTopRecomputesTop3WithMysqldAlwaysIn` plus a new
caption-presence test in the renderer test suite
**Target Platform**: local repository and GitHub CI
**Project Type**: Go CLI report generator
**Performance Goals**: no runtime change; one extra `<p>` per report
**Constraints**: one canonical caption location, no parallel chart
labelling helpers, no source file over 1000 lines, English-only
**Scale/Scope**: one template edit, one new Go regression test (and
optionally one CSS rule for caption styling)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Static Binary | PASS | No CGO or release build change. |
| II. Preserve Inputs | PASS | No input write path. |
| III. Graceful Degradation | PASS | Caption is gated by `HasTop`. |
| IV. Deterministic Output | PASS | Caption is a static string. |
| V. Self-Contained HTML | PASS | No external resource added. |
| VI. Library-First | PASS | Template-only change in `render`. |
| VII. Typed Errors | PASS | No new error path. |
| VIII. Fixtures and Goldens | PASS | New regression test reuses existing test patterns; golden output for affected templates updated in the same change. |
| IX. Zero Network | PASS | No network access. |
| X. Minimal Dependencies | PASS | No new dependency. |
| XI. Human Pressure Optimization | PASS | Caption removes a real source of triage confusion. |
| XII. Pinned Go Version | PASS | No Go version change. |
| XIII. Canonical Code Path | PASS | Caption has one home (`os.html.tmpl`); pin keeps its single home (`concat.go`). |
| XIV. English-Only | PASS | Caption text is English. |
| XV. Bounded Source Size | PASS | Template edit is ~3 lines; no source crosses 1000 lines. |

**Canonical Path Audit (Principle XIII)**:
- Canonical owner/path for touched behaviour:
  - Caption rendering: `render/templates/os.html.tmpl`, `sub-os-top`
    `<details>` block.
  - Mysqld pin (unchanged): `render/concat.go::concatTop`.
- Replaced or retired paths: none. No prior caption existed; the pin
  is reused as-is.
- External degradation paths, if any: none. The caption is a static
  string inside the same `if .HasTop` branch as the chart container.
- Review check: reviewers grep `os.html.tmpl` for the new caption
  text and confirm it appears exactly once, inside the `sub-os-top`
  block, and verify the new regression test in `render/` exercises
  `concatTop` rather than reimplementing `isMysqldCommand`.

## Project Structure

### Documentation (this feature)

```text
specs/018-top-cpu-caption/
├── spec.md
├── plan.md
├── research.md
├── quickstart.md
├── contracts/caption.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
render/
├── templates/os.html.tmpl       # caption added inside sub-os-top
├── concat.go                    # mysqld pin (unchanged, asserted by test)
├── concat_test.go               # new regression test added
└── os_test.go                   # new caption-presence test added
```

**Structure Decision**: Single Go module (`render` package). The
caption lives in the existing OS template; the regression test lives
beside the existing top-process tests in `render/`.

## Complexity Tracking

No constitution exceptions are required.
