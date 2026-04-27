# Data Model: pt-stalk Report MVP

All types live in package `model`. No type here has non-trivial methods
— the package is a pure data contract shared between `parse/` (producer)
and `render/` (consumer). Go struct sketches are shown for clarity but
the godoc contract in `contracts/packages.md` is the normative form.

---

## Core entities

### `Collection`

Top of the tree. One per CLI invocation.

```go
type Collection struct {
    RootPath     string        // absolute path to the pt-stalk dir
    Hostname     string        // derived from -hostname or pt-summary.out; "" if unknown
    PtStalkSize  int64         // total bytes across all files in RootPath
    Snapshots    []*Snapshot   // sorted by Timestamp ascending
    Diagnostics  []Diagnostic  // collection-wide diagnostics (e.g., an unreadable subdir)
}
```

Invariants:

- `Snapshots` is sorted by `Timestamp` ascending.
- `Snapshots` is non-empty when `Collection` is returned successfully
  by `parse.Discover`. (Empty-but-valid directories raise
  `ErrNotAPtStalkDir` per research R5.)
- `Hostname` is `""` rather than `"unknown"` when not derivable, so
  `render/` can decide how to present that.

Lifecycle:

- Constructed by `parse.Discover`.
- Never mutated after `parse.Discover` returns.

---

### `Snapshot`

One timestamped pt-stalk collection pass. A `Collection` may have one
or more.

```go
type Snapshot struct {
    Timestamp   time.Time           // parsed from the file prefix; UTC normalised where possible
    Prefix      string              // raw prefix string, e.g., "2026_04_21_16_52_11"
    SourceFiles map[Suffix]*SourceFile
}

type Suffix string

const (
    SuffixIostat       Suffix = "iostat"
    SuffixTop          Suffix = "top"
    SuffixVariables    Suffix = "variables"
    SuffixVmstat       Suffix = "vmstat"
    SuffixMeminfo      Suffix = "meminfo"       // added mid-feature, commit 6e2c150
    SuffixInnodbStatus Suffix = "innodbstatus1"
    SuffixMysqladmin   Suffix = "mysqladmin"
    SuffixProcesslist  Suffix = "processlist"
    SuffixNetstat      Suffix = "netstat"       // post-MVP, commit 0790e17 — per-sample socket dump
    SuffixNetstatS     Suffix = "netstat_s"     // post-MVP, commit 0790e17 — aggregate kernel counters
)
```

Invariants:

- `SourceFiles` is keyed by one of the declared `Suffix` constants.
- `SourceFiles` MAY omit any suffix — a missing file is represented by
  the map entry being absent, never by a `nil` value. `render/`
  distinguishes absence from presence-with-errors via the
  `SourceFile.Status` field.

Ordering note: render walks `SourceFiles` via the declared `Suffix`
constant order, never via Go map iteration, to satisfy determinism.

---

### `SourceFile`

One pt-stalk collector output file.

```go
type SourceFile struct {
    Suffix      Suffix
    Path        string         // absolute path on disk
    Size        int64          // bytes
    Version     FormatVersion  // detected pt-stalk format (see research R2)
    Status      ParseStatus    // ParseOK | ParsePartial | ParseFailed | ParseUnsupported
    Diagnostics []Diagnostic   // file-level diagnostics
    Parsed      any            // typed payload; concrete type depends on Suffix (see table below)
}

type FormatVersion int

const (
    FormatUnknown FormatVersion = iota
    FormatV1                    // current Percona Toolkit stable
    FormatV2                    // one major back
)

type ParseStatus int

const (
    ParseOK ParseStatus = iota
    ParsePartial
    ParseFailed
    ParseUnsupported
)
```

`Parsed` is typed per suffix. The mapping is:

| Suffix | `Parsed` concrete type |
|---|---|
| `iostat` | `*IostatData` |
| `top` | `*TopData` |
| `variables` | `*VariablesData` |
| `vmstat` | `*VmstatData` |
| `innodbstatus1` | `*InnodbStatusData` |
| `mysqladmin` | `*MysqladminData` |
| `processlist` | `*ProcesslistData` |

