package render

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// makeRedoReport builds a synthetic *model.Report whose VariablesSection
// contains the supplied variable map and whose DBSection.Mysqladmin
// carries an Innodb_os_log_written counter time-series with the given
// per-sample byte deltas. Each delta covers stepSeconds of wall-clock
// time. To match the pt-mext convention used by the production
// helpers, the first sample of each block holds a "raw initial tally"
// value that the readers ignore; the caller-supplied deltas occupy
// indices 1..len(deltas). Pass nil deltas to omit the counter
// entirely.
func makeRedoReport(t *testing.T, vars map[string]string, deltas []float64, stepSeconds float64) *model.Report {
	t.Helper()
	r := &model.Report{}

	if vars != nil {
		entries := make([]model.VariableEntry, 0, len(vars))
		for k, v := range vars {
			entries = append(entries, model.VariableEntry{Name: k, Value: v})
		}
		r.VariablesSection = &model.VariablesSection{
			PerSnapshot: []model.SnapshotVariables{
				{Data: &model.VariablesData{Entries: entries}},
			},
		}
	}

	r.DBSection = &model.DBSection{}
	if deltas != nil {
		// Sample 0 is the bogus initial tally; sample i (i>=1) holds
		// deltas[i-1]. Timestamps are stepSeconds apart.
		n := len(deltas) + 1
		ts := make([]time.Time, n)
		base := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
		for i := 0; i < n; i++ {
			ts[i] = base.Add(time.Duration(float64(i)*stepSeconds) * time.Second)
		}
		series := make([]float64, n)
		series[0] = 999_999_999 // ignored by readers
		for i, d := range deltas {
			series[i+1] = d
		}
		r.DBSection.Mysqladmin = &model.MysqladminData{
			SampleCount:        n,
			Timestamps:         ts,
			Deltas:             map[string][]float64{"Innodb_os_log_written": series},
			IsCounter:          map[string]bool{"Innodb_os_log_written": true},
			SnapshotBoundaries: []int{0},
			VariableNames:      []string{"Innodb_os_log_written"},
		}
	}
	return r
}

// vars57 returns a synthetic 5.7-style variable map with the given
// per-file size (bytes) and number of files.
func vars57(perFileBytes int64, files int) map[string]string {
	return map[string]string{
		"innodb_log_file_size":      itoa(perFileBytes),
		"innodb_log_files_in_group": itoa(int64(files)),
	}
}

// vars80 returns a synthetic 8.0.30+ variable map setting
// innodb_redo_log_capacity directly. The legacy variables are also
// included to confirm the reader prefers the dynamic capacity.
func vars80(capacityBytes int64) map[string]string {
	return map[string]string{
		"innodb_redo_log_capacity":  itoa(capacityBytes),
		"innodb_log_file_size":      itoa(1 << 30),
		"innodb_log_files_in_group": "2",
	}
}

func itoa(n int64) string {
	// Avoid pulling fmt for one call site; this is plenty for a test.
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// constantDeltas returns n samples each carrying the same byte delta,
// useful for setting up a known peak rate (bytes per stepSeconds).
func constantDeltas(n int, perSample float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = perSample
	}
	return out
}

func TestComputeRedoSizing_57_WellSized(t *testing.T) {
	// 5.7 server, configured 2 GiB. Workload writes 1 MiB per second
	// for 30 samples (30 seconds). Coverage on the available window
	// computes to (2 GiB / 1 MiB/s) / 60 ≈ 34 minutes, well-sized.
	r := makeRedoReport(t, vars57(1<<30, 2), constantDeltas(30, 1<<20), 1)
	v := computeRedoSizing(r)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	if v.State != "ok" {
		t.Fatalf("State = %q, want %q", v.State, "ok")
	}
	if v.ConfigSource != "innodb_log_file_size x innodb_log_files_in_group" {
		t.Fatalf("ConfigSource = %q", v.ConfigSource)
	}
	if v.ConfiguredText != "2.00 GiB" {
		t.Fatalf("ConfiguredText = %q, want 2.00 GiB", v.ConfiguredText)
	}
	if v.UnderSized {
		t.Fatalf("UnderSized = true, want false")
	}
	if v.WarningLine != "" {
		t.Fatalf("WarningLine = %q, want empty", v.WarningLine)
	}
	if v.KBReference != kbReferenceRedoSizing {
		t.Fatalf("KBReference = %q", v.KBReference)
	}
	if v.Recommended15MinText == "n/a" || v.Recommended1HourText == "n/a" {
		t.Fatalf("recommendations should be populated, got %q / %q",
			v.Recommended15MinText, v.Recommended1HourText)
	}
}

