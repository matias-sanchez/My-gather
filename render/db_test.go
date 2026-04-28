package render_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/render"
	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// TestDBGoldenHTML: T069 / FR-014..017 / FR-028..030 / Principle VIII.
//
// Render a Collection populated by parse.Discover but filtered to
// expose only the DB-section parsers (innodbstatus, mysqladmin,
// processlist). Extract the top-level `<details id="sec-db">…</details>`
// block and snapshot-compare against a committed golden fragment.
// Covers:
//
//   - innodbstatus scalar panels (FR-014);
//   - mysqladmin counter / gauge charts (FR-015, FR-028, FR-030);
//   - processlist thread-state stacked chart (FR-017).
//
// Multi-snapshot merge behaviours are exercised end-to-end because
// the example2 fixture ships two Snapshots.
func TestDBGoldenHTML(t *testing.T) {
	root := goldens.RepoRoot(t)
	goldenPath := filepath.Join(root, "testdata", "golden", "db.example2.html")

	html := renderGolden(t, model.SuffixInnodbStatus, model.SuffixMysqladmin, model.SuffixProcesslist)
	section := extractDetailsSection(t, html, "sec-db")

	goldens.Compare(t, goldenPath, []byte(section))
}

// TestMysqladminToggleMarkup: T070 / FR-015.
//
// FR-015 requires the mysqladmin view to expose a keyboard-accessible
// multi-select toggle over every counter variable. The current
// implementation (per F26 resolution, tasks T076 and FR-034) ships a
// two-layer design:
//
//   - a static server-rendered container: a `<div class="chart"
//     id="chart-mysqladmin" data-chart="mysqladmin">` host and a
//     `data-mysqladmin-host` wrapper that the JS in
//     render/assets/app.js latches onto on DOMContentLoaded;
//   - a JSON payload under `<script id="report-data">` that carries
//     every `MysqladminData.VariableNames` entry plus the counter
//     deltas. The toggle widget is built client-side from that
//     payload — the server-rendered HTML does NOT enumerate each
//     variable name in the DOM.
//
// Asserting a per-variable `data-variable-name=…` attribute against
// the static HTML would pin an aspirational contract instead of the
// shipped one. Instead this test pins what actually has to be true
// for the toggle to work:
//
//  1. The sec-db block contains the `id="chart-mysqladmin"` host and
//     a `data-mysqladmin-host` wrapper the JS can latch onto;
//  2. Every `MysqladminData.VariableNames` entry from the parsed
//     fixture appears at least once in the JSON payload (so the
//     toggle has a complete list to render);
//  3. The JSON payload is valid JSON (so the client-side widget can
//     actually parse it);
//  4. No interactive element inside sec-db carries a network URL
//     (FR-015 aligns with Principle IX / FR-022).
func TestMysqladminToggleMarkup(t *testing.T) {
	// Full discover so we can read MysqladminData.VariableNames; the
	// render is still filtered to only the DB-section parsers so
	// surrounding sections stay minimal and the section extractor
	// finds a clean sec-db block.
	full := discoverExample2(t)
	var names []string
	for _, s := range full.Snapshots {
		sf := s.SourceFiles[model.SuffixMysqladmin]
		if sf == nil || sf.Parsed == nil {
			continue
		}
		d, ok := sf.Parsed.(*model.MysqladminData)
		if !ok || d == nil {
			continue
		}
		names = append(names, d.VariableNames...)
	}
	if len(names) == 0 {
		t.Fatalf("mysqladmin fixture produced zero VariableNames; cannot exercise FR-015 toggle markup")
	}
	seen := map[string]struct{}{}
	var unique []string
	for _, n := range names {
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		unique = append(unique, n)
	}

	html := renderGolden(t, model.SuffixInnodbStatus, model.SuffixMysqladmin, model.SuffixProcesslist)
	section := extractDetailsSection(t, html, "sec-db")

	// Invariant 1: the static host the JS widget latches onto exists.
	// id="chart-mysqladmin" is the anchor; data-mysqladmin-host is
	// the wrapper the toggle scoped its DOM manipulation to.
	if !strings.Contains(section, `id="chart-mysqladmin"`) {
		t.Errorf(`sec-db missing id="chart-mysqladmin" host; FR-015 widget binds to this id`)
	}
	if !strings.Contains(section, `data-chart="mysqladmin"`) {
		t.Errorf(`sec-db missing data-chart="mysqladmin"; the dispatch table in app.js keys on this attribute`)
	}
	if !strings.Contains(section, `data-mysqladmin-host`) {
		t.Errorf(`sec-db missing data-mysqladmin-host wrapper; the toggle scopes its DOM surgery to this host`)
	}

	// Invariant 2: every variable name appears in the
	// `charts.mysqladmin.variables` array of the JSON payload — NOT
	// just anywhere in the raw payload text. The toggle widget reads
	// its dropdown options from exactly this field (app.js comment on
	// mysqladminChartPayload: "app.js reads { variables, timestamps,
	// deltas, … }"). A raw-text Contains check would pass even if
	// `variables` were empty as long as the names happened to appear
	// as `deltas` map keys or inside the category metadata — in which
	// case the client-side selector list breaks in the browser while
	// this test stays green.
	payload := extractJSONPayload(t, html)
	// Invariant 3a: payload is valid JSON (precondition for the
	// structural assertion below). An invalid payload would
	// otherwise surface as an opaque `charts` lookup failure.
	var parsed struct {
		Charts struct {
			Mysqladmin struct {
				Variables []string `json:"variables"`
			} `json:"mysqladmin"`
		} `json:"charts"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("report-data payload is not valid JSON: %v", err)
	}
	// Build a set for O(1) membership lookup, then diff against
	// `unique` so the error message lists the missing names directly.
	varsInPayload := make(map[string]struct{}, len(parsed.Charts.Mysqladmin.Variables))
	for _, v := range parsed.Charts.Mysqladmin.Variables {
		varsInPayload[v] = struct{}{}
	}
	missingInPayload := []string{}
	for _, n := range unique {
		if _, ok := varsInPayload[n]; !ok {
			missingInPayload = append(missingInPayload, n)
		}
	}
	if len(missingInPayload) > 0 {
		report := missingInPayload
		if len(report) > 10 {
			report = report[:10]
		}
		t.Errorf("charts.mysqladmin.variables is missing %d/%d MysqladminData.VariableNames entries (the toggle reads this array — a raw-substring Contains check would have passed on deltas keys and missed this). First 10 missing: %v",
			len(missingInPayload), len(unique), report)
	}

	// Invariant 4: offline operation. Scan sec-db for src=/href=
	// pointing at http(s). The forbidden-import linter enforces
	// Principle IX at the Go level; this is the runtime-output
	// equivalent.
	for _, probe := range []string{`src="http://`, `src="https://`, `href="http://`, `href="https://`} {
		if strings.Contains(section, probe) {
			t.Errorf("sec-db contains a network URL (%q) on an interactive element; FR-015 / Principle IX require offline operation", probe)
		}
	}
}

