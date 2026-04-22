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
	// This is stricter than "[] or [0]" because the parser has no
	// business writing boundaries at all; delegating to the render
	// layer keeps the ownership line sharp. Compare against nil
	// rather than len() so an empty-but-non-nil slice (`make([]int,
	// 0)` or `[]int{}`) also fails — both shapes would indicate a
	// parser that mutated the field it has no business touching.
	if data.SnapshotBoundaries != nil {
		t.Errorf("parseProcesslist must not set SnapshotBoundaries (render layer owns it); got %#v (len=%d)",
			data.SnapshotBoundaries, len(data.SnapshotBoundaries))
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

func TestStripHostPort(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		// IPv4 + port.
		{"10.0.0.1:53412", "10.0.0.1"},
		// IPv4 bare (no port).
		{"10.0.0.1", "10.0.0.1"},
		// Hostname + port / bare.
		{"localhost:3306", "localhost"},
		{"localhost", "localhost"},
		{"host.example.com:3306", "host.example.com"},
		// Bracketed IPv6.
		{"[::1]:53412", "[::1]"},
		{"[2001:db8::5]:1234", "[2001:db8::5]"},
		{"[2001:db8::5]", "[2001:db8::5]"},
		// Unbracketed IPv6 (no port possible — keep intact).
		{"::1", "::1"},
		{"2001:db8::5", "2001:db8::5"},
		{"fe80::1", "fe80::1"},
	}
	for _, tc := range cases {
		if got := stripHostPort(tc.in); got != tc.want {
			t.Errorf("stripHostPort(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
