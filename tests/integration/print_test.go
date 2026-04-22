package integration_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/render"
)

// TestPrintStylesheetPresent: T106 / FR-037 / Principle VIII.
//
// FR-037 requires the rendered report to carry a `@media print`
// stylesheet so users can hand the HTML to their browser's
// "Print to PDF" without losing the structural layout. The shipped
// implementation lives in `render/assets/app.css`; the rendered
// document concatenates the chart CSS + app CSS into exactly one
// top-level `<style>` block (spec FR-039).
//
// This test renders a minimal report, extracts the (expected to
// be exactly one) `<style>…</style>` block, and asserts that:
//
//  1. The rendered document carries exactly one top-level `<style>`
//     block (FR-039 — the concatenation of chart CSS + app CSS).
//  2. At least one `@media print` rule is present inside it.
//  3. That rule targets `details` (so section bodies expand in
//     print) and `nav.index` (so the nav rail hides in print) —
//     the two selectors FR-037 explicitly calls out.
//  4. The page-layout override (the `.layout` grid collapse or an
//     equivalent single-column rule) is present so multi-column
//     layouts don't bleed onto a second page.
//
// The fixture is a minimal Collection — the CSS is embedded via
// `//go:embed` and doesn't depend on parser output, so a smaller
// Collection keeps this test focused on the print invariant.
func TestPrintStylesheetPresent(t *testing.T) {
	c := &model.Collection{
		RootPath: "/tmp/fr037",
		Hostname: "example-db-01",
		Snapshots: []*model.Snapshot{{
			Timestamp:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Prefix:      "snap",
			SourceFiles: map[model.Suffix]*model.SourceFile{},
		}},
	}

	var buf bytes.Buffer
	opts := render.RenderOptions{
		GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Version:     "v0.0.1-test",
	}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("render.Render: %v", err)
	}
	html := buf.String()

	// Extract every `<style>…</style>` block. A hostile template
	// change that moved the print rules into a `<style media="print">`
	// block would technically still satisfy FR-037 but not this
	// test — in that case the test and the template should BOTH
	// be updated to the new shape. FR-039 requires the rendered
	// document to carry exactly one top-level `<style>` block (the
	// concatenation of chart CSS + app CSS); asserting that here
	// keeps the two invariants aligned.
	styleRE := regexp.MustCompile(`(?s)<style[^>]*>(.*?)</style>`)
	blocks := styleRE.FindAllStringSubmatch(html, -1)
	if len(blocks) == 0 {
		t.Fatalf("rendered HTML carries no <style> blocks; FR-037 requires an embedded print stylesheet")
	}
	if len(blocks) != 1 {
		t.Errorf("rendered HTML carries %d <style> blocks; FR-039 requires exactly one (chart CSS + app CSS concatenated)", len(blocks))
	}
	var css strings.Builder
	for _, m := range blocks {
		css.WriteString(m[1])
		css.WriteString("\n")
	}
	combined := css.String()

	// Invariant 1: one `@media print` rule present.
	mediaPrintRE := regexp.MustCompile(`@media\s+print\b`)
	if !mediaPrintRE.MatchString(combined) {
		t.Fatalf("no @media print rule found in any <style> block; FR-037 requires a print stylesheet")
	}

	// Invariants 2 + 3: locate the @media print body and assert it
	// contains the required selectors. Using a brace-counting walker
	// rather than a regex because the body contains nested rules
	// and an opaque regex would either under-match or over-match.
	start := mediaPrintRE.FindStringIndex(combined)
	if start == nil {
		t.Fatalf("regex match failed after presence check — should not happen")
	}
	// Advance to the opening brace of the media block.
	openIdx := strings.Index(combined[start[1]:], "{")
	if openIdx == -1 {
		t.Fatalf("@media print declaration has no opening brace")
	}
	openIdx += start[1]
	depth := 1
	i := openIdx + 1
	for i < len(combined) && depth > 0 {
		switch combined[i] {
		case '{':
			depth++
		case '}':
			depth--
		}
		i++
	}
	if depth != 0 {
		t.Fatalf("@media print block is unterminated (depth=%d)", depth)
	}
	// Strip CSS block comments before running selector assertions.
	// Without this, a `/* details > .body { display: block } */`
	// comment inside the @media print block would satisfy a bare
	// `\bdetails\b` substring/word check, leaving a real stylesheet
	// regression (e.g. the `details` rule deleted but a comment
	// left behind) invisible to this test.
	rawBody := combined[openIdx+1 : i-1]
	printBody := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(rawBody, "")

	// Selectors the shipped app.css emits inside @media print.
	// FR-037 mandates the first two (details + nav rail hide);
	// the third is the layout override that keeps columns from
	// bleeding across pages — the shipped implementation uses
	// `.layout { grid-template-columns: 1fr; }`.
	requiredSelectors := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"details (section bodies expand in print)", regexp.MustCompile(`\bdetails\b`)},
		{"nav rail hide (nav.index)", regexp.MustCompile(`nav\.index|\.nav-rail`)},
		{"layout override (single-column or .layout rule)", regexp.MustCompile(`\.layout\b|grid-template-columns`)},
	}
	for _, sel := range requiredSelectors {
		if !sel.pattern.MatchString(printBody) {
			t.Errorf("@media print block is missing %q; FR-037 requires this selector", sel.name)
		}
	}
}
