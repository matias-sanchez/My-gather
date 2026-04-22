---

description: "Task list for pt-stalk Report MVP (feature 001)"
---

# Tasks: pt-stalk Report MVP

**Input**: Design documents from `/specs/001-ptstalk-report-mvp/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md,
contracts/cli.md, contracts/packages.md, quickstart.md

**Tests**: Required. Constitution Principle VIII and spec FR-021
mandate a committed fixture and golden file per parser, plus a
determinism round-trip test per render (FR-006 / SC-003). Tests appear
inline with the phase they protect.

**Organization**: Tasks are grouped by user story. US1 is the MVP —
completing Phase 1 + Phase 2 + US1 alone delivers a working
self-contained HTML report (with every section empty, per spec
Edge Case #2 and FR-020). US2, US3, US4 each fill in one section.

## Format: `[ID] [P?] [Story?] Description with file path`

- **[P]** — task can run in parallel (different file, no dependency
  on an incomplete task in the same phase).
- **[Story]** — US1..US4; omitted for Setup, Foundational, and Polish.
- File paths follow the single-project Go layout declared in plan.md.

## Path Conventions

```text
cmd/my-gather/       CLI entry point
parse/               Per-collector parsers + discovery
model/               Typed representation
render/              HTML renderer + embedded assets
testdata/            Fixtures + golden files
tests/               Integration + cross-cutting tests
scripts/             Dev-only helpers
```

All tasks write paths relative to the repository root.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialisation and build/CI scaffolding.

- [x] T001 Create the directory layout declared in plan.md (`cmd/my-gather/`, `parse/`, `model/`, `render/templates/`, `render/assets/`, `testdata/golden/`, `testdata/pt-mext/`, `tests/`, `scripts/`, `.github/workflows/`). Empty `.gitkeep` files where a directory must exist but has no content yet.
- [x] T002 Initialize the Go module at `go.mod`: `module github.com/matias-sanchez/My-gather`, pin `go 1.24`, no non-stdlib direct dependencies (Principle X, XII).
- [x] T003 [P] Add `Makefile` with targets `build` (local), `test`, `release` (cross-compile CGO_ENABLED=0 for linux/{amd64,arm64} and darwin/{amd64,arm64} into `dist/` per plan.md Distribution section), and `clean`.
- [x] T004 [P] Add `.github/workflows/ci.yml` running `go vet ./...` and `go test ./...` on the pinned Go version across ubuntu-latest and macos-latest. Include a determinism check step and a cross-compile smoke step.
- [x] T005 [P] Add a minimal `README.md` at the repo root covering install, quickstart run, and a pointer to `specs/001-ptstalk-report-mvp/` for deeper context. Content based on `quickstart.md`.
- [x] T006 [P] Commit `scripts/anonymise-fixtures.sh` (bash + sed/awk only, per research R6) with the anonymisation rules listed in R6; script is idempotent and deterministic.

**Checkpoint**: Repository builds (`go build ./...` succeeds with empty packages), CI runs, cross-compile targets produce artifacts.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Types, errors, skeletons, and the CI guards that every user story depends on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [x] T007 [P] Define the `model` package types in `model/model.go`: `Collection`, `Snapshot`, `SourceFile`, `Sample`, `MetricSeries`, `VariableEntry`, `Diagnostic`, `Severity` (enum), `Suffix` (enum + `KnownSuffixes` slice + all seven constants), `FormatVersion` (enum), `ParseStatus` (enum). Every exported identifier has a godoc comment (Principle VI). (`ThreadStateSample` is defined in T008 next to its `ProcesslistData` container.)
- [x] T008 [P] Define the per-collector typed payloads in `model/model.go` (continuing T007): `IostatData`, `DeviceSeries`, `TopData`, `ProcessSample`, `ProcessSeries`, `VariablesData`, `VmstatData`, `InnodbStatusData`, `AHIActivity`, `MysqladminData` (including `SnapshotBoundaries []int`), `ProcesslistData`, `ThreadStateSample`. Match `data-model.md` exactly.
- [x] T009 [P] Define the section / render types in `model/sections.go`: `OSSection`, `VariablesSection`, `DBSection`, `SnapshotVariables`, `SnapshotInnoDB`, `Report`, `NavEntry`. `Report.Navigation []NavEntry` and `Report.ReportID string` populated per FR-031 / FR-032.
- [x] T010 [P] Define `parse/errors.go` with sentinels `ErrNotAPtStalkDir`, and typed `SizeError` (with `SizeErrorKind`, `SizeErrorTotal`, `SizeErrorFile`), `PathError`, `ParseError`. Implement `Error()` / `Unwrap()` per contracts/packages.md.
- [x] T011 [P] Define `parse/version.go` with `DetectFormat(peekedBytes []byte, suffix model.Suffix) model.FormatVersion` helper and documentation referencing research R2.
- [x] T012 [P] Define `parse/parse.go` skeleton with `DefaultMaxCollectionBytes`, `DefaultMaxFileBytes`, `DiscoverOptions`, `DiagnosticSink`, and the `Discover(ctx, rootDir, opts) (*model.Collection, error)` signature. Initial body: snapshot-regex discovery, size-bound preflight, pt-summary.out fallback — no per-collector parsing yet (that lands per-story).
- [x] T013 [P] Define `render/deterministic.go` with `sortedKeys[K, V]`, `sortedEntries[K, V]`, `formatFloat(v, precision)`, ordinal ID counter, and UTC timestamp formatter. Unit tests in `render/deterministic_test.go` covering every helper's determinism.
- [x] T014 [P] Vendor the uPlot asset (research R1): commit `render/assets/chart.min.js`, `render/assets/chart.min.css`, `render/assets/NOTICE` (upstream MIT license + copyright), `render/assets/app.js` skeleton (placeholder for collapse persistence + mysqladmin toggle), `render/assets/app.css` skeleton (sticky nav rail styles). Add `render/assets.go` with `//go:embed` directives for each.
- [x] T015 [P] Define `render/render.go` skeleton with `Render(w io.Writer, c *model.Collection, opts RenderOptions) error` signature, report-envelope construction (including `Navigation` + `ReportID`), and template dispatch; initial body renders every section as empty / "data not available". Also define `RenderOptions`.
- [x] T016 [P] Create the HTML template tree: `render/templates/report.html.tmpl` (top-level layout, top-of-doc verbatim banner per FR-026, nav rail per FR-031, `<details>` wrappers per FR-032), `render/templates/os.html.tmpl`, `render/templates/variables.html.tmpl`, `render/templates/db.html.tmpl`. Templates use only sorted data; no raw map iteration (Principle IV).
- [x] T017 [P] CLI skeleton in `cmd/my-gather/main.go`: flag parsing per `contracts/cli.md` (including `-o`/`--out`, `--overwrite`, `-v`/`--verbose`, `--version`, `-h`/`--help`), positional arg handling, output-path-inside-input guard (FR-029, exit code 7), exit-code dispatch (0, 2, 3, 4, 5, 6, 7, 70), `--version` output format.
- [x] T018 [P] Forbidden-import linter test at `tests/lint/imports_test.go` (research R7). Walks the import graph of `cmd/`, `parse/`, `model/`, `render/`; fails if any of `net/http`, `net`, `net/rpc`, `net/smtp`, `crypto/tls` appear. Stdlib-only via `go/parser` + `go/ast`.
- [ ] T019 Commit anonymised fixtures under `testdata/example1/` and `testdata/example2/` produced by `scripts/anonymise-fixtures.sh` run against `_references/examples/example1/` and `_references/examples/example2/`. Only the seven source-file suffixes this feature supports need be committed (plus `pt-summary.out` / `pt-mysql-summary.out` for discovery tests).
- [x] T020 Commit the pt-mext golden fixture under `testdata/pt-mext/`: `input.txt` is a verbatim copy of `_references/pt-mext/pt-mext-improved.cpp` comment lines 146–177; `expected.txt` is a verbatim copy of lines 181–189 with a `testdata/pt-mext/README.md` describing provenance (research R8).
- [x] T021 Coverage guard test at `tests/coverage/testdata_coverage_test.go`: fails the build if any `model.Suffix` constant lacks a fixture under `testdata/example1/` or `testdata/example2/`. Enforces the fixture-coverage half of Constitution Principle VIII / spec FR-021 / SC-006; golden-coverage enforcement lands in T080 once per-collector golden outputs are generated (`go test ./parse/... -update` today; widen the scope as more packages adopt goldens).
- [x] T022 [P] godoc-coverage test at `tests/coverage/godoc_coverage_test.go`: walks AST of `model`, `parse`, `render` and asserts every exported identifier has a non-empty `//` doc comment (Constitution Principle VI).

