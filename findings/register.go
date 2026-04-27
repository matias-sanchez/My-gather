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
// New rules append a RuleDefinition entry; the legacy free-floating
// rule functions are wrapped via legacyAdapter while their migration
// to native RuleDefinition entries is in flight.
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
	// The quality harness enforces the match for native registrations
	// (TestRuleQuality_RegistryMatchesEmittedFinding); legacy-adapter
	// entries can diverge until they are migrated.
	Title string

	// FormulaText is the symbolic formula or threshold the rule
	// applies. Must be non-empty (enforced by the quality test). It
	// is the canonical text used by the rules catalogue and is the
	// value the registered Run function SHOULD return as
	// Finding.FormulaText; the quality harness enforces the match
	// for native registrations.
	FormulaText string

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
// entries. Populated at package init by register() helpers in this
// file and the per-subsystem files that register native rules. The
// registry is sorted by ID lazily on the first Analyze() call via a
// sync.Once in findings.go (registrySortOnce). Sorting at init time
// would race per-file init order — the sort would run before all
// registrations completed — so we defer to first-use; iteration
// order is still stable across runs (Constitution Principle IV)
// because every Analyze call sees the registry post-sort.
//
// Migration plan: as of this PR 5 of the 26 shipped rules are native
// RuleDefinition entries; the remaining 21 are wrapped with
// legacyAdapter so the registry is the only dispatch path while their
// per-rule conversions ship one-by-one in follow-up PRs. Adapter is
// deleted when the legacy count reaches 0.
var registry []RuleDefinition