func TestComputeRedoSizing_57_UnderSized(t *testing.T) {
	// 5.7 server, configured 256 MiB total (128 MiB x 2). Peak rate
	// 10 MiB/s -> coverage ≈ 0.4 minutes, far below the 15-minute
	// threshold.
	r := makeRedoReport(t, vars57(128<<20, 2), constantDeltas(30, 10<<20), 1)
	v := computeRedoSizing(r)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	if v.State != "ok" {
		t.Fatalf("State = %q", v.State)
	}
	if !v.UnderSized {
		t.Fatalf("UnderSized = false, want true (coverage = %.2f min)",
			v.CoverageMinutes)
	}
	want := "Current redo space holds only "
	if !strings.HasPrefix(v.WarningLine, want) {
		t.Fatalf("WarningLine = %q, want prefix %q", v.WarningLine, want)
	}
	if !strings.Contains(v.WarningLine, "minutes of peak writes - consider raising") {
		t.Fatalf("WarningLine missing canonical suffix: %q", v.WarningLine)
	}
}

func TestComputeRedoSizing_80Plus_WellSized(t *testing.T) {
	// 8.0.30+: innodb_redo_log_capacity set to 16 GiB, with the
	// legacy variables also present (vars80 includes them). The
	// reader must prefer the dynamic capacity and ignore the legacy
	// product.
	r := makeRedoReport(t, vars80(16<<30), constantDeltas(30, 1<<20), 1)
	v := computeRedoSizing(r)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	if v.State != "ok" {
		t.Fatalf("State = %q", v.State)
	}
	if v.ConfigSource != "innodb_redo_log_capacity" {
		t.Fatalf("ConfigSource = %q, want innodb_redo_log_capacity", v.ConfigSource)
	}
	if v.ConfiguredText != "16.00 GiB" {
		t.Fatalf("ConfiguredText = %q, want 16.00 GiB", v.ConfiguredText)
	}
	if v.UnderSized {
		t.Fatalf("UnderSized = true, want false")
	}
}

func TestComputeRedoSizing_80Plus_UnderSized(t *testing.T) {
	// 8.0.30+ with a 512 MiB capacity against a 10 MiB/s workload.
	r := makeRedoReport(t, vars80(512<<20), constantDeltas(30, 10<<20), 1)
	v := computeRedoSizing(r)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	if v.State != "ok" {
		t.Fatalf("State = %q", v.State)
	}
	if v.ConfigSource != "innodb_redo_log_capacity" {
		t.Fatalf("ConfigSource = %q", v.ConfigSource)
	}
	if !v.UnderSized {
		t.Fatalf("UnderSized = false, want true")
	}
	if v.WarningLine == "" {
		t.Fatal("WarningLine should be populated for under-sized 8.0+ case")
	}
}

func TestComputeRedoSizing_ConfigMissing(t *testing.T) {
	// Variables absent; counter present.
	r := makeRedoReport(t, nil, constantDeltas(30, 1<<20), 1)
	v := computeRedoSizing(r)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	if v.State != "config_missing" {
		t.Fatalf("State = %q, want config_missing", v.State)
	}
	if v.ConfiguredText != "unavailable" {
		t.Fatalf("ConfiguredText = %q", v.ConfiguredText)
	}
	if v.CoverageText != "n/a" {
		t.Fatalf("CoverageText = %q", v.CoverageText)
	}
	if v.KBReference != kbReferenceRedoSizing {
		t.Fatalf("KBReference = %q", v.KBReference)
	}
}

