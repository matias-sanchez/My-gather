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
	"strings"
	"sync"

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

// DiagnosticCategory describes what kind of diagnostic signal a
// finding represents in the USE-style Advisor model.
type DiagnosticCategory string

const (
	// CategoryUtilization marks capacity or throughput usage without
	// necessarily implying queueing or failed work.
	CategoryUtilization DiagnosticCategory = "Utilization"

	// CategorySaturation marks waits, queues, misses, or pressure that
	// indicates demand is exceeding available capacity.
	CategorySaturation DiagnosticCategory = "Saturation"

	// CategoryError marks failed work or explicit error evidence.
	CategoryError DiagnosticCategory = "Error"

	// CategoryCombined marks findings that combine multiple diagnostic
	// categories into one user-visible conclusion.
	CategoryCombined DiagnosticCategory = "Combined"
)

// EvidenceKind classifies how directly an evidence row was observed.
type EvidenceKind string

const (
	// EvidenceDirectMeasurement is a raw numeric value from the capture.
	EvidenceDirectMeasurement EvidenceKind = "direct measurement"

	// EvidenceDerivedRate is a rate computed from captured counters.
	EvidenceDerivedRate EvidenceKind = "derived rate"

	// EvidenceDerivedRatio is a ratio computed from captured values.
	EvidenceDerivedRatio EvidenceKind = "derived ratio"

	// EvidenceObservedState is a processlist or InnoDB state observed
	// directly in the capture.
	EvidenceObservedState EvidenceKind = "observed state"

	// EvidenceObservedError is explicit error text or an error counter.
	EvidenceObservedError EvidenceKind = "observed error"

	// EvidenceInference is an interpretation derived from other evidence.
	EvidenceInference EvidenceKind = "inference"
)

// EvidenceStrength classifies how strongly an evidence item supports
// the finding.
type EvidenceStrength string

const (
	// EvidenceStrong is direct or high-confidence evidence.
	EvidenceStrong EvidenceStrength = "Strong"

	// EvidenceModerate is useful supporting evidence.
	EvidenceModerate EvidenceStrength = "Moderate"

	// EvidenceWeak is contextual evidence that should not drive
	// escalation by itself.
	EvidenceWeak EvidenceStrength = "Weak"
)

// Confidence communicates how much evidence supports a finding.
type Confidence string

const (
	// ConfidenceHigh means the finding is backed by direct or multiple
	// supporting signals.
	ConfidenceHigh Confidence = "High"

	// ConfidenceMedium means the finding is supported but may need
	// confirmation from adjacent evidence.
	ConfidenceMedium Confidence = "Medium"

	// ConfidenceLow means the finding is useful context but should not
	// be treated as a primary driver by itself.
	ConfidenceLow Confidence = "Low"
)

// RecommendationKind describes the intent of a recommendation.
type RecommendationKind string

const (
	// RecommendationConfirm asks the analyst to verify the diagnosis.
	RecommendationConfirm RecommendationKind = "Confirm"

	// RecommendationInvestigate points to the next evidence to inspect.
	RecommendationInvestigate RecommendationKind = "Investigate"

	// RecommendationMitigate suggests a bounded operational change.
	RecommendationMitigate RecommendationKind = "Mitigate"

	// RecommendationCaution scopes or limits the advice.
	RecommendationCaution RecommendationKind = "Caution"
)

// RelationshipKind describes how two findings are related.
type RelationshipKind string

const (
	// RelationshipSupports means one finding provides supporting
	// evidence for another.
	RelationshipSupports RelationshipKind = "Supports"

	// RelationshipMayExplain means one finding may explain another.
	RelationshipMayExplain RelationshipKind = "May explain"

	// RelationshipMayBeCausedBy means one finding may be caused by
	// another.
	RelationshipMayBeCausedBy RelationshipKind = "May be caused by"

	// RelationshipSamePressure means two findings are symptoms of the
	// same pressure pattern.
	RelationshipSamePressure RelationshipKind = "Same pressure"
)

// MetricRef pins one numeric input the rule consumed.
type MetricRef struct {
	Name  string
	Value float64
	Unit  string // "/s", "%", "bytes", "count"
	Note  string // e.g. "delta over capture window"
}

// EvidenceRef is one user-visible fact supporting a Finding.
type EvidenceRef struct {
	Name     string
	Value    string
	Unit     string
	Kind     EvidenceKind
	Strength EvidenceStrength
	Note     string
}

// Recommendation is one structured Advisor next step.
type Recommendation struct {
	Kind            RecommendationKind
	Text            string
	AppliesWhen     string
	RelatedEvidence []string
}

