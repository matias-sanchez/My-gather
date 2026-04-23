package render

import (
	"html/template"

	"github.com/matias-sanchez/My-gather/model"
)

// --- Template view (what the .tmpl files actually see) ----------------------

// reportView is the value passed to the top-level template. It
// flattens the Report into fields the template can consume without
// complex logic.
type reportView struct {
	Title              string
	Hostname           string
	Version            string
	GitCommit          string
	GeneratedAtDisplay string
	SnapshotCount      int
	Navigation         []model.NavEntry
	NavGroups          []navGroupView

	EmbeddedCSS     template.CSS
	EmbeddedChartJS template.JS
	EmbeddedAppJS   template.JS
	DataPayload     template.JS // JSON, emitted inside <script type="application/json">
	LogoDataURI     template.URL

	AdvisorBadge   string
	VariablesBadge string
	EnvBadge       string

	// Environment section payload (Host + MySQL panels).
	HasEnvironment bool
	Environment    envView

	// Advisor section payload.
	Findings      []findingView
	AdvisorCounts findingCountsView
	HasFindings   bool

	// OS section payload
	HasIostat      bool
	HasTop         bool
	HasVmstat      bool
	HasMeminfo     bool
	IostatSummary  *iostatSummaryView
	TopSummary     *topSummaryView
	VmstatSummary  *vmstatSummaryView
	MeminfoSummary *meminfoSummaryView

	// Variables section payload
	HasVariables      bool
	VariableSnapshots []variableSnapshotView

	// DB section payload
	HasInnoDB           bool
	InnoDBMetrics       []innoDBMetricView
	HasMysqladmin       bool
	MysqladminVariables []string
	MysqladminCount     int
	MysqladminSelectID  string
	HasProcesslist      bool
}

type variableSnapshotView struct {
	DetailsID string
	Title     string
	Badge     string
	// rangeLo / rangeHi are the 1-based inclusive snapshot numbers
	// this kept panel covers. Canonical numeric state for the dedup
	// range; the presentation string RangeNote is derived from them
	// once at the end of buildView and never parsed back.
	rangeLo int
	rangeHi int
	// RangeNote is the dedup subtitle shown inside the snapshot body
	// when this kept panel represents a run of identical snapshots
	// (e.g. "Identical values seen in snapshots #2–#10"). Empty for
	// unique snapshots. Derived from rangeLo/rangeHi; do not mutate
	// directly.
	RangeNote     string
	Count         int
	ModifiedCount int
	// ChangedCount is the number of rows flagged .Changed — useful
	// for showing a "N changed" hint on panels that aren't the first.
	ChangedCount int
	Entries      []variableRowView
}

type variableRowView struct {
	Name   string
	Value  string
	Status string // "default" | "modified" | "unknown"
	// Changed marks this variable as different from the previous kept
	// snapshot. Volatile variables (gtid_executed/gtid_purged) are
	// intentionally never marked so the highlight doesn't light up
	// every snapshot on a busy replica.
	Changed bool
}

