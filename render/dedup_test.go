package render

import (
	"fmt"
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

// buildDedupView is the shared test harness for dedup assertions:
// feed it per-snapshot VariableEntry slices and it returns the
// collapsed reportView the renderer would produce. Canonical fixture
// shape for dedup tests — adding a new test must not duplicate this.
func buildDedupView(t *testing.T, snapshots ...[]model.VariableEntry) *reportView {
	t.Helper()
	per := make([]model.SnapshotVariables, len(snapshots))
	snaps := make([]*model.Snapshot, len(snapshots))
	for i, e := range snapshots {
		per[i] = model.SnapshotVariables{
			SnapshotPrefix: fmt.Sprintf("snap%d", i+1),
			Data:           &model.VariablesData{Entries: e},
		}
		snaps[i] = &model.Snapshot{}
	}
	rpt := &model.Report{
		VariablesSection: &model.VariablesSection{PerSnapshot: per},
		Collection:       &model.Collection{Snapshots: snaps},
	}
	sigs := computeVariableSignatures(rpt.VariablesSection)
	view, err := buildView(rpt, rpt.Collection, sigs)
	if err != nil {
		t.Fatalf("buildView: %v", err)
	}
	return view
}

// changedRowNames returns the sorted names of rows flagged Changed in
// a collapsed variable-snapshot view. Extracted so both the linear and
// the cyclic dedup tests assert Changed semantics through the same
// lens.
func changedRowNames(v variableSnapshotView) []string {
	var out []string
	for _, row := range v.Entries {
		if row.Changed {
			out = append(out, row.Name)
		}
	}
	return out
}

// TestDedupPreservesFirstAndChanges — an [A, A, B, B] sequence
// collapses to two panels. The first panel's range covers snapshots
// #1–#2, the second covers #3–#4, and the second panel's Changed flag
// is true ONLY for rows that actually differ from panel 1.
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

	view := buildDedupView(t, a, a, b, b)
	if len(view.VariableSnapshots) != 2 {
		t.Fatalf("want 2 kept panels, got %d", len(view.VariableSnapshots))
	}
	assertRange(t, view.VariableSnapshots[0], 1, 2, "first panel")
	assertRange(t, view.VariableSnapshots[1], 3, 4, "second panel")

	// Second panel: only innodb_buffer_pool_size should be flagged
	// Changed. sort_buffer_size and max_connections did not change.
	changedNames := changedRowNames(view.VariableSnapshots[1])
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

// assertRange pins a kept panel's numeric range (rangeLo..rangeHi)
// and asserts RangeNote derives consistently: empty for a single-
// snapshot run, populated with formatRangeNote output otherwise.
// Extracted so every dedup test reads range through one lens.
func assertRange(t *testing.T, vs variableSnapshotView, wantLo, wantHi int, label string) {
	t.Helper()
	if vs.rangeLo != wantLo || vs.rangeHi != wantHi {
		t.Errorf("%s range: want #%d-#%d, got #%d-#%d",
			label, wantLo, wantHi, vs.rangeLo, vs.rangeHi)
	}
	wantNote := ""
	if wantLo != wantHi {
		wantNote = formatRangeNote(wantLo, wantHi)
	}
	if vs.RangeNote != wantNote {
		t.Errorf("%s RangeNote: want %q, got %q", label, wantNote, vs.RangeNote)
	}
}

// TestDedupCyclePattern documents the chosen behaviour for a
// non-monotonic signature sequence like [A, A, B, B, A]: the second
// run of A (at snapshot #5) is kept as a fresh panel rather than
// bridged back to the first A range. The dedup only compares against
// the last KEPT panel, not every prior panel. Pinning this keeps the
// reader's mental model predictable — each panel is a contiguous
// identical-snapshots run — and prevents future refactors from
// silently turning this into history-aware collapsing.
func TestDedupCyclePattern(t *testing.T) {
	a := []model.VariableEntry{
		{Name: "innodb_buffer_pool_size", Value: "128M"},
		{Name: "max_connections", Value: "200"},
	}
	b := []model.VariableEntry{
		{Name: "innodb_buffer_pool_size", Value: "256M"},
		{Name: "max_connections", Value: "200"},
	}
	view := buildDedupView(t, a, a, b, b, a)
	if got := len(view.VariableSnapshots); got != 3 {
		t.Fatalf("want 3 kept panels for [A,A,B,B,A]; got %d", got)
	}
	// First two runs span multiple snapshots (populated RangeNote);
	// the second-A panel is a single-snapshot run (#5 only, no note).
	assertRange(t, view.VariableSnapshots[0], 1, 2, "first A-run")
	assertRange(t, view.VariableSnapshots[1], 3, 4, "B-run")
	assertRange(t, view.VariableSnapshots[2], 5, 5, "second-A single-snapshot")

	// Panel 3 is compared against panel 2 (the last kept panel, which
	// held B), so innodb_buffer_pool_size must be flagged Changed on
	// the re-entry to A.
	changed := changedRowNames(view.VariableSnapshots[2])
	if len(changed) != 1 || changed[0] != "innodb_buffer_pool_size" {
		t.Errorf("second-A panel Changed rows: want [innodb_buffer_pool_size] (vs prior B); got %v", changed)
	}
}

// TestFormatRangeNote — formatRangeNote is the one-way presentation
// formatter; pin its output shape so templates and screen readers
// stay aligned. No parseRangeNote pair exists anymore — the range is
// tracked as numeric state on variableSnapshotView.
func TestFormatRangeNote(t *testing.T) {
	cases := []struct {
		lo, hi int
		want   string
	}{
		{1, 1, "Identical values seen in snapshot #1"},
		{1, 5, "Identical values seen in snapshots #1–#5"},
		{3, 10, "Identical values seen in snapshots #3–#10"},
	}
	for _, c := range cases {
		got := formatRangeNote(c.lo, c.hi)
		if got != c.want {
			t.Errorf("formatRangeNote(%d,%d) = %q; want %q", c.lo, c.hi, got, c.want)
		}
	}
}
