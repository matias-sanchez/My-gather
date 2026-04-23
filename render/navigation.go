package render

import (
	"fmt"

	"github.com/matias-sanchez/My-gather/model"
)

// buildNavigation produces a flat, deterministic list of navigation
// entries. Level-1 and Level-2 entries interleave in the order the
// templates render them.
//
// sigs is the per-snapshot signature cache produced by
// computeVariableSignatures — passing it in avoids a second pass over
// every VariableEntry slice during buildView.
func buildNavigation(r *model.Report, sigs []string) []model.NavEntry {
	var nav []model.NavEntry

	// Environment section — rendered first in the report, so the nav rail
	// mirrors that order.
	nav = append(nav, model.NavEntry{ID: "sec-environment", Title: "Environment", Level: 1})

	// OS section.
	nav = append(nav, model.NavEntry{ID: "sec-os", Title: "OS Usage", Level: 1})
	nav = append(nav, model.NavEntry{ID: "sub-os-iostat", Title: "Disk utilization", Level: 2, ParentID: "sec-os"})
	nav = append(nav, model.NavEntry{ID: "sub-os-top", Title: "Top CPU processes", Level: 2, ParentID: "sec-os"})
	nav = append(nav, model.NavEntry{ID: "sub-os-vmstat", Title: "vmstat saturation", Level: 2, ParentID: "sec-os"})
	nav = append(nav, model.NavEntry{ID: "sub-os-meminfo", Title: "Memory usage", Level: 2, ParentID: "sec-os"})

	// Variables section (one Level-2 per *unique* snapshot — adjacent
	// snapshots with identical variables are collapsed, matching the
	// panels rendered in the Variables body). nil-Data snapshots
	// always get their own entry so partial captures stay visible.
	// keptVariableRuns is the single source of truth for which
	// snapshots survive dedup; buildView consumes the same runs so
	// nav entries and rendered panels cannot disagree.
	nav = append(nav, model.NavEntry{ID: "sec-variables", Title: "Variables", Level: 1})
	if r.VariablesSection != nil {
		for _, run := range keptVariableRuns(r.VariablesSection, sigs) {
			sv := r.VariablesSection.PerSnapshot[run.StartIdx]
			nav = append(nav, model.NavEntry{
				ID:       variablesSnapshotID(run.StartIdx),
				Title:    sv.SnapshotPrefix,
				Level:    2,
				ParentID: "sec-variables",
			})
		}
	}

	// DB section.
	nav = append(nav, model.NavEntry{ID: "sec-db", Title: "Database Usage", Level: 1})
	nav = append(nav, model.NavEntry{ID: "sub-db-innodb", Title: "InnoDB status", Level: 2, ParentID: "sec-db"})
	nav = append(nav, model.NavEntry{ID: "sub-db-mysqladmin", Title: "Counter deltas", Level: 2, ParentID: "sec-db"})
	nav = append(nav, model.NavEntry{ID: "sub-db-processlist", Title: "Thread states", Level: 2, ParentID: "sec-db"})

	// Advisor section — rule-based findings.
	nav = append(nav, model.NavEntry{ID: "sec-advisor", Title: "Advisor", Level: 1})

	return nav
}

func variablesSnapshotID(idx int) string {
	return fmt.Sprintf("sub-var-%d", idx+1)
}

// groupNavigation flattens Report.Navigation into one navGroupView per
// Level-1 entry with its Level-2 children nested. Order is preserved.
func groupNavigation(nav []model.NavEntry) []navGroupView {
	var groups []navGroupView
	idxByID := map[string]int{}
	for _, e := range nav {
		if e.Level == 1 {
			groups = append(groups, navGroupView{ID: e.ID, Title: e.Title})
			idxByID[e.ID] = len(groups) - 1
		} else if e.Level == 2 {
			parentIdx, ok := idxByID[e.ParentID]
			if !ok {
				continue
			}
			groups[parentIdx].Children = append(groups[parentIdx].Children, navChildView{ID: e.ID, Title: e.Title})
		}
	}
	return groups
}
