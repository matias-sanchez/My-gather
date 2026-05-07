# Feature Specification: Synchronized Chart Zoom + Per-Window Legend Stats

**Feature Branch**: `017-chart-zoom-sync-stats`
**Created**: 2026-05-07
**Status**: Ready for planning
**Input**: User description: bundle GitHub issues #52 (synchronized x-axis zoom across all
charts) and #51 (per-series Min/Max/Avg in each chart's legend, computed over the
visible time window) into one feature because they share the same chart-layer surgery.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Zooming One Chart Zooms Every Chart (Priority: P1)

A support engineer reading a report wants to focus on a specific time window. They
drag-zoom on any one chart. Every other chart on the page snaps to the same time
window so the engineer can correlate behaviour across collectors without zooming
each chart individually. Charts inside currently-collapsed sections also adopt the
new window so when the engineer expands them later, they already match.

**Why this priority**: This is the primary value of the feature. Without sync,
correlating spikes across charts requires manual zoom on each one.

**Independent Test**: Open a report with at least two registered time-series charts
in different sections. Drag-zoom chart A to a sub-range. Verify chart B's x-axis
matches the same range. Collapse a section, zoom on a still-visible chart, expand
the collapsed section, verify the chart in it now shows the zoomed window.

**Acceptance Scenarios**:

1. **Given** a report with multiple registered charts, **When** the user drag-zooms on
   any chart's x-axis, **Then** every other registered chart updates its x-axis to
   the exact same `{min, max}` window.
2. **Given** a chart inside a collapsed `<details>` section, **When** the user
   zooms a visible chart and then expands the collapsed section, **Then** the
   newly-revealed chart already shows the zoomed window with no extra interaction.
3. **Given** any registered chart is zoomed, **When** the user clicks the
   reset-zoom button on any chart, **Then** every registered chart resets to its
   own data's full extent and re-broadcasts the unzoomed state.
4. **Given** the HLL sparkline (intentionally not registered), **When** any other
   chart is zoomed, **Then** the sparkline does not change — the carve-out is
   preserved.

---

### User Story 2 - Legend Shows Stats For The Visible Window (Priority: P1)

The same engineer wants to know the Min, Max, and Avg of each series for the
window they are currently looking at, not for the full capture. Each legend pill
displays per-series Min, Max, and Avg computed over the visible window only.
Whenever the window changes (zoom in, zoom out, pan, reset, or sync from another
chart), the values recompute.

**Why this priority**: Without windowed stats, legend numbers either lie about
the visible window or are absent entirely; the engineer falls back to mental
arithmetic on the chart's pixels.

**Independent Test**: For each chart, the legend pill for each series shows three
numeric stats (Min, Max, Avg). Drag-zoom to a sub-range that excludes the
historical maximum and verify the displayed Max drops to the new window's
maximum. Reset zoom and verify the values return to the full-capture stats.

**Acceptance Scenarios**:

1. **Given** any chart with at least one numeric series, **When** the chart
   renders, **Then** each series' legend pill displays its Min, Max, and Avg
   computed over the full data window.
2. **Given** the user zooms or pans, **When** the visible x-window changes,
   **Then** every visible chart's legend pill re-renders Min/Max/Avg using only
   the samples whose timestamps fall inside the new window.
3. **Given** a stacked chart (e.g., processlist by state), **When** the legend
   stats render, **Then** the values come from the per-series un-stacked raw
   samples, not from the cumulative stacked values that uPlot draws.
4. **Given** a window that contains no non-null samples for some series,
   **When** the legend stats render for that series, **Then** the pill shows a
   neutral placeholder (`–`) for Min, Max, and Avg without breaking layout.

---

### Edge Cases

- A chart whose data range is narrower than the broadcast window (sparse
  collector, late-starting capture) accepts the broadcast window unchanged and
  paints empty plot regions outside its data — this is the desired behaviour and
  surfaces the gap to the reader.
- The reset-zoom button broadcasts `{null, null}` semantics by reverting every
  chart to its own data extent; sync remains in effect after reset (a subsequent
  zoom on any chart still propagates).
- Panning (uPlot supports drag-pan when configured) is treated identically to
  zoom — both produce a new `{min, max}` window and broadcast through the same
  store.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: All registered time-series charts MUST share a single in-memory
  shared store that holds the current `{min, max}` x-axis window.
- **FR-002**: Any registered chart's x-axis change (drag-zoom, drag-pan, or
  reset) MUST publish the new `{min, max}` to the shared store.
- **FR-003**: When the shared store updates, every registered chart MUST apply
  the new `{min, max}` to its x-scale, including charts inside collapsed
  sections.
- **FR-004**: Newly registered charts MUST adopt the current shared-store window
  on registration so a chart that mounts after a zoom event still matches.
- **FR-005**: The reset-zoom button MUST reset every registered chart to its own
  data extent and republish so the shared store reflects "unzoomed" state.
- **FR-006**: The broadcaster MUST be re-entrancy-safe — applying a window to
  chart B's x-scale MUST NOT trigger a second broadcast that re-applies to
  chart A.
- **FR-007**: Each chart's legend MUST display per-series Min, Max, and Avg
  computed over the currently-visible x-window.
