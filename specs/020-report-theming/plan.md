# Implementation Plan: Report Theming

**Branch**: `020-report-theming` | **Date**: 2026-05-07 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/020-report-theming/spec.md`

## Summary

Ship a user-toggleable theme system for the embedded HTML report with three
themes — `dark` (default), `light`, and `colorblind` (Okabe-Ito) — all driven
from a single CSS-design-token block keyed off `[data-theme]` on `<html>`.
The selection persists in `localStorage` under `my-gather:theme` and is
re-applied before first paint by an inline `<head>` script. The chart series
palette becomes `--series-1`…`--series-N` CSS variables read at decoration
time via the existing `cssVar()` helper. The legacy
`@media (prefers-color-scheme: light)` block and the hardcoded
`SERIES_COLORS` JS array are deleted in the same change (Principle XIII).

## Technical Context

**Language/Version**: Go 1.26.2 (rendering pipeline + tests), HTML, CSS,
JavaScript (vanilla, no framework — embedded inside the binary).
**Primary Dependencies**: Go standard library only. No new third-party
dependencies. Existing `uPlot` chart library (already bundled) consumes the
new tokens through its existing `axes`/`series` callback surface; no API
change to that library.
**Storage**: Browser `localStorage` (`my-gather:theme` key). No server-side
persistence.
**Testing**: `go test` over the embedded asset strings and over the rendered
template output (existing pattern in `render/assets_test.go`,
`render/render_test.go`, and `render/golden_helpers_test.go`).
**Target Platform**: The single-file HTML report; runs in any modern browser
without a network (Principle V).
**Project Type**: Go CLI with embedded HTML/CSS/JS report assets.
**Performance Goals**: No measurable regression in report generation or
in-browser first paint. Theme application before first paint must add at
most a single small inline `<script>` block (a few hundred bytes).
**Constraints**: Single canonical path for every theme-sensitive value
(Principle XIII). No new network I/O (Principle IX). No source file may
exceed 1000 lines (Principle XV). Output remains byte-deterministic
(Principle IV).
**Scale/Scope**: One feature on one report surface. Touches the embedded
CSS asset parts under `render/assets/app-css/`, the embedded JS asset
parts under `render/assets/app-js/`, the top-level template
`render/templates/report.html.tmpl`, and Go tests under `render/`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Static Binary | PASS | No CGO, no new build step. Pure embedded asset edits. |
| II. Read-Only Inputs | PASS | No new write path. Theme preference lives in browser localStorage, not in the pt-stalk input tree. |
| III. Graceful Degradation | PASS | Unknown stored theme → dark default. localStorage unavailable → toggle still flips current view. JS disabled → dark renders. All observable. |
| IV. Deterministic Output | PASS | Server-rendered HTML carries no theme-dependent content; the inline pre-paint script and toggle markup are static. Goldens regenerate once and stay byte-identical. |
| V. Self-Contained HTML | PASS | All theme tokens, JS, CSS continue to inline. No fonts or palette fetched at view time. |
| VI. Library-First | PASS | Pure render-package and asset change. No new exported Go API. |
| VII. Typed Errors | PASS | No new Go error path. |
| VIII. Fixtures and Goldens | PASS | Golden tests are regenerated explicitly via the existing reviewed `-update` flow as part of this feature; no silent regen. |
| IX. Zero Network | PASS | No network I/O added. Theme picker is local. |
| X. Minimal Dependencies | PASS | No new Go module dependency. Toggle uses a native `<select>` element; no UI framework added. No JS test runner added — coverage is Go-side over embedded asset strings (per FR-012). |
| XI. Human Pressure Optimization | PASS | Toggle sits in the report header, unobtrusive, alongside existing controls. Doesn't compete with primary narrative. |
| XII. Pinned Go Version | PASS | No Go version change. |
| XIII. Canonical Code Path | PASS | The `prefers-color-scheme` block and the `SERIES_COLORS` JS array are the prior canonical paths for "light variant" and "series colors". Both are **deleted** in this change. The new design-token block and `cssVar('--series-N')` call are the single canonical replacements. No fallback, shim, or feature flag is left behind. |
| XIV. English-Only | PASS | All artifacts in English. |
| XV. Bounded Source Size | PASS | New tokens land in a new asset part `render/assets/app-css/04.css` to keep `00.css` under 1000 lines. New theme JS lands in a new asset part `render/assets/app-js/05.js`. The toggle markup is a small addition to `report.html.tmpl`. The `tests/coverage/file_size_test.go` gate continues to pass. |

**Canonical Path Audit (Principle XIII)**:
- **Canonical owner/path for touched behaviour**:
  - Design tokens: a new `:root` + `[data-theme="..."]` token block lives
    in `render/assets/app-css/04.css`. This is the **single** place that
    defines theme-sensitive color values.
  - Theme application: a new module in `render/assets/app-js/05.js`
    reads/writes `localStorage` and toggles `data-theme` on `<html>`.
  - Pre-paint application: a small inline `<script>` block in the
    `<head>` of `report.html.tmpl` (between `<meta>` and `<style>`)
    sets `data-theme` from `localStorage` before CSS is parsed, so the
    correct theme is applied at first paint.
  - Toggle markup: a `<select>` inside `header.app-header` adjacent to
    the existing `#feedback-open` button.
  - Chart palette: `app-js/00.js` reads `--series-1..N` via the
    existing `cssVar()` helper.
