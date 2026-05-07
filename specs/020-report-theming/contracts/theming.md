# Contract: Report Theming

This document fixes the user-facing and code-facing contract surface for
the theming feature so reviewers can verify it without reading the
implementation.

## DOM contract

### Document root attribute

The active theme MUST be expressed as a single attribute on `<html>`:

```html
<html lang="en" data-theme="dark">    <!-- or "light" or "colorblind" -->
```

- The attribute is set by the inline pre-paint `<head>` script before
  CSS parsing reaches `<body>`. The server-rendered HTML does NOT carry
  `data-theme` in its `<html>` opening tag — that preserves byte-level
  determinism of the server output (Principle IV). The attribute is
  applied by the inline script reading `localStorage`.
- Consumer code MAY read the active theme as
  `document.documentElement.dataset.theme`.

### Toggle markup

The toggle MUST be rendered inside `header.app-header`:

```html
<label class="theme-label" for="theme-picker">Theme</label>
<select id="theme-picker" class="theme-picker">
  <option value="dark">Dark</option>
  <option value="light">Light</option>
  <option value="colorblind">Color-blind safe</option>
</select>
```

- `id` is a stable identifier other code MAY rely on.
- `class` is the CSS hook for unobtrusive header styling.
- The `<select>` is positioned to the left of `#feedback-open` so the
  primary action (feedback) keeps the rightmost visual slot. The
  picker `<select>` therefore appears earlier in DOM source order than
  `#feedback-open`; tests in `render/theming_test.go` enforce this
  ordering.

## Storage contract

- **Key**: `my-gather:theme` (string literal).
- **Value**: one of `"dark" | "light" | "colorblind"` (string).
- **Read time**: in the inline pre-paint script (`<head>`) on every
  page load.
- **Write time**: on the `<select>`'s `change` event.
- **Validation**: any value not in the closed set is treated as if the
  key were absent and the report renders `dark`.

## CSS-token contract

The design-token block lives in `render/assets/app-css/04.css` and
defines, per theme, every value listed below. The order in which themes
appear in the file is fixed: `:root` (default = dark), then
`:root[data-theme="dark"]`, then `:root[data-theme="light"]`, then
`:root[data-theme="colorblind"]`.

### Required tokens (all themes)

Surface and chrome:

- `--bg`, `--bg-elev`, `--bg-elev-2`
- `--border`, `--border-strong`
- `--fg`, `--fg-muted`, `--fg-dim`
- `--accent`, `--accent-soft`
- `--ok`, `--warn`, `--err`
- `--shadow`

Tooltip:

- `--tooltip-bg`, `--tooltip-fg`

Charts (axis / grid / ticks):

- `--axis-stroke`, `--grid-stroke`, `--tick-stroke`

Chart series palette (sixteen tokens):

- `--series-1` through `--series-16`

### Surface palette (theme = `colorblind`)

The Okabe-Ito 8-color set is defined in the literature against a
light/white surface; the 8th color is `#000000` (black), which is
invisible on a dark page. The `colorblind` theme therefore uses a
**light surface palette** (white / off-white background, near-black
text) so all eight Okabe-Ito series colors render with usable contrast.
This is the canonical accessibility configuration of the palette, and
it matches Okabe-Ito's published exemplars verbatim.

### Okabe-Ito values (theme = `colorblind`)

The `colorblind` block MUST set `--series-1` through `--series-8` to
the Okabe-Ito 8 colors in conventional order, and cycle them for
positions 9 through 16:

| Token | Color | Name |
|-------|-------|------|
| `--series-1` | `#E69F00` | orange |
| `--series-2` | `#56B4E9` | sky blue |
| `--series-3` | `#009E73` | bluish green |
| `--series-4` | `#F0E442` | yellow |
| `--series-5` | `#0072B2` | blue |
| `--series-6` | `#D55E00` | vermillion |
| `--series-7` | `#CC79A7` | reddish purple |
| `--series-8` | `#000000` | black |
| `--series-9`  | `#E69F00` | orange (cycle) |
| `--series-10` | `#56B4E9` | sky blue (cycle) |
| `--series-11` | `#009E73` | bluish green (cycle) |
| `--series-12` | `#F0E442` | yellow (cycle) |
| `--series-13` | `#0072B2` | blue (cycle) |
| `--series-14` | `#D55E00` | vermillion (cycle) |
| `--series-15` | `#CC79A7` | reddish purple (cycle) |
| `--series-16` | `#000000` | black (cycle) |