Rationale for `any` (rather than separate typed pointers on `Snapshot`):
keeps `Snapshot` topology uniform (a map keyed by suffix), lets the
renderer iterate in declared order, and makes future collectors
additive without changing the struct. Consumers switch on `Suffix`.

---

## Time-series payload types

### `Sample`

A single time-stamped observation.

```go
type Sample struct {
    Timestamp    time.Time
    Measurements map[string]float64  // never iterated directly by render; see determinism helpers
}
```

### `MetricSeries`

An ordered series of samples for one metric on one subject.

```go
type MetricSeries struct {
    Metric  string   // e.g., "util_percent"
    Unit    string   // e.g., "%", "kB/s", "count"
    Subject string   // e.g., "sda", "root_process_id=1234"
    Samples []Sample
}
```

Invariants:

- `Samples` is sorted by `Timestamp` ascending.
- `Samples` may be empty when the underlying file was present but
  unparseable past the header — `render` shows "data not available"
  in that case.

---

## Per-collector payload types

### `IostatData`

```go
type IostatData struct {
    Devices            []DeviceSeries
    SnapshotBoundaries []int            // sample indexes at which a new Snapshot's first sample sits
}

type DeviceSeries struct {
    Device     string
    Utilization MetricSeries  // metric "util_percent", unit "%"
    AvgQueueSize MetricSeries // metric "avgqu_sz", unit "count"
}
```

Invariants:

