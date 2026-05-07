---
description: "Task list for feature 020-report-theming"
---

# Tasks: Report Theming

**Input**: Design documents from `/specs/020-report-theming/`
**Prerequisites**: plan.md, spec.md, research.md, contracts/theming.md

## Canonical Path Metadata *(Principle XIII)*

- **Canonical owner/path**:
  - CSS design tokens: `render/assets/app-css/04.css` (NEW).
  - Theme module: `render/assets/app-js/06.js` (NEW).
  - Toggle markup + pre-paint script: `render/templates/report.html.tmpl`.
  - Chart palette read: `render/assets/app-js/00.js` (modified
    `decorateSeries`).
- **Old path treatment** (deleted in this feature, by task IDs):
  - `@media (prefers-color-scheme: light)` block in
    `render/assets/app-css/00.css` → deleted by **T010**.
  - `SERIES_COLORS` JS array in `render/assets/app-js/00.js` →
    deleted by **T012**.
- **External degradation evidence**:
  - Stored unknown theme name → dark default. Covered by **T015**.
  - localStorage unavailable / disabled → toggle still flips current
    view; persistence silently absent. Covered by **T016**.
  - JS disabled → dark renders, toggle inert. Covered by review check
    in **T021** (the inline pre-paint script is the only theme path
    the JS-off case can reach; the default `:root` CSS block already
    matches dark).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies).
- **[Story]**: User story this task serves (US1, US2, US3).

---

## Phase 1: Setup (shared infrastructure)

- [ ] **T001** Create new asset part stubs at exact paths:
  - `render/assets/app-css/04.css` (file present, single comment header).
  - `render/assets/app-js/06.js` (file present, IIFE shell only).
  These exist so the embed pattern in `render/assets.go`
  (`mustConcatEmbeddedAssetParts`) picks them up; the embed directive
  `//go:embed assets/app-js/*.js assets/app-css/*.css` already includes
  them by glob.

---

## Phase 2: Foundational (blocking prerequisites)

- [ ] **T002** Decide and document the final token list in
  `render/assets/app-css/04.css` as a leading comment block (mirrors
  the contract list in `specs/020-report-theming/contracts/theming.md`
  so a future reader does not have to cross-jump). No CSS rules yet —
  this is the schema definition that the rest of the phase fills in.

**Checkpoint**: token schema agreed in code; user-story phases can
populate values in parallel.

---

## Phase 3: User Story 1 — Choose a theme that survives reload (P1) — MVP

**Goal**: Three themes selectable from the report header. Selection
persists via `localStorage`. Default is `dark`. Pre-paint script
prevents the dark→saved flash.

**Independent Test**: see Quickstart, "How to verify by hand" steps
1–4.

### Implementation for User Story 1

- [ ] **T003** [US1] Populate `render/assets/app-css/04.css` with the
  three theme blocks (`:root` + dark, light, colorblind) and the
  `theme-picker` styling. Define every required token from
  `contracts/theming.md` for `dark` and `light`. Leave `colorblind`
  with placeholder Okabe-Ito values to be locked in by T007 (US2);
  the *structure* lands here so US1 can be tested without US2.
- [ ] **T004** [US1] In `render/assets/app-js/06.js`, write the theme
  module: `KNOWN`, `STORAGE_KEY = "my-gather:theme"`, `applyTheme()`
  with `try`/`catch` storage write, `<select>` change-handler wiring,
  and `mygather:theme` `CustomEvent` dispatch on `document`.
- [ ] **T005** [US1] In `render/templates/report.html.tmpl`:
  - Add the inline pre-paint `<script>` in `<head>` immediately after
    `<meta name="referrer">` and immediately before the existing
    `<link rel="icon">` line. Synchronous (no `defer`, no `async`),
    `try`/`catch` around the `localStorage` read, closed-set
    validation against `dark`, `light`, `colorblind`, fall back to
    `dark`. Sets `document.documentElement.dataset.theme`. Placing it
    here ensures the active theme attribute is set before the
    `<style>` block (line 13 today) is parsed, so the
    `:root[data-theme="..."]` rules in `04.css` apply on first paint.
  - Add the `<label>` + `<select id="theme-picker">` with the three
    options inside `header.app-header`, immediately to the left of
    the existing feedback button, so the feedback button keeps the
    rightmost slot.
- [ ] **T006** [US1] Regenerate render goldens for the changed
  template:
  - Run `go test ./render/... -update` after T003–T005 stabilise the
    HTML.
  - Manually inspect the diff. Confirm the only changes are: the new
    `<script>` block in `<head>`, the new `<label>` + `<select>` in
    the header, and any cascade-driven CSS bytes from the new asset
    part. Reject any other diff.