- **Replaced or retired paths**:
  - The `@media (prefers-color-scheme: light) { :root { ... } }` block
    in `render/assets/app-css/00.css` (lines 48–65) is **deleted** in
    this same change. No replacement at the OS-preference layer; the
    new explicit selection (defaulting to dark) replaces it.
  - The hardcoded `SERIES_COLORS` JS array in
    `render/assets/app-js/00.js` (lines 267–272) is **deleted** in this
    same change; `decorateSeries` now resolves
    `cssVar('--series-' + ((idx % N) + 1))` instead.
- **External degradation paths**:
  - Stored theme name is unknown → dark applied (visible in DOM).
  - localStorage unavailable → toggle works for current view; choice
    not persisted (visible on next reload).
  - JS disabled → dark renders, toggle inert (visible — the report
    already requires JS for charts).
  All three are observable in DOM/UI and covered by Go tests over the
  embedded JS / template strings (the JS source is asserted to call
  `localStorage.getItem`/`setItem` and to fall back on unknown values;
  the template is asserted to default the `<select>` to "Dark").
- **Review check**:
  1. `git grep '@media (prefers-color-scheme'` returns nothing in
     `render/assets/`.
  2. `git grep 'SERIES_COLORS'` returns nothing in `render/assets/`.
  3. `git grep -n '#[0-9a-fA-F]\{6\}' render/assets/app-css/` returns
     hits **only** inside the new design-token block in `04.css`.
  4. `git grep '\[data-theme' render/assets/app-css/` returns hits
     **only** inside the new design-token block in `04.css`. No other
     CSS rule keys off the theme attribute.
  5. The new `render/theming_test.go` Go test asserts the embedded
     strings carry the three theme blocks, the Okabe-Ito values
     verbatim, the inline pre-paint script, and the absence of
     `SERIES_COLORS`.

## Project Structure

### Documentation (this feature)

```text
specs/020-report-theming/
├── plan.md
├── spec.md
├── research.md
├── quickstart.md
├── contracts/
│   └── theming.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
render/
├── assets/
│   ├── app-css/
│   │   ├── 00.css   # legacy `prefers-color-scheme` block REMOVED here
│   │   ├── 01.css
│   │   ├── 02.css
│   │   ├── 03.css
│   │   └── 04.css   # NEW: design-token block (one `:root` + three
│   │                #      `[data-theme="..."]` blocks) + toggle styling
│   └── app-js/
│       ├── 00.js    # SERIES_COLORS array REMOVED; decorateSeries reads
│       │            #   --series-N via cssVar()
│       ├── 01.js
│       ├── 02.js
│       ├── 03.js
│       ├── 04.js
│       └── 05.js    # NEW: theme module — reads localStorage, exposes
│                    #      apply(name), wires the <select>, dispatches
│                    #      a 'mygather:theme' event so charts can re-pick
├── templates/
│   └── report.html.tmpl   # NEW <head>-inline pre-paint script + NEW
│                          # <select id="theme-picker"> in app-header
└── theming_test.go        # NEW Go test (FR-012) asserting embedded
                           # asset strings carry the right tokens, the
                           # right Okabe-Ito values, the pre-paint script,
                           # and the cssVar()-driven series palette.
```

**Structure Decision**: One canonical token block in CSS (`04.css`), one
canonical theme JS module (`05.js`), one inline pre-paint script
(`report.html.tmpl`), one new `<select>` (`report.html.tmpl`). No
parallel paths. The 1000-line cap is honoured by adding new asset
parts rather than growing existing ones; the embed-and-concat
mechanism in `render/assets.go` (`mustConcatEmbeddedAssetParts`)
already orders by filename so the new parts land deterministically
at the end of their concatenated bundle.

## Complexity Tracking

No constitution exceptions are required.
