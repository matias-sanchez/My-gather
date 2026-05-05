// Package model is the typed in-memory representation shared between
// parse/ (producer) and render/ (consumer) in the My-gather tool.
//
// All types in this package are pure data — no I/O, no goroutines, no
// blocking operations. Iteration order that affects rendered HTML is
// expressed through sorted slices or declared constant orderings rather
// than through Go map iteration, in support of Constitution Principle IV
// (Deterministic Output).
//
// Every exported identifier carries a godoc contract per Constitution
// Principle VI. See specs/001-ptstalk-report-mvp/data-model.md for the
// normative shape and specs/001-ptstalk-report-mvp/contracts/packages.md
// for the public API surface.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"time"
)

// MaxObservedProcesslistQueries is the maximum number of grouped active
// processlist query summaries rendered in the report.
const MaxObservedProcesslistQueries = 10

// MaxObservedProcesslistQuerySnippetRunes is the maximum visible length of a
// processlist query snippet, including the trailing truncation marker.
const MaxObservedProcesslistQuerySnippetRunes = 160

var (
	processlistQuotedLiteralRE  = regexp.MustCompile(`'([^'\\]|\\.|'')*'|"([^"\\]|\\.|"")*"`)
	processlistNumericLiteralRE = regexp.MustCompile(`\b0x[0-9a-fA-F]+\b|\b\d+(?:\.\d+)?\b`)
)

// Suffix is the pt-stalk collector file-suffix this feature recognises
// and parses. Values are stable across versions; KnownSuffixes defines
// the canonical iteration order used by rendering.
type Suffix string

// Known suffix constants. The value is the suffix as it appears in a
// pt-stalk filename after the timestamp prefix — e.g., a file named
// "2026_04_21_16_52_11-iostat" has suffix "iostat".
const (
	SuffixIostat       Suffix = "iostat"
	SuffixTop          Suffix = "top"
	SuffixVariables    Suffix = "variables"
	SuffixVmstat       Suffix = "vmstat"
	SuffixMeminfo      Suffix = "meminfo"
	SuffixInnodbStatus Suffix = "innodbstatus1" // parses the first innodb-status snapshot; -innodbstatus2 is out of scope for v1
	SuffixMysqladmin   Suffix = "mysqladmin"
	SuffixProcesslist  Suffix = "processlist"
	SuffixNetstat      Suffix = "netstat"   // per-sample socket dump (ss / netstat -an)
	SuffixNetstatS     Suffix = "netstat_s" // aggregate kernel counters (netstat -s)
)

// KnownSuffixes is the declared iteration order used by render-side
// code that walks a Snapshot's SourceFiles map. Templates and
// render-time logic MUST iterate this slice rather than the map
// directly, to satisfy Constitution Principle IV.
var KnownSuffixes = []Suffix{
	SuffixIostat,
	SuffixTop,
	SuffixVariables,
	SuffixVmstat,
	SuffixMeminfo,
	SuffixInnodbStatus,
	SuffixMysqladmin,
	SuffixProcesslist,
	SuffixNetstat,
	SuffixNetstatS,
}

// FormatVersion identifies which pt-stalk release produced a given
// source file. Supported values are FormatV1 (current Percona Toolkit
// stable) and FormatV2 (immediately preceding major version). Files
// whose header does not match either are recorded as FormatUnknown and
// typically result in SourceFile.Status == ParseUnsupported.
type FormatVersion int

// Supported FormatVersion values per spec FR-024.
const (
	// FormatUnknown: the parser could not classify the file as
	// either of the supported pt-stalk releases.
	FormatUnknown FormatVersion = iota

	// FormatV1: current Percona Toolkit stable release.
	FormatV1

	// FormatV2: immediately preceding major Percona Toolkit release.
	FormatV2
)

// ParseStatus describes the outcome of parsing one SourceFile. Render-
// side logic uses this to decide whether to render the typed payload,
// a partial-data banner, or a "data not available" banner.
type ParseStatus int

const (
	// ParseOK: every sample parsed successfully.
	ParseOK ParseStatus = iota

	// ParsePartial: a prefix of the file parsed successfully; trailing
	// bytes were malformed (e.g., the collecting process was killed
	// mid-sample). Parsed carries the recovered data; Diagnostics
	// carries the warning.
	ParsePartial

	// ParseFailed: the file was present and readable but could not be
	// parsed by any supported parser. Parsed may be nil.
	ParseFailed

	// ParseUnsupported: the file header matched neither supported
	// FormatVersion. The subview renders an "unsupported pt-stalk
	// version" banner per spec FR-024.
	ParseUnsupported
)

