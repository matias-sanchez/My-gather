# Research: pt-stalk Report MVP

All Phase 0 open questions resolved. Each entry records Decision,
Rationale, and Alternatives considered.

---

## R1. Time-series chart library to vendor

**Decision**: Vendor **uPlot** (`leeoniya/uPlot`) version pinned at the
time of first build. Single-file `uPlot.iife.min.js` at roughly 40 KB
minified + 4 KB CSS, zero external dependencies, MIT licensed.

**Rationale**:

- **Size**: uPlot's minified bundle is roughly 40 KB; nearest
  competitors (Chart.js ≈ 200 KB minified; Plotly ≈ 3 MB) inflate
  every report file and violate Principle XI's "prioritise signal over
  clutter" more than it helps.
- **Performance**: uPlot renders tens of thousands of time-series
  points smoothly on commodity laptops — matches the SC-008 1 GB input
  profile, which can yield ~5000+ iostat samples per device.
- **License compatibility**: MIT is compatible with the project's
  license and requires only that we ship the copyright notice with the
  bundled asset. We satisfy that by committing `render/assets/NOTICE`
  alongside `chart.min.js`.
- **Zero dependencies**: uPlot does not pull transitive JavaScript
  dependencies, consistent with Principle X's spirit even for vendored
  frontend assets.
- **Static output**: The library produces `<canvas>` renderings with
  no network calls of its own (Principle IX).

**Alternatives considered**:

- **Chart.js** — popular, excellent API, but ~200 KB minified plus a
  dependency on `date-fns` for time axes (or a larger bundle with it
  inlined). Too big for a report file that must be emailable.
- **Plotly.js** — feature-rich but ≥ 3 MB bundles; makes every report
  a multi-megabyte file. Hard no.
- **ECharts** — ~1 MB bundle. Better than Plotly but still
  uncompetitive with uPlot on size.
- **Hand-rolled SVG** — no third-party JS at all. Tempting for
  Principle X, but reinvents interactivity (tooltips, pan, legend
  toggle) that the spec's Q5 "interactive mysqladmin graph" requires.
  Engineering cost outweighs the size saving.
- **D3.js** — a library for building charts, not a chart library.
  Shifts the cost to us.

**Implementation notes**:
- Asset committed at `render/assets/chart.min.js` and
  `render/assets/chart.min.css`. Upstream NOTICE / LICENSE committed
  beside them.
- Upgrade cadence: major version bumps are a reviewed change per
  Principle X, documented in the PR that bumps the pinned version.

---

## R2. Per-file format-version detection

**Decision**: Each parser implements a signature check at the top of
its parse routine that inspects the first few lines (or the first
header block) of the file and branches between the two supported
format variants. The detected version is recorded on the resulting
`SourceFile.Version` field. If the signature matches neither variant,
the parser records a `Diagnostic` with severity `Error` and status
`ParseFailed`, and the renderer shows the "unsupported pt-stalk
version" banner for that subview (spec FR-024).

**Rationale**:

- `pt-stalk` collectors historically prepend an identifying header line
  (e.g., `TS <epoch>` with a `date` line for sampled collectors; the
  `variables` file is pure `SHOW GLOBAL VARIABLES` output with a known
  column count). Columns sometimes shift between versions, so we read
  the column header too and validate width.
- Keeping detection inside each parser (rather than a central version
  registry) localises the knowledge: when we add a new collector, the
  same file holds both the parser and the version heuristic.
- Failing per-file (not per-collection) preserves Principle III —
  other collectors can still succeed.

**Alternatives considered**:

- **Central version registry** — one table mapping file fingerprints to
  parsers. Tighter but adds an indirection that rarely pays off with
  only two supported versions.
- **Parse by suffix only** — implicitly assume one version. Fails when
  the customer runs an older `pt-stalk`.

**Implementation notes**:
- `parse/version.go` exposes typed constants `FormatV1`, `FormatV2`
  plus a helper `DetectFormat(peekedBytes []byte, suffix string)`.
- Each parser calls `DetectFormat` and switches on the returned const.

---

## R3. Byte-deterministic HTML output

**Decision**: Determinism is enforced at a single choke point:
`render/deterministic.go`. Every rendering path that could introduce
order- or locale-dependence routes through helpers in that file.
Golden tests enforce the result.

Specific rules:

1. **Map iteration**: never render a Go map directly. Helpers
   `sortedKeys[K Ordered, V any](m map[K]V) []K` and
   `sortedEntries[K Ordered, V any](m map[K]V) []Entry[K, V]`
   produce sorted slices. Templates iterate only over those.