**Checkpoint**: a freshly built binary produces a report whose
`<select>` flips themes live, persists to `localStorage`, and survives
reload without flash.

---

## Phase 4: User Story 2 — Color-blind-safe palette (P2)

**Goal**: The `colorblind` theme uses the Okabe-Ito 8 colors verbatim
for `--series-1`…`--series-8`, cycled to fill `--series-9`…`--series-16`.

**Independent Test**: see Quickstart, "How to verify by hand" step 5.

### Implementation for User Story 2

- [ ] **T007** [US2] Lock in the `colorblind` block's `--series-1`
  through `--series-16` to the Okabe-Ito values from
  `contracts/theming.md` (orange, sky blue, bluish green, yellow, blue,
  vermillion, reddish purple, black; cycle).
- [ ] **T008** [US2] In the `colorblind` block, set the **surface
  tokens to a light palette** (white / off-white background, near-black
  text). Per `contracts/theming.md`, the Okabe-Ito set is published
  against a light surface and includes `#000000` as series 8 — that
  series would be invisible on a dark background. Reuse the existing
  `light` theme's surface values as the canonical starting point so
  there is exactly one well-tuned light surface palette in the codebase
  shared by `light` and `colorblind`. The series tokens (T007) are the
  only tokens that meaningfully diverge between `light` and
  `colorblind`. Visual review of the chosen palette happens in T021.

---

## Phase 5: User Story 3 — Single canonical token source (P3)

**Goal**: Every theme-sensitive color in the shipped report flows
through exactly one CSS custom property defined in `04.css`. No JS
array, no `prefers-color-scheme` override, no per-component theme
conditionals.

**Independent Test**: `git grep` checks listed in plan.md "Review
check"; the `theming_test.go` assertions land in T013.

### Implementation for User Story 3

- [ ] **T009** [US3] In `render/assets/app-js/00.js`, replace the
  hardcoded series-stroke selection in `decorateSeries(label,
  colorIndex)` with a `cssVar('--series-' + ((colorIndex % 16) + 1),
  "#60a5fa")` read. Also store `colorIndex` on the returned series
  descriptor as `__themeIdx: colorIndex` so the runtime theme-change
  handler (T011) can re-resolve the stroke for the same series after
  a theme switch. Keep the function signature and call sites unchanged.
- [ ] **T010** [US3] **Delete** the
  `@media (prefers-color-scheme: light) { :root { ... } }` block in
  `render/assets/app-css/00.css` (current lines 48–65). Update the
  surrounding comment header so it no longer mentions a "dark-first
  palette with a `prefers-color-scheme: light` override"; replace with
  a one-line pointer to `04.css` as the canonical token source.
- [ ] **T011** [US3] In `render/assets/app-js/00.js`, add a runtime
  theme-change handler. uPlot caches each series' resolved `stroke` at
  construction time, so a bare resize re-paints with the cached colors
  and would NOT pick up new `--series-N` values. The handler MUST
  walk every registered chart and, for each:
  - For every series `s` with `s.__themeIdx != null` set
    `s.stroke = cssVar('--series-' + ((s.__themeIdx % 16) + 1),
    "#60a5fa")`.
  - Re-resolve `--axis-stroke`, `--grid-stroke`, `--tick-stroke` and
    assign them into the corresponding fields of `u.axes[i]` (both
    x-axis index 0 and y-axis index 1).
  - Call `u.redraw(false)` so the chart re-paints with the new colors
    without re-walking data.
  Wire this once via `document.addEventListener('mygather:theme', ...)`.
  This is the canonical re-pick path (FR-010); no other module may
  mutate chart colors in response to theme changes.
- [ ] **T012** [US3] **Delete** the `SERIES_COLORS` array literal at
  `render/assets/app-js/00.js:267-272` and the now-stale comment
  header above it. Confirm no other reference to `SERIES_COLORS`
  exists in the codebase (`git grep SERIES_COLORS`).

---

## Phase 6: Tests (FR-012)

- [ ] **T013** [P] [US1, US2, US3] Add `render/theming_test.go`. New
  Go test file. Required cases:
  - `TestThemingTokensPresent`: assert the embedded CSS string
    contains `:root[data-theme="dark"]`, `:root[data-theme="light"]`,
    `:root[data-theme="colorblind"]` and that each block defines all
    documented tokens (loop over the contract token list).
  - `TestThemingOkabeItoPalette`: assert the `colorblind` block's
    `--series-1`…`--series-16` values match the Okabe-Ito sequence
    verbatim (with cycle).
  - `TestThemingSeriesColorsArrayRemoved`: assert the embedded JS
    string does NOT contain `SERIES_COLORS` (regression guard against
    re-introducing the parallel path).
  - `TestThemingSeriesViaCssVar`: assert the embedded JS string
    contains `cssVar("--series-"` (or single-quote variant).
  - `TestThemingPrePaintScriptInTemplate`: render the template and
    assert the rendered HTML contains a `<script>` block in `<head>`
    that reads `localStorage` and sets
    `document.documentElement.dataset.theme`, AND that `<style>`
    appears AFTER it.
  - `TestThemingPickerInHeader`: assert the rendered HTML contains
    `id="theme-picker"` inside `header.app-header`.
  - `TestThemingPrefersColorSchemeRemoved`: assert the embedded CSS
    string no longer contains `prefers-color-scheme`.