func TestProcesslistRicherMetricMarkup(t *testing.T) {
	html := renderGolden(t, model.SuffixInnodbStatus, model.SuffixMysqladmin, model.SuffixProcesslist)
	section := extractDetailsSection(t, html, "sec-db")

	for _, want := range []string{
		`peak active`,
		`peak sleeping`,
		`longest age`,
		`peak rows examined`,
		`peak rows sent`,
		`query text rows`,
	} {
		if !strings.Contains(section, want) {
			t.Errorf("sec-db processlist markup missing %q", want)
		}
	}
}

func TestProcesslistSlowestObservedQueriesMarkup(t *testing.T) {
	ts := time.Date(2026, 1, 29, 16, 5, 19, 0, time.UTC)
	q, ok := model.NewObservedProcesslistQuery(ts, "horizon", "horizon", "Query", "Waiting for table metadata lock",
		"select count(contract0_.pk) from contract contract0_ where contract0_.pk = 123",
		3723000, true, 6114, true, 17, true)
	if !ok {
		t.Fatal("query unexpectedly ineligible")
	}
	c := &model.Collection{
		RootPath: "/tmp/example",
		Hostname: "example-db-01",
		Snapshots: []*model.Snapshot{{
			Timestamp: ts,
			Prefix:    "2026_01_29_16_05_19",
			SourceFiles: map[model.Suffix]*model.SourceFile{
				model.SuffixProcesslist: {
					Suffix: model.SuffixProcesslist,
					Parsed: &model.ProcesslistData{
						States: []string{"Waiting for table metadata lock"},
						ThreadStateSamples: []model.ThreadStateSample{{
							Timestamp:     ts,
							StateCounts:   map[string]int{"Waiting for table metadata lock": 1},
							TotalThreads:  1,
							ActiveThreads: 1,
						}},
						ObservedQueries: []model.ObservedProcesslistQuery{q},
					},
				},
			},
		}},
	}

	var buf bytes.Buffer
	if err := render.Render(&buf, c, render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	section := extractDetailsSection(t, buf.String(), "sec-db")
	for _, want := range []string{
		"<summary><span>Slowest observed queries</span><span class=\"badge\">1 query</span></summary>",
		`class="slow-query-search"`,
		`placeholder="Search query or fingerprint"`,
		`class="slow-query-filter"`,
		`data-filter-field="user"`,
		`data-filter-field="db"`,
		`data-filter-field="state"`,
		`class="slow-query-count"`,
		`data-slow-query-empty`,
		`data-query-user="horizon"`,
		`data-query-db="horizon"`,
		`data-query-state="waiting for table metadata lock"`,
		`data-query-text="select count(contract0_.pk) from contract contract0_ where contract0_.pk = ? ` + q.Fingerprint + `"`,
		"Waiting for table metadata lock",
		"select count(contract0_.pk)",
		q.Fingerprint,
		"3723.0 s",
		"6114",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("sec-db slowest-query markup missing %q", want)
		}
	}
}