func TestComputeRedoSizing_RateUnavailable(t *testing.T) {
	// Variables present; counter absent.
	r := makeRedoReport(t, vars57(1<<30, 2), nil, 1)
	v := computeRedoSizing(r)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	if v.State != "rate_unavailable" {
		t.Fatalf("State = %q, want rate_unavailable", v.State)
	}
	if v.ObservedRateText != "unavailable" {
		t.Fatalf("ObservedRateText = %q", v.ObservedRateText)
	}
	if v.CoverageText != "n/a" {
		t.Fatalf("CoverageText = %q", v.CoverageText)
	}
	if v.Recommended15MinText != "n/a" || v.Recommended1HourText != "n/a" {
		t.Fatalf("recommendations should be n/a, got %q / %q",
			v.Recommended15MinText, v.Recommended1HourText)
	}
	if v.ConfiguredText != "2.00 GiB" {
		t.Fatalf("ConfiguredText = %q, want 2.00 GiB", v.ConfiguredText)
	}
}

func TestComputeRedoSizing_ShortCaptureWindow(t *testing.T) {
	// Capture window is 30s total, far shorter than 15 minutes. The
	// panel should collapse the window and label it accordingly.
	r := makeRedoReport(t, vars57(1<<30, 2), constantDeltas(30, 1<<20), 1)
	v := computeRedoSizing(r)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	if v.PeakWindowSeconds >= 900 {
		t.Fatalf("PeakWindowSeconds = %v, want < 900", v.PeakWindowSeconds)
	}
	if !strings.HasPrefix(v.PeakWindowLabel, "available ") {
		t.Fatalf("PeakWindowLabel = %q, want prefix %q",
			v.PeakWindowLabel, "available ")
	}
	if v.CoverageText == "n/a" || v.CoverageText == "" {
		t.Fatalf("CoverageText = %q, want populated coverage", v.CoverageText)
	}
}

func TestComputeRedoSizing_NoWrites(t *testing.T) {
	// Counter present but every delta is zero (idle workload).
	r := makeRedoReport(t, vars57(1<<30, 2), constantDeltas(30, 0), 1)
	v := computeRedoSizing(r)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	if v.State != "no_writes" {
		t.Fatalf("State = %q, want no_writes", v.State)
	}
	if v.ConfiguredText != "2.00 GiB" {
		t.Fatalf("ConfiguredText = %q", v.ConfiguredText)
	}
	if v.CoverageText != "n/a" {
		t.Fatalf("CoverageText = %q", v.CoverageText)
	}
	if v.Recommended15MinText != "n/a" || v.Recommended1HourText != "n/a" {
		t.Fatalf("recommendations should be n/a, got %q / %q",
			v.Recommended15MinText, v.Recommended1HourText)
	}
	if v.UnderSized {
		t.Fatalf("UnderSized = true on no-writes, want false")
	}
	if v.WarningLine != "" {
		t.Fatalf("WarningLine = %q, want empty", v.WarningLine)
	}
	if v.KBReference != kbReferenceRedoSizing {
		t.Fatalf("KBReference = %q", v.KBReference)
	}
}

