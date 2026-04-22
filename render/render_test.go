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

// twoSnapshotCollection builds a Collection with two Snapshots, each
// carrying a minimal -iostat / -vmstat / -processlist / -mysqladmin
// payload with distinct timestamps, so every time-series collector
// exercises the concat-across-snapshots path (T054 / FR-018 / FR-030).
func twoSnapshotCollection() *model.Collection {
	ts := func(sec int) time.Time {
		return time.Date(2026, 4, 21, 16, 52, 11+sec, 0, time.UTC)
	}

	iostat := func(tsOffset int) *model.IostatData {
		mkSamples := func(util, aqsz float64) []model.Sample {
			return []model.Sample{
				{Timestamp: ts(tsOffset), Measurements: map[string]float64{"util_percent": util, "avgqu_sz": aqsz}},
				{Timestamp: ts(tsOffset + 1), Measurements: map[string]float64{"util_percent": util + 1, "avgqu_sz": aqsz + 0.1}},
			}
		}
		return &model.IostatData{
			Devices: []model.DeviceSeries{{
				Device:       "sda",
				Utilization:  model.MetricSeries{Metric: "util_percent", Unit: "%", Subject: "sda", Samples: mkSamples(10, 0.5)},
				AvgQueueSize: model.MetricSeries{Metric: "avgqu_sz", Unit: "count", Subject: "sda", Samples: mkSamples(10, 0.5)},
			}},
			SnapshotBoundaries: []int{0},
		}
	}

	vmstat := func(tsOffset int) *model.VmstatData {
		return &model.VmstatData{
			Series: []model.MetricSeries{{
				Metric: "cpu_user",
				Unit:   "%",
				Samples: []model.Sample{
					{Timestamp: ts(tsOffset), Measurements: map[string]float64{"cpu_user": 5}},
					{Timestamp: ts(tsOffset + 1), Measurements: map[string]float64{"cpu_user": 6}},
				},
			}},
			SnapshotBoundaries: []int{0},
		}
	}

	processlist := func(tsOffset int) *model.ProcesslistData {
		return &model.ProcesslistData{
			States: []string{"Sending data", "Sleep"},
			ThreadStateSamples: []model.ThreadStateSample{
				{Timestamp: ts(tsOffset), StateCounts: map[string]int{"Sending data": 2, "Sleep": 8}},
				{Timestamp: ts(tsOffset + 1), StateCounts: map[string]int{"Sending data": 3, "Sleep": 7}},
			},
			SnapshotBoundaries: []int{0},
		}
	}

	mysqladmin := func(tsOffset int, base float64) *model.MysqladminData {
		return &model.MysqladminData{
			VariableNames: []string{"Com_select", "Threads_running"},
			SampleCount:   2,
			Timestamps:    []time.Time{ts(tsOffset), ts(tsOffset + 1)},
			Deltas: map[string][]float64{
				"Com_select":      {base, base + 10}, // counter
				"Threads_running": {12, 13},          // gauge (raw values)
			},
			IsCounter:          map[string]bool{"Com_select": true, "Threads_running": false},
			SnapshotBoundaries: []int{0},
		}
	}

	snap := func(prefix string, tsOffset int, mysqlBase float64) *model.Snapshot {
		return &model.Snapshot{
			Timestamp: ts(tsOffset),
			Prefix:    prefix,
			SourceFiles: map[model.Suffix]*model.SourceFile{
				model.SuffixIostat:      {Suffix: model.SuffixIostat, Parsed: iostat(tsOffset)},
				model.SuffixVmstat:      {Suffix: model.SuffixVmstat, Parsed: vmstat(tsOffset)},
				model.SuffixProcesslist: {Suffix: model.SuffixProcesslist, Parsed: processlist(tsOffset)},
				model.SuffixMysqladmin:  {Suffix: model.SuffixMysqladmin, Parsed: mysqladmin(tsOffset, mysqlBase)},
			},
		}
	}

	return &model.Collection{
		RootPath:  "/tmp/example",
		Hostname:  "example-db-01",
		Snapshots: []*model.Snapshot{snap("2026_04_21_16_52_11", 0, 100), snap("2026_04_21_16_52_41", 30, 500)},
	}
}

