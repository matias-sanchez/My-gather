# Feature Specification: Top CPU Processes Caption

**Feature Branch**: `018-top-cpu-caption`
**Created**: 2026-05-07
**Status**: Draft
**Input**: User description: "Add a clarifying caption above the Top CPU processes chart in the report. The caption must state explicitly that the chart shows only the top 3 most-consuming processes and that mysqld is always included (pinned) regardless of its current CPU rank."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Reader understands chart scope at a glance (Priority: P1)

A support engineer opens a generated pt-stalk report and scrolls to the
"Top CPU processes" chart. Without reading source code or summary
statistics, the reader needs to know two things immediately:

1. The chart only renders the three highest-CPU processes (it is not a
   complete process census).
2. The `mysqld` server process is always rendered, even when it does not
   make the top-3 by average CPU.

The clarifying caption sits directly above the chart so the reader's
eye encounters it before interpreting the curves.

**Why this priority**: Without the caption, readers misinterpret the
chart. They either assume a missing process means the host did not run
it, or they assume `mysqld` is hidden because its CPU was low. Both
mistakes cause wasted incident-response time on a tool whose entire
reason for existing is fast triage.

**Independent Test**: Render any report that includes a `-top` capture
and confirm the caption text appears immediately above the
`#chart-top` element, on its own line, before any chart curves are
visible.

**Acceptance Scenarios**:

1. **Given** a report with `-top` data, **When** the OS section's "Top
   CPU processes" subview is opened, **Then** a caption above the
   chart explicitly names the "top 3" rule and the always-included
   `mysqld` rule.