**Checkpoint**: `go test ./...` passes (lint and coverage guards pass over empty packages); CLI builds; an invocation against any test fixture prints exactly one HTML file whose sections all read "data not available".

---

## Phase 3: User Story 1 - First-look triage report end-to-end (Priority: P1) 🎯 MVP

**Goal**: Prove the parse → model → render → embed-assets → byte-deterministic HTML pipeline end-to-end against a real pt-stalk directory. Every section shows "data not available" because no per-collector parsers are wired yet; this is intentional. The report, navigation, collapse behaviour, and determinism guarantees are all demonstrable against the empty report.

**Independent Test**: Run `./bin/my-gather -o /tmp/report.html testdata/example2`. The command exits 0, `/tmp/report.html` exists, opening it in an offline browser shows the three section headers (OS Usage, Variables, Database Usage), a nav index linking to each, all three collapsed/expanded correctly, the verbatim-passthrough banner at the top, and each section body reading "data not available". Running twice in a row produces byte-identical HTML except the `Report generated at` timestamp.

### Tests for User Story 1 ⚠️ (Write first; implementation makes them pass)

- [x] T023 [P] [US1] `render/render_test.go::TestRenderEmptyReport`: render a Collection with zero Snapshots (or with Snapshots whose `SourceFiles` is empty); assert every section emits its "data not available" banner.
- [x] T024 [US1] `render/render_test.go::TestDeterminism`: render twice against the same Collection with the same `RenderOptions.GeneratedAt`; assert `bytes.Equal` on outputs (spec SC-003). **Sequential with T023** (same file).
- [x] T025 [US1] `render/render_test.go::TestGeneratedAtIsOnlyDiff`: render twice with different `GeneratedAt` values; assert the diff consists of exactly one line (the `Report generated at` line) (spec FR-006). **Sequential with T023/T024** (same file).
- [x] T026 [P] [US1] `parse/parse_test.go::TestNotAPtStalkDir`: run `Discover` against an empty tempdir; assert `errors.Is(err, parse.ErrNotAPtStalkDir)`.
- [x] T027 [US1] `parse/parse_test.go::TestEmptyButValid`: run `Discover` against a dir containing one timestamped placeholder file with an unsupported suffix (e.g., `-hostname` only) OR a bare `pt-summary.out`; assert `err == nil` and `Collection.Snapshots` is non-empty and every `SourceFile` entry is absent (per FR-020). **Sequential with T026** (same file).
- [x] T028 [US1] `parse/parse_test.go::TestSizeBoundTotalExceeded`: run `Discover` against a fixture whose total size exceeds `DiscoverOptions.MaxCollectionBytes`; assert `errors.As(err, &parse.SizeError{})` with `Kind == SizeErrorTotal`. **Sequential with T026/T027** (same file).
- [x] T029 [US1] `parse/parse_test.go::TestSizeBoundFileExceeded`: same as T028 but with a single oversized file; assert `Kind == SizeErrorFile`. **Sequential with T026-T028** (same file).
- [ ] T030 [P] [US1] `cmd/my-gather/main_test.go::TestOutputInsideInputRejected`: invoke the CLI with `-o <somewhere-inside-input-dir>`; assert exit code 7 and the report file was NOT written (spec FR-029).
- [ ] T031 [US1] `cmd/my-gather/main_test.go::TestOverwriteGuard`: invoke twice without `--overwrite`; assert exit code 6 on the second call. **Sequential with T030** (same file).
- [ ] T032 [P] [US1] `tests/integration/e2e_test.go::TestE2EExample2`: run the compiled binary against `testdata/example2/`; assert exit 0, output file exists, contains the three section headers, and contains the verbatim banner text.
- [ ] T033 [P] [US1] `tests/integration/offline_test.go::TestOfflineRender`: open the rendered HTML with a headless-browser–free check — walk the raw HTML and assert no `http://`, `https://`, or `//` (protocol-relative) URLs appear in `<script src>`, `<link href>`, or `<img src>` attributes; assert all `<script>` and `<style>` tags are inline or embed-sourced (spec FR-004, SC-002).
- [ ] T034 [P] [US1] `render/app_test.go::TestCollapsePersistenceRoundTrip`: run the collapse-persistence logic from `render/assets/app.js` under a lightweight JS test harness (or port to a Go-testable form); stub `localStorage`; assert save→load round-trip keyed by `Report.ReportID` (spec SC-010).
- [ ] T035 [P] [US1] `render/nav_test.go::TestNavCoverage`: for a rendered report, assert every `NavEntry.ID` in `Report.Navigation` has a matching `id="..."` target in the HTML; assert the count of nav entries equals the count of unique anchor targets (spec SC-009).

### Implementation for User Story 1

- [x] T036 [US1] Implement `parse.Discover` body in `parse/parse.go`: size preflight (T028/T029 green), timestamped-file regex scan (research R5), `pt-summary.out` fallback, Snapshot grouping, absent-SourceFile handling, `ErrNotAPtStalkDir` only in the true "no timestamps AND no summary" case (per FR-020 remediation).
- [x] T037 [US1] Implement `render.Render` body in `render/render.go`: build `Report`, compute `Navigation` as a deterministic flat list derived from section iteration order (OS → Variables → DB → Parser Diagnostics), compute `ReportID` as SHA-256 of canonicalised Collection (excluding `RootPath` and `GeneratedAt`) truncated to 12 hex chars (research R9 with F21 correction), dispatch to templates, pipe through deterministic helpers.
- [x] T038 [US1] Flesh out `render/templates/report.html.tmpl`: HTML5 skeleton, top-of-document verbatim-passthrough banner (FR-026 — fixed wording: "This report is derived verbatim from a pt-stalk collection. Hostnames, IP addresses, query text, and MySQL variables appear as captured. Responsible sharing is the reader's responsibility."), sticky nav rail on the left, header with tool version + GeneratedAt, main content column with three `<details>` section wrappers, footer with Parser Diagnostics `<details>` (default closed per FR-032 / Principle XI).
- [x] T039 [US1] Implement `render/assets/app.js` collapse-persistence module: read `Report.ReportID` from a JSON metadata block embedded into the HTML, key `localStorage` entries as `mygather:<reportID>:collapse:<sectionID>`, restore state on `DOMContentLoaded`, save on `<details>` `toggle` events. Degrade silently if `localStorage` is unavailable (F22: private browsing / disabled storage).
- [x] T040 [US1] Implement `render/assets/app.css` sticky nav rail styles: CSS Grid layout with rail on the left, main content on the right, `position: sticky` on the nav element, `@media print { details { open: true } }` print rule (F23). `@media (max-width: 800px)` fallback to top-horizontal nav.
- [x] T041 [US1] Wire `cmd/my-gather/main.go` end-to-end: resolve paths (with `filepath.EvalSymlinks`), run the output-inside-input guard (FR-029), call `parse.Discover`, map errors to exit codes per `contracts/cli.md` exit-code table, call `render.Render`, write atomically via `os.CreateTemp` + `Sync` + `os.Rename` in the output's parent directory, mirror `Diagnostic` warnings/errors to stderr per FR-027.
- [x] T042 [US1] Implement `-v` / `--verbose` stderr progress lines (FR-027): emit the bracketed-tag lines defined as the minimum contract in `contracts/cli.md` (`[parse]` input path, `[parse]` snapshot/size summary, `[render]` output path, `[done]` bytes-written; elapsed-time suffix optional); also ensure `SeverityInfo` diagnostics do NOT mirror to stderr (F13 resolution — only `Warning` and `Error` do).

**Checkpoint**: Running `./bin/my-gather -o /tmp/report.html testdata/example2` produces a working, self-contained, deterministic HTML file with navigation, collapse, banner, and three empty sections. Tests T023–T029 pass today; T030–T035 are still pending test authorship and will go green once written.

---

