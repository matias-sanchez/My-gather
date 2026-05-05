# Package Contracts

Public API surface for `parse/`, `model/`, and `render/`. This document
is normative for v1; signatures below are the godoc-level contracts each
package ships. Every exported identifier listed here MUST carry a
matching godoc comment at implementation time (Constitution Principle
VI).

---

## Package `model`

`model` is a pure data package. No I/O, no goroutines, no blocking.

### Types

All types listed in `data-model.md`. The exported surface is:

```go
// Entities
type Collection struct { /* see data-model.md */ }
type Snapshot   struct { /* ... */ }
type SourceFile struct { /* ... */ }
type Sample     struct { /* ... */ }
type MetricSeries struct { /* ... */ }
type VariableEntry struct { /* ... */ }
type Diagnostic struct { /* ... */ }
type Report     struct {
    // See data-model.md for the canonical, field-by-field definition
    // (including the display-only `Title` and `BuiltAt` that originate
    // from RenderOptions rather than from the parsed Collection).
}

// Per-collector payloads
type IostatData       struct { /* ... */ }
type DeviceSeries     struct { /* ... */ }
type TopData          struct { /* ... */ }
type ProcessSample    struct { /* ... */ }
type ProcessSeries    struct { /* ... */ }
type VariablesData    struct { /* ... */ }
type VmstatData       struct { /* ... */ }
type InnodbStatusData struct { /* ... */ }
type AHIActivity      struct { /* ... */ }
type MysqladminData   struct { /* ... */ }
type ProcesslistData  struct { /* ... */ }
type ThreadStateSample struct { /* ... */ }

// Section views
type OSSection        struct { /* ... */ }
type VariablesSection struct { /* ... */ }
type DBSection        struct { /* ... */ }
type SnapshotVariables struct { /* ... */ }
type SnapshotInnoDB   struct { /* ... */ }

// Navigation (FR-031)
type NavEntry struct { /* ... */ }  // see data-model.md

// Enums
type Suffix        string
type FormatVersion int
type ParseStatus   int
type Severity      int

// Suffix constants
const (
    SuffixIostat       Suffix = "iostat"
    SuffixTop          Suffix = "top"
    SuffixVariables    Suffix = "variables"
    SuffixVmstat       Suffix = "vmstat"
    SuffixInnodbStatus Suffix = "innodbstatus1"
    SuffixMysqladmin   Suffix = "mysqladmin"
    SuffixProcesslist  Suffix = "processlist"
)

// FormatVersion / ParseStatus / Severity constants — see data-model.md
```

### Constants

- `KnownSuffixes []Suffix` — the declared order used by render iteration.

### Functions

None. The package is data-only; construction happens in `parse/`.

### Contract notes

- No type has a `Validate()` method in v1. Parse-time validation is the
  parser's responsibility; post-parse the tree is assumed consistent.
- All slices are never `nil` after `parse.Discover` returns; they may
  be zero-length but will be allocated, to simplify template
  iteration.
- All map values for declared keys are never `nil`; absent data is
  indicated by the map entry being missing entirely.

---

## Package `parse`

`parse` reads pt-stalk directories and produces a `*model.Collection`.

### Functions

```go
// Discover walks rootDir, identifies pt-stalk snapshots, runs every
// applicable collector parser, and returns a fully-populated Collection.
//
// Discover returns ErrNotAPtStalkDir (wrapped) when rootDir does not
// look like a pt-stalk output directory (see research R5).
// Discover returns a *SizeError (wrapped) when rootDir or any single
// source file exceeds the supported bounds (see FR-025).
// Discover returns a *PathError (wrapped) when rootDir is missing,
// unreadable, or not a directory.
//
// Discover never writes to rootDir and never takes locks there.
//
// Discover does not return per-collector parse errors as Go errors;
// those are captured in Collection.Diagnostics and in each SourceFile's
// Status/Diagnostics fields. The returned error is nil unless the
// whole collection is unusable.
//
// Discover is safe to call concurrently from different goroutines on
// different directories. It does not spawn goroutines internally in
// v1 (determinism concerns dominate; parallelism is a future feature).
func Discover(ctx context.Context, rootDir string, opts DiscoverOptions) (*model.Collection, error)
```

```go
// DiscoverOptions are optional knobs for Discover. Zero value is valid.
type DiscoverOptions struct {
    // Sink receives parser diagnostics as they are recorded. Used by
    // cmd/my-gather to mirror diagnostics to stderr per spec FR-027.
    // If nil, diagnostics are still recorded on Collection.Diagnostics
    // and the corresponding SourceFile.
    Sink DiagnosticSink

    // MaxCollectionBytes overrides the default 1 GB total-size bound.
    // Zero means use the default. Passing a negative value is invalid.
    MaxCollectionBytes int64

    // MaxFileBytes overrides the default 200 MB per-file bound. Zero
    // means use the default.
    MaxFileBytes int64
}
```