// Severity classifies a Diagnostic. Info-severity diagnostics surface
// only in the report's Parser Diagnostics panel. Warning and Error
// additionally mirror to the tool's stderr as they are recorded
// (spec FR-027).
type Severity int

const (
	// SeverityInfo: informational — e.g., a snapshot-boundary marker
	// in the mysqladmin stream (FR-030). Never mirrored to stderr.
	SeverityInfo Severity = iota

	// SeverityWarning: recoverable issue — e.g., a malformed sample
	// was skipped. Mirrored to stderr.
	SeverityWarning

	// SeverityError: unrecoverable for this file/subview but not fatal
	// for the collection. Mirrored to stderr.
	SeverityError
)

// Collection is the top of the parsed-data tree. One Collection per
// CLI invocation. Constructed by parse.Discover and never mutated
// after it returns.
type Collection struct {
	// RootPath is the absolute path of the pt-stalk directory the
	// user pointed the tool at. Excluded from ReportID canonicalisation
	// so moving the dump between paths does not reset collapse state
	// (spec FR-032, research R9).
	RootPath string

	// Hostname is derived from -hostname or pt-summary.out when
	// present. Empty string when not derivable — render decides how
	// to present that.
	Hostname string

	// PtStalkSize is the total bytes across every file under RootPath
	// at Discover time. Used by the size-bound preflight (FR-025).
	PtStalkSize int64

	// Snapshots is sorted by Timestamp ascending. Guaranteed
	// non-empty when Discover returns err == nil.
	Snapshots []*Snapshot

	// Diagnostics is the collection-wide diagnostic list (e.g., an
	// unreadable subdir). Per-file diagnostics live on their
	// SourceFile.
	Diagnostics []Diagnostic

	// RawEnvSidecars caches the raw contents of the environment-only
	// sidecar files (-hostname, -meminfo, -procstat, -sysctl, -top,
	// -df, -output) at Discover time. Keyed by suffix. The render
	// layer consumes it instead of re-reading from disk so two
	// renders of the same Collection stay byte-identical even if the
	// underlying files are later modified or removed.
	RawEnvSidecars map[string]string

	// EnvMeminfo is the typed view of the selected -meminfo sidecar.
	// parse.Discover populates it while collecting diagnostics so render
	// does not re-parse the same raw sidecar content.
	EnvMeminfo *EnvMeminfo

	// EnvSidecarTimestamps records the timestamp parsed from the
	// prefix of the sidecar file selected for each suffix in
	// RawEnvSidecars. Keyed by the same suffix. Used by the
	// Environment renderer to anchor point-in-time derivations (e.g.
	// OS uptime from /proc/stat btime) to the clock of the sample the
	// sidecar was taken from — NOT the last Collection snapshot,
	// which can be a newer snapshot without these sidecars.
	EnvSidecarTimestamps map[string]time.Time
}

// Snapshot is one timestamped pt-stalk collection pass within a
// Collection. A Collection may contain one or more Snapshots
// (spec FR-018).
type Snapshot struct {
	// Timestamp is parsed from the snapshot prefix; UTC-normalised
	// where the source provides an offset, otherwise recorded as
	// host-local.
	Timestamp time.Time

	// Prefix is the raw prefix string as it appeared in filenames,
	// e.g. "2026_04_21_16_52_11". Retained verbatim for rendering.
	Prefix string

	// SourceFiles maps each supported Suffix to its SourceFile. A
	// missing entry (map lookup returning the zero value) means the
	// corresponding collector was not present in this snapshot;
	// render-side code shows "data not available" in that case. A
	// present entry with Status >= ParseFailed indicates a present-
	// but-unparseable file.
	SourceFiles map[Suffix]*SourceFile
}

// SourceFile represents one pt-stalk collector output file belonging
// to a single Snapshot.
type SourceFile struct {
	Suffix Suffix
	Path   string // absolute path on disk
	Size   int64  // file size in bytes

	// Version is the detected pt-stalk format (research R2). When the
	// detector cannot classify, Version is FormatUnknown and Status is
	// ParseUnsupported.
	Version FormatVersion

	// Status captures the outcome of parsing. See ParseStatus.
	Status ParseStatus

	// Diagnostics carries file-scoped warnings and errors emitted
	// during parsing.
	Diagnostics []Diagnostic

	// Parsed is the typed payload appropriate to Suffix. Concrete
	// types: *IostatData, *TopData, *VariablesData, *VmstatData,
	// *InnodbStatusData, *MysqladminData, *ProcesslistData. Nil when
	// Status is ParseFailed or ParseUnsupported.
	Parsed any
}

