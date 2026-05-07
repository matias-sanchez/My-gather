# Tasks: Synchronized Chart Zoom + Per-Window Legend Stats

**Feature**: 017-chart-zoom-sync-stats
**Input**: [spec.md](./spec.md), [plan.md](./plan.md), [research.md](./research.md), [contracts/chart-sync.md](./contracts/chart-sync.md)

Tasks below are dependency-ordered. Tasks marked `[P]` may run in parallel
with other `[P]` tasks in the same numbered group.

## Phase 0 — Pre-flight

- **T001** Confirm branch is `017-chart-zoom-sync-stats` and worktree is clean
  apart from the new `specs/017-chart-zoom-sync-stats/` directory and the
  updated `.specify/feature.json`.

## Phase 1 — Shared zoom store + windowed-stats helper (new file)

- **T010** Create `render/assets/app-js/05.js` with:
  - the `chartSyncStore` singleton bound to `window.__chartSyncStore`
    implementing `getWindow`, `setWindow`, `reset`, `subscribe` per
    `contracts/chart-sync.md`;
  - the `computeWindowedStats(timestamps, series, win)` helper using binary
    search to find the index range and a single linear scan per series;
  - a `formatStat(v)` helper that delegates to `formatYTick` (defined in
    `00.js`) for consistent number formatting;
  - the store-subscriber wiring inside an IIFE that runs after `initCharts`
    completes (registers a single `chartSyncStore.subscribe` callback that
    walks the chart registry and calls each entry's
    `entry.setLegendStats(win)` if present).

## Phase 2 — Extend canonical owner (00.js)

- **T020** In `render/assets/app-js/00.js`:
  - extend `registerChart(plot, containerEl, options, data)` to accept and
    store the optional `data` payload on the registry entry;
  - inside `basePlotOpts`, add a `setScale` hook on the `x` scale that, when
    `chartSyncStore.isBroadcasting` is false, reads `u.scales.x.min` /
    `u.scales.x.max` and calls `chartSyncStore.setWindow({min, max}, u)`;
  - update `mountResetZoomButton`'s click handler to call
    `chartSyncStore.reset()` instead of `getPlot().setScale("x", …)`;
  - extend `mountLegend(containerEl, series, plot, opts)` to:
    - read `opts.statsSource` (`{timestamps, series}`),
    - render a third `<span class="series-pill-stats">…</span>` element
      inside each pill,
    - return `{setStats(win), destroy()}`,
    - store `setStats` on the registry entry as
      `entry.setLegendStats = setStats` so the store subscriber finds it;
  - on first call after `initCharts`, every entry's `setStats({min:null, max:null})`
    is invoked once so the legend renders full-extent stats at first paint.

## Phase 3 — Update call sites in the same commit (Principle XIII)

- **T030** [P] In `render/assets/app-js/01.js`:
  - `buildLineChart`: pass `data` to `registerChart(plot, el, opts, data)`;
    pass `statsSource: { timestamps: data.timestamps, series: data.series }`
    to `mountLegend`.
  - `buildStackedChart`: build an un-stacked `rawSeries` (using `series` and
    `tooltipRaw` already constructed) and pass it as `statsSource:
    { timestamps: bucketed.timestamps, series: rawSeries }`.
- **T031** [P] In `render/assets/app-js/02.js` (mysqladmin chart): pass
  `data` to `registerChart` and `statsSource` to `mountLegend` for every
  composed sub-chart.
- **T032** [P] In `render/assets/app-js/03.js`:
  - vmstat tab rebuild path: pass `data` to `registerChart` and
    `statsSource` to `mountLegend` on every rebuild;
  - on chart destruction, call the returned `legend.destroy()` to drop the
    stats subscription before re-mounting.
  - `renderHLLSparkline`: leave the deliberate non-registration as-is; this
    is the canonical opt-out (covered by a unit-marker test below).

