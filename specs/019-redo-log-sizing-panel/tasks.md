---
description: "Task list for feature 019-redo-log-sizing-panel"
---

# Tasks: Redo log sizing panel

**Input**: Design documents from `specs/019-redo-log-sizing-panel/`
**Prerequisites**: plan.md, spec.md, research.md, contracts/redo-sizing-panel.md, quickstart.md

**Tests**: Included; the spec explicitly requires unit tests for both
the 5.7 path and the 8.0.30+ path (well-sized and under-sized cases).

## Canonical Path Metadata (Principle XIII)

- **Canonical owner/path**: `render/redo_sizing.go` is the single
  source of redo-log sizing analysis (configured space + observed
  rate + coverage + recommendations). The InnoDB section template
  partial in `render/templates/db.html.tmpl` is the single rendering
  path. The view field `RedoSizing *redoSizingView` on the existing
  view struct in `render/view.go` is the single carrier between
  computation and template.
- **Old path treatment**: N/A. Additive panel; no existing path is
  replaced. Existing `findings/rules_redo.go` rules remain unchanged.
- **External degradation evidence**: T016 (variables-missing test),
  T017 (counter-missing test), T018 (short-capture-window test),
  T019 (no-writes test), T020 (warning-boundary test).

## Format: `[ID] [P?] [Story] Description`

- **[P]** marks tasks that touch different files and can run in parallel.
- **[Story]** ties the task to a user story from spec.md.

## Phase 1: Scaffolding

- [ ] T001 [P] Add the `redoSizingView` struct to `render/view.go`
      next to the existing `innoDBMetricView` definition. Include
      every field and godoc comment described in
      `specs/019-redo-log-sizing-panel/contracts/redo-sizing-panel.md`.
- [ ] T002 [P] Add a `RedoSizing *redoSizingView` field on the main
      view struct in `render/view.go` in the DB section payload block
      (immediately after `InnoDBMetrics`). Document the contract that
      a nil value means "panel not rendered" and a non-nil value with
      `State == "config_missing"` means "panel rendered with placeholder
      content".

## Phase 2: Computation (canonical owner)

- [ ] T003 [US1] Create `render/redo_sizing.go` with the
      `computeRedoSizing(r *model.Report) *redoSizingView` function.
      Implement the canonical configured-space reader: try
      `innodb_redo_log_capacity` via `reportutil.VariableFloat`, then
      fall back to `innodb_log_file_size * innodb_log_files_in_group`.
      Populate `ConfiguredBytes`, `ConfiguredText` (via
      `reportutil.HumanBytes`), and `ConfigSource`. Set
      `KBReference = "Percona KB0010732"` in every state. (Implements
      FR-001, FR-002, FR-007, FR-008.)
- [ ] T004 [US1] In the same file, implement the average-rate
      computation: walk `r.DBSection.Mysqladmin.Deltas["Innodb_os_log_written"]`
      and sum non-NaN entries past index 0; divide by the sum of
      per-snapshot wall-clock spans (mirroring
      `findings/inputs.go::captureSeconds` semantics, computed locally).
      Populate `ObservedRateBytesPerSec`, `ObservedRateText`, and
      `ObservedRatePerMinText`. Render as
      `reportutil.HumanBytes(rate) + "/s"` and
      `reportutil.HumanBytes(rate*60) + "/min"`. (Implements FR-003.)
- [ ] T005 [US1] In the same file, implement the rolling-window peak
      computation: scan the deltas + timestamps slice with a two-pointer
      sliding window of length 900 seconds. Track the maximum
      `bytes_in_window / actual_window_seconds`. NaN slots split the
      walk: when one is encountered, drop the in-progress accumulator
      and restart from the next non-NaN sample. If no contiguous block
      of length 900s exists, set `PeakWindowSeconds` to the longest
      contiguous block's wall-clock span and `PeakWindowLabel` to the
      `"available N-second"` form; otherwise set them to 900 and
      `"15-minute"`. Populate `PeakRateBytesPerSec` and `PeakRateText`.
      (Implements FR-004 plus the short-window edge case.)
