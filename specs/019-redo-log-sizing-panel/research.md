# Research: Redo log sizing panel

**Feature**: 019-redo-log-sizing-panel
**Date**: 2026-05-07

## Methodology source

Percona Knowledge Base article **KB0010732** is the methodology source
for the panel. The user supplied the article reference and instructed
the panel to cite it directly.

The KB describes the canonical sizing recommendation as:
"the redo log should hold at least 15 minutes of writes at the peak
write rate, ideally 1 hour of writes". The panel computes both the
15-minute and 1-hour target sizes and surfaces a warning when the
configured redo space holds less than 15 minutes of peak writes.

## MySQL configuration model

Two models exist and the panel must support both:

1. **MySQL 5.6 / 5.7 / early 8.0**: Total redo space is the product of
   two static variables - `innodb_log_file_size` (per file) times
   `innodb_log_files_in_group` (number of files). Both are static and
   require a server restart to change.

2. **MySQL 8.0.30 and later**: Total redo space is a single dynamic
   variable - `innodb_redo_log_capacity` - in bytes. The legacy
   variables still appear in `SHOW GLOBAL VARIABLES` for backward
   compatibility, but MySQL ignores them and uses
   `innodb_redo_log_capacity` exclusively.

The detection rule, applied inside the single canonical computation
function, is: if `innodb_redo_log_capacity` is present and parses to
a positive number, use it. Otherwise fall back to
`innodb_log_file_size * innodb_log_files_in_group`. This is one
canonical branching path inside one function, not two parallel
implementations (Principle XIII).

## Observed write rate

`Innodb_os_log_written` is the cumulative byte counter for redo log
writes. pt-stalk's mysqladmin extended-status capture stores it in the
counter time-series consumed by the existing My-gather Findings rules
(see `findings/inputs.go::counterTotal` and `counterRatePerSec`). The
existing helper convention is:

- `model.MysqladminData.Deltas[name]` is a `[]float64` of length
  `SampleCount`.
- For counters (`IsCounter[name] == true`), index 0 is the bogus initial
  raw tally and is excluded; later indices are per-sample deltas in the
  variable's native units (bytes for `Innodb_os_log_written`).
- `NaN` slots mark snapshot boundaries inserted by
  `model.MergeMysqladminData`; the rolling-window code must skip them.
- `model.MysqladminData.Timestamps[i]` is the wall-clock timestamp of
  sample `i` (`time.Time`).

The new computation walks the `Innodb_os_log_written` delta slice and
the parallel `Timestamps` slice once, using a sliding-window pointer
to compute the maximum bytes-per-second rate over any 15-minute window
(or the full capture window when shorter). The window walk treats NaN
slots as "drop the in-progress accumulator and restart from the next
non-NaN sample" so cross-snapshot gaps do not get folded into the rate.

## Capture-window-shorter-than-15-minutes case

pt-stalk runs typically capture about 30 seconds per trigger. A literal
"15-minute rolling window" is undefined when the entire capture is
shorter than 15 minutes. The panel collapses the window size to
`min(15 minutes, total observed seconds)` and labels the coverage value
("over the available N seconds") so the reader is not misled into
treating a short-burst peak as a sustained 15-minute peak.

## Where the panel renders

The InnoDB Status section is `<details id="sub-db-innodb">` inside
`render/templates/db.html.tmpl`. It currently contains a `.callouts` div
that ranges over `.InnoDBMetrics`. The new panel renders as an
additional `<div class="callout">`-style block immediately after the
existing callouts, gated on a new `.RedoSizing` view-struct pointer.
This keeps the panel visually adjacent to the existing InnoDB scalars
and preserves the current section structure.

## Reuse of existing helpers

- `reportutil.VariableFloat(r, name)` reads a single
  SHOW GLOBAL VARIABLES value as a float. Used for
  `innodb_redo_log_capacity`, `innodb_log_file_size`, and
  `innodb_log_files_in_group`.
- `reportutil.HumanBytes(v)` renders a byte count using binary units.
  Used for the configured space, the recommended sizes, and the
  observed bytes/sec and bytes/min rate displays.
- `model.MysqladminData.Deltas` and `Timestamps` provide the
  `Innodb_os_log_written` series directly. The rolling-window walk
  is local to `render/redo_sizing.go`; it does not duplicate
  `findings/inputs.go::counterTotal` because that helper sums the
  whole window rather than computing a sliding-window peak.

## Out of scope

- The new panel does not modify, extend, or replace any existing
  `findings/rules_redo.go` rule. Those rules cover orthogonal signals
  (checkpoint age, pending I/O, log-buffer waits) and remain the
  canonical owners of those signals.
- The new panel does not introduce a new findings rule. Sizing
  recommendations are presented inline in the panel itself, not as a
  separate Advisor entry, because the answer is a quantitative
  computation rather than a heuristic threshold and pairing the
  recommendation with the configured-vs-observed numbers in one panel
  matches Principle XI's "humans under pressure" priority.
