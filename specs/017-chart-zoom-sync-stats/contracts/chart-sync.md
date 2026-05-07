# Contract: Chart Sync Store + Windowed Legend Stats

This contract defines the JS-internal API introduced by feature
017-chart-zoom-sync-stats. It is consumed only by the report's embedded JS.

## 1. `chartSyncStore` (shared zoom store)

Singleton attached to `window.__chartSyncStore`, defined in
`render/assets/app-js/05.js`.

### State

```text
{
  win:        { min: number|null, max: number|null },   // current x-window
  generation: number,                                   // monotonic, for re-entrancy guard
  isBroadcasting: boolean,                              // set true while applying to all charts
  subscribers: Array<function(win, generation)>
}
```

`win.min === null && win.max === null` means "no synced zoom" (full extent
per chart).

### Methods

- `getWindow() -> { min, max }` — returns the current store window.
- `setWindow({min, max}, sourcePlot?)` — publishes a new window. Bumps
  `generation`. Walks the chart registry; for every entry whose plot is not
  `sourcePlot` and whose container is connected, calls
  `plot.setScale("x", { min, max })` with `isBroadcasting = true` set around
  each call so the per-chart `setScale` hook can skip publishing back. After
  the walk, fires every subscriber.
- `reset()` — restores every registered chart to its own data extent. For each
  registered chart, calls `plot.setScale("x", { min: xs[0], max: xs[xs.length-1] })`
  with `isBroadcasting = true`. Then sets `win = {min:null, max:null}` and
  fires subscribers.
- `subscribe(fn) -> unsubscribe` — registers a subscriber called as
  `fn(win, generation)` after every store update. Returns a function that
  removes the subscriber.

### Per-chart hook contract

Every registered chart's uPlot config gains a `setScale` hook on the `x`
scale. The hook:

1. If `chartSyncStore.isBroadcasting` is true, returns immediately.
2. Reads `u.scales.x.min`, `u.scales.x.max`.
3. Calls `chartSyncStore.setWindow({min, max}, u)`.

This hook is added inside `basePlotOpts` so every chart that uses
`basePlotOpts` participates automatically.

### `registerChart` extension

`registerChart(plot, containerEl, options, data)` — the optional `data`
argument is `{timestamps: number[], series: Array<{label, values}>, rawSeries?: Array<{label, values}>}`.

- `timestamps` is the un-bucketed Unix-second array.
- `series` is what's drawn (post-bucketing, post-stacking for stacked charts).
- `rawSeries` is supplied by stacked charts; it carries the un-stacked raw
  per-series values. When present, windowed stats use `rawSeries` instead of
  `series`.

A chart that does not register (HLL sparkline) is intentionally excluded from
sync and from windowed stats.

## 2. `computeWindowedStats(timestamps, series, win)` (helper)

Defined in `render/assets/app-js/05.js`.

Input:

- `timestamps: number[]` — sorted ascending (Unix seconds).
- `series: Array<{label, values}>` — `values[i]` aligned to `timestamps[i]`.
- `win: {min, max}` — when both are `null`, scan the entire array.

Output:

- `Array<{min, max, avg, count}>` — one entry per series in input order.
  Series with `count === 0` (no non-null samples in window) return
  `{min: null, max: null, avg: null, count: 0}` and the legend renders `–`.

Behaviour:

- Use binary search on `timestamps` to find `[i0, i1]` such that
  `timestamps[i0] >= win.min` and `timestamps[i1-1] <= win.max`. When `win`
  is `{null, null}`, `[i0, i1] = [0, timestamps.length]`.
- Skip `null`, `undefined`, and `NaN` values inside the window.

## 3. `mountLegend` extension

Updated signature:

```text
mountLegend(containerEl, series, plot, opts) -> { setStats(win), destroy() }
```

`opts` gains:

- `statsSource: { timestamps, series }` — the data the stats are computed
  against. For stacked charts the caller passes the un-stacked raw values
  here.

Returned object:

- `setStats(win)` — recompute and re-render the per-pill stats for the given
  window. Called by the store subscriber on every window change.
- `destroy()` — remove the legend node and unsubscribe (used by the vmstat tab
  rebuild path).

The legend pill HTML adds one element:

```html
<span class="stats" data-stats="min·avg·max">…</span>
```

When `count === 0` for a series, the element renders `–`.

## 4. Reset-zoom button contract

`mountResetZoomButton(containerEl, getPlot)` is unchanged at the call-site
level. The click handler now calls `chartSyncStore.reset()` instead of
`getPlot().setScale("x", …)`. This guarantees the reset propagates to every
registered chart and the store reflects "no synced zoom".

## 5. Embedded-JS canonical markers (test contract)

`render/assets_test.go` MUST grep `embeddedAppJS` for the following:

Required substrings (presence asserted):

- `__chartSyncStore`
- `chartSyncStore.subscribe`
- `chartSyncStore.setWindow`
- `chartSyncStore.reset`
- `computeWindowedStats`
- `setStats(`
- `series-pill-stats` (the new pill element class)

Forbidden substrings (absence asserted):

- A reset-zoom click handler that calls `setScale("x"` directly without
  routing through the store. Implementation places all `setScale("x"` calls
  inside `chartSyncStore`.
- Any second `mountLegend(` definition.

## 6. Determinism contract

The store and stats are computed in the browser at view time. The HTML
embedded by `render/` is unaffected by sync state. Two consecutive
`my-gather` runs against the same input directory MUST produce byte-identical
HTML; the existing CI determinism job covers this.