## Phase 4 — Embed and styles

- **T040** Confirm `render/assets.go` already concatenates `app-js/*.js` in
  lexical order; if it relies on a hard-coded list, append `05.js`.
- **T041** Add CSS for the new `.series-pill-stats` element to
  `render/assets/app-css/03.css` (the part with the most slack). Style:
  small monospace, `--fg-muted` colour, separated from the label by an
  em-space.

## Phase 5 — Tests (Go-side string-grep against embedded JS)

- **T050** Extend `render/assets_test.go` with `TestChartZoomSyncContract`:
  - assert `embeddedAppJS` contains `__chartSyncStore`,
    `chartSyncStore.subscribe`, `chartSyncStore.setWindow`,
    `chartSyncStore.reset`, `computeWindowedStats`, `series-pill-stats`,
    `setStats(`;
  - assert `embeddedAppJS` does NOT contain a `setScale("x"` call outside
    `chartSyncStore` (canonical-path guard) — implementation places all
    `setScale("x"` calls inside the store;
  - assert `mountLegend(` is defined exactly once.
- **T051** Extend `render/assets_test.go` with `TestVmstatLegendDestroyOnTabSwitch`:
  - grep that the vmstat tab rebuild path in `03.js` calls `.destroy()` on
    the legend handle before re-mounting (guards against subscription leaks).
- **T052** Extend `render/assets_test.go` with `TestHLLSparklineExemptFromSync`:
  - grep that `renderHLLSparkline` does not call `registerChart` (preserves
    the canonical opt-out).
- **T053** Extend the existing render integration test (or add a new one
  under `render/`) that exercises the embed path and asserts `embeddedAppJS`
  contains the marker comment from `05.js` (e.g. `// app-js/05.js — chart
  sync store`), proving the new part is concatenated.

## Phase 6 — Goldens + determinism

- **T060** Run `go test ./...`. If any golden test fails because legend HTML
  now contains the new `<span class="series-pill-stats">` element, regenerate
  via the reviewed update step (`go test ./... -update` or the
  feature-specific helper, whichever is canonical in this repo) and review
  the diff before committing. Document the regeneration in the commit
  message.
- **T061** Re-run `go test ./...` and confirm the determinism job (CI's
  `diff -u` between two report runs) still passes locally:
  `go test ./tests/... -run Determinism` (or whatever the canonical name is
  in this repo).

## Phase 7 — Constitution gates

- **T070** Run `bash scripts/hooks/pre-push-constitution-guard.sh` against
  the staged change set.
- **T071** Run `go test ./tests/coverage/... -run TestFileSize` to confirm no
  governed source file exceeds 1000 lines.
- **T072** Run `go test ./tests/coverage/... -run TestGodocCoverage` (no new
  Go exports are added; the test is a regression sanity check).

## Phase 8 — Documentation pointers

- **T080** Update `CLAUDE.md`, `AGENTS.md`, and `.specify/feature.json` so
  the active feature reads `017-chart-zoom-sync-stats`. Add the prior
  `015-compliance-closure` entry to the historical-features list.

## Phase 9 — Commit, push, PR

- **T090** Commit on branch `017-chart-zoom-sync-stats`. Include in the
  commit body the canonical-path summary and a reference to issues
  `#52` and `#51`.
- **T091** Push the branch.
- **T092** Open the PR with
  `gh pr create --title "Synchronized chart zoom + per-window legend stats (#52, #51)" --body …`.

## Dependency notes

- T020 depends on T010 (the store must exist before the hook can call it).
- T030 / T031 / T032 depend on T020 (the new `mountLegend` signature must
  exist before call sites switch to it).
- T050 / T051 / T052 / T053 depend on the JS implementation being in place.
- T060 depends on T050 to make sure regeneration only updates legend-HTML
  drift, not unrelated drift.
- T070–T072 are the merge gates; they depend on every JS + Go change being
  staged.
