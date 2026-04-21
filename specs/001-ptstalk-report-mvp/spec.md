# Feature Specification: pt-stalk Report MVP

**Feature Branch**: `001-ptstalk-report-mvp`
**Created**: 2026-04-21
**Status**: Draft
**Input**: User description: "Parse a pt-stalk output directory and emit a minimal self-contained HTML report. Exercises parse/, model/, render/ end-to-end, proves determinism and the embedded-assets pipeline, and gives every subsequent parser a golden-test home to plug into. The report should have 3 sections: OS usage (CPU/DISK/MEMORY), Variables, and Database usage. Source files: -iostat, -top, -variables, -vmstat, -innodbstatus1, -mysqladmin, -processlist."

## Clarifications

### Session 2026-04-21

- Q: Which pt-stalk release formats must the parsers accept? → A: Latest Percona Toolkit `pt-stalk` stable plus one major version back. Each supported version ships its own fixture under `testdata/` and its own golden file, per Principle VIII.
- Q: What is the upper bound on input size the tool must survive? → A: ≤ 1 GB total collection size; any single source file up to 200 MB. Streaming parsing where cheap, buffered where simpler. Beyond 1 GB is out of scope for the MVP.
- Q: Does the MVP redact sensitive data (hostnames, IPs, query text, user names) from the rendered HTML? → A: No. The report passes pt-stalk content through verbatim; redaction is the engineer's responsibility before sharing. Documented in the README and in a visible banner on the report.
- Q: What does the tool write to stderr during a run? → A: Silent on success by default; parser diagnostics are mirrored to stderr as one-line warnings as they are emitted; `-v` / `--verbose` streams per-file progress lines. Structural errors (bad input path, size-bound exceeded) always go to stderr.
- Q: Does the MVP emit any machine-readable side-car output (JSON, CSV) alongside the HTML? → A: No. HTML is the only CLI output in this feature; no `--json`, no side-car files. The `model/` package remains importable (required by Principle VI) but is not advertised, stabilised, or documented for third-party consumers in v1.
- Q: What is the normative algorithm for computing `-mysqladmin` deltas between samples? → A: The algorithm is **ported to native Go** inside `parse/mysqladmin.go`. The C++ reference at `_references/pt-mext/pt-mext-improved.cpp` is retained as the *behavioural specification source*: the worked example in its tail comments (input table lines 146–177 → expected output lines 181–189) is committed verbatim as a test fixture and golden file under `testdata/pt-mext/`. The Go implementation MUST reproduce that expected output for counter variables. Non-counter variables (e.g., `Threads_running`, `Uptime`) are classified as gauges and stored as raw values, not deltas — this is a deliberate improvement over the C++ reference, which treats everything as a counter.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - First-look triage report from a pt-stalk collection (Priority: P1)

A MySQL/Percona support engineer receives a pt-stalk output directory from
a customer jump host (often via scp or a zipped archive). Under pressure
during an incident, the engineer runs the tool against that directory and
gets back a single HTML file they can open locally — no extraction of
individual files, no command-line grepping, no server to spin up. The
report answers the question: "what did this system look like when pt-stalk
fired?"

**Why this priority**: This is the whole point of the tool. Without a
usable report coming out of a real pt-stalk directory, nothing else
matters. It also proves the parse → model → render pipeline end-to-end
and establishes the fixture + golden test foundation that every later
parser will plug into.

**Independent Test**: Point the CLI at `_references/examples/example2/`
(or any real pt-stalk dump), run it, open the resulting HTML in any
browser while disconnected from the internet, and confirm the three
top-level sections (OS usage, Variables, Database usage) are all present
and the document renders with working charts.

**Acceptance Scenarios**:

1. **Given** a pt-stalk output directory containing all seven source
   files (`-iostat`, `-top`, `-variables`, `-vmstat`, `-innodbstatus1`,
   `-mysqladmin`, `-processlist`), **When** the engineer runs the tool
   against that directory, **Then** the tool writes one self-contained
   `.html` file to the path the engineer specifies, and opening it in a
   browser with no network access shows the three sections with all
   graphs and tables rendered.