## Phase 4: User Story 2 - OS Usage section (Priority: P2)

**Goal**: Fill in the OS Usage section with three subviews — per-device disk utilisation, top-3 CPU processes, and vmstat resource saturation — from parsed real data.

**Independent Test**: Run against `testdata/example2/`; assert the OS Usage section renders three non-empty chart subviews with correct device names, PID labels, and time-series data ranges.

### Tests for User Story 2

- [x] T043 [P] [US2] `parse/iostat_test.go::TestIostatGolden`: parse `testdata/example2/2026_04_21_16_51_41-iostat` and compare resulting `IostatData` JSON-serialised against `testdata/golden/iostat.example2.2026_04_21_16_51_41.json`. Regenerate with `go test ./parse/... -update` (the `-update` flag is only registered in packages that import `tests/goldens`; see the package godoc for the scoping rule). Also asserts alphabetical device ordering, matched `Utilization`/`AvgQueueSize` sample counts per device, and single-file `SnapshotBoundaries ≤ 1`.
- [x] T044 [P] [US2] `parse/top_test.go::TestTopGolden`: parse `-top` fixture; compare against golden. Also asserts `Top3ByAverage` is ranked with **average CPUPercent normalised over total batch count** (absent-in-batch counts as 0 — F7 resolution, aligning with spec FR-010 "aggregate"). The (higher PID first) tiebreaker is pinned by a deterministic synthetic-tie assertion inside the test (floating-point averages from a real fixture rarely compare equal, so a fixture-only check would leave the tiebreaker branch effectively unexercised). Golden at `testdata/golden/top.example2.2026_04_21_16_51_41.json`; regenerate with `go test ./parse/... -update`.
- [x] T045 [P] [US2] `parse/vmstat_test.go::TestVmstatGolden`: parse `-vmstat` fixture; compare against golden. Asserts the declared `Series` order is present (runqueue, blocked, free_kb, buff_kb, cache_kb, swap_in, swap_out, io_in, io_out, cpu_user, cpu_sys, cpu_idle, cpu_iowait) and that every non-empty Series shares the same sample count. Golden at `testdata/golden/vmstat.example2.2026_04_21_16_51_41.json`.
- [x] T046 [P] [US2] `render/os_test.go::TestOSGoldenHTML`: render a Collection from `parse.Discover(testdata/example2/)` filtered to only the OS parsers (iostat, top, vmstat); extract the `<details id="sec-os">` block via the shared `extractDetailsSection` helper and snapshot-compare against `testdata/golden/os.example2.html`. Regenerate with `go test ./render/... -update`.
- [x] T047 [US2] `parse/iostat_test.go::TestIostatPartialRecovery`: truncate the fixture at 50% mid-sample; asserts `parseIostat` returns non-nil data AND at least one `Diagnostic(Severity≥Warning)` whose `Location` field is non-empty (Principle III, FR-008). **Sequential with T043** (same file).

### Implementation for User Story 2

- [x] T048 [P] [US2] Implement `parse/iostat.go` — per-device utilisation + avgqu_sz series; handle the two supported pt-stalk format variants (FR-024); emit diagnostics for malformed rows. Covered by T043.
- [x] T049 [P] [US2] Implement `parse/top.go` — parse repeated `top` batch snapshots; build per-process samples; compute `Top3ByAverage` using **aggregate CPU normalised by total sample count** (absent = 0) so sustained load ranks above short spikes (F7 resolution). Covered by T044.
- [x] T050 [P] [US2] Implement `parse/vmstat.go` — 13-series fixed-order time-series extraction across the two supported format variants. Missing columns are silently zero-filled (per-row map lookup returns the float64 zero-value); the parser does NOT emit a Diagnostic for a missing column today. Covered by T045.
- [x] T051 [US2] Wire OS parsers into `parse.Discover` so each Snapshot's `SourceFiles[SuffixIostat|SuffixTop|SuffixVmstat].Parsed` is populated with typed data.
- [x] T052 [US2] Implement `render/templates/os.html.tmpl` — three subview blocks, each with its own `<details>` wrapper, each linked from the nav rail (T037 Navigation population is extended to list "Disk utilization", "Top CPU processes", "vmstat saturation" as Level=2 NavEntries under "OS Usage").
- [x] T053 [US2] Wire uPlot chart instantiation in `render/assets/app.js` for the three OS subviews: read JSON data payload from `<script type="application/json">` blocks, instantiate uPlot with deterministic options (fixed colours, no animations), attach to the correct `<canvas>` elements.
- [x] T054 [US2] Implement multi-snapshot concatenation with boundary markers for every time-series collector (`-iostat`, `-top`, `-vmstat`, `-mysqladmin`, `-processlist` per FR-018 / FR-030): `render.concatIostat` / `concatTop` / `concatVmstat` / `concatMysqladmin` / `concatProcesslist` fold per-Snapshot payloads onto a single time axis and record sample-index `SnapshotBoundaries` on each merged `*Data` struct. Chart payloads (`iostatChartPayload`, `topChartPayload`, `vmstatChartPayload`, `mysqladminChartPayload`, `processlistChartPayload`) export `snapshotBoundaries` into the embedded JSON; `render/assets/app.js::drawSnapshotBoundaries` is a uPlot `drawAxes` hook that paints a dashed vertical line at each boundary timestamp. Counter-vs-gauge semantics respected at the mysqladmin boundary: the first post-boundary slot of every counter variable is `NaN` (`null` in JSON) per FR-030; gauges concat their raw values. Covered by `render/render_test.go::TestMultiSnapshotMergerRecordsBoundaries`, `TestChartPayloadExportsSnapshotBoundaries`, and `TestMysqladminBoundaryInsertsNaN`.
- [x] T055 [US2] "Data not available" banner wiring for the three OS subviews (FR-007) when any of `-iostat` / `-top` / `-vmstat` is absent from the collection.

**Checkpoint**: US1 + US2 produce a report whose OS Usage section renders correctly against `testdata/example2/`. Every test T023–T047 passes.

---

## Phase 5: User Story 3 - Variables section (Priority: P3)

**Goal**: Fill in the Variables section with a searchable, alphabetically-sorted table of every global variable captured.

**Independent Test**: Run against `testdata/example2/`; assert the Variables section shows all global variables from the `-variables` fixture in one table per snapshot, with working client-side search.

### Tests for User Story 3

- [x] T056 [P] [US3] `parse/variables_test.go::TestVariablesGolden`: parse `-variables` fixture; compare `VariablesData` against golden; assert alphabetical sort, no duplicates (first-global-wins dedup per FR-012). Golden at `testdata/golden/variables.example2.2026_04_21_16_51_41.json`; regenerate with `go test ./parse/... -update` (the `-update` flag is only registered in packages that import `tests/goldens`; see the package godoc for the scoping rule).
- [x] T057 [P] [US3] `render/variables_test.go::TestVariablesGoldenHTML`: render Variables section from the filtered example2 Collection; snapshot-compare against `testdata/golden/variables.example2.html`. Regenerate with `go test ./render/... -update`.
- [x] T058 [P] [US3] `render/variables_test.go::TestVariablesSearchMarkup`: assert the rendered Variables subview carries `<input type="search">` with BOTH `class="variables-search"` AND `data-snapshot="<id>"` on the same tag, matching the shipped JS selector in `render/assets/app.js::initVariablesSearch` (`input.variables-search[data-snapshot]`); assert every data row carries a `data-variable-name` attribute for the client-side filter (FR-013). Dropping either attribute — or keeping only an `id` — would break the filter in the browser, so the test requires the full pair rather than accepting any hook.

### Implementation for User Story 3

- [x] T059 [P] [US3] Implement `parse/variables.go` — pipe-delimited table parser; alphabetical sort; dedup by name; emit Diagnostic for any duplicate-name entries it dropped.
- [x] T060 [US3] Wire `-variables` into `parse.Discover` (similar to OS parsers in T051).
- [x] T061 [US3] Implement `render/templates/variables.html.tmpl` — one `<details>` per snapshot (per FR-018) containing one `<table>` with the search input and all variable rows. Nav rail gets one Level=2 NavEntry per snapshot under "Variables".
- [x] T062 [US3] Implement the client-side search filter in `render/assets/app.js` — input event listener that toggles `hidden` on `<tr>` rows whose `data-variable-name` doesn't substring-match the input value. No network, no libraries (FR-013, Principle X).

