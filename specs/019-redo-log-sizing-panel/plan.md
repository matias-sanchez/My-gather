# Implementation Plan: Redo log sizing panel

**Branch**: `019-redo-log-sizing-panel` | **Date**: 2026-05-07 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/019-redo-log-sizing-panel/spec.md`

## Summary

Add a new "Redo log sizing" panel under the existing InnoDB Status section
of the My-gather report. The panel reads MySQL's redo configuration from the
captured `-variables` file (using `innodb_redo_log_capacity` on 8.0.30+ or
`innodb_log_file_size * innodb_log_files_in_group` on 5.6/5.7), reads the
observed redo write rate from the `Innodb_os_log_written` counter
time-series in the captured `-mysqladmin` file, computes the busiest
15-minute rolling window of writes (or the full available capture window
when shorter), and renders a single panel that shows configured space,
observed rate, coverage estimate in minutes, recommended target sizes for
15 minutes and 1 hour of peak writes, an under-15-minutes warning, and a
citation of Percona KB0010732 as the methodology source.

## Technical Context

**Language/Version**: Go 1.26.2 (matches the existing `go.mod` directive)
**Primary Dependencies**: Standard library only. Reuses existing My-gather
packages: `model`, `parse`, `findings`, `render`, `reportutil`.
**Storage**: N/A
**Testing**: `go test` with the existing `testdata/`-driven golden test
infrastructure. The new panel adds focused Go unit tests at the package
level only; no new golden fixture is required because the panel is part of
the existing `db.html.tmpl` output and the existing report-level golden
tests already exercise that template.
**Target Platform**: Same release matrix as the rest of My-gather
(`linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`), built
`CGO_ENABLED=0`.
**Project Type**: Go CLI (single static binary) with HTML report output
embedded via `//go:embed`.
**Performance Goals**: No measurable regression on report generation time.
The redo-sizing computation is one pass over the existing
`Innodb_os_log_written` delta slice plus a constant-time variable lookup.
**Constraints**: One canonical computation path branching on detected
MySQL version internally rather than two parallel implementations. No
hidden fallbacks. No new external dependencies. No source file over
1000 lines (Principle XV).
**Scale/Scope**: One panel, one new render-side computation file, one new
view struct, one new template partial under the existing `db.html.tmpl`,
and focused tests covering the 5.7 path, the 8.0.30+ path, the under-sized
case, the well-sized case, and the missing-input degradation paths.

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Single Static Binary | PASS | No CGO, no new dynamic dependency. |
| II. Read-Only Inputs | PASS | The panel reads parsed model state only; no path writes inside the input tree. |
| III. Graceful Degradation (NON-NEGOTIABLE) | PASS | Missing variables, missing counter, and zero observed rate render observable "unavailable" / "no observed redo writes" states (FR-008, FR-009, FR-010). |
| IV. Deterministic Output | PASS | The computation is a pure function of the parsed report; no map iteration order, no goroutine, no time.Now. |
| V. Self-Contained HTML | PASS | Panel renders inline in the existing embedded template; no external asset added. |
| VI. Library-First Architecture | PASS | New computation lives in `render/` next to the existing InnoDB aggregation; new view type is `package render` private; godoc on every exported identifier. |
| VII. Typed Errors | PASS | The panel surfaces missing-input states via the view struct, not as branchable errors; no new `fmt.Errorf` branches added for callers. |
| VIII. Reference Fixtures & Golden Tests | PASS | No new collector parser is added (the panel reads existing parsed `-variables` and `-mysqladmin` data). The existing report-level golden test continues to cover `db.html.tmpl`. |
| IX. Zero Network at Runtime | PASS | No network code introduced. |
| X. Minimal Dependencies | PASS | Standard library only; no new direct dependency. |
| XI. Reports Optimized for Humans Under Pressure | PASS | The panel directly answers a top-of-funnel sizing question that would otherwise require manual computation against the KB0010732 method; the warning line surfaces the verdict without scanning. |
| XII. Pinned Go Version | PASS | No Go version change. |
| XIII. Canonical Code Path (NON-NEGOTIABLE) | PASS | See Canonical Path Audit below. |
| XIV. English-Only Durable Artifacts | PASS | All new files are in English. |
| XV. Bounded Source File Size | PASS | New `render/redo_sizing.go` file estimated at well under 300 lines. `render/innodb.go` (currently 352 lines) is unchanged; the new code lives in its own file rather than growing the existing one. |

**Canonical Path Audit (Principle XIII)**:

- **Canonical owner/path for touched behavior**: The new panel's
  computation lives in a single new file `render/redo_sizing.go` that
  exposes one builder function consumed by `render/build_view.go`. The
  configured-space reader inside that function is the canonical source
  for "how big is this server's redo log"; it branches on
  `innodb_redo_log_capacity` presence inside one function rather than
  via two parallel implementations. The observed-rate reader uses the
  existing `Innodb_os_log_written` delta slice on
  `model.MysqladminData.Deltas` (the same slice consumed by the
  existing redo Findings rules) and reuses
  `reportutil.VariableFloat` for variable lookup, so there is no
  duplicate variable-reading or counter-reading helper introduced.
- **Replaced or retired paths**: None. This is an additive panel.
  The existing `findings/rules_redo.go` rules (`redo.checkpoint_age`,
  `redo.pending_writes`, `redo.pending_fsyncs`, `redo.log_waits`)
  remain unchanged; they cover orthogonal signals (checkpoint age,
  pending I/O backlog, log-buffer waits) that the new panel does not
  duplicate. The new panel is the canonical owner of the
  capacity-vs-write-rate sizing analysis, which no existing rule
  performs.
- **External degradation paths**: When the `-variables` file is absent
  the panel renders "redo configuration unavailable" (FR-008); when
  `Innodb_os_log_written` is absent or insufficient it renders "rate
  unavailable" (FR-009); when the observed peak rate is zero it
  renders "no observed redo writes during the capture" (FR-010). All
  three are observable to the report reader and covered by tests.
  No silent internal fallback path exists.
- **Review check**: Reviewer verifies that
  (a) `render/redo_sizing.go` is the only file in the repository that
  computes configured redo space from the variables, that
  (b) the rolling-window helper is defined once and not duplicated by
  any other render or findings code, and that
  (c) no new helper duplicates `reportutil.VariableFloat` or the
  existing `findings/inputs.go` counter-reading helpers.

## Project Structure

### Documentation (this feature)

```text
specs/019-redo-log-sizing-panel/
├── spec.md
├── plan.md
├── research.md
├── quickstart.md
├── contracts/redo-sizing-panel.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
render/
├── redo_sizing.go            # NEW - computeRedoSizing(report) -> redoSizingView
├── redo_sizing_test.go       # NEW - 5.7 path, 8.0.30+ path, under-sized, well-sized, missing-input paths
├── view.go                   # MODIFIED - adds RedoSizing *redoSizingView field on view + redoSizingView type
├── build_view.go             # MODIFIED - calls computeRedoSizing in the existing DBSection block
├── innodb.go                 # UNCHANGED
└── templates/
    └── db.html.tmpl          # MODIFIED - adds the new panel partial under the existing InnoDB Status <details>
```

**Structure Decision**: Reuses the existing My-gather Go layout (CLI in
`cmd/`, parsers in `parse/`, model in `model/`, render in `render/`,
findings in `findings/`, shared helpers in `reportutil/`). The new panel
is fully scoped to the `render/` package because it consumes already
parsed data and produces view fields consumed by the existing template
infrastructure. No new package is introduced.

## Complexity Tracking

No constitution exceptions are required. All gates pass without a
justification entry.
