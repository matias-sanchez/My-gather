package parse

import (
	"os"
	"path/filepath"
	"testing"

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
