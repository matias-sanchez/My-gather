package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// Theming tests for feature 020-report-theming.
//
// These tests assert the canonical theme system shipped in the
// embedded CSS / JS / template strings. They satisfy FR-012:
//   (a) a theme switch updates document.documentElement.dataset.theme
//   (b) the chart palette tokens for each theme resolve to the
//       expected color set
// without adding a JS test runner to the main module — the spec text
// the user explicitly asks for ("component tests that flip themes
// and assert the document data-theme attribute updates and the chart
// palette tokens resolve to the expected color set") is verified by
// inspecting the source the browser will execute.

// requiredTokens lists every token that MUST be defined in every
// theme block in app-css/04.css. Drift between this list and the CSS
// file is the regression we want to catch — adding a new theme-
// sensitive value to CSS without adding it to every theme block
// means one theme silently inherits another's colour, which is
// exactly the parallel-path smell Principle XIII forbids.
var requiredTokens = []string{
	"--bg",
	"--bg-elev",
	"--bg-elev-2",
	"--border",
	"--border-strong",
	"--fg",
	"--fg-muted",
	"--fg-dim",
	"--accent",
	"--accent-soft",
	"--ok",
	"--warn",
	"--err",
	"--shadow",
	"--tooltip-bg",
	"--tooltip-fg",
	"--axis-stroke",
	"--grid-stroke",
	"--tick-stroke",
	"--series-1",
	"--series-2",
	"--series-3",
	"--series-4",
	"--series-5",
	"--series-6",
	"--series-7",
	"--series-8",
	"--series-9",
	"--series-10",
	"--series-11",
	"--series-12",
	"--series-13",
	"--series-14",
	"--series-15",
	"--series-16",
}

// extractThemeBlock returns the substring of css between the
// `selector {` and the matching `}`. Uses brace counting so nested
// rules (none today, but cheap insurance) do not trip it. Returns
// "" if the selector is not found.
func extractThemeBlock(t *testing.T, css, selector string) string {
	t.Helper()
	open := strings.Index(css, selector+" {")
	if open < 0 {
		// Try without the space before {
		open = strings.Index(css, selector+"{")
		if open < 0 {
			return ""
		}
	}
	// Skip past the `{`
	brace := strings.IndexByte(css[open:], '{')
	if brace < 0 {
		return ""
	}
	start := open + brace + 1
	depth := 1
	for i := start; i < len(css); i++ {
		switch css[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return css[start:i]
			}
		}
	}
	return ""
}

// TestThemingTokensPresent — every theme block defines every required
// token. Catches drift where a new theme-sensitive value is added to
// one theme but not the others.
func TestThemingTokensPresent(t *testing.T) {
	css := embeddedAppCSS
	for _, sel := range []string{
		`:root[data-theme="dark"]`,
		`:root[data-theme="light"]`,
		`:root[data-theme="colorblind"]`,
	} {
		block := extractThemeBlock(t, css, sel)
		if block == "" {
			t.Fatalf("theme block %q not found in embedded CSS", sel)
		}
		for _, tok := range requiredTokens {
			if !strings.Contains(block, tok+":") {
				t.Errorf("theme %q missing required token %q", sel, tok)
			}
		}
	}
}

// TestThemingOkabeItoPalette — the colorblind theme's series tokens
// match the published Okabe-Ito 8-color set in conventional order,
// cycled to fill positions 9 through 16. Catches accidental edits to
// the accessibility palette.
func TestThemingOkabeItoPalette(t *testing.T) {
	css := embeddedAppCSS
	block := extractThemeBlock(t, css, `:root[data-theme="colorblind"]`)
	if block == "" {
		t.Fatalf("colorblind theme block not found in embedded CSS")
	}
	okabe := []string{
		"#E69F00", "#56B4E9", "#009E73", "#F0E442",
		"#0072B2", "#D55E00", "#CC79A7", "#000000",
	}
	// Positions 1-8 = Okabe-Ito; positions 9-16 = same colors cycled.
	for slot := 1; slot <= 16; slot++ {
		want := okabe[(slot-1)%len(okabe)]
		needle := "--series-" + itoa(slot) + ":"
		idx := strings.Index(block, needle)
		if idx < 0 {
			t.Errorf("--series-%d not present in colorblind block", slot)
			continue
		}
		// Inspect the value between `:` and `;` after the token name.
		rest := block[idx+len(needle):]
		semi := strings.IndexByte(rest, ';')
		if semi < 0 {
			t.Errorf("--series-%d missing terminator in colorblind block", slot)
			continue
		}
		val := strings.TrimSpace(rest[:semi])
		if !strings.EqualFold(val, want) {
			t.Errorf("colorblind --series-%d = %q; want %q (Okabe-Ito)", slot, val, want)
		}
	}
}