func TestComputeRedoSizing_WarningBoundary(t *testing.T) {
	// Set up a constant 1 MiB/s workload. Configured = 900 MiB
	// gives coverage of exactly 15.0 minutes; configured = 899 MiB
	// gives coverage just below 15 minutes.
	const oneMiBperSec = 1 << 20

	rEqual := makeRedoReport(t, vars57(900<<20, 1),
		constantDeltas(60, oneMiBperSec), 1)
	vEqual := computeRedoSizing(rEqual)
	if vEqual == nil {
		t.Fatal("expected non-nil view (equal)")
	}
	if vEqual.State != "ok" {
		t.Fatalf("equal: State = %q", vEqual.State)
	}
	if vEqual.CoverageMinutes != 15 {
		t.Fatalf("equal: CoverageMinutes = %v, want 15", vEqual.CoverageMinutes)
	}
	if vEqual.UnderSized {
		t.Fatalf("equal: UnderSized = true, want false (boundary inclusive at 15)")
	}
	if vEqual.WarningLine != "" {
		t.Fatalf("equal: WarningLine = %q, want empty", vEqual.WarningLine)
	}

	rBelow := makeRedoReport(t, vars57(899<<20, 1),
		constantDeltas(60, oneMiBperSec), 1)
	vBelow := computeRedoSizing(rBelow)
	if vBelow == nil {
		t.Fatal("expected non-nil view (below)")
	}
	if !vBelow.UnderSized {
		t.Fatalf("below: UnderSized = false, want true (coverage = %.4f)",
			vBelow.CoverageMinutes)
	}
}

// TestComputeRedoSizing_FullWindowRequired_BurstyPrefix verifies the
// rolling-peak walk does NOT report a sub-window prefix burst as the
// peak when the capture is long enough to reach a full window.
//
// Setup: 1800 samples at 1s steps (30 minutes total, well beyond the
// 900s target window). The first 60 samples write 100 MiB/s (an early
// 60-second burst); every later sample writes 1 MiB/s. The peak over
// any 900-second window is therefore:
//
//	worst case window straddling the burst:
//	  60 * 100 MiB + 840 * 1 MiB = 6840 MiB  -> 7.6 MiB/s
//
// The naive (pre-fix) implementation would have reported the prefix
// burst rate of 100 MiB/s by accepting sub-window prefix spans as
// peak candidates. The fixed implementation must report ~7.6 MiB/s.
func TestComputeRedoSizing_FullWindowRequired_BurstyPrefix(t *testing.T) {
	const burstSamples = 60
	const burstRate = 100 << 20 // 100 MiB/s
	const tailRate = 1 << 20    // 1 MiB/s
	deltas := make([]float64, 1800)
	for i := range deltas {
		if i < burstSamples {
			deltas[i] = burstRate
		} else {
			deltas[i] = tailRate
		}
	}
	r := makeRedoReport(t, vars57(1<<30, 16), deltas, 1)
	v := computeRedoSizing(r)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	if v.PeakWindowSeconds != 900 {
		t.Fatalf("PeakWindowSeconds = %v, want 900 (full window)", v.PeakWindowSeconds)
	}
	if v.PeakWindowLabel != "15-minute" {
		t.Fatalf("PeakWindowLabel = %q, want %q",
			v.PeakWindowLabel, "15-minute")
	}
	// Worst-case window covers the burst plus 840s of tail rate:
	// (60 * 100 + 840 * 1) MiB / 900s = 6840/900 MiB/s ≈ 7.6 MiB/s.
	wantPeak := float64(burstSamples*burstRate+(900-burstSamples)*tailRate) / 900.0
	if math.Abs(v.PeakRateBytesPerSec-wantPeak) > wantPeak*0.001 {
		t.Fatalf("PeakRateBytesPerSec = %.0f B/s, want ~%.0f B/s; "+
			"the prefix-burst rate of 100 MiB/s must NOT win",
			v.PeakRateBytesPerSec, wantPeak)
	}
}

