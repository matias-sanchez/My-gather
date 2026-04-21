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

All tasks write paths relative to the repository root
(`/Users/matias/git/My-gather/My-gather`).

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialisation and build/CI scaffolding.

- [ ] T001 Create the directory layout declared in plan.md (`cmd/my-gather/`, `parse/`, `model/`, `render/templates/`, `render/assets/`, `testdata/golden/`, `testdata/pt-mext/`, `tests/`, `scripts/`, `.github/workflows/`). Empty `.gitkeep` files where a directory must exist but has no content yet.
- [ ] T002 Initialize the Go module at `go.mod`: `module github.com/matias-sanchez/My-gather`, pin `go 1.24`, no non-stdlib direct dependencies (Principle X, XII).
- [ ] T003 [P] Add `Makefile` with targets `build` (local), `test`, `release` (cross-compile CGO_ENABLED=0 for linux/{amd64,arm64} and darwin/{amd64,arm64} into `dist/` per plan.md Distribution section), and `clean`.
- [ ] T004 [P] Add `.github/workflows/ci.yml` running `go vet ./...` and `go test ./...` on the pinned Go version across ubuntu-latest and macos-latest. Include a determinism check step and a cross-compile smoke step.
- [ ] T005 [P] Add a minimal `README.md` at the repo root covering install, quickstart run, and a pointer to `specs/001-ptstalk-report-mvp/` for deeper context. Content based on `quickstart.md`.
- [ ] T006 [P] Commit `scripts/anonymise-fixtures.sh` (bash + sed/awk only, per research R6) with the anonymisation rules listed in R6; script is idempotent and deterministic.

