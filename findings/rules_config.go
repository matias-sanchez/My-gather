package findings

import (
	"fmt"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

// ruleSlowLogDisabled flags slow_query_log = OFF on a busy instance.
// Silence here is suspicious: if the slow log is off, the data needed
// to diagnose high Select_scan / Handler_read_rnd_next rates isn't
// being collected.
func ruleSlowLogDisabled(r *model.Report) Finding {
	raw, ok := variableRaw(r, "slow_query_log")
	if !ok {
		return Finding{Severity: SeveritySkip}
	}
	// MySQL reports these as ON/OFF (strings) rather than 1/0.
	val := strings.ToUpper(strings.TrimSpace(raw))
	if val == "ON" || val == "1" {
		return Finding{Severity: SeveritySkip}
	}
	qps, _ := counterRatePerSec(r, "Questions")
	if qps <= 0 {
		// Idle instance — no point recommending the slow log.
		return Finding{Severity: SeveritySkip}
	}
	longQT, _ := variableFloat(r, "long_query_time")
	return Finding{
		ID:        "config.slow_log_disabled",
		Subsystem: "Configuration",
		Title:     "Slow query log is disabled",
		Severity:  SeverityWarn,
		Summary:   fmt.Sprintf("slow_query_log = OFF while the server is handling %s questions/s — you have no record of what's slow.", formatNum(qps)),
		Explanation: "The slow query log (along with performance_schema) is the primary way to identify which queries are " +
			"driving CPU and IO load. Leaving it off means that when symptoms appear — high Select_scan, Handler_read_rnd_next, " +
			"or Threads_running — you can't tell which queries are to blame.",
		FormulaText:     "slow_query_log = OFF  AND  Questions/s > 0",
		FormulaComputed: fmt.Sprintf("slow_query_log = %s  AND  Questions/s = %s", raw, formatNum(qps)),
		Metrics: []MetricRef{
			{Name: "slow_query_log", Value: 0, Unit: "bool", Note: fmt.Sprintf("raw value: %s", raw)},
			{Name: "Questions/s", Value: qps, Unit: "/s"},
			{Name: "long_query_time", Value: longQT, Unit: "s"},
		},
		Recommendations: []string{
			"SET GLOBAL slow_query_log = ON for incident windows; pair with long_query_time = 0.5 (or 0 for a full capture).",
			"Capture a digest with `pt-query-digest` against the slow log to identify the expensive queries.",
			"If performance_schema is enabled, you can also use events_statements_summary_by_digest without touching the file log.",
		},
		Source: "Rosetta Stone — Part B slow_query_log",
	}
}

// init registers ruleSlowLogDisabled as a native RuleDefinition.
func init() {
	register(RuleDefinition{
		ID:                 "config.slow_log_disabled",
		Subsystem:          "Configuration",
		Title:              "Slow query log is disabled",
		FormulaText:        "slow_query_log = OFF  AND  Questions/s > 0",
		MinRecommendations: 3,
		Severity:           SeverityHintWarning,
		Run:                ruleSlowLogDisabled,
	})
}
