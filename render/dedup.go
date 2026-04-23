package render

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

// snapshotSignature returns a stable hash of a snapshot's variable
// entries (sorted by name, skipping the volatile set). Adjacent
// snapshots with identical signatures are collapsed into a single
// rendered panel.
func snapshotSignature(entries []model.VariableEntry) string {
	filtered := make([]string, 0, len(entries))
	for _, e := range entries {
		if _, skip := volatileVariables[strings.ToLower(e.Name)]; skip {
			continue
		}
		filtered = append(filtered, e.Name+"\x00"+e.Value)
	}
	sort.Strings(filtered)
	h := fnv.New64a()
	for _, s := range filtered {
		h.Write([]byte(s))
		h.Write([]byte{'\n'})
	}
	return fmt.Sprintf("%016x", h.Sum64())
}

// computeVariableSignatures returns one snapshotSignature per
// PerSnapshot entry (empty string when Data is nil). Computing it once
// up front lets both buildNavigation and buildView consume the result
// without re-hashing every VariableEntry slice (NIT #27).
func computeVariableSignatures(sec *model.VariablesSection) []string {
	if sec == nil {
		return nil
	}
	out := make([]string, len(sec.PerSnapshot))
	for i, sv := range sec.PerSnapshot {
		if sv.Data == nil {
			continue
		}
		out[i] = snapshotSignature(sv.Data.Entries)
	}
	return out
}

// extendRange grows the RangeNote on a kept snapshot view to cover
// the given 1-based snapshot number. RangeNote is re-formatted each
// call so the final text always reflects the full inclusive run.
func extendRange(v *variableSnapshotView, snapNum int) {
	lo, hi := parseRangeNote(v.RangeNote)
	// A malformed or empty RangeNote yields (0, 0) — treat that as
	// "no prior range seeded" and anchor on the current snapNum so
	// extendRange stays resilient to callers that forgot to seed
	// v.RangeNote (LOW #11).
	if lo == 0 && hi == 0 {
		lo = snapNum
		hi = snapNum
	}
	if hi < snapNum {
		hi = snapNum
	}
	if lo > snapNum {
		lo = snapNum
	}
	v.RangeNote = formatRangeNote(lo, hi)
}

func formatRangeNote(lo, hi int) string {
	if lo == hi {
		return fmt.Sprintf("Identical values seen in snapshot #%d", lo)
	}
	return fmt.Sprintf("Identical values seen in snapshots #%d–#%d", lo, hi)
}

// parseRangeNote recovers (lo, hi) from a formatRangeNote string.
// When the note is missing or malformed (shouldn't happen) returns
// (0, 0) and callers can recover gracefully.
func parseRangeNote(note string) (int, int) {
	var lo, hi int
	if _, err := fmt.Sscanf(note, "Identical values seen in snapshot #%d", &lo); err == nil && lo > 0 {
		return lo, lo
	}
	if _, err := fmt.Sscanf(note, "Identical values seen in snapshots #%d–#%d", &lo, &hi); err == nil {
		return lo, hi
	}
	return 0, 0
}

func rangeIsSingle(note string) bool {
	lo, hi := parseRangeNote(note)
	return lo != 0 && lo == hi
}