- `Devices` sorted alphabetically by `Device`.
- For any given `Device`, `Utilization.Samples` and `AvgQueueSize.Samples`
  cover the same set of timestamps (per iostat's design).
- **All `DeviceSeries` in a merged `IostatData` share a single
  timestamp axis.** Within a single-Snapshot payload every device
  has matched-length samples by construction (pt-stalk's iostat
  reports every device at the same intervals within one file). In
  the multi-Snapshot case, `render.concatIostat` pads devices that
  were absent in some Snapshots: each absent-Snapshot sample has a
  valid `Timestamp` from that Snapshot's axis and a `NaN`
  `Measurements["util_percent"]` / `["avgqu_sz"]` value. The chart
  payload serialises NaN as JSON `null` so uPlot draws a visible
  gap rather than a misaligned series.
- `SnapshotBoundaries` indexes into the shared axis (equivalently,
  into any sorted device's `Utilization.Samples` slice — every
  device is aligned to the same length). Always includes `0`. For
  multi-Snapshot collections (spec FR-018) the merger at
  `render.concatIostat` records a boundary at each Snapshot's first
  sample index. The renderer draws a vertical boundary marker at
  each corresponding timestamp (FR-030).

### `TopData`

```go
type TopData struct {
    ProcessSamples     []ProcessSample
    Top3ByAverage      []ProcessSeries  // length 0..3
    SnapshotBoundaries []int            // see IostatData.SnapshotBoundaries
}

type ProcessSample struct {
    Timestamp time.Time
    PID       int
    Command   string
    CPUPercent float64
}

type ProcessSeries struct {
    PID     int
    Command string
    CPU     MetricSeries
}
```

Derived invariant: `Top3ByAverage` is computed at parse time ranking
by `sum(CPUPercent) / totalBatchCount` — i.e., a PID that appears in
only half the `-top` batches is penalised for the missing half
(absent-in-batch contributes 0 to the numerator). This matches spec
FR-010 "aggregate across the collection window" with the F7 remediation
and the implementation in `parse/top.go`. Each returned
`ProcessSeries.CPU.Samples` holds only the observed samples (not
zero-padded to `totalBatchCount`) — the zero-fill is an averaging
convention, not a storage shape. Ties are broken deterministically by
(higher PID first).

### `VariablesData`

```go
type VariablesData struct {
    Entries []VariableEntry  // sorted alphabetically by Name
}

type VariableEntry struct {
    Name  string
    Value string  // kept as string — values can be non-numeric (e.g., character sets)
}
```

Invariant: `Entries` is deduplicated by `Name`; if the source file
contains duplicate names (which can happen if session values leak
in), only the first global occurrence is retained and a
`Diagnostic(Severity=Warning)` records the skipped duplicates.

### `VmstatData`

```go
type VmstatData struct {
    Series             []MetricSeries  // ordered: [runqueue, blocked, free_kb, buff_kb, cache_kb,
                                       //          swap_in, swap_out, io_in, io_out, cpu_user,
                                       //          cpu_sys, cpu_idle, cpu_iowait]
    SnapshotBoundaries []int           // see IostatData.SnapshotBoundaries
}
```

Invariants:

- `Series` is in a fixed declared order (documented in
  `parse/vmstat.go`) so the rendered chart is deterministic even when
  some measurements are absent in an older `vmstat` version. A column
  absent from the source does not drop its slot: the parser
  zero-substitutes the missing value per row, so every MetricSeries
  ends up with the same sample count as the number of parsed data
  rows. The canonical Metric name is always set so the renderer can
  keep a stable legend, and a per-column absence is represented by
  those zero-substituted samples — the parser does NOT emit a
  diagnostic and does NOT produce a zero-length Samples slice for
  missing columns. No committed fixture currently exercises a
  missing column, so that behaviour is specified here but not pinned
  by fixture coverage today; `parse/vmstat_test.go`'s
  length-equality invariant only asserts equal Series lengths for
  the exercised fully-populated input.
- `SnapshotBoundaries` indexes into `Series[0].Samples` — the primary
  timestamp axis the renderer uses. Same semantics as
  IostatData.SnapshotBoundaries.

### `InnodbStatusData`

```go
type InnodbStatusData struct {
    SemaphoreCount     int          // "Number of semaphores" — scalar callout
    PendingReads       int          // "pending reads" — scalar callout
    PendingWrites      int          // "pending writes" — scalar callout
    AHIActivity        AHIActivity  // hash table size, searches / sec, hits / sec
    HistoryListLength  int          // scalar callout ("HLL")
}

type AHIActivity struct {
    HashTableSize  int
    SearchesPerSec float64
    NonHashSearchesPerSec float64
}
```

Note: the render treats most of these as single-value callouts because
`-innodbstatus1` is a point-in-time snapshot, not a time series. When
the input has multiple snapshots (spec FR-018), render shows a
side-by-side per-snapshot table of these scalars.

### `MysqladminData`

```go
type MysqladminData struct {
    VariableNames    []string          // sorted alphabetically
    SampleCount      int
    Timestamps       []time.Time       // length SampleCount, sorted ascending
    Deltas           map[string][]float64  // key: variable name; value: length SampleCount
    IsCounter        map[string]bool   // per-variable: true if raw values are monotonic counters
    SnapshotBoundaries []int           // sample indexes at which a new snapshot's first sample sits
}
```

Invariants:

- `len(Deltas[v]) == SampleCount` for every `v` in `VariableNames`.
- For counters (`IsCounter[v] == true`), `Deltas[v][0]` is the
  raw initial tally of the first sample (matching pt-mext), not
  zero; aggregate stats excluded from slot 0 (see research R8).
- For gauges (`IsCounter[v] == false`), `Deltas[v][i]` holds the
  raw per-sample value, not a delta.
- A variable missing from any sample contributes `math.NaN()` at
  that slot and emits a `Diagnostic(Severity=Warning)` (research R8
  improvement C).
- `VariableNames` and `IsCounter` keys are identical as a set.
- `SnapshotBoundaries` lists the sample indexes at which a new
  `-mysqladmin` file's first sample sits (always includes `0` for the
  very first sample). For every boundary index `b > 0`, counters
  MUST have `Deltas[v][b] == NaN` — the `previous` map is reset at
  each boundary, the first post-boundary sample carries its own
  initial tally, and the "delta" slot is explicitly undefined because
  cross-snapshot counter deltas are meaningless (counters may have
  been reset; time gap is arbitrary). An `Info` diagnostic is
  emitted per boundary (spec FR-030). The renderer draws a vertical
  boundary marker at each corresponding timestamp.
