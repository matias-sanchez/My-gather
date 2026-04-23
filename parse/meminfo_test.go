package parse

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// meminfoExpectedSeries mirrors the declared series order in
// parse/meminfo.go. Duplicated here as a test-side invariant so a
// silent reorder of the parser table is caught before a reviewer
// stares at a reordered golden diff.
var meminfoExpectedSeries = []string{
	"mem_available",
	"mem_free",
	"cached",
	"buffers",
	"anon_pages",
	"dirty",
	"slab",
	"swap_used",
}

// TestMeminfoGolden parses the committed example2 meminfo fixture and
// snapshot-compares the resulting *MeminfoData plus diagnostics
// against a golden. The declared series order MUST be preserved; a
// reordered table breaks the chart axis alignment across snapshots.
func TestMeminfoGolden(t *testing.T) {
	root := goldens.RepoRoot(t)
	fixture := filepath.Join(root, "testdata", "example2", "2026_04_21_16_51_41-meminfo")
	goldenPath := filepath.Join(root, "testdata", "golden", "meminfo.example2.2026_04_21_16_51_41.json")

	f, err := os.Open(fixture)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	data, diags := parseMeminfo(f, fixture)
	if data == nil {
		t.Fatalf("parseMeminfo returned nil data (diagnostics: %+v)", diags)
	}

	if got, want := len(data.Series), len(meminfoExpectedSeries); got != want {
		t.Fatalf("Series has %d entries; want %d", got, want)
	}
	for i, want := range meminfoExpectedSeries {
		if got := data.Series[i].Metric; got != want {
			t.Errorf("Series[%d].Metric = %q; want %q (declared order)", i, got, want)
		}
	}

	if len(data.Series[0].Samples) == 0 {
		t.Fatalf("Series[0] has zero samples — meminfo fixture should contain data rows")
	}
	ref := len(data.Series[0].Samples)
	for i := 1; i < len(data.Series); i++ {
		if got := len(data.Series[i].Samples); got != ref {
			t.Errorf("Series[%d] (%q) has %d samples; Series[0] has %d — every series must share the row count",
				i, data.Series[i].Metric, got, ref)
		}
	}

	// Every emitted value must be in GB (kB ÷ 1,048,576). Sanity-check
	// via MemTotal ~32 GB from the fixture: mem_available must be
	// under that and strictly positive on at least one sample.
	var sawPositive bool
	for _, sp := range data.Series[0].Samples {
		v := sp.Measurements["mem_available"]
		if v > 0 && v < 64 {
			sawPositive = true
			break
		}
	}
	if !sawPositive {
		t.Errorf("mem_available samples never fall into the plausible 0–64 GB range — unit conversion regression?")
	}

	got := goldens.MarshalDeterministic(t, struct {
		Series             any `json:"series"`
		SnapshotBoundaries any `json:"snapshotBoundaries"`
		Diagnostics        any `json:"diagnostics"`
	}{
		Series:             data.Series,
		SnapshotBoundaries: data.SnapshotBoundaries,
		Diagnostics:        diags,
	})
	goldens.Compare(t, goldenPath, got)
}

// TestMeminfoMissingKeyDiagnostic verifies that every declared series
// whose /proc/meminfo key never appears in any sample emits a
// SeverityWarning diagnostic. swap_used is synthesised from
// SwapTotal/SwapFree and MUST NOT be reported as missing even when
// callers happen to drop the intermediate keys from the input.
func TestMeminfoMissingKeyDiagnostic(t *testing.T) {
	// Input intentionally drops Cached, Buffers, AnonPages, Dirty, Slab,
	// and MemAvailable. MemTotal stays to mimic real pt-stalk output,
	// MemFree to exercise the "found" path, and SwapTotal/SwapFree so
	// the synthesised swap_used remains valid.
	input := "TS 1776790303.000000000 2026-04-21 16:51:43\n" +
		"MemTotal:       32000000 kB\n" +
		"MemFree:         5000000 kB\n" +
		"SwapTotal:       4000000 kB\n" +
		"SwapFree:        4000000 kB\n"

	data, diags := parseMeminfo(strings.NewReader(input), "unit-test")
	if data == nil {
		t.Fatalf("parseMeminfo returned nil data")
	}

	var warnings []model.Diagnostic
	for _, d := range diags {
		if d.Severity == model.SeverityWarning {
			warnings = append(warnings, d)
		}
	}
	// Declared keys missing: MemAvailable, Cached, Buffers, AnonPages,
	// Dirty, Slab → 6 warnings. MemFree is present and swap_used is
	// exempt from the never-seen check. Spec originally said 5 but the
	// real declared list has 6 non-synthetic keys beyond MemFree; count
	// only the "never seen" warnings.
	if got := len(warnings); got != 6 {
		t.Fatalf("got %d SeverityWarning diagnostics; want 6 (one per missing declared key except swap_used)", got)
	}
	for _, d := range warnings {
		if strings.Contains(d.Message, "swap_used") {
			t.Errorf("swap_used must never be reported missing — it is synthesised; got diag: %q", d.Message)
		}
	}
}

