package parse

import (
	"bytes"
	"io"
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
	// device at the same intervals inside one file).
	for _, dev := range data.Devices {
		if len(dev.Utilization.Samples) != len(dev.AvgQueueSize.Samples) {
			t.Errorf("device %q has mismatched sample counts: util=%d avgqu=%d",
				dev.Device, len(dev.Utilization.Samples), len(dev.AvgQueueSize.Samples))
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
	// Truncate at 50% so we cut cleanly inside a device row mid-way
	// through the file — that row becomes malformed and the parser
	// MUST emit a Warning, not give up.
	truncated := full[:len(full)/2]

	data, diags := parseIostat(bytes.NewReader(truncated), iostatSnapshotStart(), fixture+"(truncated)")
	if data == nil {
		t.Fatalf("parseIostat returned nil data on truncated input; Principle III requires graceful recovery with diagnostics")
	}
	if len(data.Devices) == 0 {
		// Depending on where the 50% cut lands we may have zero
		// *complete* samples; we still expect at least one device in
		// the device list if the header survived. Tolerate the edge
		// where even the first header got cut — in that case the
		// parser's job is to emit a diagnostic and return empty
		// data, which the banner path covers in the renderer.
		t.Logf("truncated input produced zero devices (cut before the first header completed); relying on diagnostic assertion below")
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
		hasWarningWithLocation = true
	}
	if !hasWarningWithLocation {
		t.Fatalf("expected at least one SeverityWarning diagnostic with a non-empty Location; got %d diagnostics: %+v",
			len(diags), diags)
	}
}

// compile-time assurance that parseIostat still has the signature the
// tests above rely on; cheap insurance against an accidental rename.
var _ = func() (*model.IostatData, []model.Diagnostic) {
	return parseIostat(io.Reader(bytes.NewReader(nil)), time.Time{}, "")
}