**Checkpoint**: US1 + US2 + US3 produce a report with a working Variables table. Every test through T058 passes.

---

## Phase 6: User Story 4 - Database Usage section (Priority: P4)

**Goal**: Fill in the Database Usage section with four subviews — InnoDB internals, interactive mysqladmin deltas, processlist thread states, and the snapshot-boundary-aware merger.

**Independent Test**: Run against `testdata/example2/`; assert every DB subview is populated with data from its respective fixture; assert the mysqladmin toggle UI lets the user enable/disable variable series; assert per-snapshot InnoDB scalars render side-by-side.

### Tests for User Story 4

- [x] T063 [P] [US4] `parse/innodbstatus_test.go::TestInnoDBStatusGolden`: parse `-innodbstatus1` fixture; extract SemaphoreCount, PendingReads/Writes, AHI activity, HLL; compare against golden. Asserts non-negativity on every scalar counter / rate. Golden at `testdata/golden/innodbstatus.example2.2026_04_21_16_51_41.json`.
- [x] T064 [P] [US4] `parse/mysqladmin_test.go::TestMysqladminGolden`: parse `-mysqladmin` fixture; compare against golden. Asserts `VariableNames` strictly sorted + deduplicated, `IsCounter` key set matches `VariableNames`, every `Deltas[v]` slice length == `SampleCount` == `len(Timestamps)`, single-file `SnapshotBoundaries == [0]`, and a classifier spot-check on a handful of well-known counters (Com_*, Bytes_*, Handler_*) and gauges (Threads_running, Uptime, Open_tables). `Deltas` is routed through `goldens.ScrubFloats` before marshalling so `math.NaN()` at boundary / drift slots encodes as JSON `null`. Golden at `testdata/golden/mysqladmin.example2.2026_04_21_16_51_41.json`. Multi-snapshot boundary behaviour is covered by T066 below.
- [x] T065 [US4] `parse/mysqladmin_test.go::TestPtMextFixture`: parse `testdata/pt-mext/input.txt` with the Go mysqladmin parser; format aggregates (total/min/max/avg) for each counter; assert structural equivalence against `testdata/pt-mext/expected.txt` (F10 resolution — structural comparison, not byte-for-byte, so whitespace normalisation is allowed). **Sequential with T064** (same file).
- [x] T066 [US4] `parse/mysqladmin_test.go::TestSnapshotBoundaryReset`: uses the internal `newMysqladminParser` + `parseOneFile` + `ResetForNewSnapshot` + `finish` lifecycle to parse the two committed `-mysqladmin` fixtures (16:51:41 and 16:52:11) as consecutive snapshots and assert boundary handling. Confirms `SnapshotBoundaries` includes the exact column index at which the second file began, every non-zero boundary slot is `math.NaN()` for counters and finite for gauges (FR-030), and exactly one `SeverityInfo` diagnostic is emitted per non-zero boundary. The render-layer concat code does not call this parser API — it merges finished `*MysqladminData` values via `render.concatMysqladmin`; the multi-snapshot wall-clock timeline is T054's responsibility. **Sequential with T064/T065** (same file).
- [x] T067 [US4] `parse/mysqladmin_test.go::TestVariableDrift`: builds two in-memory pt-stalk `-mysqladmin` blocks (first has three variables, second drops one — `Com_missing`, named to match the `Com_` counter prefix so the test exercises the FR-028 / R8.C counter-drift contract) so the drift condition is deterministic and independent of any fixture refresh. Asserts `IsCounter["Com_missing"] == true`, drift slots are `math.NaN()`, and exactly one `SeverityWarning` diagnostic mentions the missing variable (research R8 improvement C). **Sequential with T064-T066** (same file).
- [x] T068 [P] [US4] `parse/processlist_test.go::TestProcesslistGolden`: parse `-processlist` fixture; compare against golden; assert `Other`-bucketing for unknown/empty states (FR-017). Golden at `testdata/golden/processlist.example2.2026_04_21_16_51_41.json`; also asserts `States` alphabetical with `Other` last and `SnapshotBoundaries == nil` for a direct parse call (the parser must not set that field — render owns it via `concatProcesslist`).
- [x] T069 [P] [US4] `render/db_test.go::TestDBGoldenHTML`: render DB section from the filtered example2 Collection (innodbstatus + mysqladmin + processlist wired); snapshot-compare against `testdata/golden/db.example2.html`. Regenerate with `go test ./render/... -update`.
- [x] T070 [P] [US4] `render/db_test.go::TestMysqladminToggleMarkup`: assert the sec-db block carries the `id="chart-mysqladmin"` + `data-mysqladmin-host` anchors the JS widget latches onto (the shipped toggle is built client-side from the JSON payload, not server-rendered as per-variable DOM markup). Assert every `MysqladminData.VariableNames` entry appears at least once in the embedded `<script id="report-data">` JSON payload so the toggle has a complete list to render; assert the payload is valid JSON; and assert no interactive element inside sec-db carries a network URL (`src=` / `href=` to http(s), aligning FR-015 with Principle IX / FR-022).

### Implementation for User Story 4

- [x] T071 [P] [US4] Implement `parse/innodbstatus.go` — section-aware free-text parser for `-innodbstatus1`; extract the four scalar views; diagnose unexpected-section occurrences.
- [x] T072 [P] [US4] Implement `parse/mysqladmin.go` — the pt-mext port. Functions: `parseMysqladmin(r io.Reader, format FormatVersion) (*model.MysqladminData, []model.Diagnostic)`. Line-by-line state machine: skip non-`|` lines; `Variable_name` row → sample boundary → increment `col`; data row → `delta = value - previous[name]`, `previous[name] = value`. Floating-point aggregates (research R8 improvement B). Counter-vs-gauge classification via a hardcoded allowlist keyed by `FormatVersion`. Snapshot-boundary reset (FR-030 / R8 improvement D) applied when the caller passes concatenated input — implemented as an explicit `ResetPrevious()` call at boundaries by the Discover glue, not guessed from the stream.
- [x] T073 [P] [US4] Implement `parse/processlist.go` — pipe-delimited multi-sample parser; thread-state bucketing; `Other` fallback for empty/unknown states.
- [x] T074 [US4] Wire all three DB parsers into `parse.Discover` (one parse call per Snapshot per collector file). Cross-Snapshot merging for `-mysqladmin` and `-processlist` is delegated to the render-layer concatenators `render.concatMysqladmin` / `concatProcesslist` (see T054): FR-030's counter-boundary NaN insertion happens there, not inside the parser, because Discover parses one file at a time. `parse/mysqladmin.go::ResetForNewSnapshot` remains available for consumers that feed the parser a concatenated stream of multiple files.
- [x] T075 [US4] Implement `render/templates/db.html.tmpl` — four subview blocks: InnoDB side-by-side scalar callouts per snapshot; mysqladmin interactive chart with a keyboard-accessible toggle UI (current implementation: custom dropdown + category chips per FR-034; any UI satisfying FR-015 is acceptable); processlist thread-state stacked chart. Nav rail extended with four Level=2 NavEntries under "Database Usage". (Vertical snapshot-boundary markers across all time-series charts are required by FR-018 / FR-030 but are tracked separately by T054; T075 does not block on them.)
- [x] T076 [US4] Implement the mysqladmin interactive toggle in `render/assets/app.js` — multi-select change event toggles chart series visibility; persist selected-variables state via the same `Report.ReportID`-keyed `localStorage` scheme as collapse state (research R9).
- [x] T077 [US4] Implement uPlot chart wiring for DB subviews in `render/assets/app.js` (extension of T053): processlist stacked chart, mysqladmin multi-series chart with boundary markers.

**Checkpoint**: US1 + US2 + US3 + US4 complete. Every FR has an implementing task and a verifying test. A run against `testdata/example2/` produces a full-featured report.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Coverage guards, performance verification, CI tightening, documentation, remaining MEDIUM-severity analyze findings.

