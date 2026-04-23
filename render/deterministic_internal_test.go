package render

import (
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// minimalCollectionInternal builds a Collection with one snapshot whose
// SourceFiles map is empty. Duplicated from render_test.go's external
// helper so this internal-package test file has no dependency on the
// _test package.
func minimalCollectionInternal() *model.Collection {
	return &model.Collection{
		RootPath: "/tmp/example",
		Hostname: "example-db-01",
		Snapshots: []*model.Snapshot{
			{
				Timestamp:   time.Date(2026, 4, 21, 16, 52, 11, 0, time.UTC),
				Prefix:      "2026_04_21_16_52_11",
				SourceFiles: map[model.Suffix]*model.SourceFile{},
			},
		},
	}
}

// TestReportIDStability: spec FR-032 — ReportID is stable across
// renders of the same collection, and changes when the collection
// structure changes.
func TestReportIDStability(t *testing.T) {
	a := canonicalReportID(minimalCollectionInternal())
	b := canonicalReportID(minimalCollectionInternal())
	if a != b {
		t.Errorf("same input produced different ReportIDs: %q vs %q", a, b)
	}
	c := minimalCollectionInternal()
	c.Hostname = "a-different-host"
	if d := canonicalReportID(c); d == a {
		t.Errorf("hostname change did not alter ReportID: %q", d)
	}
}
