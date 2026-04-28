package render_test

import (
	"bytes"
	"encoding/json"
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
		`<details id="sec-advisor"`,
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

// TestAdvisorCardRenders exercises the end-to-end path from a
// Collection containing mysqladmin data through the findings engine
// and into the rendered HTML. It asserts that the severity-filter
// chips are emitted and that a Critical finding-card lands in the
// output. Keeping the assertions coarse-grained (class names + chip
// presence) lets the test survive minor template tweaks while still
// defending against the Advisor section silently breaking.
func TestAdvisorCardRenders(t *testing.T) {
	// Construct mysqladmin data whose Innodb_buffer_pool_wait_free
	// counter increments — a non-zero rate triggers findings rule
	// "bp.wait_free" at SeverityCrit. Using a two-sample window for
	// minimal bookkeeping; rate = total / windowSeconds.
	t0 := time.Date(2026, 4, 21, 16, 52, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Second)
	mysqladmin := &model.MysqladminData{
		VariableNames: []string{"Innodb_buffer_pool_wait_free"},
		SampleCount:   2,
		Timestamps:    []time.Time{t0, t1},
		Deltas: map[string][]float64{
			// Index 0 = raw initial tally (skipped by counterTotal);
			// index 1 = per-sample delta (contributes to the rate).
			"Innodb_buffer_pool_wait_free": {100, 50},
		},
		IsCounter:          map[string]bool{"Innodb_buffer_pool_wait_free": true},
		SnapshotBoundaries: []int{0},
	}

	c := &model.Collection{
		RootPath: "/tmp/example",
		Hostname: "example-db-01",
		Snapshots: []*model.Snapshot{
			{
				Timestamp: t0,
				Prefix:    "2026_04_21_16_52_00",
				SourceFiles: map[model.Suffix]*model.SourceFile{
					model.SuffixMysqladmin: {
						Suffix: model.SuffixMysqladmin,
						Parsed: mysqladmin,
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()

	// Filter chips — all four must be emitted as interactive toggles
	// with data-sev + aria-pressed present so filter state is
	// announced to assistive tech.
	for _, sev := range []string{"crit", "warn", "info", "ok"} {
		want := `data-sev="` + sev + `"`
		if !strings.Contains(out, want) {
			t.Errorf("missing severity filter chip %s", want)
		}
	}
	// Default filter state: Critical + Warning are pressed on first
	// paint, Info + OK start off (folded behind a toggle). Total of
	// aria-pressed attributes (both true and false) must still be 4,
	// and aria-pressed="true" must appear exactly twice.
	if got := strings.Count(out, `aria-pressed=`); got < 4 {
		t.Errorf("expected 4 aria-pressed attributes across severity chips; got %d", got)
	}
	if got := strings.Count(out, `aria-pressed="true"`); got != 2 {
		t.Errorf("expected exactly 2 chips with aria-pressed=\"true\" (Crit+Warn default); got %d", got)
	}
	if got := strings.Count(out, `aria-pressed="false"`); got != 2 {
		t.Errorf("expected exactly 2 chips with aria-pressed=\"false\" (Info+OK default); got %d", got)
	}
	// Chip count values must be numeric and match what findings.Summarise
	// produced: exactly 1 Critical finding (bp.wait_free), 0 others.
	if !strings.Contains(out, `<span class="n">1</span>`) {
		t.Errorf("expected a chip with count=1 for the triggered Crit finding")
	}

	// Subsystem grouping: findings are wrapped in <section class=
	// "advisor-group" data-subsystem="Buffer Pool"> and the group
	// header carries the subsystem name.
	if !strings.Contains(out, `data-subsystem="Buffer Pool"`) {
		t.Errorf("output missing Buffer Pool advisor group")
	}
	if !strings.Contains(out, `class="advisor-group-title"`) {
		t.Errorf("output missing .advisor-group-title (subsystem header)")
	}

	// At least one Critical finding card.
	if !strings.Contains(out, `class="finding sev-crit"`) {
		t.Errorf("output missing Critical finding card (class=\"finding sev-crit\")")
	}
	// Each finding card exposes its severity via data-sev so the
	// filter JS can hide/show it; ensure that's wired.
	if !strings.Contains(out, `data-sev="crit"`) {
		t.Errorf("output missing finding-card data-sev=\"crit\"")
	}
	// The severity pill in the card summary.
	if !strings.Contains(out, `class="sev-chip sev-crit"`) {
		t.Errorf("output missing Critical severity pill (class=\"sev-chip sev-crit\")")
	}
	// Specific Crit-rule identity: bp.wait_free's summary line names
	// the metric by its canonical status variable.
	if !strings.Contains(out, `Innodb_buffer_pool_wait_free`) {
		t.Errorf("output missing canonical counter name in finding body")
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

// twoSnapshotCollection builds a Collection with two Snapshots,
// each carrying a minimal payload for every time-series collector
// (-iostat / -top / -vmstat / -processlist / -mysqladmin) with
// distinct timestamps, so every concat-across-Snapshots path
// (T054 / FR-018 / FR-030) is exercised end-to-end through Render.
func twoSnapshotCollection() *model.Collection {
	ts := func(sec int) time.Time {
		return time.Date(2026, 4, 21, 16, 52, 11+sec, 0, time.UTC)
	}

	top := func(tsOffset int) *model.TopData {
		mkCPUSeries := func(base float64) model.MetricSeries {
			return model.MetricSeries{Metric: "cpu_percent", Unit: "%", Subject: "pid=1", Samples: []model.Sample{
				{Timestamp: ts(tsOffset), Measurements: map[string]float64{"cpu_percent": base}},
				{Timestamp: ts(tsOffset + 1), Measurements: map[string]float64{"cpu_percent": base + 1}},
			}}
		}
		return &model.TopData{
			ProcessSamples: []model.ProcessSample{
				{Timestamp: ts(tsOffset), PID: 1, Command: "mysqld", CPUPercent: 75},
				{Timestamp: ts(tsOffset + 1), PID: 1, Command: "mysqld", CPUPercent: 76},
			},
			Top3ByAverage:      []model.ProcessSeries{{PID: 1, Command: "mysqld", CPU: mkCPUSeries(75)}},
			SnapshotBoundaries: []int{0},
		}
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
		q, ok := model.NewObservedProcesslistQuery(ts(tsOffset+1), "app", "shop", "Query", "Sending data",
			"select * from orders where id = 123", 2500, true, 350, true, 25, true)
		if !ok {
			panic("test processlist query unexpectedly ineligible")
		}
		return &model.ProcesslistData{
			States: []string{"Sending data", "Sleep"},
			ThreadStateSamples: []model.ThreadStateSample{
				{
					Timestamp:             ts(tsOffset),
					StateCounts:           map[string]int{"Sending data": 2, "Sleep": 8},
					TotalThreads:          10,
					ActiveThreads:         2,
					SleepingThreads:       8,
					MaxTimeMS:             1200,
					HasTimeMetric:         true,
					MaxRowsExamined:       200,
					HasRowsExaminedMetric: true,
					MaxRowsSent:           20,
					HasRowsSentMetric:     true,
					RowsWithQueryText:     2,
					HasQueryTextMetric:    true,
				},
				{
					Timestamp:             ts(tsOffset + 1),
					StateCounts:           map[string]int{"Sending data": 3, "Sleep": 7},
					TotalThreads:          10,
					ActiveThreads:         3,
					SleepingThreads:       7,
					MaxTimeMS:             2500,
					HasTimeMetric:         true,
					MaxRowsExamined:       350,
					HasRowsExaminedMetric: true,
					MaxRowsSent:           25,
					HasRowsSentMetric:     true,
					RowsWithQueryText:     3,
					HasQueryTextMetric:    true,
				},
			},
			ObservedQueries:    []model.ObservedProcesslistQuery{q},
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
				model.SuffixTop:         {Suffix: model.SuffixTop, Parsed: top(tsOffset)},
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

func TestProcesslistPayloadIncludesRicherMetrics(t *testing.T) {
	c := twoSnapshotCollection()
	var buf bytes.Buffer
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("Render: %v", err)
	}

	var parsed struct {
		Charts struct {
			Processlist struct {
				SlowQueries []struct {
					Fingerprint     string  `json:"fingerprint"`
					Snippet         string  `json:"snippet"`
					SeenSamples     int     `json:"seenSamples"`
					MaxTimeSeconds  float64 `json:"maxTimeSeconds"`
					HasTimeMetric   bool    `json:"hasTimeMetric"`
					MaxRowsExamined float64 `json:"maxRowsExamined"`
					HasRowsExamined bool    `json:"hasRowsExamined"`
					MaxRowsSent     float64 `json:"maxRowsSent"`
					HasRowsSent     bool    `json:"hasRowsSent"`
					User            string  `json:"user"`
					DB              string  `json:"db"`
					Command         string  `json:"command"`
					State           string  `json:"state"`
				} `json:"slowQueries"`
				Dimensions []struct {
					Key    string `json:"key"`
					Label  string `json:"label"`
					Series []struct {
						Label  string    `json:"label"`
						Values []float64 `json:"values"`
					} `json:"series"`
				} `json:"dimensions"`
				Metrics struct {
					MaxTimeSeconds    []float64 `json:"maxTimeSeconds"`
					HasMaxTimeSeconds []bool    `json:"hasMaxTimeSeconds"`
					MaxRowsExamined   []float64 `json:"maxRowsExamined"`
					HasRowsExamined   []bool    `json:"hasRowsExamined"`
					MaxRowsSent       []float64 `json:"maxRowsSent"`
					HasRowsSent       []bool    `json:"hasRowsSent"`
					RowsWithQueryText []float64 `json:"rowsWithQueryText"`
					HasQueryText      []bool    `json:"hasQueryText"`
				} `json:"metrics"`
			} `json:"processlist"`
		} `json:"charts"`
	}
	if err := json.Unmarshal([]byte(extractJSONPayload(t, buf.String())), &parsed); err != nil {
		t.Fatalf("report-data payload is not valid JSON: %v", err)
	}

	foundActivity := false
	for _, dim := range parsed.Charts.Processlist.Dimensions {
		if dim.Key != "activity" {
			continue
		}
		foundActivity = true
		if got, want := len(dim.Series), 3; got != want {
			t.Fatalf("activity series len = %d, want %d", got, want)
		}
		if dim.Series[0].Label != "Active" || dim.Series[1].Label != "Sleeping" || dim.Series[2].Label != "Total" {
			t.Fatalf("unexpected activity labels: %#v", dim.Series)
		}
		if got, want := dim.Series[0].Values[0], 2.0; got != want {
			t.Errorf("first active value = %v, want %v", got, want)
		}
	}
	if !foundActivity {
		t.Fatal("processlist payload missing activity dimension")
	}
	if got, want := parsed.Charts.Processlist.Metrics.MaxTimeSeconds[1], 2.5; got != want {
		t.Errorf("MaxTimeSeconds[1] = %v, want %v", got, want)
	}
	if got, want := parsed.Charts.Processlist.Metrics.HasMaxTimeSeconds[1], true; got != want {
		t.Errorf("HasMaxTimeSeconds[1] = %v, want %v", got, want)
	}
	if got, want := parsed.Charts.Processlist.Metrics.MaxRowsExamined[1], 350.0; got != want {
		t.Errorf("MaxRowsExamined[1] = %v, want %v", got, want)
	}
	if got, want := parsed.Charts.Processlist.Metrics.HasRowsExamined[1], true; got != want {
		t.Errorf("HasRowsExamined[1] = %v, want %v", got, want)
	}
	if got, want := parsed.Charts.Processlist.Metrics.RowsWithQueryText[1], 3.0; got != want {
		t.Errorf("RowsWithQueryText[1] = %v, want %v", got, want)
	}
	if got, want := parsed.Charts.Processlist.Metrics.HasQueryText[1], true; got != want {
		t.Errorf("HasQueryText[1] = %v, want %v", got, want)
	}
	if got, want := len(parsed.Charts.Processlist.SlowQueries), 1; got != want {
		t.Fatalf("SlowQueries len = %d, want %d", got, want)
	}
	slow := parsed.Charts.Processlist.SlowQueries[0]
	if slow.Fingerprint == "" {
		t.Error("SlowQueries[0].Fingerprint is empty")
	}
	if !strings.Contains(slow.Snippet, "select * from orders") {
		t.Errorf("SlowQueries[0].Snippet = %q, want orders query", slow.Snippet)
	}
	if got, want := slow.MaxTimeSeconds, 2.5; got != want {
		t.Errorf("SlowQueries[0].MaxTimeSeconds = %v, want %v", got, want)
	}
	if got, want := slow.MaxRowsExamined, 350.0; got != want {
		t.Errorf("SlowQueries[0].MaxRowsExamined = %v, want %v", got, want)
	}
}

func TestProcesslistSummaryOmitsUnavailableOptionalMetrics(t *testing.T) {
	ts := time.Date(2026, 4, 21, 16, 52, 0, 0, time.UTC)
	c := &model.Collection{
		RootPath: "/tmp/example",
		Hostname: "example-db-01",
		Snapshots: []*model.Snapshot{
			{
				Timestamp: ts,
				Prefix:    "2026_04_21_16_52_00",
				SourceFiles: map[model.Suffix]*model.SourceFile{
					model.SuffixProcesslist: {
						Suffix: model.SuffixProcesslist,
						Parsed: &model.ProcesslistData{
							States: []string{"Sleep"},
							ThreadStateSamples: []model.ThreadStateSample{
								{
									Timestamp:       ts,
									StateCounts:     map[string]int{"Sleep": 4},
									TotalThreads:    4,
									ActiveThreads:   0,
									SleepingThreads: 4,
								},
							},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("Render: %v", err)
	}
	section := extractDetailsSection(t, buf.String(), "sec-db")

	for _, want := range []string{"peak active", "peak sleeping", "samples"} {
		if !strings.Contains(section, want) {
			t.Errorf("sec-db processlist markup missing available callout %q", want)
		}
	}
	for _, notWant := range []string{"longest age", "peak rows examined", "peak rows sent", "query text rows"} {
		if strings.Contains(section, notWant) {
			t.Errorf("sec-db processlist markup contains unavailable callout %q", notWant)
		}
	}
}

// TestTwoSnapshotRenderSmokeTest: FR-018 / T054 — the render
// pipeline accepts a Collection with two Snapshots carrying every
// time-series collector and produces non-empty output without
// error. This is the sanity-check sibling of
// TestChartPayloadBoundaryShape (the structural-assertion test
// below).
func TestTwoSnapshotRenderSmokeTest(t *testing.T) {
	c := twoSnapshotCollection()
	var buf bytes.Buffer
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatalf("empty render output")
	}
}

// TestChartPayloadBoundaryShape: FR-018 / FR-030 / T054 — the
// embedded JSON chart payload for every time-series subview MUST
// expose `snapshotBoundaries` with the cumulative-start index of
// each Snapshot in the merged stream. For twoSnapshotCollection
// (2 samples in each of 2 Snapshots) every time-series chart MUST
// report `"snapshotBoundaries":[0,2]`.
func TestChartPayloadBoundaryShape(t *testing.T) {
	c := twoSnapshotCollection()
	var buf bytes.Buffer
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()

	payloadIdx := strings.Index(out, `id="report-data"`)
	if payloadIdx == -1 {
		t.Fatalf("no embedded report-data payload found")
	}
	payloadEnd := strings.Index(out[payloadIdx:], "</script>")
	if payloadEnd == -1 {
		t.Fatalf("unterminated report-data payload")
	}
	payload := out[payloadIdx : payloadIdx+payloadEnd]

	// Every time-series chart must expose the [0,2] boundary shape
	// for this fixture. Counting occurrences rather than checking
	// specific positions avoids coupling the test to the JSON
	// encoder's key-ordering.
	want := `"snapshotBoundaries":[0,2]`
	got := strings.Count(payload, want)
	if got < 5 {
		t.Errorf("expected at least 5 time-series charts to expose %q (iostat, top, vmstat, processlist, mysqladmin); got %d occurrences", want, got)
	}
}

// TestIostatDevicesAlignedAfterMerge: T054 / FR-018 — after
// concatIostat, every DeviceSeries in the merged IostatData has the
// same sample count as the primary (first-sorted) device, even when
// device sets differ across Snapshots. Guards against the
// "primary device absent in a later Snapshot" correctness bug
// Copilot P1 / Codex P1 flagged on PR #3 round 1: uPlot requires
// all series to match the x-axis length, and the chart payload
// treats `Devices[0].Utilization.Samples` as authoritative.
func TestIostatDevicesAlignedAfterMerge(t *testing.T) {
	ts := func(sec int) time.Time {
		return time.Date(2026, 4, 21, 16, 52, 11+sec, 0, time.UTC)
	}
	mkSnap := func(prefix string, tsOffset int, deviceNames ...string) *model.Snapshot {
		devices := make([]model.DeviceSeries, 0, len(deviceNames))
		for _, n := range deviceNames {
			devices = append(devices, model.DeviceSeries{
				Device: n,
				Utilization: model.MetricSeries{Metric: "util_percent", Unit: "%", Subject: n, Samples: []model.Sample{
					{Timestamp: ts(tsOffset), Measurements: map[string]float64{"util_percent": 10}},
					{Timestamp: ts(tsOffset + 1), Measurements: map[string]float64{"util_percent": 11}},
				}},
				AvgQueueSize: model.MetricSeries{Metric: "avgqu_sz", Unit: "count", Subject: n, Samples: []model.Sample{
					{Timestamp: ts(tsOffset), Measurements: map[string]float64{"avgqu_sz": 0.5}},
					{Timestamp: ts(tsOffset + 1), Measurements: map[string]float64{"avgqu_sz": 0.6}},
				}},
			})
		}
		return &model.Snapshot{
			Timestamp: ts(tsOffset),
			Prefix:    prefix,
			SourceFiles: map[model.Suffix]*model.SourceFile{
				model.SuffixIostat: {Suffix: model.SuffixIostat, Parsed: &model.IostatData{Devices: devices, SnapshotBoundaries: []int{0}}},
			},
		}
	}
	// Snap1 has only sdb; Snap2 has only sda. The merged output's
	// primary-by-name is "sda" (alphabetical), which is ABSENT in
	// snap1 — the exact shape of the regression the reviewers
	// flagged. A buggy concat would emit boundaries [0,0] because
	// cumulative only advanced when the primary device was in the
	// current Snapshot.
	c := &model.Collection{
		RootPath:  "/tmp/example",
		Hostname:  "example-db-01",
		Snapshots: []*model.Snapshot{mkSnap("snap1", 0, "sdb"), mkSnap("snap2", 30, "sda")},
	}
	var buf bytes.Buffer
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()
	payloadIdx := strings.Index(out, `id="report-data"`)
	payloadEnd := strings.Index(out[payloadIdx:], "</script>")
	payload := out[payloadIdx : payloadIdx+payloadEnd]

	// iostat boundaries advance to 2 even though the primary device
	// (sda) was absent in snap1.
	if !strings.Contains(payload, `"snapshotBoundaries":[0,2]`) {
		t.Errorf("iostat boundaries do not advance when the primary device is absent in the first Snapshot; the payload must carry [0,2] for this 2×2 fixture. Payload excerpt: %s", sliceAround(payload, "snapshotBoundaries", 80))
	}
	// Absent-Snapshot samples are NaN-padded → JSON null in the
	// payload. With two devices × two Snapshots × two metrics
	// (util_percent, avgqu_sz) and half the cells absent, we expect
	// a healthy count of nulls. A too-low count means alignment
	// regressed.
	if strings.Count(payload, `null`) < 4 {
		t.Errorf("expected at least 4 NaN-padded null values in iostat payload when device sets diverge across Snapshots; payload excerpt: %s", sliceAround(payload, "sda", 120))
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
