package render

import (
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
		t.Fatalf("ConfiguredText = %q, want 2.0 GiB", v.ConfiguredText)
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
		t.Fatalf("ConfiguredText = %q, want 16.0 GiB", v.ConfiguredText)
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
		t.Fatalf("ConfiguredText = %q, want 2.0 GiB", v.ConfiguredText)
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