- **FR-008**: Every shared-store window change MUST trigger recomputation of
  every visible chart's legend stats.
- **FR-009**: Stacked-chart legend stats MUST be computed from the per-series
  un-stacked raw values, not from cumulative stacked values.
- **FR-010**: A series with no non-null samples in the visible window MUST
  display a neutral placeholder (`–`) for Min, Max, and Avg.
- **FR-011**: The HLL sparkline (which intentionally does not register with the
  chart registry) MUST NOT participate in zoom sync and MUST NOT receive
  windowed stats — it has no legend pills.
- **FR-012**: Component-level tests MUST cover: (a) the zoom-sync event flow
  (zoom on chart A → store updates → chart B applies the new window) and
  (b) the windowed-stats recomputation (legend values change when the shared
  window changes).
- **FR-013**: No governed first-party source file may exceed 1000 lines after
  the change (Principle XV).
- **FR-014**: Output MUST remain byte-identical between two runs on the same
  input (Principle IV) — sync state is window-local at view time and MUST NOT
  leak into rendered HTML.

### Canonical Path Expectations *(include when changing existing behavior)*

- **Canonical owner/path**: `render/assets/app-js/00.js` already owns the
  `CHARTS` registry, `mountResetZoomButton`, and `mountLegend`. The new shared
  store, the `setScale`-broadcast hook, and the windowed-stats helper extend
  that owner. A new ordered part (`render/assets/app-js/05.js`) is added for the
  shared store and stats helpers because every existing part is already within
  ~100 lines of the 1000-line cap (Principle XV). Concatenation order is owned
  by the embed step in `render/assets.go` and remains lexical/sorted, so the
  new part loads after the existing ones.
- **Old path treatment**: the per-chart reset-zoom click handler is replaced
  in-place to publish through the shared store rather than calling
  `p.setScale("x", …)` on a single plot; the old single-chart reset path is
  deleted in the same change. `mountLegend`'s signature gains a stats-source
  parameter; every call site (`00.js`, `01.js`, the vmstat tab rebuild path in
  `03.js`) is updated in the same commit. No re-export shim, no internal flag,
  no parallel "synced vs. unsynced" mode.
- **External degradation**: charts whose data window is narrower than the
  broadcast window paint empty regions for the out-of-data portion. This is
  observable to the reader and tested.
- **Review check**: reviewers verify (a) only one chart-sync store exists in
  the JS, (b) only one `mountLegend` signature exists with all call sites
  updated, (c) no parallel "non-synced" zoom code path remains, and (d) the
  HLL-sparkline carve-out is preserved by the unchanged decision not to call
  `registerChart` on it.

### Key Entities

- **Shared zoom store**: in-memory singleton owned by `app-js/05.js`, exposing
  `getWindow()`, `setWindow({min, max})`, `subscribe(fn)`, and `reset()`. State
  is `{min, max}` numeric timestamps (uPlot's x-scale unit) plus a generation
  counter used by the re-entrancy guard.
- **Windowed-stats record**: derived per series per render — `{min, max, avg, count}`
  computed over samples whose timestamps fall inside `[storeMin, storeMax]`.
- **Chart registry entry** (existing, extended): every entry already carries
  `{plot, el, opts}`. The extension adds `data` (timestamps + per-series raw
  values, including the un-stacked raw for stacked charts) and a
  `setLegendStats` callback returned by `mountLegend` so the store's subscriber
  can recompute and re-render that legend.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With two or more time-series charts on the page, drag-zooming any
  one chart updates every other registered chart to the same `{min, max}`
  within one animation frame (no extra user clicks required).
- **SC-002**: Each chart's legend shows three numeric stats (Min, Max, Avg) per
  series; the values match a brute-force recomputation over the samples whose
  timestamps fall inside the current `{min, max}`.
- **SC-003**: Zooming a chart and then expanding a previously-collapsed section
  containing another chart shows that chart already at the zoomed window with
  no additional interaction.
- **SC-004**: Clicking reset-zoom on any chart returns every chart to its full
  data extent and the legend stats return to the full-capture values.
- **SC-005**: `go test ./...` passes, including new tests that grep the
  embedded JS for the canonical zoom-sync and windowed-stats markers.
- **SC-006**: No first-party source file exceeds 1000 lines.
- **SC-007**: Two consecutive `my-gather` runs on the same fixture produce
  byte-identical HTML — sync state does not leak into the rendered output.

## Assumptions

- All time-series charts on a report share the same x-axis unit (Unix
  timestamps from pt-stalk samples). Cross-collector mixing is already the
  case on shipped reports.
- Heterogeneous data ranges across charts are acceptable — when one chart's
  data does not cover the broadcast window, painting an empty region is the
  desired behaviour (it surfaces the gap to the reader).
- The HLL sparkline's existing decision not to register with the chart
  registry is preserved as the canonical way to opt out of sync; no separate
  "is-synced" flag is introduced.
- Tests for the zoom-sync flow and the windowed-stats recomputation are
  written as Go tests that grep the embedded JS for canonical markers — that
  is the existing pattern in `render/assets_test.go`; no JS test runner is
  introduced.