// TestComputeRedoSizing_PeakRateZeroGuard verifies that a
// degenerate-timestamp case where rateOK could be true with peakRate
// == 0 routes to rate_unavailable rather than dividing by zero in the
// coverage calculation.
//
// Setup: clamp the per-sample timestamps so every interval is zero
// (every sample shares the same wall-clock instant). This makes the
// rolling-window walk unable to anchor a non-zero actualSpan, so
// peakRate stays 0 even though the deltas are positive.
func TestComputeRedoSizing_PeakRateZeroGuard(t *testing.T) {
	r := makeRedoReport(t, vars57(1<<30, 2), constantDeltas(30, 1<<20), 1)
	// Collapse every timestamp to the same instant so the
	// rolling-window walk sees zero spans everywhere.
	if r.DBSection == nil || r.DBSection.Mysqladmin == nil {
		t.Fatal("setup: expected mysqladmin data")
	}
	ts := r.DBSection.Mysqladmin.Timestamps
	if len(ts) == 0 {
		t.Fatal("setup: empty timestamps")
	}
	pinned := ts[0]
	for i := range ts {
		ts[i] = pinned
	}
	v := computeRedoSizing(r)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	if v.State != "rate_unavailable" {
		t.Fatalf("State = %q, want rate_unavailable (peakRate must not divide by zero)",
			v.State)
	}
	if v.CoverageText != "n/a" {
		t.Fatalf("CoverageText = %q, want n/a", v.CoverageText)
	}
	if v.UnderSized {
		t.Fatal("UnderSized must be false when state is not ok")
	}
}

// TestComputeRedoSizing_NegativeDeltaClamp verifies that a negative
// per-sample delta (a counter reset from a server restart mid-capture)
// is clamped to 0 and does NOT depress the rolling sum.
//
// Setup: 60 samples at 1s steps. Most samples write 1 MiB; one sample
// in the middle is -500 MiB (simulated counter reset). The clamped
// peak rate must equal the unclamped 1 MiB/s rate, NOT a depressed
// rate that subtracts the bogus negative.
func TestComputeRedoSizing_NegativeDeltaClamp(t *testing.T) {
	const oneMiB = 1 << 20
	deltas := make([]float64, 60)
	for i := range deltas {
		deltas[i] = oneMiB
	}
	deltas[30] = -500 * oneMiB // simulate a counter reset

	r := makeRedoReport(t, vars57(1<<30, 16), deltas, 1)
	v := computeRedoSizing(r)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	// makeRedoReport prepends a cold-start tally at index 0, so the
	// merged series has 61 samples covering 60 wall-clock seconds.
	// With the clamp the negative delta contributes 0 instead of
	// -500 MiB, giving sum = 59 * 1 MiB over 60s. WITHOUT the clamp
	// the sum would be (59 - 500) MiB ≈ -441 MiB, an absurd
	// negative rate that would route to no_writes/rate_unavailable
	// or report a negative coverage. The clamp keeps the rate
	// strictly positive and close to 1 MiB/s.
	const samples = 60
	const totalSeconds = float64(samples)
	wantRate := float64((samples-1)*oneMiB) / totalSeconds
	if math.Abs(v.PeakRateBytesPerSec-wantRate) > wantRate*0.01 {
		t.Fatalf("PeakRateBytesPerSec = %.0f, want ~%.0f (negative delta must clamp to 0)",
			v.PeakRateBytesPerSec, wantRate)
	}
	if math.Abs(v.ObservedRateBytesPerSec-wantRate) > wantRate*0.01 {
		t.Fatalf("ObservedRateBytesPerSec = %.0f, want ~%.0f (negative delta must clamp to 0)",
			v.ObservedRateBytesPerSec, wantRate)
	}
	if v.State != "ok" {
		t.Fatalf("State = %q, want ok (clamp must keep the rate positive)",
			v.State)
	}
}

