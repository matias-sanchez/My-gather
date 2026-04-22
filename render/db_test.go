package render_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/model"
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

	// Invariant 2: every variable name appears at least once in the
	// JSON payload so the toggle has a full list to render. The
	// payload is document-wide, not section-local.
	payload := extractJSONPayload(t, html)
	missingInPayload := []string{}
	for _, n := range unique {
		if !strings.Contains(payload, `"`+n+`"`) {
			missingInPayload = append(missingInPayload, n)
		}
	}
	if len(missingInPayload) > 0 {
		report := missingInPayload
		if len(report) > 10 {
			report = report[:10]
		}
		t.Errorf("report-data payload is missing %d/%d MysqladminData.VariableNames entries; first 10 missing: %v",
			len(missingInPayload), len(unique), report)
	}

	// Invariant 3: payload is valid JSON. An invalid payload breaks
	// the chart JS silently at runtime; a failed unmarshal here
	// surfaces it loudly at test time.
	var anyVal any
	if err := json.Unmarshal([]byte(payload), &anyVal); err != nil {
		t.Fatalf("report-data payload is not valid JSON: %v", err)
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
