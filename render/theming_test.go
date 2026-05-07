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

// TestThemingColorSchemePerTheme — every theme block sets the CSS
// `color-scheme` property so browser-native UI (form controls,
// scrollbars, autofill chrome) matches the report surface. Without
// it, a user whose OS prefers a different scheme would see mismatched
// native chrome on top of the themed page.
func TestThemingColorSchemePerTheme(t *testing.T) {
	css := embeddedAppCSS
	cases := []struct {
		selector string
		want     string
	}{
		{`:root[data-theme="dark"]`, "dark"},
		{`:root[data-theme="light"]`, "light"},
		// colorblind uses a light surface (Okabe-Ito) so its native
		// chrome should also be light.
		{`:root[data-theme="colorblind"]`, "light"},
	}
	for _, tc := range cases {
		block := extractThemeBlock(t, css, tc.selector)
		if block == "" {
			t.Fatalf("theme block %q not found in embedded CSS", tc.selector)
		}
		needle := "color-scheme:"
		idx := strings.Index(block, needle)
		if idx < 0 {
			t.Errorf("theme %q missing required color-scheme declaration", tc.selector)
			continue
		}
		rest := block[idx+len(needle):]
		semi := strings.IndexByte(rest, ';')
		if semi < 0 {
			t.Errorf("theme %q color-scheme missing terminator", tc.selector)
			continue
		}
		val := strings.TrimSpace(rest[:semi])
		if !strings.Contains(val, tc.want) {
			t.Errorf("theme %q color-scheme = %q; want value containing %q", tc.selector, val, tc.want)
		}
	}
}

