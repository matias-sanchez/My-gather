# Feature Specification: Report Theming

**Feature Branch**: `020-report-theming`
**Created**: 2026-05-07
**Status**: Draft
**Input**: User description: "Add user-toggleable theming to the report. Themes: Light, Dark (default), Okabe-Ito color-blind-safe palette. Toggle in unobtrusive corner of report header. Selection persists in localStorage. Themes apply to background, text, panels, tooltips, chart palettes (line/axis/grid). All theme-sensitive values flow from a single source (CSS variables / design tokens). Add tests asserting data-theme attribute updates and chart palette tokens resolve to expected colors."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Choose a theme that survives reload (Priority: P1)

A support engineer opens a generated pt-stalk diagnostic report. They prefer
the dark visual style (the report's default). On a different incident later
that day, glare on a bright office monitor pushes them to switch to the light
palette. They click the theme picker in the top-right corner of the report
header, choose "Light", and continue triage. They close the browser tab. When
they re-open the same report (or a different report) later, the page renders
in the light palette without flashing dark first.

**Why this priority**: This is the whole feature. Without it the report only
ships its current palette, and the new color-blind palette is unreachable.

**Independent Test**: Open a generated report, confirm dark by default, click
the picker, switch to "Light", reload the page, confirm light persists, switch
to "Color-blind safe", reload, confirm the Okabe-Ito palette persists.

**Acceptance Scenarios**:

1. **Given** a freshly generated report opened in a browser with no prior
   theme selection, **When** the page loads, **Then** the report renders in
   the dark palette and the toggle reflects "Dark".
2. **Given** the report is open in dark, **When** the user picks "Light" from
   the theme toggle, **Then** the page background, text, panels, tooltips,
   and chart strokes immediately switch to the light palette without a page
   reload.
3. **Given** the user has previously chosen "Light" in this browser, **When**
   they close the tab and re-open any report, **Then** the report renders in
   light from first paint (no dark flash).
4. **Given** the user picks "Color-blind safe", **When** charts re-render,
   **Then** the chart line strokes, axis text, and gridlines use the
   Okabe-Ito palette tokens, distinct from both the dark and light palettes.

---

### User Story 2 - Color-blind-safe palette for accessibility (Priority: P2)

A support engineer with deuteranopia or protanopia opens a report. The
default dark palette uses adjacent reds/greens that they cannot reliably
distinguish in dense legends. They switch to "Color-blind safe" and can now
tell adjacent series apart in a 12-line chart.

**Why this priority**: Accessibility is the named reason the third theme
exists. The Okabe-Ito palette is the canonical color-blind-safe choice.

**Independent Test**: Switch to "Color-blind safe" on a report with a chart
that has at least 8 series, confirm strokes draw from the Okabe-Ito 8-color
set in order, confirm axis and gridline contrast remain readable on the
chosen background.

**Acceptance Scenarios**:

1. **Given** a chart with 8+ series in dark palette, **When** the user picks
   "Color-blind safe", **Then** every series stroke is one of the 8 Okabe-Ito
   colors and is reused in deterministic order beyond the 8th.
2. **Given** the color-blind palette is active, **When** any chart re-renders
   for any reason (resize, view toggle), **Then** strokes still come from the
   Okabe-Ito set, not from the dark or light series tokens.

---

### User Story 3 - Single canonical token source for theme values (Priority: P3)

A future contributor needs to tweak the panel background for the dark
palette. They open exactly one place — the design-token block in the
stylesheet — change the dark `--bg-elev` value, and every panel,
tooltip, and chart background that depends on it updates without further
edits anywhere else in the codebase.

**Why this priority**: This is the maintenance promise. It is what stops the
codebase from accumulating per-component theme conditionals over time.

**Independent Test**: Grep the codebase for hardcoded color literals in
shipped CSS and JS for the report. Every theme-sensitive color resolves
through a CSS custom property defined once per theme, not duplicated.

**Acceptance Scenarios**:

1. **Given** a code review on a future change, **When** a reviewer searches
   the report's CSS and JS for hex literals or `rgb(...)` color values used
   as theme colors, **Then** they find them only inside the design-token
   block — the rest of the code consumes them via `var(--token)` (CSS) or
   the existing `cssVar()` helper (JS).
2. **Given** the chart series palette, **When** a contributor needs to
   recolor it for one theme, **Then** they edit the `--series-N` tokens for
   that theme in one CSS block, not a JS array.

---

### Edge Cases

- **Stored theme name is unknown** (user manually edited localStorage to
  e.g. `"high-contrast-pink"`): the report MUST fall back to the dark
  default rather than rendering with no `data-theme` and inheriting
  arbitrary platform colors. The fallback MUST be a single observable
  decision in one place — it is not a hidden parallel theme path.
- **localStorage is unavailable** (private browsing in some browsers,
  storage quota error): the toggle MUST still work for the current page
  view; the choice simply does not persist across reloads. The user is
  not shown an error popup; the absence of persistence is the
  user-observable degradation.
- **JavaScript disabled**: the report renders in the dark default
  (the only theme expressible without JS), and the toggle is inert.
  This is acceptable — the report already requires JS for charts.
- **First paint before JS runs**: the saved theme MUST be applied
  before the first paint to avoid a flash of the wrong theme.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The report MUST ship exactly three named themes: `dark`,
  `light`, and `colorblind` (Okabe-Ito).
- **FR-002**: The report MUST default to `dark` on first load (no prior
  selection in localStorage).
- **FR-003**: The active theme MUST be expressed as a single
  `data-theme="<name>"` attribute on `<html>` (the document root). All
  theme-sensitive CSS MUST select on this attribute, not on
  `prefers-color-scheme` and not on per-element class names.
- **FR-004**: A theme picker control MUST be rendered inside
  `header.app-header` next to the existing "Report feedback" button,
  visually unobtrusive (no loud color, no large surface) and
  keyboard-accessible.
- **FR-005**: Selecting a theme MUST update `data-theme` on `<html>` and
  persist the chosen name to `localStorage` under a single fixed key
  (`my-gather:theme`).
- **FR-006**: On every report load, the saved theme MUST be applied
  before first paint by an inline `<head>`-level script, so users with a
  saved non-default theme never see the dark default flash first.
- **FR-007**: If the value stored in localStorage is not one of the three
  known theme names, the report MUST treat it as if no value were stored
  and render `dark`. No error UI is shown.
- **FR-008**: All theme-sensitive values — page background, text colors,
  panel backgrounds, borders, tooltip backgrounds and text, chart axis
  stroke, chart gridline stroke, and the chart series palette colors —
  MUST be defined as CSS custom properties (`--token-name`) in a single
  design-token block, with one value per theme keyed off the
  `[data-theme="..."]` selector on `<html>`.
- **FR-009**: The chart series color palette MUST be expressed as
  `--series-1` through `--series-N` CSS custom properties (one set per
  theme), and the chart-rendering JavaScript MUST read those values via
  the existing `cssVar()` helper at series-decoration time. A hardcoded
  series-color array in JavaScript is forbidden.
- **FR-010**: When the user changes the theme at runtime, charts already
  on the page MUST re-pick their stroke / gridline / axis colors from the
  newly active token values without a page reload. Acceptable
  re-application paths: a re-render triggered by the theme change, or
  reading the tokens at every draw.
- **FR-011**: The Okabe-Ito palette MUST use the published Okabe-Ito 8
  colors in their conventional order so the choice is recognizable to
  accessibility-aware reviewers: orange `#E69F00`, sky blue `#56B4E9`,
  bluish green `#009E73`, yellow `#F0E442`, blue `#0072B2`, vermillion
  `#D55E00`, reddish purple `#CC79A7`, black `#000000`. Series beyond the
  8th MUST cycle the palette in the same order.
- **FR-012**: Test coverage MUST include (a) an assertion that a theme
  switch updates `document.documentElement.dataset.theme` and (b) an
  assertion that the chart palette tokens resolve to the expected color
  set for each theme. Given the project's existing testing surface
  (Go-only main module; no JS test runner is wired into the main
  module), these MAY be expressed as Go tests over the embedded CSS / JS
  asset strings: assert each theme block defines all required tokens,
  assert the JS reads the palette via `cssVar()`, and assert the
  Okabe-Ito values match the spec verbatim. This satisfies the spec's
  intent without adding a JS test runner dependency to the main module.

### Canonical Path Expectations

- **Canonical owner/path**: the report's design-token block lives in the
  embedded CSS asset under `render/assets/app-css/`; theme reading and
  application live in the embedded JS asset under `render/assets/app-js/`;
  the toggle markup lives in `render/templates/report.html.tmpl`. There is
  exactly one design-token block per theme and one theme-application
  module in JS.
- **Old path treatment**: the existing
  `@media (prefers-color-scheme: light)` override block in
  `render/assets/app-css/00.css` is **deleted in this same change**.
  Auto-following the OS preference is replaced by an explicit user
  selection (with `dark` as the default), which is the new canonical
  source of theme truth. The hardcoded `SERIES_COLORS` JS array in
  `render/assets/app-js/00.js` is **deleted in this same change** and
  replaced by reading `--series-N` CSS variables via the existing
  `cssVar()` helper.
- **External degradation**: localStorage unavailable → the toggle still
  works for the current view but the choice does not persist. JS
  disabled → dark renders, toggle is inert. Both are user-observable.
- **Review check**: reviewers verify that (1) the
  `prefers-color-scheme` media block is gone from the embedded CSS,
  (2) `SERIES_COLORS` is gone from the embedded JS, (3) every chart
  color flows through `cssVar('--series-N')` or sibling tokens, and
  (4) no per-component theme conditional was introduced (no `if
  theme === 'dark'` branches in JS, no `[data-theme]` selectors
  scattered across many CSS rules outside the token block — only the
  token block keys off the theme).

### Key Entities

- **Theme**: a named value in the closed set `{dark, light, colorblind}`,
  applied by setting `data-theme` on `<html>` and used by the CSS
  design-token block to pick the active token values.
- **Design token**: a single CSS custom property (`--bg`, `--fg`,
  `--bg-elev`, `--border`, `--accent`, `--series-1` … `--series-N`,
  axis/grid stroke tokens, tooltip tokens) whose value is defined once
  per theme.
- **Theme preference**: the persisted user choice, stored under the
  fixed localStorage key `my-gather:theme` as one of the three known
  theme name strings.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Three themes (`dark`, `light`, `colorblind`) are selectable
  from a single control in the report header, and each renders distinct
  background, text, panel, tooltip, and chart-stroke colors.
- **SC-002**: A theme picked by the user is reapplied automatically on
  100% of subsequent page loads in the same browser, with no flash of
  the previous theme on first paint.
- **SC-003**: Every theme-sensitive color in the shipped report (page,
  panel, tooltip, chart axis, chart gridline, chart series palette)
  resolves through exactly one canonical CSS custom property. A grep
  for hex / rgb literals used as theme colors outside the design-token
  block returns zero results.
- **SC-004**: With the color-blind theme active, a chart with 8 or more
  series uses the Okabe-Ito 8 colors verbatim and in their conventional
  order, cycled for any 9th-and-beyond series.
- **SC-005**: Test coverage proves both behaviours called out in the
  feature description: a theme switch updates the document
  `data-theme` attribute, and the chart palette tokens for each theme
  resolve to their expected color values.

## Assumptions

- The toggle UX is a small `<select>` (or equivalent native control)
  inline in the report header, labelled "Theme", with three options
  (`Dark`, `Light`, `Color-blind safe`). A native `<select>` keeps the
  control unobtrusive, keyboard-accessible, and avoids adding a
  bespoke dropdown component (Principle X — minimal dependencies).
- `prefers-color-scheme` is intentionally not consulted once a user
  has made an explicit choice. Until they make one, the report
  defaults to `dark` regardless of OS preference; this is a deliberate
  product choice (default Dark) and removes the only place
  `prefers-color-scheme` was previously consulted.
- The fixed localStorage key `my-gather:theme` is namespaced so it
  never collides with unrelated keys when a user opens reports for
  multiple projects in the same origin.
- The toggle and the theme system add only first-party CSS and JS
  inside the existing embedded asset folders; no new third-party
  dependency is introduced (Principle X).
- The report still renders fully on an air-gapped laptop (Principle V):
  no theme metadata, no font, no palette is fetched at view time.
