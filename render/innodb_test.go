package render

import (
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/model"
)

// TestAggregateInnoDBMetricsEmpty covers the "no snapshots with Data"
// path and the "all snapshots nil" path — both should collapse to
// nil with no panic.
func TestAggregateInnoDBMetricsEmpty(t *testing.T) {
	if got := aggregateInnoDBMetrics(nil); got != nil {
		t.Fatalf("nil input: want nil, got %+v", got)
	}
	snaps := []model.SnapshotInnoDB{
		{SnapshotPrefix: "snap1"},
		{SnapshotPrefix: "snap2"},
	}
	if got := aggregateInnoDBMetrics(snaps); got != nil {
		t.Fatalf("all-nil snapshots: want nil, got %+v", got)
	}
}

// TestAggregateInnoDBMetricsOneSnapshot checks that min/avg/max all
// degenerate to the single sample's value and that the four cards are
// emitted in the declared order.
func TestAggregateInnoDBMetricsOneSnapshot(t *testing.T) {
	snaps := []model.SnapshotInnoDB{
		{
			SnapshotPrefix: "only",
			Data: &model.InnodbStatusData{
				SemaphoreCount:    3,
				PendingReads:      1,
				PendingWrites:     2,
				HistoryListLength: 42,
				AHIActivity: model.AHIActivity{
					HashTableSize:         34679,
					SearchesPerSec:        10,
					NonHashSearchesPerSec: 0,
				},
			},
		},
	}
	got := aggregateInnoDBMetrics(snaps)
	if len(got) != 4 {
		t.Fatalf("want 4 cards, got %d", len(got))
	}
	wantLabels := []string{"Semaphores", "Pending I/O", "AHI", "History list"}
	for i, m := range got {
		if m.Label != wantLabels[i] {
			t.Errorf("card %d: want label %q, got %q", i, wantLabels[i], m.Label)
		}
	}
	if got[0].Worst != "3" || got[0].Min != "3" || got[0].Max != "3" {
		t.Errorf("Semaphores: want 3/3/3, got %s/%s/%s", got[0].Min, got[0].Avg, got[0].Max)
	}
	if got[1].Worst != "3" { // 1 read + 2 writes
		t.Errorf("Pending I/O worst: want 3, got %s", got[1].Worst)
	}
	if got[3].Worst != "42" {
		t.Errorf("HLL worst: want 42, got %s", got[3].Worst)
	}
}

// TestAggregateInnoDBMetricsMulti checks variation across three
// snapshots. Semaphore min/max should straddle the range.
func TestAggregateInnoDBMetricsMulti(t *testing.T) {
	snaps := []model.SnapshotInnoDB{
		{Data: &model.InnodbStatusData{SemaphoreCount: 1, PendingReads: 0, PendingWrites: 0, HistoryListLength: 10}},
		{Data: &model.InnodbStatusData{SemaphoreCount: 5, PendingReads: 2, PendingWrites: 0, HistoryListLength: 20}},
		{Data: &model.InnodbStatusData{SemaphoreCount: 3, PendingReads: 1, PendingWrites: 1, HistoryListLength: 30}},
	}
	got := aggregateInnoDBMetrics(snaps)
	// Semaphores
	if got[0].Min != "1" || got[0].Max != "5" || got[0].Worst != "5" {
		t.Errorf("Semaphores: want 1/_/5, worst=5; got %s/%s/%s worst=%s",
			got[0].Min, got[0].Avg, got[0].Max, got[0].Worst)
	}
	// HLL
	if got[3].Min != "10" || got[3].Max != "30" {
		t.Errorf("HLL: want 10/_/30; got %s/%s/%s", got[3].Min, got[3].Avg, got[3].Max)
	}
}

// TestBuildAHIMetricZeroActivity verifies that a capture with no AHI
// activity at all collapses to the "no activity" card: all four
// numeric fields are "–" and AHINoActivity is true.
func TestBuildAHIMetricZeroActivity(t *testing.T) {
	snaps := []model.SnapshotInnoDB{
		{Data: &model.InnodbStatusData{AHIActivity: model.AHIActivity{HashTableSize: 100}}},
		{Data: &model.InnodbStatusData{AHIActivity: model.AHIActivity{HashTableSize: 100}}},
	}
	m := buildAHIMetric(snaps)
	if m.Label != "AHI" || m.Hint != "hit ratio" {
		t.Errorf("want Label=AHI Hint=hit ratio, got %q %q", m.Label, m.Hint)
	}
	if m.Worst != "–" || m.Min != "–" || m.Avg != "–" || m.Max != "–" {
		t.Errorf("zero-activity: want –/–/–/–, got %s/%s/%s/%s", m.Worst, m.Min, m.Avg, m.Max)
	}
	if !m.AHINoActivity {
		t.Errorf("want AHINoActivity=true")
	}
	if !m.AHIHasHashTable {
		t.Errorf("want AHIHasHashTable=true when HashTableSize>0")
	}
}

