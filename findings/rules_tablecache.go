package findings

import (
	"fmt"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/reportutil"
)

// ruleTableCacheUsage flags a table cache that is approaching or at
// its configured ceiling: Open_tables / table_open_cache. A saturated
// cache is proactively critical — the next new table opened forces
// an eviction (counted as a Table_open_cache_overflow), so this rule
// fires BEFORE ruleTableCacheOverflows and explains why overflows
// are about to start. Thresholds: warn ≥ 80 %, crit ≥ 95 %.
func ruleTableCacheUsage(r *model.Report) Finding {
	const (
		warnAbove = 0.80
		critAbove = 0.95
	)
	size, ok1 := reportutil.VariableFloat(r, "table_open_cache")
	open, ok2 := reportutil.GaugeLast(r, "Open_tables")
	if !ok1 || !ok2 || size <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	ratio := open / size
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	var (
		sev     = SeverityOK
		summary = fmt.Sprintf("Table cache is at %s of capacity.", formatPercent(ratio))
	)
	switch {
	case ratio >= critAbove:
		sev = SeverityCrit
		summary = fmt.Sprintf("Table cache is saturated at %s — evictions are imminent.", formatPercent(ratio))
	case ratio >= warnAbove:
		sev = SeverityWarn
		summary = fmt.Sprintf("Table cache is running hot at %s of capacity.", formatPercent(ratio))
	}
	return Finding{
		ID:        "tablecache.usage",
		Subsystem: "Table Open Cache",
		Title:     "Table cache saturation",
		Severity:  sev,
		Summary:   summary,
		Explanation: "Open_tables counts how many table descriptors are held in the cache right now; " +
			"table_open_cache is the ceiling. Once the ratio reaches 100 %, any new table reference forces " +
			"MySQL to evict an existing descriptor (recorded as a Table_open_cache_overflow) before opening " +
			"the new one — adding latency to the query that triggered it.",
		FormulaText: "usage = Open_tables / table_open_cache",
		FormulaComputed: fmt.Sprintf("%s / %s = %s",
			reportutil.FormatNum(open), reportutil.FormatNum(size), formatPercent(ratio)),
		Metrics: []MetricRef{
			{Name: "Open_tables", Value: open, Unit: "entries", Note: "last sample"},
			{Name: "table_open_cache", Value: size, Unit: "entries"},
		},
		Recommendations: []string{
			"Increase table_open_cache. Raise open_files_limit at the OS level first if it's near the current cache size.",
			"Check how many distinct tables the workload touches; a sharp fan-out (e.g. sharded per-tenant tables) will keep the cache pinned.",
			"If the workload has heavy concurrent access to a small table set, consider lowering table_open_cache_instances so each instance holds a larger share.",
		},
		Source: "Rosetta Stone — Table Open Cache §Utilization (capacity), Part B Open_tables",
	}
}

// ruleTableCacheOverflows flags any non-zero rate of table cache
// overflows — the cache is too small for the workload and MySQL is
// evicting entries to make room.
// See Rosetta Stone — Table Open Cache §Saturation (capacity).
func ruleTableCacheOverflows(r *model.Report) Finding {
	rate, ok := counterRatePerSec(r, "Table_open_cache_overflows")
	if !ok {
		return Finding{Severity: SeveritySkip}
	}
	if rate <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	sev := SeverityWarn
	if rate > 5 {
		sev = SeverityCrit
	}
	size, _ := reportutil.VariableFloat(r, "table_open_cache")
	instances, _ := reportutil.VariableFloat(r, "table_open_cache_instances")
	return Finding{
		ID:        "tablecache.overflows",
		Subsystem: "Table Open Cache",
		Title:     "Table open cache overflows",
		Severity:  sev,
		Summary:   fmt.Sprintf("Table_open_cache_overflows is incrementing at %s/s — the table cache is evicting entries.", reportutil.FormatNum(rate)),
		Explanation: "An overflow occurs when a table-cache instance temporarily grows past its share of table_open_cache to accept a new entry. " +
			"Repeated overflows mean the configured size (possibly divided across too many instances) is insufficient for the workload.",
		FormulaText:     "Table_open_cache_overflows/s > 0",
		FormulaComputed: fmt.Sprintf("%s /s > 0", reportutil.FormatNum(rate)),
		Metrics: []MetricRef{
			{Name: "Table_open_cache_overflows/s", Value: rate, Unit: "/s"},
			{Name: "table_open_cache", Value: size, Unit: "entries"},
			{Name: "table_open_cache_instances", Value: instances, Unit: "count"},
		},
		Recommendations: []string{
			"Increase table_open_cache. Watch open_files_limit — raise it at the OS level if needed.",
			"If the workload has heavy concurrent access to a small number of tables, consider decreasing table_open_cache_instances to give each instance more room.",
		},
		Source: "Rosetta Stone — Table Open Cache §Saturation (capacity)",
	}
}