2. **Given** a report where `mysqld` ranks first in average CPU, **When**
   the same chart is rendered, **Then** the caption is unchanged
   (its text does not depend on `mysqld`'s current rank).
3. **Given** a report where `-top` data is missing or unparseable,
   **When** the subview is opened, **Then** the existing
   "Data not available" banner renders and the caption is not shown
   (no chart, no caption).

---

### User Story 2 - Caption stays truthful across all reports (Priority: P1)

The caption asserts that `mysqld` is always included. The chart-data
preparation code must therefore actually pin `mysqld` regardless of its
average-CPU rank. A regression that drops the pin would turn the
caption into a lie and silently mislead readers.

**Why this priority**: Equal weight to Story 1 because a truthful
chart and a truthful caption are inseparable. The pin already exists
in `render/concat.go::concatTop` (verified by inspection: the
`isMysqldCommand` branch appends `mysqld` after the top-3 when it is
not already present). The risk is future regression, not current
absence.

**Independent Test**: Run a regression test that constructs a `TopData`
where `mysqld` has very low CPU and several other processes dominate,
then asserts `mysqld` appears in `Top3ByAverage` after `concatTop`.

**Acceptance Scenarios**:

1. **Given** a synthetic top capture where three non-`mysqld`
   processes dominate CPU and `mysqld` is the lowest consumer, **When**
   `concatTop` merges the snapshots, **Then** `Top3ByAverage`
   contains the `mysqld` series (length 4: top-3 plus pinned mysqld).
2. **Given** a synthetic top capture where `mysqld` already ranks in
   the top 3, **When** `concatTop` merges the snapshots, **Then**
   `Top3ByAverage` has length 3 (no duplicate mysqld entry).

---

### Edge Cases

- Report has `-top` data but no `mysqld` process (rare; pt-stalk run
  on a non-MySQL host). The pin only triggers when a `mysqld` PID
  exists in the merged stream, so `Top3ByAverage` simply holds the
  top 3 actual processes. The caption still shows because the chart
  shows; readers see "mysqld is always included" and observe its
  absence, which correctly tells them no mysqld process ran.
- `mariadbd` instead of `mysqld`. The existing `isMysqldCommand`
  helper treats `mariadbd` as equivalent to `mysqld`, so the pin
  behavior already covers MariaDB hosts. Caption text uses `mysqld`
  as the canonical short name for both.
- `-top` data missing or unparseable. The chart container is not
  rendered; the caption must not render either (no caption without a
  chart).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The "Top CPU processes" chart subview MUST render a
  short caption immediately above the chart container that states
  the chart shows only the top 3 processes by CPU and that `mysqld`
  is always included regardless of its current rank.
- **FR-002**: The caption MUST render only when the chart itself
  renders (i.e., gated by the same `HasTop` condition that gates
  the chart container).
- **FR-003**: The caption text MUST NOT depend on the current
  ranking of `mysqld` in the report — it states the policy, not the
  current state.
- **FR-004**: The chart-data preparation code MUST continue to
  include the `mysqld` (or `mariadbd`) series in the chart's series
  list whenever a process matching the canonical mysqld-server
  command exists in the merged top-process stream, regardless of
  whether that process ranks in the top 3 by average CPU.
- **FR-005**: A regression test MUST assert FR-004: given a merged
  top stream where `mysqld` does not rank in the top 3 by average
  CPU, the resulting chart series list MUST still contain the
  `mysqld` series.

### Canonical Path Expectations

- **Canonical owner/path**:
  - Caption rendering: `render/templates/os.html.tmpl` (the existing
    `sub-os-top` `<details>` block; caption added inside the same
    `body` div, gated by the same `HasTop` condition).
  - Chart-series mysqld pin: `render/concat.go::concatTop` (the
    existing block that appends `mysqld` after the top-3 when it is
    not already present, using `isMysqldCommand` for matching). This
    pin already exists and is unchanged by this feature.
- **Old path treatment**: N/A. No competing caption or pin path
  exists; this feature adds a new caption to a single canonical
  template location and asserts the existing pin via a regression
  test.
- **External degradation**: Reports rendered without this feature
  will not show the caption; the chart still works. Reports rendered
  with this feature show the caption above the chart. There is no
  silent fallback path — either the caption is in the template or
  it is not.
- **Review check**: Reviewers must verify (a) the caption appears
  exactly once in `os.html.tmpl` inside the `sub-os-top` block, (b)
  no parallel caption helper or partial template was introduced,
  and (c) the new regression test exercises the existing
  `concat.go` pin rather than reimplementing it.

### Key Entities

- **Top CPU processes chart**: The `#chart-top` chart in the OS
  Usage section's "Top CPU processes" subview. Backed by
  `model.TopData.Top3ByAverage` (which, despite the name, may
  contain four series when the mysqld pin fires).
- **Caption**: A short explanatory paragraph rendered immediately
  above the chart container, explaining the chart's two non-obvious
  rules (top-3 limit and mysqld pin).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of generated reports that contain a `-top`
  chart also contain the caption text above that chart (verified
  by template inspection plus a renderer-level test that asserts
  the caption text appears in the rendered HTML when `HasTop` is
  true).
- **SC-002**: 0% of generated reports show the caption when the
  chart itself is not rendered (verified by the same template
  gating: caption sits inside the `if .HasTop` branch).
- **SC-003**: A regression test in `render/` fails if the
  `mysqld`-always-included pin is removed from `concatTop`,
  catching the regression before merge.
- **SC-004**: A reader unfamiliar with the report can correctly
  state, after looking only at the chart and its caption, that
  (a) the chart shows only three processes and (b) `mysqld` is
  always one of them when present.

## Assumptions

- The pin behavior described in FR-004 already exists in
  `render/concat.go::concatTop` (verified by direct inspection of
  the file at the start of this feature). This feature therefore
  adds the caption and a confirming regression test; it does not
  add or modify pin logic.
- The caption belongs in the existing `os.html.tmpl` template
  rather than in a new partial. The OS section already uses inline
  `<p>` and `<div class="chart-summary">` elements directly in the
  template, so a new caption follows established style.
- Caption styling reuses the existing `chart-summary` or a similarly
  unobtrusive class so no new CSS rule is required. If existing
  classes do not fit, a single small CSS rule may be added to the
  appropriate ordered CSS source part under
  `render/assets/app-css/`.
- English-only text (Principle XIV).
