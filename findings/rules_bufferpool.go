package findings

import (
	"fmt"

	"github.com/matias-sanchez/My-gather/model"
)

// ruleBPUndersized flags a suspiciously small innodb_buffer_pool_size
// relative to the server's apparent usage. The hit-ratio rule only
// catches thrashing that's already happening; this one catches
// under-provisioned pools that are coasting in a quiet moment but
// can't scale with the workload.
//
// Heuristic (no canonical threshold exists): on an instance with any
// real concurrency (Threads_connected > 50) and uptime past its warm-up
// phase (Uptime > 3600), a buffer pool below 1 GiB is Warn and below
// 256 MiB is Crit.
func ruleBPUndersized(r *model.Report) Finding {
	const (
		warnBytes  = float64(1) * 1024 * 1024 * 1024 // 1 GiB
		critBytes  = float64(256) * 1024 * 1024      // 256 MiB
		minThreads = 50.0
		minUptimeS = 3600.0
	)
	size, ok := variableFloat(r, "innodb_buffer_pool_size")
	if !ok || size <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	connected, _ := gaugeMax(r, "Threads_connected")
	uptime, _ := gaugeLast(r, "Uptime")
	if connected < minThreads || uptime < minUptimeS {
		// Dev/idle instance — size heuristic doesn't apply. Skip.
		return Finding{Severity: SeveritySkip}
	}
	sev := SeverityOK
	summary := fmt.Sprintf("innodb_buffer_pool_size is %s — sized for the observed workload.", humanBytes(size))
	switch {
	case size < critBytes:
		sev = SeverityCrit
		summary = fmt.Sprintf("innodb_buffer_pool_size is %s on an instance with %s connections — far too small for a production workload.",
			humanBytes(size), formatNum(connected))
	case size < warnBytes:
		sev = SeverityWarn
		summary = fmt.Sprintf("innodb_buffer_pool_size is only %s — likely under-provisioned; current hit ratio may only be healthy because the workload is quiet.",
			humanBytes(size))
	}
	return Finding{
		ID:        "bp.undersized",
		Subsystem: "Buffer Pool",
		Title:     "Buffer pool size vs workload",
		Severity:  sev,
		Summary:   summary,
		Explanation: "The buffer pool caches hot data and index pages in memory. A small pool relative to the active dataset will thrash " +
			"once the workload pushes past a quiet window, even if the current hit ratio looks healthy. On a production instance with " +
			"dozens of active connections, 1 GiB is the floor for most deployments; 25-70 % of system RAM is the usual target.",
		FormulaText:     "innodb_buffer_pool_size  vs  {≥1 GiB warn floor, ≥256 MiB crit floor}  (gated on Threads_connected ≥ 50 and Uptime ≥ 1h)",
		FormulaComputed: fmt.Sprintf("innodb_buffer_pool_size = %s  (Threads_connected peak = %s, Uptime = %s s)", humanBytes(size), formatNum(connected), formatNum(uptime)),
		Metrics: []MetricRef{
			{Name: "innodb_buffer_pool_size", Value: size, Unit: "bytes"},
			{Name: "Threads_connected (max)", Value: connected, Unit: "count"},
			{Name: "Uptime", Value: uptime, Unit: "s"},
		},
		Recommendations: []string{
			"Raise innodb_buffer_pool_size to 25-70 % of system RAM (reserve headroom for per-thread buffers, OS, and other processes).",
			"Confirm with `SELECT @@innodb_buffer_pool_size / 1024 / 1024 / 1024` and your system's free memory before bumping.",
			"On MySQL 8.0.14+, the pool is dynamically resizable — no restart required.",
		},
		Source: "Rosetta Stone — Buffer Pool §Utilization (capacity)",
	}
}

// humanBytes formats a byte count as "128 MiB" / "2.5 GiB" / "900 KiB".
func humanBytes(v float64) string {
	const (
		KiB = 1024.0
		MiB = KiB * 1024
		GiB = MiB * 1024
		TiB = GiB * 1024
	)
	switch {
	case v >= TiB:
		return fmt.Sprintf("%.2f TiB", v/TiB)
	case v >= GiB:
		return fmt.Sprintf("%.2f GiB", v/GiB)
	case v >= MiB:
		return fmt.Sprintf("%.0f MiB", v/MiB)
	case v >= KiB:
		return fmt.Sprintf("%.0f KiB", v/KiB)
	default:
		return fmt.Sprintf("%.0f B", v)
	}
}