// TestMultiSnapshotMergerRecordsBoundaries: FR-018 / T054 — every
// time-series collector that spans more than one Snapshot MUST carry
// SnapshotBoundaries on the merged *Data struct, with the cumulative
// start index of each Snapshot in the concatenated stream.
func TestMultiSnapshotMergerRecordsBoundaries(t *testing.T) {
	c := twoSnapshotCollection()
	// Build is package-private in render; exercise it through Render
	// and by inspecting the Report envelope via CanonicalReportID-free
	// helpers. The simpler path: render and verify the embedded JSON
	// payload carries the boundaries. That round-trip is covered by
	// TestChartPayloadExportsSnapshotBoundaries; here we just confirm
	// the full pipeline accepts multi-Snapshot input without error.
	var buf bytes.Buffer
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatalf("empty render output")
	}
}

// TestChartPayloadExportsSnapshotBoundaries: FR-018 / FR-030 / T054 —
// the embedded JSON chart payload for every time-series subview MUST
// expose its `snapshotBoundaries` field so app.js can draw the
// vertical boundary markers.
func TestChartPayloadExportsSnapshotBoundaries(t *testing.T) {
	c := twoSnapshotCollection()
	var buf bytes.Buffer
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()

	// The chart payload is embedded as a JSON island. We only need to
	// verify it mentions snapshotBoundaries; a full JSON parse would
	// tie the test to the exact payload shape, which is orthogonal.
	mustContain := []string{
		`"snapshotBoundaries"`, // at least one occurrence
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("rendered output missing %q — every time-series chart payload MUST carry snapshotBoundaries per FR-018", s)
		}
	}

	// More specifically, assert boundaries appear within the chart
	// payload block (not, say, only inside a comment).
	payloadIdx := strings.Index(out, `id="report-data"`)
	if payloadIdx == -1 {
		t.Fatalf("no embedded report-data payload found")
	}
	payloadEnd := strings.Index(out[payloadIdx:], "</script>")
	if payloadEnd == -1 {
		t.Fatalf("unterminated report-data payload")
	}
	payload := out[payloadIdx : payloadIdx+payloadEnd]
	if !strings.Contains(payload, `"snapshotBoundaries"`) {
		t.Errorf("report-data payload does not expose snapshotBoundaries; app.js cannot draw boundary markers")
	}
}

// TestMysqladminBoundaryInsertsNaN: FR-030 — when the mysqladmin
// merger spans multiple Snapshots, the first post-boundary slot of
// every counter variable MUST be NaN (serialised as JSON null) so the
// chart shows a discontinuity rather than a misleading cross-snapshot
// delta.
func TestMysqladminBoundaryInsertsNaN(t *testing.T) {
	c := twoSnapshotCollection()
	var buf bytes.Buffer
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()

	payloadIdx := strings.Index(out, `id="report-data"`)
	payloadEnd := strings.Index(out[payloadIdx:], "</script>")
	payload := out[payloadIdx : payloadIdx+payloadEnd]

	// Counter variable "Com_select": two snapshots, 2 samples each.
	// Merger emits [100, 110, NaN, 510] (the first post-boundary slot
	// is NaN per FR-030, overwriting the raw 500).
	if !strings.Contains(payload, `"Com_select":[100,110,null,510]`) {
		t.Errorf("counter Com_select did not carry the expected NaN-at-boundary shape; payload excerpt: %s", sliceAround(payload, "Com_select", 80))
	}

	// Gauge variable "Threads_running": straight concatenation, no NaN
	// introduced at the boundary.
	if !strings.Contains(payload, `"Threads_running":[12,13,12,13]`) {
		t.Errorf("gauge Threads_running should concat raw values without NaN injection; payload excerpt: %s", sliceAround(payload, "Threads_running", 80))
	}
}

// sliceAround returns a short window of s around the first occurrence
// of needle, for readable test error messages.
func sliceAround(s, needle string, pad int) string {
	i := strings.Index(s, needle)
	if i == -1 {
		return ""
	}
	start := i - pad
	if start < 0 {
		start = 0
	}
	end := i + len(needle) + pad
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}