// innoDBMetricView holds one row in the aggregated InnoDB status
// callout grid: a single "worst" (max) value for the whole capture
// window with a small min/avg/max line below. One view per metric
// replaces the previous per-snapshot card repetition so the reader
// lands on the peak pain point immediately.
type innoDBMetricView struct {
	// Key is a stable non-display identifier used by the template to
	// branch on behaviour (e.g. render the history-list sparkline
	// container) without depending on Label text that could be
	// reworded or localized. Empty for metrics with no special
	// template behaviour.
	Key   string // "" | "semaphores" | "pending_io" | "ahi" | "history_list"
	Label string // "Semaphores" / "Pending I/O" / "AHI" / "History list"
	Hint  string // tiny descriptor shown under Worst (optional)
	Worst string
	Min   string
	Avg   string
	Max   string
	// Semaphores-only: per-site wait breakdowns. When populated,
	// db.html.tmpl renders a collapsible "contention breakdown"
	// <details> under the card with two toggleable views:
	//   - Peak: breakdown from the single snapshot whose count
	//     equals Max ("at peak · snapshot <prefix> · N waits").
	//   - Total: sum of waits over ALL snapshots, by the same
	//     (file, line, mutex) key.
	// Empty when no snapshot ever saw >0 waits.
	BreakdownPeak         []semaphoreSiteRow
	BreakdownPeakSnapshot string
	BreakdownPeakTotal    int
	BreakdownTotal        []semaphoreSiteRow
	BreakdownTotalTotal   int

	// AHI-only: formula breakdown exposed when the card represents
	// the Adaptive Hash Index hit ratio. The text fields are all
	// pre-rendered so the template can concatenate them into a
	// readable "hit ratio = hash / (hash + non-hash) = X / (X + Y)
	// = Z%" line.
	AHIFormula       bool
	AHIFormulaText   string // "hash / (hash + non-hash)"
	AHIWorstSnapshot string
	AHIWorstHash     string // "3901.21"
	AHIWorstNonHash  string // "8669.42"
	AHIWorstRatio    string // "31.0"
	AHIHashTableSize string // "34,679"
	// AHIHasHashTable is true when HashTableSize > 0 and therefore
	// safe to surface in the "hash table size" caption. Templates
	// gate on this flag instead of string-comparing the formatted
	// AHIHashTableSize against the dash placeholder.
	AHIHasHashTable bool
	AHINoActivity   bool // true when every snapshot had hash+non-hash == 0
}

// semaphoreSiteRow is one row in the contention-breakdown sub-panel.
// Percent is computed against the view's total (peak-snapshot total
// or window total), so each view's percents sum to 100.
type semaphoreSiteRow struct {
	File      string
	Line      int
	MutexName string
	Count     int
	Percent   string // formatted, e.g. "94.9"
}

// navGroupView is the template-facing shape of one Level-1 nav entry
// plus its Level-2 children. Built from the flat Report.Navigation.
type navGroupView struct {
	ID       string
	Title    string
	Children []navChildView
}

type navChildView struct {
	ID    string
	Title string
}

// findingView is the template-facing shape of a findings.Finding.
type findingView struct {
	ID              string
	Subsystem       string
	Title           string
	SeverityClass   string // "crit" | "warn" | "info" | "ok"
	SeverityLabel   string // "Critical" | "Warning" | "Info" | "OK"
	OpenByDefault   bool   // crit findings start expanded
	Summary         string
	Explanation     string
	FormulaText     string
	FormulaComputed string
	Metrics         []findingMetricView
	Recommendations []string
}

type findingMetricView struct {
	Name  string
	Value string
	Unit  string
	Note  string
}

type findingCountsView struct {
	Crit int
	Warn int
	Info int
	OK   int
	Any  bool
}

type iostatSummaryView struct {
	PeakUtil        string
	PeakDevice      string
	PeakAqusz       string
	PeakAquszDevice string
	DeviceCount     int
	SampleCount     int
}

type topSummaryView struct {
	First     string
	FirstAvg  string
	Second    string
	SecondAvg string
	Third     string
	ThirdAvg  string
	// MysqldExtra is populated only when mysqld is NOT already one of
	// the top-3 by average — rendering it as a 4th chip (and a 4th
	// series in the chart) so the MySQL process is always visible.
	MysqldExtra    string
	MysqldExtraAvg string
	SampleCount    int
}

type vmstatSummaryView struct {
	PeakRunqueue string
	PeakBlocked  string
	PeakIowait   string
	SampleCount  int
}

type meminfoSummaryView struct {
	// All values are formatted in GB to match the chart axis.
	MinAvailable string // smallest MemAvailable seen — "pressure floor"
	MaxAnonPages string // peak process-memory footprint
	MaxDirty     string // peak dirty-page backlog (fsync pressure)
	MaxSwapUsed  string // peak swap pressure
	SampleCount  int
}
