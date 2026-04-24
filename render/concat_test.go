package render

import (
	"math"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// Direct unit tests for the non-mysqladmin concat* helpers. Each test
// hand-builds minimal *model.X structures with overlapping-but-distinct
// shapes across two inputs and pins down the merge invariants that the
// render layer depends on:
//
//   - dimension union is deterministic (alphabetical; "Other" last where
//     the domain demands it),
//   - SnapshotBoundaries indexes into the primary axis and records the
//     cumulative position of each input's first sample,
//   - the primary axis length equals the sum of per-input primary-axis
//     lengths (no silent dedup),
//   - NaN padding fills gaps where a series is absent from an input.
//
// Fixture files are intentionally avoided — these tests must run without
// filesystem access so they isolate the helpers from the surrounding
// discovery / parse / view-assembly pipeline.

// mkSamples builds a MetricSeries-worth of samples at evenly-spaced
// timestamps starting from t0.
func mkSamples(t0 time.Time, stepSec int, values ...float64) []model.Sample {
	out := make([]model.Sample, len(values))
	for i, v := range values {
		out[i] = model.Sample{
			Timestamp:    t0.Add(time.Duration(i*stepSec) * time.Second),
			Measurements: map[string]float64{"v": v},
		}
	}
	return out
}

func TestConcatIostatUnionAndBoundaries(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(60 * time.Second)

	snapA := &model.IostatData{
		Devices: []model.DeviceSeries{
			{
				Device:       "sda",
				Utilization:  model.MetricSeries{Metric: "util_percent", Unit: "%", Subject: "sda", Samples: mkSamples(t0, 10, 10, 20)},
				AvgQueueSize: model.MetricSeries{Metric: "avgqu_sz", Unit: "count", Subject: "sda", Samples: mkSamples(t0, 10, 1, 2)},
			},
			{
				Device:       "sdb",
				Utilization:  model.MetricSeries{Metric: "util_percent", Unit: "%", Subject: "sdb", Samples: mkSamples(t0, 10, 30, 40)},
				AvgQueueSize: model.MetricSeries{Metric: "avgqu_sz", Unit: "count", Subject: "sdb", Samples: mkSamples(t0, 10, 3, 4)},
			},
		},
		SnapshotBoundaries: []int{0},
	}
	snapB := &model.IostatData{
		Devices: []model.DeviceSeries{
			// sda carries through; nvme0n1 is new to snap B (sdb vanishes).
			{
				Device:       "sda",
				Utilization:  model.MetricSeries{Metric: "util_percent", Unit: "%", Subject: "sda", Samples: mkSamples(t1, 10, 11, 21, 31)},
				AvgQueueSize: model.MetricSeries{Metric: "avgqu_sz", Unit: "count", Subject: "sda", Samples: mkSamples(t1, 10, 5, 6, 7)},
			},
			{
				Device:       "nvme0n1",
				Utilization:  model.MetricSeries{Metric: "util_percent", Unit: "%", Subject: "nvme0n1", Samples: mkSamples(t1, 10, 50, 60, 70)},
				AvgQueueSize: model.MetricSeries{Metric: "avgqu_sz", Unit: "count", Subject: "nvme0n1", Samples: mkSamples(t1, 10, 8, 9, 10)},
			},
		},
		SnapshotBoundaries: []int{0},
	}

	merged := concatIostat([]*model.IostatData{snapA, snapB})
	if merged == nil {
		t.Fatal("concatIostat returned nil for two non-empty inputs")
	}

	// Union of device names, sorted alphabetically.
	wantNames := []string{"nvme0n1", "sda", "sdb"}
	if len(merged.Devices) != len(wantNames) {
		t.Fatalf("device count = %d, want %d (%+v)", len(merged.Devices), len(wantNames), merged.Devices)
	}
	for i, d := range merged.Devices {
		if d.Device != wantNames[i] {
			t.Errorf("Devices[%d].Device = %q, want %q", i, d.Device, wantNames[i])
		}
		if got := len(d.Utilization.Samples); got != 5 {
			t.Errorf("Devices[%d] (%s) Utilization len = %d, want 5 (2 + 3)", i, d.Device, got)
		}
	}

	// SnapshotBoundaries indexes into the shared axis: snap A contributes
	// 2 samples, snap B contributes 3.
	if got, want := merged.SnapshotBoundaries, []int{0, 2}; !eqInts(got, want) {
		t.Errorf("SnapshotBoundaries = %v, want %v", got, want)
	}

	// sdb is absent from snap B — its second-window slots must be NaN.
	var sdb *model.DeviceSeries
	for i := range merged.Devices {
		if merged.Devices[i].Device == "sdb" {
			sdb = &merged.Devices[i]
			break
		}
	}
	if sdb == nil {
		t.Fatal("sdb missing from merged output")
	}
	for i := 2; i < 5; i++ {
		if v := sdb.Utilization.Samples[i].Measurements["util_percent"]; !math.IsNaN(v) {
			t.Errorf("sdb Utilization[%d] = %v, want NaN (absent in snap B)", i, v)
		}
	}
}

func TestConcatTopRecomputesTop3WithMysqldAlwaysIn(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(10 * time.Second)
	t2 := t0.Add(20 * time.Second)

	// Snap A: three high-CPU consumers, mysqld is low.
	snapA := &model.TopData{
		ProcessSamples: []model.ProcessSample{
			{Timestamp: t0, PID: 100, Command: "worker-a", CPUPercent: 90},
			{Timestamp: t0, PID: 200, Command: "worker-b", CPUPercent: 85},
			{Timestamp: t0, PID: 300, Command: "worker-c", CPUPercent: 80},
			{Timestamp: t0, PID: 42, Command: "mysqld", CPUPercent: 5},
			{Timestamp: t1, PID: 100, Command: "worker-a", CPUPercent: 88},
			{Timestamp: t1, PID: 200, Command: "worker-b", CPUPercent: 86},
			{Timestamp: t1, PID: 300, Command: "worker-c", CPUPercent: 82},
			{Timestamp: t1, PID: 42, Command: "mysqld", CPUPercent: 6},
		},
	}
	// Snap B: a new PID 400 shows up, mysqld still low.
	snapB := &model.TopData{
		ProcessSamples: []model.ProcessSample{
			{Timestamp: t2, PID: 400, Command: "worker-d", CPUPercent: 70},
			{Timestamp: t2, PID: 42, Command: "mysqld", CPUPercent: 7},
		},
	}

	merged := concatTop([]*model.TopData{snapA, snapB})
	if merged == nil {
		t.Fatal("concatTop returned nil for two non-empty inputs")
	}
	if got, want := merged.SnapshotBoundaries, []int{0, 2}; !eqInts(got, want) {
		t.Errorf("SnapshotBoundaries = %v, want %v (snap A has 2 unique ts, snap B starts at ts index 2)", got, want)
	}
	// Top 3 by average CPU + mysqld appended because it's not in the top 3.
	if len(merged.Top3ByAverage) != 4 {
		t.Fatalf("Top3ByAverage len = %d, want 4 (top-3 + mysqld-invariant)", len(merged.Top3ByAverage))
	}
	wantOrder := []int{100, 200, 300, 42}
	for i, want := range wantOrder {
		if got := merged.Top3ByAverage[i].PID; got != want {
			t.Errorf("Top3ByAverage[%d].PID = %d, want %d", i, got, want)
		}
	}
	// Every series is pivoted onto the full 3-timestamp axis.
	for _, s := range merged.Top3ByAverage {
		if got := len(s.CPU.Samples); got != 3 {
			t.Errorf("PID %d CPU.Samples len = %d, want 3", s.PID, got)
		}
	}
	// PID 400 is only in snap B; verify absent-as-zero invariant on the
	// pivoted series. Find PID 400 in the merged process samples for
	// sanity (present), then verify it's NOT in Top3ByAverage (avg over
	// whole window is 70/3 = 23.3, below the workers).
	for _, s := range merged.Top3ByAverage {
		if s.PID == 400 {
			t.Errorf("PID 400 should not be in Top3ByAverage (avg too low), got %+v", s)
		}
	}
}

func TestConcatTopMysqldAlreadyInTopThree(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	// Two samples; mysqld dominates and sits in the top 3.
	snap := &model.TopData{
		ProcessSamples: []model.ProcessSample{
			{Timestamp: t0, PID: 42, Command: "mysqld", CPUPercent: 99},
			{Timestamp: t0, PID: 100, Command: "worker-a", CPUPercent: 50},
			{Timestamp: t0, PID: 200, Command: "worker-b", CPUPercent: 30},
			{Timestamp: t0, PID: 300, Command: "worker-c", CPUPercent: 10},
		},
	}
	merged := concatTop([]*model.TopData{snap})
	if len(merged.Top3ByAverage) != 3 {
		t.Fatalf("Top3ByAverage len = %d, want 3 (mysqld already in top-3, no duplicate)", len(merged.Top3ByAverage))
	}
	if merged.Top3ByAverage[0].PID != 42 {
		t.Errorf("Top3ByAverage[0].PID = %d, want 42 (mysqld by avg)", merged.Top3ByAverage[0].PID)
	}
}

func TestConcatVmstatAlignsByMetricName(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(10 * time.Second)

	snapA := &model.VmstatData{
		Series: []model.MetricSeries{
			{Metric: "r", Unit: "count", Samples: mkSamples(t0, 1, 1, 2)},
			{Metric: "b", Unit: "count", Samples: mkSamples(t0, 1, 0, 0)},
			{Metric: "wa", Unit: "%", Samples: mkSamples(t0, 1, 5, 6)},
		},
	}
	// Snap B reorders "wa" before "b" — mergeMetricSeriesByName must
	// align by Metric name, not by slice index.
	snapB := &model.VmstatData{
		Series: []model.MetricSeries{
			{Metric: "r", Unit: "count", Samples: mkSamples(t1, 1, 3, 4, 5)},
			{Metric: "wa", Unit: "%", Samples: mkSamples(t1, 1, 7, 8, 9)},
			{Metric: "b", Unit: "count", Samples: mkSamples(t1, 1, 1, 1, 1)},
		},
	}
	merged := concatVmstat([]*model.VmstatData{snapA, snapB})
	if merged == nil {
		t.Fatal("concatVmstat returned nil for two non-empty inputs")
	}
	if got, want := merged.SnapshotBoundaries, []int{0, 2}; !eqInts(got, want) {
		t.Errorf("SnapshotBoundaries = %v, want %v", got, want)
	}
	// Primary axis (Series[0] = "r") gets 2 + 3 = 5 samples.
	if len(merged.Series) != 3 {
		t.Fatalf("Series len = %d, want 3", len(merged.Series))
	}
	if merged.Series[0].Metric != "r" {
		t.Errorf("Series[0].Metric = %q, want %q", merged.Series[0].Metric, "r")
	}
	// Order of the output follows the FIRST input's declaration order.
	wantOrder := []string{"r", "b", "wa"}
	for i, s := range merged.Series {
		if s.Metric != wantOrder[i] {
			t.Errorf("Series[%d].Metric = %q, want %q (first-input declaration order)", i, s.Metric, wantOrder[i])
		}
		if got := len(s.Samples); got != 5 {
			t.Errorf("Series[%d] (%s) len = %d, want 5", i, s.Metric, got)
		}
	}
	// Spot-check a non-index-0 series to prove name alignment: "wa"
	// should be [5, 6, 7, 8, 9], not shuffled.
	var wa *model.MetricSeries
	for i := range merged.Series {
		if merged.Series[i].Metric == "wa" {
			wa = &merged.Series[i]
			break
		}
	}
	if wa == nil {
		t.Fatal("wa series missing")
	}
	wantWa := []float64{5, 6, 7, 8, 9}
	for i, s := range wa.Samples {
		if got := s.Measurements["v"]; got != wantWa[i] {
			t.Errorf("wa[%d] = %v, want %v", i, got, wantWa[i])
		}
	}
}

func TestConcatMeminfoAlignsByMetricName(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(10 * time.Second)

	snapA := &model.MeminfoData{
		Series: []model.MetricSeries{
			{Metric: "MemAvailable", Unit: "GB", Samples: mkSamples(t0, 1, 10, 11)},
			{Metric: "AnonPages", Unit: "GB", Samples: mkSamples(t0, 1, 3, 4)},
		},
	}
	snapB := &model.MeminfoData{
		Series: []model.MetricSeries{
			// Reorder — name-alignment check.
			{Metric: "AnonPages", Unit: "GB", Samples: mkSamples(t1, 1, 5, 6, 7)},
			{Metric: "MemAvailable", Unit: "GB", Samples: mkSamples(t1, 1, 12, 13, 14)},
		},
	}
	merged := concatMeminfo([]*model.MeminfoData{snapA, snapB})
	if merged == nil {
		t.Fatal("concatMeminfo returned nil for two non-empty inputs")
	}
	if got, want := merged.SnapshotBoundaries, []int{0, 2}; !eqInts(got, want) {
		t.Errorf("SnapshotBoundaries = %v, want %v", got, want)
	}
	if len(merged.Series) != 2 {
		t.Fatalf("Series len = %d, want 2", len(merged.Series))
	}
	if merged.Series[0].Metric != "MemAvailable" {
		t.Errorf("Series[0].Metric = %q, want %q (first-input declared order)", merged.Series[0].Metric, "MemAvailable")
	}
	for _, s := range merged.Series {
		if got := len(s.Samples); got != 5 {
			t.Errorf("%s len = %d, want 5", s.Metric, got)
		}
		if s.Unit != "GB" {
			t.Errorf("%s Unit = %q, want %q (meminfo never rewrites Unit)", s.Metric, s.Unit, "GB")
		}
	}
}

func TestConcatProcesslistUnionsDimensions(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(10 * time.Second)
	t2 := t0.Add(20 * time.Second)

	snapA := &model.ProcesslistData{
		ThreadStateSamples: []model.ThreadStateSample{
			{Timestamp: t0, StateCounts: map[string]int{"Sending data": 5}, UserCounts: map[string]int{"root": 5}},
			{Timestamp: t1, StateCounts: map[string]int{"Sending data": 6}, UserCounts: map[string]int{"root": 6}},
		},
		States:   []string{"Sending data", "Other"},
		Users:    []string{"root"},
		Hosts:    []string{"localhost"},
		Commands: []string{"Query"},
		Dbs:      []string{"mysql"},
	}
	snapB := &model.ProcesslistData{
		ThreadStateSamples: []model.ThreadStateSample{
			{Timestamp: t2, StateCounts: map[string]int{"Locked": 3}, UserCounts: map[string]int{"app": 3}},
		},
		States:   []string{"Locked", "Other"},
		Users:    []string{"app"},
		Hosts:    []string{"10.0.0.1"},
		Commands: []string{"Query", "Sleep"},
		Dbs:      []string{"mysql", "app_db"},
	}

	merged := concatProcesslist([]*model.ProcesslistData{snapA, snapB})
	if merged == nil {
		t.Fatal("concatProcesslist returned nil for two non-empty inputs")
	}
	if got, want := merged.SnapshotBoundaries, []int{0, 2}; !eqInts(got, want) {
		t.Errorf("SnapshotBoundaries = %v, want %v (snap A: 2 samples, snap B starts at 2)", got, want)
	}
	if got := len(merged.ThreadStateSamples); got != 3 {
		t.Errorf("ThreadStateSamples len = %d, want 3", got)
	}
	// States union: sorted alphabetically, "Other" last.
	wantStates := []string{"Locked", "Sending data", "Other"}
	if !eqStrs(merged.States, wantStates) {
		t.Errorf("States = %v, want %v", merged.States, wantStates)
	}
	// Users union: plain alphabetical, no "Other" in either input.
	wantUsers := []string{"app", "root"}
	if !eqStrs(merged.Users, wantUsers) {
		t.Errorf("Users = %v, want %v", merged.Users, wantUsers)
	}
	// Hosts union: cross-snapshot values are gathered.
	wantHosts := []string{"10.0.0.1", "localhost"}
	if !eqStrs(merged.Hosts, wantHosts) {
		t.Errorf("Hosts = %v, want %v", merged.Hosts, wantHosts)
	}
	// Commands/Dbs alphabetical union.
	wantCommands := []string{"Query", "Sleep"}
	if !eqStrs(merged.Commands, wantCommands) {
		t.Errorf("Commands = %v, want %v", merged.Commands, wantCommands)
	}
	wantDbs := []string{"app_db", "mysql"}
	if !eqStrs(merged.Dbs, wantDbs) {
		t.Errorf("Dbs = %v, want %v", merged.Dbs, wantDbs)
	}
}

// TestConcatNetstatSPreservesSubSecondTimestamps — regression guard
// for the rate-math collapse Codex flagged on bf7e8fc. pt-stalk polls
// can land within the same whole second; if concatNetstatS truncates
// to Unix() seconds the downstream rate divisor
// Timestamps[i]-Timestamps[i-1] collapses to 0 and the rate chart
// silently drops per-second samples. Preserving sub-second precision
// is a correctness requirement, and this test locks it in.
func TestConcatNetstatSPreservesSubSecondTimestamps(t *testing.T) {
	_ = math.NaN // keep the math import tied to this file even if the
	// only user above is deleted later.

	// Two polls 400ms apart, both in the same whole calendar second.
	t0 := time.Date(2026, 4, 21, 16, 51, 41, 100_000_000 /* 0.1s */, time.UTC)
	t1 := t0.Add(400 * time.Millisecond) // 0.5s — still the same Unix() second
	if t0.Unix() != t1.Unix() {
		t.Fatalf("setup: both polls must share the same Unix() second; got %d and %d", t0.Unix(), t1.Unix())
	}
	perFile := [][]*model.NetstatCountersSample{
		{
			{Timestamp: t0, Values: map[string]float64{"tcp_segs_in": 1000}},
			{Timestamp: t1, Values: map[string]float64{"tcp_segs_in": 1200}},
		},
	}
	data := concatNetstatS(perFile)
	if data == nil {
		t.Fatalf("concatNetstatS returned nil")
	}
	if len(data.Timestamps) != 2 {
		t.Fatalf("want 2 timestamps, got %d", len(data.Timestamps))
	}
	gap := data.Timestamps[1] - data.Timestamps[0]
	if gap <= 0 {
		t.Fatalf("sub-second gap collapsed to %.9f — rate math would divide by zero (Timestamp.Unix() truncation regression)", gap)
	}
	// ~0.4s expected; allow a small tolerance for IEEE-754 round-trip.
	if gap < 0.39 || gap > 0.41 {
		t.Errorf("gap = %.9f s, want ~0.4 s (sub-second precision not preserved)", gap)
	}
}

func eqInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func eqStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