- [ ] T006 [US1] Compute coverage: `CoverageMinutes = ConfiguredBytes /
      PeakRateBytesPerSec / 60`; render `CoverageText` as
      `fmt.Sprintf("%d minutes", int(math.Floor(CoverageMinutes)))`.
      Set `CoverageText = "n/a"` whenever any input is missing.
      (Implements FR-004.)
- [ ] T007 [US3] Compute recommended sizes:
      `Recommended15MinBytes = PeakRateBytesPerSec * 900` and
      `Recommended1HourBytes = PeakRateBytesPerSec * 3600`. Render via
      `reportutil.HumanBytes`. Set `*Text` fields to `"n/a"` when no
      observed peak rate is available. (Implements FR-005.)
- [ ] T008 [US2] Set `UnderSized = (State == "ok" && CoverageMinutes <
      15)` and populate `WarningLine` with the exact phrase from FR-006:
      `"Current redo space holds only X minutes of peak writes - consider
      raising"` where X is the floored minute count. Leave both empty
      otherwise. (Implements FR-006.)

## Phase 3: View wiring

- [ ] T009 In `render/build_view.go`, inside the existing
      `if r.DBSection != nil` block, after the `InnoDBMetrics` lines,
      call `v.RedoSizing = computeRedoSizing(r)`. No other call site
      may compute this view; this is the single canonical wiring point.

## Phase 4: Template

- [ ] T010 In `render/templates/db.html.tmpl`, inside the existing
      `<details id="sub-db-innodb">` body, after the `.callouts` div
      and before its closing `</div>` of class `body`, add a new
      `<div class="redo-sizing">` block guarded by `{{- if .RedoSizing }}`.
      Render the panel rows with explicit unambiguous labels so
      reader does not conflate the average rate with the peak rate:
      title `"Redo log sizing"`, configured space with its source
      label, the average rate row labelled
      `"Observed average write rate"` showing both per-sec and
      per-min, the peak rate row labelled
      `"Peak write rate (<window-label> rolling)"` using
      `RedoSizing.PeakWindowLabel`, coverage in minutes labelled
      `"Coverage at peak"`, recommended sizes labelled
      `"Recommended for 15 minutes of peak"` and
      `"Recommended for 1 hour of peak"`, the warning line guarded
      by `{{- if .RedoSizing.UnderSized }}`, and the KB citation
      labelled `"Methodology: Percona KB0010732"`. Branch on
      `.State` for the `"config_missing"`, `"rate_unavailable"`, and
      `"no_writes"` states. Use the existing `sev-warn` callout
      class for the warning state.

## Phase 5: Tests

- [ ] T011 [US1] Create `render/redo_sizing_test.go`. Add helpers
      `makeReportWithVarsAndCounter` (synthetic *model.Report builder
      taking variable map, deltas, and timestamps; produces a
      report.DBSection with mysqladmin populated) and
      `assertField(t, got, want)`.
- [ ] T012 [US1] Add test `TestComputeRedoSizing_57_WellSized`: 5.7
      vars (`innodb_log_file_size=1073741824`,
      `innodb_log_files_in_group=2`), low-write-rate counter series.
      Asserts `State == "ok"`, `ConfigSource ==
      "innodb_log_file_size x innodb_log_files_in_group"`,
      `ConfiguredText == "2.0 GiB"`, `UnderSized == false`,
      `WarningLine == ""`, `KBReference == "Percona KB0010732"`.
- [ ] T013 [US1, US2] Add test `TestComputeRedoSizing_57_UnderSized`:
      same 5.7 vars but with `innodb_log_file_size=134217728` (256 MiB
      total) and a high-write-rate counter series. Asserts `State ==
      "ok"`, `UnderSized == true`, `WarningLine` matches the FR-006
      phrasing, `Recommended15MinText` and `Recommended1HourText`
      populated.