```go
// DiagnosticSink is a callback invoked synchronously as parser
// diagnostics are recorded. Implementations MUST be cheap; Discover
// blocks on the callback.
type DiagnosticSink interface {
    OnDiagnostic(d model.Diagnostic)
}
```

### Errors

```go
// ErrNotAPtStalkDir reports that the input directory does not look
// like a pt-stalk output directory (no timestamped collectors and no
// pt-summary.out / pt-mysql-summary.out).
var ErrNotAPtStalkDir = errors.New("parse: not a pt-stalk directory")

// SizeError reports that Discover refused to proceed because the
// input exceeds configured size bounds. It distinguishes total-size
// vs per-file violations via Kind.
type SizeError struct {
    Kind  SizeErrorKind  // SizeErrorTotal or SizeErrorFile
    Path  string         // offending path (root for total; file for per-file)
    Bytes int64
    Limit int64
}

type SizeErrorKind int
const (
    SizeErrorTotal SizeErrorKind = iota + 1
    SizeErrorFile
)

func (e *SizeError) Error() string { /* ... */ }

// PathError wraps os-level path failures with tool-specific context.
// Callers use errors.As(err, &parse.PathError{}) to branch.
type PathError struct {
    Op   string // "open", "stat", "readdir"
    Path string
    Err  error
}

func (e *PathError) Error() string { /* ... */ }
func (e *PathError) Unwrap() error { return e.Err }

// ParseError is the typed form of per-file parsing failure. These are
// NOT returned from Discover; they are recorded on
// SourceFile.Diagnostics as the Underlying of a model.Diagnostic.
// They are exported so test code can inspect them.
type ParseError struct {
    File     string  // absolute path
    Location string  // e.g., "line 412", "byte 102938"
    Err      error   // wrapped concrete cause (io, strconv, etc.)
}

func (e *ParseError) Error() string { /* ... */ }
func (e *ParseError) Unwrap() error { return e.Err }
```

Callers use `errors.Is(err, parse.ErrNotAPtStalkDir)` and
`errors.As(err, &parse.SizeError{})` for branching.

### Contract notes

- `Discover` is read-only against `rootDir`. Violation is a bug.
- `Discover` honours `ctx.Done()` between files but not within a file
  read in v1.
- Default size bounds are declared as package-level constants
  (`DefaultMaxCollectionBytes = 1 << 30`,
  `DefaultMaxFileBytes = 200 << 20`) for test accessibility.
- The `-mysqladmin` parser implements the delta-computation algorithm
  natively in Go (ported from pt-mext; see research R8 and spec
  FR-028). Correctness is anchored by a committed golden fixture
  under `testdata/pt-mext/` derived from the worked example at the
  tail of `_references/pt-mext/pt-mext-improved.cpp`. The three
  deliberate improvements over the C++ reference (counter/gauge
  classification, float64 aggregates, drift diagnostics) are part of
  the parser's contract and are documented in `data-model.md`
  `MysqladminData`.

---

## Package `render`

`render` converts a `*model.Collection` into an HTML document.

### Exported surface (as of M1 unexports, 2026-04-22)

The public API of this package is intentionally narrow. In v1 the
exported identifiers are exactly:

- `func Render(w io.Writer, c *model.Collection, opts RenderOptions) error`
- `type RenderOptions struct { … }`
- `var ErrNilCollection = errors.New("render: nil Collection")`

Every other helper that participates in determinism, formatting, or
asset handling is unexported (package-private). In particular, the
following identifiers — documented as public in earlier drafts of
this contract — have been unexported and MUST NOT be imported by
third-party code:

| Former export | Status after M1 |
|---|---|
| `FormatFloat`       | unexported (`formatFloat`) |
| `FormatTimestamp`   | unexported (`formatTimestamp`) |
| `SortStrings`       | unexported (`sortStrings`) |
| `SortedKeys`        | unexported (`sortedKeys`) |
| `SortedEntries`     | unexported (`sortedEntries`) |
| `Entry`             | unexported (`entry`) |
| `OrdinalIDGenerator`| unexported (`ordinalIDGenerator`) |
| `NewOrdinalIDGenerator` | unexported (`newOrdinalIDGenerator`) |
| `CanonicalReportID` | unexported (`canonicalReportID`) |

Rationale: these helpers are determinism-critical implementation
details of `Render`. Exposing them invited ad-hoc usage from tests
and callers that would then pin the package to a surface broader
than what the spec actually guarantees. The M1 pass collapses the
exported surface to the three identifiers above; Principle VI
(library-first) is still satisfied because the package is
independently importable via `Render`.

