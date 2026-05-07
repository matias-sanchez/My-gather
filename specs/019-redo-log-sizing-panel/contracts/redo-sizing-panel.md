# Contract: Redo log sizing panel

**Feature**: 019-redo-log-sizing-panel
**Owner**: `render/redo_sizing.go`
**Consumers**: `render/build_view.go`, `render/templates/db.html.tmpl`

## Function contract

```go
// computeRedoSizing produces the view payload for the Redo log sizing
// panel under the InnoDB Status section. Returns nil when the report
// has no DBSection (the panel does not render at all in that case).
// In every other case it returns a non-nil view; the view's State
// field communicates which subset of the panel can be populated:
//
//   - "ok"               — configured space and observed rate both known
//   - "config_missing"   — variables for configured redo space absent
//   - "rate_unavailable" — Innodb_os_log_written absent or insufficient
//   - "no_writes"        — observed peak rate is zero
//
// The KB0010732 citation is always populated.
func computeRedoSizing(r *model.Report) *redoSizingView
```

## View struct contract

```go
type redoSizingView struct {
    // State drives which template branches render.
    // Values: "ok" | "config_missing" | "rate_unavailable" | "no_writes"
    State string

    // ConfiguredBytes is the total redo log capacity in bytes.
    // Zero when State == "config_missing".
    ConfiguredBytes float64
    // ConfiguredText is the human-readable form, or "unavailable".
    ConfiguredText string
    // ConfigSource names the variable(s) used:
    //   "innodb_redo_log_capacity"                     (8.0.30+ model)
    //   "innodb_log_file_size x innodb_log_files_in_group" (5.6/5.7 model)
    //   ""                                              (config_missing)
    ConfigSource string

    // ObservedRateBytesPerSec is the average rate over the entire capture.
    // Zero when State == "rate_unavailable".
    ObservedRateBytesPerSec float64
    ObservedRateText        string // "12.5 MiB/s" or "unavailable"
    ObservedRatePerMinText  string // "750 MiB/min" or "unavailable"

    // PeakWindowSeconds is the actual rolling-window length used.
    // 900 (== 15 minutes) when the capture is at least 15 minutes long;
    // otherwise the full available capture window in seconds.
    PeakWindowSeconds float64
    // PeakWindowLabel is the rendered window descriptor, e.g.
    // "15-minute" or "available 28-second".
    PeakWindowLabel string
    // PeakRateBytesPerSec is the peak rate observed over the rolling
    // window of length PeakWindowSeconds. Zero when State !=
    // "ok"/"under_sized" (i.e. when no rate is available or the rate
    // is uniformly zero).
    PeakRateBytesPerSec float64
    PeakRateText        string

    // CoverageMinutes is configured_bytes / peak_rate, in minutes.
    // Zero when CoverageText == "n/a".
    CoverageMinutes float64
    CoverageText    string // "12 minutes" | "62 minutes" | "n/a"

    // Recommended target sizes in bytes and human form.
    Recommended15MinBytes float64
    Recommended15MinText  string // "9.0 GiB" | "n/a"
    Recommended1HourBytes float64
    Recommended1HourText  string // "36.0 GiB" | "n/a"

    // UnderSized is true iff State == "ok" AND CoverageMinutes < 15.
    // The template uses this as the gate for the warning line.
    UnderSized   bool
    WarningLine  string // "Current redo space holds only X minutes of peak writes - consider raising"

    // KBReference is always populated; the template renders it as the
    // panel's methodology citation.
    KBReference string // "Percona KB0010732"
}
```

## Behavioral contract

1. **8.0.30+ detection**: When `innodb_redo_log_capacity` is present
   and parses to a positive float, `ConfiguredBytes` equals that value
   and `ConfigSource == "innodb_redo_log_capacity"`. The legacy
   variables, if present, are ignored.
2. **5.6/5.7 detection**: When `innodb_redo_log_capacity` is absent
   (or zero / unparseable), but `innodb_log_file_size` and
   `innodb_log_files_in_group` are both present and positive,
   `ConfiguredBytes == innodb_log_file_size * innodb_log_files_in_group`
   and
   `ConfigSource == "innodb_log_file_size x innodb_log_files_in_group"`.
3. **Configuration missing**: When neither model can be evaluated,
   `State == "config_missing"`, `ConfiguredText == "unavailable"`,
   and `CoverageText == "n/a"`.
4. **Rate computation**: The average rate (`ObservedRateBytesPerSec`)
   is `sum(deltas) / observed_seconds`, identical in spirit to
   `findings/inputs.go::counterRatePerSec` but computed locally so
   the panel does not depend on the findings package. Two
   per-sample normalisations apply before the sum:
   - **Cold-start tally skip (first block only)**: pt-mext records
     the cold-start counter value at index 0 of the first snapshot;
     this is not a per-interval delta and is excluded from the sum.
     `model.MergeMysqladminData` strips the equivalent raw tally
     from every snapshot AFTER the first by replacing it with a NaN
     boundary slot and appending `src[1:]`, so the first finite
     sample of every post-boundary block IS a real per-interval
     delta and is included.
   - **Negative-delta clamp**: a server restart mid-capture
     re-zeroes `Innodb_os_log_written`, producing a negative delta
     on the next sample. Negative deltas are clamped to 0 (a reset
     is not a write of negative bytes); this also defends against
     any other counter-reset scenario.

   **Per-block observed-seconds left bound**: each block's
   contribution to `observed_seconds` is `timestamps[endIdx-1] -
   timestamps[leftBoundIdx]`, where `leftBoundIdx` is derived by the
   canonical `blockLeftBoundIdx` helper. For the first block the
   left bound is `timestamps[startIdx]` because the cold-start tally
   at `startIdx` is excluded from the sum. For every post-boundary
   block the left bound is `timestamps[startIdx-1]` (the
   snapshot-boundary timestamp): `model.MergeMysqladminData`'s
   `NaN+src[1:]` append makes the first INCLUDED delta at `startIdx`
   cover the interval `(timestamps[startIdx-1], timestamps[startIdx]]`,
   so the observed-seconds denominator MUST include that interval to
   match the bytes counted into the numerator.
