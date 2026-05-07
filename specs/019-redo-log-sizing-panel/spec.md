# Feature Specification: Redo log sizing panel

**Feature Branch**: `019-redo-log-sizing-panel`
**Created**: 2026-05-07
**Status**: Draft
**Input**: User description: "Add a Redo log sizing panel to the InnoDB Status section, supporting both the legacy 5.6/5.7 redo configuration model and the 8.0.30+ dynamic capacity model, deriving observed redo write rate from the Innodb_os_log_written counter time-series, and warning when the configured redo space holds less than 15 minutes of peak writes. Methodology source: Percona KB0010732."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Quantify whether the configured redo log holds the workload (Priority: P1)

A support engineer reading a My-gather report for an incident wants a single
panel that tells them, in plain numbers, whether the redo log on this server
is sized for the actual write rate observed in the capture. They do not want
to compute the rate by hand from raw counters or look up which configuration
variable is current for the server's MySQL version.

**Why this priority**: Redo log sizing is one of the most common InnoDB
sizing questions on real cases and a frequent root cause of checkpoint
stalls. Without this panel the user reads `Innodb_os_log_written` counters
and `innodb_log_file_size`/`innodb_redo_log_capacity` variables manually,
applies the KB0010732 method by hand, and reaches the same conclusion.
Putting this in one panel eliminates that manual step.

**Independent Test**: Open a generated report from a pt-stalk capture that
contains both `-variables` and `-mysqladmin` files for a MySQL instance and
verify the InnoDB Status section shows a Redo log sizing panel with the
configured redo space, the observed write rate, and a coverage estimate
in minutes.

**Acceptance Scenarios**:

1. **Given** a pt-stalk capture from a MySQL 5.7 server with
   `innodb_log_file_size=1073741824` and `innodb_log_files_in_group=2`
   and an `Innodb_os_log_written` counter time-series, **When** the report
   is generated, **Then** the panel shows configured redo space `2.0 GiB`
   (1 GiB x 2), an observed write rate in bytes/sec and bytes/min, and a
   coverage estimate in minutes computed against the busiest 15-minute
   rolling window of writes.
2. **Given** a pt-stalk capture from a MySQL 8.0.30+ server with
   `innodb_redo_log_capacity=4294967296` and an `Innodb_os_log_written`
   counter time-series, **When** the report is generated, **Then** the
   panel shows configured redo space `4.0 GiB` (taken from
   `innodb_redo_log_capacity` directly, ignoring the legacy variables), an
   observed write rate, and a coverage estimate.
3. **Given** any supported capture, **When** the panel is rendered, **Then**
   it shows a one-line citation of Percona KB0010732 as the methodology
   source.

---

### User Story 2 - Surface under-sized redo as a callout (Priority: P1)

A support engineer scanning the report wants the panel to flag, without
hunting, when the configured redo space is small enough that the workload
fills it in less than 15 minutes at peak. That threshold is the actionable
boundary from KB0010732 — below it, checkpoint stalls become very likely
under load; at or above it, the configuration is healthy.

**Why this priority**: The panel is only useful at incident time if the
"this is undersized" verdict is unambiguous and pre-computed. A user under
pressure should not have to read the bytes/sec number and decide for
themselves whether 12 minutes of coverage is acceptable.

**Independent Test**: Generate a report from a synthetic capture where the
configured redo space holds less than 15 minutes of peak writes and verify
the panel renders the warning line "Current redo space holds only X
minutes of peak writes - consider raising" with X computed from the data.

**Acceptance Scenarios**:

1. **Given** a capture where coverage at peak is 7 minutes, **When** the
   panel renders, **Then** it shows the warning line "Current redo space
   holds only 7 minutes of peak writes - consider raising".
2. **Given** a capture where coverage at peak is 45 minutes, **When** the
   panel renders, **Then** it does not show the warning line.
3. **Given** a capture where coverage at peak is exactly 15 minutes, **When**
   the panel renders, **Then** it does not show the warning line (the
   threshold is "below 15 minutes", inclusive of 15 is acceptable).

---

### User Story 3 - Show recommended sizes for 15 minutes and 1 hour of peak (Priority: P2)

A support engineer who confirms the redo log is undersized wants the panel
to suggest concrete target sizes derived from the same observed peak rate,
so the recommendation in the case response is grounded in this capture
rather than a generic rule of thumb.