// Sample is one time-stamped observation within a time-series
// SourceFile. Most parsers produce a slice of Sample for each metric;
// some produce a scalar (see InnodbStatusData).
type Sample struct {
	Timestamp time.Time

	// Measurements is a metric-name -> value map for this sample. It
	// is never iterated directly by render; consumers use companion
	// MetricSeries structures that carry the canonical metric name.
	Measurements map[string]float64
}

// MetricSeries is an ordered set of Samples for one metric on one
// subject (e.g., "sda %util", "mysqld-uptime"). Invariants:
//   - Samples is sorted by Timestamp ascending.
//   - Samples may be empty when the underlying file was present but
//     unparseable beyond the header.
type MetricSeries struct {
	Metric  string // e.g., "util_percent"
	Unit    string // e.g., "%", "kB/s", "count"
	Subject string // e.g., "sda", or "" for collection-wide metrics
	Samples []Sample
}

// VariableEntry is one global MySQL variable and its value captured
// by the -variables collector. See VariablesData for usage.
type VariableEntry struct {
	Name  string
	Value string // kept as string — values can be non-numeric (e.g., character sets, paths)
}

// Diagnostic is a parser-level warning or error. Every Diagnostic with
// Severity >= SeverityWarning is mirrored to stderr by cmd/my-gather
// at the moment it is recorded (spec FR-027). Info-severity
// diagnostics surface only in the report's Parser Diagnostics panel.
type Diagnostic struct {
	// SourceFile is the absolute path, or "" for collection-wide
	// diagnostics (e.g., an unreadable subdir).
	SourceFile string

	// Location is a human-readable pointer into the file —
	// "line 412", "byte 102938" — or "" if unknown.
	Location string

	// Severity classifies the diagnostic. See Severity.
	Severity Severity

	// Message is a single line, no embedded newlines. Must not repeat
	// SourceFile or Location (they are rendered alongside).
	Message string
}

// IostatData is the typed payload for a -iostat SourceFile (or, when
// the render layer merges multiple -iostat files across Snapshots per
// spec FR-018, a concatenation of them onto a single time axis).
//
// In the single-Snapshot case, every DeviceSeries shares the
// Snapshot's iostat sample grid (pt-stalk reports every device at
// the same intervals within one -iostat file). In the merged
// multi-Snapshot case, `render.concatIostat` additionally NaN-pads
// devices that were absent in some Snapshots so that every
// DeviceSeries ends up the same length and aligned to a single,
// shared timestamp axis — uPlot requires matched-length series on
// a shared x-axis, and the chart payload treats
// `Devices[0].Utilization.Samples` as authoritative.
type IostatData struct {
	// Devices is sorted alphabetically by Device name for
	// deterministic rendering. When multiple Snapshots contribute,
	// Devices is the union of their device sets; within any given
	// Snapshot window, every DeviceSeries carries exactly one sample
	// per shared-axis timestamp. Samples that originate from a
	// Snapshot where a device was absent carry a NaN
	// `Measurements["util_percent"]` / `["avgqu_sz"]` value (the
	// chart payload surfaces these as JSON null so the plot draws a
	// visible gap).
	Devices []DeviceSeries

	// SnapshotBoundaries lists the sample indexes at which a new
	// Snapshot's first sample sits within the shared axis (always
	// includes 0 for the first sample). For single-Snapshot
	// collections SnapshotBoundaries == [0]. See FR-018 / FR-030 and
	// research R9 for the rendering semantics; the renderer draws a
	// vertical boundary marker at each boundary's timestamp.
	SnapshotBoundaries []int
}

// DeviceSeries is the per-device iostat time-series.
type DeviceSeries struct {
	Device       string
	Utilization  MetricSeries // metric "util_percent", unit "%"
	AvgQueueSize MetricSeries // metric "avgqu_sz", unit "count"
}

// TopData is the typed payload for a -top SourceFile (or, when the
// render layer merges multiple -top files across Snapshots per spec
// FR-018, a concatenation of them onto a single time axis).
type TopData struct {
	// ProcessSamples is every per-process observation across all top
	// batches, in ascending Timestamp order then ascending PID order.
	ProcessSamples []ProcessSample

	// Top3ByAverage holds at most three ProcessSeries — the three
	// processes with the highest average CPUPercent across all
	// samples in the collection (absent-in-sample counts as zero, per
	// spec FR-010 "aggregate across the collection window" and the
	// F7 remediation in the analyze pass).
	Top3ByAverage []ProcessSeries

	// SnapshotBoundaries lists the sample indexes at which a new
	// Snapshot's first sample sits within the concatenated time axis.
	// Same semantics as IostatData.SnapshotBoundaries.
	SnapshotBoundaries []int
}

