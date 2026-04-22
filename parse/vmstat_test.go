package parse

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// vmstatSnapshotStart anchors vmstat tests against a fixed wall-clock
// (the parser synthesises per-sample timestamps as snapshotStart +
// N·1s, and those values flow into the golden).
func vmstatSnapshotStart() time.Time {
	return time.Date(2026, 4, 21, 16, 51, 41, 0, time.UTC)
}

// vmstatExpectedSeries is the declared series order from spec FR-011
// and the `vmstatColumns` table in parse/vmstat.go. Duplicated here as
// a test-side invariant so a silent reorder of the parser table is
// caught before a reviewer stares at a reordered golden diff.
var vmstatExpectedSeries = []string{
	"runqueue",
	"blocked",
	"free_kb",
	"buff_kb",
	"cache_kb",
	"swap_in",
	"swap_out",
	"io_in",
	"io_out",
	"cpu_user",
	"cpu_sys",
	"cpu_idle",
	"cpu_iowait",
}

// TestVmstatGolden: T045 / FR-011 / Principle VIII.
//
// Parse `testdata/example2/2026_04_21_16_51_41-vmstat` and
// snapshot-compare the resulting *VmstatData plus diagnostics against
// a committed golden. The declared 13-series order MUST be preserved
// across vmstat version differences — missing columns still occupy
// their canonical Metric slot and currently produce a full-length
// zero-valued Samples series for the parsed rows (the parser
// substitutes 0 for an absent column value, so every Series has the
// same length as the number of data rows), not a reordered slice.
func TestVmstatGolden(t *testing.T) {
	root := goldens.RepoRoot(t)
	fixture := filepath.Join(root, "testdata", "example2", "2026_04_21_16_51_41-vmstat")
	goldenPath := filepath.Join(root, "testdata", "golden", "vmstat.example2.2026_04_21_16_51_41.json")

	f, err := os.Open(fixture)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	data, diags := parseVmstat(f, vmstatSnapshotStart(), fixture)
	if data == nil {
		t.Fatalf("parseVmstat returned nil data (diagnostics: %+v)", diags)
	}

	// Series count and order are both part of the FR-011 contract.
	// Assert them as two separate checks so the failure message is
	// specific: a wrong count and a reordered-but-right-count
	// regression have different causes.
	if got, want := len(data.Series), len(vmstatExpectedSeries); got != want {
		t.Fatalf("Series has %d entries; want %d (FR-011 declares a fixed 13-series order)", got, want)
	}
	for i, want := range vmstatExpectedSeries {
		if got := data.Series[i].Metric; got != want {
			t.Errorf("Series[%d].Metric = %q; want %q (FR-011 declared order)", i, got, want)
		}
	}

	// Every Series has the same sample count by construction — the
	// parser appends one value to each of the 13 per-column buffers
	// per data row (missing columns get zero-substituted). Assert
	// that invariant anyway: a future refactor that changes the
	// accumulation to be column-conditional (so a missing column
	// produces a shorter series) would be a visible contract change
	// worth catching here, not silently in a golden diff.
	if len(data.Series[0].Samples) == 0 {
		t.Fatalf("Series[0] (%q) has zero samples — vmstat fixture should contain data rows", data.Series[0].Metric)
	}
	ref := len(data.Series[0].Samples)
	for i := 1; i < len(data.Series); i++ {
		if got := len(data.Series[i].Samples); got != ref {
			t.Errorf("Series[%d] (%q) has %d samples; Series[0] (%q) has %d — every Series must share the row count the parser observes",
				i, data.Series[i].Metric, got,
				data.Series[0].Metric, ref)
		}
	}

	// Single-file parse: SnapshotBoundaries is [0] or empty; render
	// layer owns the multi-Snapshot concatenation boundary markers.
	if len(data.SnapshotBoundaries) > 1 {
		t.Errorf("single-file parse emitted %d boundaries; expected 0 or 1 (got %v)",
			len(data.SnapshotBoundaries), data.SnapshotBoundaries)
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
