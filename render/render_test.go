package render_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/render"
)

// fixedTime returns a deterministic time for tests so two renders
// produce byte-identical output.
func fixedTime() time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

// minimalCollection builds a Collection with one snapshot whose
// SourceFiles map is empty — the "empty-but-valid" case.
func minimalCollection() *model.Collection {
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

// TestRenderEmptyReport: FR-020 — empty-but-valid collection renders a
// report with every section's "data not available" banner visible.
func TestRenderEmptyReport(t *testing.T) {
	var buf bytes.Buffer
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}
	if err := render.Render(&buf, minimalCollection(), opts); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()

	wantSubstrings := []string{
		`<details id="sec-os"`,
		`<details id="sec-variables"`,
		`<details id="sec-db"`,
		`<details id="sec-diagnostics"`,
		`Verbatim passthrough:`,
		`Data not available`,
		`<nav class="index"`,
		`id="report-data"`,
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(out, s) {
			t.Errorf("output missing %q", s)
		}
	}
	if strings.Contains(out, `http://`) {
		// Only benign hit is the uPlot comment https://github.com/leeoniya/uPlot
		// Accept that, but forbid plain http://.
		t.Errorf("rendered HTML contains http:// (network fetch risk)")
	}
}

// TestDeterminism: FR-006 / SC-003 — two renders with the same
// GeneratedAt MUST produce byte-identical output.
func TestDeterminism(t *testing.T) {
	c := minimalCollection()
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}

	var a, b bytes.Buffer
	if err := render.Render(&a, c, opts); err != nil {
		t.Fatalf("render a: %v", err)
	}
	if err := render.Render(&b, c, opts); err != nil {
		t.Fatalf("render b: %v", err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatalf("render is not deterministic (len %d vs %d)", a.Len(), b.Len())
	}
}

// TestGeneratedAtIsOnlyDiff: FR-006 — two renders with different
// GeneratedAt values differ only in the Report generated at line.
func TestGeneratedAtIsOnlyDiff(t *testing.T) {
	c := minimalCollection()
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 1, 0, 0, 5, 0, time.UTC)

	var a, b bytes.Buffer
	if err := render.Render(&a, c, render.RenderOptions{GeneratedAt: t1, Version: "v0.0.1-test"}); err != nil {
		t.Fatalf("render a: %v", err)
	}
	if err := render.Render(&b, c, render.RenderOptions{GeneratedAt: t2, Version: "v0.0.1-test"}); err != nil {
		t.Fatalf("render b: %v", err)
	}

	linesA := strings.Split(a.String(), "\n")
	linesB := strings.Split(b.String(), "\n")
	if len(linesA) != len(linesB) {
		t.Fatalf("line count differs: %d vs %d", len(linesA), len(linesB))
	}
	diff := 0
	for i := range linesA {
		if linesA[i] != linesB[i] {
			diff++
		}
	}
	if diff != 1 {
		t.Fatalf("expected exactly 1 differing line (GeneratedAt), got %d", diff)
	}
}

// TestReportIDStability: spec FR-032 — ReportID is stable across
// renders of the same collection, and changes when the collection
// structure changes.
func TestReportIDStability(t *testing.T) {
	a := render.CanonicalReportID(minimalCollection())
	b := render.CanonicalReportID(minimalCollection())
	if a != b {
		t.Errorf("same input produced different ReportIDs: %q vs %q", a, b)
	}
	c := minimalCollection()
	c.Hostname = "a-different-host"
	if d := render.CanonicalReportID(c); d == a {
		t.Errorf("hostname change did not alter ReportID: %q", d)
	}
}
