package findings

import (
	"fmt"

	"github.com/matias-sanchez/My-gather/model"
)

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
