package parse

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// iostatSnapshotStart is a fixed clock used to anchor every iostat
// test against the same virtual wall-clock — the parser needs a
// non-zero Snapshot start time and its concrete value flows into the
// golden (so leaving it floating would make the golden
// non-deterministic across developers).
func iostatSnapshotStart() time.Time {
	return time.Date(2026, 4, 21, 16, 51, 41, 0, time.UTC)
}

// TestIostatGolden: T043 / FR-009 / Principle VIII.
//
// Parse `testdata/example2/2026_04_21_16_51_41-iostat` and
// snapshot-compare the resulting *IostatData plus any emitted
// diagnostics against a committed golden. The golden captures: the
// full device set, ordered alphabetically; every per-sample
// util_percent / avgqu_sz pair with its parsed Timestamp; and the
// single-file SnapshotBoundaries (which MUST be [0] or empty for a
// lone -iostat input).
func TestIostatGolden(t *testing.T) {
	root := goldens.RepoRoot(t)
	fixture := filepath.Join(root, "testdata", "example2", "2026_04_21_16_51_41-iostat")
	goldenPath := filepath.Join(root, "testdata", "golden", "iostat.example2.2026_04_21_16_51_41.json")

	f, err := os.Open(fixture)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	data, diags := parseIostat(f, iostatSnapshotStart(), fixture)
	if data == nil {
		t.Fatalf("parseIostat returned nil data (diagnostics: %+v)", diags)
	}

	// Invariants from data-model.md (worth surfacing as direct
	// assertions so obvious regressions fail with a clear message
	// rather than a 100-line JSON diff).
	if len(data.Devices) == 0 {
		t.Fatalf("expected at least one DeviceSeries; got zero")
	}
	for i := 1; i < len(data.Devices); i++ {
		if data.Devices[i-1].Device >= data.Devices[i].Device {
			t.Errorf("Devices not strictly sorted alphabetically: %q >= %q at index %d",
				data.Devices[i-1].Device, data.Devices[i].Device, i)
		}
	}
	// Single-file parse: Utilization and AvgQueueSize Samples MUST
	// share the same timestamp set per-device (iostat emits every
	// device at the same intervals inside one file). Compare pairwise
	// — length-equality alone would pass a bug that misaligned the two
	// series by a constant offset.
	for _, dev := range data.Devices {
		if len(dev.Utilization.Samples) != len(dev.AvgQueueSize.Samples) {
			t.Errorf("device %q has mismatched sample counts: util=%d avgqu=%d",
				dev.Device, len(dev.Utilization.Samples), len(dev.AvgQueueSize.Samples))
			continue
		}
		for i := range dev.Utilization.Samples {
			if !dev.Utilization.Samples[i].Timestamp.Equal(dev.AvgQueueSize.Samples[i].Timestamp) {
				t.Errorf("device %q sample %d: util timestamp %s != avgqu timestamp %s",
					dev.Device, i,
					dev.Utilization.Samples[i].Timestamp.Format(time.RFC3339Nano),
					dev.AvgQueueSize.Samples[i].Timestamp.Format(time.RFC3339Nano))
			}
		}
	}
	if len(data.SnapshotBoundaries) > 1 {
		t.Errorf("single-file parse emitted %d boundaries; expected 0 or 1 (got %v)",
			len(data.SnapshotBoundaries), data.SnapshotBoundaries)
	}

	got := goldens.MarshalDeterministic(t, struct {
		Devices            any `json:"devices"`
		SnapshotBoundaries any `json:"snapshotBoundaries"`
		Diagnostics        any `json:"diagnostics"`
	}{
		Devices:            data.Devices,
		SnapshotBoundaries: data.SnapshotBoundaries,
		Diagnostics:        diags,
	})
	goldens.Compare(t, goldenPath, got)
}

// TestIostatPartialRecovery: T047 / FR-008 / Principle III.
//
// Truncate the iostat fixture mid-sample and confirm the parser
// returns the rows it managed to decode plus at least one
// Warning-severity Diagnostic whose Location field is non-empty.
// Constitution Principle III (graceful degradation) makes this the
// canonical partial-recovery scenario — a panic or a nil return
// here would silently drop half a customer's iostat history.
func TestIostatPartialRecovery(t *testing.T) {
	root := goldens.RepoRoot(t)
	fixture := filepath.Join(root, "testdata", "example2", "2026_04_21_16_51_41-iostat")
	full, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if len(full) < 200 {
		t.Fatalf("fixture suspiciously small (%d bytes); cannot meaningfully truncate", len(full))
	}
	// Deterministic mid-line cut: find the line that contains the 50%
	// byte mark and truncate to the middle of that line, so the cut is
	// guaranteed to land strictly between two newlines regardless of
	// fixture size or line distribution. A naive `full[:len(full)/2]`
	// can land exactly on a newline — then the last surviving row is
	// well-formed, no Warning fires, and the test passes for the wrong
	// reason.
	mid := len(full) / 2
	lineStart := bytes.LastIndexByte(full[:mid], '\n') + 1 // index after prev '\n', or 0
	lineEndRel := bytes.IndexByte(full[mid:], '\n')
	if lineEndRel < 0 {
		t.Fatalf("no newline found after byte %d in fixture; cannot compute mid-line cut", mid)
	}
	lineEnd := mid + lineEndRel
	cut := (lineStart + lineEnd) / 2
	if cut <= lineStart || cut >= lineEnd {
		t.Fatalf("computed mid-line cut %d outside (%d, %d) — fixture has a zero-length line at the midpoint", cut, lineStart, lineEnd)
	}
	truncated := full[:cut]
	truncatedSource := fixture + "(truncated)"

	data, diags := parseIostat(bytes.NewReader(truncated), iostatSnapshotStart(), truncatedSource)
	if data == nil {
		t.Fatalf("parseIostat returned nil data on truncated input; Principle III requires graceful recovery with diagnostics")
	}
	if len(data.Devices) == 0 {
		t.Fatalf("parseIostat returned zero devices on 50%%-truncated input; 50%% of the fixture contains 15 complete Device headers, so Devices MUST be non-empty")
	}

	hasWarningWithLocation := false
	for _, d := range diags {
		if d.Severity < model.SeverityWarning {
			continue
		}
		if d.Location == "" {
			t.Errorf("Warning/Error diagnostic with empty Location: %+v (FR-008 requires file+location)", d)
			continue
		}
		// FR-008 requires BOTH file attribution and a location; the
		// Location check alone would let a parser bug that dropped
		// SourceFile slip through (e.g., a refactor that reused a
		// shared diag constructor without threading sourcePath).
		if d.SourceFile != truncatedSource {
			t.Errorf("Warning/Error diagnostic with wrong SourceFile: got %q want %q (FR-008 requires the parser to attribute every diagnostic to the input path it was handed)",
				d.SourceFile, truncatedSource)
			continue
		}
		hasWarningWithLocation = true
	}
	if !hasWarningWithLocation {
		t.Fatalf("expected at least one SeverityWarning diagnostic with SourceFile=%q and a non-empty Location; got %d diagnostics: %+v",
			truncatedSource, len(diags), diags)
	}
}
