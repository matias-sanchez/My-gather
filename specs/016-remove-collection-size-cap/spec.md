# Feature Specification: Remove Collection-Size Cap

**Feature Branch**: `016-remove-collection-size-cap`
**Created**: 2026-05-07
**Status**: Ready for review
**Input**: User description: "Remove the hard-coded 1 GiB collection-size cap that currently rejects pt-stalk inputs larger than 1073741824 bytes. This must be a true cap removal — not a higher default and not a configurable flag. Audit downstream parser stages and refactor any whole-input buffering to streaming. Add a regression test that exercises a >1.1 GiB synthetic input and asserts (a) parsing succeeds without the cap-exceeded error and (b) memory does not grow unbounded."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Engineer Parses a >1 GiB Real-World Capture (Priority: P1)

A support engineer points My-gather at a real pt-stalk capture that totals
1.63 GB on disk. The tool parses it without raising a collection-size error
and produces a useful HTML report.

**Why this priority**: Real production captures already exceed the historical
1 GiB cap; the tool currently exits non-zero on those inputs and is unusable
for the case it is most needed for. Removing the cap is the single change
that unblocks every large-capture user.

**Independent Test**: Run `my-gather -o /tmp/r.html <largeCaptureDir>` against
a >1.1 GiB synthetic or real capture and verify the binary exits 0 with the
report generated.

**Acceptance Scenarios**:

1. **Given** an input directory whose total file size is 1.63 GB, **When**
   `my-gather` is invoked against it, **Then** the run exits 0 and writes a
   report file. The historical
   `collection size N bytes exceeds 1073741824-byte limit` message MUST NOT
   appear on stderr.
2. **Given** an input directory whose total file size is 5 GB, **When**
   `my-gather` is invoked against it, **Then** the run does not fail with
   any collection-size error from the parser; only true input or parse
   failures (missing files, malformed format) can surface.

### User Story 2 - Large Captures Stream Without OOM (Priority: P1)

When parsing a multi-GB capture, the tool's resident memory stays bounded
relative to per-file working set, not to the total collection size. Removing
the cap does not become a foot-gun that OOM-kills the process.

**Why this priority**: A cap removed without verifying streaming would simply
move the failure from "cap-exceeded error" to "out-of-memory kill", which is
strictly worse — a clean error message becomes a kernel signal with no
diagnostics.

**Independent Test**: A regression test feeds a streamed >1.1 GiB synthetic
capture through `parse.Discover` and asserts that peak heap stays within a
small multiple of the largest individual file working set, not within the
total collection size.

**Acceptance Scenarios**:

1. **Given** a synthetic input whose total size exceeds 1.1 GiB, **When**
   `parse.Discover` runs against it, **Then** it returns a `*model.Collection`
   without a `*parse.SizeError` of kind `SizeErrorTotal` and without panic.
2. **Given** the same input, **When** memory is sampled before and after
   `parse.Discover`, **Then** peak in-process heap delta stays well below
   the total input size, demonstrating per-file streaming.

### Edge Cases

- A capture whose total size is exactly 1 GiB or exactly 1.1 GiB is parsed
  identically to any other capture — there is no boundary behavior at the
  former cap.
- A single source file that is itself extremely large (multi-GB) is still
  bounded by the existing per-file streaming behavior. This feature does
  not alter the per-file token-buffer cap (`maxScanTokenBytes`) used by the
  shared line scanner; per-line bounds remain in force.
- Genuine I/O or parse failures still surface as the same typed errors and
  diagnostics they did before; the cap removal does not mask them.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The parser MUST NOT reject input solely because its total size
  exceeds any byte threshold. There is no successor "higher default" cap and
  no configurable cap flag.
- **FR-002**: The error path that produced
  `parse: collection size N bytes exceeds limit M bytes at <path>` (kind
  `SizeErrorTotal`) MUST be removed. The matching CLI branch in
  `cmd/my-gather` MUST be removed in the same change. No compatibility shim
  or hidden re-introduction is permitted (Constitution XIII).
- **FR-002a**: The archive-input ceiling in `cmd/my-gather/archive_input.go`
  (`maxArchiveExtractedBytes`) previously aliased
  `parse.DefaultMaxCollectionBytes` and would still reject the same 1.63 GB
  capture if the user pointed `my-gather` at a `.tar.gz` of it. That
  alias MUST be replaced with a local typed error
  (`archiveExtractedSizeError`) and a generous defence-in-depth ceiling
  (64 GiB) chosen so that real-world captures pass while runaway archive
  extractions (infinite-loop tar, pathologically expanding gzip) still
  fail bounded. The per-file archive ceiling (`maxArchiveFileBytes`,
  200 MiB via `parse.DefaultMaxFileBytes`) remains the primary defence
  against compression-ratio bombs.
- **FR-003**: Per-file size bounding via `MaxFileBytes` / `SizeErrorFile` is
  unaffected by this feature; large individual files are still subject to
  whatever per-file rule the parser already enforces. (This feature only
  removes the *total-collection* cap.)
