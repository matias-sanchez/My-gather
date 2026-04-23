package render

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

// volatileVariables are SHOW VARIABLES entries expected to drift
// between snapshots on a busy server even when the operator has not
// changed anything (e.g. gtid_executed advances on every commit).
// Excluding them from the dedup signature keeps the "collapse
// identical snapshots" behaviour useful on busy replicas. Matching
// is case-insensitive.
var volatileVariables = map[string]struct{}{
	"gtid_executed": {},
	"gtid_purged":   {},
}

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
// up front lets both keptVariableRuns callers consume the result
// without re-hashing every VariableEntry slice.
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

// variableRun groups consecutive PerSnapshot entries that share a
// signature (or are all nil-Data) into one rendered panel. StartIdx /
// EndIdx are inclusive 0-based indexes into
// VariablesSection.PerSnapshot.
type variableRun struct {
	StartIdx int
	EndIdx   int
	NilData  bool
}

// keptVariableRuns is the single source of truth for Variables-view
// dedup. Both buildNavigation and buildView consume it so a nav entry
// and its rendered panel can never disagree on which snapshots were
// collapsed.
//
// Rules:
//   - A nil-Data snapshot always starts (and ends) its own run — a
//     capture gap is a hard boundary for dedup.
//   - A non-nil-Data snapshot extends the previous run iff that run
//     was also non-nil and shares the same signature; otherwise it
//     starts a new run.
func keptVariableRuns(sec *model.VariablesSection, sigs []string) []variableRun {
	if sec == nil || len(sec.PerSnapshot) == 0 {
		return nil
	}
	runs := make([]variableRun, 0, len(sec.PerSnapshot))
	for i, sv := range sec.PerSnapshot {
		if sv.Data == nil {
			runs = append(runs, variableRun{StartIdx: i, EndIdx: i, NilData: true})
			continue
		}
		if n := len(runs); n > 0 && !runs[n-1].NilData && sigs[i] == sigs[runs[n-1].StartIdx] {
			runs[n-1].EndIdx = i
			continue
		}
		runs = append(runs, variableRun{StartIdx: i, EndIdx: i})
	}
	return runs
}

// formatRangeNote is the one-way formatter used to surface a kept
// panel's snapshot range to the reader. It is never parsed back;
// numeric state lives on variableSnapshotView.rangeLo / rangeHi.
func formatRangeNote(lo, hi int) string {
	if lo == hi {
		return fmt.Sprintf("Identical values seen in snapshot #%d", lo)
	}
	return fmt.Sprintf("Identical values seen in snapshots #%d–#%d", lo, hi)
}
