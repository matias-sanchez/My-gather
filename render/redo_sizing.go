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

	// Defensive: rateOK == true with peakRate <= 0 should be
	// impossible when anyWrites is true, because clampNonNegative
	// keeps the sum non-negative and at least one positive delta
	// landed in some window. It can still arise from degenerate
	// timestamps (zero spans) where the rolling-window walk never
	// picks up a positive rate. Treat it as rate_unavailable rather
	// than dividing by zero in the coverage computation below.
	if rateOK && anyWrites && !(peakRate > 0) {
		rateOK = false
	}

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
//
// SkipFirst is true only for a block that begins at the very first
// global timestamp (startIdx == 0). pt-mext stores the cold-start tally
// at index 0 of the first snapshot, which is not a real per-interval
// delta; that tally only ever lives at global index 0. Every other
// block — including the first FINITE block after a leading NaN gap
// (e.g. when Innodb_os_log_written is absent in early snapshots and
// appears later) and every post-boundary block — has its raw tally
// already stripped by model.MergeMysqladminData (it inserts a NaN at
// the boundary and appends src[1:]), so its first finite sample IS a
// genuine per-interval delta and must be included. Tying SkipFirst to
// the global index, not to block ordinal, is the canonical predicate
// (Principle XIII).
type redoSampleBlock struct {
	startIdx  int
	endIdx    int
	SkipFirst bool
}

// hasUsableDelta reports whether a block carries at least one
// per-interval delta after applying the SkipFirst rule. A block with
// zero finite samples is unusable. A SkipFirst block (startIdx == 0)
// with only the cold-start tally is also unusable: the single sample
// at index 0 is the raw tally, not a per-interval delta, and the
// observed-seconds accumulator and the rolling-peak walk both skip it.
// Every other block — including a single-delta post-boundary block —
// has at least one usable delta covering a real interval bounded on
// the left by timestamps[blockLeftBoundIdx(b)]. This is the canonical
// "block carries at least one usable delta" predicate (Principle XIII).
func (b redoSampleBlock) hasUsableDelta() bool {
	n := b.endIdx - b.startIdx
	if n < 1 {
		return false
	}
	if b.SkipFirst && n < 2 {
		return false
	}
	return true
}