// ruleBPHitRatio evaluates the InnoDB buffer pool hit ratio:
//
//	hit_ratio = 1 − reads / read_requests
//
// See Rosetta Stone — Buffer Pool §Utilization (throughput).
func ruleBPHitRatio(r *model.Report) Finding {
	const (
		warnThreshold = 0.999 // below this = Warn
		critThreshold = 0.990 // below this = Crit
	)
	reads, ok1 := counterTotal(r, "Innodb_buffer_pool_reads")
	reqs, ok2 := counterTotal(r, "Innodb_buffer_pool_read_requests")
	if !ok1 || !ok2 || reqs <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	ratio := 1 - reads/reqs
	sev := SeverityOK
	summary := fmt.Sprintf("Buffer pool hit ratio is healthy at %s.", formatPercent(ratio))
	switch {
	case ratio < critThreshold:
		sev = SeverityCrit
		summary = fmt.Sprintf("Buffer pool hit ratio is LOW (%s): queries are frequently hitting disk.", formatPercent(ratio))
	case ratio < warnThreshold:
		sev = SeverityWarn
		summary = fmt.Sprintf("Buffer pool hit ratio is degraded at %s — below the 99.9%% comfort band.", formatPercent(ratio))
	}
	return Finding{
		ID:        "bp.hit_ratio",
		Subsystem: "Buffer Pool",
		Title:     "Buffer pool hit ratio",
		Severity:  sev,
		Summary:   summary,
		Explanation: "The InnoDB buffer pool caches recently-used pages in memory. " +
			"Each logical read that finds its page already cached ('read request') avoids a physical disk read. " +
			"A hit ratio near 100 % means the working set fits; a dropping ratio means queries " +
			"increasingly wait on disk I/O.",
		FormulaText: "hit_ratio = 1 − Innodb_buffer_pool_reads / Innodb_buffer_pool_read_requests",
		FormulaComputed: fmt.Sprintf("1 − %s / %s = %s",
			formatNum(reads), formatNum(reqs), formatPercent(ratio)),
		Metrics: []MetricRef{
			{Name: "Innodb_buffer_pool_reads", Value: reads, Unit: "count", Note: "physical reads during capture"},
			{Name: "Innodb_buffer_pool_read_requests", Value: reqs, Unit: "count", Note: "logical reads during capture"},
		},
		Recommendations: []string{
			"Consider increasing innodb_buffer_pool_size to fit the active dataset.",
			"Profile which tables / queries cause the misses (pt-query-digest, performance_schema).",
			"Check for very large scans pulling cold data into the pool and evicting hot pages.",
		},
		Source: "Rosetta Stone — Buffer Pool §Utilization (throughput)",
	}
}