**Checkpoint**: Repository builds (`go build ./...` succeeds with empty packages), CI runs, cross-compile targets produce artifacts.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Types, errors, skeletons, and the CI guards that every user story depends on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T007 [P] Define the `model` package types in `model/model.go`: `Collection`, `Snapshot`, `SourceFile`, `Sample`, `MetricSeries`, `VariableEntry`, `ThreadStateSample`, `Diagnostic`, `Severity` (enum), `Suffix` (enum + `KnownSuffixes` slice + all seven constants), `FormatVersion` (enum), `ParseStatus` (enum). Every exported identifier has a godoc comment (Principle VI).
- [ ] T008 [P] Define the per-collector typed payloads in `model/model.go` (continuing T007): `IostatData`, `DeviceSeries`, `TopData`, `ProcessSample`, `ProcessSeries`, `VariablesData`, `VmstatData`, `InnodbStatusData`, `AHIActivity`, `MysqladminData` (including `SnapshotBoundaries []int`), `ProcesslistData`, `ThreadStateSample`. Match `data-model.md` exactly.
- [ ] T009 [P] Define the section / render types in `model/sections.go`: `OSSection`, `VariablesSection`, `DBSection`, `SnapshotVariables`, `SnapshotInnoDB`, `Report`, `NavEntry`. `Report.Navigation []NavEntry` and `Report.ReportID string` populated per FR-031 / FR-032.
- [ ] T010 [P] Define `parse/errors.go` with sentinels `ErrNotAPtStalkDir`, and typed `SizeError` (with `SizeErrorKind`, `SizeErrorTotal`, `SizeErrorFile`), `PathError`, `ParseError`. Implement `Error()` / `Unwrap()` per contracts/packages.md.
- [ ] T011 [P] Define `parse/version.go` with `DetectFormat(peekedBytes []byte, suffix model.Suffix) model.FormatVersion` helper and documentation referencing research R2.
- [ ] T012 [P] Define `parse/parse.go` skeleton with `DefaultMaxCollectionBytes`, `DefaultMaxFileBytes`, `DiscoverOptions`, `DiagnosticSink`, and the `Discover(ctx, rootDir, opts) (*model.Collection, error)` signature. Initial body: snapshot-regex discovery, size-bound preflight, pt-summary.out fallback — no per-collector parsing yet (that lands per-story).
- [ ] T013 [P] Define `render/deterministic.go` with `sortedKeys[K, V]`, `sortedEntries[K, V]`, `formatFloat(v, precision)`, ordinal ID counter, and UTC timestamp formatter. Unit tests in `render/deterministic_test.go` covering every helper's determinism.
- [ ] T014 [P] Vendor the uPlot asset (research R1): commit `render/assets/chart.min.js`, `render/assets/chart.min.css`, `render/assets/NOTICE` (upstream MIT license + copyright), `render/assets/app.js` skeleton (placeholder for collapse persistence + mysqladmin toggle), `render/assets/app.css` skeleton (sticky nav rail styles). Add `render/assets.go` with `//go:embed` directives for each.
- [ ] T015 [P] Define `render/render.go` skeleton with `Render(w io.Writer, c *model.Collection, opts RenderOptions) error` signature, report-envelope construction (including `Navigation` + `ReportID`), and template dispatch; initial body renders every section as empty / "data not available". Also define `RenderOptions`.
- [ ] T016 [P] Create the HTML template tree: `render/templates/report.html.tmpl` (top-level layout, top-of-doc verbatim banner per FR-026, nav rail per FR-031, `<details>` wrappers per FR-032), `render/templates/os.html.tmpl`, `render/templates/variables.html.tmpl`, `render/templates/db.html.tmpl`. Templates use only sorted data; no raw map iteration (Principle IV).
- [ ] T017 [P] CLI skeleton in `cmd/my-gather/main.go`: flag parsing per `contracts/cli.md` (including `-o`/`--out`, `--overwrite`, `-v`/`--verbose`, `--version`, `-h`/`--help`), positional arg handling, output-path-inside-input guard (FR-029, exit code 7), exit-code dispatch (0, 2, 3, 4, 5, 6, 7, 70), `--version` output format.
- [ ] T018 [P] Forbidden-import linter test at `tests/lint/imports_test.go` (research R7). Walks the import graph of `cmd/`, `parse/`, `model/`, `render/`; fails if any of `net/http`, `net`, `net/rpc`, `net/smtp`, `crypto/tls` appear. Stdlib-only via `go/parser` + `go/ast`.
- [ ] T019 Commit anonymised fixtures under `testdata/example1/` and `testdata/example2/` produced by `scripts/anonymise-fixtures.sh` run against `_references/examples/example1/` and `_references/examples/example2/`. Only the seven source-file suffixes this feature supports need be committed (plus `pt-summary.out` / `pt-mysql-summary.out` for discovery tests).
- [ ] T020 Commit the pt-mext golden fixture under `testdata/pt-mext/`: `input.txt` is a verbatim copy of `_references/pt-mext/pt-mext-improved.cpp` comment lines 146–177; `expected.txt` is a verbatim copy of lines 181–189 with a `testdata/pt-mext/README.md` describing provenance (research R8).
- [ ] T021 Coverage guard test at `tests/coverage/testdata_coverage_test.go`: fails the build if any `model.Suffix` constant lacks a fixture under `testdata/example1/` or `testdata/example2/`, OR lacks an expected golden output file under `testdata/golden/`. Enforces Constitution Principle VIII and spec FR-021 / SC-006.
- [ ] T022 [P] godoc-coverage test at `tests/coverage/godoc_coverage_test.go`: walks AST of `model`, `parse`, `render` and asserts every exported identifier has a non-empty `//` doc comment (Constitution Principle VI).

**Checkpoint**: `go test ./...` passes (lint and coverage guards pass over empty packages); CLI builds; an invocation against any test fixture prints exactly one HTML file whose sections all read "data not available".

---

## Phase 3: User Story 1 - First-look triage report end-to-end (Priority: P1) 🎯 MVP