5. **Rolling-window peak**: `PeakRateBytesPerSec` is the maximum, over
   every contiguous block of samples, of `bytes_in_window /
   actual_window_seconds` for windows whose `actual_window_seconds`
   is at least `PeakWindowSeconds`. The walk uses a sliding window
   that retains the SMALLEST window of length >= target ending at
   each sample (so any larger window cannot dilute the rate).
   Sub-window prefix spans at the start of each block are NOT
   candidates: the contract requires a full target-length window
   before a rate qualifies as a peak. NaN slots (snapshot
   boundaries) split blocks; a window may not span a NaN slot.
   The cold-start tally skip and negative-delta clamp from item 4
   apply here as well.

   **Fallback longest-block span**: when no block reaches the target
   window, the rolling-window walk collapses to a single fixed
   window equal in length to the longest block. The longest-block
   span is measured as `timestamps[endIdx-1] -
   timestamps[blockLeftBoundIdx(b)]`, the same canonical
   span-derivation used by the observed-seconds accumulator in
   item 4. For a post-boundary block this means the fallback span
   anchors at `timestamps[startIdx-1]` (the snapshot-boundary
   timestamp), not `timestamps[startIdx]`, so the wall-clock window
   reflects the interval actually covered by the block's first
   included delta.
6. **Window collapse**: If the longest contiguous block is shorter than
   900 seconds, `PeakWindowSeconds` is set to the longest block's
   duration in seconds and `PeakWindowLabel` reflects the actual
   window length, computed as `math.Floor(window)` so the label
   never overstates the observed window length (e.g. a 28.6s
   longest block renders as `"available 28-second"`, never
   `"available 29-second"`). The peak in that case is the rate
   over the longest contiguous block.
7. **Coverage**: `CoverageMinutes == ConfiguredBytes / PeakRateBytesPerSec / 60`.
   Rendered as the integer floor (rounded down) so a 7.8-minute
   coverage reports as "7 minutes" and the warning fires. The
   `computeRedoSizing` helper guards `PeakRateBytesPerSec > 0`
   before this division: degenerate timestamp inputs that yield a
   zero peak rate while `rateOK` would otherwise be true are
   reclassified as `rate_unavailable` rather than dividing by zero.
8. **Recommended sizes**: `PeakRateBytesPerSec * 900` for 15 minutes,
   `PeakRateBytesPerSec * 3600` for 1 hour. Both rendered with
   `reportutil.HumanBytes`.
9. **Warning gate**: The `UnderSized` flag is set, and `WarningLine`
   populated, iff `State == "ok"` AND `CoverageMinutes < 15`. Coverage
   exactly equal to 15 does not trigger the warning.
10. **KB citation**: `KBReference == "Percona KB0010732"` in every state.

## Test contract

`render/redo_sizing_test.go` MUST cover:

- T1: 5.7 server, well-sized (configured 4 GiB, peak rate that yields >=
  15 minutes coverage). Asserts `State == "ok"`, `UnderSized == false`,
  `ConfigSource == "innodb_log_file_size x innodb_log_files_in_group"`,
  `ConfiguredText == "4.0 GiB"`, `WarningLine == ""`.
- T2: 5.7 server, under-sized (configured 256 MiB, peak rate that yields
  < 15 minutes coverage). Asserts `State == "ok"`, `UnderSized == true`,
  `WarningLine` matches `"Current redo space holds only N minutes of peak
  writes - consider raising"` with N being the floor minutes value.
- T3: 8.0.30+ server, well-sized (configured 16 GiB via
  `innodb_redo_log_capacity`, even though `innodb_log_file_size` is
  also present at the legacy value). Asserts `State == "ok"`,
  `UnderSized == false`,
  `ConfigSource == "innodb_redo_log_capacity"`, `ConfiguredText == "16.0
  GiB"`.
- T4: 8.0.30+ server, under-sized (configured 512 MiB via
  `innodb_redo_log_capacity` against a high-write-rate fixture).
  Asserts `State == "ok"`, `UnderSized == true`, `WarningLine`
  populated.
- T5: Variables missing entirely. Asserts `State == "config_missing"`,
  `ConfiguredText == "unavailable"`, `CoverageText == "n/a"`,
  `KBReference == "Percona KB0010732"`.
- T6: `Innodb_os_log_written` counter missing. Asserts
  `State == "rate_unavailable"`, `ObservedRateText == "unavailable"`,
  `CoverageText == "n/a"`, `Recommended15MinText == "n/a"`.
- T7: Capture window shorter than 15 minutes. Asserts that
  `PeakWindowSeconds < 900`, `PeakWindowLabel` reflects the actual
  available window in seconds, and the panel still produces a coverage
  estimate using that shorter window.

All tests construct synthetic `*model.Report` values directly (no
filesystem fixtures) so they remain fast and isolated from parser
behavior.
