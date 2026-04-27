package findings

import (
	"fmt"

	"github.com/matias-sanchez/My-gather/model"
)

// ruleThreadCacheHitRatio computes the thread-cache hit ratio as
// defined by the Rosetta Stone:
//
//	hit_ratio = 1 − Threads_created / Connections
//
// Want the value to be as close to 100 % as possible; a low value
// means MySQL is paying the OS cost of creating threads for every
// connection.
// See Rosetta Stone — Thread Cache §Saturation (capacity).
func ruleThreadCacheHitRatio(r *model.Report) Finding {
	const (
		warnBelow = 0.90 // < 90 % → Warn
		critBelow = 0.70 // < 70 % → Crit
	)
	threadsCreated, ok1 := counterTotal(r, "Threads_created")
	connections, ok2 := counterTotal(r, "Connections")
	if !ok1 || !ok2 || connections <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	ratio := 1 - threadsCreated/connections
	// Guard: the ratio can go negative if, say, the fixture only
	// captured a few seconds and Threads_created > Connections (edge
	// artifact). Clamp to [0, 1] for display, but keep the raw value
	// in the computed formula for honesty.
	display := ratio
	if display < 0 {
		display = 0
	}
	if display > 1 {
		display = 1
	}
	sev := SeverityOK
	summary := fmt.Sprintf("Thread-cache hit ratio is healthy at %s.", formatPercent(display))
	switch {
	case ratio < critBelow:
		sev = SeverityCrit
		summary = fmt.Sprintf("Thread-cache hit ratio is LOW (%s): most connections are creating new OS threads.", formatPercent(display))
	case ratio < warnBelow:
		sev = SeverityWarn
		summary = fmt.Sprintf("Thread-cache hit ratio is %s — below the 90 %% comfort band.", formatPercent(display))
	}
	cacheSize, _ := variableFloat(r, "thread_cache_size")
	return Finding{
		ID:        "threadcache.hit_ratio",
		Subsystem: "Thread Cache",
		Title:     "Thread cache hit ratio",
		Severity:  sev,
		Summary:   summary,
		Explanation: "MySQL caches per-connection threads to avoid paying thread-creation cost on every new connection. " +
			"If the cache is too small for the connection rate, every new connection pays the full create cost.",
		FormulaText: "hit_ratio = 1 − Threads_created / Connections",
		// FormulaComputed reports the REAL (unclamped) ratio. A
		// negative number here signals short/reset windows where
		// Threads_created > Connections — clamping that to 0 % would
		// hide the anomaly. The Summary above still uses the clamped
		// display value for human readability.
		FormulaComputed: fmt.Sprintf("1 − %s / %s = %s",
			formatNum(threadsCreated), formatNum(connections), formatPercent(ratio)),
		Metrics: []MetricRef{
			{Name: "Threads_created", Value: threadsCreated, Unit: "count"},
			{Name: "Connections", Value: connections, Unit: "count"},
			{Name: "thread_cache_size", Value: cacheSize, Unit: "threads"},
		},
		Recommendations: []string{
			"Increase thread_cache_size — MySQL's auto-sized default is often too small for bursty connection patterns.",
			"Investigate applications reconnecting instead of reusing connections (short-lived connections are the usual cause).",
		},
		Source: "Rosetta Stone — Thread Cache §Saturation (capacity)",
	}
}

// init registers the native RuleDefinition for the Thread Cache rule.
func init() {
	register(RuleDefinition{
		ID:                 "threadcache.hit_ratio",
		Subsystem:          "Thread Cache",
		Title:              "Thread cache hit ratio",
		FormulaText:        "1 - Threads_created / Connections",
		MinRecommendations: 1,
		Severity:           SeverityHintVariable,
		Run:                ruleThreadCacheHitRatio,
	})
}