- [ ] T078 [P] Performance test at `tests/perf/perf_test.go`: cold-start run against a 50 MB synthetic or anonymised fixture; assert wall time < 10 s on the CI runner (spec SC-007). Also assert end-to-end run on a 100 MB fixture completes under 60 s (spec SC-001).
- [ ] T079 [P] Memory test at `tests/perf/memory_test.go`: run against a 1 GB synthetic fixture; assert peak RSS < 2 GB via `runtime.MemStats` + `runtime.GC()` snapshots (spec SC-008).
- [ ] T080 [P] Coverage metatest at `tests/coverage/suffix_golden_test.go`: for every `model.Suffix` constant in `KnownSuffixes`, assert at least one matching fixture file exists under `testdata/example1/` or `testdata/example2/` AND at least one golden file exists under `testdata/golden/` — strengthens T021 (spec SC-006).
- [ ] T081 [P] Accessibility smoke test at `tests/integration/a11y_test.go`: render a report; assert the nav index is a `<nav>` element containing `<a href="#...">` links; assert every `<details>` has a `<summary>` child; assert no interactive element relies on a custom role without an `aria-label`. Matches F25 remediation scope.
- [ ] T082 [P] Determinism stress test at `render/determinism_stress_test.go`: render 10 times in a loop against `testdata/example2/` with the same `GeneratedAt`; assert all outputs are byte-identical (SC-003 hardening).
- [x] T083 [P] Cross-compile matrix verification in CI: extend `.github/workflows/ci.yml` to build and smoke-run `./bin/my-gather --version` for each of linux/{amd64,arm64} and darwin/{amd64,arm64} (Principle I, Technical Context).
- [x] T084 Flesh out `README.md` with the content from `quickstart.md` plus: project status badge, the twelve constitution principles as a short bullet list, pointer to `specs/001-ptstalk-report-mvp/`, build/install/run/contribute sections.
- [ ] T085 Run the `quickstart.md` flow end-to-end manually and record any friction in a new `specs/001-ptstalk-report-mvp/retrospective.md`; feed fixes back as follow-up tasks if needed.
- [ ] T086 Confirm `--out <nonexistent-dir>/...` behaviour in `cmd/my-gather/main_test.go::TestOutputParentMissing` — assert clean error and exit code 3 (or a new code if the team prefers) rather than auto-mkdir (F8 resolution).
- [x] T087 Final consistency sweep across `spec.md`, `data-model.md`, `contracts/packages.md`, and `contracts/cli.md`: confirm every `-<suffix>` name and `Suffix<Name>` constant matches 1:1, no dangling `StateSample` tokens exist (only `ThreadStateSample` is defined), and every path reference uses `_references/examples/` (leading underscore) consistent with constitution v1.0.1. No automatic edits — read-only check that fails the checklist if any drift is detected.
- [ ] T088 [P] `cmd/my-gather/main_test.go::TestOutputFileSingletonNoSideCar` (F27 — FR-002 gap): after a successful run, assert the output path's parent directory contains exactly one new file matching the `-o` path (no `.tmp`, no `.bak`, no `.part`, no lock files). Walk the parent dir with `os.ReadDir` before and after; diff.
- [ ] T089 [P] `parse/partial_recovery_test.go::TestPartialRecoveryAllParsers` (F28 — FR-008 gap): parametrised test that iterates over the six collectors lacking a partial-recovery test (top, vmstat, variables, innodbstatus, mysqladmin, processlist); for each, copy its fixture to a tempfile, truncate to 50% of its original length, parse, assert result status is `ParsePartial` with ≥1 `Diagnostic(Severity=Warning)` whose `Location` field is non-empty. Closes Principle III coverage gap.
- [ ] T090 [P] `parse/version_test.go::TestDetectFormat` (F29 — FR-024 gap): feed `parse.DetectFormat` a representative V1 header byte sequence and a V2 header byte sequence for each of the seven supported suffixes (14 subtests); assert the correct `model.FormatVersion` enum is returned. Exercises per-file version detection (research R2) that the golden tests don't.
- [ ] T091 `cmd/my-gather/main_test.go::TestStderrSilentOnSuccess` (F30 — FR-027 gap, 1 of 3): successful run against a clean fixture; assert stderr is empty. **Sequential with T030/T031/T088** (same file).
- [ ] T092 `cmd/my-gather/main_test.go::TestStderrWarningMirrored` (F30 — FR-027 gap, 2 of 3): run against a fixture with a deliberately truncated source file; assert stderr contains a line matching `^\[warning\]\s+\S+:\s+.+$` per the contracts/cli.md format, with the warning count equal to the emitted `Diagnostic(Severity=Warning)` count. **Sequential with T091** (same file).
- [ ] T093 `cmd/my-gather/main_test.go::TestStderrVerboseProgress` (F30 — FR-027 gap, 3 of 3): run with `-v`; assert stderr contains `[parse] …`, `[render] writing …`, and `[done] … bytes written in …s` lines in that order. Assert `SeverityInfo` diagnostics do NOT appear on stderr (F13 behavior). **Sequential with T092** (same file).
- [ ] T094 [P] `tests/integration/degraded_test.go::TestOneMissingCollector` (F31 — SC-004 gap): parametrised test iterating over the seven supported suffixes. For each: copy `testdata/example2/` to a tempdir, delete the one matching file from both snapshots, run the CLI, assert exit 0 and the output HTML contains the other six sections' data AND a "data not available" banner naming the missing file. Seven subtests, one task.
- [ ] T095 `cmd/my-gather/main_test.go::TestVersionOutput` (FR-023 coverage): invoke `--version` with `ldflags` injecting a fixed semver + commit + build date; assert the five-line format from `contracts/cli.md`. **Sequential with T030/T031/T088/T091/T092/T093** (same file).
- [x] T096 [P] `render/os_test.go::TestOSSubviewAnchors` (SC-005 coverage): render a report with all three OS parsers wired; assert the HTML carries `id="sub-os-iostat"`, `id="sub-os-top"`, and `id="sub-os-vmstat"` anchors (the real IDs the `render/templates/os.html.tmpl` template emits — the task description earlier referenced aspirational `os-disk-util`/`os-top-cpu`/`os-vmstat` names that never landed), all three nested inside `sec-os`, and all three reachable from a `<nav class="index">` entry via `href="#sub-os-*"` so every OS subview is deep-linkable.

### New feature coverage (FR-033–FR-037)

These tasks exist for features already shipping in `render/assets/`.
The implementation tasks are marked `[x]` where the feature is live in
the binary; the paired test tasks remain `[ ]` until the test file is
written.

