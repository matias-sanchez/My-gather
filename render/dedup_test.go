package render

import (
	"testing"

	"github.com/matias-sanchez/My-gather/model"
)

// TestSnapshotSignature — two snapshots that only differ in the
// volatile set (gtid_executed / gtid_purged) MUST produce the same
// signature; any non-volatile difference MUST flip it.
func TestSnapshotSignature(t *testing.T) {
	base := []model.VariableEntry{
		{Name: "innodb_buffer_pool_size", Value: "128M"},
		{Name: "max_connections", Value: "200"},
		{Name: "gtid_executed", Value: "aaaa:1-100"},
	}
	volatileOnly := []model.VariableEntry{
		{Name: "innodb_buffer_pool_size", Value: "128M"},
		{Name: "max_connections", Value: "200"},
		{Name: "gtid_executed", Value: "aaaa:1-999"}, // moved forward
		{Name: "gtid_purged", Value: "aaaa:1-50"},    // new-but-volatile
	}
	meaningful := []model.VariableEntry{
		{Name: "innodb_buffer_pool_size", Value: "256M"}, // doubled
		{Name: "max_connections", Value: "200"},
		{Name: "gtid_executed", Value: "aaaa:1-100"},
	}

	sigBase := snapshotSignature(base)
	sigVolatile := snapshotSignature(volatileOnly)
	sigMeaningful := snapshotSignature(meaningful)

	if sigBase != sigVolatile {
		t.Errorf("volatile-only change must keep signature stable: %q != %q", sigBase, sigVolatile)
	}
	if sigBase == sigMeaningful {
		t.Errorf("non-volatile change must flip signature: both %q", sigBase)
	}
	// Case-insensitive volatile lookup.
	weirdCase := []model.VariableEntry{
		{Name: "innodb_buffer_pool_size", Value: "128M"},
		{Name: "max_connections", Value: "200"},
		{Name: "GTID_EXECUTED", Value: "zzz:1-1"},
	}
	if snapshotSignature(weirdCase) != sigBase {
		t.Errorf("volatile match must be case-insensitive")
	}
}

// TestDedupPreservesFirstAndChanges — an [A, A, B, B] sequence
// collapses to two panels. The first panel's RangeNote covers #1–#2,
// the second covers #3–#4, and the second panel's Changed flag is
// true ONLY for rows that actually differ from panel 1.
func TestDedupPreservesFirstAndChanges(t *testing.T) {
	mkEntries := func(bp, maxConn string) []model.VariableEntry {
		return []model.VariableEntry{
			{Name: "innodb_buffer_pool_size", Value: bp},
			{Name: "max_connections", Value: maxConn},
			{Name: "sort_buffer_size", Value: "262144"}, // stays constant in both A and B
		}
	}
	a := mkEntries("128M", "200")
	b := mkEntries("256M", "200") // only innodb_buffer_pool_size differs

	rpt := &model.Report{
		VariablesSection: &model.VariablesSection{
			PerSnapshot: []model.SnapshotVariables{
				{SnapshotPrefix: "snap1", Data: &model.VariablesData{Entries: a}},
				{SnapshotPrefix: "snap2", Data: &model.VariablesData{Entries: a}},
				{SnapshotPrefix: "snap3", Data: &model.VariablesData{Entries: b}},
				{SnapshotPrefix: "snap4", Data: &model.VariablesData{Entries: b}},
			},
		},
		Collection: &model.Collection{Snapshots: []*model.Snapshot{{}, {}, {}, {}}},
	}
	sigs := computeVariableSignatures(rpt.VariablesSection)
	view, err := buildView(rpt, rpt.Collection, sigs)
	if err != nil {
		t.Fatalf("buildView: %v", err)
	}
	if len(view.VariableSnapshots) != 2 {
		t.Fatalf("want 2 kept panels, got %d", len(view.VariableSnapshots))
	}
	if view.VariableSnapshots[0].RangeNote == "" {
		t.Errorf("first panel: want non-empty RangeNote spanning #1–#2, got empty")
	}
	lo, hi := parseRangeNote(view.VariableSnapshots[0].RangeNote)
	if lo != 1 || hi != 2 {
		t.Errorf("first panel RangeNote: want #1-#2, got #%d-#%d (note=%q)", lo, hi, view.VariableSnapshots[0].RangeNote)
	}
	lo2, hi2 := parseRangeNote(view.VariableSnapshots[1].RangeNote)
	if lo2 != 3 || hi2 != 4 {
		t.Errorf("second panel RangeNote: want #3-#4, got #%d-#%d", lo2, hi2)
	}

	// Second panel: only innodb_buffer_pool_size should be flagged
	// Changed. sort_buffer_size and max_connections did not change.
	var changedNames []string
	for _, row := range view.VariableSnapshots[1].Entries {
		if row.Changed {
			changedNames = append(changedNames, row.Name)
		}
	}
	if len(changedNames) != 1 || changedNames[0] != "innodb_buffer_pool_size" {
		t.Errorf("second panel Changed rows: want [innodb_buffer_pool_size], got %v", changedNames)
	}
	if view.VariableSnapshots[1].ChangedCount != 1 {
		t.Errorf("second panel ChangedCount: want 1, got %d", view.VariableSnapshots[1].ChangedCount)
	}
	// First panel must not flag anything — nothing to compare against.
	if view.VariableSnapshots[0].ChangedCount != 0 {
		t.Errorf("first panel ChangedCount: want 0, got %d", view.VariableSnapshots[0].ChangedCount)
	}
}

