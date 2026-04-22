package render_test

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/render"
	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// TestVariablesGoldenHTML: T057 / FR-012 / FR-013 / Principle VIII.
//
// Render a Collection populated by parse.Discover but filtered to
// expose only the Variables parser. Extract the top-level
// `<details id="sec-variables">…</details>` block and snapshot-
// compare against a committed golden fragment. Any change in the
// Variables table's layout, badges (default / modified per FR-033),
// or search-input wiring shows up as a reviewable diff.
func TestVariablesGoldenHTML(t *testing.T) {
	root := goldens.RepoRoot(t)
	goldenPath := filepath.Join(root, "testdata", "golden", "variables.example2.html")

	html := renderGolden(t, model.SuffixVariables)
	section := extractDetailsSection(t, html, "sec-variables")

	goldens.Compare(t, goldenPath, []byte(section))
}

// TestVariablesSearchMarkup: T058 / FR-013.
//
// FR-013 requires the Variables table to be filterable client-side
// via a search input. The shipped JS in `render/assets/app.js`
// binds with `document.querySelectorAll("input.variables-search[data-snapshot]")`,
// so the runtime contract is `class="variables-search"` AND
// `data-snapshot="<id>"` on the SAME input element. An input with
// only one of the two attributes would not be picked up by
// `initVariablesSearch`, even though it might look reasonable in a
// static inspection.
//
// Per-row invariant: every data row MUST carry a
// `data-variable-name` attribute so the filter can match/hide rows
// by variable name without re-parsing the rendered cell content.
//
// Structural invariant (not a value assertion) — sits alongside
// TestVariablesGoldenHTML rather than inside the golden so a
// regression surfaces with a direct message instead of a confusing
// table-shape diff.
func TestVariablesSearchMarkup(t *testing.T) {
	html := renderGolden(t, model.SuffixVariables)
	section := extractDetailsSection(t, html, "sec-variables")

	// Search input presence. The `type="search"` attribute activates
	// the UA-provided clear button and inputmode defaults; regressing
	// to `type="text"` would still filter but lose the FR-013
	// affordances.
	searchRE := regexp.MustCompile(`<input[^>]*type="search"[^>]*>`)
	if !searchRE.MatchString(section) {
		t.Errorf(`sec-variables missing <input type="search">; FR-013 requires a client-side filter input`)
	}

	// The binding contract from app.js is
	// `input.variables-search[data-snapshot]` — both attributes on
	// the same element. Check each candidate `<input …>` tag for BOTH
	// attributes rather than scanning the document for either: a
	// regression that keeps the class but drops data-snapshot (or
	// vice-versa) would break the filter in the browser silently.
	inputTagRE := regexp.MustCompile(`<input[^>]*>`)
	tags := inputTagRE.FindAllString(section, -1)
	var matched []string
	for _, tag := range tags {
		if !strings.Contains(tag, `type="search"`) {
			continue
		}
		hasClass := strings.Contains(tag, `class="variables-search"`) ||
			// Allow a multi-class attribute string that contains the
			// token; CSS class matching in the DOM is
			// whitespace-delimited. A future template refactor that
			// mixes classes ("variables-search filterable") is still
			// acceptable as long as the token is present.
			regexp.MustCompile(`class="[^"]*\bvariables-search\b[^"]*"`).MatchString(tag)
		hasSnapshot := regexp.MustCompile(`data-snapshot="[^"]+"`).MatchString(tag)
		if hasClass && hasSnapshot {
			matched = append(matched, tag)
		}
	}
	if len(matched) == 0 {
		t.Errorf(`no <input type="search"> carries BOTH class="variables-search" AND data-snapshot="…" on the same tag; app.js's selector "input.variables-search[data-snapshot]" will not bind. sec-variables excerpt: %s`,
			truncateForError(section, 400))
	}

	// Every data row MUST have a data-variable-name attribute. The
	// example2 fixture carries thousands of variables; a low count
	// indicates the attribute disappeared silently.
	attrCount := strings.Count(section, "data-variable-name=")
	if attrCount < 10 {
		t.Fatalf("expected many data-variable-name attributes in sec-variables (the example2 fixture carries thousands of variables); got %d — the per-row attribute is likely missing", attrCount)
	}

	// Rough bound check: the attribute count should approximately
	// match the <tr> count under <tbody>. Allow some slack for
	// header / aria rows without the attribute. Rendering is
	// deterministic, so an off-by-a-lot indicates a real regression
	// rather than a flake.
	trCount := strings.Count(section, "<tr")
	if attrCount < trCount-5 {
		t.Errorf("sec-variables has %d <tr> rows but only %d data-variable-name attributes; every data row should carry the attribute (FR-013 client-side filter)",
			trCount, attrCount)
	}
}