- [ ] **T014** [P] [US1] Add `TestThemingDataThemeApplied` in
  `render/theming_test.go`: assert the inline pre-paint script source
  contains both `localStorage.getItem("my-gather:theme")` and an
  assignment of the form `documentElement.dataset.theme` (covers FR-005,
  FR-006).

- [ ] **T015** [P] [US1] Add `TestThemingUnknownStoredValueFallsBack`:
  assert the inline pre-paint script source contains the closed-set
  validation against `dark`, `light`, `colorblind`. Covers FR-007.

- [ ] **T016** [P] [US1] Add `TestThemingStorageWriteIsTryCatch`:
  assert the JS module source for the storage write is wrapped in
  `try`/`catch` (FR-005, edge case "localStorage unavailable").

---

## Phase 7: Polish & cross-cutting

- [ ] **T017** Run `go vet ./...` and `go test ./...` from the repo
  root. Expect green. The file-size gate in
  `tests/coverage/file_size_test.go` checks `04.css` and `06.js` are
  under 1000 lines each (they will be, comfortably).
- [ ] **T018** Run the pre-push constitution guard:
  `scripts/hooks/pre-push-constitution-guard.sh`. Expect green
  (no CGO, no new network call, no input-tree write, no non-English
  text).
- [ ] **T019** Update `AGENTS.md` and `CLAUDE.md` so the "Active
  feature" line and the `Prior features` list both name
  `020-report-theming` and reference its `plan.md` / `spec.md` /
  `tasks.md` / `quickstart.md` / `contracts/theming.md`. Move
  `015-compliance-closure` under `Prior features`.
- [ ] **T020** Confirm `.specify/feature.json` reads
  `{"feature_directory": "specs/020-report-theming"}`.
- [ ] **T021** **Canonical-path audit** (Principle XIII review). Verify
  on the diff:
  - `git grep -n 'prefers-color-scheme' render/assets/` returns zero
    results.
  - `git grep -n 'SERIES_COLORS' .` returns zero results.
  - `git grep -nE '#[0-9a-fA-F]{6}' render/assets/app-css/` returns
    hits **only** inside `04.css` token definitions. The grep is
    scoped to `app-css/` deliberately so it does not catch the
    documented degradation fallback `#60a5fa` literal in
    `render/assets/app-js/00.js` (the only allowed hex literal in
    `app-js/`, called out in `contracts/theming.md`).
  - `git grep -n '\[data-theme' render/assets/app-css/` returns hits
    **only** inside `04.css`.
  - No new `if (theme === ...)` branch exists in any JS asset part.
- [ ] **T022** Run the quickstart by-hand validation
  (`specs/020-report-theming/quickstart.md`): build, generate, open in
  a browser, walk through steps 1–6.

---

## Dependencies

- T001 → all others.
- T002 (token schema) → T003, T007, T008.
- T003 (CSS structure) → T006 (golden regen) and T013 (token tests).
- T004 (JS module) → T005 (template wires `<select>` to a module that
  exists), T013–T016.
- T005 (template changes) → T006, T013, T014.
- T009 (cssVar series read) → T013 (palette tests can assert against
  the live JS).
- T010 (delete `prefers-color-scheme`) → T013, T021.
- T012 (delete `SERIES_COLORS`) → T013, T021.
- T017–T022 are the close-out gate and run last.

## Parallel opportunities

- T003 (CSS) and T004 (JS module) touch different files: parallel.
- T009 / T010 / T011 / T012 each touch the same file
  (`render/assets/app-js/00.js` for T009/T011/T012, and `00.css` for
  T010): keep T010 in its own commit but T009 / T011 / T012 may share
  a single edit if convenient.
- All T013–T016 test cases live in the new `theming_test.go` and run
  in the same `go test` invocation; mark `[P]` because they are
  independent assertions.

## Notes

- The single `[P]` rule (different files, no dependencies) applies
  here exactly as in the template.
- Per Principle VIII, `T006` (golden regen) is the only task that
  rewrites committed goldens; nothing else triggers `-update`.
- Per Principle XV, the new asset parts are sized to stay well under
  1000 lines (`04.css` ~80 lines for tokens + ~30 for picker styling;
  `06.js` ~60 lines).