2. **Given** a pt-stalk output directory where some of the seven source
   files are missing or empty, **When** the engineer runs the tool,
   **Then** the report still renders, each section corresponding to a
   missing input displays an explicit "data not available" banner naming
   the file that was missing, and the tool exits with a zero status code.
3. **Given** the same input directory, **When** the engineer runs the
   tool twice in a row, **Then** both runs produce byte-identical HTML
   output (modulo a single explicit "generated at" timestamp the report
   itself labels as non-deterministic).

---

### User Story 2 - OS Usage section answers "what was the box doing?" (Priority: P2)

The OS Usage section gives the engineer a fast read on host-level
pressure at trigger time: which disks were hot, which processes were
burning CPU, and whether the system as a whole was saturated on CPU,
memory, or I/O.

**Why this priority**: After the end-to-end pipeline is proven (US1), the
OS view is the single highest-signal section for the 80% incident type
(I/O storms, runaway processes, memory pressure). It also forces the
three most operationally tricky parsers — `-iostat`, `-top`, `-vmstat` —
into place early.

**Independent Test**: Given a pt-stalk directory with only OS-related
source files present, the OS Usage section renders all three of its
subviews with correct values, matches the committed golden output
byte-for-byte, and is demonstrably useful to an engineer reviewing the
collection.

**Acceptance Scenarios**:

1. **Given** an `-iostat` file containing multiple samples for multiple
   block devices, **When** the report renders, **Then** the OS section
   includes one time-series chart showing per-device utilisation
   percentage across samples, with each device's average queue size
   available as a labelled series or hoverable value on the same chart.
2. **Given** a `-top` file containing repeated `top` batch snapshots,
   **When** the report renders, **Then** the OS section includes a
   time-series chart of the top 3 CPU-consuming processes across the
   collection window, where "top 3" is computed from aggregate CPU usage
   across all samples.
3. **Given** a `-vmstat` file, **When** the report renders, **Then** the
   OS section includes a time-series chart showing resource saturation
   indicators over time (at minimum: CPU user+system, run queue length,
   blocked processes, free/cached memory, swap in/out).
4. **Given** any one of `-iostat`, `-top`, or `-vmstat` is missing or
   unparseable, **Then** only that specific subview shows the "data not
   available" banner naming the missing file, and the other subviews in
   the OS section still render normally.

---

### User Story 3 - Variables section is a searchable reference for the MySQL config at trigger time (Priority: P3)

The Variables section shows every MySQL global variable captured by
pt-stalk so the engineer can answer "was this setting default? was it
changed? does it match what the customer said?" without opening the raw
file.

**Why this priority**: Variables are a high-value but low-complexity
section — a single table, no graphs, no per-session noise. It is the
simplest of the three sections and gives the engineer a reference they
will consult throughout the investigation. Ranks below OS because it is
less time-critical during live triage.

**Independent Test**: Given only a `-variables` file, the Variables
section renders a complete table matching a committed golden fixture.

**Acceptance Scenarios**:

1. **Given** a `-variables` file, **When** the report renders, **Then**
   the Variables section contains a single table listing every global
   variable and its value, sorted deterministically (alphabetical by
   variable name).
2. **Given** the `-variables` file contains session-scoped values or
   duplicate entries, **When** the report renders, **Then** only the
   global value per variable is displayed (no per-session rows).
3. **Given** the rendered table has more than a screenful of rows,
   **When** the engineer views the report, **Then** a client-side search
   input filters the table by variable name, operating entirely offline
   on the embedded data.

---

### User Story 4 - Database Usage section surfaces server-internal pressure (Priority: P4)

The Database Usage section answers "what was MySQL itself doing?": InnoDB
internals, throughput deltas across the collection window, and what
threads were stuck on.

**Why this priority**: Database internals are the deepest of the three
sections and benefit most from having the OS + Variables context already
in place. It also depends on the most complex parsers (InnoDB status is
multi-section free text; mysqladmin samples require delta computation).

**Independent Test**: Given a directory with only the three DB-section
source files, the Database Usage section renders all four subviews
matching their respective golden files.

**Acceptance Scenarios**:

