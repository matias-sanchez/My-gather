package render_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/model"
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
