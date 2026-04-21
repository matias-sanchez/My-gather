# Implementation Plan: pt-stalk Report MVP

**Branch**: `001-ptstalk-report-mvp` | **Date**: 2026-04-21 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-ptstalk-report-mvp/spec.md`

## Summary

Ship the first vertical slice of My-gather: a Go CLI that reads a pt-stalk
output directory and writes one self-contained HTML diagnostic report with
three sections — OS Usage, Variables, Database Usage — sourced from
`-iostat`, `-top`, `-variables`, `-vmstat`, `-innodbstatus1`, `-mysqladmin`,
and `-processlist` files. Implementation honours the twelve constitution
principles: one statically-linked binary, stdlib-first, golden-tested
parsers, byte-deterministic HTML with embedded chart JS, graceful
degradation when inputs are missing. The MVP supports Percona Toolkit
`pt-stalk` current stable and one version back; inputs up to 1 GB /
200 MB per file; no redaction, no JSON export, no network.

## Technical Context

**Language/Version**: Go (current stable at implementation time; minimum
`go 1.24` required for the embed and `slices` stdlib features used).
Version is pinned in `go.mod` and verified in CI per Constitution
Principle XII.

**Primary Dependencies**:

- **Standard library only for core logic**: `flag` (CLI), `html/template`
  and `text/template` (rendering), `embed` (asset inlining), `bufio` +
  `io` (streaming reads), `encoding/json` (embedded data payload),
  `sort`, `strconv`, `strings`, `time`, `path/filepath`, `os`,
  `errors`, `fmt`, `log`, `testing`.
- **One third-party JS asset** vendored into `render/assets/`: a
  lightweight time-series chart library (decision in `research.md`).
  This is a client-side asset bundled via `//go:embed`, **not a Go
  dependency**; `go.mod` remains empty beyond the Go version directive.
- **Dev-only** (`testing` package only): `go test` with `-update`
  convention for regenerating golden files; no `testify`, no `gomock`.
- **Historical reference** (not shipped, not a dependency, not built):
  `_references/pt-mext/pt-mext-improved.cpp` is the C++ source from
  which the `-mysqladmin` delta algorithm was ported to native Go in
  `parse/mysqladmin.go` (see research R8 and spec FR-028). The worked
  example at the tail of that file (lines 146–189) is promoted to a
  committed golden fixture under `testdata/pt-mext/` as the
  correctness anchor. No C++ toolchain is required to build, test, or
  ship My-gather.

**Storage**: N/A — the tool reads from a filesystem directory and writes
one output file. No databases, no caches, no lock files.

**Testing**: `go test ./...` with golden-file round-trips per collector
(spec FR-021). Fixtures live under `testdata/<example-name>/` and are
anonymised subsets of `_references/examples/`. Golden HTML outputs and
golden parsed-model JSON live under `testdata/golden/`. A determinism
test runs the render pipeline twice on the same fixture and asserts
byte-identical output (spec SC-003).

**Target Platform**: `linux/amd64`, `linux/arm64`, `darwin/amd64`,
`darwin/arm64`. Built with `CGO_ENABLED=0` for full static linkage
(Principle I). Windows is not a target for this feature.

**Project Type**: Single-project Go CLI with three library packages
(`parse/`, `model/`, `render/`) consumable independently per Principle VI.

**Performance Goals** (from spec Success Criteria):

- SC-001: end-to-end report generation for a ≤ 100 MB collection under
  60 seconds of wall time.
- SC-007: cold-start run on a 50 MB collection under 10 seconds.
- SC-008: 1 GB collection completes under 2 GB resident memory, no OOM.

**Constraints**:

- Byte-deterministic HTML output except for the one explicit
  "Report generated at" timestamp (spec FR-006, SC-003).
- Zero network I/O at runtime (Principle IX, spec FR-022).
- No writes inside the input directory (Principle II, spec FR-003).
- Single output file — no side-cars (spec FR-002 post-clarification).
- Inputs bounded to ≤ 1 GB total / ≤ 200 MB per file (spec FR-025).
- Fully static binary, no libc coupling (Principle I).