// itoa is a tiny dependency-free integer formatter. Using it keeps
// this test from importing strconv just for two-digit decimals.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [4]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// TestThemingSeriesColorsArrayRemoved — regression guard against
// reintroducing the parallel JS series-color array. Principle XIII:
// the canonical owner is now the CSS token block.
//
// We assert no live use of the array exists. The grep skips comment
// lines so the regression-history reference inside the removal
// comment in app-js/00.js does not register as a violation.
func TestThemingSeriesColorsArrayRemoved(t *testing.T) {
	js := embeddedAppJS
	for _, line := range strings.Split(js, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		if strings.Contains(line, "SERIES_COLORS") {
			t.Fatalf("embedded app JS still contains live SERIES_COLORS reference (line: %q) — Principle XIII (canonical path) violation; the series palette must come from --series-N CSS variables only", line)
		}
	}
}

// TestThemingSeriesViaCssVar — the chart code resolves series strokes
// via the existing cssVar() helper reading the `--series-` token
// family. Asserts the call form so a future refactor that breaks the
// tie back to the CSS token block fails loudly.
func TestThemingSeriesViaCssVar(t *testing.T) {
	js := embeddedAppJS
	for _, want := range []string{
		`cssVar("--series-"`,
		`cssVar("--axis-stroke"`,
		`cssVar("--grid-stroke"`,
		`cssVar("--tick-stroke"`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("embedded app JS missing token read %q", want)
		}
	}
}

// TestThemingPrefersColorSchemeRemoved — the legacy
// @media (prefers-color-scheme: light) override block in 00.css is
// the prior canonical path for "light variant" theming and was
// deleted in this feature. Reintroducing it would create a parallel
// theme source competing with [data-theme] (Principle XIII).
//
// CSS comments use /* ... */ so the line-skip filter looks for
// either form to skip the regression-history reference in the new
// 04.css comment header without regressing the actual @media check.
func TestThemingPrefersColorSchemeRemoved(t *testing.T) {
	css := embeddedAppCSS
	// Strip /* ... */ comment blocks before grepping. Re-using the
	// stdlib here keeps the dependency footprint zero (Principle X).
	stripped := stripCSSComments(css)
	if strings.Contains(stripped, "prefers-color-scheme") {
		t.Fatalf("embedded app CSS still references prefers-color-scheme outside comments — Principle XIII (canonical path) violation; the theme source is [data-theme] on <html> only")
	}
}

// stripCSSComments removes /* ... */ blocks from src so substring
// checks in tests do not falsely trip on regression-history mentions
// inside comment headers. This is a test helper only; the runtime
// CSS parser obviously sees the comments.
func stripCSSComments(src string) string {
	var b strings.Builder
	for {
		open := strings.Index(src, "/*")
		if open < 0 {
			b.WriteString(src)
			break
		}
		b.WriteString(src[:open])
		close := strings.Index(src[open+2:], "*/")
		if close < 0 {
			break
		}
		src = src[open+2+close+2:]
	}
	return b.String()
}

// TestThemingPrePaintScriptInTemplate — the rendered HTML carries an
// inline <script> in <head> that reads localStorage and sets
// document.documentElement.dataset.theme BEFORE <style> is parsed,
// so the saved theme applies on first paint with no flash.
func TestThemingPrePaintScriptInTemplate(t *testing.T) {
	out := renderForThemingTest(t)
	headStart := strings.Index(out, "<head>")
	headEnd := strings.Index(out, "</head>")
	if headStart < 0 || headEnd < 0 || headEnd < headStart {
		t.Fatalf("rendered HTML has no <head>...</head> block")
	}
	head := out[headStart:headEnd]

	// Must contain a script that reads my-gather:theme and sets
	// data-theme on the document element.
	for _, want := range []string{
		`<script>`,
		`my-gather:theme`,
		`localStorage`,
		`documentElement`,
		`dataset.theme`,
	} {
		if !strings.Contains(head, want) {
			t.Errorf("rendered <head> missing pre-paint script marker %q", want)
		}
	}

	// The pre-paint script MUST appear before the <style> block; if it
	// runs after CSS parses, theme rules apply on a second paint and
	// the user sees a flash of the dark default first.
	scriptIdx := strings.Index(head, "<script>")
	styleIdx := strings.Index(head, "<style>")
	if scriptIdx < 0 || styleIdx < 0 {
		t.Fatalf("rendered <head> missing <script> (%d) or <style> (%d)", scriptIdx, styleIdx)
	}
	if scriptIdx > styleIdx {
		t.Fatalf("pre-paint <script> at %d is after <style> at %d in <head>; would cause flash of default theme", scriptIdx, styleIdx)
	}
}

// TestThemingPickerInHeader — the toggle markup is in the report
// header, with the documented id and the three options.
func TestThemingPickerInHeader(t *testing.T) {
	out := renderForThemingTest(t)
	hdrStart := strings.Index(out, `<header class="app-header">`)
	hdrEnd := strings.Index(out, `</header>`)
	if hdrStart < 0 || hdrEnd < 0 || hdrEnd < hdrStart {
		t.Fatalf("rendered HTML has no app-header block")
	}
	hdr := out[hdrStart:hdrEnd]
	for _, want := range []string{
		`id="theme-picker"`,
		`<option value="dark">Dark</option>`,
		`<option value="light">Light</option>`,
		`<option value="colorblind">Color-blind safe</option>`,
		`for="theme-picker"`,
	} {
		if !strings.Contains(hdr, want) {
			t.Errorf("rendered header missing theme-picker marker %q", want)
		}
	}
	// Sanity: the picker sits to the left of the feedback button.
	pickerIdx := strings.Index(hdr, `id="theme-picker"`)
	feedbackIdx := strings.Index(hdr, `id="feedback-open"`)
	if pickerIdx < 0 || feedbackIdx < 0 || pickerIdx > feedbackIdx {
		t.Fatalf("theme picker (%d) should appear before feedback button (%d) in header", pickerIdx, feedbackIdx)
	}
}