## JS contract

### Module surface (`render/assets/app-js/06.js`)

Inside the existing IIFE wrapper used by the report's app JS, the new
module:

- Defines `KNOWN = ["dark", "light", "colorblind"]`.
- Defines `STORAGE_KEY = "my-gather:theme"`.
- Exposes a single function `applyTheme(name)` that:
  - Validates `name` is in `KNOWN` (otherwise applies `"dark"`).
  - Sets `document.documentElement.dataset.theme = applied`.
  - Writes the applied value to `localStorage[STORAGE_KEY]` if storage
    is available; silently no-ops if not.
  - Dispatches a `mygather:theme` `CustomEvent` with `detail = {theme}`
    on `document` so other modules can re-pick token-derived state.
- Wires the `change` event on `#theme-picker` to `applyTheme`.
- On script init, syncs the `<select>`'s value to whatever the inline
  pre-paint script applied (or `"dark"` if absent).

### Inline pre-paint script (`<head>` of `report.html.tmpl`)

A literal `<script>...</script>` block, no `defer`, no `async`, that:

- Tries to read `localStorage["my-gather:theme"]`.
- Validates against the closed set `{dark, light, colorblind}`.
- Falls back to `"dark"` on unknown / missing / storage-error.
- Sets `document.documentElement.dataset.theme = applied`.

The script is intentionally `try`/`catch`-wrapped around the storage
read so a private-browsing storage error never throws into the
critical first-paint path.

### Series palette read (modified `render/assets/app-js/00.js`)

The existing `decorateSeries(label, colorIndex)` function MUST resolve
the stroke as:

```js
var idx = (colorIndex % 16) + 1;
var stroke = cssVar("--series-" + idx, "#60a5fa");
```

The fallback `"#60a5fa"` matches the previous default series-1 color
and is reached only if the CSS custom property somehow does not
resolve (e.g., the document is mid-rebuild). It does not represent a
parallel theme path. **This `#60a5fa` literal lives in
`render/assets/app-js/00.js`, not in any CSS asset, and is the only
hex literal in `app-js/` that is allowed by the canonical-path audit.
The audit grep is scoped to `app-css/` precisely so this fallback does
not register as a violation.**

`decorateSeries` also stores the resolved `colorIndex` on the returned
object as `__themeIdx` so the runtime theme-change handler can later
re-resolve the stroke for the same series without keeping a side map.

### Live theme switch — chart re-paint contract

uPlot caches the resolved `stroke` value on each series at chart
construction. A bare `resizeAllCharts()` call re-paints with the cached
stroke and does NOT pick up the new `--series-N` CSS variables. To
satisfy FR-010 the theme module's response to `mygather:theme` MUST,
for every registered chart `u`:

1. Walk `u.series` and, for each `s` with `s.__themeIdx != null`, set
   `s.stroke = cssVar("--series-" + ((s.__themeIdx % 16) + 1),
   "#60a5fa")`.
2. Re-resolve axis / grid / tick strokes from `--axis-stroke`,
   `--grid-stroke`, `--tick-stroke` and assign into `u.axes[i]`
   accordingly.
3. Call `u.redraw(false)` to re-paint with the new series and axis
   strokes (the `false` argument keeps the existing data path; we
   only invalidated colors, not data). For uPlot versions where
   axis stroke needs `u.axes[i].stroke = ...` followed by
   `u.redraw()` to take effect, the same call covers it.

This is the canonical re-pick path. No other code path may attempt to
react to theme changes by mutating chart colors.

## Server output contract

The Go-side render output stays byte-deterministic across runs
(Principle IV). Specifically:

- The `<html>` opening tag is `<html lang="en">` — no `data-theme`
  baked in. The attribute is applied client-side.
- The inline pre-paint script and the `<select>` markup are static
  strings inside the template; their content does not depend on input
  data or render time.
- Goldens are regenerated once with `go test -update ./render/...` as
  an explicit reviewed step in the tasks file (Principle VIII).