func TestProcesslistSlowestObservedQueriesEmptyState(t *testing.T) {
	ts := time.Date(2026, 1, 29, 16, 5, 19, 0, time.UTC)
	c := &model.Collection{
		RootPath: "/tmp/example",
		Hostname: "example-db-01",
		Snapshots: []*model.Snapshot{{
			Timestamp: ts,
			Prefix:    "2026_01_29_16_05_19",
			SourceFiles: map[model.Suffix]*model.SourceFile{
				model.SuffixProcesslist: {
					Suffix: model.SuffixProcesslist,
					Parsed: &model.ProcesslistData{
						States: []string{"Sleep"},
						ThreadStateSamples: []model.ThreadStateSample{{
							Timestamp:       ts,
							StateCounts:     map[string]int{"Sleep": 4},
							TotalThreads:    4,
							SleepingThreads: 4,
						}},
					},
				},
			},
		}},
	}

	var buf bytes.Buffer
	if err := render.Render(&buf, c, render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	section := extractDetailsSection(t, buf.String(), "sec-db")
	if !strings.Contains(section, "No active query text was observed in processlist rows.") {
		t.Errorf("sec-db missing slowest-query empty state")
	}
}

func TestProcesslistSlowestObservedQueriesMissingAge(t *testing.T) {
	ts := time.Date(2026, 1, 29, 16, 5, 19, 0, time.UTC)
	q, ok := model.NewObservedProcesslistQuery(ts, "app", "shop", "Query", "Sending data",
		"select * from orders", 0, false, 0, false, 0, false)
	if !ok {
		t.Fatal("query unexpectedly ineligible")
	}
	c := &model.Collection{
		RootPath: "/tmp/example",
		Hostname: "example-db-01",
		Snapshots: []*model.Snapshot{{
			Timestamp: ts,
			Prefix:    "2026_01_29_16_05_19",
			SourceFiles: map[model.Suffix]*model.SourceFile{
				model.SuffixProcesslist: {
					Suffix: model.SuffixProcesslist,
					Parsed: &model.ProcesslistData{
						States: []string{"Sending data"},
						ThreadStateSamples: []model.ThreadStateSample{{
							Timestamp:     ts,
							StateCounts:   map[string]int{"Sending data": 1},
							TotalThreads:  1,
							ActiveThreads: 1,
						}},
						ObservedQueries: []model.ObservedProcesslistQuery{q},
					},
				},
			},
		}},
	}

	var buf bytes.Buffer
	if err := render.Render(&buf, c, render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	section := extractDetailsSection(t, buf.String(), "sec-db")
	if strings.Contains(section, "0.0 s") {
		t.Fatalf("missing query age rendered as 0.0 s")
	}
	if !strings.Contains(section, `<td class="mono">-</td>`) {
		t.Fatalf("missing query age did not render as dash")
	}
}