### Functions

```go
// Render writes a self-contained HTML report for the given Collection
// to w. All CSS, JavaScript, fonts, and data are embedded inline; the
// resulting file makes zero network requests at view-time.
//
// Render is deterministic: given the same Collection and the same
// RenderOptions, two invocations MUST write byte-identical output to w.
//
// Render returns an error only for I/O failures against w. Rendering
// of a Collection with missing or failed sections never returns an
// error — the missing parts become "data not available" banners.
func Render(w io.Writer, c *model.Collection, opts RenderOptions) error
```

```go
// RenderOptions controls optional aspects of rendering. Zero value is
// valid and derives GeneratedAt from the input Collection, with empty
// Version/GitCommit/BuiltAt.
type RenderOptions struct {
    // GeneratedAt is rendered into the report header. When omitted,
    // Render derives it from the first non-zero Snapshot timestamp in
    // the input. If no snapshot timestamp is known, the header renders
    // an explicit "unknown" marker instead of a fabricated date.
    GeneratedAt time.Time

    // Version is the tool's reported version string (semver).
    Version string

    // GitCommit is the short commit hash.
    GitCommit string

    // BuiltAt is the tool's build timestamp (ISO-8601 UTC), injected
    // via -ldflags at link time. Surfaced in the report header and in
    // `--version` output. Empty at dev-build time is acceptable.
    BuiltAt string
}
```

### Contract notes

- `Render` does not take a filesystem path. The caller (`cmd/my-gather`)
  is responsible for opening the output file, flushing, fsyncing, and
  removing temporary files on failure.
- `Render` never panics on nil `Collection.Snapshots`; it writes a
  report with all-missing sections and the "Report generated from an
  empty collection" note.
- `Render` embeds the chart library and stylesheet via `go:embed`; no
  filesystem assets are read at runtime.

---

---

## Package `findings`

`findings` computes rule-based Advisor findings from a parsed
`*model.Report`. Rules live in `findings/rules_*.go`; output is
deterministic.

### Exported surface

- `func Analyze(r *model.Report) []Finding`
- `func Summarise(fs []Finding) Counts`
- `type Finding struct { … }`
- `type Counts struct { … }`
- `type MetricRef struct { … }`
- `type Severity int` with constants `SeveritySkip`, `SeverityInfo`,
  `SeverityWarn`, `SeverityCrit`.

The earlier exported helpers `FormatNum` / `HumanInt` have been
unexported (`formatNum` / `humanInt`). They are formatting details
consumed only by the rules package internally; consumers should
format Finding explanations from the structured fields rather than
re-using the package's internal numeric formatters.

---

## Out-of-band: `cmd/my-gather`

The CLI entry point is not a library; it has no exported surface. Its
behaviour is specified entirely by `contracts/cli.md`. Implementation
responsibilities:

1. Parse flags per `cli.md`.
2. Resolve positional argument and `--out` to absolute paths.
3. Refuse to overwrite an existing output path unless `--overwrite`.
4. Call `parse.Discover(ctx, inputDir, opts)`; map known errors to
   exit codes per `cli.md`.
5. On success, `os.CreateTemp` a sibling of the output, call
   `render.Render(tempFile, collection, renderOpts)`, `Sync`, then
   `os.Rename` to the final path.
6. Wire a `DiagnosticSink` that mirrors warnings/errors to stderr per
   FR-027 and that honours `-v` verbosity for progress lines.
7. On failure after partial writes, remove the temp file.

No other package imports `cmd/my-gather`.

---

## Stability statement

All three packages (`model`, `parse`, `render`) are exported because
Constitution Principle VI requires library-first architecture, and they
ship with godoc so a future maintainer (or the `cmd/my-gather` binary
itself) can consume them. They are **not advertised, promoted, or
stabilised** for third-party consumers in v1 (spec Clarifications Q5).

In practice, within v1:

- The **internal** stability of these packages — the shape that
  `cmd/my-gather` depends on — is maintained across patch releases;
  breaking changes are coordinated with the CLI.
- **No backwards-compatibility guarantee is offered to third-party
  importers.** Types, function signatures, error constants, and
  struct fields may change between v1.x releases without a major
  version bump. Third-party code that imports these packages in v1
  does so knowing a v1.minor release may require code changes.
- `parse` per-collector functions remain unexported; exported surface
  is limited to `Discover`, `DiscoverOptions`, `DiagnosticSink`,
  `ErrNotAPtStalkDir`, `SizeError`, `PathError`, `ParseError`, and the
  default-bound constants.
- A public stability guarantee, if ever offered, is a separate
  future feature requiring its own schema review (Q5 clarification).