// ruleTableCacheMissRatio computes the table cache miss ratio over
// the capture window:
//
//	miss_ratio = misses / (hits + misses)
//
// A climbing miss ratio means the cache is churning.
// See Rosetta Stone — Table Open Cache §Utilization (capacity), Part B Table_open_cache_.
func ruleTableCacheMissRatio(r *model.Report) Finding {
	const (
		warnAbove = 0.01 // > 1 %
		critAbove = 0.10 // > 10 %
	)
	misses, ok1 := counterTotal(r, "Table_open_cache_misses")
	hits, ok2 := counterTotal(r, "Table_open_cache_hits")
	if !ok1 || !ok2 {
		return Finding{Severity: SeveritySkip}
	}
	total := hits + misses
	if total <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	ratio := misses / total
	sev := SeverityOK
	summary := fmt.Sprintf("Table cache miss ratio is low at %s.", formatPercent(ratio))
	switch {
	case ratio > critAbove:
		sev = SeverityCrit
		summary = fmt.Sprintf("Table cache miss ratio is %s — the cache is churning heavily.", formatPercent(ratio))
	case ratio > warnAbove:
		sev = SeverityWarn
		summary = fmt.Sprintf("Table cache miss ratio is elevated at %s.", formatPercent(ratio))
	}
	return Finding{
		ID:        "tablecache.miss_ratio",
		Subsystem: "Table Open Cache",
		Title:     "Table cache miss ratio",
		Severity:  sev,
		Summary:   summary,
		Explanation: "Each cache miss is a table-open operation — opening a table descriptor from disk. " +
			"Frequent misses mean table_open_cache isn't large enough to hold the active working set.",
		FormulaText: "miss_ratio = Table_open_cache_misses / (Table_open_cache_hits + Table_open_cache_misses)",
		FormulaComputed: fmt.Sprintf("%s / (%s + %s) = %s",
			reportutil.FormatNum(misses), reportutil.FormatNum(hits), reportutil.FormatNum(misses), formatPercent(ratio)),
		Metrics: []MetricRef{
			{Name: "Table_open_cache_misses", Value: misses, Unit: "count"},
			{Name: "Table_open_cache_hits", Value: hits, Unit: "count"},
		},
		Recommendations: []string{
			"Increase table_open_cache (mind open_files_limit).",
			"Profile whether a specific worker pattern is opening many tables per query.",
		},
		Source: "Rosetta Stone — Part B Table_open_cache_{hits,misses}",
	}
}

// init registers the native RuleDefinition entries for every Table
// Open Cache rule in this file.
func init() {
	register(RuleDefinition{
		ID:                 "tablecache.usage",
		Subsystem:          "Table Open Cache",
		Title:              "Table cache saturation",
		FormulaText:        "usage = Open_tables / table_open_cache",
		MinRecommendations: 3,
		Severity:           SeverityHintVariable,
		Run:                ruleTableCacheUsage,
	})
	register(RuleDefinition{
		ID:                 "tablecache.overflows",
		Subsystem:          "Table Open Cache",
		Title:              "Table open cache overflows",
		FormulaText:        "Table_open_cache_overflows/s > 0",
		MinRecommendations: 2,
		Severity:           SeverityHintVariable,
		Run:                ruleTableCacheOverflows,
	})
	register(RuleDefinition{
		ID:                 "tablecache.miss_ratio",
		Subsystem:          "Table Open Cache",
		Title:              "Table cache miss ratio",
		FormulaText:        "miss_ratio = Table_open_cache_misses / (Table_open_cache_hits + Table_open_cache_misses)",
		MinRecommendations: 2,
		Severity:           SeverityHintVariable,
		Run:                ruleTableCacheMissRatio,
	})
}
