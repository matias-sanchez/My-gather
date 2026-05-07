# Tasks: Remove Collection-Size Cap

**Feature directory**: `specs/016-remove-collection-size-cap/`
**Branch**: `016-remove-collection-size-cap`

This feature is a focused, low-blast-radius refactor: delete the
historical 1 GiB total-collection refusal path everywhere it lives, and
add a regression test that pins the streaming-not-buffering invariant.
No new product surface, no parser-internal refactor (the audit confirmed
streaming).

## Phase 1: Setup

(none — no scaffolding required, all work is in existing files)

## Phase 2: Foundational

(none — no shared prerequisite blocks the user stories)

## Phase 3: User Story 1 — Engineer Parses a >1 GiB Real-World Capture (P1)

**Goal**: A previously-rejected >1 GiB capture is parsed without the
cap-exceeded error path.

**Independent test**: After this phase, `go vet ./...` and `go build
./...` succeed. Manually running `my-gather` against any real >1 GiB
capture exits 0 instead of `exitSizeBound`.

- [X] T001 [US1] Remove the total-collection cap from `parse/parse.go`:
      delete `DefaultMaxCollectionBytes`, delete the
      `MaxCollectionBytes` field from `DiscoverOptions` (and its godoc),
      drop the `opts.MaxCollectionBytes < 0` guard half (keep the
      `MaxFileBytes < 0` half), drop the `maxCollection := ...` /
      `if maxCollection == 0 { maxCollection = ... }` block, drop the
      `if totalBytes > maxCollection { return nil, &SizeError{Kind:
      SizeErrorTotal, ...} }` block, and update the godoc on `Discover`
      and `DiscoverOptions` so they no longer reference the total-size
      bound. (Keep `totalBytes` only if still used; remove if dead.)
- [X] T002 [US1] Remove `SizeErrorTotal` from `parse/errors.go`: delete
      the enum constant, delete its godoc, delete its `case` arm in
      `SizeError.Error()`, and update the `SizeErrorKind` godoc and
      `SizeErrorFile`'s comment so it stands alone (renumber if
      necessary by re-anchoring `iota + 1`).
- [X] T003 [US1] Remove the `SizeErrorTotal` arm from
      `cmd/my-gather/main.go` `mapDiscoverError`: delete the `case
      parse.SizeErrorTotal:` block. Keep the `case parse.SizeErrorFile:`
      block. Update any godoc on `mapDiscoverError` that mentions the
      total-collection branch.
- [X] T003a [US1] Update `cmd/my-gather/archive_input.go`: replace
      `const maxArchiveExtractedBytes = parse.DefaultMaxCollectionBytes`
      with a local 64 GiB ceiling (rationale in research R5);
      replace the `&parse.SizeError{Kind: parse.SizeErrorTotal, ...}`
      construction inside `writeExtractedFileWithLimits` with a new
      local typed error `archiveExtractedSizeError`; add an
      `errors.As` branch in `mapInputPreparationError`
      (`cmd/my-gather/main.go`) that prints a one-line message and
      returns `exitSizeBound`.
- [X] T004 [US1] Sweep the test files: in `parse/parse_test.go` (and
      any other `parse/*_test.go`), remove or rewrite tests that
      asserted the cap-exceeded behaviour. Tests that exercise the
      per-file `SizeErrorFile` path stay. Verify with
      `go test ./parse/...` after the sweep.

**Checkpoint**: After T001–T004, `go vet ./...` and
`go test ./...` are green and no occurrence of `MaxCollection`,
`SizeErrorTotal`, or `DefaultMaxCollectionBytes` remains in `parse/`,
`model/`, `cmd/`, `render/`, `findings/`, or `reportutil/`. The
existing per-file size error path still works.

## Phase 4: User Story 2 — Large Captures Stream Without OOM (P1)

**Goal**: Pin the streaming invariant by measurement so a future
refactor that quietly slurps a whole file gets caught.

**Independent test**: A new regression test parses a >1.1 GiB synthetic
capture and asserts (a) no `*SizeError` of any kind is returned, and
(b) peak heap delta during the call stays under 256 MiB.

- [X] T005 [P] [US2] Add `parse/streaming_large_test.go`:
      a `TestDiscoverStreamingLargeCollection` function that
      - skips when `testing.Short()` is set,
      - creates a `t.TempDir()`,
      - writes one synthetic `<prefix>-iostat`, `<prefix>-vmstat`,
        `<prefix>-mysqladmin`, `<prefix>-meminfo`, `<prefix>-hostname`,
        and `<prefix>-top` file at the same valid pt-stalk timestamp
        prefix (e.g. `2026_05_07_12_00_00`), keeping each individual
        file under `parse.DefaultMaxFileBytes` (200 MiB) and the total
        > 1.1 GiB by writing realistic-ish per-collector lines
        chunk-by-chunk so the writer itself never holds the whole file
        in memory,
      - runs `runtime.GC()` and reads `runtime.MemStats.HeapAlloc`
        before calling `parse.Discover(ctx, dir, parse.DiscoverOptions{
        MaxFileBytes: parse.DefaultMaxFileBytes })`,
      - asserts no error,
      - asserts the returned `*model.Collection` has at least one
        snapshot,
      - runs `runtime.GC()` again and reads `runtime.MemStats.HeapAlloc`
        after the call, asserting the delta is `< 256 << 20`,
      - reports the observed delta on failure for triage.
- [X] T006 [US2] Run `go test -count=1 ./parse/... -run
      TestDiscoverStreamingLargeCollection -v` locally and confirm it
      passes. If the threshold is too tight in practice, raise it in
      the test only after recording the observed delta in this tasks
      file (no constitution amendment needed; the threshold is a
      test-only ceiling).

**Checkpoint**: Streaming regression is in place and green.

## Phase 5: Polish & Cross-Cutting Concerns

- [X] T007 Update `CLAUDE.md`, `AGENTS.md`, and `.specify/feature.json`
      to point at `016-remove-collection-size-cap` (already done in the
      plan step; re-verify before commit).
- [X] T008 Run the full test sweep: `go vet ./...`, `go test -count=1
      ./...`, and the constitution guard
      `bash scripts/hooks/pre-push-constitution-guard.sh`.
- [X] T009 Grep audit:
      `grep -rn "MaxCollectionBytes\|SizeErrorTotal\|DefaultMaxCollectionBytes" parse/ model/ cmd/ render/ findings/ reportutil/`
      MUST return zero matches.
- [X] T010 `git add` only the touched files
      (`specs/016-remove-collection-size-cap/`, `parse/parse.go`,
      `parse/errors.go`, `parse/parse_test.go`,
      `parse/streaming_large_test.go`, `cmd/my-gather/main.go`,
      `CLAUDE.md`, `AGENTS.md`, `.specify/feature.json`). Commit with a
      message referencing issue #50. Push and open the PR.

## Dependencies

- T001 → T002 → T003 → T004 (sequential within US1; same package and
  same test sweep).
- T005 depends on T001–T003 being landed in the same branch (the test
  must compile against the new `DiscoverOptions` shape).
- T006 depends on T005.
- T007–T010 depend on US1 + US2.

## Parallel execution opportunities

- T005 [P] is the only meaningful parallel candidate; in practice all
  changes are co-located in one small diff and one author, so parallel
  execution is not useful here.

## MVP scope

US1 alone makes the user-visible bug go away. US2 is necessary to
ship the change responsibly and is in-scope for this PR per the issue
description and Constitution Principle XIII (the canonical streaming
path must be pinned by test, not just by audit).
