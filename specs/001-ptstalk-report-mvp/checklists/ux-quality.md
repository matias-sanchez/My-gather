# UX-Quality Audit Checklist

**Normative**: FR-038, FR-039, FR-040, FR-041, SC-011
**Research**: R13
**Principle**: Constitution XI ("Reports Optimized for Humans Under
Pressure")

---

## How to run an audit

1. Render the report against `testdata/example2/` (multi-snapshot)
   and, where specified, against a 0-sample / 1-sample / large
   synthetic fixture.
2. Open the rendered HTML in the three reference browsers (latest
   stable Chromium, Firefox, Safari) at 1440×900, 1920×1080, and
   375×667 (mobile is out of scope per Assumptions but the
   layout MUST NOT crash below 800 px — it collapses to top-horizontal
   nav per FR-031).
3. Walk every row below. Mark **PASS** / **FAIL** / **DEFERRED**.
   DEFERRED items MUST link a follow-up task ID (T###).
4. Commit the result as
   `specs/001-ptstalk-report-mvp/ux-audits/<YYYY-MM-DD>-<label>.md`.

A PASS run is the precondition for any feature-001 release cut
(SC-011). FAIL items block merge; DEFERRED items require owner
approval recorded in the same audit file.

---

## 0. Global invariants

Applies to every rendered surface, independent of section.

- [ ] **G-1** Every font size on every rendered surface MUST be
      expressed via one of the four named CSS custom properties
      `var(--font-h2)`, `var(--font-h3)`, `var(--font-body)`, or
      `var(--font-caption)`. Check is performed **against the
      embedded stylesheet in the rendered HTML** (the single
      top-level `<style>` block the report ships, not the source
      file): extract that block and assert no literal `font-size:`
      declaration appears outside the `:root`-level custom property
      definitions. Fully auditable from the shipped report alone.
- [ ] **G-2** Spacing rhythm is pinned to a single `8px` base grid
      exposed as `var(--grid)` on `:root`. Every margin, padding,
      and gap value in the rendered report's embedded `<style>`
      block MUST be either `0`, `var(--grid)`, or
      `calc(var(--grid) * N)` for an integer `N`. Same extraction
      method as G-1; the audit does not require access to
      `render/assets/app.css` in source form.
- [ ] **G-3** Colour palette: one primary accent, one muted-text
      tone, one warning tone, one success tone. Charts use the same
      named palette across all sections.
- [ ] **G-4** All chart axes format numbers via the shared
      `formatYTick` helper (compact: `1.2K` / `3.4M` / `1.2B`). No
      axis falls back to raw `toLocaleString()`.
- [ ] **G-5** Every rendered timestamp — including server-rendered
      header / banner / table text, chart tooltips and other
      JavaScript-rendered timestamp labels, and timestamp fields in
      the embedded JSON payload — uses the shared UTC ISO-8601
      formatter (Principle IV). No timestamp-formatting call site
      uses locale-dependent (`toLocaleDateString`, `toLocaleTime…`)
      or raw-epoch time. Auditable via inspection of every function
      in `app.js` that takes a timestamp or `Date` argument and
      returns a human-readable string: each MUST route through the
      shared UTC formatter helper. `toLocaleString()` is NOT
      forbidden outright — it remains permitted for non-timestamp
      numeric formatting (e.g., integer separators), which is
      governed by G-4. The audit check is "no locale-dependent
      timestamp formatting", not "no `toLocaleString` anywhere".
- [ ] **G-6** Every interactive element (button, dropdown, chip,
      checkbox, search input) is reachable and operable by keyboard
      alone (FR-031, FR-036).
- [ ] **G-7** No visible element relies on colour alone to convey
      meaning (pass/fail badge, modified/default badge, severity
      icon); each uses both colour **and** a text label or shape
      cue.
- [ ] **G-8** The shared `render/assets/app.css` expresses all the
      above as CSS custom properties (`--fg`, `--accent`,
      `--muted`, `--warn`, `--ok`, `--grid`, `--font-h2`,
      `--font-h3`, `--font-body`). Per-template inline `<style>`
      blocks are forbidden (FR-039).

## 1. Header + banners

- [ ] **H-1** Verbatim-passthrough banner (FR-026) is visible above
      the fold at 1440×900 for a 2-section-minimum report.
- [ ] **H-2** Report title (hostname + snapshot count) is
      immediately scannable.
- [ ] **H-3** `GeneratedAt` timestamp is labelled as the sole
      non-deterministic field and rendered via the shared formatter.
- [ ] **H-4** Tool version (`my-gather <semver>`) + short commit
      visible in the header or footer (`render/templates/report.html.tmpl`;
      no standalone FR — FR-023 governs the CLI counterpart).
- [ ] **H-5** No duplicated metadata between header, footer, and
      section breadcrumbs.

## 2. Navigation rail

- [ ] **N-1** Every named subview is listed once and reachable in a
      single click (SC-009).
- [ ] **N-2** At ≥ 801 px the rail is `position: sticky`; at
      < 801 px it collapses to top-horizontal (R9).
- [ ] **N-3** Without JavaScript the rail degrades to a plain anchor
      list (FR-031).
- [ ] **N-4** Cmd/Ctrl+`\` toggles the rail (FR-036); the
      accelerator is discoverable via a title attribute or visible
      cue.
- [ ] **N-5** Collapse state of each nav group persists across
      reloads via the `Report.ReportID`-keyed localStorage scheme
      (FR-032).
- [ ] **N-6** Rail does NOT re-present any data; only titles and
      anchors (FR-038).

## 3. OS Usage section

- [ ] **OS-1** Disk utilization chart (`-iostat`): per-device series
      labelled; peak-utilization device identifiable at a glance.
- [ ] **OS-2** Top CPU processes chart (`-top`): each plotted
      process identified by both PID and command, not just PID.
- [ ] **OS-3** vmstat saturation chart (`-vmstat`): all 13 declared
      series (`VmstatData.Series` in `data-model.md`) are
      legend-labelled without overlapping the plot; a collapsed-
      legend affordance exists when screen real estate is tight.
- [ ] **OS-4** Three subview anchors — `os-disk-util`, `os-top-cpu`,
      `os-vmstat` — present and nav-linked (SC-005 / T096).
- [ ] **OS-5** Multi-snapshot: all three time-series charts show
      snapshot-boundary markers at the right timestamps (FR-018 /
      FR-030, T054).
- [ ] **OS-6** When one of `-iostat` / `-top` / `-vmstat` is absent
      the corresponding subview shows "data not available" naming
      the missing file; the other two render normally (FR-007).
- [ ] **OS-7** No OS-Usage chart re-plots data that canonically
      belongs to Database Usage (e.g., no mysqladmin-derived series
      embedded in vmstat) and vice versa (FR-038).

## 4. Variables section

- [ ] **V-1** Search input (`<input type="search">`) is visible
      without scrolling below the first screenful of variables.
- [ ] **V-2** The modified-vs-default badge (FR-033) is visible
      without hover; variables absent from the defaults map render
      without either badge.
- [ ] **V-3** Entries sorted alphabetically; duplicates deduped
      with a Diagnostic recorded (FR-012).
- [ ] **V-4** Multi-snapshot: one readable `<details>` block per
      snapshot, each with its own table (FR-018).
- [ ] **V-5** Client-side filter toggles `hidden` on `<tr>` rows
      whose `data-variable-name` doesn't substring-match (FR-013);
      filter is bypassable by clearing the input.
- [ ] **V-6** No variable row spills beyond its container at any
      screen width ≥ 800 px.

## 5. Database Usage section

- [ ] **DB-1** InnoDB scalars (FR-014): four subviews — semaphores,
      pending I/O, AHI activity, HLL — laid out side-by-side when
      multiple snapshots exist; visual form matches the shape of
      the data.
- [ ] **DB-2** Counter-deltas chart (FR-015): keyboard-accessible
      toggle UI surfaces every variable present in the embedded
      `report-data` JSON payload (reviewer reads the payload's
      `variables` array and confirms every entry is reachable from
      the UI). Category chooser (FR-034) lists every category
      present in the payload's `categories` array, in array order.
      Reviewer reads both from the rendered HTML alone — no source
      file lookup required.
- [ ] **DB-3** Counter-deltas chart respects `defaultVisible`
      (FR-035): column 0 (initial tally) is NOT in the default
      series; viewer may opt in via the toggle.
- [ ] **DB-4** Thread-states stacked chart (FR-017): legend listing
      every state does not obscure the plot; "Other" bucket present
      when unknown states exist.
- [ ] **DB-5** Multi-snapshot boundary markers visible on the
      counter-deltas and thread-states time-series (FR-018 /
      FR-030, T054).
- [ ] **DB-6** No DB-Usage widget re-plots OS-Usage data (FR-038).

## 6. Parser Diagnostics panel

- [ ] **PD-1** Collapsed by default (FR-032, Principle XI).
- [ ] **PD-2** Each diagnostic row carries: file basename,
      location (line or byte when known, otherwise `—`), severity
      (one of `Info` / `Warning` / `Error`), and a single-line
      message.
- [ ] **PD-3** `SeverityInfo` diagnostics appear here and only here
      (never on stderr, per FR-027 / FR-030). Snapshot-boundary
      info from R8 improvement D is the canonical example and MUST
      appear in this panel for any multi-Snapshot `-mysqladmin`
      input.
- [ ] **PD-4** Warnings / Errors are colour-coded AND prefixed with
      a word (`[warning]` / `[error]`) — not colour-only (G-7).
- [ ] **PD-5** No diagnostic is duplicated from a subview banner
      (FR-038).

## 7. Print layout (FR-037)

- [ ] **P-1** `@media print` expands every `<details>`.
- [ ] **P-2** Sticky nav rail is hidden in print.
- [ ] **P-3** Layout collapses to a single scrolling column.
- [ ] **P-4** Verbatim-passthrough banner (FR-026) is visible on
      the first printed page.
- [ ] **P-5** Charts render at a legible size and don't split
      awkwardly across page breaks (use `page-break-inside: avoid`
      on chart containers).
- [ ] **P-6** Colour palette degrades acceptably under greyscale
      printing.

## 8. Robustness across the data envelope (FR-040)

Run the full audit against each of these fixture classes.

- [ ] **R-0** Zero-sample collection: every subview shows its FR-007
      banner; no empty `<canvas>`, no bare header.
- [ ] **R-1** One-sample collection: every subview renders a scalar
      callout or single-row table; never an empty plot area.
- [ ] **R-2** Typical collection (~10²–10³ samples): reference run
      against `testdata/example2/` passes all § 0–7 items.
- [ ] **R-M** Maximum collection (approaching 1 GB / 200 MB per
      FR-025): axis labels not clipped; legend not overlapping plot;
      no subview overflows its container. (The near-1 GB synthetic
      fixture is built as part of T116; no pre-committed testdata
      path exists for this class.)
- [ ] **R-MS** Multi-snapshot collection (`testdata/example2/` has
      two): snapshot-scoped subviews show one block per snapshot;
      time-series subviews show vertical boundary markers.
- [ ] **R-PC** Partial collector (file truncated mid-sample): the
      subview renders whatever parsed samples are available, shows
      a "partial data" sub-banner, and records a Warning diagnostic
      (FR-008).
- [ ] **R-UV** Unsupported pt-stalk version (format signature
      matches neither V1 nor V2): subview shows "unsupported
      pt-stalk version" banner naming the file; no abort (FR-024).

## 9. No-duplication invariant (FR-038)

- [ ] **D-1** Each of the seven canonical data surfaces (iostat,
      top, vmstat, variables, innodbstatus, mysqladmin, processlist)
      has exactly one home view. Spot-check: grep the rendered HTML
      for key values (e.g., a distinctive variable name, a specific
      PID) and confirm each appears in only one subview outside of
      the Parser Diagnostics panel.
- [ ] **D-2** No section introduces a summary or overview widget that
      re-plots or re-tabulates data already present in another
      section's canonical subview (confirm by scanning the rendered
      HTML for any element that aggregates or repeats values from a
      non-home source). When a widget is added in a future feature,
      add a corresponding audit row to this section at that time.
- [ ] **D-3** Cross-references between sections are navigation aids
      only (anchor links, `<a href="#sec-...">`), never duplicated
      data displays.

---

## Audit record template

File path: `specs/001-ptstalk-report-mvp/ux-audits/<YYYY-MM-DD>-<label>.md`

```markdown
# UX Audit — <label> — <YYYY-MM-DD>

**Auditor**: <name>
**Reviewed commit**: `<short-sha>`
**Reference fixtures**: testdata/example2/, <synthetic-1-sample>, …
**Browsers**: Chromium <v>, Firefox <v>, Safari <v>
**Viewports**: 1440×900, 1920×1080, 375×667 (crash-check only — mobile UX out of scope per spec Assumptions)

## Results

| ID | Status | Notes |
|---|---|---|
| G-1 | PASS | — |
| G-2 | PASS | — |
| …   | …    | … |
| OS-3 | DEFERRED -> T110 | vmstat legend overlaps at 1280 px |
| …   | …    | … |

## Deferred items

| ID | Follow-up task | Owner | Target |
|---|---|---|---|
| OS-3 | T110 | @matias-sanchez | next cut |

## Summary

<one paragraph: what's shippable, what's not, any thematic observations>
```

---

## Changelog

- 2026-04-22 — Initial version (feature 001 MVP baseline).