// TestBuildAHIMetricMysqldHitRatio is the canonical numeric case:
// hash=3901.21, nonHash=8669.42 should produce a ratio of ~31.0%.
func TestBuildAHIMetricMysqldHitRatio(t *testing.T) {
	snaps := []model.SnapshotInnoDB{
		{
			SnapshotPrefix: "worst",
			Data: &model.InnodbStatusData{
				AHIActivity: model.AHIActivity{
					HashTableSize:         34679,
					SearchesPerSec:        3901.21,
					NonHashSearchesPerSec: 8669.42,
				},
			},
		},
	}
	m := buildAHIMetric(snaps)
	if m.AHIWorstRatio != "31.0" {
		t.Errorf("want AHIWorstRatio=31.0, got %q", m.AHIWorstRatio)
	}
	if m.Worst != "31.0%" {
		t.Errorf("want Worst=31.0%%, got %q", m.Worst)
	}
	if m.AHIWorstSnapshot != "worst" {
		t.Errorf("want AHIWorstSnapshot=worst, got %q", m.AHIWorstSnapshot)
	}
	if m.AHIHashTableSize != "34,679" {
		t.Errorf("want AHIHashTableSize=34,679, got %q", m.AHIHashTableSize)
	}
	if !m.AHIHasHashTable {
		t.Errorf("want AHIHasHashTable=true")
	}
}

// TestAttachSemaphoreBreakdownEmpty — no snapshots ever saw waits:
// BreakdownPeak/Total must both be empty and no panic.
func TestAttachSemaphoreBreakdownEmpty(t *testing.T) {
	m := innoDBMetricView{Label: "Semaphores"}
	snaps := []model.SnapshotInnoDB{
		{Data: &model.InnodbStatusData{SemaphoreCount: 0}},
	}
	attachSemaphoreBreakdown(&m, snaps)
	if len(m.BreakdownPeak) != 0 || len(m.BreakdownTotal) != 0 {
		t.Errorf("want no breakdown rows, got peak=%d total=%d",
			len(m.BreakdownPeak), len(m.BreakdownTotal))
	}
}

// TestAttachSemaphoreBreakdownPeakAndTotal — 3 snapshots, the middle
// one has the peak; the total is the window-wide sum. Also verifies
// the peak snapshot prefix is reported verbatim.
func TestAttachSemaphoreBreakdownPeakAndTotal(t *testing.T) {
	snaps := []model.SnapshotInnoDB{
		{
			SnapshotPrefix: "a",
			Data: &model.InnodbStatusData{
				SemaphoreCount: 2,
				SemaphoreSites: []model.SemaphoreSite{
					{File: "a.cc", Line: 10, MutexName: "M1", WaitCount: 2},
				},
			},
		},
		{
			SnapshotPrefix: "b",
			Data: &model.InnodbStatusData{
				SemaphoreCount: 5,
				SemaphoreSites: []model.SemaphoreSite{
					{File: "a.cc", Line: 10, MutexName: "M1", WaitCount: 3},
					{File: "b.cc", Line: 20, MutexName: "M2", WaitCount: 2},
				},
			},
		},
		{
			SnapshotPrefix: "c",
			Data: &model.InnodbStatusData{
				SemaphoreCount: 1,
				SemaphoreSites: []model.SemaphoreSite{
					{File: "b.cc", Line: 20, MutexName: "M2", WaitCount: 1},
				},
			},
		},
	}
	m := innoDBMetricView{Label: "Semaphores"}
	attachSemaphoreBreakdown(&m, snaps)

	if m.BreakdownPeakSnapshot != "b" {
		t.Errorf("want peak snapshot=b, got %q", m.BreakdownPeakSnapshot)
	}
	if m.BreakdownPeakTotal != 5 {
		t.Errorf("want peak total=5, got %d", m.BreakdownPeakTotal)
	}
	if len(m.BreakdownPeak) != 2 {
		t.Errorf("want 2 peak rows, got %d", len(m.BreakdownPeak))
	}
	// Window total: a.cc#10 (2+3=5), b.cc#20 (2+1=3) — grand total 8.
	if m.BreakdownTotalTotal != 8 {
		t.Errorf("want window total=8, got %d", m.BreakdownTotalTotal)
	}
	if len(m.BreakdownTotal) != 2 {
		t.Errorf("want 2 window-total rows, got %d", len(m.BreakdownTotal))
	}
	// Highest-count site must sort first.
	if m.BreakdownTotal[0].File != "a.cc" || m.BreakdownTotal[0].Count != 5 {
		t.Errorf("want first window row a.cc/5, got %s/%d",
			m.BreakdownTotal[0].File, m.BreakdownTotal[0].Count)
	}
	if !strings.HasPrefix(m.BreakdownTotal[0].Percent, "62") {
		// 5/8 = 62.5%
		t.Errorf("want ~62.5%% for top row, got %s", m.BreakdownTotal[0].Percent)
	}
}

// TestIsMysqldCommandMariaDB exercises MED #2 — both mysqld and
// mariadbd must match; wrappers must not.
func TestIsMysqldCommandMariaDB(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"mysqld", true},
		{"  mysqld  ", true},
		{"MYSQLD", true},
		{"mariadbd", true},
		{" MariaDBd ", true},
		{"mysqld_safe", false},
		{"mariadbd-safe", false},
		{"postgres", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isMysqldCommand(c.in); got != c.want {
			t.Errorf("isMysqldCommand(%q): want %v, got %v", c.in, c.want, got)
		}
	}
}
