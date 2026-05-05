package findings

import (
	"fmt"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/reportutil"
)

// ruleTmpDiskRatio computes the fraction of internal temporary tables
// that had to be materialised on disk instead of in memory.
// See Rosetta Stone — Part B Created_tmp_{disk_,}tables.
func ruleTmpDiskRatio(r *model.Report) Finding {
	const (
		warnAbove = 0.25
		critAbove = 0.50
	)
	disk, ok1 := counterTotal(r, "Created_tmp_disk_tables")
	total, ok2 := counterTotal(r, "Created_tmp_tables")
	if !ok1 || !ok2 {
		return Finding{Severity: SeveritySkip}
	}
	if total <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	ratio := disk / total
	sev := SeverityOK
	summary := fmt.Sprintf("%s of internal temp tables went to disk — within comfort band.", formatPercent(ratio))
	switch {
	case ratio > critAbove:
		sev = SeverityCrit
		summary = fmt.Sprintf("%s of internal temp tables had to be created on disk.", formatPercent(ratio))
	case ratio > warnAbove:
		sev = SeverityWarn
		summary = fmt.Sprintf("%s of internal temp tables went to disk.", formatPercent(ratio))
	}
	tmpSize, _ := reportutil.VariableFloat(r, "tmp_table_size")
	heapSize, _ := reportutil.VariableFloat(r, "max_heap_table_size")
	return Finding{
		ID:        "tmp.disk_ratio",
		Subsystem: "Temp Tables",
		Title:     "Internal temp tables spilling to disk",
		Severity:  sev,
		Summary:   summary,
		Explanation: "MySQL converts an in-memory internal temp table to a disk-based one when it exceeds min(tmp_table_size, max_heap_table_size). " +
			"High spill ratios indicate queries returning or sorting datasets larger than the configured in-memory limit.",
		FormulaText: "disk_ratio = Created_tmp_disk_tables / Created_tmp_tables",
		FormulaComputed: fmt.Sprintf("%s / %s = %s",
			reportutil.FormatNum(disk), reportutil.FormatNum(total), formatPercent(ratio)),
		Metrics: []MetricRef{
			{Name: "Created_tmp_disk_tables", Value: disk, Unit: "count"},
			{Name: "Created_tmp_tables", Value: total, Unit: "count"},
			{Name: "tmp_table_size", Value: tmpSize, Unit: "bytes"},
			{Name: "max_heap_table_size", Value: heapSize, Unit: "bytes"},
		},
		Recommendations: []string{
			"Rewrite queries to avoid large GROUP BY / DISTINCT / temporary scans where possible.",
			"Increase tmp_table_size AND max_heap_table_size (both must be raised in lockstep).",
			"Check for SELECTs that return very wide rows (e.g. TEXT/BLOB columns) — those always go to disk.",
		},
		Source: "Rosetta Stone — Part B Created_tmp_{disk_,}tables",
	}
}

// init registers the native RuleDefinition for the Temp Tables rule.
func init() {
	register(RuleDefinition{
		ID:                 "tmp.disk_ratio",
		Subsystem:          "Temp Tables",
		Title:              "Internal temp tables spilling to disk",
		FormulaText:        "disk_ratio = Created_tmp_disk_tables / Created_tmp_tables",
		MinRecommendations: 3,
		Severity:           SeverityHintVariable,
		Run:                ruleTmpDiskRatio,
	})
}
