package parse

import (
	"os"
	"path/filepath"
	"strings"
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
	pids := make([]int, len(data.Top3ByAverage))
	for i, ps := range data.Top3ByAverage {
		var sum float64
		for _, s := range ps.CPU.Samples {
			sum += s.Measurements["cpu_percent"]
		}
		avgs[i] = sum / float64(batchCount)
		pids[i] = ps.PID
	}
	// Strictly non-increasing by average on the fixture-derived Top3.
	// (The (higher PID first) tiebreaker contract is exercised against
	// the parser itself in TestTopTiebreakerViaParser below; float
	// averages from the committed fixture rarely compare exactly equal,
	// so a tiebreaker check against this data alone would almost never
	// execute the tie branch.)
	for i := 1; i < len(avgs); i++ {
		if avgs[i-1] < avgs[i] {
			t.Errorf("Top3ByAverage not sorted by average desc: idx %d avg=%.4f < idx %d avg=%.4f",
				i-1, avgs[i-1], i, avgs[i])
		}
		if avgs[i-1] == avgs[i] && pids[i-1] < pids[i] {
			t.Errorf("Top3ByAverage tie at avg=%.4f resolved to lower PID first: idx %d PID=%d < idx %d PID=%d",
				avgs[i], i-1, pids[i-1], i, pids[i])
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

// TestTopTiebreakerViaParser: T044 tiebreaker pin (companion to
// TestTopGolden).
//
// parseTop's ranking contract (data-model.md §TopData): ties in
// sum(CPUPercent) / totalBatchCount resolve to higher PID first.
// TestTopGolden cannot reliably exercise the tie branch because the
// committed fixture rarely produces exact float-equal averages. This
// test drives `parseTop` on an in-memory minimal `top -b` fixture
// whose two processes have identical CPUPercent across every batch,
// so the averages compare exactly equal and the parser's tiebreaker
// is the only thing that picks the order. A regression in
// `parse/top.go` that swaps the comparator (e.g., sorts by PID ascending
// on ties) would fail this test immediately with a specific message.
//
// Feeding literal pre-sorted arrays into the assertion helper — which
// an earlier iteration of TestTopGolden did — only validates the
// helper. Only a parser-driven assertion pins the parser's behaviour,
// which is the invariant the data-model contract actually promises.
func TestTopTiebreakerViaParser(t *testing.T) {
	// Minimal two-batch `top -b` fixture. Both PIDs (100 and 200)
	// appear in both batches with %CPU=50.0, so their averages are
	// exactly equal and the tiebreaker alone decides the Top3 order.
	fixture := strings.Join([]string{
		"top - 00:00:00 up 1 day,  0:00,  0 users,  load average: 0,0,0",
		"Tasks:   0 total",
		"%Cpu(s):  0 us",
		"MiB Mem :   0 total",
		"MiB Swap:   0 total",
		"",
		"    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND",
		"    100 root      20   0       0      0      0 S  50.0   0.0   0:00.00 procA",
		"    200 root      20   0       0      0      0 S  50.0   0.0   0:00.00 procB",
		"",
		"top - 00:00:01 up 1 day,  0:00,  0 users,  load average: 0,0,0",
		"Tasks:   0 total",
		"%Cpu(s):  0 us",
		"MiB Mem :   0 total",
		"MiB Swap:   0 total",
		"",
		"    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND",
		"    100 root      20   0       0      0      0 S  50.0   0.0   0:00.00 procA",
		"    200 root      20   0       0      0      0 S  50.0   0.0   0:00.00 procB",
		"",
	}, "\n")

	data, diags := parseTop(strings.NewReader(fixture), topSnapshotStart(), "synthetic-tie")
	if data == nil {
		t.Fatalf("parseTop returned nil data on tie fixture (diagnostics: %+v)", diags)
	}
	if len(data.Top3ByAverage) < 2 {
		t.Fatalf("Top3ByAverage has %d entries; synthetic fixture defines 2 processes — expected at least 2", len(data.Top3ByAverage))
	}

	// Parser-contract assertion: equal averages → higher PID first.
	// A regression that flipped the tie direction would fail here.
	if data.Top3ByAverage[0].PID < data.Top3ByAverage[1].PID {
		t.Errorf("parseTop tiebreaker regression: Top3ByAverage[0].PID=%d < [1].PID=%d; data-model.md specifies higher PID first on equal averages",
			data.Top3ByAverage[0].PID, data.Top3ByAverage[1].PID)
	}
}