// ProcessSample is one (timestamp, PID) row from a -top batch.
type ProcessSample struct {
	Timestamp  time.Time
	PID        int
	Command    string
	CPUPercent float64
}

// ProcessSeries is a per-process time-series, produced by pivoting
// ProcessSample rows for the top-3 processes.
type ProcessSeries struct {
	PID     int
	Command string
	CPU     MetricSeries
}

// VariablesData is the typed payload for a -variables SourceFile.
type VariablesData struct {
	// Entries is sorted alphabetically by Name and deduplicated (first
	// global wins per spec FR-012). Duplicates are recorded as
	// SeverityWarning diagnostics on the SourceFile.
	Entries []VariableEntry
}

// VmstatData is the typed payload for a -vmstat SourceFile.
type VmstatData struct {
	// Series is in a fixed declared order so the rendered chart is
	// deterministic even when some measurements are absent from a
	// particular vmstat version. See parse/vmstat.go for the full
	// ordering. A column absent from the source does not drop its
	// MetricSeries slot: the parser's per-row map lookup returns
	// float64's zero-value for the missing key, so every MetricSeries
	// ends up with the same sample count as the number of parsed data
	// rows. The canonical Metric name is still set. Column absence is
	// otherwise silent in the parsed data model: the parser does NOT
	// emit a Diagnostic for a missing column today and does NOT
	// produce a zero-length Samples slice. When multiple Snapshots
	// contribute (spec FR-018) each Series.Samples is the
	// concatenation of per-Snapshot samples in Snapshot order.
	Series []MetricSeries

	// SnapshotBoundaries lists the sample indexes at which a new
	// Snapshot's first sample sits within the concatenated time axis.
	// Same semantics as IostatData.SnapshotBoundaries.
	SnapshotBoundaries []int
}

// MeminfoData is the typed payload for a -meminfo SourceFile (a
// snapshot of /proc/meminfo captured once per second for the
// snapshot window, TS-delimited). A curated set of series (the
// fields meaningful for DB capacity + pressure analysis) is emitted
// in the fixed order declared in parse/meminfo.go. All values are
// reported in gigabytes for chart readability.
type MeminfoData struct {
	Series             []MetricSeries
	SnapshotBoundaries []int
}

// InnodbStatusData is the typed payload for an -innodbstatus1
// SourceFile. Most fields are point-in-time scalars rather than
// time-series — "-innodbstatus1" is a snapshot taken once per
// pt-stalk collection pass.
type InnodbStatusData struct {
	SemaphoreCount int // total threads currently waiting (sum of SemaphoreSites[*].WaitCount)
	PendingReads   int // aggregate pending IO reads (summed across BP instances)
	PendingWrites  int // aggregate pending IO writes (summed across BP instances)
	// Pending writes broken down by flushing path. Summed across BP
	// instances. Each path maps to a distinct diagnostic story — see
	// render/innodb.go for how they drive the Pending I/O card colour.
	//
	//   LRU        → eviction-path pressure (free list exhausted)
	//   FlushList  → checkpoint-age pressure (redo saturation)
	//   SinglePage → synchronous stall — a user thread waited for a
	//                clean page (ALWAYS bad; promotes to critical).
	PendingWritesLRU        int
	PendingWritesFlushList  int
	PendingWritesSinglePage int
	// Pending fsync queues from the FILE I/O section's
	// "Pending flushes (fsync) log: X; buffer pool: Y" line. Non-zero
	// log fsync backlog is a user-visible commit stall; non-zero
	// buffer-pool fsync backlog indicates the page cleaners can't keep
	// up with the I/O layer.
	PendingFsyncLog        int
	PendingFsyncBufferPool int
	// ModifiedDBPages is the total dirty-page backlog across all BP
	// instances at the snapshot moment.
	ModifiedDBPages   int
	AHIActivity       AHIActivity // adaptive-hash-index view
	HistoryListLength int         // "HLL"
	// SemaphoreSites groups the SEMAPHORES-section "--Thread … has
	// waited at FILE line N the semaphore:" records by (file, line,
	// mutex name) so the reader can see the contention hotspots
	// instead of a single total. Sorted descending by WaitCount,
	// tie-broken by File / Line ascending for stability.
	SemaphoreSites []SemaphoreSite
}