// RelatedFinding points from one finding to another correlated
// finding in the same Advisor result set.
type RelatedFinding struct {
	ID           string
	Relationship RelationshipKind
	Reason       string
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

	// Category describes the USE-style diagnostic class represented
	// by the dominant signal.
	Category DiagnosticCategory

	// Confidence communicates evidence strength to the analyst.
	Confidence Confidence

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

	// Evidence is the structured evidence bundle rendered in the
	// Advisor card. Rules may set this directly; otherwise Analyze
	// derives it from Metrics.
	Evidence []EvidenceRef

	// Recommendations: bulleted remediation steps.
	Recommendations []string

	// RecommendationItems is the structured form of Recommendations.
	// Rules may set it directly; otherwise Analyze derives it from
	// Recommendations.
	RecommendationItems []Recommendation

	// RelatedFindings lists correlated findings that should be shown
	// as supporting context in the Advisor UI.
	RelatedFindings []RelatedFinding

	// RelatedFindingIDs is the compact ID-only relation list used by
	// tests and simple rule declarations.
	RelatedFindingIDs []string

	// Source: Rosetta Stone anchor — "Rosetta Stone — Buffer Pool
	// §Utilization (throughput)" or similar.
	Source string

	// CoverageTopic is the Rosetta Stone topic represented by this
	// finding in the coverage map.
	CoverageTopic string
}

// registrySortOnce ensures the registry is sorted exactly once,
// after every per-file init() has populated it. Go runs in-package
// init()s in source-file alphabetical order, so any sort placed in a
// findings.go-level init() would execute before files like
// register.go and rules_*.go contribute their entries. Doing the
// sort lazily on first Analyze() call sidesteps that ordering trap.
var registrySortOnce sync.Once