// ruleBPFreePagesLow flags buffer pool saturation when free pages
// drop below the LRU scan depth × instances AND physical reads > 0.
// See Rosetta Stone — Buffer Pool §Saturation (capacity).
func ruleBPFreePagesLow(r *model.Report) Finding {
	free, ok1 := gaugeLast(r, "Innodb_buffer_pool_pages_free")
	scanDepth, ok2 := variableFloat(r, "innodb_lru_scan_depth")
	instances, ok3 := variableFloat(r, "innodb_buffer_pool_instances")
	readsRate, ok4 := counterRatePerSec(r, "Innodb_buffer_pool_reads")
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return Finding{Severity: SeveritySkip}
	}
	threshold := scanDepth * instances
	// Canonical Rosetta Stone predicate is
	//   (free <= threshold) AND (reads > 0)
	// so each of the three outcomes gets its own branch with an
	// accurate message — no branch claims a condition that wasn't
	// actually checked.
	if readsRate <= 0 {
		// Idle window: without physical reads we can't evaluate
		// whether the pool is under pressure. Drop the finding.
		return Finding{Severity: SeveritySkip}
	}
	if free > threshold {
		return Finding{
			ID:          "bp.free_pages_low",
			Subsystem:   "Buffer Pool",
			Title:       "Buffer pool free pages headroom",
			Severity:    SeverityOK,
			Summary:     fmt.Sprintf("Free pages (%s) comfortably above the LRU-scan-depth threshold (%s).", formatNum(free), formatNum(threshold)),
			FormulaText: "free_pages > innodb_lru_scan_depth × innodb_buffer_pool_instances",
			FormulaComputed: fmt.Sprintf("%s > %s × %s = %s",
				formatNum(free), formatNum(scanDepth), formatNum(instances), formatNum(threshold)),
			Metrics: []MetricRef{
				{Name: "Innodb_buffer_pool_pages_free", Value: free, Unit: "pages", Note: "last sample"},
				{Name: "innodb_lru_scan_depth", Value: scanDepth, Unit: "pages"},
				{Name: "innodb_buffer_pool_instances", Value: instances, Unit: "count"},
				{Name: "Innodb_buffer_pool_reads/s", Value: readsRate, Unit: "/s"},
			},
			Source: "Rosetta Stone — Buffer Pool §Saturation (capacity)",
		}
	}
	sev := SeverityWarn
	summary := fmt.Sprintf("Free pages (%s) have fallen below scan_depth × instances (%s) while physical reads continue at %s/s.",
		formatNum(free), formatNum(threshold), formatNum(readsRate))
	// Strong signal if we're also actively flushing via LRU
	if v, ok := counterRatePerSec(r, "Innodb_buffer_pool_pages_LRU_flushed"); ok && v > 0 {
		sev = SeverityCrit
		summary += " Background LRU flushing is active, confirming the buffer pool is under pressure."
	}
	return Finding{
		ID:        "bp.free_pages_low",
		Subsystem: "Buffer Pool",
		Title:     "Buffer pool free pages exhausted",
		Severity:  sev,
		Summary:   summary,
		Explanation: "When the page cleaner can't maintain a healthy free-list (at least lru_scan_depth free pages per instance), " +
			"incoming read requests will stall waiting for a free page, and the page cleaner has to flush synchronously on the query path.",
		FormulaText: "free_pages <= innodb_lru_scan_depth × innodb_buffer_pool_instances  AND  Innodb_buffer_pool_reads/s > 0",
		FormulaComputed: fmt.Sprintf("%s <= %s × %s = %s  AND  %s /s > 0",
			formatNum(free), formatNum(scanDepth), formatNum(instances), formatNum(threshold), formatNum(readsRate)),
		Metrics: []MetricRef{
			{Name: "Innodb_buffer_pool_pages_free", Value: free, Unit: "pages", Note: "last sample"},
			{Name: "innodb_lru_scan_depth", Value: scanDepth, Unit: "pages"},
			{Name: "innodb_buffer_pool_instances", Value: instances, Unit: "count"},
			{Name: "Innodb_buffer_pool_reads/s", Value: readsRate, Unit: "/s"},
		},
		Recommendations: []string{
			"Increase innodb_buffer_pool_size so the working set fits with headroom.",
			"Check the MySQL error log for \"[Warning] InnoDB: Difficult to Find Free Blocks in the Buffer Pool\".",
			"If CPU is spare, raising innodb_page_cleaners (and innodb_lru_scan_depth) helps the cleaner keep up.",
		},
		Source: "Rosetta Stone — Buffer Pool §Saturation (capacity)",
	}
}