// blockLeftBoundIdx returns the timestamp index that anchors the left
// edge of the wall-clock interval covered by a block's included
// deltas. This is the canonical span-derivation helper shared by both
// the average-rate observed-seconds accumulator and the rolling-peak
// fallback span; routing both call sites through it keeps the math
// canonical (Principle XIII).
//
// For the first block (SkipFirst == true) the cold-start tally at
// startIdx is excluded, so the first INCLUDED delta is at startIdx+1
// and covers the interval (timestamps[startIdx], timestamps[startIdx+1]];
// the block's left bound is therefore timestamps[startIdx].
//
// For every post-boundary block (SkipFirst == false) the raw tally was
// already stripped by model.MergeMysqladminData (it appends a NaN
// boundary slot and then src[1:]), so the first INCLUDED delta at
// startIdx covers the interval (timestamps[startIdx-1], timestamps[startIdx]];
// the block's left bound is therefore timestamps[startIdx-1]. By
// construction such a block always has startIdx >= 1 because the NaN
// boundary at startIdx-1 was inserted before src[1:] was appended; the
// startIdx <= 0 clamp below is defensive insurance only.
func blockLeftBoundIdx(b redoSampleBlock) int {
	if b.SkipFirst || b.startIdx <= 0 {
		return b.startIdx
	}
	return b.startIdx - 1
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
	// deltas[i] is finite. Only a block starting at global index 0
	// carries the raw cold-start tally at its start index (pt-mext
	// writes the cold-start counter value there for the very first
	// snapshot, the same convention used by findings.counterTotal);
	// model.MergeMysqladminData strips that raw tally from every
	// snapshot after the first by appending src[1:] after the NaN
	// boundary slot, so a block whose startIdx > 0 — whether it is
	// the first FINITE block after leading NaN padding or any later
	// post-boundary block — has its first finite sample as a genuine
	// per-interval delta that MUST be included in both the average
	// and the rolling-window peak. Tying SkipFirst to startIdx == 0
	// rather than block ordinal is the canonical predicate
	// (Principle XIII).
	var blocks []redoSampleBlock
	startIdx := -1
	for i, d := range deltas {
		if math.IsNaN(d) || math.IsInf(d, 0) {
			if startIdx >= 0 {
				blocks = append(blocks, redoSampleBlock{
					startIdx:  startIdx,
					endIdx:    i,
					SkipFirst: startIdx == 0,
				})
				startIdx = -1
			}
			continue
		}
		if startIdx < 0 {
			startIdx = i
		}
	}
	if startIdx >= 0 {
		blocks = append(blocks, redoSampleBlock{
			startIdx:  startIdx,
			endIdx:    len(deltas),
			SkipFirst: startIdx == 0,
		})
	}

	// Sum totals and accumulate observed seconds across blocks. For a
	// SkipFirst block (one that begins at global index 0) we skip
	// its first index (the cold-start tally) but include its
	// timestamp as the block's start boundary. For every other block
	// — including a single-delta post-boundary block — we include
	// every finite sample because MergeMysqladminData has already
	// stripped the raw tally at the snapshot boundary. Negative
	// deltas are clamped to 0 to absorb counter resets (server
	// restart mid-capture re-zeroes Innodb_os_log_written, producing
	// a negative delta on the next sample); a counter reset is not a
	// write of negative bytes.
	var totalBytes, totalSeconds float64
	var anyData bool
	for _, b := range blocks {
		if !b.hasUsableDelta() {
			continue
		}
		anyData = true
		span := m.Timestamps[b.endIdx-1].Sub(m.Timestamps[blockLeftBoundIdx(b)]).Seconds()
		if span <= 0 {
			continue
		}
		totalSeconds += span
		i0 := b.startIdx
		if b.SkipFirst {
			i0 = b.startIdx + 1
		}
		for i := i0; i < b.endIdx; i++ {
			d := clampNonNegative(deltas[i])
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

// computeRollingPeak returns the peak per-second rate observed over a
// sliding window whose length is at least targetWindow seconds, taken
// over the supplied blocks. When no block reaches targetWindow the
// function collapses the window to the longest block's wall-clock
// duration and reports the rate over that block (a single fixed
// window equal in length to the longest block).
//
// The returned label is "15-minute" when the chosen window equals
// targetWindow == 900s, otherwise "available N-second" where N is
// math.Floor(window) so the label never overstates the observed
// window length (28.6s renders as "28-second", never "29-second").
//
// Negative deltas are clamped to 0 inside the sum to absorb
// Innodb_os_log_written counter resets without depressing the peak.
//
// Per the contract, a candidate window is only considered when its
// actualSpan >= window. This excludes sub-window prefix spans at the
// start of each block, which would otherwise let an early burst be
// reported as the 15-minute peak.
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
		if !b.hasUsableDelta() {
			continue
		}
		span := timestamps[b.endIdx-1].Sub(timestamps[blockLeftBoundIdx(b)]).Seconds()
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
		peakLabel = fmt.Sprintf("available %d-second", int(math.Floor(window)))
	} else {
		peakLabel = "15-minute"
	}

	for _, b := range blocks {
		// We slide a window over the per-sample deltas within this
		// block. SkipFirst is true only for a block that begins at
		// global index 0 (the cold-start tally); every other block —
		// including a single-delta post-boundary block — includes
		// every finite sample because MergeMysqladminData has already
		// stripped the raw tally at the snapshot boundary.
		startI := b.startIdx
		if b.SkipFirst {
			startI = b.startIdx + 1
		}
		endI := b.endIdx
		if endI-startI < 1 {
			continue
		}
		// Each delta covers the interval (timestamps[i-1],
		// timestamps[i]]. The window of samples [sumStart..i]
		// therefore covers (timestamps[sumStart-1], timestamps[i]],
		// so actualSpan = timestamps[i] - timestamps[sumStart-1].
		// We need sumStart >= 1 for this to be well-defined, so the
		// first sample of the very first block (which has no prior
		// timestamp) is consumed as boundary-only and excluded from
		// the rolling sum; this matches the SkipFirst rule above and
		// the per-block startIdx semantics for subsequent blocks
		// (startIdx is the post-NaN index whose previous slot is the
		// NaN boundary, never used as a window edge). Note that the
		// initial sumStart-1 here equals blockLeftBoundIdx(b) by
		// construction (startI is startIdx+1 for SkipFirst blocks and
		// startIdx otherwise); both paths express the same canonical
		// span-derivation invariant.
		sumStart := startI
		sumBytes := 0.0
		for i := startI; i < endI; i++ {
			if i == 0 {
				// Defensive: a block that begins at the global
				// index 0 cannot anchor actualSpan against a
				// preceding timestamp; treat it as boundary-only.
				sumStart = i + 1
				continue
			}
			sumBytes += clampNonNegative(deltas[i])
			// Trim from the left while removing sumStart still
			// leaves actualSpan >= window. This produces the
			// SMALLEST window of length >= window ending at i,
			// which is the correct denominator for the rolling
			// peak: any larger window would only dilute the rate.
			for sumStart < i {
				spanAfterTrim := timestamps[i].Sub(timestamps[sumStart]).Seconds()
				if spanAfterTrim < window {
					break
				}
				sumBytes -= clampNonNegative(deltas[sumStart])
				sumStart++
			}
			actualSpan := timestamps[i].Sub(timestamps[sumStart-1]).Seconds()
			if actualSpan < window {
				// Skip sub-window prefix spans; the contract
				// requires a full window before a rate becomes a
				// peak candidate.
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

// clampNonNegative returns 0 for negative inputs and the value
// otherwise. Used to absorb Innodb_os_log_written counter resets
// (server restart mid-capture re-zeroes the counter, so the next
// delta is a large negative number); a reset is not a write of
// negative bytes, so it must not depress the rolling sum.
func clampNonNegative(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}