// TestComputeRedoSizing_MultiBlock_PostBoundaryFirstSampleIncluded
// verifies that the first finite sample of a post-NaN block is
// included in the rolling-window peak. model.MergeMysqladminData
// strips the raw cold-start tally from snapshots after the first
// (it inserts a NaN at the boundary and appends src[1:]), so the
// first finite sample after the NaN IS a real per-interval delta.
//
// Setup: build a synthetic two-block series by hand (the
// makeRedoReport helper builds only single-block reports). Block A
// has its raw tally at index 0 followed by 60 zero samples. A NaN
// boundary follows. Block B is 60 samples each at 1 MiB/s. A naive
// implementation that skips the first finite sample of every block
// would drop Block B's first 1 MiB sample, yielding 59 MiB / 60s
// ≈ 1006 KiB/s; the correct implementation includes all 60 samples
// for a peak of 1 MiB/s.
func TestComputeRedoSizing_MultiBlock_PostBoundaryFirstSampleIncluded(t *testing.T) {
	const blockA = 60
	const blockB = 60
	const oneMiB = 1 << 20
	r := makeRedoReport(t, vars57(1<<30, 16), nil, 1)
	if r.DBSection == nil {
		t.Fatal("setup: expected DBSection")
	}
	// Total samples: blockA finite + 1 NaN boundary + blockB finite.
	n := blockA + 1 + blockB
	ts := make([]time.Time, n)
	base := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		ts[i] = base.Add(time.Duration(i) * time.Second)
	}
	series := make([]float64, n)
	// Block A: index 0 is the raw cold-start tally (skipped); the
	// remaining samples are 0 (idle).
	series[0] = 999_999_999
	for i := 1; i < blockA; i++ {
		series[i] = 0
	}
	// NaN boundary at index blockA (snapshot break).
	series[blockA] = math.NaN()
	// Block B: every sample is a real 1 MiB/s delta. Note that
	// after the NaN boundary, MergeMysqladminData would have
	// already stripped the raw tally upstream, so series[blockA+1]
	// IS a real delta and MUST be included.
	for i := blockA + 1; i < n; i++ {
		series[i] = oneMiB
	}
	r.DBSection.Mysqladmin = &model.MysqladminData{
		SampleCount:        n,
		Timestamps:         ts,
		Deltas:             map[string][]float64{"Innodb_os_log_written": series},
		IsCounter:          map[string]bool{"Innodb_os_log_written": true},
		SnapshotBoundaries: []int{0, blockA + 1},
		VariableNames:      []string{"Innodb_os_log_written"},
	}

	v := computeRedoSizing(r)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	if v.State != "ok" {
		t.Fatalf("State = %q, want ok", v.State)
	}
	// Block B is the longest block reaching a full peak; both
	// blocks are 60 samples / 60 seconds long, so the window
	// collapses to 60s and the peak is the full Block B average.
	const wantPeak = float64(oneMiB)
	if math.Abs(v.PeakRateBytesPerSec-wantPeak) > wantPeak*0.005 {
		t.Fatalf("PeakRateBytesPerSec = %.0f, want %.0f "+
			"(post-boundary first sample must be included)",
			v.PeakRateBytesPerSec, wantPeak)
	}
}

