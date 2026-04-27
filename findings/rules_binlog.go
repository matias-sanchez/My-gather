package findings

import (
	"fmt"

	"github.com/matias-sanchez/My-gather/model"
)

// ruleBinlogCacheDiskUse flags binlog cache spill-to-disk events —
// transactions exceeded binlog_cache_size and had to write the cache
// to a temporary file.
// See Rosetta Stone — Binlog Cache §Saturation (transactional).
func ruleBinlogCacheDiskUse(r *model.Report) Finding {
	rate, ok := counterRatePerSec(r, "Binlog_cache_disk_use")
	if !ok {
		return Finding{Severity: SeveritySkip}
	}
	if rate <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	sev := SeverityWarn
	if rate > 1 {
		sev = SeverityCrit
	}
	cacheSize, _ := variableFloat(r, "binlog_cache_size")
	return Finding{
		ID:        "binlog.cache_disk_use",
		Subsystem: "Binlog Cache",
		Title:     "Binlog cache overflowing to disk",
		Severity:  sev,
		Summary:   fmt.Sprintf("%s transactions/s exceeded binlog_cache_size and spilled to disk.", formatNum(rate)),
		Explanation: "Binlog_cache_disk_use counts transactions whose cached binlog data exceeded binlog_cache_size and " +
			"had to be written to a temporary file. A steady increase indicates binlog_cache_size should be raised.",
		FormulaText:     "Binlog_cache_disk_use/s > 0",
		FormulaComputed: fmt.Sprintf("%s /s > 0", formatNum(rate)),
		Metrics: []MetricRef{
			{Name: "Binlog_cache_disk_use/s", Value: rate, Unit: "/s"},
			{Name: "binlog_cache_size", Value: cacheSize, Unit: "bytes"},
		},
		Recommendations: []string{
			"Increase binlog_cache_size (session-scoped; 1 MB to a few MB is typical for busy OLTP).",
			"Avoid very large transactions — split them into smaller batches where possible.",
			"Watch max_binlog_cache_size — exceeding it errors with ER_TRANS_CACHE_FULL.",
		},
		Source: "Rosetta Stone — Binlog Cache §Saturation (transactional)",
	}
}

// ruleBinlogStmtCacheDiskUse is the statement-cache counterpart.
// See Rosetta Stone — Binlog Cache §Saturation (non-transactional).
func ruleBinlogStmtCacheDiskUse(r *model.Report) Finding {
	rate, ok := counterRatePerSec(r, "Binlog_stmt_cache_disk_use")
	if !ok {
		return Finding{Severity: SeveritySkip}
	}
	if rate <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	sev := SeverityWarn
	if rate > 1 {
		sev = SeverityCrit
	}
	stmtSize, _ := variableFloat(r, "binlog_stmt_cache_size")
	return Finding{
		ID:        "binlog.stmt_cache_disk_use",
		Subsystem: "Binlog Cache",
		Title:     "Binlog statement cache overflowing to disk",
		Severity:  sev,
		Summary:   fmt.Sprintf("%s statements/s exceeded binlog_stmt_cache_size and spilled to disk.", formatNum(rate)),
		Explanation: "Binlog_stmt_cache_disk_use counts non-transactional statements whose binlog data exceeded " +
			"binlog_stmt_cache_size and had to be written to a temporary file.",
		FormulaText:     "Binlog_stmt_cache_disk_use/s > 0",
		FormulaComputed: fmt.Sprintf("%s /s > 0", formatNum(rate)),
		Metrics: []MetricRef{
			{Name: "Binlog_stmt_cache_disk_use/s", Value: rate, Unit: "/s"},
			{Name: "binlog_stmt_cache_size", Value: stmtSize, Unit: "bytes"},
		},
		Recommendations: []string{
			"Increase binlog_stmt_cache_size.",
			"Watch max_binlog_stmt_cache_size — exceeding it errors with ER_STMT_CACHE_FULL.",
		},
		Source: "Rosetta Stone — Binlog Cache §Saturation (non-transactional)",
	}
}

// init registers the native RuleDefinition entries for the two
// Binlog Cache rules in this file.
func init() {
	register(RuleDefinition{
		ID:                 "binlog.cache_disk_use",
		Subsystem:          "Binlog Cache",
		Title:              "Binlog cache overflowing to disk",
		FormulaText:        "Binlog_cache_disk_use/s > 0",
		MinRecommendations: 3,
		Severity:           SeverityHintVariable,
		Run:                ruleBinlogCacheDiskUse,
	})
	register(RuleDefinition{
		ID:                 "binlog.stmt_cache_disk_use",
		Subsystem:          "Binlog Cache",
		Title:              "Binlog statement cache overflowing to disk",
		FormulaText:        "Binlog_stmt_cache_disk_use/s > 0",
		MinRecommendations: 2,
		Severity:           SeverityHintVariable,
		Run:                ruleBinlogStmtCacheDiskUse,
	})
}
