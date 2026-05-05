package render

import (
	"strconv"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/reportutil"
)

// aggregateInnoDBMetrics rolls every per-snapshot InnoDB scalar up
// into one "worst-plus-distribution" card per metric. Max is treated
// as "worst" for all four metrics — for Semaphores, Pending I/O, and
// History list, higher means more trouble; for AHI searches/sec we
// also surface the peak because an unexplained spike or collapse is
// what a reader would want to notice first. Snapshots whose Data is
// nil (e.g. a missing -innodbstatus1 file) are simply skipped.
func aggregateInnoDBMetrics(snaps []model.SnapshotInnoDB) []innoDBMetricView {
	var sem, pio, hll []int
	for _, si := range snaps {
		if si.Data == nil {
			continue
		}
		sem = append(sem, si.Data.SemaphoreCount)
		pio = append(pio, si.Data.PendingReads+si.Data.PendingWrites)
		hll = append(hll, si.Data.HistoryListLength)
	}
	if len(sem) == 0 {
		return nil
	}
	semMetric := innoDBIntMetric("Semaphores", "waiting", sem)
	semMetric.Key = "semaphores"
	attachSemaphoreBreakdown(&semMetric, snaps)
	// Peak semaphore waits is a reliable contention signal — any
	// non-zero reading is a yellow flag, and high peaks indicate a
	// hotspot bad enough to surface as a red callout.
	if peak := maxOfInts(sem); peak >= 500 {
		semMetric.Severity = "crit"
	} else if peak > 0 {
		semMetric.Severity = "warn"
	}
	pioMetric := innoDBIntMetric("Pending I/O", "reads + writes", pio)
	pioMetric.Key = "pending_io"
	attachPendingIOBreakdown(&pioMetric, snaps)
	ahiMetric := buildAHIMetric(snaps)
	ahiMetric.Key = "ahi"
	hllMetric := innoDBIntMetric("History list", "HLL (undo depth)", hll)
	hllMetric.Key = "history_list"
	return []innoDBMetricView{
		semMetric,
		pioMetric,
		ahiMetric,
		hllMetric,
	}
}

// buildAHIMetric renders the AHI card as a hit-ratio view instead of
// a raw "searches/sec" rate. The hit ratio is the fraction of index
// lookups served directly from the adaptive hash index rather than
// by walking the B-tree, so a low value is the interesting signal —
// it means AHI is contributing little and the memory it occupies
// might be better spent elsewhere. Worst = lowest ratio across
// snapshots that had any activity; min/avg/max mirror that view.
//
// Formula: hit_ratio = hash_searches / (hash_searches + non_hash_searches)
//
// Snapshots with zero activity (hash + non-hash == 0) are skipped —
// dividing 0/0 is meaningless and lets an idle capture collapse the
// ratio to 0 which would misrepresent the server.
//
// AHIHashTableSize is the last non-zero size observed in iteration
// order; it usually equals the worst snapshot's size but can diverge
// if the instance restarted mid-window.
func buildAHIMetric(snaps []model.SnapshotInnoDB) innoDBMetricView {
	var ratios []float64
	type active struct {
		prefix        string
		hash, nonHash float64
		ratio         float64
	}
	var activeSnaps []active
	hashTblSize := 0
	for _, si := range snaps {
		if si.Data == nil {
			continue
		}
		if si.Data.AHIActivity.HashTableSize > 0 {
			hashTblSize = si.Data.AHIActivity.HashTableSize
		}
		h := si.Data.AHIActivity.SearchesPerSec
		nh := si.Data.AHIActivity.NonHashSearchesPerSec
		if h+nh <= 0 {
			continue
		}
		r := h * 100.0 / (h + nh)
		ratios = append(ratios, r)
		activeSnaps = append(activeSnaps, active{
			prefix: si.SnapshotPrefix, hash: h, nonHash: nh, ratio: r,
		})
	}
	if len(ratios) == 0 {
		return innoDBMetricView{
			Label:            "AHI",
			Hint:             "hit ratio",
			Worst:            "–",
			Min:              "–",
			Avg:              "–",
			Max:              "–",
			AHIFormula:       true,
			AHIFormulaText:   "hash / (hash + non-hash)",
			AHIHashTableSize: reportutil.HumanIntOrDash(int64(hashTblSize)),
			AHIHasHashTable:  hashTblSize > 0,
			AHINoActivity:    true,
		}
	}
	mn, mx, total := ratios[0], ratios[0], 0.0
	worstIdx := 0
	for i, r := range ratios {
		if r < mn {
			mn = r
			worstIdx = i
		}
		if r > mx {
			mx = r
		}
		total += r
	}
	avg := total / float64(len(ratios))
	worst := activeSnaps[worstIdx]
	return innoDBMetricView{
		Label:            "AHI",
		Hint:             "hit ratio",
		Worst:            formatFloat(mn, 1) + "%",
		Min:              formatFloat(mn, 1) + "%",
		Avg:              formatFloat(avg, 1) + "%",
		Max:              formatFloat(mx, 1) + "%",
		AHIFormula:       true,
		AHIFormulaText:   "hash / (hash + non-hash)",
		AHIWorstSnapshot: worst.prefix,
		AHIWorstHash:     formatFloat(worst.hash, 2),
		AHIWorstNonHash:  formatFloat(worst.nonHash, 2),
		AHIWorstRatio:    formatFloat(worst.ratio, 1),
		AHIHashTableSize: reportutil.HumanIntOrDash(int64(hashTblSize)),
		AHIHasHashTable:  hashTblSize > 0,
	}
}