// ruleBPLRUFlushing flags any LRU flushing during the capture —
// background LRU flushing means the page cleaner couldn't keep the
// free list stocked purely from the flush list.
// See Rosetta Stone — Buffer Pool §Saturation (capacity).
func ruleBPLRUFlushing(r *model.Report) Finding {
	rate, ok := counterRatePerSec(r, "Innodb_buffer_pool_pages_LRU_flushed")
	if !ok {
		return Finding{Severity: SeveritySkip}
	}
	if rate <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	return Finding{
		ID:        "bp.lru_flushing",
		Subsystem: "Buffer Pool",
		Title:     "LRU flushing active",
		Severity:  SeverityWarn,
		Summary:   fmt.Sprintf("Background LRU flushing observed at %s pages/s — the free list is not keeping up with demand.", formatNum(rate)),
		Explanation: "LRU flushing is a secondary path the page cleaner takes when the free list is short: " +
			"it evicts dirty pages from the LRU tail to replenish free pages. Regular activity here indicates " +
			"the buffer pool is under steady-state pressure.",
		FormulaText:     "Innodb_buffer_pool_pages_LRU_flushed/s > 0",
		FormulaComputed: fmt.Sprintf("%s /s > 0", formatNum(rate)),
		Metrics: []MetricRef{
			{Name: "Innodb_buffer_pool_pages_LRU_flushed/s", Value: rate, Unit: "/s"},
		},
		Recommendations: []string{
			"Increase innodb_buffer_pool_size to reduce churn.",
			"Consider raising innodb_lru_scan_depth and/or innodb_page_cleaners so the cleaner stays ahead.",
		},
		Source: "Rosetta Stone — Buffer Pool §Saturation (capacity)",
	}
}

// ruleBPWaitFree fires when any transaction had to wait for a free
// page. This is a strong saturation signal.
// See Rosetta Stone — Buffer Pool §Saturation (throughput).
func ruleBPWaitFree(r *model.Report) Finding {
	rate, ok := counterRatePerSec(r, "Innodb_buffer_pool_wait_free")
	if !ok {
		return Finding{Severity: SeveritySkip}
	}
	if rate <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	return Finding{
		ID:        "bp.wait_free",
		Subsystem: "Buffer Pool",
		Title:     "Query threads waiting for free buffer pool pages",
		Severity:  SeverityCrit,
		Summary:   fmt.Sprintf("Innodb_buffer_pool_wait_free is incrementing at %s/s — user queries are stalling on buffer pool exhaustion.", formatNum(rate)),
		Explanation: "This counter increments every time a thread could not find a clean page in the buffer pool " +
			"and had to wait for the page cleaner to produce one. Any non-zero rate is a red flag.",
		FormulaText:     "Innodb_buffer_pool_wait_free/s > 0",
		FormulaComputed: fmt.Sprintf("%s /s > 0", formatNum(rate)),
		Metrics: []MetricRef{
			{Name: "Innodb_buffer_pool_wait_free/s", Value: rate, Unit: "/s"},
		},
		Recommendations: []string{
			"Increase innodb_buffer_pool_size.",
			"Increase the aggressiveness of the page cleaner (innodb_lru_scan_depth, innodb_page_cleaners).",
			"Inspect the SEMAPHORES section of SHOW ENGINE INNODB STATUS for buf0* waits.",
		},
		Source: "Rosetta Stone — Buffer Pool §Saturation (throughput)",
	}
}

// ruleBPDirtyPct flags the buffer pool dirty-page ratio crossing the
// configured innodb_max_dirty_pages_pct threshold. Saturation here
// means the page cleaner is behind and InnoDB will flush
// aggressively, eating IO budget.
// See Rosetta Stone — Buffer Pool §Saturation (capacity).
func ruleBPDirtyPct(r *model.Report) Finding {
	dirty, ok1 := gaugeMax(r, "Innodb_buffer_pool_pages_dirty")
	total, ok2 := gaugeLast(r, "Innodb_buffer_pool_pages_total")
	limit, ok3 := variableFloat(r, "innodb_max_dirty_pages_pct")
	if !ok1 || !ok2 || !ok3 || total <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	pct := dirty / total * 100
	threshold := limit * 0.9
	sev := SeverityOK
	summary := fmt.Sprintf("Dirty-page ratio peaked at %.2f %% — within the %.0f %% limit.", pct, limit)
	switch {
	case pct >= limit:
		sev = SeverityCrit
		summary = fmt.Sprintf("Dirty-page ratio reached %.2f %% during the capture, hitting the %.0f %% limit — InnoDB is flushing aggressively.", pct, limit)
	case pct >= threshold:
		sev = SeverityWarn
		summary = fmt.Sprintf("Dirty-page ratio peaked at %.2f %%, within 10 %% of the %.0f %% limit.", pct, limit)
	}
	return Finding{
		ID:        "bp.dirty_pct",
		Subsystem: "Buffer Pool",
		Title:     "Dirty-page ratio",
		Severity:  sev,
		Summary:   summary,
		Explanation: "InnoDB starts aggressive flushing when the fraction of dirty pages crosses innodb_max_dirty_pages_pct. " +
			"At that point the page cleaner consumes IO that would otherwise serve user queries, which can look like a latency spike.",
		FormulaText: "dirty_pct = Innodb_buffer_pool_pages_dirty / Innodb_buffer_pool_pages_total × 100  vs  innodb_max_dirty_pages_pct",
		FormulaComputed: fmt.Sprintf("%s / %s × 100 = %.2f %%  vs  %.0f %%",
			formatNum(dirty), formatNum(total), pct, limit),
		Metrics: []MetricRef{
			{Name: "Innodb_buffer_pool_pages_dirty (max)", Value: dirty, Unit: "pages"},
			{Name: "Innodb_buffer_pool_pages_total", Value: total, Unit: "pages"},
			{Name: "innodb_max_dirty_pages_pct", Value: limit, Unit: "%"},
		},
		Recommendations: []string{
			"If the workload is write-heavy, increase the redo log size so checkpointing is less frequent.",
			"Tune innodb_io_capacity / innodb_io_capacity_max to match the storage's true IOPS budget.",
			"Consider lowering innodb_max_dirty_pages_pct_lwm so flushing starts earlier and more gradually.",
		},
		Source: "Rosetta Stone — Buffer Pool §Saturation (capacity), Part B Innodb_buffer_pool_pages_dirty",
	}
}

