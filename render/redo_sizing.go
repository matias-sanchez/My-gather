package render

import (
	"fmt"
	"math"
	"time"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/reportutil"
)

// kbReferenceRedoSizing is the methodology source cited in the
// rendered Redo log sizing panel. The user explicitly requested this
// reference in feature 019-redo-log-sizing-panel; it is held as a
// package-level constant so tests can assert against the same string
// the template will render.
const kbReferenceRedoSizing = "Percona KB0010732"

// redoSizingPeakWindowSeconds is the canonical 15-minute rolling
// window length (in seconds) used by the panel's peak computation.
// When the longest contiguous block of mysqladmin samples is shorter
// than this, computeRedoSizing collapses the window to that block's
// duration and labels the result so the reader knows the peak is
// observed over the available window rather than a true 15-minute
// window.
const redoSizingPeakWindowSeconds = 15.0 * 60.0

// redoSizingView is the payload for the Redo log sizing panel rendered
// under the InnoDB Status subsection. The State field communicates
// which subset of the panel can be populated; the KBReference field
// is always populated regardless of state. See
// specs/019-redo-log-sizing-panel/contracts/redo-sizing-panel.md for
// the full contract.
type redoSizingView struct {
	// State drives which template branches render.
	// Values: "ok" | "config_missing" | "rate_unavailable" | "no_writes".
	State string

	// ConfiguredBytes is the total redo log capacity in bytes. Zero
	// when State == "config_missing".
	ConfiguredBytes float64
	// ConfiguredText is the human-readable form, or "unavailable".
	ConfiguredText string
	// ConfigSource names the variable(s) used:
	//   "innodb_redo_log_capacity"                          (8.0.30+)
	//   "innodb_log_file_size x innodb_log_files_in_group"  (5.6/5.7)
	//   ""                                                  (config_missing)
	ConfigSource string

	// ObservedRateBytesPerSec is the average rate over the entire
	// capture. Zero when State == "rate_unavailable".
	ObservedRateBytesPerSec float64
	ObservedRateText        string
	ObservedRatePerMinText  string

	// PeakWindowSeconds is the actual rolling-window length used.
	// 900 (== 15 minutes) when the longest contiguous block of
	// samples is at least 15 minutes long; otherwise the longest
	// block's wall-clock duration in seconds.
	PeakWindowSeconds float64
	// PeakWindowLabel is the rendered window descriptor used by the
	// template, for example "15-minute" or "available 28-second".
	PeakWindowLabel string
	// PeakRateBytesPerSec is the peak rate observed over the rolling
	// window of length PeakWindowSeconds. Zero when no rate is
	// available or the rate is uniformly zero.
	PeakRateBytesPerSec float64
	PeakRateText        string

	// CoverageMinutes is configured_bytes / peak_rate / 60. Zero when
	// CoverageText == "n/a".
	CoverageMinutes float64
	CoverageText    string

	// Recommended target sizes in bytes plus their human form.
	Recommended15MinBytes float64
	Recommended15MinText  string
	Recommended1HourBytes float64
	Recommended1HourText  string

	// UnderSized is true iff State == "ok" AND CoverageMinutes < 15.
	// The template uses this as the gate for the warning line.
	UnderSized  bool
	WarningLine string

	// KBReference is always populated; the template renders it as
	// the panel's methodology citation.
	KBReference string
}

