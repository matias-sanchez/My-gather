package findings

import (
	"fmt"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

// ruleSemaphoreWaits flags InnoDB SEMAPHORES-section contention —
// i.e. any "--Thread X has waited at FILE:LINE" records in the
// captured SHOW ENGINE INNODB STATUS output. Even a single waiter
// means at least one thread is blocked on a latch, and sustained
// peaks in the hundreds are a classic symptom of hot mutex contention
// (TRX_SYS, buffer pool LRU, index mutex, etc.).
//
// Thresholds are per-snapshot peaks (not rates). The breakdown and
// visual red/yellow accent on the InnoDB Semaphores callout come from
// the same underlying data, so the advisor card and the callout
// severity always agree.
//
//	peak ≥ 500 → crit
//	peak >   0 → warn
//	otherwise  → skip (no contention observed)
func ruleSemaphoreWaits(r *model.Report) Finding {
	const (
		warnAbove = 1   // any non-zero waiting thread
		critAbove = 500 // sustained heavy contention
	)
	if r == nil || r.DBSection == nil {
		return Finding{Severity: SeveritySkip}
	}
	snaps := r.DBSection.InnoDBPerSnapshot
	if len(snaps) == 0 {
		return Finding{Severity: SeveritySkip}
	}

	var (
		peakCount    int
		peakSnapshot string
		peakSites    []model.SemaphoreSite
		totalWaits   int
		snapsWithHit int
	)
	for _, si := range snaps {
		if si.Data == nil {
			continue
		}
		totalWaits += si.Data.SemaphoreCount
		if si.Data.SemaphoreCount > 0 {
			snapsWithHit++
		}
		if si.Data.SemaphoreCount > peakCount {
			peakCount = si.Data.SemaphoreCount
			peakSnapshot = si.SnapshotPrefix
			peakSites = si.Data.SemaphoreSites
		}
	}
	if peakCount < warnAbove {
		return Finding{Severity: SeveritySkip}
	}

	sev := SeverityWarn
	head := fmt.Sprintf("InnoDB semaphore contention — peak %s waiting threads in one snapshot.", formatNum(float64(peakCount)))
	if peakCount >= critAbove {
		sev = SeverityCrit
		head = fmt.Sprintf("Critical InnoDB semaphore contention — peak %s waiting threads in one snapshot.", formatNum(float64(peakCount)))
	}

	// Top-3 hot sites from the peak snapshot — give the operator a
	// direct pointer to the likely hot mutex before they even expand
	// the advisor card.
	topSites := ""
	if len(peakSites) > 0 {
		limit := 3
		if len(peakSites) < limit {
			limit = len(peakSites)
		}
		parts := make([]string, 0, limit)
		for i := 0; i < limit; i++ {
			s := peakSites[i]
			name := s.MutexName
			if name == "" {
				name = "–"
			}
			parts = append(parts, fmt.Sprintf("%s:%d (%s, %d waits)", s.File, s.Line, name, s.WaitCount))
		}
		topSites = strings.Join(parts, "; ")
	}

	metrics := []MetricRef{
		{Name: "peak waiting threads", Value: float64(peakCount), Unit: "count", Note: "snapshot " + peakSnapshot},
		{Name: "snapshots with waiters", Value: float64(snapsWithHit), Unit: "count", Note: fmt.Sprintf("of %d total", len(snaps))},
		{Name: "total waits across window", Value: float64(totalWaits), Unit: "count"},
	}

	explanation := "InnoDB's SEMAPHORES section lists threads currently blocked on internal latches " +
		"(mutexes, rw-locks). Zero waiters is the normal steady state. Any sustained waiters mean " +
		"requests are queueing behind a hot mutex — throughput is capped by that single point of " +
		"serialization and latency grows with concurrency."
	if topSites != "" {
		explanation += " The peak snapshot's top call sites were: " + topSites + "."
	}

	recs := []string{
		"Identify the hot mutex from the call sites above (TRX_SYS, IBUF, buffer-pool, index, lock-sys). Each points at a distinct remediation path.",
		"For TRX_SYS pressure (trx0sys.*): reduce long-running transactions, lower concurrent writers, and enable innodb_adaptive_hash_index only if it isn't already fighting TRX_SYS itself.",
		"For IBUF / insert buffer contention (ibuf0ibuf.*): the change buffer is oversubscribed — consider disabling it (innodb_change_buffering=none) on SSD-backed instances or raising innodb_io_capacity.",
		"For buffer pool contention (buf0buf.*): add instances (innodb_buffer_pool_instances) and verify the pool is large enough for the working set.",
		"If the workload is genuinely over-concurrent for the hardware, cap innodb_thread_concurrency or reduce application-side parallelism.",
	}

	return Finding{
		ID:              "innodb.semaphores",
		Subsystem:       "InnoDB Semaphores",
		Title:           "InnoDB semaphore contention",
		Severity:        sev,
		Summary:         head,
		Explanation:     explanation,
		FormulaText:     "peak waiting threads > 0  (crit ≥ 500)",
		FormulaComputed: fmt.Sprintf("peak = %d waiting threads @ snapshot %s", peakCount, peakSnapshot),
		Metrics:         metrics,
		Recommendations: recs,
		Source:          "Rosetta Stone — InnoDB SEMAPHORES section (contention detection)",
	}
}