**Goal**: Prove the parse → model → render → embed-assets → byte-deterministic HTML pipeline end-to-end against a real pt-stalk directory. Every section shows "data not available" because no per-collector parsers are wired yet; this is intentional. The report, navigation, collapse behaviour, and determinism guarantees are all demonstrable against the empty report.

**Independent Test**: Run `./bin/my-gather testdata/example2 -o /tmp/report.html`. The command exits 0, `/tmp/report.html` exists, opening it in an offline browser shows the three section headers (OS Usage, Variables, Database Usage), a nav index linking to each, all three collapsed/expanded correctly, the verbatim-passthrough banner at the top, and each section body reading "data not available". Running twice in a row produces byte-identical HTML except the `Report generated at` timestamp.

### Tests for User Story 1 ⚠️ (Write first; implementation makes them pass)

- [ ] T023 [P] [US1] `render/render_test.go::TestRenderEmptyReport`: render a Collection with zero Snapshots (or with Snapshots whose `SourceFiles` is empty); assert every section emits its "data not available" banner.
- [ ] T024 [P] [US1] `render/render_test.go::TestDeterminism`: render twice against the same Collection with the same `RenderOptions.GeneratedAt`; assert `bytes.Equal` on outputs (spec SC-003).
- [ ] T025 [P] [US1] `render/render_test.go::TestGeneratedAtIsOnlyDiff`: render twice with different `GeneratedAt` values; assert the diff consists of exactly one line (the `Report generated at` line) (spec FR-006).
- [ ] T026 [P] [US1] `parse/parse_test.go::TestNotAPtStalkDir`: run `Discover` against an empty tempdir; assert `errors.Is(err, parse.ErrNotAPtStalkDir)`.
- [ ] T027 [P] [US1] `parse/parse_test.go::TestEmptyButValid`: run `Discover` against a dir containing one timestamped placeholder file with an unsupported suffix (e.g., `-hostname` only) OR a bare `pt-summary.out`; assert `err == nil` and `Collection.Snapshots` is non-empty and every `SourceFile` entry is absent (per FR-020).
- [ ] T028 [P] [US1] `parse/parse_test.go::TestSizeBoundTotalExceeded`: run `Discover` against a fixture whose total size exceeds `DiscoverOptions.MaxCollectionBytes`; assert `errors.As(err, &parse.SizeError{})` with `Kind == SizeErrorTotal`.
- [ ] T029 [P] [US1] `parse/parse_test.go::TestSizeBoundFileExceeded`: same as T028 but with a single oversized file; assert `Kind == SizeErrorFile`.
- [ ] T030 [P] [US1] `cmd/my-gather/main_test.go::TestOutputInsideInputRejected`: invoke the CLI with `-o <somewhere-inside-input-dir>`; assert exit code 7 and the report file was NOT written (spec FR-029).
- [ ] T031 [P] [US1] `cmd/my-gather/main_test.go::TestOverwriteGuard`: invoke twice without `--overwrite`; assert exit code 6 on the second call.
- [ ] T032 [P] [US1] `tests/integration/e2e_test.go::TestE2EExample2`: run the compiled binary against `testdata/example2/`; assert exit 0, output file exists, contains the three section headers, and contains the verbatim banner text.
- [ ] T033 [P] [US1] `tests/integration/offline_test.go::TestOfflineRender`: open the rendered HTML with a headless-browser–free check — walk the raw HTML and assert no `http://`, `https://`, or `//` (protocol-relative) URLs appear in `<script src>`, `<link href>`, or `<img src>` attributes; assert all `<script>` and `<style>` tags are inline or embed-sourced (spec FR-004, SC-002).
- [ ] T034 [P] [US1] `render/app_test.go::TestCollapsePersistenceRoundTrip`: run the collapse-persistence logic from `render/assets/app.js` under a lightweight JS test harness (or port to a Go-testable form); stub `localStorage`; assert save→load round-trip keyed by `Report.ReportID` (spec SC-010).
- [ ] T035 [P] [US1] `render/nav_test.go::TestNavCoverage`: for a rendered report, assert every `NavEntry.ID` in `Report.Navigation` has a matching `id="..."` target in the HTML; assert the count of nav entries equals the count of unique anchor targets (spec SC-009).

