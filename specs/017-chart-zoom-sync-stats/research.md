# Research: Synchronized Chart Zoom + Per-Window Legend Stats

## Existing chart layer landmarks

- `render/assets/app-js/00.js`
  - `CHARTS = []` registry; `registerChart(plot, containerEl, options)` and
    `unregisterChart(plot)`. Each entry is `{plot, el, opts}`.
  - `resizeAllCharts()` already iterates the registry and skips entries whose
    container is detached or has zero width — proves a registry-walk pattern is
    the canonical way to act on every chart at once.
  - `basePlotOpts` builds uPlot config; cursor is `drag.x = true`,
    `setScale: true`, `uni: 15`; double-click resets the x-zoom (uPlot built-in).
  - `mountResetZoomButton(containerEl, getPlot)` attaches a `⛶` button that
    resets a single chart by calling `p.setScale("x", { min: xs[0], max: xs[xs.length-1] })`.
  - `mountLegend(containerEl, series, plot, opts)` builds the per-chart pill
    legend; today it shows series swatch + label only — no stats.
- `render/assets/app-js/01.js`
  - `buildLineChart(el, data, unit)` constructs a multi-line uPlot, calls
    `registerChart(plot, el, opts)`, then `mountLegend(el, series, plot)`.
  - `buildStackedChart(el, data, unit, hiddenLabels, onRebuild)` builds the
    stacked-bar variant. Stores per-series un-stacked raw values on
    `plot.__rawData` (used today by the tooltip). Cumulative values live in
    `plotData`, which uPlot draws.
- `render/assets/app-js/03.js`
  - vmstat tab path destroys + re-mounts a chart and the legend on tab change;
    must keep working with the new mountLegend signature.
  - `renderHLLSparkline` deliberately does NOT call `registerChart` (existing
    comment: "Do NOT registerChart — sparkline follows its own container and
    should not participate in global reset-zoom flows"). This is the canonical
    way to opt a chart out of sync; preserved without modification.
- `render/assets.go` concatenates `app-js/*.js` in lexical order at build time
  via `embed`. Adding `05.js` slots cleanly after `04.js`.

## Why a new ordered part (05.js)

`wc -l render/assets/app-js/*.js` on `main` shows 00–03 are 892–899 lines and
04.js is 490. Principle XV caps governed source files at 1000 lines and is
mechanically enforced by `tests/coverage/file_size_test.go`. Adding ~100–200
lines of zoom-store + stats logic to any of 00–03 would put that file over the
cap. 05.js is the canonical home for the new behaviour and keeps every file
under the limit.

## Chart sync mechanics (what uPlot supports)

- uPlot fires `setScale` hooks any time a scale changes, including from
  user-initiated drag-zoom. The hook receives `(u, scaleKey)`. Reading
  `u.scales.x.min` / `u.scales.x.max` after the hook gives the current window.
- Calling `chart.setScale("x", {min, max})` on a chart whose container is
  hidden (e.g., inside a closed `<details>`) is safe: uPlot stores the new
  scale and applies it on next paint when the container becomes visible.
  Verified by inspection of `resizeAllCharts`'s skip-if-zero-width logic — the
  same charts handle late re-paints correctly.
- Re-entrancy: the broadcaster MUST guard against the cycle "chart A's user
  zoom → store update → setScale on chart B → chart B's setScale hook fires →
  store update → setScale on chart A". A monotonic generation counter is the
  smallest robust guard: each broadcast bumps the generation; chart hooks
  ignore setScale events whose source is the current broadcast generation.
  Concretely the broadcaster tags itself as the source by setting a flag
  before calling `setScale` on each chart and clearing it after; chart hooks
  skip when the flag is set.

## Windowed stats mechanics

- Each registered chart's data is `{timestamps[], series[{label, values[]}]}`.
- For a window `[lo, hi]`, find the index range `[i0, i1]` via binary search on
  `timestamps`. For each series, scan `values[i0..i1]`, ignoring `null`/`NaN`,
  to compute `min`, `max`, `sum`, `count`. `avg = sum / count` if `count > 0`.
- Stacked charts: stats source is the un-stacked per-series raw values
  (already cached on `plot.__rawData[idx]` for the tooltip). The cumulative
  `plotData` would lie about per-series Min/Max/Avg.
- Empty window for a series (count == 0): legend shows `–` for all three.

## Legend display

- Pill content today: `<span class="swatch"></span><span class="lbl">label</span>`.
- New: append `<span class="stats">min · avg · max</span>` to each pill. CSS
  lives in `render/assets/app-css/03.css` (which currently has the most slack;
  701/1000 lines).
- Numeric formatting: reuse the existing `formatYTick` helper (in 00.js) so
  pill stats render in the same scale (`1.2M`, `300`, `99.9 %`) as the y-axis
  ticks.

## Test pattern

`render/assets_test.go::TestFeedbackWorkerClientContract` greps `embeddedAppJS`
for required and forbidden substrings. New tests follow the same shape:

- Required markers (must be present): `__chartSyncStore`,
  `chartSyncStore.subscribe`, `chartSyncStore.broadcast`,
  `computeWindowedStats`, `setLegendStats`.
- Forbidden markers (must be absent): the old single-chart reset pattern that
  doesn't go through the store, any second copy of `mountLegend` with the old
  3-arg signature, any "synced:false" feature flag.

## Determinism check

The new behaviour is entirely client-side. Rendered HTML changes only because
each legend pill now contains a `<span class="stats">` element computed from
the embedded `data` payload. Both runs of `my-gather` against the same input
produce the same payload, so the same HTML — byte-identical determinism is
preserved.

## Decisions

1. Shared store lives on `window.__chartSyncStore`, defined in `05.js`. Single
   instance per page, no module system in this codebase.
2. Re-entrancy guard: an `_isBroadcasting` boolean toggled by the broadcaster
   around each `setScale` call.
3. `registerChart` is extended with an optional `data` argument:
   `registerChart(plot, el, opts, data)`. All current call sites updated in
   the same commit. `data` is `{timestamps, series, rawSeries?}` where
   `rawSeries` is provided for stacked charts (un-stacked raw values).
4. `mountLegend` is extended with an optional `statsSource` argument and
   returns `{setStats}`. The store subscriber calls `setStats(window)` for
   every chart on every window change. All current call sites updated.
5. Reset-zoom button on any chart calls `chartSyncStore.reset()`, which
   iterates the registry and resets each chart to its own data extent, then
   sets the store window to `null` (= no synced zoom).
6. Legend stat formatter reuses `formatYTick` from 00.js.
7. The HLL sparkline carve-out is preserved by not calling `registerChart` —
   no new opt-out mechanism is added.