// computeRedoSizing produces the view payload for the Redo log sizing
// panel under the InnoDB Status section. Returns nil when the report
// has no DBSection (the panel does not render at all in that case).
// In every other case it returns a non-nil view; the view's State
// field communicates which subset of the panel can be populated. The
// KBReference field is always populated.
//
// The configured-space reader is the canonical source for "how big is
// this server's redo log": it tries innodb_redo_log_capacity first
// (the 8.0.30+ model where this single dynamic variable replaces the
// legacy product) and falls back to
// innodb_log_file_size * innodb_log_files_in_group (the 5.6/5.7
// model). The branch is internal to this one function rather than two
// parallel implementations, per Principle XIII.
func computeRedoSizing(r *model.Report) *redoSizingView {
	if r == nil || r.DBSection == nil {
		return nil
	}
	v := &redoSizingView{KBReference: kbReferenceRedoSizing}

	configured, source := readConfiguredRedoSpace(r)
	if configured > 0 {
		v.ConfiguredBytes = configured
		v.ConfiguredText = reportutil.HumanBytes(configured)
		v.ConfigSource = source
	} else {
		v.ConfiguredText = "unavailable"
	}

	avgRate, peakRate, peakWindow, peakLabel, rateOK, anyWrites :=
		readObservedRedoRate(r.DBSection.Mysqladmin)

	switch {
	case !rateOK:
		v.State = "rate_unavailable"
		v.ObservedRateText = "unavailable"
		v.ObservedRatePerMinText = "unavailable"
		v.PeakRateText = "unavailable"
		v.CoverageText = "n/a"
		v.Recommended15MinText = "n/a"
		v.Recommended1HourText = "n/a"
		if configured <= 0 {
			v.State = "config_missing"
		}
		v.PeakWindowSeconds = peakWindow
		v.PeakWindowLabel = peakLabel
		return v
	case !anyWrites:
		v.State = "no_writes"
		v.ObservedRateBytesPerSec = avgRate
		v.ObservedRateText = reportutil.HumanBytes(avgRate) + "/s"
		v.ObservedRatePerMinText = reportutil.HumanBytes(avgRate*60) + "/min"
		v.PeakWindowSeconds = peakWindow
		v.PeakWindowLabel = peakLabel
		v.PeakRateBytesPerSec = 0
		v.PeakRateText = reportutil.HumanBytes(0) + "/s"
		v.CoverageText = "n/a"
		v.Recommended15MinText = "n/a"
		v.Recommended1HourText = "n/a"
		if configured <= 0 {
			v.State = "config_missing"
		}
		return v
	}

	// Rate is real and at least one write was observed.
	v.ObservedRateBytesPerSec = avgRate
	v.ObservedRateText = reportutil.HumanBytes(avgRate) + "/s"
	v.ObservedRatePerMinText = reportutil.HumanBytes(avgRate*60) + "/min"
	v.PeakRateBytesPerSec = peakRate
	v.PeakRateText = reportutil.HumanBytes(peakRate) + "/s"
	v.PeakWindowSeconds = peakWindow
	v.PeakWindowLabel = peakLabel

	if configured <= 0 {
		v.State = "config_missing"
		v.CoverageText = "n/a"
	} else {
		v.State = "ok"
		v.CoverageMinutes = configured / peakRate / 60.0
		coverFloor := int(math.Floor(v.CoverageMinutes))
		v.CoverageText = fmt.Sprintf("%d minutes", coverFloor)
		if v.CoverageMinutes < 15.0 {
			v.UnderSized = true
			v.WarningLine = fmt.Sprintf(
				"Current redo space holds only %d minutes of peak writes - consider raising",
				coverFloor)
		}
	}

	v.Recommended15MinBytes = peakRate * redoSizingPeakWindowSeconds
	v.Recommended15MinText = reportutil.HumanBytes(v.Recommended15MinBytes)
	v.Recommended1HourBytes = peakRate * 3600.0
	v.Recommended1HourText = reportutil.HumanBytes(v.Recommended1HourBytes)

	return v
}

// readConfiguredRedoSpace returns the configured redo log capacity in
// bytes plus a source label describing which variable(s) supplied it.
// On 8.0.30+ instances innodb_redo_log_capacity is the single dynamic
// variable that replaces the legacy product, and MySQL ignores
// innodb_log_file_size + innodb_log_files_in_group when capacity is
// set; this function mirrors that precedence. Returns (0, "") when
// neither model can be evaluated.
func readConfiguredRedoSpace(r *model.Report) (float64, string) {
	if capacity, ok := reportutil.VariableFloat(r, "innodb_redo_log_capacity"); ok && capacity > 0 {
		return capacity, "innodb_redo_log_capacity"
	}
	size, sizeOK := reportutil.VariableFloat(r, "innodb_log_file_size")
	groups, groupsOK := reportutil.VariableFloat(r, "innodb_log_files_in_group")
	if sizeOK && groupsOK && size > 0 && groups > 0 {
		return size * groups, "innodb_log_file_size x innodb_log_files_in_group"
	}
	return 0, ""
}

// redoSampleBlock is one contiguous run of finite Innodb_os_log_written
// delta samples, expressed as a half-open index range [startIdx, endIdx)
// into the parallel deltas + timestamps slices on
// model.MysqladminData. Blocks are separated by NaN slots inserted at
// snapshot boundaries by model.MergeMysqladminData; the rolling-window
// peak walk treats each block independently so a single peak window
// never spans a cross-snapshot gap.
type redoSampleBlock struct {
	startIdx int
	endIdx   int
}

