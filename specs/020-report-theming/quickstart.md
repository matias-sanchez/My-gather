# Quickstart: Report Theming

## What this feature ships

A user-toggleable theme picker on every generated My-gather report,
backed by a single CSS-design-token system.

- Default theme: **dark**.
- Other themes: **light**, **colorblind** (Okabe-Ito).
- Persisted in browser `localStorage` under `my-gather:theme`.
- Toggle: a `<select id="theme-picker">` in the report header, next to
  the **Report feedback** button.

## Local commands

From the repo root, in this worktree:

```sh
# Build the binary (no flags change for this feature)
go build ./cmd/my-gather

# Run the rendering tests, including the new theming tests
go test ./render/...

# Whole repo, including the file-size and godoc gates
go test ./...

# Re-run the report on a sample fixture
./my-gather -in testdata/<fixture-dir> -out /tmp/report.html
open /tmp/report.html        # macOS
```

## How to verify by hand

1. Build and generate a report against any fixture under `testdata/`.
2. Open the report in a browser. Confirm:
   - The page background is dark.
   - The theme picker in the header reads "Dark".
3. Pick "Light". Confirm:
   - The page background, panels, text, and chart strokes flip to the
     light palette without a page reload.
   - `document.documentElement.dataset.theme === "light"` in DevTools.
   - `localStorage.getItem("my-gather:theme") === "light"`.
4. Reload the page. Confirm:
   - The report renders **light** from first paint, with no flash of
     dark first.
5. Pick "Color-blind safe". Confirm:
   - Chart strokes use the Okabe-Ito 8 colors. In DevTools,
     `getComputedStyle(document.documentElement).getPropertyValue('--series-1').trim()`
     returns `#E69F00` (orange).
6. In DevTools, run `localStorage.setItem("my-gather:theme", "garbage")`
   and reload. Confirm the report renders **dark** (unknown values fall
   back).

## How to verify with tests

```sh
go test ./render/ -run TestTheming
```

The `TestTheming*` tests assert:

- The embedded CSS asset string contains exactly three theme blocks
  (`:root[data-theme="dark"]`, `:root[data-theme="light"]`,
  `:root[data-theme="colorblind"]`).
- Each block defines all the documented tokens (`--bg`, `--fg`,
  `--bg-elev`, `--bg-elev-2`, `--border`, `--border-strong`,
  `--fg-muted`, `--fg-dim`, `--accent`, `--accent-soft`, `--ok`,
  `--warn`, `--err`, `--shadow`, `--tooltip-bg`, `--tooltip-fg`,
  `--axis-stroke`, `--grid-stroke`, `--tick-stroke`, `--series-1`
  through `--series-16`).
- The `colorblind` block's `--series-1`…`--series-8` values are exactly
  the Okabe-Ito colors in conventional order (FR-011).
- The embedded JS asset string no longer contains the
  `SERIES_COLORS = [` array literal.
- The embedded JS reads the series palette via
  `cssVar('--series-' + …)`.
- The embedded JS reads / writes `localStorage` under the
  `my-gather:theme` key and validates against the closed set.
- The rendered template HTML carries the inline pre-paint
  `<script>` block in `<head>` ahead of `<style>`.
- The rendered template HTML carries the
  `<select id="theme-picker">` next to the feedback button.
- The rendered template HTML's `<html>` opening tag is unchanged in
  the server output (i.e., the `data-theme` attribute is set by the
  client, not baked into the server output — this preserves
  Principle IV byte-determinism).

## Where the implementation lives

- `render/assets/app-css/04.css` — token block for all three themes,
  toggle styling.
- `render/assets/app-js/05.js` — theme-picker module: reads/writes
  localStorage, dispatches `mygather:theme`, wires the `<select>`.
- `render/assets/app-js/00.js` — `decorateSeries()` reads
  `--series-N` via `cssVar()`. Listens for `mygather:theme` and
  re-decorates so live-switching updates open charts.
- `render/templates/report.html.tmpl` — inline `<head>` pre-paint
  script and `<select>` in `header.app-header`.
- `render/theming_test.go` — Go tests asserting all of the above
  against the embedded asset strings and the rendered template.

## Constitution gates that this feature trips

- **Principle IV (determinism)**: golden snapshots will need a single
  reviewed regen because the template now ships the toggle markup and
  the inline pre-paint script. The tasks file calls this out
  explicitly as `T0xx — regen goldens (reviewed)`.
- **Principle XIII (canonical path)**: the `prefers-color-scheme` block
  and the `SERIES_COLORS` JS array are deleted in the same change.
  Reviewers verify with `git grep`.
- **Principle XV (1000-line cap)**: new tokens land in `04.css` (new
  file) and new theme JS in `05.js` (new file). No existing asset part
  grows past 1000 lines.
