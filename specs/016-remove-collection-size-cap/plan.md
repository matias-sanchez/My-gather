# Implementation Plan: Remove Collection-Size Cap

**Branch**: `016-remove-collection-size-cap` | **Date**: 2026-05-07 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/016-remove-collection-size-cap/spec.md`

## Summary

Delete the historical 1 GiB total-collection refusal path so My-gather can
parse real-world pt-stalk captures larger than `1 << 30` bytes. The cap is
removed entirely — no successor higher default, no flag, no shim. An audit
of the per-collector parsers confirms each one already streams its source
file via `io.Reader` + the shared `newLineScanner`, so removing the cap
does not introduce out-of-memory risk; a new regression test pins that
invariant by parsing a >1.1 GiB synthetic capture and asserting bounded
heap growth.

## Technical Context

**Language/Version**: Go 1.26.2
**Primary Dependencies**: Go standard library only (`io`, `bufio`, `os`,
`path/filepath`, `runtime`, `testing`).
**Storage**: N/A — read-only filesystem inputs (Principle II).
**Testing**: `go test ./...` plus a focused streaming-regression test under
`parse/` that uses tempdir + streamed temp files.
**Target Platform**: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
(Principle I; unchanged).
**Project Type**: Single Go CLI + library (`cmd/`, `parse/`, `model/`,
`render/`, `findings/`, `reportutil/`).
**Performance Goals**: No regression versus the pre-feature baseline on
small captures; per-file streaming on multi-GB captures.
**Constraints**: One canonical path per behaviour (Principle XIII), no
hidden fallback, no compatibility shim. Source files remain ≤ 1000 lines
(Principle XV). English-only artifacts (Principle XIV). No new direct
dependency (Principle X).
**Scale/Scope**: One parser module touched (`parse/parse.go`,
`parse/errors.go`), one CLI error-mapping function touched
(`cmd/my-gather/main.go` `mapDiscoverError`/`mapInputPreparationError`),
one archive-input module touched
(`cmd/my-gather/archive_input.go`) to break the alias on
`parse.DefaultMaxCollectionBytes` and switch to a local typed error
`archiveExtractedSizeError` with a generous local ceiling, one test
file added (`parse/streaming_large_test.go`), and a small
`parse_test.go` cleanup removing cap-exceeded assertions. Targeted,
small diff.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Static Binary | PASS | No CGO, no new build tag, no link-time change. |
| II. Read-Only Inputs | PASS | Discover remains read-only; no new write. The new test writes only inside `t.TempDir()`. |
| III. Graceful Degradation | PASS | Per-collector parse failures still surface as diagnostics. The removed path was a hard refusal, not a degradation; removing it makes more inputs usable. |
| IV. Deterministic Output | PASS | No render-side change. Existing golden tests stand. |
| V. Self-Contained HTML | PASS | No render-asset change. |
| VI. Library-First | PASS | Parser package boundary unchanged; the removed `MaxCollectionBytes` field shrinks the public surface (a deletion, not an addition). |
| VII. Typed Errors | PASS | `SizeErrorTotal` is removed cleanly; `SizeErrorFile` remains. Callers branching via `errors.As(&SizeError{})` and switching on `Kind` still work for the file case. No untyped error introduced. |
| VIII. Reference Fixtures & Goldens | PASS | No collector parser added; no fixture/golden required. The new test exercises Discover-level streaming, not per-collector golden output. |
| IX. Zero Network | PASS | No network code touched. |
| X. Minimal Dependencies | PASS | Stdlib only; no `go.mod` change. |
| XI. Human-Pressure Reports | PASS | Report content unchanged. |
| XII. Pinned Go Version | PASS | No `go.mod` `go` directive change. |
| XIII. Canonical Code Path | PASS | The total-collection cap is the single canonical refusal path being removed. Every call site (`parse.Discover`, `parse.SizeError.Error`, `cmd/my-gather mapDiscoverError`) is updated in the same change. No alias, no flag, no preserved alternative. See Canonical Path Audit below. |
| XIV. English-Only | PASS | All new artifacts are English. |
| XV. Bounded Source Size | PASS | `parse/parse.go` shrinks; no file approaches 1000 lines. |

**Canonical Path Audit (Principle XIII)**:

- Canonical owner/path for touched behaviour:
  - `parse.Discover` (in `parse/parse.go`) — collection enumeration and the
    historical total-size guard.
  - `parse.SizeError` and `parse.SizeErrorKind` (in `parse/errors.go`) —
    typed error for size-bound refusals.
  - `cmd/my-gather` `mapDiscoverError` (in `cmd/my-gather/main.go`) — CLI
    surfacing of `SizeError`.
- Replaced or retired paths: the total-collection guard
  (`DefaultMaxCollectionBytes`, `DiscoverOptions.MaxCollectionBytes`, the
  `totalBytes > maxCollection` check, the `SizeErrorTotal` enum value, the
  `SizeErrorTotal` arm of `SizeError.Error`, and the `SizeErrorTotal` arm
  of `mapDiscoverError`) is **deleted in this change**. No flag preserved,
  no internal alias, no compatibility shim. The per-file guard
  (`MaxFileBytes` / `SizeErrorFile`) is unchanged because it is a
  different rule with a different scope.
- External degradation paths, if any: none. The removed path was an
  internal refusal, not an external boundary failure. There is no
  fallback or retry to add — the cap simply ceases to exist. Streaming of
  per-collector files is already the canonical behaviour and is now the
  only behaviour (no buffered alternative).
- Review check: reviewers grep the diff for `MaxCollection`,
  `SizeErrorTotal`, `1 << 30`, and `DefaultMaxCollectionBytes` and confirm
  zero remaining matches in `parse/`, `model/`, `cmd/`, `render/`,
  `findings/`, and `reportutil/`. Reviewers also confirm the new
  streaming-regression test asserts both the no-error and the
  bounded-heap invariants.

## Project Structure

### Documentation (this feature)

```text
specs/016-remove-collection-size-cap/
├── plan.md
├── research.md
├── quickstart.md
├── contracts/
│   └── parse.md
├── checklists/
│   └── requirements.md
├── spec.md
└── tasks.md
```

### Source Code (repository root, scope of this feature only)

```text
parse/
├── parse.go               # remove MaxCollectionBytes field + totalBytes guard
├── errors.go              # remove SizeErrorTotal enum value + its Error arm
├── parse_test.go          # remove any cap-exceeded test cases
└── streaming_large_test.go  # NEW: >1.1 GiB streaming regression

cmd/my-gather/
├── main.go                # remove SizeErrorTotal branch in mapDiscoverError;
│                          # add archiveExtractedSizeError branch in
│                          # mapInputPreparationError
└── archive_input.go       # break alias on parse.DefaultMaxCollectionBytes;
                           # add local typed error archiveExtractedSizeError;
                           # define local maxArchiveExtractedBytes (64 GiB)
```

**Structure Decision**: This is a targeted refactor against existing
package boundaries. No new package, no new layer.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No constitution exceptions are required.
