package findings

import (
	"fmt"

	"github.com/matias-sanchez/My-gather/model"
)

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
		Summary:   fmt.Sprintf("Select_scan is incrementing at %s/s — queries are fully scanning their first table.", FormatNum(rate)),
		Explanation: "Select_scan counts SELECTs that had to scan the entire first table in their plan, usually because the " +
			"relevant column is not indexed (or the planner could not use the index). Deltas from this should be 0 in a healthy OLTP workload.",
		FormulaText:     "Select_scan/s > 0",
		FormulaComputed: fmt.Sprintf("%s /s > 0", FormatNum(rate)),
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
		Summary:   fmt.Sprintf("Select_full_join is incrementing at %s/s — joins are running without a usable index.", FormatNum(rate)),
		Explanation: "Select_full_join counts the number of joins that performed full scans because no suitable index could be used. " +
			"Even a low non-zero rate is cause for investigation — each full join multiplies the work across the join's cardinality.",
		FormulaText:     "Select_full_join/s > 0",
		FormulaComputed: fmt.Sprintf("%s /s > 0", FormatNum(rate)),
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
// rows during a table scan.
// See Rosetta Stone — Part B Handler_read_rnd_next.
func ruleFullScanHandlerRndNext(r *model.Report) Finding {
	const (
		warnRate = 10_000.0 // rows/s of rnd_next is a lot
		critRate = 100_000.0
	)
	rate, ok := counterRatePerSec(r, "Handler_read_rnd_next")
	if !ok || rate <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	sev := SeverityOK
	summary := fmt.Sprintf("Handler_read_rnd_next is %s/s — normal for this workload.", FormatNum(rate))
	switch {
	case rate > critRate:
		sev = SeverityCrit
		summary = fmt.Sprintf("Handler_read_rnd_next at %s/s is very high — likely unindexed scans.", FormatNum(rate))
	case rate > warnRate:
		sev = SeverityWarn
		summary = fmt.Sprintf("Handler_read_rnd_next at %s/s is high — investigate for unindexed scans.", FormatNum(rate))
	}
	return Finding{
		ID:        "queryshape.handler_read_rnd_next",
		Subsystem: "Query Shape",
		Title:     "Sequential row reads (table scans)",
		Severity:  sev,
		Summary:   summary,
		Explanation: "Handler_read_rnd_next counts rows read by advancing through a table in storage order — the footprint of table scans. " +
			"A high rate without the workload being explicitly analytical usually indicates missing or unused indexes.",
		FormulaText:     "Handler_read_rnd_next/s",
		FormulaComputed: fmt.Sprintf("%s /s", FormatNum(rate)),
		Metrics: []MetricRef{
			{Name: "Handler_read_rnd_next/s", Value: rate, Unit: "/s"},
		},
		Recommendations: []string{
			"Cross-reference with slow log / query digest to identify the scan-heavy queries.",
			"Check whether an index exists on the filter column; if it does, inspect EXPLAIN to see why it's not used.",
		},
		Source: "Rosetta Stone — Part B Handler_read_rnd_next",
	}
}