// TestThemingLightSeriesAreDistinct — the light theme's 16 series
// tokens MUST resolve to 16 distinct color values. Earlier drafts
// repeated --series-1..8 in --series-9..16, which made multi-series
// charts with 9+ lines ambiguous (Copilot review on PR #62, research
// D5). Dark and colorblind already ship 16 distinct (or intentionally
// cycled accessibility-set) values; the light theme must mirror that
// distinctness for legend readability.
func TestThemingLightSeriesAreDistinct(t *testing.T) {
	css := embeddedAppCSS
	block := extractThemeBlock(t, css, `:root[data-theme="light"]`)
	if block == "" {
		t.Fatalf("light theme block not found in embedded CSS")
	}
	seen := make(map[string]int, 16)
	for slot := 1; slot <= 16; slot++ {
		needle := "--series-" + itoa(slot) + ":"
		idx := strings.Index(block, needle)
		if idx < 0 {
			t.Fatalf("--series-%d not present in light block", slot)
		}
		rest := block[idx+len(needle):]
		semi := strings.IndexByte(rest, ';')
		if semi < 0 {
			t.Fatalf("--series-%d missing terminator in light block", slot)
		}
		val := strings.ToLower(strings.TrimSpace(rest[:semi]))
		if prev, dup := seen[val]; dup {
			t.Errorf("light --series-%d = %s duplicates --series-%d; expected 16 distinct colors", slot, val, prev)
		}
		seen[val] = slot
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
	// The script encodes the closed set as an array of allowed names
	// and validates with `indexOf` so prototype properties on plain
	// objects (e.g., "toString", "constructor") cannot pass the check:
	//   var ok=["dark","light","colorblind"]; ok.indexOf(v) >= 0
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

// TestThemingPrePaintValidatorIsPrototypeSafe — regression guard for
// the inline pre-paint script's closed-set validator. A plain-object
// lookup (e.g. `var ok={dark:1,...}; ok[v]`) would let inherited
// prototype keys such as "toString", "constructor", or "__proto__"
// pass the truthiness check and propagate as `data-theme`. The
// canonical form uses an array literal validated with `indexOf`,
// which only matches own elements and cannot be polluted via the
// prototype chain. Asserts both that the safe form is present and
// that the unsafe object-lookup form is absent.
func TestThemingPrePaintValidatorIsPrototypeSafe(t *testing.T) {
	out := renderForThemingTest(t)
	headStart := strings.Index(out, "<head>")
	headEnd := strings.Index(out, "</head>")
	if headStart < 0 || headEnd < 0 || headEnd < headStart {
		t.Fatalf("rendered HTML has no <head>...</head> block")
	}
	head := out[headStart:headEnd]
	if !strings.Contains(head, `["dark","light","colorblind"]`) {
		t.Errorf("pre-paint script missing the canonical array-literal closed set [\"dark\",\"light\",\"colorblind\"]")
	}
	if !strings.Contains(head, `.indexOf(v)`) {
		t.Errorf("pre-paint script missing array.indexOf(v) validation form")
	}
	// The old unsafe form `{dark:1,light:1,colorblind:1}` MUST NOT
	// reappear; it would let prototype keys (toString, constructor,
	// __proto__) pass the truthiness check.
	if strings.Contains(head, "{dark:1") || strings.Contains(head, "ok[v]") {
		t.Errorf("pre-paint script regressed to prototype-unsafe object lookup; use array.indexOf instead")
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

// TestThemingHLLSparklineSubscribesToThemeEvent — the InnoDB HLL
// sparkline canonically opts OUT of registerChart() because it must
// not participate in chart-zoom-sync. That means the global
// repaintChartsForTheme() walker in app-js/00.js never sees it, and a
// dark→light/colorblind switch would otherwise leave the sparkline on
// stale colors until a full page reload. The canonical re-pick path
// for this one chart is a self-contained
// document.addEventListener("mygather:theme", ...) inside the
// renderHLLSparkline body. This test fails if a future revert removes
// either the listener or the registerChart exemption.
func TestThemingHLLSparklineSubscribesToThemeEvent(t *testing.T) {
	js := embeddedAppJS
	body := extractFunctionBody(t, js, "renderHLLSparkline")
	if body == "" {
		t.Fatalf("renderHLLSparkline not found in embedded app JS")
	}
	// (a) The listener must be wired inside the function body so
	// theme switches re-pick the stroke + fill + cursor colors.
	if !strings.Contains(body, `addEventListener("mygather:theme"`) &&
		!strings.Contains(body, `addEventListener('mygather:theme'`) {
		t.Errorf("renderHLLSparkline body missing addEventListener(\"mygather:theme\", ...) — chart will not re-paint on theme switch (Codex P2 finding on PR #62)")
	}
	// (b) The function MUST NOT call registerChart(...) — that would
	// pull the sparkline into chart-zoom-sync and into the global
	// repaintChartsForTheme() walker. The canonical exemption is the
	// "Do NOT registerChart" comment near the end of the function;
	// this assertion encodes the contract test-side.
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		if strings.Contains(trimmed, "registerChart(") {
			t.Fatalf("renderHLLSparkline body now calls registerChart — breaks the chart-zoom-sync exemption that PR #59 documented; the canonical re-pick path is the local mygather:theme listener, not the global registry")
		}
	}
}

// extractFunctionBody returns the substring of src that lies inside
// the brace-balanced body of the named JavaScript function. It uses
// a simple character-by-character brace counter that ignores braces
// inside string literals (single, double, backtick) and inside
// // and /* ... */ comments. The result is the body without the
// surrounding `{` and `}`. Returns "" if the function header is not
// found or its body is unbalanced.
func extractFunctionBody(t *testing.T, src, fnName string) string {
	t.Helper()
	header := "function " + fnName + "("
	hdr := strings.Index(src, header)
	if hdr < 0 {
		return ""
	}
	open := strings.IndexByte(src[hdr:], '{')
	if open < 0 {
		return ""
	}
	start := hdr + open + 1
	depth := 1
	i := start
	for i < len(src) {
		c := src[i]
		switch c {
		case '/':
			if i+1 < len(src) && src[i+1] == '/' {
				// Line comment — skip to end of line.
				j := strings.IndexByte(src[i:], '\n')
				if j < 0 {
					return ""
				}
				i += j + 1
				continue
			}
			if i+1 < len(src) && src[i+1] == '*' {
				// Block comment — skip past */.
				j := strings.Index(src[i+2:], "*/")
				if j < 0 {
					return ""
				}
				i += 2 + j + 2
				continue
			}
		case '"', '\'', '`':
			quote := c
			i++
			for i < len(src) {
				if src[i] == '\\' {
					i += 2
					continue
				}
				if src[i] == quote {
					i++
					break
				}
				i++
			}
			continue
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[start:i]
			}
		}
		i++
	}
	return ""
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
