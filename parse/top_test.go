package parse

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// topSnapshotStart anchors every top test against a fixed virtual
// wall-clock — parseTop stamps each ProcessSample with a synthesised
// timestamp derived from this value, and the derived values flow into
// the golden. Leaving this floating would make the golden
// non-deterministic across developers.
func topSnapshotStart() time.Time {
	return time.Date(2026, 4, 21, 16, 51, 41, 0, time.UTC)
}

// TestTopGolden: T044 / FR-010 / Principle VIII.
//
// Parse `testdata/example2/2026_04_21_16_51_41-top` and snapshot-
// compare the resulting *TopData plus diagnostics against a committed
// golden. The golden captures ProcessSamples, the Top3ByAverage pivot,
// and the single-file SnapshotBoundaries.
//
// Top3ByAverage is ranked by sum(CPUPercent) / totalBatchCount — that
// is, absent-in-batch counts as 0 in the RANKING denominator (F7
// remediation against FR-010 "aggregate"). Per data-model.md,
// ProcessSeries.CPU.Samples still contains only the observed samples
// (the zero-fill is an averaging convention, not a storage shape).
// Inline invariants pin both halves so a refactor can't silently flip
// either semantic.
func TestTopGolden(t *testing.T) {
	root := goldens.RepoRoot(t)
	fixture := filepath.Join(root, "testdata", "example2", "2026_04_21_16_51_41-top")
	goldenPath := filepath.Join(root, "testdata", "golden", "top.example2.2026_04_21_16_51_41.json")

	f, err := os.Open(fixture)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	data, diags := parseTop(f, topSnapshotStart(), fixture)
	if data == nil {
		t.Fatalf("parseTop returned nil data (diagnostics: %+v)", diags)
	}

	if len(data.ProcessSamples) == 0 {
		t.Fatalf("expected at least one ProcessSample; got zero")
	}
	// ProcessSamples MUST be ordered by (Timestamp asc, PID asc). A
	// swap would silently corrupt the Top3 pivot below; enforce both
	// halves independently so the failure message identifies which
	// one drifted.
	for i := 1; i < len(data.ProcessSamples); i++ {
		prev, cur := data.ProcessSamples[i-1], data.ProcessSamples[i]
		if cur.Timestamp.Before(prev.Timestamp) {
			t.Errorf("ProcessSamples[%d] timestamp %s is before [%d] %s — must be non-decreasing",
				i, cur.Timestamp.Format(time.RFC3339Nano),
				i-1, prev.Timestamp.Format(time.RFC3339Nano))
		}
		if cur.Timestamp.Equal(prev.Timestamp) && cur.PID < prev.PID {
			t.Errorf("ProcessSamples[%d] PID %d < [%d] PID %d at the same timestamp — secondary sort must be PID asc",
				i, cur.PID, i-1, prev.PID)
		}
	}

	// FR-010 "aggregate across the collection window" with the F7
	// remediation: Top3ByAverage ranks by `sum(CPUPercent) /
	// totalBatchCount`, so a PID that appears in only half the
	// batches is penalised for the missing half. The implementation
	// divides by the total batch count (see parse/top.go), matching
	// F7. data-model.md additionally specifies that
	// ProcessSeries.CPU.Samples contains the *observed* samples only
	// (not zero-padded to totalBatchCount) — the zero-fill is an
	// averaging-only convention. Tiebreaker is higher PID first.
	if len(data.Top3ByAverage) > 3 {
		t.Errorf("Top3ByAverage has %d entries; must be at most 3", len(data.Top3ByAverage))
	}
	seen := map[time.Time]struct{}{}
	for _, ps := range data.ProcessSamples {
		seen[ps.Timestamp] = struct{}{}
	}
	batchCount := len(seen)
	if batchCount == 0 {
		t.Fatalf("zero distinct timestamps across ProcessSamples — cannot assert ranking denominator")
	}
	// Per-series Sample count may be < batchCount when a PID was
	// absent from some batches; it must never exceed batchCount
	// (that would indicate duplicate rows at the same timestamp).
	for i, ps := range data.Top3ByAverage {
		if got := len(ps.CPU.Samples); got > batchCount {
			t.Errorf("Top3ByAverage[%d] (PID %d) CPU.Samples has %d entries; must not exceed batchCount=%d",
				i, ps.PID, got, batchCount)
		}
	}
	// Strictly non-increasing by average (ties broken by higher PID
	// first, per data-model.md). Compute each series' average the
	// same way the parser should: sum(cpu_percent) / batchCount,
	// NOT len(Samples). Using len(Samples) as the divisor would
	// contradict F7 and silently re-classify the ranking here.
	avgs := make([]float64, len(data.Top3ByAverage))
	for i, ps := range data.Top3ByAverage {
		var sum float64
		for _, s := range ps.CPU.Samples {
			sum += s.Measurements["cpu_percent"]
		}
		avgs[i] = sum / float64(batchCount)
	}
	for i := 1; i < len(avgs); i++ {
		if avgs[i-1] < avgs[i] {
			t.Errorf("Top3ByAverage not sorted by average desc: idx %d avg=%.4f < idx %d avg=%.4f",
				i-1, avgs[i-1], i, avgs[i])
		}
		if avgs[i-1] == avgs[i] {
			// Tiebreaker: higher PID first (data-model.md).
			if data.Top3ByAverage[i-1].PID < data.Top3ByAverage[i].PID {
				t.Errorf("Top3ByAverage tie at avg=%.4f resolved to lower PID first: idx %d PID=%d < idx %d PID=%d",
					avgs[i], i-1, data.Top3ByAverage[i-1].PID, i, data.Top3ByAverage[i].PID)
			}
		}
	}

	// Single-file parse MUST NOT emit multi-snapshot boundaries. The
	// render layer owns concatenation (same ownership line as iostat
	// and processlist). A singleton [0] is acceptable (0 marks the
	// start of the one and only snapshot) but anything else is a
	// parser-layer regression.
	if len(data.SnapshotBoundaries) > 1 {
		t.Errorf("single-file parse emitted %d boundaries; expected 0 or 1 (got %v)",
			len(data.SnapshotBoundaries), data.SnapshotBoundaries)
	}

	got := goldens.MarshalDeterministic(t, struct {
		ProcessSamples     any `json:"processSamples"`
		Top3ByAverage      any `json:"top3ByAverage"`
		SnapshotBoundaries any `json:"snapshotBoundaries"`
		Diagnostics        any `json:"diagnostics"`
	}{
		ProcessSamples:     data.ProcessSamples,
		Top3ByAverage:      data.Top3ByAverage,
		SnapshotBoundaries: data.SnapshotBoundaries,
		Diagnostics:        diags,
	})
	goldens.Compare(t, goldenPath, got)
}