// TestDedupCyclePattern documents the chosen behaviour for a
// non-monotonic signature sequence like [A, A, B, B, A]: the second
// run of A (at snapshot #5) is kept as a fresh panel with its own
// RangeNote rather than bridged back to the first A range. The dedup
// only compares against the last KEPT panel, not every prior panel.
// Pinning this keeps the reader's mental model predictable — each
// panel is a contiguous identical-snapshots run — and prevents future
// refactors from silently turning this into history-aware collapsing.
func TestDedupCyclePattern(t *testing.T) {
	a := []model.VariableEntry{
		{Name: "innodb_buffer_pool_size", Value: "128M"},
		{Name: "max_connections", Value: "200"},
	}
	b := []model.VariableEntry{
		{Name: "innodb_buffer_pool_size", Value: "256M"},
		{Name: "max_connections", Value: "200"},
	}
	rpt := &model.Report{
		VariablesSection: &model.VariablesSection{
			PerSnapshot: []model.SnapshotVariables{
				{SnapshotPrefix: "snap1", Data: &model.VariablesData{Entries: a}},
				{SnapshotPrefix: "snap2", Data: &model.VariablesData{Entries: a}},
				{SnapshotPrefix: "snap3", Data: &model.VariablesData{Entries: b}},
				{SnapshotPrefix: "snap4", Data: &model.VariablesData{Entries: b}},
				{SnapshotPrefix: "snap5", Data: &model.VariablesData{Entries: a}},
			},
		},
		Collection: &model.Collection{Snapshots: []*model.Snapshot{{}, {}, {}, {}, {}}},
	}
	sigs := computeVariableSignatures(rpt.VariablesSection)
	view, err := buildView(rpt, rpt.Collection, sigs)
	if err != nil {
		t.Fatalf("buildView: %v", err)
	}
	if got := len(view.VariableSnapshots); got != 3 {
		t.Fatalf("want 3 kept panels for [A,A,B,B,A]; got %d", got)
	}
	// Panel 3 (second A) is a single-snapshot run (#5). RangeNote is
	// deliberately cleared for single-snapshot runs (rangeIsSingle
	// branch in buildView), so the note must be empty — NOT bridged
	// back to the first A run at #1-#2.
	if note := view.VariableSnapshots[2].RangeNote; note != "" {
		t.Errorf("second-A panel RangeNote: want \"\" (single-snapshot run, no cycle collapse); got %q",
			note)
	}
	// The first two kept panels DO span multiple snapshots, so their
	// RangeNote must remain populated (contrast with panel 3).
	if lo, hi := parseRangeNote(view.VariableSnapshots[0].RangeNote); lo != 1 || hi != 2 {
		t.Errorf("first A-run RangeNote: want #1-#2, got #%d-#%d (note=%q)",
			lo, hi, view.VariableSnapshots[0].RangeNote)
	}
	if lo, hi := parseRangeNote(view.VariableSnapshots[1].RangeNote); lo != 3 || hi != 4 {
		t.Errorf("B-run RangeNote: want #3-#4, got #%d-#%d (note=%q)",
			lo, hi, view.VariableSnapshots[1].RangeNote)
	}
	// Panel 3 is compared against panel 2 (the last kept panel, which
	// held B), so innodb_buffer_pool_size must be flagged Changed on
	// the re-entry to A.
	var changed []string
	for _, row := range view.VariableSnapshots[2].Entries {
		if row.Changed {
			changed = append(changed, row.Name)
		}
	}
	if len(changed) != 1 || changed[0] != "innodb_buffer_pool_size" {
		t.Errorf("second-A panel Changed rows: want [innodb_buffer_pool_size] (vs prior B); got %v", changed)
	}
}

// TestRangeNoteRoundtrip — formatRangeNote and parseRangeNote are
// inverses for every valid (lo, hi) pair.
func TestRangeNoteRoundtrip(t *testing.T) {
	cases := [][2]int{
		{1, 1},
		{1, 5},
		{3, 10},
		{100, 200},
	}
	for _, c := range cases {
		note := formatRangeNote(c[0], c[1])
		lo, hi := parseRangeNote(note)
		if lo != c[0] || hi != c[1] {
			t.Errorf("roundtrip (%d,%d): note=%q parsed as (%d,%d)", c[0], c[1], note, lo, hi)
		}
	}
}

// TestExtendRangeEmptyInput — LOW #11 — a blank RangeNote (or any
// malformed note that parseRangeNote returns (0,0) for) must be
// treated as "no range seeded" and anchored on the current snapNum
// rather than collapsing to (0, snapNum).
func TestExtendRangeEmptyInput(t *testing.T) {
	v := &variableSnapshotView{}
	extendRange(v, 5)
	lo, hi := parseRangeNote(v.RangeNote)
	if lo != 5 || hi != 5 {
		t.Errorf("extendRange on empty note: want (5,5), got (%d,%d); note=%q", lo, hi, v.RangeNote)
	}
}