// readObservedRedoRate scans the Innodb_os_log_written counter
// time-series in the captured mysqladmin data and returns:
//
//	avgRate   — bytes/sec averaged over the total observed seconds
//	peakRate  — bytes/sec over the busiest 15-minute rolling window
//	            (or over the longest contiguous block when shorter)
//	peakWindow — actual rolling-window length used, in seconds
//	peakLabel  — "15-minute" when peakWindow == 900,
//	             "available N-second" when shorter
//	rateOK     — false when the counter is absent or insufficient
//	             samples / time exist to compute a rate
//	anyWrites  — true iff at least one non-zero non-NaN delta was seen
//
// NaN slots in the delta slice mark snapshot boundaries inserted by
// model.MergeMysqladminData. They split the rolling-window walk: a
// single window may not span a NaN boundary, matching the existing
// findings.captureSeconds policy of excluding cross-snapshot gaps.
func readObservedRedoRate(m *model.MysqladminData) (
	avgRate, peakRate, peakWindow float64,
	peakLabel string,
	rateOK, anyWrites bool,
) {
	if m == nil {
		return 0, 0, 0, "", false, false
	}
	deltas, ok := m.Deltas["Innodb_os_log_written"]
	if !ok || len(deltas) < 2 {
		return 0, 0, 0, "", false, false
	}
	if !m.IsCounter["Innodb_os_log_written"] {
		// Defensive: should always be a counter; if it is not, we
		// cannot interpret the values as bytes-written deltas.
		return 0, 0, 0, "", false, false
	}
	if len(m.Timestamps) != len(deltas) {
		return 0, 0, 0, "", false, false
	}

	// Build contiguous blocks separated by NaN slots. Each block is a
	// half-open index range [start, end) of indices i such that
	// deltas[i] is finite. We skip index 0 within each block because
	// pt-mext stores the bogus initial tally at the start of every
	// snapshot (the same convention used by findings.counterTotal),
	// which manifests as a non-NaN value at the first index of each
	// block.
	var blocks []redoSampleBlock
	startIdx := -1
	for i, d := range deltas {
		if math.IsNaN(d) || math.IsInf(d, 0) {
			if startIdx >= 0 {
				blocks = append(blocks, redoSampleBlock{startIdx, i})
				startIdx = -1
			}
			continue
		}
		if startIdx < 0 {
			startIdx = i
		}
	}
	if startIdx >= 0 {
		blocks = append(blocks, redoSampleBlock{startIdx, len(deltas)})
	}

	// Sum totals and accumulate observed seconds across blocks. Skip
	// the first index of each block (the bogus initial tally) when
	// summing values, but include its timestamp as the block's start
	// boundary.
	var totalBytes, totalSeconds float64
	var anyData bool
	for _, b := range blocks {
		if b.endIdx-b.startIdx < 2 {
			continue
		}
		anyData = true
		span := m.Timestamps[b.endIdx-1].Sub(m.Timestamps[b.startIdx]).Seconds()
		if span <= 0 {
			continue
		}
		totalSeconds += span
		for i := b.startIdx + 1; i < b.endIdx; i++ {
			d := deltas[i]
			totalBytes += d
			if d > 0 {
				anyWrites = true
			}
		}
	}
	if !anyData || totalSeconds <= 0 {
		return 0, 0, 0, "", false, false
	}
	avgRate = totalBytes / totalSeconds

	// Compute the rolling-window peak. Iterate every block; within
	// each block, slide a window of length redoSizingPeakWindowSeconds
	// across consecutive sample pairs and track the maximum
	// bytes-per-second.
	peakRate, peakWindow, peakLabel = computeRollingPeak(
		blocks, deltas, m.Timestamps, redoSizingPeakWindowSeconds)
	return avgRate, peakRate, peakWindow, peakLabel, true, anyWrites
}

// computeRollingPeak returns the peak per-second rate observed over
// any contiguous window of length at least targetWindow seconds in
// the supplied blocks. If no block is long enough, the function
// collapses the window to the longest block's wall-clock duration and
// reports the rate over that block.
//
// The returned label is "15-minute" when the actual window matches
// targetWindow == 900s, otherwise "available N-second" where N is the
// integer wall-clock seconds of the actual window.
func computeRollingPeak(
	blocks []redoSampleBlock,
	deltas []float64,
	timestamps []time.Time,
	targetWindow float64,
) (peakRate, peakWindow float64, peakLabel string) {
	// Determine the longest available block in seconds first; this
	// drives the window-collapse fallback when no block reaches the
	// target window.
	longest := 0.0
	for _, b := range blocks {
		if b.endIdx-b.startIdx < 2 {
			continue
		}
		span := timestamps[b.endIdx-1].Sub(timestamps[b.startIdx]).Seconds()
		if span > longest {
			longest = span
		}
	}
	if longest <= 0 {
		return 0, 0, ""
	}
	window := targetWindow
	if longest < targetWindow {
		window = longest
		peakLabel = fmt.Sprintf("available %d-second", int(math.Round(longest)))
	} else {
		peakLabel = "15-minute"
	}

	for _, b := range blocks {
		// We slide a window over the per-sample deltas within this
		// block. Skip the first index per the bogus-initial-tally
		// convention used in readObservedRedoRate.
		startI := b.startIdx + 1
		endI := b.endIdx
		if endI-startI < 1 {
			continue
		}
		sumStart := startI
		sumBytes := 0.0
		for i := startI; i < endI; i++ {
			sumBytes += deltas[i]
			// Each delta covers the interval (timestamps[i-1],
			// timestamps[i]]. The window starts at
			// timestamps[sumStart-1]. Trim from the left while the
			// span exceeds the chosen window.
			for sumStart < i {
				spanLeft := timestamps[i].Sub(timestamps[sumStart]).Seconds()
				if spanLeft <= window {
					break
				}
				sumBytes -= deltas[sumStart]
				sumStart++
			}
			actualSpan := timestamps[i].Sub(timestamps[sumStart-1]).Seconds()
			if actualSpan <= 0 {
				continue
			}
			rate := sumBytes / actualSpan
			if rate > peakRate {
				peakRate = rate
			}
		}
	}
	peakWindow = window
	return peakRate, peakWindow, peakLabel
}
