package findings

import (
	"sort"

	"github.com/matias-sanchez/My-gather/model"
)

// SeverityHint classifies the expected severity bucket of a rule for
// the rule-quality test harness. It is intentionally separate from the
// dynamic Severity field of Finding: a single rule may evaluate to
// SeverityCrit on one fixture and SeverityOK on another, but it has a
// fixed worst-case severity that the harness can sanity-check against
// (e.g. a CRITICAL-class rule must offer at least three remediation
// recommendations).
type SeverityHint int

const (
	// SeverityHintInfo: rule's worst case is informational.
	SeverityHintInfo SeverityHint = iota

	// SeverityHintWarning: rule's worst case is a warning.
	SeverityHintWarning

	// SeverityHintCritical: rule's worst case is critical.
	SeverityHintCritical

	// SeverityHintVariable: rule may produce CRIT, WARN or INFO
	// depending on inputs. Treated as "could be critical" by the
	// quality harness.
	SeverityHintVariable
)

// RuleDefinition is the structured registration entry for one Advisor
// rule. Each rule publishes its metadata (ID, Subsystem, Title,
// FormulaText, severity hint, minimum remediation count) alongside a
// Run function that produces the actual Finding from a *model.Report.
//
// The registry is the single source of dispatch: Analyze iterates the
// sorted registry and never knows about individual rule functions.
// Every rule registers itself in an init() block in its own
// per-subsystem file, so each rule has exactly one declaration site.
type RuleDefinition struct {
	// ID is a stable, dotted slug used for ordering, deduplication,
	// and as the rendered Finding.ID. Example:
	// "buffer_pool.dirty_pages_high".
	ID string

	// Subsystem is the human-readable group label rendered in the
	// Advisor section (matches Finding.Subsystem). Must be one of the
	// strings handled by subsystemOrder for deterministic rendering.
	Subsystem string

	// Title is the short human-readable rule name recorded in the
	// registry metadata. It is the canonical title used by static
	// rule listings (the rules catalogue), and it is also the value
	// the registered Run function SHOULD return as Finding.Title.
	// Drift between this field and the rendered Finding.Title is
	// not yet enforced by the quality harness; a future cross-check
	// (driven by Registry()) will pin every registration.
	Title string

	// FormulaText is the symbolic formula or threshold the rule
	// applies. Must be non-empty (enforced by TestRuleQuality_-
	// MetadataIsComplete). It is the canonical text used by the
	// rules catalogue and is the value the registered Run function
	// SHOULD return as Finding.FormulaText; like Title, the
	// per-Finding match is left to a future cross-check rather
	// than enforced today.
	FormulaText string

	// Category is the USE-style diagnostic category represented by
	// the rule's dominant signal.
	Category DiagnosticCategory

	// CoverageTopic is the Rosetta Stone topic this rule covers.
	CoverageTopic string

	// MinRecommendations is the minimum number of remediation steps
	// the rule must produce when it fires non-OK. The quality harness
	// requires CRITICAL-bucket rules to have at least 3 and INFO-only
	// rules to have at least 1.
	MinRecommendations int

	// Severity is the expected worst-case severity bucket. See
	// SeverityHint constants.
	Severity SeverityHint

	// Run computes the Finding for the supplied Report. It must
	// return a Finding with Severity == SeveritySkip when the rule's
	// inputs are missing or it has no signal to emit.
	Run func(*model.Report) Finding
}

// registry is the deterministically-ordered registry of RuleDefinition
// entries. Populated at package init by register() helpers in the
// per-subsystem files. The registry is sorted by ID lazily on the
// first Analyze() call via a sync.Once in findings.go
// (registrySortOnce). Sorting at init time would race per-file init
// order — the sort would run before all registrations completed — so
// we defer to first-use; iteration order is still stable across runs
// (Constitution Principle IV) because every Analyze call sees the
// registry post-sort.
//
// Every rule is now a native RuleDefinition: the transitional
// legacyAdapter wrapper that bridged the rule-DSL-v2 migration was
// removed once the legacy count reached 0.
var registry []RuleDefinition

// register appends a RuleDefinition to the package-level registry. It
// is intended to be called from init() in the per-subsystem files.
func register(d RuleDefinition) {
	if d.Category == "" {
		d.Category = inferCategory(d.ID, d.Subsystem, d.Title, d.FormulaText)
	}
	if d.CoverageTopic == "" {
		d.CoverageTopic = defaultCoverageTopic(d.Subsystem, d.Title)
	}
	registry = append(registry, d)
}

// Registry returns a sorted-by-ID copy of the rule registry. Intended
// for external observers (test harnesses, documentation generators)
// that need to inspect rule metadata without mutating dispatch state.
// Calling Registry primes the same lazy sort Analyze uses, so the
// returned slice is in canonical order even on first call.
func Registry() []RuleDefinition {
	registrySortOnce.Do(sortRegistry)
	out := make([]RuleDefinition, len(registry))
	copy(out, registry)
	return out
}

// sortRegistry sorts the registry by RuleDefinition.ID. Called from
// the package-level init in findings.go after every per-file init has
// populated the registry.
func sortRegistry() {
	sort.SliceStable(registry, func(i, j int) bool {
		return registry[i].ID < registry[j].ID
	})
}
