# Research: Remove Collection-Size Cap

## R1 — What does the cap actually do today?

**Decision**: The historical cap has two surfaces — a parser-internal
guard and a CLI surface — and both go away in this feature.

**Evidence**:

- `parse/parse.go`:
  - `DefaultMaxCollectionBytes int64 = 1 << 30` — the literal 1 GiB.
  - `DiscoverOptions.MaxCollectionBytes int64` — caller override (zero
    means default; negative is rejected).
  - In `Discover`: `if totalBytes > maxCollection { return nil,
    &SizeError{Kind: SizeErrorTotal, ...} }`.
- `parse/errors.go`:
  - `SizeErrorTotal SizeErrorKind = iota + 1` — the typed-error tag.
  - `SizeError.Error()` switch arm for `SizeErrorTotal`.
- `cmd/my-gather/main.go` `mapDiscoverError`:
  - `case parse.SizeErrorTotal: fmt.Fprintf(stderr, "my-gather:
    collection size %d bytes exceeds %d-byte limit at %s\n", ...)`
    plus exit code `exitSizeBound`.

**Rationale for full deletion**: A cap that the user cannot configure
without surgery is not a useful safety net — it is a refusal. The user
decision (issue #50) is to remove the cap entirely, not to bump it,
because any successor numeric default would simply move the same
refusal to a slightly larger input. Constitution Principle XIII forbids
preserving the old behaviour behind a flag or internal alias.

**Alternatives considered**:

- **Bump default to 10 GiB / 100 GiB**: rejected by user. Would still
  reject some real captures, would still need to be removed eventually,
  and would leave the cap-vs-streaming question unanswered.
- **Make the cap configurable via CLI flag**: rejected by user.
  Configurable defaults are a Principle XIII smell here because the
  cap is a refusal, not a tunable behaviour.
- **Deprecate `MaxCollectionBytes` but keep the field**: rejected by
  Principle XIII. Public deprecation shims are forbidden for internal
  identifiers; all call sites are updated in the same change.

## R2 — Do downstream parsers stream?

**Decision**: Yes. No per-collector parser refactor is required.

**Evidence** (verified by reading every file under `parse/` other than
`parse.go`/`errors.go`/`version.go`):

- All per-collector parsers (`parseIostat`, `parseTop`, `parseVmstat`,
  `parseMeminfo`, `parseVariables`, `parseInnodbStatus`,
  `parseMysqladmin`, `parseProcesslist`, `parseNetstat`,
  `parseNetstatS`) accept `io.Reader` and use the package-shared
  `newLineScanner`, which caps a single token at 32 MiB
  (`maxScanTokenBytes`) and otherwise streams line-by-line.
- `runOneParser` in `parse/parse.go` opens the source file with
  `os.Open` (read-only, Principle II), peeks 8 KiB for format
  detection, then `Seek(0, io.SeekStart)` and hands the `*os.File`
  directly to the per-collector parser. No `os.ReadFile`, no
  `io.ReadAll`, no `bytes.Buffer{}` accumulating the whole file.
- The only `os.ReadFile` call sites in `parse/` are
  `loadEnvSidecars` and `readHostnameFrom`. They read small
  fixed-set env sidecars (`-hostname`, `-meminfo`, `-procstat`,
  `-sysctl`, `-top`, `-df`, `-output`) that are bounded by
  `MaxFileBytes` (200 MiB default) and that the design already
  treats as one-shot machine-inventory dumps. They are out of scope
  for this feature.
- `dfsnapshot.go` `splitDFSamples` builds a `strings.Builder` per
  sample block (between `TS` markers), not per file. Per-sample
  buffering is bounded by typical pt-stalk sample size (kB), not by
  the collection.

**Rationale**: The audit confirms the cap was not protecting the
parsers from a buffered-everything code path. It was a separate, older
refusal threshold. Removing it does not require any parser refactor.
The new streaming-regression test pins this property so a future
refactor that quietly slurps a whole file would be caught.

**Alternatives considered**:

- **Refactor parsers anyway as defence-in-depth**: rejected. They
  already stream; rewriting them would be churn for no behaviour
  change and would risk golden drift on completely unrelated code.
- **Add a parser-side "max file bytes read into memory at once"
  invariant**: subsumed by the new streaming-regression test, which
  asserts the property by measurement instead of by code-path
  inspection.

## R3 — How to test "memory does not grow unbounded" in `go test`?

**Decision**: Generate a >1.1 GiB synthetic capture into a
`t.TempDir()` as a small set of streamed temp files (each one written
chunk-by-chunk so the test itself does not allocate >1 GiB during
setup), then call `parse.Discover` against the tempdir, and bound the
peak heap delta with `runtime.MemStats.HeapAlloc` sampled before/after,
plus a runtime ceiling using `runtime.ReadMemStats` after a
`runtime.GC()`.

**Evidence**: This is the standard pattern used by Go's stdlib tests
that need to assert bounded memory (e.g., `compress/gzip` streaming
tests). It avoids checking in a >1 GiB fixture (Principle II /
repository hygiene), is deterministic, and runs entirely under
`testing`.

**Threshold**: 256 MiB peak heap delta. Justification:

- Per-file working set: the largest single file the test creates is on
  the order of 200 MiB; the per-collector parser may hold ≤ a single
  scanner token (32 MiB cap) plus its parsed-model slice. Real working
  set is far below 256 MiB even when summed across the snapshot.
- Whole-collection buffering would be ≥ 1.1 GiB, far above 256 MiB —
  any buffered-everything stage would fail the test loudly.
- Go runtime variability (heap fragmentation, GC headroom): 256 MiB
  is generous enough to avoid CI flakes.
- On failure the test prints the observed delta so a real regression
  is easy to triage.

**Test gating**: The test is tagged with `testing.Short()` skip so
`go test -short ./...` does not run the multi-GB allocation. The
default `go test ./...` still exercises it (which matches CI
expectations).

**Alternatives considered**:

- **Check in a >1 GiB fixture**: rejected. Repo hygiene; no need.
- **Use `testing/quick` or `os.Pipe`**: not necessary; tempdir +
  streamed writes is simpler and matches what `Discover` really walks.
- **Use `runtime/pprof`**: overkill for a single bound assertion.

## R5 — Why does the archive cap stay (at 64 GiB), instead of being removed?

**Decision**: Keep an archive-extraction total ceiling, but raise it to
64 GiB and give it a local typed error
(`archiveExtractedSizeError`) so it no longer aliases the parser cap.

**Evidence and rationale**:

- `cmd/my-gather/archive_input.go` previously declared
  `const maxArchiveExtractedBytes = parse.DefaultMaxCollectionBytes`,
  i.e. 1 GiB — the same threshold as the parser cap. That meant the
  same 1.63 GB capture would be rejected if the user passed a
  `.tar.gz` of it instead of the unpacked directory. Removing the
  parser cap without touching the archive cap would leave the bug
  half-fixed.
- The archive cap is genuinely a different rule: it guards an
  external boundary (untrusted archive input) against a runaway
  extraction (infinite-loop tar with circular hard links, gzip
  stream that pathologically expands beyond declared size).
  Constitution Principle XIII explicitly permits external-boundary
  degradation of this kind, observable to the caller and routed
  through a typed error.
- Picking a value: 64 GiB. Real pt-stalk captures observed in the
  wild are in the low single-digit GB range (the prompting case is
  1.63 GB; an aggressive multi-day stalk might reach 10–20 GB).
  64 GiB is well above any realistic capture and well below the
  point where filling a typical jump-host disk would happen
  silently. 16 GiB was considered and rejected as still
  uncomfortably close to legitimate inputs once you account for
  hour-long stalks at high sample density. 1 TiB was considered
  and rejected as effectively no defence at all — at that point we
  may as well drop the check.
- No CLI flag to tune the value: same Principle XIII reasoning as
  the parser cap. The threshold is a defence-in-depth ceiling, not
  a tunable behaviour.

**Alternatives considered**:

- **Drop the archive total ceiling entirely**: rejected. The
  per-file ceiling (`maxArchiveFileBytes`, 200 MiB) primarily
  defends against compression-ratio bombs but does not stop a
  multi-million-file tar from filling the temp filesystem. The
  total ceiling is cheap defence-in-depth.
- **Keep the alias to `parse.DefaultMaxCollectionBytes`**:
  impossible — that constant is removed. Even if it weren't, the
  archive cap would re-encode the parser refusal at the archive
  boundary, which is exactly the bug.
- **Reuse `parse.SizeError{Kind: SizeErrorTotal, ...}`**: rejected.
  `SizeErrorTotal` is removed; re-adding it for the archive case
  would resurrect the very enum value the parser-side change
  deletes (Principle XIII: no compatibility shim). A local typed
  error in `cmd/my-gather` keeps the canonical owner where the
  rule lives.

## R4 — Backwards compatibility surface

**Decision**: There is one user-visible behaviour change and one
library-API surface shrink; both are acceptable under Principle XIII
because they are the literal point of this feature.

- **CLI**: `my-gather` no longer prints
  `collection size N bytes exceeds 1073741824-byte limit` and no
  longer exits with `exitSizeBound` for total-collection size. It
  still exits `exitSizeBound` for per-file size violations
  (`SizeErrorFile`). Documented in the spec FR-002.
- **Library**: `parse.DiscoverOptions.MaxCollectionBytes` is removed
  along with `parse.DefaultMaxCollectionBytes` and
  `parse.SizeErrorTotal`. Callers in this repository (`cmd/my-gather`
  only, plus any tests) are updated in the same commit. There is no
  external Go-module consumer to consider; the package is at the
  module path `github.com/matias-sanchez/My-gather/parse` and is used
  only by this repo's `cmd/`.

**Rationale**: Public-surface shrinkage is the canonical Principle XIII
move when removing a behaviour. Deprecation shims are explicitly
forbidden for internal identifiers.
