---

description: "Task list for feature 023-nested-root-discovery"
---

# Tasks: Nested Root Discovery for Directory Inputs

**Input**: Design documents from `/specs/023-nested-root-discovery/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md,
contracts/discovery.md, quickstart.md

**Tests**: Test tasks are mandatory for this feature. The contract
document `contracts/discovery.md` lists 11 specific test names that
must exist; each is a discrete task here. Article II of the
constitution requires tests precede implementation in this list.

**Organization**: Tasks are grouped by user story so each story can be
implemented and validated independently.

## Canonical Path Metadata *(Principle XIII)*

- **Canonical owner/path**: `parse.FindPtStalkRoot` in
  `parse/discover_root.go` is the single canonical implementation that
  walks a directory tree to locate a pt-stalk root. Both directory-
  input and archive-input call sites in
  `cmd/my-gather/archive_input.go` route through it.
- **Old path treatment**: deleted by tasks T013 and T014 -
  `cmd/my-gather/archive_input.go::findExtractedPtStalkRoot` and
  `cmd/my-gather/archive_input.go::multiplePtStalkRootsError` are
  removed in the same change.
- **External degradation evidence**: zero-root and multi-root
  outcomes are typed errors, exercised by tasks T002, T003, T009,
  T010 (parse-level) and T015, T016 (CLI end-to-end).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Parallelizable (different file, no in-flight dependency).
- **[Story]**: User story tag (US1 / US2 / US3). Setup, Foundational,
  and Polish phases carry no story tag.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Confirm the working tree, branch, and feature pointers
are aligned before any code change lands.

- [X] T000 Confirm branch is `023-nested-root-discovery` and
  `.specify/feature.json` points at `specs/023-nested-root-discovery`
  (already done by the speckit-specify hook; verify with
  `git branch --show-current` and `cat .specify/feature.json`).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Land the canonical types and constants the rest of the
work depends on. These tasks unblock both implementation and tests.

**CRITICAL**: All tasks in Phase 3+ depend on T001.

- [X] T001 Create `parse/discover_root.go` with the canonical
  exported surface only (no walker logic yet): the
  `parse.FindPtStalkRootOptions` struct, the
  `parse.MultiplePtStalkRootsError` struct with its `Error()`
  method, and the constants
  `parse.DefaultMaxRootSearchDepth = 8` and
  `parse.DefaultMaxRootSearchEntries = 100000`. Add godoc comments
  to every exported identifier (Principle VI). The file must
  declare `package parse` and live next to `parse/parse.go`.
  Implement `parse.FindPtStalkRoot(ctx, rootDir, opts) (string, error)`
  as a stub that returns `parse.ErrNotAPtStalkDir` so the package
  compiles; the stub will be replaced in T011.

**Checkpoint**: package `parse` compiles, the tests written in
Phase 3 below can fail meaningfully against the stub.

---

## Phase 3: User Story 1 - Find a single nested capture (Priority: P1) MVP

**Goal**: `my-gather <dir>` succeeds when the pt-stalk capture lives
in a subdirectory of `<dir>`. The top-level fast path remains
bit-identical to today.

**Independent Test**: build a synthetic six-level-deep fixture, run
the binary against the input root, observe exit 0 and a non-empty
output file.

### Tests for User Story 1 (write FIRST, ensure they FAIL)

- [X] T002 [P] [US1] In `parse/discover_root_test.go` add
  `TestFindPtStalkRoot_TopLevel`: build a tempdir containing one
  pt-stalk-shaped file at the top level; assert that
  `parse.FindPtStalkRoot(ctx, tempdir, parse.FindPtStalkRootOptions{})`
  returns the absolute path of `tempdir` with `err == nil`.
- [X] T003 [P] [US1] In `parse/discover_root_test.go` add
  `TestFindPtStalkRoot_NestedSingle`: build a tempdir containing
  one pt-stalk-shaped file at depth 6; assert that the function
  returns the absolute path of the depth-6 directory with
  `err == nil`.
- [X] T004 [P] [US1] In `parse/discover_root_test.go` add
  `TestFindPtStalkRoot_HiddenDirSkipped`: build a tempdir whose
  only pt-stalk-shaped file lives under `.cache/pt/...`; assert
  that the function returns `parse.ErrNotAPtStalkDir`.
- [X] T005 [P] [US1] In `parse/discover_root_test.go` add
  `TestFindPtStalkRoot_DepthCap`: build a tempdir with a
  pt-stalk-shaped file at depth 12; call `FindPtStalkRoot` with
  the default options (MaxDepth = 8); assert
  `parse.ErrNotAPtStalkDir`.
- [X] T006 [P] [US1] In `parse/discover_root_test.go` add
  `TestFindPtStalkRoot_SymlinkNotFollowed`: build a tempdir whose
  only pt-stalk-shaped file lives inside a directory reachable
  only via a symlink; assert `parse.ErrNotAPtStalkDir`. (Skip on
  platforms where symlink creation requires elevated permission.)
- [X] T006a [P] [US1] In `parse/discover_root_test.go` add
  `TestFindPtStalkRoot_PermDeniedTolerated`: build a tempdir
  containing one readable pt-stalk-shaped subdirectory and one
  unreadable (mode `0o000`) subdirectory at the same level. Run
  the function and assert it returns the readable root with
  `err == nil`, demonstrating that the unreadable subdir does not
  abort the walk. Skip on Windows where the permission model
  does not support `chmod 0o000`. Restore mode `0o700` in a
  `t.Cleanup` so `os.RemoveAll` succeeds.
- [X] T007 [P] [US1] In `cmd/my-gather/main_test.go` add
  `TestCLIDirInputNestedSingle`: build the same six-level-deep
  fixture as T003 (use `_references/examples/` content), run the
  binary in-process via the existing `runCLI` helper, assert exit
  0 and that the output file is non-empty.

### Implementation for User Story 1

- [X] T008 [P] [US1] In `parse/discover_root.go` implement the
  walker body of `parse.FindPtStalkRoot` so it (a) absolutises
  rootDir, (b) calls `parse.LooksLikePtStalkRoot` on every visited
  directory including rootDir itself, (c) uses `filepath.WalkDir`
  with a closure that increments an entry counter, computes depth
  from the relative path, returns `fs.SkipDir` for hidden dirs
  (`name[0] == '.'`) and for any directory at or beyond `MaxDepth`,
  tolerates non-root `walkErr` values by pruning unreadable
  directories with `fs.SkipDir` (or returning nil for non-directory
  entries), and appends matching directories to a slice. The
  function MUST keep walking after a match so an input that is
  itself a pt-stalk root and also contains a nested pt-stalk root
  surfaces as a multi-root ambiguity. Return `parse.ErrNotAPtStalkDir`
  when the slice is empty, the single match when length is 1, or a
  `*parse.MultiplePtStalkRootsError` (with `Roots` sorted
  lexically) when length is >= 2. Stop the walk when the entry
  counter reaches `MaxEntries` and report the result based on what
  was found.
- [X] T009 [US1] Update `cmd/my-gather/archive_input.go`:
  rewrite the `prepareInput` directory branch so that it calls
  `parse.FindPtStalkRoot(ctx, inputPath, parse.FindPtStalkRootOptions{})`
  using the existing `ctx` parameter on `prepareInput` (no
  signature change needed) and place the returned root in the
  `parseDir` field of the returned `*preparedInput`.
- [X] T010 [US1] In `parse/parse.go` update the
  "Sub-directories ... are not walked in v1" comment to point
  readers at `parse.FindPtStalkRoot` and clarify that
  `parse.Discover` itself remains single-root by design.

**Checkpoint**: `my-gather <case-folder>` succeeds against a
synthetic six-level-deep capture; the existing top-level invocation
is bit-identical.

---

## Phase 4: User Story 2 - Refuse and explain when several captures coexist (Priority: P1)

**Goal**: When the input tree contains more than one pt-stalk root,
the CLI exits non-zero, writes no output, and prints a deterministic,
lexically-ordered list of every discovered root.

**Independent Test**: build a synthetic input with two pt-stalk roots
in distinct subdirectories. Run the binary. Verify non-zero exit, no
output file, and the stderr block lists both roots in lexical order.

### Tests for User Story 2

- [X] T011 [P] [US2] In `parse/discover_root_test.go` add
  `TestFindPtStalkRoot_NestedMultiple`: build a tempdir with two
  pt-stalk-shaped roots in distinct subdirectories; assert
  `errors.As` extracts a `*parse.MultiplePtStalkRootsError` whose
  `Roots` field has length 2 and is sorted lexically.
- [X] T012 [P] [US2] In `parse/discover_root_test.go` add
  `TestFindPtStalkRoot_Deterministic`: invoke `FindPtStalkRoot`
  twice against the same multi-root fixture and compare the two
  resulting `Error()` strings byte-for-byte; assert equality.
- [X] T013 [P] [US2] In `cmd/my-gather/main_test.go` add
  `TestCLIDirInputMultipleRoots`: build a synthetic two-root
  fixture; run the binary; assert non-zero exit code, that no
  output file was written at the requested path, and that stderr
  contains both root paths on consecutive lines in lexical order.

### Implementation for User Story 2

- [X] T014 [US2] In `cmd/my-gather/archive_input.go` delete the
  internal `findExtractedPtStalkRoot` function and the internal
  `multiplePtStalkRootsError` type. Update the archive-extraction
  call site in `prepareInput` to call
  `parse.FindPtStalkRoot(ctx, tempDir, parse.FindPtStalkRootOptions{})`
  on the extracted directory. Add `import "errors"` if not
  already present and switch any local `errors.As` calls to
  branch on `*parse.MultiplePtStalkRootsError`.
- [X] T015 [US2] In `cmd/my-gather/main.go` update
  `mapInputPreparationError` so that the
  `*parse.MultiplePtStalkRootsError` branch (replacing the old
  internal type branch) prints the error's `Error()` output to
  stderr verbatim and returns the existing exit code. Confirm
  `errors.As` is used, not type assertion.

**Checkpoint**: a synthetic two-root fixture under
`my-gather <dir>` produces a deterministic lexically-ordered stderr
block and a non-zero exit code.

---

## Phase 5: User Story 3 - Refuse and explain when no capture exists (Priority: P1)

**Goal**: When the input tree contains no pt-stalk root, the CLI
exits non-zero, writes no output, and prints a stderr message that
makes it clear both the directory and its subdirectories were
searched, and names the depth limit.

**Independent Test**: invoke the binary on a directory of unrelated
files. Verify non-zero exit, no output file, and a stderr message
explicitly mentioning "subdirectories were searched" and the depth.

### Tests for User Story 3

- [X] T016 [P] [US3] In `parse/discover_root_test.go` add
  `TestFindPtStalkRoot_NoRoot`: build a tempdir containing only
  unrelated files (no pt-stalk-shaped file at any depth). Assert
  the function returns `parse.ErrNotAPtStalkDir`.
- [X] T017 [P] [US3] In `cmd/my-gather/main_test.go` add
  `TestCLIDirInputNoRoot`: build a tempdir of unrelated files,
  run the binary, assert non-zero exit code, no output file,
  and that stderr contains the substring "subdirectories were
  searched" and the depth-limit number.

### Implementation for User Story 3

- [X] T018 [US3] In `cmd/my-gather/main.go` update the
  `parse.ErrNotAPtStalkDir` branch of `mapInputPreparationError`
  / `mapDiscoverError` (whichever surfaces from `prepareInput`'s
  new path) to print the wording specified in
  `research.md::Decision 8`:
  `"my-gather: <input-path> is not a pt-stalk output directory and no pt-stalk collection was found in its subdirectories (searched up to depth 8)"`.
  Use the actual depth value from
  `parse.DefaultMaxRootSearchDepth` so the message stays in sync
  if the constant changes.

**Checkpoint**: invoking the binary on an unrelated directory
produces the new wording.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T019 [P] Update `cmd/my-gather/main.go::usageText`. The
  `<input>` line under ARGUMENTS becomes:
  `Path to a pt-stalk output directory (or a directory that contains one, searched up to 8 levels) or a supported archive (.zip, .tar, .tar.gz, .tgz, .gz).`
  Match the wording in
  `specs/023-nested-root-discovery/contracts/discovery.md::CLI contract::--help text`.
- [X] T020 [P] Update `specs/001-ptstalk-report-mvp/contracts/cli.md`
  so the directory-input description and any matching `--help`
  excerpt reflect auto-descent. Cross-reference
  `specs/023-nested-root-discovery/contracts/discovery.md`. Do not
  alter exit codes; they are unchanged.
- [X] T021 Run the full Go test suite locally
  (`go test ./...`) and the existing constitution pre-push guard
  (`scripts/hooks/pre-push-constitution-guard.sh` against the
  staged diff). Both must pass; if Principle XIII review flags any
  duplicate walker, return to T014. The full `go test ./...` run
  also exercises every existing top-level golden test, so any
  byte-drift in the existing-already-a-root fast path
  (Principle IV / SC-003) fails here.
- [X] T022 Canonical-path audit: verify on the staged diff that
  (a) `parse.FindPtStalkRoot` is the only function in the repo
  that walks a directory tree to find a pt-stalk root,
  (b) `cmd/my-gather/archive_input.go::findExtractedPtStalkRoot`
  is gone, (c) `cmd/my-gather/archive_input.go::multiplePtStalkRootsError`
  is gone, (d) the CLI directory branch uses
  `parse.FindPtStalkRoot`, and (e) the CLI archive branch uses
  `parse.FindPtStalkRoot`. Record this audit finding in the PR
  description.
- [X] T023 Run the quickstart spot check listed in
  `specs/023-nested-root-discovery/quickstart.md::How to verify
  locally`: invoke the binary against a deeply-nested local
  capture (the `CS0061420` example) and confirm the multi-root
  stderr block names all three host roots in lexical order.

---

## Dependencies & Execution Order

### Phase Dependencies

- Setup (Phase 1) -> Foundational (Phase 2) -> User stories (Phases
  3-5) -> Polish (Phase 6).
- T001 (Foundational) blocks every test task and every
  implementation task in Phases 3-5 because the package must
  compile against the new exported surface before tests can run.
- Within Phases 3, 4, 5: tests come before their implementation
  tasks (Article II).

### User Story Dependencies

- US1, US2, US3 are independently completable. The shared walker
  body (T008) is logically inside US1 because it ships the
  single-nested case; US2 and US3 depend on T008 being merged
  (or at least staged in the same branch) before their CLI tests
  can pass.
- Phase 6 depends on all of US1, US2, US3 being implemented.

### Within Each User Story

- All test tasks marked [P] within a story can be authored in
  parallel because they touch the same file at distinct test
  function names; serialise the writes if the same file is being
  edited in one IDE session, but the work itself is parallel.
- Implementation tasks within a story are sequential where they
  touch shared files (e.g., T009 and T014 both edit
  `cmd/my-gather/archive_input.go`).

### Parallel Opportunities

- T002, T003, T004, T005, T006, T007 (US1 tests): same file
  except T007 (different file). T007 is parallel with the others.
- T011, T012, T013: T011 and T012 share `parse/discover_root_test.go`
  with the US1 tests; T013 is in `cmd/my-gather/main_test.go`.
  T013 is parallel with T011/T012.
- T016, T017: T016 shares `parse/discover_root_test.go`; T017
  shares `cmd/my-gather/main_test.go`. T017 is parallel with T016.
- T019, T020: different files, fully parallel.

---

## Parallel Example: User Story 1 tests

```bash
# All US1 test functions can be authored in one editor session,
# touching parse/discover_root_test.go (T002, T003, T004, T005, T006)
# and cmd/my-gather/main_test.go (T007). Run:
go test ./parse/... -run TestFindPtStalkRoot_ -v
go test ./cmd/my-gather/... -run TestCLIDirInputNestedSingle -v
```

---

## Implementation Strategy

### MVP First (User Story 1 only)

1. T000 (verify branch).
2. T001 (canonical surface).
3. T002-T007 (US1 tests authored, must FAIL).
4. T008-T010 (US1 implementation; tests now PASS).
5. STOP and validate manually against a real-world local capture
   (one of the deeply-nested ones from the local corpus).
6. Commit and continue to US2.

### Incremental Delivery

- After US1: directory inputs auto-descend for the single-root case.
- After US2: multi-root inputs produce the deterministic stderr
  block.
- After US3: zero-root inputs produce the new wording.
- After Phase 6: help text and external CLI contract document are in
  sync with implementation.

### Rollback

If a regression appears after merge (e.g. a top-level invocation
behaves differently), the CLI fast path in T009 isolates the change:
the regression must be in T008's walker logic, and the existing
top-level golden test catches it before merge.

---

## Notes

- [P] tasks = different files, no in-flight dependency.
- [Story] tag traces each task to its user story for reviewer audit.
- All US tasks are P1; the priority is uniform because the three
  outcomes (single, multi, zero) form one coherent contract and
  shipping any subset would leave the others' callers in an
  inconsistent state.
- Verify each test FAILS before its implementation task runs;
  otherwise the test was not actually exercising the new path.
- Commit after each numbered task or after a tight group (e.g. all
  US1 tests in one commit, all US1 implementation in the next).