### Implementation for User Story 1

- [ ] T036 [US1] Implement `parse.Discover` body in `parse/parse.go`: size preflight (T028/T029 green), timestamped-file regex scan (research R5), `pt-summary.out` fallback, Snapshot grouping, absent-SourceFile handling, `ErrNotAPtStalkDir` only in the true "no timestamps AND no summary" case (per FR-020 remediation).
- [ ] T037 [US1] Implement `render.Render` body in `render/render.go`: build `Report`, compute `Navigation` as a deterministic flat list derived from section iteration order (OS → Variables → DB → Parser Diagnostics), compute `ReportID` as SHA-256 of canonicalised Collection (excluding `RootPath` and `GeneratedAt`) truncated to 12 hex chars (research R9 with F21 correction), dispatch to templates, pipe through deterministic helpers.
- [ ] T038 [US1] Flesh out `render/templates/report.html.tmpl`: HTML5 skeleton, top-of-document verbatim-passthrough banner (FR-026 — fixed wording: "This report is derived verbatim from a pt-stalk collection. Hostnames, IP addresses, query text, and MySQL variables appear as captured. Responsible sharing is the reader's responsibility."), sticky nav rail on the left, header with tool version + GeneratedAt, main content column with three `<details>` section wrappers, footer with Parser Diagnostics `<details>` (default closed per FR-032 / Principle XI).
- [ ] T039 [US1] Implement `render/assets/app.js` collapse-persistence module: read `Report.ReportID` from a JSON metadata block embedded into the HTML, key `localStorage` entries as `mygather:<reportID>:collapse:<sectionID>`, restore state on `DOMContentLoaded`, save on `<details>` `toggle` events. Degrade silently if `localStorage` is unavailable (F22: private browsing / disabled storage).
- [ ] T040 [US1] Implement `render/assets/app.css` sticky nav rail styles: CSS Grid layout with rail on the left, main content on the right, `position: sticky` on the nav element, `@media print { details { open: true } }` print rule (F23). `@media (max-width: 800px)` fallback to top-horizontal nav.
- [ ] T041 [US1] Wire `cmd/my-gather/main.go` end-to-end: resolve paths (with `filepath.EvalSymlinks`), run the output-inside-input guard (FR-029), call `parse.Discover`, map errors to exit codes per `contracts/cli.md` exit-code table, call `render.Render`, write atomically via `os.CreateTemp` + `Sync` + `os.Rename` in the output's parent directory, mirror `Diagnostic` warnings/errors to stderr per FR-027.
- [ ] T042 [US1] Implement `-v` / `--verbose` stderr progress lines (FR-027): per-file `[parse]` and `[render]` lines using the format in `contracts/cli.md`; also ensure `SeverityInfo` diagnostics do NOT mirror to stderr (F13 resolution — only `Warning` and `Error` do).

**Checkpoint**: Running `./bin/my-gather testdata/example2 -o /tmp/report.html` produces a working, self-contained, deterministic HTML file with navigation, collapse, banner, and three empty sections. Every test T023–T035 passes.

---

## Phase 4: User Story 2 - OS Usage section (Priority: P2)

**Goal**: Fill in the OS Usage section with three subviews — per-device disk utilisation, top-3 CPU processes, and vmstat resource saturation — from parsed real data.

**Independent Test**: Run against `testdata/example2/`; assert the OS Usage section renders three non-empty chart subviews with correct device names, PID labels, and time-series data ranges.

### Tests for User Story 2