**Why this priority**: This converts the diagnostic from "you have a
problem" into "here is the size to set", which is the recommendation a
support engineer would otherwise compute by hand from the same numbers.

**Independent Test**: For any capture with a non-zero observed peak rate,
verify the panel shows two recommended target sizes labelled "15 minutes
of peak" and "1 hour of peak" in human-readable bytes.

**Acceptance Scenarios**:

1. **Given** an observed peak rate of `10 MiB/s`, **When** the panel
   renders, **Then** the recommended sizes are computed from
   `peak_rate * 15 * 60` and `peak_rate * 60 * 60` respectively, and
   shown in human-readable bytes (for example `9.0 GiB` and `36.0 GiB`).
2. **Given** the configured redo space already exceeds the 1-hour
   recommendation, **When** the panel renders, **Then** the recommended
   sizes are still shown alongside the configured value so the user can
   see the headroom.

---

### Edge Cases

- **Capture window shorter than 15 minutes** (the common pt-stalk case is
  about 30 seconds per trigger): the panel falls back to the full
  available capture window as the rolling-window size and labels the
  coverage line so the reader knows the peak is "observed over the
  available window" rather than a true 15-minute peak. The panel does
  not silently treat a 30-second peak as a 15-minute peak.
- **`Innodb_os_log_written` absent or only one sample**: the observed
  write rate cannot be computed; the panel renders a "rate unavailable"
  state for the rate, coverage, and recommendation rows, but still shows
  the configured redo space if the variables are present.
- **Server variables absent**: the panel renders a "redo configuration
  unavailable" state and skips the coverage and recommendation rows
  entirely. The KB reference is still shown.
- **Mixed-model variables** (legacy `innodb_log_file_size` present
  alongside `innodb_redo_log_capacity` on an 8.0.30+ server): the panel
  uses `innodb_redo_log_capacity` as the source of truth and ignores the
  legacy variables, matching MySQL 8.0.30+ behavior.
- **Zero observed write rate**: coverage is undefined; the panel shows
  the configured size and a "no observed redo writes during the capture"
  state for coverage and recommendations.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The InnoDB Status section MUST contain a panel titled
  "Redo log sizing".
- **FR-002**: The panel MUST display the configured redo log space in
  human-readable bytes, computed as
  `innodb_redo_log_capacity` when that variable is present (8.0.30+
  model), otherwise as
  `innodb_log_file_size * innodb_log_files_in_group` (legacy 5.6/5.7
  model).
- **FR-003**: The panel MUST display the observed redo write rate as
  bytes per second and bytes per minute, derived from the
  `Innodb_os_log_written` counter time-series in the captured pt-stalk
  data using the existing canonical counter-rate computation (delta
  divided by interval, excluding the bogus first sample).
- **FR-004**: The panel MUST display a coverage estimate, expressed as
  the number of minutes of peak observed write rate that the configured
  redo space currently holds, where the peak is computed as the busiest
  rolling 15-minute window of `Innodb_os_log_written` deltas in the
  capture. If the capture is shorter than 15 minutes, the rolling
  window collapses to the full available capture window and the panel
  labels the coverage value so the reader knows the peak is observed
  over the available window rather than a true 15-minute window.
- **FR-005**: The panel MUST display two recommended redo log sizes,
  computed as `peak_rate * 15 * 60` and `peak_rate * 60 * 60`, in
  human-readable bytes.
- **FR-006**: When the coverage estimate is below 15 minutes, the panel
  MUST display the warning line "Current redo space holds only X minutes
  of peak writes - consider raising" with X being the computed coverage
  rounded to a whole number of minutes.
- **FR-007**: The panel MUST display a citation of Percona KB0010732 as
  the methodology source.
- **FR-008**: When the configuration variables required to compute
  configured redo space are missing, the panel MUST render a
  "redo configuration unavailable" state for the configured space,
  coverage, and recommendation rows, but MUST still render the panel
  shell and the KB citation.
- **FR-009**: When the `Innodb_os_log_written` counter is absent or
  cannot produce a rate (insufficient samples), the panel MUST render
  a "rate unavailable" state for the rate, coverage, and recommendation
  rows, but MUST still render the configured space when available.
- **FR-010**: When the observed peak rate is zero, the panel MUST render
  a "no observed redo writes during the capture" state for the coverage
  and recommendation rows.