- [ ] T014 [US1] Add test `TestComputeRedoSizing_80Plus_WellSized`:
      vars include `innodb_redo_log_capacity=17179869184` (16 GiB) AND
      `innodb_log_file_size=1073741824` + `innodb_log_files_in_group=2`
      (the legacy values that 8.0.30+ ignores). Counter series chosen
      so coverage at peak >= 60 minutes. Asserts `State == "ok"`,
      `ConfigSource == "innodb_redo_log_capacity"`, `ConfiguredText
      == "16.0 GiB"`, `UnderSized == false`. Confirms the legacy
      variables are ignored when `innodb_redo_log_capacity` is set.
- [ ] T015 [US1, US2] Add test `TestComputeRedoSizing_80Plus_UnderSized`:
      vars include `innodb_redo_log_capacity=536870912` (512 MiB)
      against a high-write-rate counter series. Asserts `State == "ok"`,
      `ConfigSource == "innodb_redo_log_capacity"`, `UnderSized ==
      true`, `WarningLine` populated.
- [ ] T016 Add test `TestComputeRedoSizing_ConfigMissing`: variables
      section absent; counter series present. Asserts `State ==
      "config_missing"`, `ConfiguredText == "unavailable"`,
      `CoverageText == "n/a"`, `KBReference == "Percona KB0010732"`.
- [ ] T017 Add test `TestComputeRedoSizing_RateUnavailable`: variables
      present; `Innodb_os_log_written` absent from
      `Mysqladmin.Deltas`. Asserts `State == "rate_unavailable"`,
      `ConfiguredText` populated, `ObservedRateText == "unavailable"`,
      `CoverageText == "n/a"`, `Recommended15MinText == "n/a"`.
- [ ] T018 Add test `TestComputeRedoSizing_ShortCaptureWindow`:
      variables present; counter series spans 30 seconds total.
      Asserts `PeakWindowSeconds < 900`, `PeakWindowLabel` matches
      `"available N-second"` (where N is the actual span), and
      `CoverageText` is populated using the shorter window's peak.
- [ ] T019 Add test `TestComputeRedoSizing_NoWrites`: variables and
      `Innodb_os_log_written` counter both present, but every counter
      delta is zero (idle workload). Asserts `State == "no_writes"`,
      `ConfiguredText` populated, `ObservedRateText == "0 B/s"` (or
      the equivalent rendering of zero), `CoverageText == "n/a"`,
      `Recommended15MinText == "n/a"`, `Recommended1HourText == "n/a"`,
      `UnderSized == false`, `WarningLine == ""`, `KBReference ==
      "Percona KB0010732"`. (Implements FR-010.)
- [ ] T020 Add test `TestComputeRedoSizing_WarningBoundary`: choose
      configured space and a constant peak write rate so coverage is
      exactly 15.0 minutes (e.g. configured = 900 MiB, peak rate =
      1 MiB/s). Asserts `CoverageMinutes == 15`, `UnderSized ==
      false`, `WarningLine == ""`. Then re-run with configured =
      899 MiB and assert `UnderSized == true`. (Implements US2 AC3
      explicitly.)

## Phase 6: Validation

- [ ] T021 Run `go vet ./...` and `go test ./...` from repo root.
      Fix any failure.
- [ ] T022 Run the constitution pre-push guard locally
      (`scripts/hooks/pre-push-constitution-guard.sh` or via
      `git push` after staging) and address any guard finding before
      pushing.

## Phase 7: Cross-agent sync

- [ ] T023 Update `CLAUDE.md`, `AGENTS.md`, and `.specify/feature.json`
      so all three signals point at `019-redo-log-sizing-panel` (per
      the side-by-side agent contract in CLAUDE.md). The
      `.specify/feature.json` change is already applied during
      `/speckit-specify`; this task confirms it and updates the
      Markdown context files.

## Dependency graph

```
T001 ─┐
T002 ─┴─> T003 ─> T004 ─> T005 ─> T006 ─> T007 ─> T008 ─> T009 ─> T010
                                                                    │
                                  T011 ──────────┬─> T012, T013, T014, T015, T016, T017, T018, T019, T020
                                                  │
                                                  └─> T021 ─> T022 ─> T023
```

T012-T020 may run in parallel once T011 lands. T001 and T002 may run
in parallel because they touch the same file (`view.go`) at separate
locations and can be staged together.