- [ ] T043 [P] [US2] `parse/iostat_test.go::TestIostatGolden`: parse `testdata/example2/.../-iostat` and compare resulting `IostatData` JSON-serialised against `testdata/golden/example2.iostat.json`. Regenerate with `go test ./parse/... -update`.
- [ ] T044 [P] [US2] `parse/top_test.go::TestTopGolden`: parse `-top` fixture; compare against golden. Also assert `Top3ByAverage` is correctly computed via **average CPUPercent normalised over total samples** (absent in a sample = contributes 0) — F7 resolution, aligning with spec FR-010 "aggregate".
- [ ] T045 [P] [US2] `parse/vmstat_test.go::TestVmstatGolden`: parse `-vmstat` fixture; compare against golden. Assert the declared `Series` order is present (runqueue, blocked, free_kb, buff_kb, cache_kb, swap_in, swap_out, io_in, io_out, cpu_user, cpu_sys, cpu_idle, cpu_iowait).
- [ ] T046 [P] [US2] `render/os_test.go::TestOSGoldenHTML`: render a Collection with only OS parsers wired; compare the OS section's rendered HTML against `testdata/golden/example2.os.html`.
- [ ] T047 [P] [US2] `parse/iostat_test.go::TestIostatPartialRecovery`: truncate the fixture mid-sample; assert parser returns usable data + `Diagnostic(Severity=Warning)` with `Location` pointing at the truncation (Principle III, FR-008).

### Implementation for User Story 2

- [ ] T048 [P] [US2] Implement `parse/iostat.go` — per-device utilisation + avgqu_sz series; handle the two supported pt-stalk format variants (FR-024); emit diagnostics for malformed rows. Covered by T043.
- [ ] T049 [P] [US2] Implement `parse/top.go` — parse repeated `top` batch snapshots; build per-process samples; compute `Top3ByAverage` using **aggregate CPU normalised by total sample count** (absent = 0) so sustained load ranks above short spikes (F7 resolution). Covered by T044.
- [ ] T050 [P] [US2] Implement `parse/vmstat.go` — 13-series fixed-order time-series extraction; diagnostics for missing columns across the two supported format variants. Covered by T045.
- [ ] T051 [US2] Wire OS parsers into `parse.Discover` so each Snapshot's `SourceFiles[SuffixIostat|SuffixTop|SuffixVmstat].Parsed` is populated with typed data.
- [ ] T052 [US2] Implement `render/templates/os.html.tmpl` — three subview blocks, each with its own `<details>` wrapper, each linked from the nav rail (T037 Navigation population is extended to list "Disk utilization", "Top CPU processes", "vmstat saturation" as Level=2 NavEntries under "OS Usage").
- [ ] T053 [US2] Wire uPlot chart instantiation in `render/assets/app.js` for the three OS subviews: read JSON data payload from `<script type="application/json">` blocks, instantiate uPlot with deterministic options (fixed colours, no animations), attach to the correct `<canvas>` elements.
- [ ] T054 [US2] Implement multi-snapshot concatenation with boundary markers for `-iostat`, `-top`, `-vmstat` (FR-018): concatenate samples on a shared time axis; emit a vertical boundary annotation at each snapshot-boundary timestamp.
- [ ] T055 [US2] "Data not available" banner wiring for the three OS subviews (FR-007) when any of `-iostat` / `-top` / `-vmstat` is absent from the collection.

**Checkpoint**: US1 + US2 produce a report whose OS Usage section renders correctly against `testdata/example2/`. Every test T023–T047 passes.

---

## Phase 5: User Story 3 - Variables section (Priority: P3)

**Goal**: Fill in the Variables section with a searchable, alphabetically-sorted table of every global variable captured.

**Independent Test**: Run against `testdata/example2/`; assert the Variables section shows all global variables from the `-variables` fixture in one table per snapshot, with working client-side search.

### Tests for User Story 3