2. **Float formatting**: all numeric values in the rendered output and
   in the embedded JSON payload are formatted via
   `formatFloat(v float64, precision int) string`, which wraps
   `strconv.FormatFloat(v, 'f', precision, 64)`. Default precision is
   4 for percentages, 2 for byte/sec values, 0 for counts.
3. **Element IDs**: generated from an ordinal counter initialised per
   render and incremented in a stable order (section by section,
   subview by subview). No UUIDs, no `math/rand`.
4. **Template iteration**: `html/template` range is invoked only over
   sorted slices — never over maps, never over channel reads.
5. **Timestamp handling**: all timestamps read from input are
   serialised via `time.Format(time.RFC3339Nano)` using UTC after
   conversion from the pt-stalk locale (which is the collecting
   host's local TZ — we capture the offset from the file header when
   present and convert to UTC; when absent, we label the value as
   "host-local").
6. **The only non-deterministic field**: `GeneratedAt` in the report
   header, explicitly labelled. A determinism test runs the render
   twice and asserts that diffing the two outputs produces exactly
   one differing line, matching the `GeneratedAt` marker.

**Rationale**: Principle IV and spec SC-003 require byte equality. A
single helper module is cheaper to audit than audits scattered across
render sites.

**Alternatives considered**:

- **Ad-hoc sorts at every render site** — rejected; too easy to miss
  one and break the golden test.
- **Post-hoc normaliser** (run the HTML through a formatter as a final
  pass) — rejected; it masks bugs rather than preventing them and
  doesn't help with the embedded JSON payload.

---

## R4. Offline interactive mysqladmin chart

**Decision**: The `-mysqladmin` parser emits a time-series payload for
every variable present across samples. The renderer writes that
payload as a JSON document embedded inside a
`<script type="application/json" id="mysqladmin-data">` block. A
tiny companion script (`render/assets/app.js`, ~2–3 KB) reads the
block on DOMContentLoaded, renders an uPlot chart, and wires a
`<select multiple>` control to toggle which variables are plotted.
Delta computation happens at render time in Go (not in the browser),
so the JSON payload already contains per-sample deltas.

**Rationale**:

- **Deterministic**: deltas computed in Go go through the same
  float-formatting helpers as the rest of the output; computing them
  in JavaScript would pull floating-point rounding into the browser
  and risk golden-test flakiness across engines.
- **Offline**: everything the chart needs is in the one HTML file.
  No fetch, no script imports other than the `go:embed`'d ones.
- **Small surface**: the client-side glue is small enough to audit in
  one sitting. No framework.
- **Toggle UX**: a multi-select control with keyboard support is
  simple and accessible enough for engineer use; no libraries required.

**Rationale for delta-in-Go over delta-in-JS**:

Counters like `Com_select` in `SHOW GLOBAL STATUS` are monotonic; a
"sample delta" is `s_i - s_{i-1}` for most counters but not all (e.g.,
`Uptime`, `Threads_running` are not counters). The parser knows which
category each variable falls into (hard-coded allowlist maintained per
pt-stalk version). Computing that in Go keeps the classification logic
version-aware and testable via golden files; computing it in the
browser would duplicate the allowlist and could drift.

**Alternatives considered**:

- **Ship raw counter values + delta-in-JS** — reject, as above.
- **Pre-render the chart as static SVG** — kills the "interactive
  toggle" requirement (spec FR-015/FR-016).
- **Separate data file + fetch()** — breaks Principle V (self-contained).

---

## R5. Detecting "is this a pt-stalk directory?"

**Decision**: A directory is recognised as pt-stalk iff it contains
**at least one file matching the pattern**
`YYYY_MM_DD_HH_MM_SS-<suffix>` where `<suffix>` is in the set of
known pt-stalk collectors (a superset of the seven we parse — includes
`-hostname`, `-pt-summary.out`, `-trigger`, etc.) **OR** it contains
the files `pt-summary.out` or `pt-mysql-summary.out` at the root.

Discovery precedence inside `parse.Discover`:

1. Locate timestamped files matching the regex
   `^\d{4}_\d{2}_\d{2}_\d{2}_\d{2}_\d{2}-[a-z0-9_-]+$`.
2. Group them by timestamp prefix → set of Snapshots.
3. If zero snapshots were found AND neither `pt-summary.out` nor
   `pt-mysql-summary.out` is present, return `ErrNotAPtStalkDir`. The
   CLI maps this to exit code 4 and writes nothing.
