package parse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// TestProcesslistGolden: T068 / FR-017 / Principle VIII.
//
// Parse the committed `-processlist` fixture from testdata/example2/
// and compare the resulting *ProcesslistData against a committed
// golden JSON. The golden captures the full shape — per-sample
// StateCounts, the canonical States ordering, any "Other"-bucketing
// of unknown or empty state labels, and the SnapshotBoundaries list
// the render layer consumes for multi-Snapshot concatenation.
func TestProcesslistGolden(t *testing.T) {
	root := goldens.RepoRoot(t)
	fixture := filepath.Join(root, "testdata", "example2", "2026_04_21_16_51_41-processlist")
	goldenPath := filepath.Join(root, "testdata", "golden", "processlist.example2.2026_04_21_16_51_41.json")

	f, err := os.Open(fixture)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	data, diags := parseProcesslist(f, fixture)
	if data == nil {
		t.Fatalf("parseProcesslist returned nil data (diagnostics: %+v)", diags)
	}

	// Shape invariants called out in data-model.md and the parser's
	// own docstring. Golden-compare below covers drift; these
	// assertions make obvious regressions fail with a direct message.
	if len(data.ThreadStateSamples) == 0 {
		t.Fatalf("expected at least one ThreadStateSample; got zero")
	}
	// States is alphabetical with "Other" last when present — test
	// both halves of the invariant independently so the failure
	// message pinpoints the broken half.
	seenOther := false
	var nonOther []string
	for _, s := range data.States {
		if s == "Other" {
			if seenOther {
				t.Errorf("States contains more than one \"Other\" entry: %v", data.States)
			}
			seenOther = true
			continue
		}
		if seenOther {
			t.Errorf("States: non-\"Other\" entry %q appears after \"Other\" — FR-017 requires \"Other\" to be last", s)
		}
		nonOther = append(nonOther, s)
	}
	for i := 1; i < len(nonOther); i++ {
		if nonOther[i-1] >= nonOther[i] {
			t.Errorf("States (excluding \"Other\") not strictly sorted: %q >= %q at index %d",
				nonOther[i-1], nonOther[i], i)
		}
	}
	// The parser never sets SnapshotBoundaries — that is the render
	// layer's responsibility (concatProcesslist sets it during
	// multi-Snapshot merging). A direct parse call must return nil.
	if len(data.SnapshotBoundaries) != 0 {
		t.Errorf("parseProcesslist must not set SnapshotBoundaries (render layer owns it); got %v",
			data.SnapshotBoundaries)
	}

	got := goldens.MarshalDeterministic(t, struct {
		ThreadStateSamples any `json:"threadStateSamples"`
		States             any `json:"states"`
		SnapshotBoundaries any `json:"snapshotBoundaries"`
		Diagnostics        any `json:"diagnostics"`
	}{
		ThreadStateSamples: data.ThreadStateSamples,
		States:             data.States,
		SnapshotBoundaries: data.SnapshotBoundaries,
		Diagnostics:        diags,
	})
	goldens.Compare(t, goldenPath, got)
}