- Aggregate stats (total, min, max, avg) skip any sample whose slot
  in `Deltas` is `NaN`, which includes both pre-first-sample slots
  and all post-boundary "first slots".

Rationale for pre-computed deltas: see research R4.
Normative algorithm reference: see research R8 and spec FR-028 —
parity with `_references/pt-mext/pt-mext-improved.cpp` for counter
variables is a correctness invariant.
Snapshot-boundary handling is spec FR-030 (F4 remediation of the
2026-04-21 analyze pass).

### `ProcesslistData`

```go
type ProcesslistData struct {
    ThreadStateSamples []ThreadStateSample
    States             []string  // sorted alphabetically; "Other" always last if present
    SnapshotBoundaries []int     // see IostatData.SnapshotBoundaries
}

type ThreadStateSample struct {
    Timestamp   time.Time
    StateCounts map[string]int  // keyed by state label; missing states = 0
}
```

Invariants:

- `States` is the canonical rendering order. For multi-Snapshot
  collections `States` is the union across Snapshots (plus `Other`
  last when present).
- `StateCounts` in each sample may omit states with count 0; render
  fills zeros.
- `SnapshotBoundaries` indexes into the `ThreadStateSamples` slice —
  the primary timestamp axis the renderer uses. Same semantics as
  IostatData.SnapshotBoundaries.

---

## Diagnostics

### `Diagnostic`

```go
type Diagnostic struct {
    SourceFile string       // absolute path, or "" for collection-wide diagnostics
    Location   string       // "line 412", "byte 102938", or "" if unknown
    Severity   Severity     // SeverityInfo | SeverityWarning | SeverityError
    Message    string       // human-readable; MUST NOT include absolute paths outside SourceFile
}

type Severity int

const (
    SeverityInfo Severity = iota
    SeverityWarning
    SeverityError
)
```

Invariants:

- Every `Diagnostic` with `Severity >= SeverityWarning` MUST mirror to
  stderr at the moment it is recorded (spec FR-027). The `parse`
  package takes a `DiagnosticSink` interface (see
  `contracts/packages.md`) so `cmd/my-gather` can wire this without
  coupling the library to stderr.
- `Message` is a single line, no newlines.

---

## Report envelope

### `Report`

The render-facing view of a Collection.

```go
type Report struct {
    Version          string       // tool semver
    GitCommit        string       // 7-char short
    BuiltAt          string       // build timestamp, ISO-8601 UTC; injected via -ldflags
    Title            string       // human-readable title (hostname + snapshot count);
                                  // derived from Collection by render.collectionTitle
    GeneratedAt      time.Time    // the ONLY non-deterministic field (Principle IV)
    Collection       *Collection  // original, unmodified
    EnvironmentSection *EnvironmentSection // post-MVP (commit 0790e17); rendered first per FR-005
    OSSection          *OSSection
    VariablesSection   *VariablesSection
    DBSection          *DBSection

    // Navigation is computed from the rendered sections at render
    // time; it is not input to Render but exposed in the model so
    // render/templates/report.html.tmpl can iterate it deterministically
    // without re-walking the section tree. See FR-031.
    Navigation []NavEntry

    // ReportID is a stable-per-report identifier used as the prefix
    // for localStorage keys in the collapse/expand feature (FR-032).
    // Computed as a short hash over a canonicalisation of Collection
    // (excluding GeneratedAt), so re-rendering the same input
    // produces the same ReportID. Deterministic (Principle IV).
    ReportID string
}

// NavEntry describes one entry in the report's navigation index
// (FR-031). Entries are flat (one level of nesting: section → subview)
// in v1; the rendered UI renders them as a two-level tree.
type NavEntry struct {
    ID       string  // anchor target; stable across re-renders of same input
    Title    string  // human-readable label
    Level    int     // 1 = top-level section, 2 = subview within a section
    ParentID string  // "" for Level==1
}
```