// Analyze runs every registered rule against the given Report and
// returns the non-Skip findings in a deterministic order: by
// Subsystem, then by descending Severity (Crit first), then by ID
// alphabetically. Internally it iterates the package-level registry,
// which is sorted by RuleDefinition.ID on first call so the dispatch
// order is stable independent of source-file or build order.
func Analyze(r *model.Report) []Finding {
	registrySortOnce.Do(sortRegistry)
	if r == nil {
		return nil
	}
	out := make([]Finding, 0, len(registry))
	for _, def := range registry {
		f := def.Run(r)
		if f.Severity == SeveritySkip {
			continue
		}
		f = enrichFinding(def, f)
		out = append(out, f)
	}
	out = correlateFindings(out)
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

func enrichFinding(def RuleDefinition, f Finding) Finding {
	if f.ID == "" {
		f.ID = def.ID
	}
	if f.Subsystem == "" {
		f.Subsystem = def.Subsystem
	}
	if f.Title == "" {
		f.Title = def.Title
	}
	if f.FormulaText == "" {
		f.FormulaText = def.FormulaText
	}
	if f.Category == "" {
		f.Category = def.Category
	}
	if f.CoverageTopic == "" {
		f.CoverageTopic = def.CoverageTopic
	}
	if f.Confidence == "" {
		f.Confidence = inferConfidence(f)
	}
	if len(f.Evidence) == 0 {
		f.Evidence = evidenceFromMetrics(f.Metrics)
	}
	if len(f.RecommendationItems) == 0 {
		f.RecommendationItems = recommendationsFromText(f.Recommendations)
	}
	for _, id := range f.RelatedFindingIDs {
		f.RelatedFindings = append(f.RelatedFindings, RelatedFinding{
			ID:           id,
			Relationship: RelationshipSupports,
			Reason:       "Related Advisor evidence may describe the same pressure pattern.",
		})
	}
	return f
}

func correlateFindings(fs []Finding) []Finding {
	if len(fs) < 2 {
		return fs
	}
	seen := make(map[string]struct{}, len(fs))
	for _, f := range fs {
		seen[f.ID] = struct{}{}
	}
	add := func(fromID, toID string, relationship RelationshipKind, reason string) {
		if _, ok := seen[fromID]; !ok {
			return
		}
		if _, ok := seen[toID]; !ok {
			return
		}
		for i := range fs {
			if fs[i].ID != fromID {
				continue
			}
			if hasRelatedFinding(fs[i], toID) {
				return
			}
			fs[i].RelatedFindings = append(fs[i].RelatedFindings, RelatedFinding{
				ID:           toID,
				Relationship: relationship,
				Reason:       reason,
			})
			return
		}
	}

	add("bp.wait_free", "bp.free_pages_low", RelationshipSupports, "Free-page exhaustion supports query waits for buffer pool pages.")
	add("bp.wait_free", "bp.lru_flushing", RelationshipSamePressure, "LRU flushing and wait_free both indicate free-list pressure.")
	add("bp.free_pages_low", "bp.lru_flushing", RelationshipSamePressure, "Both findings describe buffer pool free-list pressure.")
	add("innodb.flushing", "bp.dirty_pct", RelationshipSupports, "Dirty-page pressure can force more aggressive flushing.")
	add("innodb.flushing", "redo.checkpoint_age", RelationshipMayBeCausedBy, "Redo checkpoint pressure can drive flushing pressure.")
	add("redo.checkpoint_age", "innodb.flushing", RelationshipMayExplain, "Flush pressure may be part of checkpoint-age growth.")
	add("redo.pending_writes", "redo.pending_fsyncs", RelationshipSamePressure, "Both findings point to redo log I/O latency.")
	add("tablecache.overflows", "tablecache.miss_ratio", RelationshipSamePressure, "Overflow and miss-ratio symptoms both point to table cache pressure.")
	add("tablecache.overflows", "tablecache.usage", RelationshipSupports, "High usage reduces table cache headroom.")
	add("queryshape.select_full_join", "queryshape.handler_read_rnd_next", RelationshipSupports, "Full joins often explain high sequential row-read volume.")
	add("queryshape.select_scan", "queryshape.handler_read_rnd_next", RelationshipSupports, "First-table scans can contribute to sequential row reads.")
	add("queryshape.observed_slow_processlist", "queryshape.select_full_join", RelationshipSupports, "Slow active query evidence should be compared with join-scan counters.")
	add("queryshape.observed_slow_processlist", "queryshape.select_scan", RelationshipSupports, "Slow active query evidence should be compared with scan counters.")
	add("connections.saturation", "thread.max_queue_depth", RelationshipSamePressure, "Connection pressure and running-thread peaks can describe the same concurrency surge.")
	add("config.max_connections_high", "connections.saturation", RelationshipMayExplain, "Connection-limit configuration changes how saturation should be interpreted.")
	return fs
}

func hasRelatedFinding(f Finding, id string) bool {
	for _, rel := range f.RelatedFindings {
		if rel.ID == id {
			return true
		}
	}
	return false
}

func evidenceFromMetrics(metrics []MetricRef) []EvidenceRef {
	out := make([]EvidenceRef, 0, len(metrics))
	for _, m := range metrics {
		out = append(out, EvidenceRef{
			Name:     m.Name,
			Value:    formatNum(m.Value),
			Unit:     m.Unit,
			Kind:     evidenceKindForMetric(m),
			Strength: evidenceStrengthForMetric(m),
			Note:     m.Note,
		})
	}
	return out
}

func evidenceKindForMetric(m MetricRef) EvidenceKind {
	name := m.Name
	switch {
	case containsAny(name, "/s", "rate"):
		return EvidenceDerivedRate
	case containsAny(name, "ratio", "pct", "%", " per "):
		return EvidenceDerivedRatio
	case containsAny(name, "state", "processlist", "metadata lock"):
		return EvidenceObservedState
	case containsAny(name, "error", "aborted"):
		return EvidenceObservedError
	default:
		return EvidenceDirectMeasurement
	}
}

func evidenceStrengthForMetric(m MetricRef) EvidenceStrength {
	if containsAny(m.Name, "wait", "pending", "metadata lock", "single-page", "error", "aborted") {
		return EvidenceStrong
	}
	if m.Value == 0 {
		return EvidenceWeak
	}
	return EvidenceModerate
}

func recommendationsFromText(items []string) []Recommendation {
	out := make([]Recommendation, 0, len(items))
	for i, text := range items {
		kind := RecommendationInvestigate
		if i == 0 {
			kind = RecommendationConfirm
		}
		if containsAny(text, "Increase", "Raise", "Move", "lower", "grow") {
			kind = RecommendationMitigate
		}
		if containsAny(text, "Check", "Confirm", "Validate", "Review", "Inspect", "Correlate") {
			kind = RecommendationConfirm
		}
		out = append(out, Recommendation{Kind: kind, Text: text})
	}
	hasConfirm := false
	for _, rec := range out {
		if rec.Kind == RecommendationConfirm {
			hasConfirm = true
			break
		}
	}
	if !hasConfirm && len(out) > 0 {
		out[0].Kind = RecommendationConfirm
	}
	return out
}

func inferConfidence(f Finding) Confidence {
	switch {
	case f.Severity == SeverityCrit:
		return ConfidenceHigh
	case len(f.Metrics) >= 2 || len(f.Evidence) >= 2:
		return ConfidenceHigh
	case f.Severity == SeverityOK:
		return ConfidenceMedium
	case f.Severity == SeverityInfo:
		return ConfidenceLow
	default:
		return ConfidenceMedium
	}
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(strings.ToLower(s), strings.ToLower(needle)) {
			return true
		}
	}
	return false
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
	case "InnoDB Semaphores":
		return 2
	case "Binlog Cache":
		return 3
	case "Thread Cache":
		return 4
	case "Table Open Cache":
		return 5
	case "Temp Tables":
		return 6
	case "Query Shape":
		return 7
	case "Connections":
		return 8
	case "Configuration":
		return 9
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
