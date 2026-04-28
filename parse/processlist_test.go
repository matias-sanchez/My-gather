package parse

import (
	"os"
	"path/filepath"
	"strings"
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

func TestProcesslistRicherMetrics(t *testing.T) {
	input := strings.NewReader(`TS 1769702442.006126802 2026-01-29 16:00:42
*************************** 1. row ***************************
           Id: 12
         User: app
         Host: 10.0.0.1:59136
           db: shop
      Command: Sleep
         Time: 10
        State:
         Info: NULL
      Time_ms: 10050
    Rows_sent: 3
Rows_examined: 100
*************************** 2. row ***************************
           Id: 13
         User: app
         Host: 10.0.0.2:59137
           db: shop
      Command: Query
         Time: 1
        State: Sending data
         Info: select * from orders
    Rows_sent: 5
Rows_examined: not-a-number
*************************** 3. row ***************************
           Id: 15
         User: app
         Host: 10.0.0.4:59139
           db: shop
      Command: Execute
         Time: 99
        State: executing
         Info: NULL
      Time_ms: not-a-number
    Rows_sent: 2
Rows_examined: 50
TS 1769702443.006126802 2026-01-29 16:00:43
*************************** 1. row ***************************
           Id: 14
         User: app
         Host: 10.0.0.3:59138
           db: shop
      Command: Connect
         Time: 7
        State:
         Info:
`)

	data, diags := parseProcesslist(input, "synthetic-processlist")
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if data == nil {
		t.Fatal("parseProcesslist returned nil")
	}
	if got, want := len(data.ThreadStateSamples), 2; got != want {
		t.Fatalf("ThreadStateSamples len = %d, want %d", got, want)
	}

	first := data.ThreadStateSamples[0]
	if got, want := first.TotalThreads, 3; got != want {
		t.Errorf("first.TotalThreads = %d, want %d", got, want)
	}
	if got, want := first.ActiveThreads, 2; got != want {
		t.Errorf("first.ActiveThreads = %d, want %d", got, want)
	}
	if got, want := first.SleepingThreads, 1; got != want {
		t.Errorf("first.SleepingThreads = %d, want %d", got, want)
	}
	if got, want := first.MaxTimeMS, 99000.0; got != want {
		t.Errorf("first.MaxTimeMS = %v, want %v", got, want)
	}
	if got, want := first.MaxRowsExamined, 100.0; got != want {
		t.Errorf("first.MaxRowsExamined = %v, want %v", got, want)
	}
	if got, want := first.MaxRowsSent, 5.0; got != want {
		t.Errorf("first.MaxRowsSent = %v, want %v", got, want)
	}
	if got, want := first.RowsWithQueryText, 1; got != want {
		t.Errorf("first.RowsWithQueryText = %d, want %d", got, want)
	}

	second := data.ThreadStateSamples[1]
	if got, want := second.ActiveThreads, 1; got != want {
		t.Errorf("second.ActiveThreads = %d, want %d", got, want)
	}
	if got, want := second.SleepingThreads, 0; got != want {
		t.Errorf("second.SleepingThreads = %d, want %d", got, want)
	}
	if got, want := second.MaxTimeMS, 7000.0; got != want {
		t.Errorf("second.MaxTimeMS = %v, want %v", got, want)
	}
	if got, want := second.RowsWithQueryText, 0; got != want {
		t.Errorf("second.RowsWithQueryText = %d, want %d", got, want)
	}
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
