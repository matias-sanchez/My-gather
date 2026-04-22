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

// TestMysqladminCategoryMap: T100 / FR-034 / Principle VIII.
//
// FR-034 requires the mysqladmin view to surface a category chooser
// so users can focus on related counters without scanning a
// several-hundred-entry dropdown. The resolution runs at render
// time in `classifyMysqladminCategory` (render/render.go):
//
//   1. A variable's explicit `members` list wins over any matcher.
//   2. A matcher hit is kept UNLESS an `exclude_matchers` entry also
//      matches (e.g., "buffer-pool" matches `innodb_buffer_pool_`
//      but explicitly excludes `innodb_buffer_pool_read*` /
//      `_write*` so those counters land in InnoDB Reads / Writes
//      instead).
//   3. The ordered `categories` array in the JSON payload preserves
//      the order declared in render/assets/mysqladmin-categories.json
//      so the UI can render the chooser without re-sorting.
//
// This test drives the rendered JSON payload against a synthetic
// MysqladminData that covers all three resolution paths
// deterministically. Using a synthetic fixture (rather than example2)
// pins behaviour independent of fixture refreshes.
func TestMysqladminCategoryMap(t *testing.T) {
	// Three carefully chosen variable names:
	//
	//   - "Innodb_buffer_pool_size" hits the "buffer-pool" matcher
	//     (`innodb_buffer_pool_` prefix), is NOT excluded, and does
	//     NOT appear in any other category's explicit `members`
	//     list. It should classify as a single-category "buffer-pool"
	//     membership.
	//   - "Innodb_buffer_pool_read_requests" hits the "buffer-pool"
	//     matcher AND the "innodb-reads" matcher; "buffer-pool"
	//     excludes it via `innodb_buffer_pool_read`, so the only
	//     category it lands in is "innodb-reads".
	//   - "Innodb_rows_inserted" does not match any matcher prefix
	//     but is listed explicitly in "innodb-writes"'s `members`
	//     array; per rule 1 it classifies as "innodb-writes".
	matcherHit := "Innodb_buffer_pool_size"
	matcherHitExcluded := "Innodb_buffer_pool_read_requests"
	memberOverride := "Innodb_rows_inserted"

	md := &model.MysqladminData{
		VariableNames: []string{matcherHit, matcherHitExcluded, memberOverride},
		SampleCount:   1,
		Timestamps:    []time.Time{fixedTime()},
		Deltas: map[string][]float64{
			matcherHit:         {100},
			matcherHitExcluded: {200},
			memberOverride:     {300},
		},
		IsCounter: map[string]bool{
			matcherHit:         true,
			matcherHitExcluded: true,
			memberOverride:     true,
		},
		SnapshotBoundaries: []int{0},
	}

	c := &model.Collection{
		RootPath: "/tmp/fr034",
		Hostname: "example-db-01",
		Snapshots: []*model.Snapshot{{
			Timestamp: fixedTime(),
			Prefix:    "snap",
			SourceFiles: map[model.Suffix]*model.SourceFile{
				model.SuffixMysqladmin: {Suffix: model.SuffixMysqladmin, Parsed: md},
			},
		}},
	}

	var buf bytes.Buffer
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("Render: %v", err)
	}
	payload := extractJSONPayload(t, buf.String())

	var parsed struct {
		Charts struct {
			Mysqladmin struct {
				Variables   []string            `json:"variables"`
				Categories  []map[string]any    `json:"categories"`
				CategoryMap map[string][]string `json:"categoryMap"`
			} `json:"mysqladmin"`
		} `json:"charts"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("report-data payload is not valid JSON: %v", err)
	}
	m := parsed.Charts.Mysqladmin

	// Invariant 1: each of the three fixture variables resolves to
	// exactly the expected categories. Membership set comparison is
	// order-independent and tolerates a variable being in multiple
	// categories in principle.
	expected := map[string][]string{
		matcherHit:         {"buffer-pool"},
		matcherHitExcluded: {"innodb-reads"},
		memberOverride:     {"innodb-writes"},
	}
	for name, wantCats := range expected {
		got, ok := m.CategoryMap[name]
		if !ok {
			t.Errorf("categoryMap missing entry for %q; expected membership in %v", name, wantCats)
			continue
		}
		if !stringSliceEqualsIgnoringOrder(got, wantCats) {
			t.Errorf("categoryMap[%q] = %v; want %v (matchers + members + excludes must resolve to this exact set)",
				name, got, wantCats)
		}
	}

	// Invariant 2: categories list preserves the declared order from
	// render/assets/mysqladmin-categories.json. Assert the first few
	// keys land in the order the JSON file emits so a silent sort
	// refactor gets caught.
	if len(m.Categories) < 3 {
		t.Fatalf("categories list has %d entries; expected at least 3 (buffer-pool, innodb-reads, innodb-writes)", len(m.Categories))
	}
	wantOrder := []string{"buffer-pool", "innodb-reads", "innodb-writes"}
	for i, want := range wantOrder {
		got, _ := m.Categories[i]["key"].(string)
		if got != want {
			t.Errorf("categories[%d].key = %q; want %q (declared order in mysqladmin-categories.json must survive to the payload)",
				i, got, want)
		}
	}
}

// stringSliceEqualsIgnoringOrder returns true if a and b contain the
// same elements regardless of order. Used by TestMysqladminCategoryMap
// to compare category-membership slices without coupling the test to
// whichever order the resolver happens to emit.
func stringSliceEqualsIgnoringOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
	}
	for _, v := range seen {
		if v != 0 {
			return false
		}
	}
	return true
}

// TestDefaultVisibleSkipsInitialTally: T102 / FR-035 / Principle VIII.
//
// FR-035 requires the mysqladmin chart to boot with a curated set of
// counters visible (the defaultVisible list) while still shipping the
// raw initial-tally samples in the payload so the UI can restore them
// on request. The shipped behaviour in render/render.go:
//
//   - `pickDefaultCounters(d)` returns a handful of well-known
//     counter names (Com_select, Questions, …). These are variable
//     names, NOT a "counter_initial" pseudo-variant — the UI hides
//     the initial-tally VIEW by default, not the data.
//   - `mysqladminChartPayload` still emits every `Deltas[v]` slice
//     in full, including `Deltas[v][0]` which for counters holds the
//     raw initial value (per pt-mext semantics + our classifier).
//
// This test drives a synthetic fixture with three counters and
// asserts both halves:
//
//   1. `defaultVisible` is a subset of `variables` and contains only
//      counter names (no gauge, no "*_initial"-style pseudo entry).
//   2. Every counter's `deltas[v][0]` is present and not null so a
//      user toggling the initial-tally view sees real data.
func TestDefaultVisibleSkipsInitialTally(t *testing.T) {
	// Build a fixture with the well-known counters pickDefaultCounters
	// looks for, plus a gauge, so we can check that defaultVisible
	// excludes the gauge.
	counters := []string{"Com_select", "Com_insert", "Questions", "Bytes_received"}
	gauge := "Threads_running"
	names := append([]string{}, counters...)
	names = append(names, gauge)

	deltas := map[string][]float64{}
	isCounter := map[string]bool{}
	for _, n := range counters {
		deltas[n] = []float64{1000, 10, 12} // raw initial + two deltas
		isCounter[n] = true
	}
	deltas[gauge] = []float64{5, 6, 7}
	isCounter[gauge] = false

	md := &model.MysqladminData{
		VariableNames:      sortStrings(names),
		SampleCount:        3,
		Timestamps:         []time.Time{fixedTime(), fixedTime().Add(1), fixedTime().Add(2)},
		Deltas:             deltas,
		IsCounter:          isCounter,
		SnapshotBoundaries: []int{0},
	}

	c := &model.Collection{
		RootPath: "/tmp/fr035",
		Hostname: "example-db-01",
		Snapshots: []*model.Snapshot{{
			Timestamp: fixedTime(),
			Prefix:    "snap",
			SourceFiles: map[model.Suffix]*model.SourceFile{
				model.SuffixMysqladmin: {Suffix: model.SuffixMysqladmin, Parsed: md},
			},
		}},
	}

	var buf bytes.Buffer
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("Render: %v", err)
	}
	payload := extractJSONPayload(t, buf.String())

	var parsed struct {
		Charts struct {
			Mysqladmin struct {
				Variables      []string             `json:"variables"`
				Deltas         map[string][]*float64 `json:"deltas"`
				IsCounter      map[string]bool      `json:"isCounter"`
				DefaultVisible []string             `json:"defaultVisible"`
			} `json:"mysqladmin"`
		} `json:"charts"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("report-data payload is not valid JSON: %v", err)
	}
	m := parsed.Charts.Mysqladmin

	// Invariant 1a: defaultVisible ⊆ variables. A defaultVisible
	// entry that isn't in `variables` is a broken UI dropdown.
	variables := map[string]struct{}{}
	for _, v := range m.Variables {
		variables[v] = struct{}{}
	}
	for _, v := range m.DefaultVisible {
		if _, ok := variables[v]; !ok {
			t.Errorf("defaultVisible entry %q is not in the variables list; UI would show a ghost series", v)
		}
	}

	// Invariant 1b: defaultVisible contains only counter names. A
	// gauge leaked in would produce a chart line from a gauge's raw
	// reading — meaningless next to counter deltas.
	for _, v := range m.DefaultVisible {
		if !m.IsCounter[v] {
			t.Errorf("defaultVisible includes gauge %q; FR-035 expects counters only (gauges belong to the raw-value chart family)", v)
		}
	}

	// Invariant 1c: no "initial_tally" / "*_initial" pseudo-variants
	// in defaultVisible. The UI hides the initial-tally VIEW by
	// default; it does NOT represent that view as a fake variable
	// name. A regression that did would break the toggle's
	// round-trip.
	for _, v := range m.DefaultVisible {
		if strings.Contains(strings.ToLower(v), "initial") {
			t.Errorf("defaultVisible contains a suspicious pseudo-name %q; FR-035 stores the initial-tally data inside each counter's Deltas[0] rather than as a separate variable", v)
		}
	}

	// Invariant 2: every counter's Deltas[0] is present and not null
	// in the payload. The initial tally is the FIRST value in each
	// counter's deltas slice; if it got scrubbed, the UI can't
	// restore the initial-tally view even though defaultVisible
	// hides it.
	for _, v := range counters {
		slots, ok := m.Deltas[v]
		if !ok {
			t.Errorf("deltas missing entry for counter %q; the payload lost the raw per-sample values", v)
			continue
		}
		if len(slots) == 0 {
			t.Errorf("deltas[%q] is empty; expected SampleCount=3 values", v)
			continue
		}
		if slots[0] == nil {
			t.Errorf("deltas[%q][0] is null in the payload; FR-035 keeps the raw initial tally so the UI can restore the initial-tally view on request", v)
		}
	}
}

// sortStrings returns a sorted copy of its input (so callers can
// build a VariableNames slice in arbitrary order and still satisfy
// MysqladminData's "strictly sorted" invariant). Kept inline rather
// than imported from sort because one line is smaller than an import.
func sortStrings(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