- **FR-004**: Every parser stage that runs after Discover's enumeration
  pass MUST consume its source file as a stream (line scanner over an
  `io.Reader`), not by reading the whole file into memory. Any stage that
  buffers an entire collector file MUST be refactored to stream as part of
  this feature. Small env-sidecar files (e.g. `-hostname`, `-meminfo`)
  bounded by `MaxFileBytes` are exempt because they are already
  size-bounded and read once.
- **FR-005**: The tool MUST continue to satisfy Constitution Principle III
  (graceful degradation): per-collector parse failures remain attached as
  diagnostics; only structural preconditions (missing input path,
  unreadable directory) cause early termination.
- **FR-006**: A regression test MUST exercise a synthetic input larger than
  1.1 GiB and assert (a) `parse.Discover` returns no `*SizeError` of kind
  `SizeErrorTotal` and (b) in-process heap growth during the call stays
  well below the total input size — demonstrating that no parser stage
  buffers the whole collection.

### Canonical Path Expectations

- **Canonical owner/path**: `parse.Discover` (in `parse/parse.go`) owns
  collection discovery; `parse.SizeError` / `parse.SizeErrorTotal` (in
  `parse/errors.go`) owned the historical cap path; `cmd/my-gather`
  `mapDiscoverError` owned the user-facing message; per-collector
  `parse.parse*` functions own per-file streaming.
- **Old path treatment**: The total-collection cap path is **deleted in
  this feature**. `DefaultMaxCollectionBytes`, the
  `DiscoverOptions.MaxCollectionBytes` field, the `totalBytes >
  maxCollection` check, the `SizeErrorTotal` enum value, the
  `SizeErrorTotal` branch of `SizeError.Error`, and the
  `SizeErrorTotal` branch of `mapDiscoverError` are removed in the same
  change. No compatibility shim, alias, or feature flag is left behind.
- **External degradation**: None. This change removes a refusal path; the
  user-observable outcome is that a previously-rejected input is now
  parsed. There is no fallback or retry — the cap simply does not exist.
- **Review check**: Reviewers grep the diff for `MaxCollection*`,
  `SizeErrorTotal`, and `1 << 30` and verify (a) no occurrence remains in
  `parse/`, `model/`, `cmd/`, `render/`, `findings/`, or `reportutil/`,
  (b) the per-collector parsers all take `io.Reader` and use the shared
  `newLineScanner`, (c) the new regression test asserts both the
  no-error and the bounded-memory invariants.

### Key Entities

- **Collection size cap**: The total-byte refusal threshold being removed.
  Was `DefaultMaxCollectionBytes int64 = 1 << 30` plus the
  `DiscoverOptions.MaxCollectionBytes` override. Both are removed.
- **`SizeErrorTotal`**: The typed-error kind that signalled the cap had
  been hit. Removed.
- **Per-collector streaming parsers**: Existing `parseIostat`,
  `parseTop`, `parseVmstat`, `parseMeminfo`, `parseVariables`,
  `parseInnodbStatus`, `parseMysqladmin`, `parseProcesslist`,
  `parseNetstat`, `parseNetstatS` — all already consume `io.Reader` and
  use `newLineScanner`; this feature confirms they need no behavioral
  change.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `go test -count=1 ./...` passes, including the new
  >1.1 GiB streaming regression test.
- **SC-002**: A `>1.1 GiB` synthetic input is parsed by `parse.Discover`
  without raising any `*parse.SizeError`.
- **SC-003**: During the regression test, peak in-process heap delta
  remains under 256 MiB even though the input exceeds 1.1 GiB —
  demonstrating that no parser stage buffers the entire collection.
- **SC-004**: The pre-push constitution guard
  (`scripts/hooks/pre-push-constitution-guard.sh`) passes on the diff.
- **SC-005**: Grep for `MaxCollection`, `SizeErrorTotal`, and
  `DefaultMaxCollectionBytes` across `parse/`, `model/`, `cmd/`,
  `render/`, `findings/`, and `reportutil/` finds zero matches after
  the change.

## Assumptions

- The pre-existing per-file size guard (`MaxFileBytes` /
  `SizeErrorFile`) and the per-line token buffer cap
  (`maxScanTokenBytes` = 32 MiB) remain in force. This feature is
  scoped to removing the *total-collection* cap only.
- Per-collector parsers already consume their inputs via `io.Reader`
  with the shared `newLineScanner`; the audit confirms this and no
  behavioral parser refactor is required. The `os.ReadFile` calls
  inside `loadEnvSidecars` and `readHostnameFrom` apply to
  small fixed-set env sidecars, are bounded by `MaxFileBytes`, and are
  not altered.
- The regression test uses an in-memory `io.Reader`-backed synthetic
  capture (or a tempdir of streamed temp files) rather than a checked-in
  >1 GiB fixture, to keep the repository small and deterministic.
- The 256 MiB peak-heap delta threshold in SC-003 is a generous
  ceiling: it must be small enough to fail if any stage buffers the
  whole collection (>1.1 GiB) and large enough to absorb per-file
  working sets and Go runtime variability. The test reports the
  observed delta on failure to make tuning easy if a false positive
  ever appears.
