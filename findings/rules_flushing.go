package findings

import (
	"fmt"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

// ruleInnoDBFlushing surfaces InnoDB page-flushing back-pressure by
// inspecting each snapshot's BUFFER POOL AND MEMORY + FILE I/O
// details from SHOW ENGINE INNODB STATUS. It fires whenever the
// capture saw any flushing backlog and classifies severity by which
// path is saturated:
//
//	Single-page flushes > 0   → crit (user thread stalled waiting for a clean page)
//	Pending log fsyncs > 0    → crit (commits stalled on disk)
//	Flush-list backlog        → warn (checkpoint-age / redo pressure)
//	LRU flush backlog         → warn (free list exhausted)
//	Pending buffer-pool fsync → warn (page cleaners lagging)
//
// Recommendations branch by the dominant path so operators know which
// knob to turn (innodb_io_capacity_max, innodb_lru_scan_depth, redo
// sizing, buffer-pool instances, etc.).
func ruleInnoDBFlushing(r *model.Report) Finding {
	if r == nil || r.DBSection == nil {
		return Finding{Severity: SeveritySkip}
	}
	snaps := r.DBSection.InnoDBPerSnapshot
	if len(snaps) == 0 {
		return Finding{Severity: SeveritySkip}
	}

	var (
		peakLRU, peakFL, peakSP          int
		peakFsyncLog, peakFsyncBP        int
		peakModified                     int
		peakReads                        int
		peakWrites                       int
		peakSnapshotWrites               string
		peakWritesCombined               = -1
		anySingle, anyFsyncLog, anyFsync bool
	)
	for _, si := range snaps {
		if si.Data == nil {
			continue
		}
		d := si.Data
		if d.PendingWritesLRU > peakLRU {
			peakLRU = d.PendingWritesLRU
		}
		if d.PendingWritesFlushList > peakFL {
			peakFL = d.PendingWritesFlushList
		}
		if d.PendingWritesSinglePage > peakSP {
			peakSP = d.PendingWritesSinglePage
		}
		if d.PendingFsyncLog > peakFsyncLog {
			peakFsyncLog = d.PendingFsyncLog
		}
		if d.PendingFsyncBufferPool > peakFsyncBP {
			peakFsyncBP = d.PendingFsyncBufferPool
		}
		if d.ModifiedDBPages > peakModified {
			peakModified = d.ModifiedDBPages
		}
		if d.PendingReads > peakReads {
			peakReads = d.PendingReads
		}
		if d.PendingWrites > peakWrites {
			peakWrites = d.PendingWrites
		}
		if d.PendingWrites > peakWritesCombined {
			peakWritesCombined = d.PendingWrites
			peakSnapshotWrites = si.SnapshotPrefix
		}
		if d.PendingWritesSinglePage > 0 {
			anySingle = true
		}
		if d.PendingFsyncLog > 0 {
			anyFsyncLog = true
		}
		if d.PendingFsyncLog > 0 || d.PendingFsyncBufferPool > 0 {
			anyFsync = true
		}
	}

	anyBacklog := peakLRU > 0 || peakFL > 0 || peakSP > 0 ||
		peakFsyncLog > 0 || peakFsyncBP > 0 || peakWrites > 0 || peakReads > 0
	if !anyBacklog {
		return Finding{Severity: SeveritySkip}
	}

	sev := SeverityWarn
	head := fmt.Sprintf("InnoDB flushing backlog observed — peak %s pending writes.", formatNum(float64(peakWrites)))
	if anySingle || anyFsyncLog {
		sev = SeverityCrit
		switch {
		case anySingle && anyFsyncLog:
			head = "InnoDB is stalling — single-page flushes AND pending log fsyncs were both observed."
		case anySingle:
			head = fmt.Sprintf("InnoDB single-page flushes observed (peak %s) — user threads are stalling on clean pages.", formatNum(float64(peakSP)))
		default:
			head = fmt.Sprintf("InnoDB pending log fsyncs peaked at %s — commits are stalling on disk.", formatNum(float64(peakFsyncLog)))
		}
	}

	// Branch-specific recommendations — emit only the ones whose
	// underlying metric was actually non-zero so operators don't get
	// drowned in generic advice.
	recs := []string{}
	if anySingle {
		recs = append(recs,
			"Single-page flushes: grow the buffer pool or free pages are too scarce. Raise innodb_buffer_pool_size if memory allows, or raise innodb_lru_scan_depth so the cleaner finds more free pages per pass.")
	}
	if anyFsyncLog {
		recs = append(recs,
			"Pending log fsyncs > 0: redo is fsync-bound. Check disk await/util for the log device (correlate with the iostat panel), consider group commit tuning, or move redo to a faster device.")
	}
	if peakFL > 0 {
		recs = append(recs,
			"Flush-list backlog: checkpoint age is pushing the cleaner. Raise innodb_io_capacity_max or grow innodb_log_file_size so the allowed dirty window is wider.")
	}
	if peakLRU > 0 {
		recs = append(recs,
			"LRU flush backlog: buffer-pool free list is thin. Raise innodb_lru_scan_depth and consider increasing innodb_buffer_pool_instances.")
	}
	if peakFsyncBP > 0 {
		recs = append(recs,
			"Buffer-pool fsync backlog: page cleaners can't drain. Raise innodb_page_cleaners (one per BP instance), and review innodb_flush_neighbors on SSD.")
	}
	if peakReads > 0 && len(recs) == 0 {
		recs = append(recs,
			"Sustained pending reads: buffer pool is too small for the working set. Grow innodb_buffer_pool_size or review whether a specific workload is scanning large tables.")
	}
	// Always include a generic hint at the tail.
	recs = append(recs,
		"Correlate with the -iostat panel: %util and await on the log/data devices usually explain which knob matters first.")

	metrics := []MetricRef{
		{Name: "peak single-page flushes", Value: float64(peakSP), Unit: "count"},
		{Name: "peak flush-list backlog", Value: float64(peakFL), Unit: "count"},
		{Name: "peak LRU-flush backlog", Value: float64(peakLRU), Unit: "count"},
		{Name: "peak pending log fsyncs", Value: float64(peakFsyncLog), Unit: "count"},
		{Name: "peak pending BP fsyncs", Value: float64(peakFsyncBP), Unit: "count"},
		{Name: "peak modified db pages", Value: float64(peakModified), Unit: "pages"},
		{Name: "peak pending reads", Value: float64(peakReads), Unit: "count"},
		{Name: "peak pending writes", Value: float64(peakWrites), Unit: "count", Note: "snapshot " + peakSnapshotWrites},
	}

	// Build a human-readable Computed line listing only the non-zero
	// paths so the Advisor doesn't print "0, 0, 0, 0" for healthy
	// pools. FormulaText stays symbolic.
	var paths []string
	if peakLRU > 0 {
		paths = append(paths, fmt.Sprintf("LRU=%d", peakLRU))
	}
	if peakFL > 0 {
		paths = append(paths, fmt.Sprintf("flushList=%d", peakFL))
	}
	if peakSP > 0 {
		paths = append(paths, fmt.Sprintf("singlePage=%d", peakSP))
	}
	if peakFsyncLog > 0 {
		paths = append(paths, fmt.Sprintf("fsyncLog=%d", peakFsyncLog))
	}
	if peakFsyncBP > 0 {
		paths = append(paths, fmt.Sprintf("fsyncBP=%d", peakFsyncBP))
	}
	computed := strings.Join(paths, ", ")
	if computed == "" {
		computed = fmt.Sprintf("pendingReads=%d pendingWrites=%d", peakReads, peakWrites)
	}

	_ = anyFsync // included in summary via anyFsyncLog/anyBacklog
	return Finding{
		ID:              "innodb.flushing",
		Subsystem:       "Buffer Pool",
		Title:           "InnoDB flushing pressure",
		Severity:        sev,
		Summary:         head,
		Explanation:     "InnoDB flushes dirty pages through three separate paths: LRU (eviction-driven), flush list (checkpoint-age driven) and single-page (synchronous, last-resort). A non-zero single-page count or a non-zero pending log fsync count represents a user-visible stall. Flush-list or LRU backlogs are earlier warnings — the cleaner is falling behind.",
		FormulaText:     "any of {singlePage, fsyncLog, LRU, flushList, fsyncBP} > 0",
		FormulaComputed: fmt.Sprintf("peak: %s", computed),
		Metrics:         metrics,
		Recommendations: recs,
		Source:          "Rosetta Stone — Buffer Pool §Saturation (flushing), Redo Log §Saturation (fsync)",
	}
}

// init registers ruleInnoDBFlushing as a native RuleDefinition.
func init() {
	register(RuleDefinition{
		ID:                 "innodb.flushing",
		Subsystem:          "Buffer Pool",
		Title:              "InnoDB flushing pressure",
		FormulaText:        "any of {singlePage, fsyncLog, LRU, flushList, fsyncBP} > 0",
		MinRecommendations: 3,
		Severity:           SeverityHintVariable,
		Run:                ruleInnoDBFlushing,
	})
}