// SemaphoreSite is one row of the aggregated SEMAPHORES contention
// breakdown — all threads waiting at the same caller line on the
// same mutex.
type SemaphoreSite struct {
	File      string // e.g. "trx0sys.h"
	Line      int    // e.g. 602
	MutexName string // e.g. "TRX_SYS" (empty when the partner "Mutex ..." line couldn't be associated)
	WaitCount int
}

// SortSemaphoreSites orders sites in the canonical rendering order:
// WaitCount descending first, then stable tie-break by File
// ascending, Line ascending, MutexName ascending. Both parse/ and
// render/ use this to guarantee identical bytes across the two layers
// (NIT #44). Sorts in place; does not copy.
func SortSemaphoreSites(sites []SemaphoreSite) {
	sort.SliceStable(sites, func(i, j int) bool {
		if sites[i].WaitCount != sites[j].WaitCount {
			return sites[i].WaitCount > sites[j].WaitCount
		}
		if sites[i].File != sites[j].File {
			return sites[i].File < sites[j].File
		}
		if sites[i].Line != sites[j].Line {
			return sites[i].Line < sites[j].Line
		}
		return sites[i].MutexName < sites[j].MutexName
	})
}

// AHIActivity summarises InnoDB adaptive-hash-index metrics at the
// snapshot moment.
type AHIActivity struct {
	HashTableSize         int
	SearchesPerSec        float64
	NonHashSearchesPerSec float64
}

// MysqladminData is the typed payload for a -mysqladmin SourceFile
// (repeated SHOW GLOBAL STATUS snapshots). See spec FR-028 and
// research R8 for the pt-mext-derived algorithm; see spec FR-030 and
// research R8 improvement D for the snapshot-boundary reset behaviour.
type MysqladminData struct {
	// VariableNames is sorted alphabetically. Rendering iterates this
	// slice, not the Deltas map.
	VariableNames []string

	// SampleCount is the total number of SHOW GLOBAL STATUS columns
	// observed in the source (concatenated across snapshots when
	// multiple pt-stalk snapshots include this collector).
	SampleCount int

	// Timestamps has length SampleCount, sorted ascending.
	Timestamps []time.Time

	// Deltas maps variable name -> per-sample value slice of length
	// SampleCount.
	//
	// For counters (IsCounter[v] == true):
	//   - Deltas[v][0] is the raw initial tally of the first sample
	//     (matching pt-mext line 47). Excluded from aggregate stats.
	//   - For every boundary index b > 0 in SnapshotBoundaries,
	//     Deltas[v][b] is math.NaN() (per FR-030).
	//   - Other slots hold current - previous deltas.
	//
	// For gauges (IsCounter[v] == false):
	//   - Deltas[v][i] holds the raw per-sample value, not a delta.
	//
	// A variable missing from some samples contributes math.NaN()
	// at the missing slot and emits a SeverityWarning Diagnostic
	// (research R8 improvement C).
	Deltas map[string][]float64

	// IsCounter classifies each variable as a monotonic counter
	// (true) or a point-in-time gauge (false). Keys mirror the
	// VariableNames set. Classification is decided by
	// parse.classifyAsCounter, which follows Percona mysqld_exporter's
	// rules (collector/global_status.go) augmented by an explicit
	// overrides file at parse/mysql-status-types.json.
	IsCounter map[string]bool

	// SnapshotBoundaries lists the sample indexes at which a new
	// -mysqladmin file's first sample sits. Always includes 0.
	// Rendering draws a vertical boundary marker at each
	// corresponding timestamp.
	SnapshotBoundaries []int
}

// ProcesslistData is the typed payload for a -processlist SourceFile.
type ProcesslistData struct {
	// ThreadStateSamples is one record per -processlist snapshot,
	// ordered by Timestamp ascending. Each record carries per-state
	// counts plus parallel buckets for other grouping dimensions
	// (User / Host / Command / db). When multiple Snapshots contribute
	// (spec FR-018) the slice concatenates per-Snapshot samples.
	ThreadStateSamples []ThreadStateSample

	// States is the canonical rendering order of state labels across
	// the collection: sorted alphabetically with "Other" always last
	// if present. For multi-Snapshot collections States is the union.
	States []string

	// Users / Hosts / Commands / Dbs are the canonical label orders
	// for the matching grouping dimension. Same sorting semantics as
	// States (alphabetical, "Other" last when present).
	Users    []string
	Hosts    []string
	Commands []string
	Dbs      []string

	// SnapshotBoundaries lists the sample indexes at which a new
	// Snapshot's first sample sits within the concatenated time axis.
	// Same semantics as IostatData.SnapshotBoundaries.
	SnapshotBoundaries []int

	// ObservedQueries is a deterministically sorted list of active
	// processlist query shapes observed across the data set. Parser
	// outputs may keep all fingerprints so later snapshot concatenation
	// can merge complete evidence; render-facing merged data is bounded.
	ObservedQueries []ObservedProcesslistQuery
}

