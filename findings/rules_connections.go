package findings

import (
	"fmt"

	"github.com/matias-sanchez/My-gather/model"
)

// ruleConnectionsSaturation flags connection pressure: either
// Threads_connected approaching max_connections, or a high count of
// actively-running threads (Threads_running). Both are reliable
// indicators that the server is at or beyond its designed concurrency.
// See Rosetta Stone — Connections §Saturation.
func ruleConnectionsSaturation(r *model.Report) Finding {
	const (
		warnUtil    = 0.70 // >70% of max_connections = warn
		critUtil    = 0.90 // >90% = crit
		warnRunning = 50.0 // Threads_running persistently ≥50 = warn
		critRunning = 100.0
	)
	connected, okC := gaugeMax(r, "Threads_connected")
	running, okR := gaugeMax(r, "Threads_running")
	if !okC && !okR {
		return Finding{Severity: SeveritySkip}
	}
	maxConns, okM := variableFloat(r, "max_connections")
	canEvalUtil := okC && okM && maxConns > 0
	// If we can evaluate neither the utilization ratio nor a meaningful
	// Threads_running ceiling, we have no signal — emit Skip instead of
	// a green "within safe bounds" that would falsely reassure the
	// reader on an incomplete capture (e.g. -variables missing).
	if !canEvalUtil && !okR {
		return Finding{Severity: SeveritySkip}
	}
	sev := SeverityOK
	var util float64
	var summary string
	if canEvalUtil {
		summary = "Connection concurrency is within safe bounds."
		util = connected / maxConns
		switch {
		case util >= critUtil:
			sev = SeverityCrit
			summary = fmt.Sprintf("Threads_connected peaked at %s of %s (%.0f %% of max_connections) — server is near its connection ceiling.",
				formatNum(connected), formatNum(maxConns), util*100)
		case util >= warnUtil:
			sev = SeverityWarn
			summary = fmt.Sprintf("Threads_connected peaked at %s (%.0f %% of max_connections = %s) — connection pool is stretched.",
				formatNum(connected), util*100, formatNum(maxConns))
		}
	} else {
		// Utilization ratio unavailable — rely only on the running-
		// threads ceiling and call that out honestly in the summary
		// so the reader knows we couldn't assess max_connections
		// saturation.
		summary = "max_connections unavailable — only Threads_running concurrency was evaluated."
	}
	if okR {
		switch {
		case running >= critRunning && sev < SeverityCrit:
			sev = SeverityCrit
			summary = fmt.Sprintf("Threads_running peaked at %s — very high active-thread concurrency starves CPU and contends on latches.", formatNum(running))
		case running >= warnRunning && sev < SeverityWarn:
			sev = SeverityWarn
			summary = fmt.Sprintf("Threads_running peaked at %s — above the comfort band for typical OLTP workloads.", formatNum(running))
		}
	}
	metrics := []MetricRef{}
	if okC {
		metrics = append(metrics, MetricRef{Name: "Threads_connected (max)", Value: connected, Unit: "count"})
	}
	if okR {
		metrics = append(metrics, MetricRef{Name: "Threads_running (max)", Value: running, Unit: "count"})
	}
	if okM {
		metrics = append(metrics, MetricRef{Name: "max_connections", Value: maxConns, Unit: "count"})
	}
	formulaText := "Threads_connected / max_connections  and  Threads_running (max)"
	formulaComputed := "(n/a)"
	if okC && okM && maxConns > 0 {
		formulaComputed = fmt.Sprintf("%s / %s = %.0f %%", formatNum(connected), formatNum(maxConns), util*100)
	}
	return Finding{
		ID:        "connections.saturation",
		Subsystem: "Connections",
		Title:     "Connection pool saturation",
		Severity:  sev,
		Summary:   summary,
		Explanation: "Every established client session consumes memory for thread-local buffers (sort, join, tmp_table) and file descriptors. " +
			"High Threads_connected without matching Threads_running usually means the application's connection pool is oversized or leaking. " +
			"High Threads_running typically means work is queuing behind locks, IO, or CPU.",
		FormulaText:     formulaText,
		FormulaComputed: formulaComputed,
		Metrics:         metrics,
		Recommendations: []string{
			"Cap the application-side connection pool to a value aligned with max_connections and the CPU count.",
			"Investigate Threads_running with pt-stalk/processlist — look for long-running queries, lock waits, or semaphore contention.",
			"Consider a connection multiplexer (ProxySQL, MySQL Router) in front of the server for traffic spikes.",
		},
		Source: "Rosetta Stone — Connections §Saturation",
	}
}

// ruleAbortedConnectsRate flags rising Aborted_connects — client
// connection attempts that never became usable sessions.
// See Rosetta Stone — Part B Aborted_connects.
func ruleAbortedConnectsRate(r *model.Report) Finding {
	const (
		warnAbove = 0.1 // > 1 per 10 s
		critAbove = 1.0 // > 1 per second
	)
	rate, ok := counterRatePerSec(r, "Aborted_connects")
	if !ok || rate <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	sev := SeverityOK
	summary := fmt.Sprintf("Aborted_connects rate is %s/s — normal background noise.", formatNum(rate))
	switch {
	case rate > critAbove:
		sev = SeverityCrit
		summary = fmt.Sprintf("Aborted_connects rate is very high at %s/s — possible DoS, auth failures, or misbehaving client.", formatNum(rate))
	case rate > warnAbove:
		sev = SeverityWarn
		summary = fmt.Sprintf("Aborted_connects rate is %s/s — worth investigating.", formatNum(rate))
	}
	return Finding{
		ID:        "connections.aborted_rate",
		Subsystem: "Connections",
		Title:     "Aborted connection attempts",
		Severity:  sev,
		Summary:   summary,
		Explanation: "Aborted_connects increments when a client fails to establish a usable session. Common causes: " +
			"access to a database it lacks rights for, bad connection packet, wrong password, exceeded connect_timeout, " +
			"or max_allowed_packet being exceeded before the handshake completed.",
		FormulaText:     "Aborted_connects/s > 0",
		FormulaComputed: fmt.Sprintf("%s /s > 0", formatNum(rate)),
		Metrics: []MetricRef{
			{Name: "Aborted_connects/s", Value: rate, Unit: "/s"},
		},
		Recommendations: []string{
			"Set log_error_verbosity=3 and check the error log for specific abort reasons.",
			"Cross-check Connection_errors_* counters (accept, internal, max_connections, peer_address, select, tcpwrap) for the specific failure category.",
			"If traffic is coming from a known IP range, inspect network / NIC / DNS for latency issues.",
		},
		Source: "Rosetta Stone — Part B Aborted_connects",
	}
}

// init registers the native RuleDefinition entries for both
// Connections rules in this file.
func init() {
	register(RuleDefinition{
		ID:                 "connections.aborted_rate",
		Subsystem:          "Connections",
		Title:              "Aborted connections rate",
		FormulaText:        "Aborted_connects/s",
		MinRecommendations: 1,
		Severity:           SeverityHintVariable,
		Run:                ruleAbortedConnectsRate,
	})
	register(RuleDefinition{
		ID:                 "connections.saturation",
		Subsystem:          "Connections",
		Title:              "Connection slot saturation",
		FormulaText:        "Threads_connected / max_connections",
		MinRecommendations: 3,
		Severity:           SeverityHintVariable,
		Run:                ruleConnectionsSaturation,
	})
}
