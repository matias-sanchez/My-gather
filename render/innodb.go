package render

import (
	"fmt"

	"github.com/matias-sanchez/My-gather/model"
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
	attachSemaphoreBreakdown(&semMetric, snaps)
	return []innoDBMetricView{
		semMetric,
		innoDBIntMetric("Pending I/O", "reads + writes", pio),
		buildAHIMetric(snaps),
		innoDBIntMetric("History list", "HLL (undo depth)", hll),
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
			AHIHashTableSize: formatThousands(hashTblSize),
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
		AHIHashTableSize: formatThousands(hashTblSize),
		AHIHasHashTable:  hashTblSize > 0,
	}
}

func formatThousands(n int) string {
	if n == 0 {
		return "–"
	}
	s := fmt.Sprintf("%d", n)
	// Insert commas every 3 digits from the right.
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
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
		Worst: fmt.Sprintf("%d", mx),
		Min:   fmt.Sprintf("%d", mn),
		Avg:   formatFloat(avg, 1),
		Max:   fmt.Sprintf("%d", mx),
	}
}