// attachSemaphoreBreakdown populates the Semaphores card with two
// contention-breakdown views: the peak-snapshot list and the
// summed-over-window list. No-op when no snapshot saw >0 waits.
func attachSemaphoreBreakdown(m *innoDBMetricView, snaps []model.SnapshotInnoDB) {
	// 1. Find the peak snapshot (first one tying the max) and sum
	//    sites across the whole window using a single pass.
	type siteKey struct {
		file  string
		line  int
		mutex string
	}
	totalByKey := map[siteKey]int{}
	totalAll := 0
	peakIdx := -1
	peakCount := -1
	for i, si := range snaps {
		if si.Data == nil {
			continue
		}
		if si.Data.SemaphoreCount > peakCount {
			peakCount = si.Data.SemaphoreCount
			peakIdx = i
		}
		for _, s := range si.Data.SemaphoreSites {
			totalByKey[siteKey{s.File, s.Line, s.MutexName}] += s.WaitCount
			totalAll += s.WaitCount
		}
	}
	if peakIdx < 0 || peakCount <= 0 {
		return
	}
	peak := snaps[peakIdx].Data
	peakTotal := peak.SemaphoreCount
	m.BreakdownPeak = buildSiteRows(peak.SemaphoreSites, peakTotal)
	m.BreakdownPeakSnapshot = snaps[peakIdx].SnapshotPrefix
	m.BreakdownPeakTotal = peakTotal

	// 2. Window-total breakdown: flatten the map, sort desc, format.
	if totalAll > 0 {
		rows := make([]model.SemaphoreSite, 0, len(totalByKey))
		for k, c := range totalByKey {
			rows = append(rows, model.SemaphoreSite{
				File:      k.file,
				Line:      k.line,
				MutexName: k.mutex,
				WaitCount: c,
			})
		}
		model.SortSemaphoreSites(rows)
		m.BreakdownTotal = buildSiteRows(rows, totalAll)
		m.BreakdownTotalTotal = totalAll
	}
}

// buildSiteRows materialises every SemaphoreSite into its rendered row
// form. The view caps what is *visible by default* via the template's
// `cb-tail hidden` class (rows with index >= 10); the corresponding
// "Show all (N sites)" button in db.html.tmpl + the initSemaphoreBreakdown
// handler in app.js unhide the remainder. We intentionally do NOT
// truncate here — high-cardinality contention captures still expose
// the full list to the user, just behind one click.
func buildSiteRows(sites []model.SemaphoreSite, total int) []semaphoreSiteRow {
	out := make([]semaphoreSiteRow, 0, len(sites))
	for _, s := range sites {
		pct := 0.0
		if total > 0 {
			pct = float64(s.WaitCount) * 100.0 / float64(total)
		}
		out = append(out, semaphoreSiteRow{
			File:      s.File,
			Line:      s.Line,
			MutexName: s.MutexName,
			Count:     s.WaitCount,
			Percent:   formatFloat(pct, 1),
		})
	}
	return out
}

