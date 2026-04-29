package findings

import (
	"fmt"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

const observedSlowQueryWarnSeconds = 60.0

// ruleFullScanSelectScan flags any Select_scan activity — joins that
// did a full scan of the first table.
// See Rosetta Stone — Part B Select_scan.
func ruleFullScanSelectScan(r *model.Report) Finding {
	rate, ok := counterRatePerSec(r, "Select_scan")
	if !ok || rate <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	sev := SeverityWarn
	if rate > 10 {
		sev = SeverityCrit
	}
	return Finding{
		ID:        "queryshape.select_scan",
		Subsystem: "Query Shape",
		Title:     "Full table scans on the first join input",
		Severity:  sev,
		Summary:   fmt.Sprintf("Select_scan is incrementing at %s/s — queries are fully scanning their first table.", formatNum(rate)),
		Explanation: "Select_scan counts SELECTs that had to scan the entire first table in their plan, usually because the " +
			"relevant column is not indexed (or the planner could not use the index). Deltas from this should be 0 in a healthy OLTP workload.",
		FormulaText:     "Select_scan/s > 0",
		FormulaComputed: fmt.Sprintf("%s /s > 0", formatNum(rate)),
		Metrics: []MetricRef{
			{Name: "Select_scan/s", Value: rate, Unit: "/s"},
		},
		Recommendations: []string{
			"Identify the offending queries (slow log / pt-query-digest / performance_schema.events_statements_summary_by_digest).",
			"Review their EXPLAIN plans — add the missing index if appropriate.",
		},
		Source: "Rosetta Stone — Part B Select_scan",
	}
}

// init registers the native RuleDefinition entries for every Query
// Shape rule in this file. MinRecommendations is 1 across the board
// because these rules' INFO/WARN/CRIT outputs share their two-step
// remediation lists — the symptom is the same; the severity scales
// with rate.
func init() {
	register(RuleDefinition{
		ID:                 "queryshape.select_scan",
		Subsystem:          "Query Shape",
		Title:              "Full table scans on the first join input",
		FormulaText:        "Select_scan/s > 0  (crit > 10/s)",
		MinRecommendations: 1,
		Severity:           SeverityHintVariable,
		Run:                ruleFullScanSelectScan,
	})
	register(RuleDefinition{
		ID:                 "queryshape.select_full_join",
		Subsystem:          "Query Shape",
		Title:              "Joins without usable indexes",
		FormulaText:        "Select_full_join/s > 0",
		MinRecommendations: 2,
		Severity:           SeverityHintVariable,
		Run:                ruleFullScanSelectFullJoin,
	})
	register(RuleDefinition{
		ID:                 "queryshape.handler_read_rnd_next",
		Subsystem:          "Query Shape",
		Title:              "Sequential row reads (table scans)",
		FormulaText:        "Handler_read_rnd_next/s  and  Handler_read_rnd_next / Com_select",
		MinRecommendations: 2,
		Severity:           SeverityHintVariable,
		Run:                ruleFullScanHandlerRndNext,
	})
	register(RuleDefinition{
		ID:                 "queryshape.is_processlist",
		Subsystem:          "Query Shape",
		Title:              "information_schema.PROCESSLIST usage",
		FormulaText:        "Deprecated_use_i_s_processlist_count/s",
		MinRecommendations: 2,
		Severity:           SeverityHintVariable,
		Run:                ruleProcesslistAbuse,
	})
	register(RuleDefinition{
		ID:                 "queryshape.observed_slow_processlist",
		Subsystem:          "Query Shape",
		Title:              "Slow observed processlist query",
		FormulaText:        "max observed processlist query age >= 60s",
		MinRecommendations: 3,
		Severity:           SeverityHintVariable,
		Run:                ruleObservedSlowProcesslistQuery,
	})
}

// ruleObservedSlowProcesslistQuery flags the most impactful active
// processlist query shape observed during the capture. The processlist
// model is already sorted by max age, so the first summary at or above
// threshold is the strongest evidence to present.
func ruleObservedSlowProcesslistQuery(r *model.Report) Finding {
	if r == nil || r.DBSection == nil || r.DBSection.Processlist == nil {
		return Finding{Severity: SeveritySkip}
	}
	var top model.ObservedProcesslistQuery
	found := false
	for _, q := range r.DBSection.Processlist.ObservedQueries {
		if !q.HasTimeMetric {
			continue
		}
		if q.MaxTimeMS/1000 < observedSlowQueryWarnSeconds {
			continue
		}
		top = q
		found = true
		break
	}
	if !found {
		return Finding{Severity: SeveritySkip}
	}

	ageSeconds := top.MaxTimeMS / 1000
	sev := SeverityWarn
	summary := fmt.Sprintf("Observed query %s ran for %s s across %s sightings.",
		top.Fingerprint, formatNum(ageSeconds), formatNum(float64(top.SeenSamples)))
	stateLower := strings.ToLower(top.State)
	if strings.Contains(stateLower, "metadata lock") {
		sev = SeverityCrit
		summary = fmt.Sprintf("Observed query %s waited on metadata lock for %s s across %s sightings.",
			top.Fingerprint, formatNum(ageSeconds), formatNum(float64(top.SeenSamples)))
	}

	explanation := fmt.Sprintf(
		"The slowest observed active processlist query was seen as user %s on database %s with state %s. Query shape: %s",
		top.User, top.DB, top.State, top.Snippet,
	)
	return Finding{
		ID:          "queryshape.observed_slow_processlist",
		Subsystem:   "Query Shape",
		Title:       "Slow observed processlist query",
		Severity:    sev,
		Summary:     summary,
		Explanation: explanation,
		FormulaText: "max observed processlist query age >= 60s",
		FormulaComputed: fmt.Sprintf("%s s >= %s s  (fingerprint %s, user %s, db %s, state %s)",
			formatNum(ageSeconds), formatNum(observedSlowQueryWarnSeconds), top.Fingerprint, top.User, top.DB, top.State),
		Metrics: []MetricRef{
			{Name: "max observed age", Value: ageSeconds, Unit: "s"},
			{Name: "sightings", Value: float64(top.SeenSamples), Unit: "count"},
			{Name: "rows examined", Value: top.MaxRowsExamined, Unit: "rows"},
			{Name: "rows sent", Value: top.MaxRowsSent, Unit: "rows"},
		},
		Recommendations: []string{
			"Use the fingerprint and query shape in this report to locate matching application code, slow log entries, or performance_schema statement history.",
			"If the state is a lock wait, identify the blocking session or DDL path before tuning the query itself.",
			"Review the query's EXPLAIN plan and supporting indexes once lock waits or blocking transactions are ruled out.",
		},
		Source: "pt-stalk processlist observed query summary",
	}
}

// ruleFullScanSelectFullJoin flags joins that performed table scans
// because they do not use indexes.
// See Rosetta Stone — Part B Select_full_join.
func ruleFullScanSelectFullJoin(r *model.Report) Finding {
	rate, ok := counterRatePerSec(r, "Select_full_join")
	if !ok || rate <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	sev := SeverityWarn
	if rate > 1 {
		sev = SeverityCrit
	}
	return Finding{
		ID:        "queryshape.select_full_join",
		Subsystem: "Query Shape",
		Title:     "Joins without usable indexes",
		Severity:  sev,
		Summary:   fmt.Sprintf("Select_full_join is incrementing at %s/s — joins are running without a usable index.", formatNum(rate)),
		Explanation: "Select_full_join counts the number of joins that performed full scans because no suitable index could be used. " +
			"Even a low non-zero rate is cause for investigation — each full join multiplies the work across the join's cardinality.",
		FormulaText:     "Select_full_join/s > 0",
		FormulaComputed: fmt.Sprintf("%s /s > 0", formatNum(rate)),
		Metrics: []MetricRef{
			{Name: "Select_full_join/s", Value: rate, Unit: "/s"},
		},
		Recommendations: []string{
			"Enable the slow log / performance_schema and identify the JOIN queries.",
			"Index the join column on at least one side of the join.",
		},
		Source: "Rosetta Stone — Part B Select_full_join",
	}
}

// ruleFullScanHandlerRndNext flags a high rate of
// Handler_read_rnd_next — symptom of queries reading many sequential
// rows during a table scan. Evaluated as BOTH an absolute rate AND a
// per-SELECT ratio (rows scanned per SELECT); the ratio is much more
// sensitive to missing-index problems than a raw rate threshold.
// See Rosetta Stone — Part B Handler_read_rnd_next.
func ruleFullScanHandlerRndNext(r *model.Report) Finding {
	const (
		warnRate     = 10_000.0 // rows/s of rnd_next is a lot
		critRate     = 100_000.0
		warnPerSel   = 100.0 // average rows scanned per SELECT
		critPerSel   = 1000.0
		minSelectsPs = 1.0 // need some SELECT activity before the ratio is meaningful
	)
	rate, ok := counterRatePerSec(r, "Handler_read_rnd_next")
	if !ok || rate <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	selRate, _ := counterRatePerSec(r, "Com_select")
	var perSelect float64
	haveRatio := false
	if selRate > 0 {
		// Always compute the ratio when we have any SELECT activity so
		// the metrics block can report it, but only *apply severity
		// thresholds* against it when the SELECT rate is high enough
		// for the ratio to be statistically meaningful. At 0.5 q/s a
		// single accidental scan blows up the ratio; minSelectsPs
		// guards against that noise.
		perSelect = rate / selRate
		haveRatio = true
	}
	applyRatio := haveRatio && selRate >= minSelectsPs
	sev := SeverityOK
	summary := fmt.Sprintf("Handler_read_rnd_next is %s/s — normal for this workload.", formatNum(rate))
	// Evaluate ratio first (more sensitive); then absolute rate as a fallback.
	switch {
	case applyRatio && perSelect > critPerSel:
		sev = SeverityCrit
		summary = fmt.Sprintf("Handler_read_rnd_next averages %s rows per SELECT (%s rows/s over %s SELECTs/s) — almost certainly unindexed scans dominating the workload.",
			formatNum(perSelect), formatNum(rate), formatNum(selRate))
	case applyRatio && perSelect > warnPerSel:
		sev = SeverityWarn
		summary = fmt.Sprintf("Handler_read_rnd_next averages %s rows per SELECT (%s rows/s over %s SELECTs/s) — high scan-per-query ratio.",
			formatNum(perSelect), formatNum(rate), formatNum(selRate))
	case rate > critRate:
		sev = SeverityCrit
		summary = fmt.Sprintf("Handler_read_rnd_next at %s/s is very high — likely unindexed scans.", formatNum(rate))
	case rate > warnRate:
		sev = SeverityWarn
		summary = fmt.Sprintf("Handler_read_rnd_next at %s/s is high — investigate for unindexed scans.", formatNum(rate))
	}
	metrics := []MetricRef{
		{Name: "Handler_read_rnd_next/s", Value: rate, Unit: "/s"},
	}
	if haveRatio {
		metrics = append(metrics,
			MetricRef{Name: "Com_select/s", Value: selRate, Unit: "/s"},
			MetricRef{Name: "rows scanned per SELECT", Value: perSelect, Unit: "rows"},
		)
	}
	return Finding{
		ID:        "queryshape.handler_read_rnd_next",
		Subsystem: "Query Shape",
		Title:     "Sequential row reads (table scans)",
		Severity:  sev,
		Summary:   summary,
		Explanation: "Handler_read_rnd_next counts rows read by advancing through a table in storage order — the footprint of table scans. " +
			"A high rate alone can be acceptable for reporting workloads, but when the ratio of rnd_next to Com_select climbs past ~100 rows per query, " +
			"you are almost certainly missing an index.",
		FormulaText: "Handler_read_rnd_next/s  and  Handler_read_rnd_next / Com_select",
		FormulaComputed: func() string {
			if haveRatio {
				return fmt.Sprintf("%s /s  and  %s rows/SELECT", formatNum(rate), formatNum(perSelect))
			}
			return fmt.Sprintf("%s /s  (no Com_select activity — ratio n/a)", formatNum(rate))
		}(),
		Metrics: metrics,
		Recommendations: []string{
			"Cross-reference with slow log / query digest to identify the scan-heavy queries.",
			"Check whether an index exists on the filter column; if it does, inspect EXPLAIN to see why it's not used.",
		},
		Source: "Rosetta Stone — Part B Handler_read_rnd_next",
	}
}

// ruleProcesslistAbuse flags heavy use of the deprecated
// information_schema.PROCESSLIST table, which performs a full
// thread-list scan on every call — noticeably expensive on instances
// with hundreds of open connections.
//
// Uses Deprecated_use_i_s_processlist_count (the counter) rather than
// the _last_timestamp sibling (a gauge). The gauge is the Unix-time
// stamp of the last use and is classified as a gauge in
// parse/mysql-status-types.json, so counterRatePerSec on it always
// returns ok=false and the rule would permanently skip.
// See Rosetta Stone — Part B Deprecated_use_i_s_processlist_count.
func ruleProcesslistAbuse(r *model.Report) Finding {
	const (
		warnRate = 1.0   // calls per second
		critRate = 100.0 // hammering
	)
	rate, ok := counterRatePerSec(r, "Deprecated_use_i_s_processlist_count")
	if !ok || rate <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	sev := SeverityInfo
	summary := fmt.Sprintf("information_schema.PROCESSLIST is being queried at %s calls/s.", formatNum(rate))
	switch {
	case rate > critRate:
		sev = SeverityCrit
		summary = fmt.Sprintf("information_schema.PROCESSLIST is being hit %s times/s — this scans every open thread on every call and burns CPU at high connection counts.", formatNum(rate))
	case rate > warnRate:
		sev = SeverityWarn
		summary = fmt.Sprintf("information_schema.PROCESSLIST is being queried %s times/s — deprecated on 8.0+, noticeable overhead at high connection counts.", formatNum(rate))
	}
	return Finding{
		ID:        "queryshape.is_processlist",
		Subsystem: "Query Shape",
		Title:     "information_schema.PROCESSLIST usage",
		Severity:  sev,
		Summary:   summary,
		Explanation: "Since MySQL 8.0.22, the information_schema.PROCESSLIST implementation is deprecated because it locks the thread list on every read. " +
			"Monitoring tools that poll it frequently pay a real CPU cost proportional to Threads_connected. " +
			"performance_schema.processlist is the drop-in replacement and is non-blocking.",
		FormulaText:     "Deprecated_use_i_s_processlist_count/s",
		FormulaComputed: fmt.Sprintf("%s /s", formatNum(rate)),
		Metrics: []MetricRef{
			{Name: "Deprecated_use_i_s_processlist_count/s", Value: rate, Unit: "/s"},
		},
		Recommendations: []string{
			"Switch monitoring/agents to performance_schema.processlist (or SHOW PROCESSLIST) and set a minimum poll interval.",
			"Set performance_schema_show_processlist=ON to make SHOW PROCESSLIST read from performance_schema too.",
		},
		Source: "Rosetta Stone — Part B Deprecated_use_i_s_processlist_count",
	}
}
