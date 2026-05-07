# Implementation Plan: Synchronized Chart Zoom + Per-Window Legend Stats

**Branch**: `017-chart-zoom-sync-stats` | **Date**: 2026-05-07 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/017-chart-zoom-sync-stats/spec.md`

## Summary

Make every report's time-series charts share one in-memory `{min, max}` x-axis
window, so a zoom or pan on any chart instantly updates every other chart on the
page (including charts inside collapsed sections). Recompute and display
per-series Min/Max/Avg in each chart's legend pills over the visible window on
every store update. Implement both behaviours in one chart-layer pass on the
embedded report JS, the canonical chart owner.

## Technical Context

**Language/Version**: Go 1.x (host binary, builds the embedded report); embedded
JS is plain ES5/ES2015-compatible browser code (matches the rest of `app-js/`).
**Primary Dependencies**: existing bundled uPlot (`render/assets/chart.min.js`).
No new Go dependencies, no new JS dependencies.
**Storage**: N/A — sync state is window-local at view time and never persisted
to the rendered HTML (preserves Principle IV byte-identical determinism).
**Testing**: Go tests under `render/` that grep `embeddedAppJS` for canonical
markers — same pattern as `TestFeedbackWorkerClientContract` in
`render/assets_test.go`. Plus a generation-side test that asserts the new
`05.js` part is concatenated into the embedded bundle in deterministic order.
**Target Platform**: self-contained HTML report viewed in any modern browser on
an air-gapped laptop (Principle V).
**Project Type**: single Go binary (`cmd/my-gather`) producing a single HTML
report; the change is internal to `render/`.
**Performance Goals**: every store update applies the new window to all charts
within one animation frame; legend stats recompute in O(N samples × S series)
per chart per update — N is at most a few thousand on shipped fixtures, so a
linear scan is well below 1 ms per chart.
**Constraints**: zero network at runtime (Principle IX), zero file writes
inside the input tree (Principle II), single static binary (Principle I), no
file over 1000 lines (Principle XV), byte-identical output between two runs
(Principle IV).
**Scale/Scope**: a single shipped report has on the order of 5–20 registered
charts. The sync broadcaster is O(charts) per update.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Single Static Binary** — pure JS/Go change, no new CGO, no new runtime
  step. PASS.
- **II. Read-Only Inputs** — no parser change, no write paths added. PASS.
- **III. Graceful Degradation** — a chart whose data is narrower than the
  broadcast window paints empty regions for the gap; this is observable and
  intentional, not a panic. PASS.
- **IV. Deterministic Output** — sync state lives in browser memory; rendered
  HTML is unaffected. The new `05.js` part is concatenated in deterministic
  lexical order by the existing `render/assets.go` embed step. PASS.
- **V. Self-Contained HTML Reports** — no new external assets; new logic ships
  as one more `app-js/` part embedded into the binary. PASS.
- **VI. Library-First Architecture** — change is inside `render/`, no new
  exports, all touched JS continues to live under the existing canonical owner
  (`render/assets/app-js/`). PASS.
- **VII. Typed Errors** — JS-only, no Go error surface. PASS.
- **VIII. Reference Fixtures & Golden Tests** — no parser change, no fixture
  change. Goldens may shift if legend HTML structure changes; if so, regenerate
  via the reviewed `-update` step and review the diff. Plan covers this in
  Phase 2 task list.
- **IX. Zero Network at Runtime** — no network code added. PASS.
- **X. Minimal Dependencies** — uses bundled uPlot only. PASS.
- **XI. Reports Optimized for Humans Under Pressure** — windowed stats and
  cross-chart sync directly serve the incident-response narrative. PASS.
- **XII. Pinned Go Version** — no Go version change. PASS.
- **XIII. Canonical Code Path (NON-NEGOTIABLE)** — see Canonical Path Audit
  below.
- **XIV. English-Only Durable Artifacts** — all artifacts in this plan are
  English. PASS.
- **XV. Bounded Source File Size** — every existing `app-js/` part is within
  ~100 lines of the 1000-line cap. New logic lands in a new part
  (`render/assets/app-js/05.js`) so no existing part crosses the cap. Plan
  covers a `tests/coverage/file_size_test.go` re-run after implementation.

**Canonical Path Audit (Principle XIII)**:

- Canonical owner/path for touched behaviour:
  - `render/assets/app-js/00.js` already owns `CHARTS` registry,
    `mountResetZoomButton`, `mountLegend`, `basePlotOpts`. The new shared
    zoom store, the `setScale` broadcast hook, and the windowed-stats helper
    extend that owner.
  - `render/assets/app-js/05.js` is added as a new ordered part containing
    the shared zoom store + `computeWindowedStats` helper. It is not a
    parallel implementation; it is the new canonical home for behaviour that
    didn't exist before.
- Replaced or retired paths:
  - The per-chart reset-zoom click handler (currently calls
    `p.setScale("x", …)` on a single plot) is replaced in-place to publish
    through the shared zoom store. Old single-chart reset path is deleted in
    the same change.
  - `mountLegend`'s signature gains a stats-source parameter and a returned
    `setLegendStats` callback. Every call site (`00.js` mountLegend internal
    refs, `01.js` line+stacked builders, vmstat tab rebuild in `03.js`) is
    updated in the same commit; no compat shim, no overload. Tests grep the
    embedded JS to assert no caller passes the old 4-arg form.
  - `registerChart` now takes an additional optional `data` payload (so the
    store subscriber can recompute legend stats and the shared store can ask
    each chart for its data extent on reset). All current call sites pass
    the payload; no two-form variant remains.
- External degradation paths:
  - Charts whose data window is narrower than the broadcast window paint
    empty plot regions for the out-of-data portion. Observable to the
    reader, covered by the dedicated edge-case test. Routed through the
    canonical store — no per-chart "fallback" code path.
  - The HLL sparkline preserves its existing decision not to call
    `registerChart`. That is the canonical opt-out; no separate `synced:
    false` flag is introduced.
- Review check: reviewers verify (a) only one shared zoom store exists in the
  embedded JS (grep `__chartSyncStore`), (b) only one `mountLegend` signature
  exists with all call sites updated (grep call-site count), (c) no parallel
  "non-synced" reset path remains (grep `setScale("x"` outside the store),
  and (d) the HLL sparkline carve-out is preserved (grep the comment +
  absence of `registerChart` in `renderHLLSparkline`).

## Project Structure

### Documentation (this feature)

```text
specs/017-chart-zoom-sync-stats/
├── plan.md                          # this file
├── spec.md                          # feature spec
├── research.md                      # Phase 0 output
├── quickstart.md                    # Phase 1 output
├── contracts/
│   └── chart-sync.md                # store API + legend stats contract
├── checklists/
│   └── requirements.md              # spec quality checklist
└── tasks.md                         # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
render/
├── assets.go                        # already concatenates app-js/* in lexical order
├── assets/
│   └── app-js/
│       ├── 00.js                    # extend: registerChart signature, mountResetZoomButton broadcast, mountLegend signature
│       ├── 01.js                    # update: pass data + un-stacked raw to mountLegend, register with data
│       ├── 02.js                    # update: pass data to registerChart + mountLegend (mysqladmin chart)
│       ├── 03.js                    # update: vmstat tab rebuild path, preserve HLL sparkline carve-out
│       ├── 04.js                    # unchanged
│       └── 05.js                    # NEW: shared zoom store + computeWindowedStats helper + legend stats renderer
└── assets_test.go                   # extend: grep markers for store + windowed-stats + signature change

tests/coverage/
└── file_size_test.go                # already enforces XV; re-runs in CI/pre-push
```

**Structure Decision**: extend the existing `render/assets/app-js/` part scheme.
Add one new ordered part (`05.js`) for the shared store + stats helpers; modify
the existing parts in-place to use the new APIs. No new Go packages, no new
directories.

## Complexity Tracking

> No constitution violations to justify. Adding `05.js` is mandated by gate 9
> (file-size cap), not a deviation.
