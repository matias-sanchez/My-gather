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
// via a search input. The real contract the current template emits
// (see render/templates/variables.html.tmpl) is:
//
//   - one `<input type="search" class="variables-search" …>` per
//     Variables subview, carrying a `data-snapshot` attribute that
//     the client-side filter in render/assets/app.js pairs with the
//     matching `<table class="variables-table" data-snapshot=…>`;
//   - every data row MUST carry a `data-variable-name` attribute so
//     the filter can match/hide rows by variable name without
//     re-parsing the rendered cell content.
//
// We do NOT require an `id=` attribute on the search input: the JS
// filter binds via the `class + data-snapshot` pair, which is the
// shipped mechanism. Asserting an id would pin an aspirational
// contract instead of the real one.
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

	// The search input must also carry a binding hook the JS filter
	// can pair with its matching table: today that's
	// `class="variables-search"` + `data-snapshot="<id>"`. Accept
	// either attribute (or an id) — the invariant is "SOMETHING
	// addressable exists", not a specific scheme, so a template
	// refactor that swaps the mechanism stays green as long as it
	// still exposes a hook.
	classBoundRE := regexp.MustCompile(`<input[^>]*type="search"[^>]*class="[^"]*variables-search[^"]*"|<input[^>]*class="[^"]*variables-search[^"]*"[^>]*type="search"`)
	dataBoundRE := regexp.MustCompile(`<input[^>]*type="search"[^>]*data-snapshot="[^"]+"|<input[^>]*data-snapshot="[^"]+"[^>]*type="search"`)
	idBoundRE := regexp.MustCompile(`<input[^>]*type="search"[^>]*id="[^"]+"|<input[^>]*id="[^"]+"[^>]*type="search"`)
	if !(classBoundRE.MatchString(section) || dataBoundRE.MatchString(section) || idBoundRE.MatchString(section)) {
		t.Errorf(`<input type="search"> must carry a binding hook (class="variables-search", data-snapshot="…", or id="…") for the client-side filter to bind to; sec-variables excerpt: %s`,
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