4. **Otherwise** (snapshots exist OR a summary file is present), the
   directory is recognised as pt-stalk even if none of the seven
   supported collectors are among the timestamped files. `Discover`
   returns a populated `Collection` anyway (possibly with no typed
   payload for any SourceFile). The renderer then emits a report in
   which every section shows its "data not available" banner, and the
   tool exits 0. This matches spec Edge Cases ("empty-but-valid
   collection" vs "not a pt-stalk dir at all"). No wrapper error;
   distinction between the two cases is made by whether Discover
   returns an error at all, not by error typing.

**Rationale**: The presence of any timestamped collector file is a
near-zero-false-positive indicator. `pt-summary.out` fallback catches
pt-stalk dumps that happened to not capture per-interval files (rare
but observed).

**Alternatives considered**:

- **Look for a marker file only** — too brittle; pt-stalk doesn't
  write a sentinel like `.pt-stalk`.
- **Parse one candidate file and see if it matches** — too expensive
  before we've even committed to the directory.

---

## R6. Fixture anonymisation strategy

**Decision**: Fixtures committed to `testdata/` are derived from
`_references/examples/` via a scripted anonymisation pass documented in
`testdata/README.md`. The script applies a deterministic, reversible
(for local debugging) substitution over the following fields:

- Hostnames → `example-db-01`, `example-db-02`, …
- IPv4 / IPv6 addresses → RFC-5737 / RFC-3849 documentation ranges.
- MySQL user names → `appuser`, `repluser`, `sysadmin` (structural
  roles preserved).
- Database names → `app_db`, `reporting_db`, `analytics_db`.
- Query text in `-processlist` / `-innodbstatus1` →
  `/* redacted query :<hash> */` keyed by a stable hash so the same
  query reduces to the same redaction across samples (preserves
  deduplication in the rendered output).
- Absolute paths containing usernames → `/home/redacted/…`.

Structural properties (timestamps, numeric counts, variable names,
process IDs, states) are preserved verbatim — they are what the
parsers and golden tests actually care about.

**Rationale**: Spec FR-026 commits the tool itself to verbatim
passthrough, but committed fixtures live in a public repository;
anonymising at the fixture boundary is a reasonable and common
practice.

**Alternatives considered**:

- **Commit verbatim** — risks leaking real customer data into the
  public repo.
- **Generate synthetic fixtures from scratch** — kills the "real
  customer data drives the parser" advantage; we'd be writing
  parsers against our own fiction.
- **Encrypt fixtures and decrypt in tests** — unreviewable; also
  breaks the "anyone can `go test`" invariant.

**Implementation notes**:

- Anonymisation script lives at `scripts/anonymise-fixtures.sh`
  (shell + `sed`/`awk`, no third-party tool). Input: a source dir
  from `_references/examples/`. Output: a sibling under `testdata/`.
- The script is deterministic given the same input; re-running
  produces a byte-identical fixture.
- `_references/examples/` remains in the repo as the source material
  and is not itself used by any test — only `testdata/`.

---

## R7. CI linter for forbidden imports (Principle IX)

**Decision**: A custom `go test`-shaped check in a file named
`internal/lint/imports_test.go` (or its out-of-tree equivalent if we
avoid `internal/` in v1) walks the package AST for every package under
`parse/`, `model/`, `render/`, and `cmd/` and fails if any of the
following packages appear in the import graph: `net/http`, `net`,
`net/rpc`, `net/smtp`, `crypto/tls`. Test-only helper files (`_test.go`
suffixed) are exempt.

**Rationale**: Principle IX (zero network) needs enforcement that
outlives human review. A stdlib-only linter (using `go/parser`,
`go/ast`, `go/types`) is trivial to write and has no dependencies
(Principle X). Running it as a regular test means CI catches violations
in the normal `go test ./...` pass.

**Alternatives considered**:

- **Rely on code review** — brittle, doesn't scale beyond a handful of
  contributors.
- **Use `golangci-lint` depguard** — adds a direct dependency on the
  linter binary in CI; the stdlib version is less overhead.

---

## R8. `-mysqladmin` delta-computation algorithm (native Go port of pt-mext)

**Decision**: Port the algorithm from
`_references/pt-mext/pt-mext-improved.cpp` to native Go inside
`parse/mysqladmin.go` as the single source of truth for delta
computation. Lock correctness via a committed test fixture derived
from the worked example at the tail of the C++ file (lines 146–189),
not via runtime parity against the C++ binary. Retain the C++ file as
historical reference and algorithmic documentation.

**Algorithm ported from pt-mext**:

1. Skip every line that does not begin with `|` — this filters out the
   `+---+` separator rows and blank lines emitted by `mysqladmin
   extended-status`.
2. The `Variable_name | Value` header row marks a new sample: when
   encountered, increment the column index and continue.
3. Between two data rows for the same variable, `delta = current -
   previous` where `previous` is tracked per-variable.
4. The first column's "delta" is the raw initial tally; it is stored
   but EXCLUDED from aggregate statistics (matches the C++'s `col > 1`
   guard at line 49).
5. Aggregates (total, min, max, avg) are computed over columns 2..N,
   using `col==2` to seed the running min (matches the C++ ternary at
   line 51 of the reference).
6. Variables iterate in alphabetical order because the C++ uses
   `std::map`; the Go implementation matches by sorting
   `VariableNames` before output (already required by Principle IV
   determinism).

**Deliberate improvements the Go port makes over the C++ reference**:

A. **Counter vs. gauge classification** (the C++ treats everything as
   a counter). The Go parser ships with a per-format-version
   allowlist of counter variables (e.g., `Com_select`, `Questions`,
   `Bytes_received`) derived from MySQL's `SHOW GLOBAL STATUS`
   documentation. Non-counter variables (e.g., `Threads_running`,
   `Uptime`, `Open_tables`) are stored as raw per-sample values, not
   deltas. The `IsCounter map[string]bool` on `MysqladminData`
   captures this classification (see data-model.md).

B. **Floating-point aggregates** (the C++ uses integer division for
   `avg` at line 53). The Go implementation uses `float64` for
   aggregates so `avg` does not truncate to zero on small-delta
   variables.

C. **Variable-set drift across samples** (the C++ silently treats a
   missing variable as zero). The Go parser records a
   `model.Diagnostic(Severity=Warning)` when a variable appears in
   some samples but not others, so the engineer knows a restart or
   version change happened mid-collection. The delta for a missing
   slot is stored as `NaN`, not 0, and the renderer draws a
   discontinuity gap at that point.

**Correctness anchor** (replaces the previously-planned parity test):

The worked example at lines 146–177 of
`_references/pt-mext/pt-mext-improved.cpp` is committed verbatim to
`testdata/pt-mext/input.txt`. The expected output at lines 181–189 of
the same file is committed as `testdata/pt-mext/expected.txt`. A
golden test (`parse/mysqladmin_test.go::TestPtMextFixture`) runs the
Go parser on `input.txt` and asserts byte-for-byte equality with
`expected.txt` for counter aggregates (total / min / max / avg).
Because the C++ author authored this example and expected output
together, treating it as a golden fixture gives us pt-mext's
correctness guarantee without any C++ toolchain.

**No parity test, no build-tag test target, no `tests/parity/`
directory.** The C++ binary does not need to be compiled to run any
My-gather test.

**Rationale**:

- **Single source of truth** — the algorithm exists once, in Go,
  reducing drift risk to zero.
- **Zero toolchain cost** — CI and contributors never install a C++
  compiler to build or verify My-gather.
- **Correctness preserved** — the worked example in the C++ file is
  author-certified expected output; promoting it to a committed golden
  fixture gives us the same confidence a parity test would, forever,
  with no runtime dependency.
- **Improvements are unambiguous** — the three Go-specific behaviours
  (counter/gauge, float64 avg, NaN-on-missing) are simply how the Go
  code works, not annotated deviations from a parity target.

**Alternatives considered**:

- **Keep pt-mext as runtime parity target** — rejected (Option A of
  the triage we did). Creates drift risk and ongoing maintenance cost
  of a C++ toolchain path.
- **Port and delete the C++ file** — rejected (Option B). The C++
  reference is useful documentation of algorithmic intent and the
  expected-output comments at the bottom are the cleanest fixture
  source we have. Keeping `_references/pt-mext/` as historical
  reference is free.
- **Reinvent the algorithm from the MySQL docs** — rejected. pt-mext
  already codifies the edge cases (separator rows, new-sample
  detection, initial-tally handling) that the docs gloss over.
- **Call pt-mext as a subprocess at runtime** — rejected. Violates
  Principle I (single static Go binary).

**Optional future**: a `my-gather mext <file>` CLI subcommand that
exposes the delta algorithm directly for shell-piping workflows.
Not in scope for this feature — the function itself must already
exist in `parse/` to populate `MysqladminData`, so a subcommand is
cheap to add in a later feature. Flagged here so the port is written
with that future in mind (pure function, no rendering coupling).

---

## R9. Navigation index and collapsible sections

**Decision**: Navigation index is a plain `<nav>` element rendered
server-side (in Go) from a deterministic `Report.Navigation []NavEntry`
list, with CSS `position: sticky` for on-scroll visibility. Collapse /
expand is driven by native HTML `<details>` / `<summary>` elements
wrapping each section and subview; a small JavaScript module persists
their `open` state to `localStorage` under a per-report key
(`Report.ReportID` — a deterministic hash of the embedded data).
Default first-load state: all `<details>` `open` except the Parser
Diagnostics panel, which is closed. Works without JavaScript because
`<details>` / `<summary>` are native HTML controls.

**Rationale**:

- **`<details>` is the right primitive**. Native browser support
  everywhere we care about, keyboard-accessible by default, screen-
  reader semantic, zero CSS-only alternatives approach this on cost.
- **`position: sticky` for the nav rail** keeps us out of JavaScript
  for the scroll-following behaviour. Progressive enhancement only
  adds the state-persistence layer.
- **localStorage, not cookies, not sessionStorage**. Cookies would
  violate "self-contained" (they'd travel with the file to other
  viewers). `sessionStorage` doesn't persist across reloads. Local-
  storage is per-origin; for a `file://` report it's partitioned per-
  file-URL in modern browsers, which is the scope we want.
- **Per-report key via `ReportID`** avoids cross-report state
  collisions when a user opens two reports side by side. A hash of
  the canonicalised `Collection` (timestamps + filenames, excluding
  the non-deterministic `GeneratedAt`) gives a stable ID across
  re-renders of the same input — so reopening a regenerated report
  retains the viewer's collapse choices.
- **Parser Diagnostics collapsed by default** follows Principle XI
  ("prioritise signal over clutter"): most reports have zero or one
  diagnostics; showing an empty expanded panel adds noise for no
  signal.
- **Graceful degradation** (Principle III spirit): with JS disabled
  the `<details>` elements open/close, just without persisted state.
  Nav rail degrades to an anchor list at the top of the document via
  a simple `<noscript>` fallback CSS rule.

**Implementation notes**:

- `render.Render` populates `Report.Navigation` in a fixed order
  derived from section iteration (OS → Variables → DB → Parser
  Diagnostics), with one NavEntry per top-level section (Level=1) and
  one per named subview (Level=2). IDs are ordinal
  (`sec-os`, `sub-os-iostat`, `sec-variables`, etc.) — deterministic.
- `Report.ReportID` uses SHA-256 of the canonicalised Collection
  (excluding `GeneratedAt`) truncated to 12 hex chars — collision
  probability is negligible for the volumes a single engineer
  handles and is small enough to read.
- `render/assets/app.js` is extended (not a new file) with the
  collapse-persistence module. Bundle stays under ~4 KB target.
- The mysqladmin toggle UI state (from R4) uses the same
  `ReportID`-prefixed localStorage convention so state is consistent
  across features.

**Alternatives considered**:

- **Client-side-rendered nav via JS** — rejected. Requires JS to see
  the index at all; fails progressive-enhancement goals and adds a
  first-paint flash.
- **Accordion library** (e.g., a small third-party a11y-accordion) —
  rejected. `<details>` + CSS does the job natively; Principle X
  (minimal deps) disfavours adding a JS library for a native element.
- **Checkbox-hack CSS-only collapse** — rejected. `<details>` is a
  superior native primitive since Edge adopted it (2017+).
- **Cookies for state persistence** — rejected; would leak with the
  file.
- **URL-hash-encoded state** — rejected; pollutes the URL and
  interferes with anchor-link jumping from the nav index.
- **Always-expanded, no collapse** — rejected by FR-032.
- **Per-report-ID via timestamp only** — rejected; `GeneratedAt`
  changes every render, so collapse state would reset on every
  regeneration.

---

## Summary of decisions

| ID | Decision |
|----|----------|
| R1 | Vendor uPlot for charts. |
| R2 | Per-file version detection inside each parser; record on `SourceFile.Version`. |
| R3 | Centralise determinism helpers in `render/deterministic.go`. |
| R4 | Compute deltas in Go; embed JSON payload; tiny JS glue for toggle UI. |
| R5 | Discover via timestamped-file regex + `pt-summary.out` fallback. |
| R6 | Anonymise fixtures via deterministic `scripts/anonymise-fixtures.sh`. |
| R7 | Stdlib-only forbidden-import linter as a `go test` check. |
| R8 | Port pt-mext delta algorithm to native Go; commit the C++ file's worked example as a golden fixture; add counter/gauge classification, float64 aggregates, drift diagnostics. No parity test, no C++ toolchain requirement. |
| R9 | Navigation = `<nav>` + `position:sticky`; collapse = native `<details>`/`<summary>`; localStorage keyed by deterministic `Report.ReportID`; works (degraded) without JS. |

All Phase 0 items resolved. Phase 1 artifacts proceed below.