// TestMeminfoRejectsPreTSData covers the canonical contract: value
// lines before any TS marker are non-canonical input and the parser
// returns (nil, ErrorDiag) rather than silently discarding them. An
// empty reader hits the same rejection path (no samples found).
func TestMeminfoRejectsPreTSData(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty input",
			input: "",
			want:  "no samples found",
		},
		{
			name:  "value line before first TS",
			input: "MemTotal: 32000000 kB\nTS 1776790303.000000000 2026-04-21 16:51:43\n",
			want:  "value line before first TS marker",
		},
		{
			name: "missing SwapTotal/SwapFree",
			// /proc/meminfo always emits both (zero on systems without
			// swap); absence signals a truncated capture and must be
			// rejected instead of synthesising swap_used=0 silently.
			input: "TS 1776790303.000000000 2026-04-21 16:51:43\n" +
				"MemTotal:       32000000 kB\n" +
				"MemFree:         5000000 kB\n",
			want: "missing SwapTotal and/or SwapFree",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, diags := parseMeminfo(strings.NewReader(tc.input), "unit-test")
			if data != nil {
				t.Errorf("parseMeminfo returned non-nil data (%+v); want nil", data)
			}
			var found bool
			for _, d := range diags {
				if d.Severity == model.SeverityError && strings.Contains(d.Message, tc.want) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("want SeverityError diagnostic containing %q; got %+v", tc.want, diags)
			}
		})
	}
}

// TestMeminfoSwapUsedSynthesis verifies that swap_used is derived as
// SwapTotal - SwapFree and clamps to zero when SwapFree > SwapTotal
// (pathological /proc/meminfo snapshot).
func TestMeminfoSwapUsedSynthesis(t *testing.T) {
	// 33554428 kB - 31457280 kB = 2097148 kB ≈ 2 GiB. Converted to GB
	// by the parser: 2097148 / (1024*1024) ≈ 1.9999... GB.
	input := "TS 1776790303.000000000 2026-04-21 16:51:43\n" +
		"MemTotal:       32000000 kB\n" +
		"MemFree:         5000000 kB\n" +
		"SwapTotal:      33554428 kB\n" +
		"SwapFree:       31457280 kB\n"

	data, _ := parseMeminfo(strings.NewReader(input), "unit-test")
	if data == nil {
		t.Fatalf("parseMeminfo returned nil data")
	}

	var swapSeries *model.MetricSeries
	for i := range data.Series {
		if data.Series[i].Metric == "swap_used" {
			swapSeries = &data.Series[i]
			break
		}
	}
	if swapSeries == nil {
		t.Fatalf("swap_used series not found")
	}
	if len(swapSeries.Samples) != 1 {
		t.Fatalf("swap_used has %d samples; want 1", len(swapSeries.Samples))
	}
	got := swapSeries.Samples[0].Measurements["swap_used"]
	want := float64(33554428-31457280) / (1024.0 * 1024.0) // GB
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("swap_used = %v GB; want %v GB (~2 GiB)", got, want)
	}

	// Pathological: SwapFree > SwapTotal → clamp to 0.
	badInput := "TS 1776790303.000000000 2026-04-21 16:51:43\n" +
		"MemTotal:       32000000 kB\n" +
		"MemFree:         5000000 kB\n" +
		"SwapTotal:        100000 kB\n" +
		"SwapFree:         500000 kB\n"
	data2, _ := parseMeminfo(strings.NewReader(badInput), "unit-test")
	if data2 == nil {
		t.Fatalf("parseMeminfo returned nil data for clamp case")
	}
	for i := range data2.Series {
		if data2.Series[i].Metric != "swap_used" {
			continue
		}
		if len(data2.Series[i].Samples) != 1 {
			t.Fatalf("swap_used clamp case: %d samples; want 1", len(data2.Series[i].Samples))
		}
		if v := data2.Series[i].Samples[0].Measurements["swap_used"]; v != 0 {
			t.Errorf("swap_used under SwapFree>SwapTotal = %v; want 0 (clamped)", v)
		}
	}
}