// init registers the native RuleDefinition entries for every
// Buffer Pool rule in this file. Each rule's metadata lives next
// to its Run function so the registry's rules catalogue and the
// rendered Finding cannot drift independently.
func init() {
	register(RuleDefinition{
		ID:                 "bp.hit_ratio",
		Subsystem:          "Buffer Pool",
		Title:              "Buffer pool hit ratio",
		FormulaText:        "hit_ratio = 1 − Innodb_buffer_pool_reads / Innodb_buffer_pool_read_requests",
		MinRecommendations: 3,
		Severity:           SeverityHintVariable,
		Run:                ruleBPHitRatio,
	})
	register(RuleDefinition{
		ID:                 "bp.undersized",
		Subsystem:          "Buffer Pool",
		Title:              "Buffer pool size vs workload",
		FormulaText:        "innodb_buffer_pool_size  vs  {>=1 GiB warn floor, >=256 MiB crit floor}  (gated on Threads_connected >= 50 and Uptime >= 1h)",
		MinRecommendations: 3,
		Severity:           SeverityHintVariable,
		Run:                ruleBPUndersized,
	})
	register(RuleDefinition{
		ID:                 "bp.free_pages_low",
		Subsystem:          "Buffer Pool",
		Title:              "Buffer pool free-page headroom",
		FormulaText:        "Innodb_buffer_pool_pages_free  vs  innodb_lru_scan_depth * innodb_buffer_pool_instances",
		MinRecommendations: 3,
		Severity:           SeverityHintVariable,
		Run:                ruleBPFreePagesLow,
	})
	register(RuleDefinition{
		ID:                 "bp.lru_flushing",
		Subsystem:          "Buffer Pool",
		Title:              "LRU-list page flushing rate",
		FormulaText:        "Innodb_buffer_pool_pages_LRU_flushed/s",
		MinRecommendations: 2,
		Severity:           SeverityHintVariable,
		Run:                ruleBPLRUFlushing,
	})
	register(RuleDefinition{
		ID:                 "bp.wait_free",
		Subsystem:          "Buffer Pool",
		Title:              "Buffer pool wait_free events",
		FormulaText:        "Innodb_buffer_pool_wait_free/s > 0",
		MinRecommendations: 3,
		Severity:           SeverityHintCritical,
		Run:                ruleBPWaitFree,
	})
	register(RuleDefinition{
		ID:                 "bp.dirty_pct",
		Subsystem:          "Buffer Pool",
		Title:              "Buffer pool dirty-page percentage",
		FormulaText:        "Innodb_buffer_pool_pages_dirty / Innodb_buffer_pool_pages_total  vs  innodb_max_dirty_pages_pct",
		MinRecommendations: 3,
		Severity:           SeverityHintVariable,
		Run:                ruleBPDirtyPct,
	})
}
