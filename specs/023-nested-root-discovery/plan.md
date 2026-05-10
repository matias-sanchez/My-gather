# Implementation Plan: Nested Root Discovery for Directory Inputs

**Branch**: `023-nested-root-discovery` | **Date**: 2026-05-08 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/023-nested-root-discovery/spec.md`

## Summary

Make `my-gather <directory>` discover the pt-stalk capture even when it lives
in a nested subdirectory of the directory the user passed. Today only the
archive-input path walks subdirectories to find the capture; directory-input
silently bypasses that walk and returns "not recognised" if the capture is
not at the top level. The plan is to lift the existing
`findExtractedPtStalkRoot` walker out of `cmd/my-gather/archive_input.go`
into the `parse/` package as the single canonical root-discovery function,
gate it on a depth bound (8 levels), skip hidden dirs, ignore symlinked
subdirectories, tolerate per-subdir read errors, and route both directory
input and archive input through it. The new function returns one of three
typed outcomes: a single root path, a multiple-roots error listing the
discovered paths in lexical order, or `parse.ErrNotAPtStalkDir`. The CLI
contract and `--help` text are updated to reflect the new directory-input
behaviour.

## Technical Context

**Language/Version**: Go (pinned in `go.mod` per Principle XII; current
toolchain at the time of writing).
**Primary Dependencies**: standard library only (`os`, `path/filepath`,
`errors`, `sort`). No new direct dependency is added (Principle X).
**Storage**: N/A - read-only filesystem access against the user-supplied
input tree (Principle II).
**Testing**: `go test ./...` (matrix CI job) plus the project's existing
golden-test discipline (Principle VIII). New tests live in `parse/` and
`cmd/my-gather/` next to the code they cover.
**Target Platform**: `linux/amd64`, `linux/arm64`, `darwin/amd64`,
`darwin/arm64` static binaries (Principle I).
**Project Type**: Single-binary CLI with library-first packages
(`parse/`, `model/`, `render/`); `cmd/my-gather/` is the thin CLI
wrapper.
**Performance Goals**: Directory walk completes in under 2 seconds on the
largest local capture (a 397M tree containing one root nested 3
levels deep). The walk only enumerates entries - it does not read
snapshot file contents - so the cost is dominated by directory entry
count, not file size.
**Constraints**: Depth cap = 8 levels below the input root; total
scanned-entry cap = 100000 entries (belt-and-braces guard against
pathological trees, well above any real-world corpus). Hidden
directories (name starts with `.`) are skipped. Symlinked
subdirectories are not followed. Per-subdirectory read errors are
swallowed; the walk continues. All output is byte-deterministic
(Principle IV).
**Scale/Scope**: Local-corpus deepest layout is 6 directories below the
input root the user typically points at; 8-level cap gives 2 levels of
headroom while still preventing worst-case walks on huge unrelated trees.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

The constitution at `.specify/memory/constitution.md` v1.6.2 has 15
principles. Audit per principle:

- **I. Single Static Binary** - No effect. Discovery runs in the same
  binary; no new linkage, no CGO. PASS.
- **II. Read-Only Inputs** - The discovery walk is `os.ReadDir` /
  `filepath.WalkDir`-equivalent only. No `os.Create`, `os.Mkdir`,
  `os.Rename`, `os.Remove`, `os.WriteFile`, or `ioutil.WriteFile`
  calls land anywhere in the search code. The pre-push hook's
  Principle II grep on `parse/` and `cmd/` keeps this honest. PASS.
- **III. Graceful Degradation (NON-NEGOTIABLE)** - Per-subdir read
  errors are tolerated; the walk continues. The "no roots found" and
  "multiple roots found" outcomes surface as typed errors with
  human-readable messages, not panics. PASS.
- **IV. Deterministic Output** - Discovered roots are sorted with
  `sort.Strings` before being returned (single-root case) or before
  being formatted into the multiple-roots error message. Two runs
  against the same tree produce byte-identical stderr. The existing
  golden-test on the top-level fast path confirms zero behavioural
  change for inputs that already are pt-stalk roots. PASS.
- **V. Self-Contained HTML Reports** - N/A; this feature does not
  touch rendering.
- **VI. Library-First Architecture** - The canonical discovery
  function lives in `parse/` (the existing parse package), not in
  `cmd/my-gather/`. Exported, godoc-commented, callable by external
  code without depending on `main`. PASS.
- **VII. Typed Errors** - Two typed errors used: existing
  `parse.ErrNotAPtStalkDir` for the zero-root case; a new exported
  `parse.MultiplePtStalkRootsError` (struct with `Roots []string`
  and an `Error()` method) for the multi-root case. Callers branch
  via `errors.Is` / `errors.As`. The current internal
  `cmd/my-gather/archive_input.go::multiplePtStalkRootsError` is
  promoted to this exported type and the old internal type is
  deleted. PASS.
- **VIII. Reference Fixtures & Golden Tests** - Discovery does not
  parse new collector formats, so no new fixture/golden pair is
  required by the gate. The directory-input top-level fast-path
  golden continues to apply unchanged. PASS.
- **IX. Zero Network at Runtime** - No network surface added; the
  pre-push hook will continue to flag any `net/http` import. PASS.
- **X. Minimal Dependencies** - Standard library only. No `go.mod`
  changes. PASS.
- **XI. Reports Optimized for Humans Under Pressure** - N/A; this
  feature does not change report content.
- **XII. Pinned Go Version** - No toolchain change. PASS.
- **XIII. Canonical Code Path (NON-NEGOTIABLE)** - This is the
  load-bearing principle for the feature. The existing private
  `findExtractedPtStalkRoot` in `cmd/my-gather/archive_input.go` is
  promoted to a single exported function in `parse/` and is the
  only place in the repo that walks a directory tree to locate a
  pt-stalk root. The directory-input code path is rewritten to call
  it; the archive-input code path is rewritten to call it. The
  internal helper is deleted in the same change. The "Sub-directories
  are not walked in v1" comment in `parse/parse.go` is updated to
  point at the new function so the rule is documented in one place.
  PASS by construction.
- **XIV. English-Only Durable Artifacts** - All artifacts in this
  feature are English. No accented Latin letters in code, comments,
  spec, plan, tasks, contracts, quickstart, error messages, or help
  text. PASS.
- **XV. Bounded Source File Size** - The new function is well under
  1000 lines (a few dozen). The two files that change
  (`parse/parse.go`, `cmd/my-gather/archive_input.go`) are well below
  the cap today; the change reduces the latter (lines move out) and
  adds modest weight to the former. PASS.

**Canonical Path Audit (Principle XIII)**:

- **Canonical owner/path for touched behaviour**: a new function in
  `parse/discover_root.go`, exported as
  `parse.FindPtStalkRoot(ctx, rootDir, opts)`. Both directory-input
  preparation in `cmd/my-gather/archive_input.go::prepareInput` and
  archive-input preparation in the same file route through this
  function.
- **Replaced or retired paths**:
  - `cmd/my-gather/archive_input.go::findExtractedPtStalkRoot` -
    deleted in the same change; logic moves to
    `parse.FindPtStalkRoot`.
  - `cmd/my-gather/archive_input.go::multiplePtStalkRootsError` -
    deleted; replaced by the exported
    `parse.MultiplePtStalkRootsError` used by both call sites. The
    CLI's user-facing error mapping continues to call `errors.As`
    on this type.
  - `parse/parse.go` "Sub-directories ... are not walked in v1"
    comment - rewritten to point at `FindPtStalkRoot` and to
    clarify that `parse.Discover` itself remains single-root by
    design.
- **External degradation paths, if any**: zero-root -> typed
  `ErrNotAPtStalkDir`; multi-root -> typed
  `MultiplePtStalkRootsError`; per-subdir read failure -> walk
  continues, single-root outcome unaffected (silent skip is the
  canonical handling here, not a hidden fallback to a parallel
  implementation). All three are observable via stderr+exit-code,
  covered by tests in `parse/discover_root_test.go` and end-to-end
  tests in `cmd/my-gather/main_test.go`.
- **Review check**: reviewers verify on the diff that (a) only one
  exported `FindPtStalkRoot`-shaped function exists, (b) both call
  sites use it, (c) no internal `multiplePtStalkRootsError` /
  `findExtractedPtStalkRoot` remains, (d) the comment in
  `parse/parse.go` is updated, and (e) the CLI contract document
  reflects directory-input auto-descent.

No constitution violations to track in Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/023-nested-root-discovery/
├── plan.md              # This file
├── spec.md              # Already written
├── research.md          # Phase 0 output (this command)
├── data-model.md        # Phase 1 output (this command)
├── quickstart.md        # Phase 1 output (this command)
├── contracts/
│   └── discovery.md     # Phase 1 output
├── checklists/
│   └── requirements.md  # Already written
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
parse/
├── parse.go                       # Existing; comment near the "Sub-directories not walked" note updated to point at FindPtStalkRoot
├── discover_root.go               # NEW: exports FindPtStalkRoot, MultiplePtStalkRootsError, FindPtStalkRootOptions, default depth/entry caps
├── discover_root_test.go          # NEW: unit tests for the canonical discovery function
└── errors.go                      # Existing; ErrNotAPtStalkDir reused, no change

cmd/my-gather/
├── archive_input.go               # MODIFIED: prepareInput uses parse.FindPtStalkRoot for both branches; the internal findExtractedPtStalkRoot and multiplePtStalkRootsError are deleted
├── main.go                        # MODIFIED: mapInputPreparationError uses errors.As for parse.MultiplePtStalkRootsError, and the no-root path message reflects "subdirectories were searched"
└── main_test.go                   # MODIFIED: new end-to-end tests for nested directory inputs

specs/001-ptstalk-report-mvp/
└── contracts/cli.md               # MODIFIED: directory-input description reflects auto-descent
```