// attachPendingIOBreakdown enriches the Pending I/O callout with the
// full flushing picture and picks a severity tint:
//
//	sev-crit  — any snapshot saw >0 single-page flushes or >0 pending log fsyncs
//	sev-warn  — any snapshot saw pending writes or pending fsyncs
//	""        — every snapshot was clean
//
// Each pending-* peak is the max observed across snapshots, so a
// transient backlog stays visible even if the capture also covered
// quiescent periods. PeakSnapshot is the snapshot whose combined
// pending-writes total matched the window peak; it anchors the
// "at peak" caption in the expanded breakdown.
func attachPendingIOBreakdown(m *innoDBMetricView, snaps []model.SnapshotInnoDB) {
	var (
		peakReads, peakWrites, peakFsyncs    int
		peakLRU, peakFL, peakSP              int
		peakFsyncLog, peakFsyncBP            int
		peakModified                         int
		peakWritesSnap                       string
		anyFsync, anySingle, anyPendingWrite bool
	)
	peakWritesCombined := -1
	for _, si := range snaps {
		if si.Data == nil {
			continue
		}
		d := si.Data
		if d.PendingReads > peakReads {
			peakReads = d.PendingReads
		}
		writes := d.PendingWrites
		if writes > peakWrites {
			peakWrites = writes
		}
		if writes > peakWritesCombined {
			peakWritesCombined = writes
			peakWritesSnap = si.SnapshotPrefix
		}
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
		if fs := d.PendingFsyncLog + d.PendingFsyncBufferPool; fs > peakFsyncs {
			peakFsyncs = fs
		}
		if d.ModifiedDBPages > peakModified {
			peakModified = d.ModifiedDBPages
		}
		if d.PendingFsyncLog > 0 || d.PendingFsyncBufferPool > 0 {
			anyFsync = true
		}
		if d.PendingWritesSinglePage > 0 {
			anySingle = true
		}
		if writes > 0 || d.PendingReads > 0 {
			anyPendingWrite = true
		}
	}

	m.PendingIO = &pendingIOView{
		PeakReads:           reportutil.HumanIntOrDash(int64(peakReads)),
		PeakWrites:          reportutil.HumanIntOrDash(int64(peakWrites)),
		PeakFsyncs:          reportutil.HumanIntOrDash(int64(peakFsyncs)),
		PeakLRU:             reportutil.HumanIntOrDash(int64(peakLRU)),
		PeakFlushList:       reportutil.HumanIntOrDash(int64(peakFL)),
		PeakSinglePage:      reportutil.HumanIntOrDash(int64(peakSP)),
		PeakFsyncLog:        reportutil.HumanIntOrDash(int64(peakFsyncLog)),
		PeakFsyncBufferPool: reportutil.HumanIntOrDash(int64(peakFsyncBP)),
		PeakModifiedPages:   reportutil.HumanIntOrDash(int64(peakModified)),
		PeakSnapshot:        peakWritesSnap,
		HasLRU:              peakLRU > 0,
		HasFlushList:        peakFL > 0,
		HasSinglePage:       anySingle,
		HasFsyncLog:         peakFsyncLog > 0,
		HasFsyncBP:          peakFsyncBP > 0,
		HasPendingFsync:     anyFsync,
		HasPendingWrite:     anyPendingWrite,
	}

	switch {
	case anySingle || peakFsyncLog > 0:
		m.Severity = "crit"
	case anyFsync || anyPendingWrite:
		m.Severity = "warn"
	}
}

func maxOfInts(vals []int) int {
	m := 0
	for _, v := range vals {
		if v > m {
			m = v
		}
	}
	return m
}

func innoDBIntMetric(label, hint string, vals []int) innoDBMetricView {
	mn, mx, total := vals[0], vals[0], 0
	for _, v := range vals {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
		total += v
	}
	avg := float64(total) / float64(len(vals))
	return innoDBMetricView{
		Label: label,
		Hint:  hint,
		Worst: strconv.Itoa(mx),
		Min:   strconv.Itoa(mn),
		Avg:   formatFloat(avg, 1),
		Max:   strconv.Itoa(mx),
	}
}