// ObservedProcesslistQuery is one grouped active SQL shape observed in
// processlist rows. It is intentionally a summary, not a raw row dump:
// repeated sightings of the same normalized query shape are merged, snippets
// are bounded, and context fields come from the slowest sighting.
type ObservedProcesslistQuery struct {
	// Fingerprint is a stable identifier derived from normalized query text.
	Fingerprint string

	// Snippet is a compact, bounded query shape suitable for report display.
	Snippet string

	// FirstSeen is the first sample timestamp where this query shape appeared.
	FirstSeen time.Time

	// LastSeen is the last sample timestamp where this query shape appeared.
	LastSeen time.Time

	// SeenSamples counts eligible processlist rows grouped into this summary.
	SeenSamples int

	// MaxTimeMS is the largest observed age for this query shape.
	MaxTimeMS float64

	// HasTimeMetric reports whether MaxTimeMS came from Time_ms or Time.
	HasTimeMetric bool

	// MaxRowsExamined is the highest observed Rows_examined value.
	MaxRowsExamined float64

	// HasRowsExaminedMetric reports whether Rows_examined was observed.
	HasRowsExaminedMetric bool

	// MaxRowsSent is the highest observed Rows_sent value.
	MaxRowsSent float64

	// HasRowsSentMetric reports whether Rows_sent was observed.
	HasRowsSentMetric bool

	// User is the MySQL user from the slowest observed sighting.
	User string

	// DB is the database from the slowest observed sighting.
	DB string

	// Command is the processlist command from the slowest observed sighting.
	Command string

	// State is the processlist state from the slowest observed sighting.
	State string
}

// NewObservedProcesslistQuery returns a grouped-query seed for one eligible
// active processlist row. The boolean is false when the row should not
// participate in slow-query ranking, such as Sleep/Daemon/empty commands or
// empty/NULL query text.
func NewObservedProcesslistQuery(
	ts time.Time,
	user string,
	db string,
	command string,
	state string,
	info string,
	timeMS float64,
	hasTime bool,
	rowsExamined float64,
	hasRowsExamined bool,
	rowsSent float64,
	hasRowsSent bool,
) (ObservedProcesslistQuery, bool) {
	command = strings.TrimSpace(command)
	if command == "" || strings.EqualFold(command, "Sleep") || strings.EqualFold(command, "Daemon") {
		return ObservedProcesslistQuery{}, false
	}
	if !hasProcesslistQueryText(info) {
		return ObservedProcesslistQuery{}, false
	}

	normalized := NormalizeProcesslistQuery(info)
	fingerprint := processlistQueryFingerprint(normalized)
	return ObservedProcesslistQuery{
		Fingerprint:           fingerprint,
		Snippet:               boundProcesslistSnippet(normalized),
		FirstSeen:             ts,
		LastSeen:              ts,
		SeenSamples:           1,
		MaxTimeMS:             timeMS,
		HasTimeMetric:         hasTime,
		MaxRowsExamined:       rowsExamined,
		HasRowsExaminedMetric: hasRowsExamined,
		MaxRowsSent:           rowsSent,
		HasRowsSentMetric:     hasRowsSent,
		User:                  emptyAsOther(user),
		DB:                    nullOrEmptyAsOther(db),
		Command:               command,
		State:                 nullOrEmptyAsOther(state),
	}, true
}

// MergeObservedProcesslistQueries groups query sightings by fingerprint,
// updates first/last seen bounds, keeps peak metrics, sorts deterministically,
// and returns at most MaxObservedProcesslistQueries rows.
func MergeObservedProcesslistQueries(groups ...[]ObservedProcesslistQuery) []ObservedProcesslistQuery {
	out := MergeAllObservedProcesslistQueries(groups...)
	if len(out) > MaxObservedProcesslistQueries {
		out = out[:MaxObservedProcesslistQueries]
	}
	return out
}