**Structure Decision**: The feature lands in the existing repository
layout. No new top-level packages or directories are introduced. The
file `parse/discover_root.go` is the single canonical home for the
walker; the rest of the change is call-site rewires and contract
updates.

## Phase 0: Research

Captured separately in [research.md](./research.md). Summary of
decisions:

- Where the canonical walker lives: in the `parse/` package, not in
  `cmd/my-gather/`. Reason: it is reusable library logic by
  definition (Principle VI) and both call sites - directory and
  archive - need it.
- Walker mechanism: `filepath.WalkDir` with a `WalkDirFunc` that calls
  `parse.LooksLikePtStalkRoot` per directory and accumulates matches.
  The depth bound and hidden-dir skip are implemented by returning
  `filepath.SkipDir` from the walk function. Symlinks are not
  followed because `WalkDir` does not follow them by default.
  Per-entry read errors propagate to the walk function's `walkErr`
  argument and are swallowed (return nil) so the walk continues.
- No early return: `FindPtStalkRoot` always walks the bounded
  subtree of `rootDir`. The recognition predicate fires on every
  visited directory including `rootDir` itself, and the walk does
  not stop after a match. This is what makes the multi-root
  ambiguity surface in the case where the input is itself a
  pt-stalk root AND has a nested second root below (Codex round-3
  regression on PR #64).
- Error type: `parse.MultiplePtStalkRootsError` is an exported
  struct with a `Roots []string` field (sorted lexically) and an
  `Error()` method that formats them on multiple lines with a final
  "re-run pointing at one of these paths" hint. The CLI's error
  mapper unwraps it via `errors.As` and prints the message to
  stderr.
- "No roots found" message: includes both the input directory path
  and the explicit phrase "subdirectories were searched up to depth
  N" so the engineer knows the answer is "the capture is not in
  this tree at all," not "the tool only looks at the top level."

## Phase 1: Design & Contracts

- [data-model.md](./data-model.md) - describes the
  `FindPtStalkRootOptions`, the discovery outcome (single root
  string, MultiplePtStalkRootsError, or ErrNotAPtStalkDir), and the
  invariants on each.
- [contracts/discovery.md](./contracts/discovery.md) - Go-level
  contract for `parse.FindPtStalkRoot` and CLI-level behavioural
  contract for the directory-input path including help text and
  exit-code semantics.
- [quickstart.md](./quickstart.md) - operator-facing walkthrough:
  the new directory layouts that now work, the multi-root error
  message format, the no-root error message format, the `--help`
  text update, and a one-liner that exercises a synthetic
  six-level-deep fixture.
- Agent context update: `AGENTS.md`, `CLAUDE.md`, and
  `.specify/feature.json` point at `specs/023-nested-root-discovery`
  (`feature.json` is already done in Phase 0).

## Complexity Tracking

No constitution violations to track. The feature is a refactor that
*reduces* complexity by collapsing two near-identical walks into one
canonical function.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| _none_    | _n/a_      | _n/a_                                |
