// Package findings computes interpretive "Advisor" findings from a
// parsed pt-stalk Report. Rules are sourced from the internal MySQL
// Rosetta Stone engineering document (USE methodology: Utilization /
// Saturation / Errors) and produce short, severity-rated statements
// about weak points in the captured data.
//
// Output is deterministic: the same Report yields the same slice of
// Findings in the same order, so the rendered HTML stays byte-stable
// between runs (Constitution Principle IV).
package findings

import (
	"sort"

	"github.com/matias-sanchez/My-gather/model"
)

// Severity classifies a Finding. The three "visible" levels drive the
// UI colour palette; SeveritySkip is filtered out of the final slice
// and never reaches the template.
type Severity int

const (
	// SeveritySkip: rule did not apply (missing inputs, zero signal).
	// Dropped by Analyze before returning.
	SeveritySkip Severity = iota

	// SeverityOK: healthy — formula computed, result is in the safe band.
	SeverityOK

	// SeverityInfo: informational — e.g. a metric worth noting without
	// an associated problem.
	SeverityInfo

	// SeverityWarn: yellow — deserves attention.
	SeverityWarn

	// SeverityCrit: red — needs action.
	SeverityCrit
)

// MetricRef pins one numeric input the rule consumed.
type MetricRef struct {
	Name  string
	Value float64
	Unit  string // "/s", "%", "bytes", "count"
	Note  string // e.g. "delta over capture window"
}

// Finding is one Advisor card rendered in the report.
type Finding struct {
	// ID is a stable slug ("bp.hit_ratio"). Used for deterministic
	// ordering and template element IDs.
	ID string

	// Subsystem is the Rosetta-Stone-derived group label
	// ("Buffer Pool", "Redo Log", ...). Findings are rendered grouped
	// by this value.
	Subsystem string

	// Title is the short human-readable rule name.
	Title string

	// Severity drives the UI colour. Rules that evaluate but don't
	// fire should return SeverityOK (shows a green card). Rules
	// whose inputs are missing should return SeveritySkip (filtered
	// out entirely by Analyze).
	Severity Severity

	// Summary is the one-line headline shown on the closed card.
	Summary string

	// Explanation is 1–3 short paragraphs shown when the card is
	// expanded.
	Explanation string

	// FormulaText is the symbolic formula — e.g.
	// "1 − reads / read_requests".
	FormulaText string

	// FormulaComputed is the same formula with literal values
	// substituted, ending in "= <result> <unit>".
	FormulaComputed string

	// Metrics are the raw numeric inputs the formula used.
	Metrics []MetricRef

	// Recommendations: bulleted remediation steps.
	Recommendations []string

	// Source: Rosetta Stone anchor — "Rosetta Stone — Buffer Pool
	// §Utilization (throughput)" or similar.
	Source string
}

// rule is the unit of composition: a function that takes a Report and
// returns one Finding (or Severity=Skip to opt out).
type rule func(r *model.Report) Finding

// allRules is the deterministically-ordered registry of analytical
// rules. Ordering is by subsystem-group then by rule ID; the exact
// order in this slice does not matter because Analyze sorts the
// results. New rules just append here.
var allRules = []rule{
	// Buffer Pool
	ruleBPHitRatio,
	ruleBPUndersized,
	ruleBPFreePagesLow,
	ruleBPLRUFlushing,
	ruleBPWaitFree,
	ruleBPDirtyPct,

	// Redo Log
	ruleRedoCheckpointAge,
	ruleRedoPendingWrites,
	ruleRedoPendingFsyncs,
	ruleRedoLogWaits,

	// Binlog Cache
	ruleBinlogCacheDiskUse,
	ruleBinlogStmtCacheDiskUse,

	// Thread Cache
	ruleThreadCacheHitRatio,

	// Table Open Cache
	ruleTableCacheOverflows,
	ruleTableCacheMissRatio,

	// Temp Tables
	ruleTmpDiskRatio,

	// Query shape
	ruleFullScanSelectScan,
	ruleFullScanSelectFullJoin,
	ruleFullScanHandlerRndNext,
	ruleProcesslistAbuse,

	// Connections
	ruleAbortedConnectsRate,
	ruleConnectionsSaturation,

	// Configuration
	ruleSlowLogDisabled,
}

// Analyze runs every rule against the given Report and returns the
// non-Skip findings in a deterministic order: by Subsystem, then by
// descending Severity (Crit first), then by ID alphabetically.
func Analyze(r *model.Report) []Finding {
	if r == nil {
		return nil
	}
	out := make([]Finding, 0, len(allRules))
	for _, fn := range allRules {
		f := fn(r)
		if f.Severity == SeveritySkip {
			continue
		}
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Subsystem != b.Subsystem {
			return subsystemOrder(a.Subsystem) < subsystemOrder(b.Subsystem)
		}
		if a.Severity != b.Severity {
			return a.Severity > b.Severity // Crit before Warn before Info before OK
		}
		return a.ID < b.ID
	})
	return out
}

// subsystemOrder returns a canonical rank for each Subsystem name so
// the advisor renders in the same top-to-bottom order as the Rosetta
// Stone document. Unknown subsystems sort to the end, alphabetically
// among themselves.
func subsystemOrder(s string) int {
	switch s {
	case "Buffer Pool":
		return 0
	case "Redo Log":
		return 1
	case "Binlog Cache":
		return 2
	case "Thread Cache":
		return 3
	case "Table Open Cache":
		return 4
	case "Temp Tables":
		return 5
	case "Query Shape":
		return 6
	case "Connections":
		return 7
	case "Configuration":
		return 8
	}
	return 99
}

// Counts summarises the severity distribution. Rendered at the top of
// the Advisor section as three chips.
type Counts struct {
	Crit int
	Warn int
	Info int
	OK   int
}

// Summarise tallies findings by Severity.
func Summarise(fs []Finding) Counts {
	var c Counts
	for _, f := range fs {
		switch f.Severity {
		case SeverityCrit:
			c.Crit++
		case SeverityWarn:
			c.Warn++
		case SeverityInfo:
			c.Info++
		case SeverityOK:
			c.OK++
		}
	}
	return c
}
