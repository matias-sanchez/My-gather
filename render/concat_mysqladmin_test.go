package render

import (
	"math"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// TestConcatMysqladminDivergentVariables exercises the two-pass algorithm
// in concatMysqladmin when two consecutive -mysqladmin snapshots declare
// partially-overlapping variable sets. It locks in four invariants of
// the merge:
//
//  1. Variables present in snap A but absent in snap B are NaN-padded
//     for snap B's span (and vice versa).
//  2. Counter-vs-gauge distinction is respected at the boundary — the
//     first post-boundary slot of a counter is NaN (FR-030); gauges
//     carry their raw values across.
//  3. SnapshotBoundaries indexes into the merged axis and points at
//     each snapshot's first sample.
//  4. Timestamp order is preserved (snap A's timestamps come first,
//     followed by snap B's, unmodified).
func TestConcatMysqladminDivergentVariables(t *testing.T) {
	t0 := time.Date(2026, 4, 21, 16, 51, 41, 0, time.UTC)
	t1 := t0.Add(10 * time.Second)
	t2 := t0.Add(20 * time.Second) // snap B
	t3 := t2.Add(10 * time.Second)

	// Snapshot A: counter "Com_select" + gauge "Threads_running" +
	// counter "Only_in_A" (will vanish in snap B).
	snapA := &model.MysqladminData{
		VariableNames: []string{"Com_select", "Only_in_A", "Threads_running"},
		SampleCount:   2,
		Timestamps:    []time.Time{t0, t1},
		Deltas: map[string][]float64{
			"Com_select":      {100, 5}, // raw tally, then delta
			"Only_in_A":       {50, 3},
			"Threads_running": {4, 5},
		},
		IsCounter: map[string]bool{
			"Com_select":      true,
			"Only_in_A":       true,
			"Threads_running": false,
		},
		SnapshotBoundaries: []int{0},
	}

	// Snapshot B: same counter "Com_select" (boundary-NaN expected) +
	// gauge "Threads_running" (continues) + new counter "Only_in_B".
	snapB := &model.MysqladminData{
		VariableNames: []string{"Com_select", "Only_in_B", "Threads_running"},
		SampleCount:   2,
		Timestamps:    []time.Time{t2, t3},
		Deltas: map[string][]float64{
			"Com_select":      {200, 7},
			"Only_in_B":       {10, 2},
			"Threads_running": {6, 7},
		},
		IsCounter: map[string]bool{
			"Com_select":      true,
			"Only_in_B":       true,
			"Threads_running": false,
		},
		SnapshotBoundaries: []int{0},
	}

	merged := concatMysqladmin([]*model.MysqladminData{snapA, snapB})
	if merged == nil {
		t.Fatal("concatMysqladmin returned nil for two non-empty inputs")
	}

	// 3) SnapshotBoundaries must point at snap A's first sample (0) and
	//    snap B's first sample (2, after two samples from snap A).
	wantBoundaries := []int{0, 2}
	if len(merged.SnapshotBoundaries) != len(wantBoundaries) {
		t.Fatalf("SnapshotBoundaries len = %d, want %d (%v)",
			len(merged.SnapshotBoundaries), len(wantBoundaries), merged.SnapshotBoundaries)
	}
	for i, got := range merged.SnapshotBoundaries {
		if got != wantBoundaries[i] {
			t.Errorf("SnapshotBoundaries[%d] = %d, want %d", i, got, wantBoundaries[i])
		}
	}

	// 4) Timestamp order preserved end-to-end.
	wantTimes := []time.Time{t0, t1, t2, t3}
	if len(merged.Timestamps) != len(wantTimes) {
		t.Fatalf("Timestamps len = %d, want %d", len(merged.Timestamps), len(wantTimes))
	}
	for i, got := range merged.Timestamps {
		if !got.Equal(wantTimes[i]) {
			t.Errorf("Timestamps[%d] = %v, want %v", i, got, wantTimes[i])
		}
	}

	// SampleCount aligned with merged Timestamps length.
	if merged.SampleCount != 4 {
		t.Errorf("SampleCount = %d, want 4", merged.SampleCount)
	}

	// 2a) Counter boundary: Com_select[2] must be NaN (boundary), then
	//     snap B's "delta" slot [3] is 7 (we pass src[1:], i.e. skip
	//     the raw tally slot at index 0 of the second snapshot).
	com := merged.Deltas["Com_select"]
	if len(com) != 4 {
		t.Fatalf("Com_select len = %d, want 4 (%v)", len(com), com)
	}
	if com[0] != 100 || com[1] != 5 {
		t.Errorf("Com_select snap-A slots = [%v %v], want [100 5]", com[0], com[1])
	}
	if !math.IsNaN(com[2]) {
		t.Errorf("Com_select[2] = %v, want NaN (FR-030 boundary)", com[2])
	}
	if com[3] != 7 {
		t.Errorf("Com_select[3] = %v, want 7", com[3])
	}

	// 2b) Gauge carries raw values across the boundary — no NaN injected.
	thr := merged.Deltas["Threads_running"]
	if len(thr) != 4 {
		t.Fatalf("Threads_running len = %d, want 4", len(thr))
	}
	wantThr := []float64{4, 5, 6, 7}
	for i, got := range thr {
		if got != wantThr[i] {
			t.Errorf("Threads_running[%d] = %v, want %v", i, got, wantThr[i])
		}
	}

	// 1) Variables present in only one snapshot are NaN-padded for the
	//    other snapshot's span.
	onlyA := merged.Deltas["Only_in_A"]
	if len(onlyA) != 4 {
		t.Fatalf("Only_in_A len = %d, want 4", len(onlyA))
	}
	if onlyA[0] != 50 || onlyA[1] != 3 {
		t.Errorf("Only_in_A snap-A slots = [%v %v], want [50 3]", onlyA[0], onlyA[1])
	}
	if !math.IsNaN(onlyA[2]) || !math.IsNaN(onlyA[3]) {
		t.Errorf("Only_in_A snap-B slots = [%v %v], want [NaN NaN]", onlyA[2], onlyA[3])
	}

	// Only_in_B is a counter that first appears in snap B. Two paths
	// interact here: the first pass treats its position like any
	// counter-crossing-a-boundary (NaN at the boundary slot, then
	// src[1:]), so snap B produces [NaN, 2] rather than [10, 2]. The
	// raw initial tally 10 is dropped on purpose — this matches the
	// FR-030 boundary semantics used for every counter across snap
	// boundaries. Snap-A's span is NaN-padded (variable was absent).
	onlyB := merged.Deltas["Only_in_B"]
	if len(onlyB) != 4 {
		t.Fatalf("Only_in_B len = %d, want 4", len(onlyB))
	}
	if !math.IsNaN(onlyB[0]) || !math.IsNaN(onlyB[1]) {
		t.Errorf("Only_in_B snap-A slots = [%v %v], want [NaN NaN]", onlyB[0], onlyB[1])
	}
	if !math.IsNaN(onlyB[2]) {
		t.Errorf("Only_in_B[2] = %v, want NaN (counter boundary drops raw tally)", onlyB[2])
	}
	if onlyB[3] != 2 {
		t.Errorf("Only_in_B[3] = %v, want 2", onlyB[3])
	}

	// VariableNames is the sorted union.
	wantNames := []string{"Com_select", "Only_in_A", "Only_in_B", "Threads_running"}
	if len(merged.VariableNames) != len(wantNames) {
		t.Fatalf("VariableNames = %v, want %v", merged.VariableNames, wantNames)
	}
	for i, got := range merged.VariableNames {
		if got != wantNames[i] {
			t.Errorf("VariableNames[%d] = %q, want %q", i, got, wantNames[i])
		}
	}

	// IsCounter: union-of-truths.
	wantCounter := map[string]bool{
		"Com_select":      true,
		"Only_in_A":       true,
		"Only_in_B":       true,
		"Threads_running": false,
	}
	for name, want := range wantCounter {
		if got := merged.IsCounter[name]; got != want {
			t.Errorf("IsCounter[%q] = %v, want %v", name, got, want)
		}
	}
}