// TestComputeRedoSizing_MultiBlock_PostBoundarySpanLeftBound is the
// regression test for the post-boundary span-derivation fix. The
// average-rate observed-seconds accumulator and the rolling-peak
// fallback span both anchor a post-boundary block's left edge at
// timestamps[startIdx-1] (the snapshot-boundary timestamp) because
// model.MergeMysqladminData's NaN+src[1:] append makes the included
// delta at startIdx cover (timestamps[startIdx-1], timestamps[startIdx]].
// A naive implementation that anchors at timestamps[startIdx] biases
// both the observed-seconds denominator (under-counted) and the peak
// fallback span (under-counted) downward, biasing both rates upward.
//
// Setup: a synthetic two-block series tuned so the post-boundary
// block dominates and a one-step left-bound shift is a meaningful
// fraction of total observed-seconds. Block A holds only the
// cold-start tally plus one finite delta (1 MiB over 10 s). The NaN
// boundary follows. Block B holds 6 finite deltas of 1 MiB each at
// a 10 s step. With the correct math:
//
//   - Block A span = 10 s, contributes 1 MiB.
//   - Block B span = ts[end-1] - ts[startIdx-1] = 6 * 10 = 60 s,
//     contributes 6 MiB.
//   - ObservedAvg = 7 MiB / 70 s = 104 857.6 B/s.
//   - longest block = 60 s, window collapses to 60 s, label
//     "available 60-second", peak walk over Block B yields
//     6 MiB / 60 s = 104 857.6 B/s.
//
// A naive implementation that anchors Block B at ts[startIdx]
// instead would compute span_B = 50 s, yielding ObservedAvg
// ≈ 122 137 B/s and Peak (window collapsed to 50 s)
// ≈ 104 857.6 B/s but with a "available 50-second" label. The peak
// label and the observed-average value are both discriminating.
func TestComputeRedoSizing_MultiBlock_PostBoundarySpanLeftBound(t *testing.T) {
	const oneMiB = 1 << 20
	const stepSeconds = 10
	r := makeRedoReport(t, vars57(1<<30, 16), nil, 1)
	if r.DBSection == nil {
		t.Fatal("setup: expected DBSection")
	}
	// Total samples: 2 (Block A) + 1 (NaN boundary) + 6 (Block B) = 9.
	const blockA = 2
	const blockB = 6
	n := blockA + 1 + blockB
	ts := make([]time.Time, n)
	base := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		ts[i] = base.Add(time.Duration(i*stepSeconds) * time.Second)
	}
	series := make([]float64, n)
	// Block A: index 0 raw cold-start tally (skipped), index 1 a real
	// 1 MiB delta covering ts[0]..ts[1] = 10 s.
	series[0] = 999_999_999
	series[1] = oneMiB
	// NaN boundary at index 2.
	series[2] = math.NaN()
	// Block B: 6 real 1 MiB deltas at indices 3..8. The first one at
	// index 3 covers (ts[2], ts[3]] per MergeMysqladminData's
	// NaN+src[1:] convention; its left bound is ts[2].
	for i := blockA + 1; i < n; i++ {
		series[i] = oneMiB
	}
	r.DBSection.Mysqladmin = &model.MysqladminData{
		SampleCount:        n,
		Timestamps:         ts,
		Deltas:             map[string][]float64{"Innodb_os_log_written": series},
		IsCounter:          map[string]bool{"Innodb_os_log_written": true},
		SnapshotBoundaries: []int{0, blockA + 1},
		VariableNames:      []string{"Innodb_os_log_written"},
	}

	v := computeRedoSizing(r)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	if v.State != "ok" {
		t.Fatalf("State = %q, want ok", v.State)
	}
	// Block A: 1 MiB over 10 s. Block B: 6 MiB over 60 s. Total
	// 7 MiB over 70 s = 104 857.6 B/s.
	const wantAvg = float64(oneMiB) * 7 / 70
	if math.Abs(v.ObservedRateBytesPerSec-wantAvg) > wantAvg*0.001 {
		t.Fatalf("ObservedRateBytesPerSec = %.4f, want %.4f "+
			"(post-boundary block left bound must be timestamps[startIdx-1])",
			v.ObservedRateBytesPerSec, wantAvg)
	}
	// Longest block is Block B at 60 s; window collapses to 60 s.
	if math.Abs(v.PeakWindowSeconds-60.0) > 1e-9 {
		t.Fatalf("PeakWindowSeconds = %.3f, want 60.000 "+
			"(post-boundary block fallback span must be 60 s)",
			v.PeakWindowSeconds)
	}
	if v.PeakWindowLabel != "available 60-second" {
		t.Fatalf("PeakWindowLabel = %q, want %q",
			v.PeakWindowLabel, "available 60-second")
	}
	// Peak = 6 MiB / 60 s = 104 857.6 B/s.
	const wantPeak = float64(oneMiB) * 6 / 60
	if math.Abs(v.PeakRateBytesPerSec-wantPeak) > wantPeak*0.001 {
		t.Fatalf("PeakRateBytesPerSec = %.4f, want %.4f",
			v.PeakRateBytesPerSec, wantPeak)
	}
}