**Scale/Scope**:

- 7 pt-stalk collector parsers × 2 supported pt-stalk versions = up to
  14 fixture sets under `testdata/` (one per version where the format
  differs; shared fixture where it does not).
- ~3 internal Go packages + 1 CLI entry point.
- ~3 rendered sections with ~9 distinct views total.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Evaluated against `.specify/memory/constitution.md` v1.0.0.

| # | Principle | Pre-design | Rationale |
|---|-----------|------------|-----------|
| I | Single Static Binary | ✅ PASS | `CGO_ENABLED=0`, Go-only, cross-compiled for four targets. |
| II | Read-Only Inputs | ✅ PASS | All input access via `os.Open` read-only; no tempfiles inside input tree; output path explicit via `-o`. |
| III | Graceful Degradation (NON-NEGOTIABLE) | ✅ PASS | Per-collector parsers isolated; each captures errors as `model.Diagnostic` rather than returning fatal; missing files → "data not available" banner (FR-007). `recover()` at the top of each collector parse ensures a panic there does not kill the whole report (but panics remain unusual — they're not expected paths). |
| IV | Deterministic Output | ✅ PASS | Golden-test determinism gate (FR-006, SC-003). Map iteration replaced with sorted keys at render boundary; float formatting via `strconv.FormatFloat` with fixed precision; IDs deterministic via ordinal counters; `GeneratedAt` is the sole declared non-deterministic field. |
| V | Self-Contained HTML | ✅ PASS | Chart JS library + CSS + fonts inlined via `//go:embed` and rendered into a single `<script>` / `<style>` block. Data payload embedded as JSON in a `<script type="application/json">`. No CDN references. |
| VI | Library-First | ✅ PASS | `parse/`, `model/`, `render/` are top-level packages with no `main` coupling. `cmd/my-gather/` is a thin orchestrator. Each package will carry godoc on every exported identifier per FR-024-era test. |
| VII | Typed Errors | ✅ PASS | Sentinels `ErrNotAPtStalkDir`, `ErrSizeBoundExceeded`, `ErrOutputExists` in `parse/` and `render/`; `ParseError` struct with `File`, `Location`, `Underlying error` for branchable parser failures (spec Diagnostic entity maps 1:1). |
| VIII | Reference Fixtures & Golden Tests | ✅ PASS | Fixtures under `testdata/example1/` and `testdata/example2/` drawn from `_references/examples/`; one golden per supported version per collector. Build fails if a parser is added without a fixture+golden (enforced by a `testdata_coverage_test.go` guard). |
| IX | Zero Network | ✅ PASS | `net/http`, `net`, and `net/url` (except `url.PathEscape`-style stdlib helpers) are banned in shipped code via a `go vet`-style linter check in CI. |
| X | Minimal Dependencies | ✅ PASS | Zero direct Go module dependencies. One vendored JavaScript asset, justified in `research.md`. |
| XI | Human Pressure Optimization | ✅ PASS | Section order in report hardcoded to OS → Variables → DB (spec FR-005); Parser Diagnostics panel is collapsible secondary section. No cluttering metric dumps. |
| XII | Pinned Go Version | ✅ PASS | `go.mod` pins `go 1.24`; CI matrix uses exactly that version. |

**Pre-design gate: PASS.** No violations requiring Complexity Tracking.

*(Post-design re-evaluation appears at the bottom of this document.)*

## Project Structure

### Documentation (this feature)

```text
specs/001-ptstalk-report-mvp/
├── plan.md                 # This file
├── research.md             # Phase 0 output
├── data-model.md           # Phase 1 output
├── quickstart.md           # Phase 1 output
├── contracts/              # Phase 1 output
│   ├── cli.md              # CLI surface contract
│   └── packages.md         # parse/model/render public-API contracts
├── checklists/
│   └── requirements.md     # From /speckit.specify
└── tasks.md                # Phase 2 output (NOT created here)
```

### Source Code (repository root)

```text
cmd/
└── my-gather/
    └── main.go             # Thin CLI wrapper over parse/model/render

parse/
├── parse.go                # Collection / Snapshot discovery; size-bound check
├── errors.go               # ErrNotAPtStalkDir, ErrSizeBoundExceeded, ParseError
├── version.go              # Per-collector format-version detection
├── iostat.go
├── iostat_test.go
├── top.go
├── top_test.go
├── variables.go
├── variables_test.go
├── vmstat.go
├── vmstat_test.go
├── innodbstatus.go
├── innodbstatus_test.go
├── mysqladmin.go
├── mysqladmin_test.go
├── processlist.go
└── processlist_test.go

model/
├── model.go                # Collection, Snapshot, SourceFile, Sample, MetricSeries,
│                           # VariableEntry, ThreadStateSample, Diagnostic, Report
└── sections.go             # OSSection, VariablesSection, DBSection typed shapes

render/
├── render.go               # Render(w, report) orchestrator
├── deterministic.go        # Sorted-keys helpers, fixed-precision formatters
├── assets.go               # //go:embed directives
├── templates/
│   ├── report.html.tmpl
│   ├── os.html.tmpl
│   ├── variables.html.tmpl
│   └── db.html.tmpl
└── assets/
    ├── chart.min.js        # Vendored chart library (decision in research.md)
    ├── app.js              # Small glue script: mysqladmin toggle UI, variables
    │                       # search box, and collapse-state persistence (FR-032)
    └── app.css             # Including sticky nav rail styles (FR-031)

testdata/
├── example1/               # Anonymised subset of _references/examples/example1
│   └── ...
├── example2/               # Anonymised subset of _references/examples/example2
│   └── ...
├── pt-mext/                # Worked example from pt-mext-improved.cpp tail
│   ├── input.txt           # verbatim copy of comment lines 146-177
│   └── expected.txt        # verbatim copy of comment lines 181-189
└── golden/
    ├── example1.report.html
    ├── example1.model.json
    ├── example2.report.html
    └── example2.model.json

.github/
└── workflows/
    └── ci.yml              # go test, determinism check, size-bound check,
                            # cross-compile matrix, network-import linter

Makefile                    # build / test / cross-compile targets
go.mod                      # pins Go version; no non-stdlib dependencies
.gitignore                  # already exists
README.md                   # Developer + user-facing docs (created in implement phase)
CLAUDE.md                   # Agent guidance; updated by /speckit.plan to reference this plan
```

**Structure Decision**: Single-project Go CLI layout with three top-level
library packages (`parse/`, `model/`, `render/`) and a thin
`cmd/my-gather/` entry point, matching Constitution Principle VI
(library-first). Tests live beside their source in the conventional Go
style (`_test.go` files); fixtures live under a top-level `testdata/`
directory so they are automatically excluded from `go build` output but
are available to `go test`. No `internal/` directory is needed in v1 —
every package we create is intentionally library-first; if a later
feature needs internal-only code it can be added then.

## Complexity Tracking

*No Constitution violations. Table intentionally empty.*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|--------------------------------------|
| — | — | — |

## Phase 0: Outline & Research

Research artifacts are captured in [`research.md`](./research.md). Items
resolved there:

1. **Which time-series chart library do we vendor?** → decision + size
   + license + fallback behaviour.
2. **How do we detect pt-stalk format version per file?** → the
   heuristic used by each parser.
3. **How do we enforce byte-deterministic HTML?** → concrete rules for
   map iteration, float formatting, ID generation, template ordering.
4. **How do we implement the interactive mysqladmin toggle UI offline?**
   → data-embedding approach + client-side state shape.
5. **How do we detect a directory is not pt-stalk?** → signature files
   and suffix matching.
6. **Fixture anonymisation strategy.** → what's redacted from
   `_references/examples/` when committing to `testdata/`.
7. **Forbidden-import linter for Principle IX.** → stdlib-only check
   run as part of `go test ./...`.
8. **`-mysqladmin` delta-computation algorithm** → port the pt-mext
   algorithm (`_references/pt-mext/pt-mext-improved.cpp`) to native
   Go with three documented improvements (counter/gauge
   classification, float64 aggregates, drift diagnostics). The C++
   file's tail worked example is promoted to a committed golden
   fixture under `testdata/pt-mext/`. No parity test, no C++
   toolchain requirement.
9. **Navigation index and collapsible sections** → `<nav>` rendered
   server-side from a deterministic `Report.Navigation`, `position:
   sticky` for on-scroll visibility, native `<details>`/`<summary>`
   for collapse, `localStorage` keyed by `Report.ReportID` (a
   deterministic hash of the canonicalised Collection) for persisting
   open/closed state across reloads. Degrades gracefully without
   JavaScript.

Output: `research.md` with all open questions resolved.

## Phase 1: Design & Contracts

Artifacts produced:

1. **`data-model.md`** — typed entities derived from the spec's Key
   Entities section, with Go struct sketches, invariants, validation
   rules, and lifecycle notes.
2. **`contracts/cli.md`** — CLI flag surface, positional arguments,
   exit-code table, stdout/stderr contract, version output format.
3. **`contracts/packages.md`** — the public API of `parse/`, `model/`,
   `render/`: exported types, functions, and error sentinels, with
   godoc-style contracts.
4. **`quickstart.md`** — developer setup: clone, Go install, test,
   build, run against `_references/examples/example2`, open the report.
5. **Agent context update** — `CLAUDE.md` is updated to point to
   this plan between `<!-- SPECKIT START -->` and `<!-- SPECKIT END -->`
   markers.

## Post-Design Constitution Re-Check

Re-evaluated after completing Phase 1 artifacts (see `data-model.md`,
`contracts/cli.md`, `contracts/packages.md`).

| # | Principle | Post-design | Notes |
|---|-----------|-------------|-------|
| I | Single Static Binary | ✅ PASS | Build section of `Makefile` will set `CGO_ENABLED=0`; Go `net` import surface limited to `net/url` helpers (path escaping) if used at all. |
| II | Read-Only Inputs | ✅ PASS | `parse.Discover` contract explicitly forbids writes; output path is always the user's `-o` value or default `report.html` in CWD. |
| III | Graceful Degradation | ✅ PASS | `parse.Discover` returns a Collection even when some collectors fail; `render.Render` handles nil sections by emitting the banner. |
| IV | Deterministic Output | ✅ PASS | `render/deterministic.go` centralises sorted-keys + fixed-precision formatters; `render.Render` computes element IDs from an ordinal counter seeded by section iteration order. |
| V | Self-Contained HTML | ✅ PASS | `render/assets/` + `//go:embed` pattern enumerated in `research.md`. |
| VI | Library-First | ✅ PASS | Public API in `contracts/packages.md` has `parse`, `model`, `render` as separate importable packages; `cmd/my-gather` exists only to wire them. |
| VII | Typed Errors | ✅ PASS | Sentinels + `ParseError` enumerated in `contracts/packages.md`. |
| VIII | Reference Fixtures & Golden Tests | ✅ PASS | `testdata/example{1,2}/` structure locked in Project Structure; golden tests per collector; `testdata_coverage_test.go` guard documented in tasks. |
| IX | Zero Network | ✅ PASS | CI linter will reject any import of `net/http`, `net/rpc`, etc. in shipped packages. |
| X | Minimal Dependencies | ✅ PASS | `go.mod` has zero direct non-stdlib dependencies. The one vendored JS asset is justified in `research.md` with size and license. |
| XI | Human Pressure Optimization | ✅ PASS | Template order locked; Parser Diagnostics rendered as a collapsible `<details>` section at the bottom, not between primary views. |
| XII | Pinned Go Version | ✅ PASS | `go 1.24` directive in `go.mod`; CI uses matching version. |

**Post-design gate: PASS.** Ready to emit Phase 2 tasks via
`/speckit.tasks`.