- [x] T097 [US3] Implement MySQL-defaults modified-vs-default badging for the Variables section (FR-033, research R10): commit `render/assets/mysql-defaults.json` with the curated MySQL 8.0 subset + `_source`/`_updated` metadata header, embed it via `//go:embed`, and surface "modified" / "default" badges in `render/templates/variables.html.tmpl`. Variables absent from the defaults map render without a badge.
- [x] T098 [P] [US3] `render/variables_test.go::TestDefaultsBadges` (FR-033 coverage): builds a hand-crafted Collection with `max_connections=151` (documented default), `max_connections=4096` (modified), and `DefinitelyNotAMySQLVariable_2026` (absent from the defaults map) across two Snapshots. Asserts the rendered HTML carries `data-variable-name=… data-status="default"` for the at-default row, `data-status="modified"` for the changed row, and `data-status="unknown"` for the undocumented row; also asserts the visible `default` / `modified` status-label cells render.
- [x] T099 [US4] Implement mysqladmin category taxonomy (FR-034, research R11): commit `render/assets/mysqladmin-categories.json` with the curated category list and matcher/member rules + provenance header, embed it, compute `variable → category` in `render/render.go::classifyMysqladminCategory`, and emit `categories` + `categoryMap` into the embedded JSON payload consumed by `render/assets/app.js`.
- [x] T100 [P] [US4] `render/db_test.go::TestMysqladminCategoryMap` (FR-034 coverage): drives a synthetic MysqladminData carrying three variables that cover the three resolution paths of `classifyMysqladminCategory` — a matcher-only hit (`Innodb_buffer_pool_size` → `buffer-pool`), a matcher + exclude_matchers override (`Innodb_buffer_pool_read_requests` → `innodb-reads` only because `buffer-pool` excludes `innodb_buffer_pool_read*`), and a members-list override (`Innodb_rows_inserted` → `innodb-writes` via explicit membership). Asserts the rendered `charts.mysqladmin.categoryMap` reproduces those classifications and that the first three entries of `charts.mysqladmin.categories` preserve the declared order in `mysqladmin-categories.json`.
- [x] T101 [US4] Implement default-hidden initial-tally column in the counter-deltas chart (FR-035, research R12): populate `defaultVisible` in the embedded JSON payload excluding column 0's raw tally view; respect `defaultVisible` in `render/assets/app.js`'s initial series selection; preserve the raw tally inside `MysqladminData` so pt-mext parity (research R8) and the `TestPtMextFixture` golden (T065) continue to hold.
- [x] T102 [P] [US4] `render/db_test.go::TestDefaultVisibleSkipsInitialTally` (FR-035 coverage): drives a synthetic MysqladminData with four well-known counters + one gauge. Asserts `charts.mysqladmin.defaultVisible` ⊆ `variables`, contains only counter names (no gauge leaks), carries no `*_initial` pseudo-variant names (the initial-tally VIEW is hidden, the data is not renamed), and every counter's `deltas[v][0]` is present and non-null in the payload so the UI can restore the initial-tally view on request.
- [x] T103 [US1] Implement the Cmd/Ctrl+`\` nav toggle shortcut (FR-036) in `render/assets/app.js`: bind the accelerator to show/hide the nav rail; degrade silently when `window.addEventListener` is unavailable.
- [x] T104 [P] [US1] `render/app_test.go::TestKeyboardShortcutWiring` (FR-036 coverage): source-level assertion on `render/assets/app.js` — a real browser-level harness is rejected per Principle X (no external deps). The test walks the JS source and pins: (a) a `keydown` addEventListener, (b) a `(metaKey || ctrlKey)` modifier check, (c) a `key === "\\"` backslash-literal check, (d) `preventDefault()`, (e) a `classList.contains("nav-hidden")` toggle, (f) the modifier + backslash checks co-locate inside the SAME keydown handler body (not split across functions), (g) the handler is bound to `document.addEventListener("keydown")` and sits within 500 chars of a `"nav-hidden"` reference so the document-level scope is preserved. Rewrite to a real harness if T034 (collapse-persistence) introduces one.
- [x] T105 Implement `@media print` stylesheet (FR-037) in `render/assets/app.css`: expand every `<details>`, suppress the sticky nav rail, flatten to a single scrolling column, keep the verbatim-passthrough banner visible at the top of the first printed page.
- [x] T106 [P] `tests/integration/print_test.go::TestPrintStylesheetPresent` (FR-037 coverage): renders a minimal Collection; extracts every `<style>…</style>` block; asserts at least one `@media print` rule is present and its body (located via a brace-counting walker because the block contains nested rules) contains selectors for `details`, `nav.index` or `.nav-rail`, and a layout override (`.layout` or `grid-template-columns`). Lives in a new `tests/integration/` package created by this task.

---

## Phase 8: UX Quality Audit (Priority: Release Gate) 🔒

**Purpose**: Concretise Constitution Principle XI and the new FR-038–FR-041 / SC-011 via a structured, committed audit pass against `specs/001-ptstalk-report-mvp/checklists/ux-quality.md`. Phase 8 is a **release-cut gate**: a passing audit record (or explicit DEFERRED entries with owner approval) is the precondition for merging any feature-001 cut to `main`. Each section task below produces one reviewable deliverable; the pieces compose into a full audit record under `specs/001-ptstalk-report-mvp/ux-audits/<YYYY-MM-DD>-<label>.md`.

**Independent Test**: After completing the audit, a reviewer can open the audit record and, from that document alone, reproduce every PASS / FAIL / DEFERRED decision against the same rendered report without consulting Slack, PR comments, or the auditor's memory.

### Prerequisites for Phase 8

- [x] T107 Commit the UX-quality audit checklist at `specs/001-ptstalk-report-mvp/checklists/ux-quality.md` (this branch). Normative reference for FR-038–FR-041 / SC-011 / research R13.

### Shared stylesheet invariants (FR-039)

- [ ] T108 Refactor `render/assets/app.css` to express the typography scale, spacing grid, colour palette, chart palette, and axis/legend rules as named CSS custom properties (`--fg`, `--accent`, `--muted`, `--warn`, `--ok`, `--grid`, `--font-h2`, `--font-h3`, `--font-body`, `--chart-series-1…N`). No per-template `<style>` blocks survive the refactor (FR-039). Covered by G-8 in the checklist.
- [ ] T109 [P] `render/render_test.go::TestNoPerTemplateStyleBlocks` (FR-039 coverage): parse the rendered HTML; assert the document contains exactly one top-level `<style>` block (the existing `render.go` concatenation of `embeddedChartCSS + embeddedAppCSS` is acceptable), that the block contains the named CSS custom-property invariants introduced by T108 (`--fg`, `--accent`, `--muted`, `--warn`, `--ok`, `--grid`, `--font-h2`, `--font-h3`, `--font-body`, `--font-caption`), and that every `<section>` / `<details>` in the document is free of inline `<style>` children. The test does NOT prescribe the block's origin — only the invariant set it carries and the per-template absence.

### Section audits (one audit item per subview; produces the per-section PASS/FAIL table for the audit record)

- [ ] T110 OS Usage audit: walk § 3 of `checklists/ux-quality.md` (OS-1 through OS-7) against `testdata/example2/`. Produce a short Markdown fragment (`ux-audits/_pending-os.md` or inline in the audit PR) with per-item status. Fix any FAIL items inline or open a remediation task (TXXX with clear scope).
- [ ] T111 Variables audit: walk § 4 (V-1 through V-6) against `testdata/example2/`; same deliverable shape.
- [ ] T112 Database Usage audit: walk § 5 (DB-1 through DB-6) against `testdata/example2/`; same deliverable shape.
- [ ] T113 [P] Navigation + Header + Banners audit: walk § 1 (H-1..H-5) + § 2 (N-1..N-6).
- [ ] T114 [P] Parser Diagnostics audit: walk § 6 (PD-1..PD-5). Requires a fixture with at least one Warning and one Info diagnostic (e.g., a truncated source file + a multi-snapshot mysqladmin).
- [ ] T115 [P] Print layout audit: walk § 7 (P-1..P-6). Uses the browser's print-preview; greyscale pass included.

### Cross-cutting audits (the harder ones; run after § 3–7 are PASS)

- [ ] T116 Robustness audit (§ 8, FR-040): build synthetic 0-sample, 1-sample, and "near the 1 GB envelope" fixtures and rerun the full checklist against each. Captures R-0, R-1, R-2, R-M, R-MS, R-PC, R-UV. Produces a sub-table of the audit record.
- [ ] T117 No-duplication audit (§ 9, FR-038): grep-based spot-check plus a manual walk. For every canonical data surface, grep the rendered HTML for a distinctive value (a specific PID, a variable name, a counter name) and assert it appears in exactly one subview outside of the Parser Diagnostics panel. Produces D-1 / D-2 / D-3 rows of the audit record.

### Synthesis + gate

- [ ] T118 Commit the full audit record at `specs/001-ptstalk-report-mvp/ux-audits/<YYYY-MM-DD>-<label>.md` using the template at the bottom of `checklists/ux-quality.md`. Every checklist item is PASS or carries an explicit DEFERRED → T### link with owner approval recorded in the same file.
- [ ] T119 CI gate for SC-011: implement a **diff-based** freshness check (not timestamp-based — commit times are unreliable under rebase / cherry-pick). Concretely: a `tests/coverage/ux_audit_freshness_test.go::TestUXAuditFreshness` test that resolves a base commit in the following priority order and fails only when the base is resolvable AND the diff violates the invariant:
      1. `GITHUB_BASE_REF` env var (GitHub Actions sets this on PR builds to the target branch name — `main` in practice). The test runs `git rev-parse "origin/${GITHUB_BASE_REF}"` to get the base SHA.
      2. `MYGATHER_UX_AUDIT_BASE` env var (developer-overridable for local runs).
      3. `git merge-base HEAD origin/main` as a best-effort fallback.
      If none of the above resolves to a valid commit (fork CI without the remote, a shallow clone missing the base, a local clone detached from any remote), the test **SKIPs with a logged reason** rather than failing — the gate is for PR / release contexts, not for every dev invocation.
      When the base IS resolved, the test runs `git diff --name-only <base>..HEAD` and filters the diff to **report-shaping runtime files only**: a non-test `.go` file (i.e. NOT matching `*_test.go`) under `render/`, `model/`, or `parse/`, OR any file under `render/assets/`. If any such file changed, at least one file under `specs/001-ptstalk-report-mvp/ux-audits/` MUST also have changed. Test-only edits (`*_test.go`), `testdata/` additions, and `specs/` edits outside `ux-audits/` are explicitly **excluded** — they cannot change the rendered HTML so they don't need to refresh the audit. No new CI lane; the guard lives inside the regular `go test ./...` pass.

**Checkpoint**: a clean Phase 8 pass means the report ships at a quality bar that any senior reviewer who just read the constitution and the checklist can independently confirm — not a matter of reviewer taste.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)** — no deps; start immediately.
- **Foundational (Phase 2)** — depends on Setup; BLOCKS every user story.
- **US1 (Phase 3)** — depends on Phase 2; is the MVP; no cross-story dependency.
- **US2 / US3 / US4 (Phases 4–6)** — each depends on Phase 2 and on US1's render skeleton (`Render`, template tree, asset loading). Can be developed in parallel by different contributors after US1 lands.
- **Polish (Phase 7)** — depends on every user story that is targeted for the release.
- **UX Quality Audit (Phase 8)** — depends on Phase 7 being substantively complete and on the UX checklist (T107) being committed. Phase 8 is a **release gate**: a clean or fully-DEFERRED audit record is the precondition for merging any feature-001 cut. Running Phase 8 mid-development (e.g., after US2 lands) is encouraged as a diagnostic, not a replacement for the release-cut audit.

### Within Each User Story

- Tests in the phase are written first; implementation makes them pass.
- Per-collector parsers (`parse/*.go`) are parallel to each other ([P]) because each touches its own file. They are NOT parallel with the `parse.Discover` wiring task in the same phase (T051, T060, T074), which integrates them.
- Per-section render tasks are serial within a story because they share `render/templates/report.html.tmpl`, `render/templates/<section>.html.tmpl`, `render/assets/app.js`, and `render/assets/app.css`.
- **The `parse.Discover` wiring tasks T051, T060, and T074 all modify `parse/parse.go`.** They therefore serialise across user stories: merging US2 then US3 then US4 into the shared branch requires sequential edits to this file. This is documented as a known cross-story serial point (F32 resolution) — it does not break story-level independent *testability*, but does mean three parallel developers can't commit these specific tasks without rebase-resolving `parse/parse.go`.
- Goldens are regenerated with `go test ./parse/... -update` (scoped to goldens-using packages; see the `tests/goldens` godoc) as a human-reviewed step — never silently (FR-021).

### Parallel Opportunities

- All Phase 1 `[P]` tasks (T003–T006) run together.
- All Phase 2 `[P]` tasks (T007–T018 except T019 and onward) run together.
- Within US1, T023–T035 test tasks all run in parallel.
- Within US2: T043–T047 tests in parallel; T048–T050 parser impls in parallel.
- Within US4: T063–T068 tests in parallel; T071–T073 parser impls in parallel.
- Phase 7 `[P]` tasks (T078–T083) all run in parallel.

---

## Parallel Example: User Story 1 test fan-out

```bash
# All US1 tests can be written concurrently before a line of implementation exists.
Task: "Write render/render_test.go::TestRenderEmptyReport"
Task: "Write render/render_test.go::TestDeterminism"
Task: "Write render/render_test.go::TestGeneratedAtIsOnlyDiff"
Task: "Write parse/parse_test.go::TestNotAPtStalkDir"
Task: "Write parse/parse_test.go::TestEmptyButValid"
Task: "Write parse/parse_test.go::TestSizeBoundTotalExceeded"
Task: "Write parse/parse_test.go::TestSizeBoundFileExceeded"
Task: "Write cmd/my-gather/main_test.go::TestOutputInsideInputRejected"
Task: "Write cmd/my-gather/main_test.go::TestOverwriteGuard"
Task: "Write tests/integration/e2e_test.go::TestE2EExample2"
Task: "Write tests/integration/offline_test.go::TestOfflineRender"
Task: "Write render/app_test.go::TestCollapsePersistenceRoundTrip"
Task: "Write render/nav_test.go::TestNavCoverage"
```

---

## Implementation Strategy

### MVP (US1 only)

1. Phase 1: Setup — 6 tasks (T001–T006).
2. Phase 2: Foundational — 16 tasks (T007–T022).
3. Phase 3: US1 — 20 tasks (T023–T042).
4. **STOP, VALIDATE**: run against `testdata/example2/`; report is empty but structurally complete and deterministic. Principle I (static binary), IV (determinism), V (self-contained), VI (library-first), VII (typed errors), VIII (fixtures + goldens), IX (zero network), X (zero deps), XII (pinned Go) all demonstrable. MVP deployable / demoable.

### Incremental delivery after MVP

1. + US2 (13 tasks, T043–T055) → OS Usage section live. Principle III, XI strengthened.
2. + US3 (7 tasks, T056–T062) → Variables section live.
3. + US4 (15 tasks, T063–T077) → Database Usage section live.
4. + Polish (29 tasks, T078–T106) → performance guarantees, a11y, docs, coverage gaps from analyze pass 3 (F27–F32, F36) plus the FR-033–FR-037 impl/test pairs added in the 2026-04-22 reconciliation (T097–T106).
5. + UX Quality Audit (13 tasks, T107–T119) → release-gate audit pass covering FR-038–FR-041 / SC-011 / research R13. Produces a committed, structured audit record that blocks release cuts on any un-DEFERRED FAIL.

### Solo-developer strategy (current context)

Serial execution by user-story priority: T001 → T119 (119 tasks total). Treat each `[P]`-marked group as "tasks you can draft in one sitting" rather than tasks to run in separate worktrees.

---

## Notes

- Every `parse/*.go` parser produces `model.Diagnostic` entries for any recoverable issue; each is mirrored to stderr as `[warning]` / `[error]` via the wired `DiagnosticSink` (FR-027).
- `SeverityInfo` diagnostics (snapshot-boundary markers in FR-030; any future informational events) surface **only** in the report's Parser Diagnostics panel — never to stderr (F13 resolution).
- `tests/coverage/testdata_coverage_test.go` (T021) is the hard gate that makes forgetting a fixture impossible; it fails loudly.
- Every `go test ./parse/... -update` run is a reviewed commit: diff the regenerated goldens, eyeball each, then commit.
- When the implementation phase starts, commit frequently — one task per commit where the scope fits; smaller if the task touches multiple files.
- Each task description includes the file path(s) it writes so an LLM (or a developer running `/speckit-implement`) can execute it without additional context.
- **`F##` markers** (e.g. `F7`, `F13`, `F28`) scattered through task descriptions are stable IDs from prior `/speckit-analyze` passes that were folded into the task list. They are informational anchors, not open work items — every `F##` cited has already been addressed by the task that cites it. No external glossary is required; the resolution lives in the task body itself.

---

## Reconciliation Summary (last updated 2026-04-22)

A mechanical pass compared every task ID against the real repository state for the full current range, **T001–T119 (119 tasks total)**. `85 / 119 tasks (≈71 %) carry an [x] mark`. The pipeline builds, `go test ./...` is green, and the binary produces a deterministic three-section report against `testdata/example2/`. The 2026-04-22 feature-tests bundle C closed T098 (FR-033 defaults badges), T100 (FR-034 mysqladmin categoryMap), T102 (FR-035 defaultVisible skips initial-tally), T104 (FR-036 keyboard shortcut wiring — source-level assertion on app.js), and T106 (FR-037 print stylesheet) — every FR-033..037 shipped feature now has its paired test; `tests/integration/` package created to host cross-cutting tests. The earlier 2026-04-22 render-goldens bundle B closed T046 (OS section golden), T057 (Variables section golden), T058 (Variables search markup), T069 (DB section golden), T070 (mysqladmin toggle markup — asserts against the JSON payload since the shipped toggle is JS-built, not server-rendered), and T096 (OS subview anchors / `sub-os-{iostat,top,vmstat}` reachable from the nav rail); `testdata/golden/` now also holds `os.example2.html`, `variables.example2.html`, `db.example2.html`. The earlier 2026-04-22 parser-goldens bundle A closed T044 (top), T045 (vmstat), T063 (innodbstatus), T064 (mysqladmin golden), T066 (mysqladmin snapshot-boundary reset per FR-030), and T067 (mysqladmin variable drift per R8.C), bringing `testdata/golden/` to seven committed parser goldens covering every per-collector parser output. The 2026-04-22 spec-drift reconciliation added five shipping features captured as FR-033–FR-037 (MySQL-defaults badging, mysqladmin category chooser, default-hidden initial-tally column, `Cmd/Ctrl+\` nav shortcut, `@media print` stylesheet); their implementation tasks (T097, T099, T101, T103, T105) are already live in `render/assets/` and `render/render.go` and carry `[x]` marks, and their paired tests (T098, T100, T102, T104, T106) now also carry `[x]` marks as of PR B's companion PR C bundle described above. The 2026-04-22 boundary-markers PR closed T054 (multi-Snapshot concatenation + dashed vertical markers on every time-series chart per FR-018 / FR-030). The 2026-04-22 UX-quality reconciliation added FR-038–FR-041 + SC-011 (research R13) with a committed audit checklist (T107 `[x]`) and Phase 8 release-gate tasks T108–T119 `[ ]`, including a shared-stylesheet refactor (T108) that extracts typography / spacing / palette / chart invariants into named CSS custom properties. What remains across the whole feature is almost entirely **missing tests** (CLI integration, performance, a11y — render-HTML section goldens are committed as of PR B, and FR-033..037 feature tests as of PR C), **the UX audit pass itself**, plus a handful of documentation and coverage gaps — no core implementation is unshipped.

### What's done

- **Phase 1 Setup** (T001–T006): complete.
- **Phase 2 Foundational** (T007–T022, minus T019): complete. `model/`, `parse/` skeleton + errors + version detector, `render/deterministic.go`, `render/render.go`, template tree, CLI skeleton, forbidden-import lint, godoc coverage guard, pt-mext fixture — all present.
- **Phase 3 US1 implementation** (T036–T042): complete. `parse.Discover`, `render.Render`, report template with verbatim banner + sticky nav + `<details>` wrappers, `localStorage`-keyed collapse persistence, end-to-end CLI wiring with atomic write and verbose progress.
- **Phase 3 US1 parse/render tests** (T023–T029): complete. US1 CLI and integration tests (T030–T035) are **not** written.
- **Phase 4 US2 implementation** (T048–T055): complete. `parse/iostat.go` / `top.go` / `vmstat.go` wired into Discover; OS template + uPlot instantiation + data-not-available banner present. **T054 now closes**: `render.concat*` helpers merge every time-series collector across Snapshots and record `SnapshotBoundaries`; chart payloads export them; `render/assets/app.js::drawSnapshotBoundaries` paints dashed vertical markers via the uPlot `drawAxes` hook. Counter-vs-gauge NaN-at-boundary semantics (FR-030) are honoured by `render.concatMysqladmin`.
- **Phase 5 US3 implementation** (T059–T062): complete. Variables parser, Discover wiring, template with `<input type="search">` + `data-variable-name` rows, client-side filter in `app.js`.
- **Phase 6 US4 implementation** (T071–T077): complete. InnoDB / mysqladmin (with `ResetForNewSnapshot` + `SnapshotBoundaries`) / processlist parsers, DB template, mysqladmin multi-select toggle with `localStorage`-persisted selection, uPlot DB charts.
- **Phase 6 US4 tests**: all eight complete. T063 (innodbstatus), T064 (mysqladmin golden), T065 (pt-mext), T066 (snapshot-boundary reset), T067 (variable drift), T068 (processlist parser-side), T069 (DB render golden), and T070 (mysqladmin toggle markup) are now `[x]`.
- **Phase 7** already closed: T083 (cross-compile CI matrix), T084 (README with 12 principles + scope + contributing), T087 (consistency sweep — this pass itself).

### What's still open (39 tasks)

**Test gaps** (biggest bucket):

- US1 CLI + integration + JS-side tests: T030, T031, T032, T033, T034, T035.
- US2 render golden: complete. T046 is now `[x]` (OS section golden).
- US3 variables render golden + search markup: complete. T057, T058 are now `[x]`.
- US4 DB render goldens + markup: complete. T069, T070 are now `[x]`.
- Polish tests still pending: T078 (perf), T079 (memory), T080 (suffix golden coverage), T081 (a11y), T082 (determinism stress), T086 (output-parent-missing), T088 (single output file), T089 (partial recovery across parsers), T090 (version detection), T091/T092/T093 (stderr contract), T094 (degraded one-missing-collector), T095 (version ldflags). SC-005 OS anchors (T096) is now `[x]`.

**Fixture / golden gaps**:

- **T019**: `testdata/example1/` was never committed — only `testdata/example2/` exists. Any test that keys off a distinct first example (or that asserts two format variants co-exist per FR-024) depends on this.
- `testdata/golden/` holds **seven parser goldens** plus **three render goldens** (os/variables/db). Parser-side and render-side `*Golden*` test coverage is now complete for the core section rendering. `render_test` imports `tests/goldens` as of PR B, so `go test ./render/... -update` is the correct regen command for the render-layer goldens. If future render golden tests land in other packages, extend the scope accordingly.

**Feature gap**:

- ~~**T054**: `parse/mysqladmin.go` and `parse/iostat/top/vmstat.go` supply multi-snapshot data, but the renderer does not draw vertical snapshot-boundary markers on the time-series charts.~~ **Closed** in the 2026-04-22 boundary-markers PR: `render.concat*` helpers merge Snapshots and export `SnapshotBoundaries`; `drawSnapshotBoundaries` paints dashed vertical markers via a uPlot hook.

**Documentation gap**:

- **T085**: no `retrospective.md` exists yet. The quickstart flow has not been formally dry-run since the feature spec was written.
- **T084**: README is substantively complete (12 principles, quickstart, scope, contributing) except for a project status badge — minor.

### Pipeline-critical order for closing the gaps

1. **Generate goldens first.** Run `go test ./parse/... -update` once all parser + render tests are written (the `-update` flag is only registered in packages that import `tests/goldens`; widen the scope as more packages adopt it). Without goldens, a dozen tests can't assert anything. The coverage guard at T021 / T080 already expects them.
2. **Parser + render-golden tests** (T043–T047, T056, T063, T064, T066–T070, T046, T057, T058, T096) are complete as of the PR A + PR B bundle merges. Remaining test work is CLI / integration, FR-033..037 feature tests, and performance / a11y.
3. **Write the CLI / integration tests** (T030, T031, T032, T033, T034, T035, T086, T088, T091–T095, T094). These need a binary built into `./bin/my-gather` — most can use `go test`-driven `exec.Command`.
4. **Write the FR-033..037 feature tests** (T098, T100, T102, T104, T106). Same file as the render section goldens already written; assert behaviours of the shipped rendering features.
5. **Polish tests** (T078, T079, T080, T081, T082, T089, T090). Slower bucket — perf and stress tests can live in a `-short`-gated tag to keep the default test loop fast.
6. ~~**Close T054**~~ **Closed** (2026-04-22). Remaining: the corresponding section-audit item in the upcoming Phase 8 pass will confirm boundary markers render correctly against `testdata/example2/`.
7. **T019 example1 fixture**: optional for MVP unless FR-024's "two supported versions" assertion is being tested.
8. **T085 retrospective**: run the quickstart flow end-to-end once everything above is green.

### Suggested immediate follow-up

- Open a thin PR per bucket (US1 tests, US2 tests, …) rather than one mega-PR — each bucket is independently mergeable and keeps golden reviews manageable.
- Gate all *Golden tests behind `-short` skip or `testing.Short()` only if they become slow; otherwise leave them in the default run.
- Consider whether the "two supported pt-stalk versions" clause in FR-024 is worth the fixture duplication (T019 ×2 + 14 golden pairs) for the MVP. If the team would accept tracking only the current stable for v1, amend FR-024 and trim the T019 scope.

Status at this checkpoint: **MVP binary is functional and demoable; test suite needs completion before `/speckit-implement` declares Phase 7 polished.**