// MergeAllObservedProcesslistQueries groups query sightings by fingerprint,
// updates first/last seen bounds, keeps peak metrics, and sorts
// deterministically without applying the render-facing top-N cap.
func MergeAllObservedProcesslistQueries(groups ...[]ObservedProcesslistQuery) []ObservedProcesslistQuery {
	byFingerprint := map[string]ObservedProcesslistQuery{}
	for _, group := range groups {
		for _, q := range group {
			if q.Fingerprint == "" {
				continue
			}
			current, ok := byFingerprint[q.Fingerprint]
			if !ok {
				byFingerprint[q.Fingerprint] = q
				continue
			}
			current.SeenSamples += q.SeenSamples
			if current.FirstSeen.IsZero() || (!q.FirstSeen.IsZero() && q.FirstSeen.Before(current.FirstSeen)) {
				current.FirstSeen = q.FirstSeen
			}
			if q.LastSeen.After(current.LastSeen) {
				current.LastSeen = q.LastSeen
			}
			if q.HasTimeMetric {
				if !current.HasTimeMetric || q.MaxTimeMS > current.MaxTimeMS {
					current.MaxTimeMS = q.MaxTimeMS
					current.HasTimeMetric = true
					current.User = q.User
					current.DB = q.DB
					current.Command = q.Command
					current.State = q.State
				}
			}
			if q.HasRowsExaminedMetric {
				current.HasRowsExaminedMetric = true
				if q.MaxRowsExamined > current.MaxRowsExamined {
					current.MaxRowsExamined = q.MaxRowsExamined
				}
			}
			if q.HasRowsSentMetric {
				current.HasRowsSentMetric = true
				if q.MaxRowsSent > current.MaxRowsSent {
					current.MaxRowsSent = q.MaxRowsSent
				}
			}
			byFingerprint[q.Fingerprint] = current
		}
	}

	out := make([]ObservedProcesslistQuery, 0, len(byFingerprint))
	for _, q := range byFingerprint {
		out = append(out, q)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.MaxTimeMS != b.MaxTimeMS {
			return a.MaxTimeMS > b.MaxTimeMS
		}
		if a.MaxRowsExamined != b.MaxRowsExamined {
			return a.MaxRowsExamined > b.MaxRowsExamined
		}
		if a.MaxRowsSent != b.MaxRowsSent {
			return a.MaxRowsSent > b.MaxRowsSent
		}
		return a.Fingerprint < b.Fingerprint
	})
	return out
}

// NormalizeProcesslistQuery returns a stable query shape by collapsing
// whitespace, lowercasing, and replacing quoted and numeric literals with a
// placeholder.
func NormalizeProcesslistQuery(query string) string {
	shape := strings.Join(strings.Fields(strings.TrimSpace(query)), " ")
	shape = strings.ToLower(shape)
	shape = processlistQuotedLiteralRE.ReplaceAllString(shape, "?")
	shape = processlistNumericLiteralRE.ReplaceAllString(shape, "?")
	return shape
}

func processlistQueryFingerprint(normalized string) string {
	sum := sha256.Sum256([]byte(normalized))
	return "q_" + hex.EncodeToString(sum[:8])
}

func boundProcesslistSnippet(s string) string {
	runes := []rune(s)
	if len(runes) <= MaxObservedProcesslistQuerySnippetRunes {
		return s
	}
	if MaxObservedProcesslistQuerySnippetRunes <= 3 {
		return string(runes[:MaxObservedProcesslistQuerySnippetRunes])
	}
	return string(runes[:MaxObservedProcesslistQuerySnippetRunes-3]) + "..."
}

func hasProcesslistQueryText(s string) bool {
	trimmed := strings.TrimSpace(s)
	return trimmed != "" && !strings.EqualFold(trimmed, "NULL")
}

func emptyAsOther(s string) string {
	if strings.TrimSpace(s) == "" {
		return "Other"
	}
	return s
}

func nullOrEmptyAsOther(s string) string {
	if strings.TrimSpace(s) == "" || strings.EqualFold(strings.TrimSpace(s), "NULL") {
		return "Other"
	}
	return s
}