1. **Given** an `-innodbstatus1` file, **When** the report renders,
   **Then** the Database section shows distinct views for: number of
   semaphores, pending I/O (reads + writes), adaptive hash index (AHI)
   activity, and history list length (HLL). Each view's visual form is
   chosen to match the shape of the data (e.g., a single-value gauge or
   callout for HLL; a small multi-series chart for pending I/O), and the
   choice is documented in the rendered section so it is reviewable.
2. **Given** a `-mysqladmin` file containing multiple `extended-status`
   samples, **When** the report renders, **Then** the Database section
   includes an interactive time-series chart showing per-sample deltas
   (not absolute counter values) where the engineer can toggle which
   variables are plotted; the toggle UI works entirely offline against
   the embedded data.
3. **Given** a `-processlist` file containing multiple snapshots, **When**
   the report renders, **Then** the Database section includes a chart
   showing the count of threads in each state across the collection
   window (states drawn directly from the `State` column, unknown/empty
   states bucketed as "Other").
4. **Given** any one of the three DB source files is missing, **Then**
   only the corresponding subview shows a "data not available" banner,
   and the remaining DB subviews render normally.

---

### Edge Cases

- Input directory does not exist, is not readable, or is a regular file
  rather than a directory → tool exits with a distinct non-zero code and
  a one-line error (no partial report written).
- Input directory exists but contains no pt-stalk files the tool
  recognises → tool exits with a distinct non-zero code identifying the
  directory as "not recognised as a pt-stalk output directory" (no
  report written).
- Input directory is recognised but every section-relevant source file is
  missing → report IS written and every section shows "data not
  available"; tool exits zero. (This distinguishes "empty but valid
  collection" from "not a pt-stalk dir at all.")
- A source file is truncated mid-sample (process killed during
  collection) → the parser returns usable data from the complete samples
  and records a diagnostic entry in the report's "Parser Diagnostics"
  panel naming the file, the byte offset, and the nature of the
  truncation.
- A source file contains bytes that are not valid UTF-8 → parser decodes
  lossily, the rendered text flags replacement characters where they
  occur, and a diagnostic is recorded.
- The pt-stalk collection directory contains more than one timestamped
  snapshot (e.g., `2026_04_21_16_51_41-*` and `2026_04_21_16_52_11-*` in
  the reference example) → snapshot-level collectors (`-variables`,
  `-innodbstatus1`) render once per snapshot as side-by-side or
  tab-switched subviews; time-series collectors (`-iostat`, `-top`,
  `-vmstat`, `-mysqladmin`, `-processlist`) concatenate samples across
  snapshots on a single shared time axis with a visible vertical marker
  at each snapshot boundary.
- Output path already exists → tool refuses to overwrite unless the
  engineer passes an explicit "overwrite" flag; otherwise exits non-zero
  with a clear message.
- Browser is opened offline and has JavaScript disabled → the three
  section headers, the Variables table, and all "data not available"
  banners are still visible and readable; graphs show a static
  "JavaScript required for charts" placeholder in place of the canvas.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The tool MUST accept exactly one input: a path to a
  directory expected to be a pt-stalk output collection.
- **FR-002**: The tool MUST accept an output path (flag-based) and MUST
  write exactly one self-contained HTML file to that path. No other
  output file (JSON side-car, CSV, archive, etc.) MUST be produced by
  the CLI in this feature.
- **FR-003**: The tool MUST NOT create, modify, move, rename, or delete
  any file or directory under the input directory.
- **FR-004**: The generated HTML file MUST be renderable in a modern
  browser with no network access of any kind: no CDN fetches, no remote
  fonts, no analytics beacons, no image URLs that resolve off-document.
- **FR-005**: The report MUST contain exactly three top-level sections,
  in this order: "OS Usage", "Variables", "Database Usage".
- **FR-006**: For a given input directory, two consecutive runs of the
  tool MUST produce byte-identical output files, except for a single
  "Report generated at" timestamp that the report explicitly labels as
  non-deterministic.
- **FR-007**: If any recognised source file is missing, empty, or
  unparseable, the corresponding subview MUST render a visible "data not
  available" banner naming the missing file; the tool MUST NOT abort
  report generation on such a condition.
- **FR-008**: If the tool cannot parse a portion of an otherwise-present
  source file, it MUST render whatever it was able to parse AND MUST
  record a diagnostic entry (file name, location, reason) in a visible
  "Parser Diagnostics" panel within the report.
