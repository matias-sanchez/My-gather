# Research: Report Theming

## Decisions

### D1. Theme attribute on `<html>`, not on `<body>`

**Decision**: Use `data-theme="<name>"` on `<html>` (the document root).

**Rationale**: The pre-paint script runs in `<head>` before `<body>`
exists, so it can only reach the document element. Putting the attribute
on `<html>` means a single CSS selector — `:root[data-theme="dark"]`,
`:root[data-theme="light"]`, `:root[data-theme="colorblind"]` — defines
all theme tokens. CSS variables on `:root` cascade to every element.
This avoids needing to wait for `DOMContentLoaded` or to swap the
attribute after first paint, which would cause a flash.

**Alternatives considered**:
- `<body data-theme="...">`: requires waiting for `<body>` to exist;
  forces either a deferred script (flash risk) or a polling pattern.
- A `class="theme-dark"` on `<html>`: works the same but reads worse;
  `data-*` attributes are the conventional carrier for state-driven
  CSS toggles in modern reports.

### D2. Pre-paint inline script in `<head>`, not deferred

**Decision**: A small inline `<script>` block lives in `<head>` between
`<meta name="referrer">` and `<style>`. It reads `localStorage`, validates
against the closed set `{dark, light, colorblind}`, and sets
`document.documentElement.dataset.theme`. It is intentionally synchronous
and tiny (no `defer`, no `async`).

**Rationale**: This is the only known way to avoid a flash of the wrong
theme. The script does not block on the network (no fetch), runs in well
under a millisecond on any modern browser, and only reads/writes a single
DOM property and one storage entry. Putting it before `<style>` means CSS
parsing already sees the active theme attribute when it computes
`:root[data-theme="..."]` rules.

**Alternatives considered**:
- Render `data-theme` server-side from a query parameter or cookie:
  My-gather is a static, single-file HTML report with no server. There
  is no request-time hook.
- A deferred script that swaps the attribute on `DOMContentLoaded`: the
  user sees the dark default for one paint frame, then the page shifts
  to their saved theme. Visible, undesirable.

### D3. Three themes in a closed set, no auto

**Decision**: The selectable themes are exactly `{dark, light, colorblind}`.
The current `prefers-color-scheme` auto-following behaviour is removed.

**Rationale**: The product decision is "Default Dark". An automatic
"system" mode would be a fourth theme and would mean the toggle has
four positions. The user task said three. Removing the media query
also removes the only place CSS branched on OS preference, which
collapses the canonical-path question to "the design-token block, full
stop" (Principle XIII).

**Alternatives considered**:
- Add a fourth `system` option that follows `prefers-color-scheme`:
  rejected — out of spec.
- Keep `prefers-color-scheme` as an extra fallback when no localStorage
  value is present: rejected — that is exactly the kind of hidden
  parallel path Principle XIII forbids. The default is dark, period.

### D4. `localStorage` key naming

**Decision**: Use the literal key `my-gather:theme`.

**Rationale**: The project name prefix avoids collisions when a user
opens reports for multiple projects in the same browser origin (e.g.,
`file://`). The colon separator matches the convention already used in
the existing report state-persistence keys (which also key by report
ID for section open/closed state).

**Alternatives considered**:
- Plain `theme`: too generic; collides with anything else served from
  the same origin.
- Per-report keys: rejected — the user's theme preference is global to
  the tool, not per-report.

### D5. Series palette as `--series-1`…`--series-N` CSS variables

**Decision**: Define 16 series tokens (`--series-1` through `--series-16`)
in each theme block. The chart code reads them via `cssVar()` and cycles
modulo 16 for series past the 16th.

**Rationale**: The current `SERIES_COLORS` JS array already has 16
entries (`render/assets/app-js/00.js:267-272`); 16 is the natural cap for
a single-screen legend. Per-theme overrides become a single CSS edit
instead of a JS-array edit. The Okabe-Ito theme provides 8 named colors;
positions 9–16 of that theme cycle the same 8 colors so a 12-line chart
still uses only Okabe-Ito values (FR-011).

**Alternatives considered**:
- Keep a JS array but make it theme-keyed: rejected — that's two
  parallel theme sources of truth (CSS for everything else, JS for
  series). Principle XIII forbids parallel sources.
- Use only 8 tokens for all themes: rejected — the dark and light
  palettes already use 16 distinct colors and dropping to 8 would
  regress legend readability for 9+ series charts in those themes.

### D6. Component testing strategy without adding a JS runner

**Decision**: Test the theme system from Go, asserting the embedded CSS
and JS asset strings carry the required tokens, the Okabe-Ito values
verbatim, and the right structural pieces (pre-paint script in template,
`SERIES_COLORS` removed, `cssVar('--series-` present in JS, three
`[data-theme="..."]` blocks in CSS).

**Rationale**: The main module has no JS test runner today. Adding
JSDOM/Vitest to the main module would add a third-party transitive
dependency footprint (Principle X), would split test runners across the
repo (only `feedback-worker/` uses Vitest), and would not actually catch
something the Go-side string assertions miss for a static-file report.
The spec calls out the *behaviours* to assert (data-theme attribute
updates, palette tokens resolve to expected values); both are satisfied
by inspecting the strings that the browser will execute.

**Alternatives considered**:
- Add JSDOM + Vitest in the main module: rejected — minimal-deps
  violation, and the report runs in real browsers that JSDOM only
  approximates.
- Add a Go-side `headless-browser` integration: rejected — would pull a
  Chromium binary into CI, far heavier than the value adds for three
  themes worth of palette tokens.

### D7. Toggle UX — native `<select>`

**Decision**: A native `<select id="theme-picker">` with three `<option>`
elements (`Dark`, `Light`, `Color-blind safe`), labelled by an adjacent
`<label class="theme-label">Theme</label>`, sits inline inside
`header.app-header` to the left of the existing `#feedback-open` button
so the primary feedback action keeps the rightmost visual slot. The
picker therefore appears earlier in DOM source order than
`#feedback-open`; this ordering is enforced by tests in
`render/theming_test.go`.

**Rationale**: Native select is keyboard-accessible by default, screen
reader announces it, weighs almost nothing, and matches the
"unobtrusive corner" placement the user asked for. No new component
library and no new ARIA wiring are needed. The styling overrides for
the select live in `04.css` next to the token block so theme contrast
is also one-stop-shop.

**Alternatives considered**:
- Three buttons or a radio group: more chrome in the header for no
  added value at three options.
- An icon-only menu with a popover: more code, more focus management,
  and worse accessibility out of the box.

## Open questions

None. All product decisions were provided up-front by the user.

## References

- Okabe, M.; Ito, K. (2002). "Color Universal Design — How to make
  figures and presentations that are friendly to colorblind people."
  J*FLY Data Depository for Drosophila Researchers. The 8-color palette
  used here is the canonical citation; reproduced in FR-011 verbatim.
- Existing canonical patterns reused:
  - `render/assets/app-js/00.js:311` — `cssVar(name, fallback)` helper.
  - `render/assets/app-js/00.js:344, 381` — already reads `--fg-dim`
    and `--fg-muted` for chart axis colors via `cssVar()`. The new
    `--series-N` reads use the same path.
  - `render/assets.go:38-58` — `mustConcatEmbeddedAssetParts(dir)`
    deterministically concatenates asset parts in lexical order, so
    `04.css` and `06.js` land at the tail of their bundles without
    code changes to the embed plumbing.