// register appends a RuleDefinition to the package-level registry. It
// is intended to be called from init() in the per-subsystem files.
func register(d RuleDefinition) {
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

// legacyAdapter wraps a free-floating rule function in a
// RuleDefinition using metadata supplied explicitly at the call
// site. It does not invoke or introspect the wrapped function;
// callers must provide the ID, Subsystem, Title, FormulaText,
// Severity, and MinRecommendations for the resulting definition.
// The adapter is a transitional bridge: each call site is removed
// once the underlying rule is converted to a native RuleDefinition
// in the per-subsystem file.
//
// The adapter is intentionally minimal: callers must supply
// FormulaText, both because the quality harness rejects empty
// FormulaText and because the symbolic formula is already encoded
// in the Finding returned by each legacy rule function — declaring
// it at the call site keeps the registry's rules catalogue aligned
// with the per-Finding output even for not-yet-migrated rules.
func legacyAdapter(id, subsystem, title, formulaText string, severity SeverityHint, minRecs int, fn func(*model.Report) Finding) RuleDefinition {
	return RuleDefinition{
		ID:                 id,
		Subsystem:          subsystem,
		Title:              title,
		FormulaText:        formulaText,
		MinRecommendations: minRecs,
		Severity:           severity,
		Run:                fn,
	}
}

// init seeds the registry with the legacy rules that have not yet
// been migrated to native RuleDefinition. Native registrations live in
// the per-subsystem files (init() in each), so every rule has exactly
// one declaration site.
//
// After every package init has run, sortRegistry() is called from
// findings.go to enforce by-ID ordering once the registry is fully
// populated.
func init() {
	// 21 of 26 rules ride through legacyAdapter while their native
	// migrations are in flight — see the migration plan in the
	// package-level registry comment above.
	register(legacyAdapter(
		"bp.undersized", "Buffer Pool", "Buffer pool size vs workload",
		"innodb_buffer_pool_size  vs  {>=1 GiB warn floor, >=256 MiB crit floor}  (gated on Threads_connected >= 50 and Uptime >= 1h)",
		SeverityHintVariable, 3, ruleBPUndersized))
	register(legacyAdapter(
		"bp.free_pages_low", "Buffer Pool", "Buffer pool free-page headroom",
		"Innodb_buffer_pool_pages_free  vs  innodb_lru_scan_depth * innodb_buffer_pool_instances",
		SeverityHintVariable, 3, ruleBPFreePagesLow))
	register(legacyAdapter(
		"bp.lru_flushing", "Buffer Pool", "LRU-list page flushing rate",
		"Innodb_buffer_pool_pages_LRU_flushed/s",
		SeverityHintVariable, 2, ruleBPLRUFlushing))
	register(legacyAdapter(
		"bp.wait_free", "Buffer Pool", "Buffer pool wait_free events",
		"Innodb_buffer_pool_wait_free/s > 0",
		SeverityHintCritical, 3, ruleBPWaitFree))
	register(legacyAdapter(
		"bp.dirty_pct", "Buffer Pool", "Buffer pool dirty-page percentage",
		"Innodb_buffer_pool_pages_dirty / Innodb_buffer_pool_pages_total  vs  innodb_max_dirty_pages_pct",
		SeverityHintVariable, 3, ruleBPDirtyPct))
	register(legacyAdapter(
		"redo.checkpoint_age", "Redo Log", "Redo checkpoint age",
		"max(Innodb_checkpoint_age) / Innodb_checkpoint_max_age",
		SeverityHintVariable, 3, ruleRedoCheckpointAge))
	register(legacyAdapter(
		"redo.pending_writes", "Redo Log", "Redo log pending writes",
		"max(Innodb_os_log_pending_writes) > 0",
		SeverityHintCritical, 3, ruleRedoPendingWrites))
	register(legacyAdapter(
		"redo.pending_fsyncs", "Redo Log", "Redo log pending fsyncs",
		"max(Innodb_os_log_pending_fsyncs) > 0",
		SeverityHintCritical, 3, ruleRedoPendingFsyncs))
	register(legacyAdapter(
		"redo.log_waits", "Redo Log", "Redo log buffer waits",
		"Innodb_log_waits/s > 0",
		SeverityHintVariable, 3, ruleRedoLogWaits))
	register(legacyAdapter(
		"binlog.cache_disk_use", "Binlog Cache", "Binlog cache spilled to disk",
		"Binlog_cache_disk_use/s > 0",
		SeverityHintVariable, 1, ruleBinlogCacheDiskUse))
	register(legacyAdapter(
		"binlog.stmt_cache_disk_use", "Binlog Cache", "Statement-binlog cache spilled to disk",
		"Binlog_stmt_cache_disk_use/s > 0",
		SeverityHintVariable, 1, ruleBinlogStmtCacheDiskUse))
	register(legacyAdapter(
		"threadcache.hit_ratio", "Thread Cache", "Thread cache hit ratio",
		"1 - Threads_created / Connections",
		SeverityHintVariable, 1, ruleThreadCacheHitRatio))
	register(legacyAdapter(
		"tablecache.usage", "Table Open Cache", "Open-tables vs table_open_cache",
		"Open_tables / table_open_cache",
		SeverityHintVariable, 2, ruleTableCacheUsage))
	register(legacyAdapter(
		"tablecache.overflows", "Table Open Cache", "Table cache overflows rate",
		"Table_open_cache_overflows/s",
		SeverityHintVariable, 1, ruleTableCacheOverflows))
	register(legacyAdapter(
		"tablecache.miss_ratio", "Table Open Cache", "Table cache miss ratio",
		"misses / (hits + misses)",
		SeverityHintVariable, 1, ruleTableCacheMissRatio))
	register(legacyAdapter(
		"tmp.disk_ratio", "Temp Tables", "Temp tables spilling to disk",
		"Created_tmp_disk_tables / Created_tmp_tables",
		SeverityHintVariable, 1, ruleTmpDiskRatio))
	register(legacyAdapter(
		"queryshape.select_full_join", "Query Shape", "Joins without usable indexes",
		"Select_full_join/s > 0",
		SeverityHintVariable, 1, ruleFullScanSelectFullJoin))
	register(legacyAdapter(
		"queryshape.handler_read_rnd_next", "Query Shape", "Sequential row reads (Handler_read_rnd_next)",
		"Handler_read_rnd_next/s  AND  Handler_read_rnd_next / Com_select",
		SeverityHintVariable, 1, ruleFullScanHandlerRndNext))
	register(legacyAdapter(
		"queryshape.is_processlist", "Query Shape", "information_schema.processlist abuse",
		"Deprecated_use_i_s_processlist_count/s > 0",
		SeverityHintVariable, 1, ruleProcesslistAbuse))
	register(legacyAdapter(
		"connections.aborted_rate", "Connections", "Aborted connections rate",
		"Aborted_connects/s",
		SeverityHintVariable, 1, ruleAbortedConnectsRate))
	register(legacyAdapter(
		"connections.saturation", "Connections", "Connection slot saturation",
		"Threads_connected / max_connections",
		SeverityHintVariable, 3, ruleConnectionsSaturation))
}

// sortRegistry sorts the registry by RuleDefinition.ID. Called from
// the package-level init in findings.go after every per-file init has
// populated the registry.
func sortRegistry() {
	sort.SliceStable(registry, func(i, j int) bool {
		return registry[i].ID < registry[j].ID
	})
}