- **FR-009**: The OS Usage section MUST include a per-device disk
  utilisation time-series derived from the `-iostat` file, with average
  queue size available per device on the same chart.
- **FR-010**: The OS Usage section MUST include a top-3 CPU process
  time-series derived from the `-top` file, where "top 3" is computed
  from aggregate CPU usage across the collection window.
- **FR-011**: The OS Usage section MUST include a resource-saturation
  time-series derived from the `-vmstat` file covering at minimum: CPU
  user+system, run queue length, blocked processes, free/cached memory,
  and swap in/out.
- **FR-012**: The Variables section MUST render a single table of all
  global variables and their values, drawn from the `-variables` file,
  sorted alphabetically by variable name, with no per-session rows.
- **FR-013**: The Variables table MUST support client-side filter-by-name
  search, operating entirely on the embedded document data, with no
  network calls.
- **FR-014**: The Database Usage section MUST include four distinct
  views derived from the `-innodbstatus1` file covering: number of
  semaphores, pending I/O, AHI activity, and HLL. The visual form of
  each view MUST match the shape of the underlying data (stable values
  vs. sampled values vs. cumulative counters).
- **FR-015**: The Database Usage section MUST include an interactive
  time-series chart derived from the `-mysqladmin` file that plots
  per-sample deltas (not absolute counter values) and allows the user to
  toggle which variables are plotted.
- **FR-016**: The interactive toggle UI for FR-015 MUST operate entirely
  on the embedded data with no network access at view-time.
- **FR-017**: The Database Usage section MUST include a time-series chart
  of thread-state counts derived from the `-processlist` file, bucketing
  unknown or empty states under "Other".
- **FR-018**: If the input directory contains more than one pt-stalk
  timestamped snapshot, snapshot-level collectors MUST render once per
  snapshot (side-by-side or tab-switched), and time-series collectors
  MUST concatenate samples on a single time axis with a visible boundary
  marker between snapshots.
- **FR-019**: If the input path does not exist, is unreadable, or is a
  non-directory, the tool MUST exit with a distinct non-zero exit code
  and a one-line error; no report file MUST be written in that case.
- **FR-020**: If the input directory is recognised but contains no
  source files the tool knows how to parse, the tool MUST exit with a
  distinct non-zero exit code identifying the directory as "not
  recognised as a pt-stalk output directory"; no report file MUST be
  written in that case.
- **FR-021**: For every source-file format the tool supports, at least
  one real-world fixture MUST be committed under `testdata/` and a
  round-trip golden test MUST assert that parsing and rendering produce
  a byte-identical reference output file. Adding a new supported file
  format without a fixture and golden file MUST fail the build.
- **FR-022**: The tool MUST make no network calls of any kind during
  normal operation.
- **FR-023**: The tool MUST report its version on request, including at
  minimum the release version string and the short git commit.
- **FR-024**: The tool MUST accept pt-stalk output produced by the
  current Percona Toolkit stable release and by the immediately
  preceding major version, and MUST commit a distinct fixture and
  golden file per supported version per collector (per Principle VIII).
  Output produced by older/forked `pt-stalk` variants MAY be parsed
  opportunistically but is not a supported guarantee; when a file's
  format cannot be recognised by any supported parser the tool MUST
  render the corresponding subview with an "unsupported pt-stalk
  version" banner naming the file, and MUST NOT abort.
- **FR-025**: The tool MUST support collections up to 1 GB total size,
  with any individual source file up to 200 MB, without exhausting
  memory or running longer than SC-007 allows. If the input directory
  exceeds these bounds, the tool MUST emit a distinct non-zero exit
  code with a one-line error identifying the bound exceeded (total
  size vs. per-file size) and MUST NOT write a partial report.
- **FR-026**: The tool MUST NOT transform, redact, mask, or obscure the
  values it reads from pt-stalk files. Hostnames, IP addresses, MySQL
  user names, query text in `-processlist` / `-innodbstatus1`, variable
  values, and all other content from the inputs MUST appear in the
  rendered report exactly as present in the source (after lossy UTF-8
  decoding, per the Edge Cases). The report MUST include a visible
  top-of-document banner stating that its content is derived verbatim
  from the input and that the viewer is responsible for any sharing-
  time redaction.