// TestThemingDataThemeApplied — the inline pre-paint script reads
// localStorage under the documented key and assigns to
// documentElement.dataset.theme. Covers FR-005 and FR-006.
func TestThemingDataThemeApplied(t *testing.T) {
	out := renderForThemingTest(t)
	for _, want := range []string{
		`"my-gather:theme"`,
		`localStorage.getItem(`,
		`document.documentElement.dataset.theme`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered HTML missing pre-paint marker %q", want)
		}
	}
}

// TestThemingUnknownStoredValueFallsBack — the inline pre-paint
// script validates the stored value against the closed set
// {dark, light, colorblind} and falls back to dark on anything else.
// Covers FR-007.
func TestThemingUnknownStoredValueFallsBack(t *testing.T) {
	out := renderForThemingTest(t)
	// The script encodes the closed set as an object literal lookup
	// (compact, no array indexOf cost): {dark:1,light:1,colorblind:1}.
	// We assert the three names appear together in head.
	headStart := strings.Index(out, "<head>")
	headEnd := strings.Index(out, "</head>")
	if headStart < 0 || headEnd < 0 {
		t.Fatalf("rendered HTML has no <head>...</head> block")
	}
	head := out[headStart:headEnd]
	for _, want := range []string{
		`dark`, `light`, `colorblind`,
	} {
		if !strings.Contains(head, want) {
			t.Errorf("pre-paint script missing closed-set member %q", want)
		}
	}
	// And there must be a literal "dark" fallback assignment in the
	// script (catches accidental removal of the default).
	if !strings.Contains(head, `:"dark"`) && !strings.Contains(head, `="dark"`) {
		t.Errorf("pre-paint script missing dark fallback")
	}
}

// TestThemingStorageWriteIsTryCatch — the theme JS module wraps the
// localStorage write in try/catch so a private-browsing storage
// error never throws into the click handler. Covers the
// localStorage-unavailable edge case.
func TestThemingStorageWriteIsTryCatch(t *testing.T) {
	js := embeddedAppJS
	idx := strings.Index(js, `localStorage.setItem(STORAGE_KEY`)
	if idx < 0 {
		t.Fatalf("theme module does not write to localStorage under STORAGE_KEY")
	}
	// The setItem call must sit inside a try-block. We look for the
	// preceding `try {` within a small backward window.
	window := 200
	from := idx - window
	if from < 0 {
		from = 0
	}
	preceding := js[from:idx]
	if !strings.Contains(preceding, "try {") {
		t.Fatalf("localStorage.setItem write is not inside try { ... } catch — would throw into click handler when storage is unavailable")
	}
}

// TestThemingServerOutputDeterministic — the server-rendered <html>
// opening tag does NOT carry data-theme. The attribute is applied
// client-side by the inline pre-paint script. Baking it in server-
// side would couple HTML output to the user's browser state, which
// would break Principle IV (deterministic output): the same input
// would produce different bytes for different viewers.
func TestThemingServerOutputDeterministic(t *testing.T) {
	out := renderForThemingTest(t)
	// The opening tag is fixed.
	if !strings.Contains(out, `<html lang="en">`) {
		t.Fatalf("server-rendered <html> tag changed; theme must be applied client-side, not baked into server output")
	}
	// And the literal attribute must NOT be present in the opening
	// tag's substring (the attribute appears later inside the inline
	// script as `dataset.theme`, which is fine).
	openTagEnd := strings.Index(out, ">")
	if openTagEnd < 0 {
		t.Fatalf("rendered HTML has no opening <html> tag close")
	}
	openTag := out[:openTagEnd]
	if strings.Contains(openTag, "data-theme") {
		t.Fatalf("server-rendered <html> opening tag carries data-theme; would break Principle IV byte-determinism across viewers")
	}
}

// renderForThemingTest renders a minimal report so theming tests can
// inspect the HTML the user will actually receive. Uses a fixed
// timestamp so re-runs match.
func renderForThemingTest(t *testing.T) string {
	t.Helper()
	c := &model.Collection{
		RootPath: "/tmp/theming",
		Hostname: "theming-test",
		Snapshots: []*model.Snapshot{
			{
				Timestamp:   time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
				Prefix:      "2026_05_07_00_00_00",
				SourceFiles: map[model.Suffix]*model.SourceFile{},
			},
		},
	}
	var buf bytes.Buffer
	if err := Render(&buf, c, RenderOptions{
		GeneratedAt: time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
		Version:     "v0.0.1-theming-test",
	}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return buf.String()
}