### Canonical Path Expectations

- **Canonical owner/path**: A new panel under the existing InnoDB Status
  section in `render/`, backed by a single redo-sizing computation
  function. The configured-space computation is the canonical source for
  reading the redo configuration, regardless of MySQL version - branching
  on the `innodb_redo_log_capacity` variable presence happens inside that
  one function rather than via two parallel implementations. The
  observed-rate computation reuses the existing canonical counter-rate
  helper (the same path used by the existing Redo Log advisor rules) so
  there is no second `Innodb_os_log_written` rate computation in the
  codebase.
- **Old path treatment**: N/A - this is an additive panel. Existing
  `findings/rules_redo.go` rules (`redo.checkpoint_age`,
  `redo.pending_writes`, `redo.pending_fsyncs`, `redo.log_waits`) are
  unchanged and remain the canonical owners of their respective signals.
  The new panel is the canonical owner of redo-log capacity sizing
  against the observed write rate, which no existing rule covers.
- **External degradation**: When configuration variables or the
  `Innodb_os_log_written` counter are missing or yield insufficient
  data, the panel renders observable "unavailable" states (FR-008,
  FR-009, FR-010) rather than silently substituting placeholder values.
  These states are covered by tests.
- **Review check**: Reviewer verifies that the redo configured-space
  reader is the only path computing configured redo size, that the
  observed-rate computation routes through the existing canonical
  counter helper, and that there is no duplicated rolling-window helper
  if a future feature needs the same primitive.

### Key Entities *(include if feature involves data)*

- **Configured redo space**: A single byte-valued quantity describing
  the total redo log space MySQL is configured to use. Sourced from
  `innodb_redo_log_capacity` on 8.0.30+, or
  `innodb_log_file_size * innodb_log_files_in_group` on 5.6/5.7.
- **Observed redo write rate**: A time-series of per-sample bytes
  written to the redo log, derived from the `Innodb_os_log_written`
  counter deltas in the mysqladmin capture. The panel uses the rolling
  15-minute window peak (or the full capture window if shorter) for its
  coverage and recommendation calculations, and the overall average
  rate for its informational bytes/sec and bytes/min display.
- **Coverage estimate**: A minute-valued quantity equal to
  `configured_redo_space / peak_rate`, expressed in minutes.
- **Recommended sizes**: Two byte-valued quantities equal to
  `peak_rate * 15 * 60` and `peak_rate * 60 * 60`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A support engineer reading a generated report can
  determine in under 30 seconds whether the redo log is sized
  appropriately for the captured workload, without consulting any
  external KB or running side calculations.
- **SC-002**: For an undersized redo log, the report displays both the
  problem statement and the recommended target sizes for 15 minutes
  and 1 hour of peak writes within the same panel.
- **SC-003**: The panel renders correctly for both the legacy 5.6/5.7
  redo configuration model and the 8.0.30+ dynamic capacity model,
  verified by tests against synthetic captures using each model.
- **SC-004**: The panel renders gracefully when configuration variables
  or the `Innodb_os_log_written` counter are missing, by showing the
  applicable "unavailable" state rather than failing the report
  generation, verified by tests.
- **SC-005**: The "warning when coverage is below 15 minutes" behavior
  is unambiguous: the warning line either renders or does not, with no
  intermediate "maybe" state, verified by tests covering the
  under-sized case and the well-sized case for both configuration
  models.

## Assumptions

- The pt-stalk captures pointed at My-gather typically contain both
  `-variables` (single-shot SHOW GLOBAL VARIABLES output) and
  `-mysqladmin` (extended-status time-series including
  `Innodb_os_log_written`). The panel relies on both being present;
  graceful-degradation states cover the case where either is missing.
- The observed pt-stalk capture window per trigger is typically much
  shorter than 15 minutes (about 30 seconds is common). The panel
  collapses the rolling window to the full available capture window
  in that case and labels the coverage value so the reader knows the
  peak is observed over the available window rather than a true
  15-minute window.
- The KB0010732 methodology source is referenced by article number
  only, with no need to embed the article body in the report.
- The new panel is additive and does not change the behavior of the
  existing Redo Log advisor rules in `findings/rules_redo.go`. Those
  rules continue to flag checkpoint-age utilization and pending I/O
  saturation; this panel covers the orthogonal capacity-vs-write-rate
  sizing question that no existing rule addresses.