// ThreadStateSample is one record per -processlist sample. Each map
// buckets the snapshot's threads by the named dimension. Buckets with
// zero count may be omitted; the renderer fills zeros using the
// canonical label lists on the parent ProcesslistData for
// deterministic ordering.
type ThreadStateSample struct {
	Timestamp time.Time

	StateCounts   map[string]int
	UserCounts    map[string]int
	HostCounts    map[string]int
	CommandCounts map[string]int
	DbCounts      map[string]int

	// TotalThreads is the number of completed processlist rows in
	// this sample.
	TotalThreads int

	// ActiveThreads is the number of rows whose Command is not
	// exactly "Sleep".
	ActiveThreads int

	// SleepingThreads is the number of rows whose Command is exactly
	// "Sleep".
	SleepingThreads int

	// MaxTimeMS is the largest row age in milliseconds for this
	// sample, derived from Time_ms when present and Time otherwise.
	MaxTimeMS float64

	// HasTimeMetric reports whether MaxTimeMS came from at least one
	// valid Time_ms or Time value in this sample.
	HasTimeMetric bool

	// MaxRowsExamined is the largest Rows_examined value seen in this
	// sample.
	MaxRowsExamined float64

	// HasRowsExaminedMetric reports whether this sample contained at
	// least one valid Rows_examined value.
	HasRowsExaminedMetric bool

	// MaxRowsSent is the largest Rows_sent value seen in this sample.
	MaxRowsSent float64

	// HasRowsSentMetric reports whether this sample contained at least
	// one valid Rows_sent value.
	HasRowsSentMetric bool

	// RowsWithQueryText is the count of rows whose Info field is
	// non-empty and not NULL.
	RowsWithQueryText int

	// HasQueryTextMetric reports whether this sample contained at
	// least one Info field, even when every value was empty or NULL.
	HasQueryTextMetric bool
}

// NetstatSocketsData is the typed payload merged from every
// -netstat SourceFile in the collection. One record per captured
// snapshot, carrying the socket-state histogram plus a flag set when
// any socket had a non-zero Recv-Q or Send-Q at that moment.
type NetstatSocketsData struct {
	// Samples ordered by Timestamp ascending, one per -netstat file.
	Samples []NetstatSocketsSample

	// States is the canonical rendering order across the collection
	// (union of every sample's keys, sorted alphabetically with
	// "Other" always last if present).
	States []string

	// SnapshotBoundaries lists the sample indexes at which a new
	// Snapshot's first sample sits. Same semantics as
	// IostatData.SnapshotBoundaries.
	SnapshotBoundaries []int
}

// NetstatSocketsSample is one per-POLL summary of the socket table.
// pt-stalk writes multiple `netstat -antp` dumps per file separated
// by `TS <epoch>` headers, and parseNetstat emits one sample per TS
// block; counting across polls would inflate state totals and let
// Recv-Q / Send-Q flags sticky-latch from older polls. State counts
// aggregate tcp + tcp6; udp sockets are counted under an "UDP"
// pseudo-state. Recv-Q / Send-Q flags are set when ANY socket in
// that single poll had non-zero queues — a coarse but actionable
// indicator that application draining or kernel send-buffer flushing
// was backlogged at that moment.
type NetstatSocketsSample struct {
	Timestamp    time.Time
	StateCounts  map[string]int
	RecvQNonZero bool
	SendQNonZero bool
}

// NetstatCountersSample is one per-POLL observation of the curated
// `netstat -s` counter set. parseNetstatS emits one sample per TS
// block so concatNetstatS can compute per-poll deltas; collapsing all
// polls in a file into a single endpoint would lose the rate signal
// between snapshot boundaries.
type NetstatCountersSample struct {
	Timestamp time.Time
	Values    map[string]float64
}

// NetstatCountersData is the typed payload merged from every
// -netstat_s SourceFile — aggregate kernel network counters (IP,
// ICMP, TCP, UDP, TcpExt). Shape mirrors MysqladminData so the same
// rendering pipeline (deltas-per-sample stacked/line charts) can
// consume both.
type NetstatCountersData struct {
	// Labels is the canonical counter-name rendering order: curated
	// list we know how to interpret, followed by any additional
	// labels observed in the captures sorted alphabetically.
	Labels []string

	// Timestamps is one epoch-seconds value per sample observed
	// across all snapshots, ascending.
	Timestamps []float64

	// Deltas[name] has len(Timestamps) entries. Slot 0 is always NaN
	// (there is no prior sample to delta against); subsequent slots
	// hold current - previous delta. NaN is also emitted for a slot
	// when either endpoint was missing or the counter went
	// backwards (e.g., counter reset).
	Deltas map[string][]float64

	// SnapshotBoundaries lists the sample indexes at which a new
	// Snapshot's first sample sits.
	SnapshotBoundaries []int
}