### Section types

```go
type OSSection struct {
    Iostat  *IostatData        // nil if source missing or parse failed
    Top     *TopData           // nil if source missing or parse failed
    Vmstat  *VmstatData        // nil if source missing or parse failed
    Missing []string           // filenames for which the banner should appear
}

type VariablesSection struct {
    // Per-snapshot when Collection has multiple snapshots (spec FR-018)
    PerSnapshot []SnapshotVariables
}

type SnapshotVariables struct {
    SnapshotPrefix string          // e.g., "2026_04_21_16_52_11"
    Data           *VariablesData  // nil if file missing from this snapshot
}

type DBSection struct {
    InnoDBPerSnapshot []SnapshotInnoDB
    Mysqladmin        *MysqladminData  // merged across snapshots; nil if none present
    Processlist       *ProcesslistData // merged across snapshots; nil if none present
    Missing           []string
}

type SnapshotInnoDB struct {
    SnapshotPrefix string
    Data           *InnodbStatusData
}
```

Multi-snapshot rendering (spec FR-018):

- `VariablesSection.PerSnapshot` and `DBSection.InnoDBPerSnapshot`
  produce one block per snapshot.
- `IostatData`, `TopData`, `VmstatData`, `MysqladminData`,
  `ProcesslistData` are **concatenated** across snapshots with a
  boundary marker rendered in the chart (the render layer inserts
  vertical annotations at snapshot-boundary timestamps).

---

## Validation & parse-status rules

A `SourceFile.Status` is determined thus:

| Condition | Status |
|---|---|
| File absent from filesystem | Entry absent from `SourceFiles`; no Status to set. |
| File present, read OK, all samples parsed | `ParseOK` |
| File present, some samples parsed but trailing bytes malformed (e.g., mid-sample truncation) | `ParsePartial` |
| File present but header / format signature unrecognised by any supported version | `ParseUnsupported` |
| File present but empty (0 bytes) | `ParseFailed` with a zero-byte diagnostic |
| File present, read IO error (permission denied mid-file) | `ParseFailed` |

Render then maps Status → banner text:

- `ParseOK` / `ParsePartial` → render the view. Partial additionally
  shows a "partial data — collection was truncated" sub-banner.
- `ParseUnsupported` → "unsupported pt-stalk version" banner.
- `ParseFailed` → "data not available" banner.
- Entry absent → "data not available" banner.

---

## Type-driven determinism obligations

Types in this package are defined such that no consumer is required to
iterate a Go map in rendering order. Specifically:

- `Snapshot.SourceFiles` — render iterates over the declared `Suffix`
  constants, not over the map itself.
- `Sample.Measurements` — render uses the companion `MetricSeries.Metric`
  name (not the map key order) for labelling.
- `MysqladminData.Deltas` / `.IsCounter` — keyed lookup only;
  iteration goes through the sorted `VariableNames` slice.
- `ThreadStateSample.StateCounts` — iteration goes through the sorted
  `ProcesslistData.States` slice.
- `VariablesData.Entries` — already a sorted slice.

A `//go:generate` directive (optional; not required in v1) can be
added later to auto-verify these invariants with a small analyser.

---

## Summary

The model forms a tree:

```text
Collection
├── []Snapshot
│   └── map[Suffix]*SourceFile
│       └── Parsed (typed per suffix)
├── []Diagnostic
└── Hostname / RootPath / PtStalkSize

Report
├── Collection (by pointer)
├── OSSection        (view over all snapshots' iostat/top/vmstat)
├── VariablesSection (per-snapshot)
├── DBSection        (per-snapshot for InnoDB scalars; merged for others)
├── Navigation       (flat list of NavEntry — FR-031)
└── ReportID         (stable-per-report hash — FR-032)
```

All types are pointer-free at leaf level (values in slices and maps),
minimising garbage-collection overhead for the 1 GB upper-bound
collection profile (SC-008).