- [ ] T056 [P] [US3] `parse/variables_test.go::TestVariablesGolden`: parse `-variables` fixture; compare `VariablesData` against golden; assert alphabetical sort, no duplicates (first-global-wins dedup per FR-012).
- [ ] T057 [P] [US3] `render/variables_test.go::TestVariablesGoldenHTML`: render Variables section; compare against golden HTML.
- [ ] T058 [P] [US3] `render/variables_test.go::TestVariablesSearchMarkup`: assert the rendered table has a `<input type="search">` with the expected id, and each `<tr>` has a `data-variable-name` attribute for the client-side filter (FR-013).

### Implementation for User Story 3

- [ ] T059 [P] [US3] Implement `parse/variables.go` — pipe-delimited table parser; alphabetical sort; dedup by name; emit Diagnostic for any duplicate-name entries it dropped.
- [ ] T060 [US3] Wire `-variables` into `parse.Discover` (similar to OS parsers in T051).
- [ ] T061 [US3] Implement `render/templates/variables.html.tmpl` — one `<details>` per snapshot (per FR-018) containing one `<table>` with the search input and all variable rows. Nav rail gets one Level=2 NavEntry per snapshot under "Variables".
- [ ] T062 [US3] Implement the client-side search filter in `render/assets/app.js` — input event listener that toggles `hidden` on `<tr>` rows whose `data-variable-name` doesn't substring-match the input value. No network, no libraries (FR-013, Principle X).

**Checkpoint**: US1 + US2 + US3 produce a report with a working Variables table. Every test through T058 passes.

---

## Phase 6: User Story 4 - Database Usage section (Priority: P4)

**Goal**: Fill in the Database Usage section with four subviews — InnoDB internals, interactive mysqladmin deltas, processlist thread states, and the snapshot-boundary-aware merger.

**Independent Test**: Run against `testdata/example2/`; assert every DB subview is populated with data from its respective fixture; assert the mysqladmin toggle UI lets the user enable/disable variable series; assert per-snapshot InnoDB scalars render side-by-side.

### Tests for User Story 4

- [ ] T063 [P] [US4] `parse/innodbstatus_test.go::TestInnoDBStatusGolden`: parse `-innodbstatus1` fixture; extract SemaphoreCount, PendingReads/Writes, AHI activity, HLL; compare against golden.
- [ ] T064 [P] [US4] `parse/mysqladmin_test.go::TestMysqladminGolden`: parse `-mysqladmin` fixture; compare against golden; assert counter vs gauge classification matches the declared allowlist; assert `SnapshotBoundaries` is populated correctly when parsing a concatenated multi-snapshot input.
- [ ] T065 [P] [US4] `parse/mysqladmin_test.go::TestPtMextFixture`: parse `testdata/pt-mext/input.txt` with the Go mysqladmin parser; format aggregates (total/min/max/avg) for each counter; assert structural equivalence against `testdata/pt-mext/expected.txt` (F10 resolution — structural comparison, not byte-for-byte, so whitespace normalisation is allowed).
- [ ] T066 [P] [US4] `parse/mysqladmin_test.go::TestSnapshotBoundaryReset`: parse two concatenated snapshots; assert post-boundary first-slot delta is `math.NaN()` for every counter (FR-030); assert one `Diagnostic(Severity=Info)` per boundary.
- [ ] T067 [P] [US4] `parse/mysqladmin_test.go::TestVariableDrift`: parse a fixture where a counter appears in snapshot 1 but not snapshot 2; assert `NaN` in the drift slot + one `Diagnostic(Severity=Warning)` (research R8 improvement C).
- [ ] T068 [P] [US4] `parse/processlist_test.go::TestProcesslistGolden`: parse `-processlist` fixture; compare against golden; assert `Other`-bucketing for unknown/empty states (FR-017).
- [ ] T069 [P] [US4] `render/db_test.go::TestDBGoldenHTML`: render DB section; compare against golden HTML.
- [ ] T070 [P] [US4] `render/db_test.go::TestMysqladminToggleMarkup`: assert the rendered DB section has a `<select multiple>` (or equivalent multi-select) whose options match `MysqladminData.VariableNames`; assert each chart series `<canvas>` element has the matching `data-variable-name` attribute.