- **FR-027**: On a successful run the tool MUST write nothing to stdout
  and nothing to stderr except: (a) each parser diagnostic mirrored as a
  one-line warning at the moment it is recorded, and (b) per-file
  progress lines when the engineer passes `-v` / `--verbose`. Structural
  errors (input path missing, not a directory, size bounds exceeded,
  output path conflict without overwrite flag) MUST always be written
  to stderr as a single human-readable line, independent of verbosity.
  No other output surface is allowed.
- **FR-028**: The `-mysqladmin` parser MUST implement the delta-
  computation algorithm natively in Go inside `parse/mysqladmin.go`.
  Specifically: (a) only lines starting with `|` are data rows; (b)
  rows whose first column is the literal string `Variable_name` mark
  new-sample boundaries; (c) per-variable, per-sample
  `delta = current − previous` for counters; (d) the first sample's
  "delta" is defined as the raw initial tally and is excluded from
  aggregate statistics; (e) aggregate stats (total, min, max, avg)
  are computed over samples 2..N only, using `float64` for `avg` so
  small deltas do not truncate to zero; (f) non-counter variables are
  classified as gauges and stored as raw values, not deltas; (g)
  variables appearing in some samples but missing from others emit a
  `Diagnostic(Severity=Warning)` and store `NaN` in the missing slot.
  The algorithm MUST reproduce byte-for-byte the expected output
  shown in the worked example at the tail of
  `_references/pt-mext/pt-mext-improved.cpp` (lines 181–189) when given
  the matching input (lines 146–177 of the same file) as a fixture —
  this is the correctness anchor. Restricted to counter variables, as
  the C++ reference does not distinguish gauges.

### Key Entities

- **Collection**: A pt-stalk output directory. Attributes: root path,
  host name (when derivable from captured files), one or more Snapshots,
  set of present source-file paths, set of parser diagnostics.
- **Snapshot**: A timestamped set of pt-stalk files sharing a common
  `YYYY_MM_DD_HH_MM_SS-` prefix within a Collection. Attributes:
  timestamp, list of source-file paths, sequence position within the
  Collection.
- **SourceFile**: A specific pt-stalk collector output identified by its
  suffix (e.g., `-iostat`, `-variables`). Attributes: suffix type, path,
  snapshot parent, size, parse status (ok / partial / failed).
- **Sample**: One time-stamped observation within a time-series
  SourceFile (e.g., one iostat interval, one top batch). Attributes:
  timestamp, measurement map, snapshot parent, source-file parent.
- **MetricSeries**: An ordered set of Samples for one metric (e.g.,
  "sda %util"). Attributes: metric name, unit, device/subject,
  samples.
- **VariableEntry**: One global variable and its value from the
  `-variables` file. Attributes: name, value, scope (global only is
  retained).
- **ThreadState**: One processlist thread snapshot. Attributes:
  timestamp, state label, count.
- **Diagnostic**: A parser-level warning or error. Attributes: source
  file, location (byte offset or line), severity, human-readable
  message.
- **Report**: The top-level rendered document. Attributes: generated-at
  timestamp (the only explicitly non-deterministic field), tool version,
  Collection, list of rendered Sections.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A support engineer can go from receiving a pt-stalk
  directory to viewing the rendered report in a browser in under 60
  seconds, including tool invocation time, on an average laptop and a
  pt-stalk collection up to 100 MB.
- **SC-002**: The report renders fully — all graphs, all tables, all
  interactive controls — in a modern browser with the machine
  disconnected from any network.
- **SC-003**: Running the tool twice on the same input directory produces
  output files whose byte-difference is limited exclusively to the single
  "Report generated at" timestamp line.
- **SC-004**: When exactly one source file is missing from a pt-stalk
  directory, 100% of the remaining subviews still render their data, and
  the missing one shows a "data not available" banner naming the file.
- **SC-005**: Within 30 seconds of opening the report, a support
  engineer reviewing an incident can name (a) which disks showed
  pressure, (b) which processes consumed the most CPU, and (c) whether
  the system was swapping, without scrolling beyond the OS Usage
  section.
