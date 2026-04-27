package findings

import (
	"fmt"

	"github.com/matias-sanchez/My-gather/model"
)

// ruleThreadMaxQueueDepth is a time-series rule: it scans every
// observed Threads_running value across the capture and flags peaks
// that imply concurrency saturation. Threads_running is the count of
// query threads actively running on the CPU at the sample instant —
// distinct from Threads_connected (which counts open sessions, most
// of which are idle). When Threads_running peaks rise toward
// max_connections, queries are queueing on row locks, mutexes, or CPU.
//
// Thresholds scale with max_connections so the rule works for both
// small and large servers:
//
//	peak >= max_connections / 2      → WARN
//	peak >= max_connections * 0.9    → CRIT
//
// When max_connections is unknown the rule falls back to fixed
// thresholds (50 → WARN, 200 → CRIT) consistent with the
// connections.saturation rule.
//
// This is the only currently-shipping rule that walks the whole
// time-series with gaugeMax — a deliberate baseline that PR-33
// introduces for future time-series rules to follow.
func ruleThreadMaxQueueDepth(r *model.Report) Finding {
	peak, ok := gaugeMax(r, "Threads_running")
	if !ok || peak <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	maxConns, hasMax := variableFloat(r, "max_connections")
	var (
		warnAt, critAt float64
		floorReason    string
	)
	if hasMax && maxConns > 0 {
		warnAt = maxConns / 2
		critAt = maxConns * 0.9
		floorReason = fmt.Sprintf("vs max_connections=%s", formatNum(maxConns))
	} else {
		warnAt = 50
		critAt = 200
		floorReason = "fixed thresholds (max_connections unknown)"
	}

	sev := SeverityOK
	summary := fmt.Sprintf("Threads_running peak was %s — concurrency well below the %s warn threshold (%s).",
		formatNum(peak), floorReason, formatNum(warnAt))
	switch {
	case peak >= critAt:
		sev = SeverityCrit
		summary = fmt.Sprintf("Threads_running peaked at %s — concurrency saturated (>=%s of max_connections=%s).",
			formatNum(peak), formatNum(critAt), formatNum(maxConns))
	case peak >= warnAt:
		sev = SeverityWarn
		summary = fmt.Sprintf("Threads_running peaked at %s — at or beyond half of max_connections (%s); queries are starting to queue.",
			formatNum(peak), formatNum(warnAt))
	}
	metrics := []MetricRef{
		{Name: "Threads_running (peak)", Value: peak, Unit: "count", Note: "max across capture window"},
		{Name: "warn threshold", Value: warnAt, Unit: "count"},
		{Name: "crit threshold", Value: critAt, Unit: "count"},
	}
	if hasMax {
		metrics = append(metrics, MetricRef{Name: "max_connections", Value: maxConns, Unit: "count"})
	}
	return Finding{
		ID:        "thread.max_queue_depth",
		Subsystem: "Connections",
		Title:     "Threads_running peak across capture",
		Severity:  sev,
		Summary:   summary,
		Explanation: "Threads_running is sampled per second by mysqladmin. Its peak across the capture is the most concrete view " +
			"of how saturated the server got under the workload. A peak nearing max_connections means queries are queueing on " +
			"locks, mutexes, or available CPU; sustained high values usually indicate a hot row, a table scan storm, or a " +
			"missing index. Compare this rule's verdict against connections.saturation (which is gauge-last) to distinguish a " +
			"steady-state load from a brief spike.",
		FormulaText: "max(Threads_running) across capture  vs  {max_connections/2 warn, max_connections*0.9 crit}",
		FormulaComputed: fmt.Sprintf("peak Threads_running = %s; warn=%s, crit=%s (%s)",
			formatNum(peak), formatNum(warnAt), formatNum(critAt), floorReason),
		Metrics: metrics,
		Recommendations: []string{
			"Cross-reference with the processlist capture to see which queries were running at the peak.",
			"Check the InnoDB Semaphores card — sustained high Threads_running with semaphore waits points at a hot mutex.",
			"If the workload genuinely needs the parallelism, validate max_connections sizing and per-thread memory budget.",
		},
		Source: "Rosetta Stone — Connections (saturation, time-series)",
	}
}

// init registers ruleThreadMaxQueueDepth as a native RuleDefinition.
// This is the time-series rule shipped in PR-33; it inspects the full
// Threads_running gauge series across the capture (gaugeMax) rather
// than just the last sample.
func init() {
	register(RuleDefinition{
		ID:                 "thread.max_queue_depth",
		Subsystem:          "Connections",
		Title:              "Threads_running peak across capture",
		FormulaText:        "max(Threads_running) across capture  vs  max_connections/2 (warn) and max_connections*0.9 (crit)",
		MinRecommendations: 3,
		Severity:           SeverityHintVariable,
		Run:                ruleThreadMaxQueueDepth,
	})
}