### Implementation for User Story 4

- [ ] T071 [P] [US4] Implement `parse/innodbstatus.go` — section-aware free-text parser for `-innodbstatus1`; extract the four scalar views; diagnose unexpected-section occurrences.
- [ ] T072 [P] [US4] Implement `parse/mysqladmin.go` — the pt-mext port. Functions: `parseMysqladmin(r io.Reader, format FormatVersion) (*model.MysqladminData, []model.Diagnostic)`. Line-by-line state machine: skip non-`|` lines; `Variable_name` row → sample boundary → increment `col`; data row → `delta = value - previous[name]`, `previous[name] = value`. Floating-point aggregates (research R8 improvement B). Counter-vs-gauge classification via a hardcoded allowlist keyed by `FormatVersion`. Snapshot-boundary reset (FR-030 / R8 improvement D) applied when the caller passes concatenated input — implemented as an explicit `ResetPrevious()` call at boundaries by the Discover glue, not guessed from the stream.
- [ ] T073 [P] [US4] Implement `parse/processlist.go` — pipe-delimited multi-sample parser; thread-state bucketing; `Other` fallback for empty/unknown states.
- [ ] T074 [US4] Wire all three DB parsers into `parse.Discover`. `-mysqladmin` specifically: when the same collector exists across multiple snapshots, build a single merged `MysqladminData` by concatenating samples and calling `ResetPrevious()` at each boundary; populate `SnapshotBoundaries`.
- [ ] T075 [US4] Implement `render/templates/db.html.tmpl` — four subview blocks: InnoDB side-by-side scalar callouts per snapshot; mysqladmin interactive chart with `<select multiple>` toggle; processlist thread-state stacked chart; vertical boundary markers on all time-series charts (FR-030 renderer requirement). Nav rail extended with four Level=2 NavEntries under "Database Usage".
- [ ] T076 [US4] Implement the mysqladmin interactive toggle in `render/assets/app.js` — multi-select change event toggles chart series visibility; persist selected-variables state via the same `Report.ReportID`-keyed `localStorage` scheme as collapse state (research R9).
- [ ] T077 [US4] Implement uPlot chart wiring for DB subviews in `render/assets/app.js` (extension of T053): processlist stacked chart, mysqladmin multi-series chart with boundary markers.

**Checkpoint**: US1 + US2 + US3 + US4 complete. Every FR has an implementing task and a verifying test. A run against `testdata/example2/` produces a full-featured report.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Coverage guards, performance verification, CI tightening, documentation, remaining MEDIUM-severity analyze findings.