- **SC-006**: Every pt-stalk source-file suffix the tool advertises
  support for has a committed real-world fixture and a passing golden
  round-trip test; CI fails if any advertised format lacks either.
- **SC-007**: The tool's cold-start run on a 50 MB pt-stalk directory
  completes and writes the HTML file in under 10 seconds on a standard
  support engineer laptop.
- **SC-008**: The tool processes a 1 GB pt-stalk collection (the
  declared upper bound) to completion on a standard support engineer
  laptop without exceeding 2 GB of resident memory and without being
  killed by the OS out-of-memory handler.

## Assumptions

- Real-world pt-stalk collections in `_references/examples/example2/`
  are representative of the formats the tool must parse. Fixtures for
  golden tests will be drawn from these (or equivalent anonymised)
  samples. Supported pt-stalk formats are limited to the current
  Percona Toolkit stable release and the immediately preceding major
  version (see FR-024); other forks are best-effort only.
- The MVP supports only the seven source-file suffixes listed here
  (`-iostat`, `-top`, `-variables`, `-vmstat`, `-innodbstatus1`,
  `-mysqladmin`, `-processlist`). Other pt-stalk collectors (e.g.,
  `-meminfo`, `-disk-space`, `-slave-status`) are explicitly out of
  scope for this feature and will be added in subsequent features.
- The MEMORY element of "OS Usage — CPU/DISK/MEMORY" is served by the
  `-vmstat` resource-saturation chart (which includes free/cached memory
  and swap activity). A dedicated `-meminfo` view is deferred to a
  future feature; if memory context is judged insufficient from vmstat
  alone, that will be addressed by adding a `-meminfo` parser later, not
  by expanding this MVP.
- When an input directory contains multiple pt-stalk snapshots (typical
  when pt-stalk fires more than once), the MVP merges them within a
  single report rather than producing multiple reports (see FR-018).
- Browsers in use by the engineer audience are modern enough to support
  the embedded chart rendering (Chromium-, Firefox-, or Safari-class,
  released within the last 3 years). Legacy browser support is not a
  goal.
- "Interactive" in FR-015 means a chart whose variable selection is
  implemented in the embedded JavaScript bundled into the report; no
  server component ever runs at view-time.
- The report's "Report generated at" timestamp is the only
  non-deterministic field the rendered HTML is allowed to contain;
  anything else that could vary between runs (map iteration, goroutine
  ordering, float formatting, generated IDs) will be normalised at
  render boundaries.
- Mobile or tablet viewing of the report is not a goal for this feature;
  layout is targeted at desktop/laptop screens.
- The supported input-size envelope for this feature is collections up to
  1 GB total with any individual source file up to 200 MB (see FR-025).
  Parser implementations may buffer entire files within this envelope
  where streaming adds complexity without value; anything beyond is a
  future-feature concern.
- The tool performs no redaction of sensitive content (hostnames, IPs,
  MySQL user names, query text) from the rendered report (see FR-026).
  Redaction is the engineer's responsibility before the report leaves
  their machine. The rendered HTML carries an explicit banner stating
  this so viewers downstream of the engineer are not surprised.
- Machine-readable export (JSON, CSV, etc.) is out of scope for this
  feature. The `model/` package remains importable per Constitution
  Principle VI, but is not promoted as a stable third-party API and has
  no backwards-compatibility guarantee in v1; a JSON export surface, if
  needed later, will be its own feature with its own schema review.
- The pt-mext C++ reference implementation
  (`_references/pt-mext/pt-mext-improved.cpp`) is the source material
  we studied to derive the `-mysqladmin` delta-computation algorithm
  (see FR-028). It remains in the repository as historical reference
  but is **not a runtime or test-time dependency** — no C++ toolchain
  is required to build, test, or ship My-gather. The worked example
  in its tail comments is promoted to a committed test fixture under
  `testdata/pt-mext/` so the algorithm has a permanent correctness
  anchor that any future maintainer can verify. The C++ tool's known
  gaps (no counter/gauge classification, integer-only arithmetic, no
  handling of appearing/disappearing variables across samples) are
  deliberately addressed by the native Go implementation and
  documented in research R8.