// truncateForError keeps test error payloads short enough to read in a
// terminal while still pointing at the offending HTML region.
func truncateForError(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// TestDefaultsBadges: T098 / FR-033 / Principle VIII.
//
// FR-033 requires the Variables table to distinguish values that
// match the documented MySQL 8.0 default from values that have been
// changed, and to surface "unknown" when no default is documented.
// The shipped implementation classifies each row via
// `classifyVariable(defaults, name, value)` in render/render.go; the
// template emits `data-status="default" | "modified" | "unknown"`
// on each `<tr>` and a corresponding status-label cell.
//
// This test synthesises a VariablesData with three rows covering
// the three branches of the classifier, renders the document, and
// asserts the rendered HTML carries the expected `data-status` for
// each row. A regression in the classifier (e.g., `unknown` silently
// becoming `default`) would fail here with a direct message.
//
// Uses a hand-built Collection rather than example2 because example2
// has ~1,164 rows and the test needs the three specific classifier
// branches guaranteed present — impossible to promise from the
// shipped fixture without adding extra parsing.
func TestDefaultsBadges(t *testing.T) {
	// Pick variable names that the curated mysql-defaults.json is
	// known to include. `max_connections` and `innodb_buffer_pool_size`
	// are among the most common tuning knobs in that defaults map;
	// use one at its documented default and one clearly modified.
	// `DefinitelyNotAMySQLVariable_2026` has no default entry so it
	// falls through classifyVariable's `unknown` branch.
	atDefault := model.VariableEntry{Name: "max_connections", Value: "151"}
	modified := model.VariableEntry{Name: "max_connections", Value: "4096"}
	unknown := model.VariableEntry{Name: "DefinitelyNotAMySQLVariable_2026", Value: "42"}

	// Build a minimal Collection with two Snapshots: the first
	// carries VariablesData entries for atDefault + unknown, and the
	// second carries modified. `max_connections` therefore appears in
	// different snapshots so the "default" + "modified" classifier
	// paths both exercise.
	c := &model.Collection{
		RootPath: "/tmp/fr033",
		Hostname: "example-db-01",
		Snapshots: []*model.Snapshot{
			{
				Timestamp: fixedTime(),
				Prefix:    "snap-atdefault",
				SourceFiles: map[model.Suffix]*model.SourceFile{
					model.SuffixVariables: {
						Suffix: model.SuffixVariables,
						Parsed: &model.VariablesData{Entries: []model.VariableEntry{atDefault, unknown}},
					},
				},
			},
			{
				Timestamp: fixedTime().Add(30 * time.Second),
				Prefix:    "snap-modified",
				SourceFiles: map[model.Suffix]*model.SourceFile{
					model.SuffixVariables: {
						Suffix: model.SuffixVariables,
						Parsed: &model.VariablesData{Entries: []model.VariableEntry{modified}},
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
	html := buf.String()

	// max_connections appears once as "default" (value 151) and once
	// as "modified" (value 4096). Assert both statuses appear at
	// least once in the document paired with the row's name by
	// matching a single `<tr …>` that carries BOTH attributes,
	// regardless of attribute order, spacing, or unrelated
	// attributes inserted between them. A strict adjacency /
	// substring check would false-fail on a semantics-preserving
	// attribute reorder.
	rowPattern := func(variableName, status string) *regexp.Regexp {
		nameAttr := `data-variable-name="` + regexp.QuoteMeta(variableName) + `"`
		statusAttr := `data-status="` + regexp.QuoteMeta(status) + `"`
		return regexp.MustCompile(
			`<tr\b[^>]*(?:` + nameAttr + `[^>]*` + statusAttr + `|` + statusAttr + `[^>]*` + nameAttr + `)[^>]*>`,
		)
	}
	if !rowPattern("max_connections", "default").MatchString(html) {
		t.Errorf("expected a <tr> for max_connections with data-status=%q (max_connections at its documented default value 151); rendered HTML does not contain it",
			"default")
	}
	if !rowPattern("max_connections", "modified").MatchString(html) {
		t.Errorf("expected a <tr> for max_connections with data-status=%q (max_connections clearly changed from default); rendered HTML does not contain it",
			"modified")
	}

	// DefinitelyNotAMySQLVariable_2026 must carry data-status="unknown"
	// because no default is documented. A regression where "unknown"
	// silently degraded to "default" would be invisible to a casual
	// glance at the HTML but would break the FR-033 contract.
	if !rowPattern("DefinitelyNotAMySQLVariable_2026", "unknown").MatchString(html) {
		t.Errorf("expected a <tr> for DefinitelyNotAMySQLVariable_2026 with data-status=%q; rendered HTML does not contain it",
			"unknown")
	}

	// Belt-and-suspenders: the status cells render a visible label
	// ("default" / "modified" / "–") so reviewers can tell the badge
	// wiring survived. Assert both the default and modified labels
	// appear in the rendered document. The "–" unknown label is
	// weaker to check (appears in many contexts), so we rely on the
	// data-status assertion above for the unknown branch.
	if !strings.Contains(html, `<td class="status-label mono">default</td>`) {
		t.Errorf("rendered HTML missing the visible `default` status-label cell")
	}
	if !strings.Contains(html, `<td class="status-label mono">modified</td>`) {
		t.Errorf("rendered HTML missing the visible `modified` status-label cell")
	}
}