- [ ] T078 [P] Performance test at `tests/perf/perf_test.go`: cold-start run against a 50 MB synthetic or anonymised fixture; assert wall time < 10 s on the CI runner (spec SC-007). Also assert end-to-end run on a 100 MB fixture completes under 60 s (spec SC-001).
- [ ] T079 [P] Memory test at `tests/perf/memory_test.go`: run against a 1 GB synthetic fixture; assert peak RSS < 2 GB via `runtime.MemStats` + `runtime.GC()` snapshots (spec SC-008).
- [ ] T080 [P] Coverage metatest at `tests/coverage/suffix_golden_test.go`: for every `model.Suffix` constant in `KnownSuffixes`, assert at least one matching fixture file exists under `testdata/example1/` or `testdata/example2/` AND at least one golden file exists under `testdata/golden/` — strengthens T021 (spec SC-006).
- [ ] T081 [P] Accessibility smoke test at `tests/integration/a11y_test.go`: render a report; assert the nav index is a `<nav>` element containing `<a href="#...">` links; assert every `<details>` has a `<summary>` child; assert no interactive element relies on a custom role without an `aria-label`. Matches F25 remediation scope.
- [ ] T082 [P] Determinism stress test at `render/determinism_stress_test.go`: render 10 times in a loop against `testdata/example2/` with the same `GeneratedAt`; assert all outputs are byte-identical (SC-003 hardening).
- [ ] T083 [P] Cross-compile matrix verification in CI: extend `.github/workflows/ci.yml` to build and smoke-run `./bin/my-gather --version` for each of linux/{amd64,arm64} and darwin/{amd64,arm64} (Principle I, Technical Context).
- [ ] T084 Flesh out `README.md` with the content from `quickstart.md` plus: project status badge, the twelve constitution principles as a short bullet list, pointer to `specs/001-ptstalk-report-mvp/`, build/install/run/contribute sections.
- [ ] T085 Run the `quickstart.md` flow end-to-end manually and record any friction in a new `specs/001-ptstalk-report-mvp/retrospective.md`; feed fixes back as follow-up tasks if needed.
- [ ] T086 Confirm `--out <nonexistent-dir>/...` behaviour in `cmd/my-gather/main_test.go::TestOutputParentMissing` — assert clean error and exit code 3 (or a new code if the team prefers) rather than auto-mkdir (F8 resolution).
- [ ] T087 Review `spec.md` once after tasks complete, remove any remaining `StateSample` references (F5 sweep), fix the `SuffixInnodbStatus` → value mismatch notes (F16), and correct the constitution's `references/examples/` path (F12) via a small PATCH-level constitution amendment.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)** — no deps; start immediately.
- **Foundational (Phase 2)** — depends on Setup; BLOCKS every user story.
- **US1 (Phase 3)** — depends on Phase 2; is the MVP; no cross-story dependency.
- **US2 / US3 / US4 (Phases 4–6)** — each depends on Phase 2 and on US1's render skeleton (`Render`, template tree, asset loading). Can be developed in parallel by different contributors after US1 lands.
- **Polish (Phase 7)** — depends on every user story that is targeted for the release.

### Within Each User Story

- Tests in the phase are written first; implementation makes them pass.
- Per-collector parsers (`parse/*.go`) are parallel to each other ([P]) because each touches its own file. They are NOT parallel with the `parse.Discover` wiring task in the same phase (T051, T060, T074), which integrates them.
- Per-section render tasks are serial within a story because they share `render/templates/` and `render/assets/app.js`.
- Goldens are regenerated with `go test ./... -update` as a human-reviewed step — never silently (FR-021).

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

1. + US2 (12 tasks, T043–T055) → OS Usage section live. Principle III, XI strengthened.
2. + US3 (7 tasks, T056–T062) → Variables section live.
3. + US4 (15 tasks, T063–T077) → Database Usage section live.
4. + Polish (10 tasks, T078–T087) → performance guarantees, a11y, docs.

### Solo-developer strategy (current context)

Serial execution by user-story priority: T001 → T087. Treat each `[P]`-marked group as "tasks you can draft in one sitting" rather than tasks to run in separate worktrees.

---

## Notes

- Every `parse/*.go` parser produces `model.Diagnostic` entries for any recoverable issue; each is mirrored to stderr as `[warning]` / `[error]` via the wired `DiagnosticSink` (FR-027).
- `SeverityInfo` diagnostics (snapshot-boundary markers in FR-030; any future informational events) surface **only** in the report's Parser Diagnostics panel — never to stderr (F13 resolution).
- `tests/coverage/testdata_coverage_test.go` (T021) is the hard gate that makes forgetting a fixture impossible; it fails loudly.
- Every `go test ./... -update` run is a reviewed commit: diff the regenerated goldens, eyeball each, then commit.
- When the implementation phase starts, commit frequently — one task per commit where the scope fits; smaller if the task touches multiple files.
- Each task description includes the file path(s) it writes so an LLM (or a developer running `/speckit-implement`) can execute it without additional context.
