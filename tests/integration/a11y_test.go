package integration_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestA11ySmoke: T081 / F25 / Principle XI.
//
// F25's remediation scope requires the rendered report to clear a
// baseline of static accessibility checks that are feasible without
// an external a11y toolchain (axe-core, pa11y, etc. — rejected per
// Principle X). The checks here intentionally stop at structural
// invariants a regex can verify; semantic quality (contrast ratios,
// screen-reader flow, keyboard traps in a live DOM) is tracked by
// the Phase 8 UX audit (T115 print layout + the in-audit-record
// a11y notes for each section).
//
// This test runs the full binary + render pipeline against
// testdata/example2/ and asserts six structural invariants:
//
//   1. A `<nav …>` element exists. The shipped template uses
//      `<nav class="index">` for the sticky side rail; the test
//      tolerates any `<nav>` so a future refactor that renamed the
//      class but kept the element still passes.
//   2. That `<nav>` element contains at least one `<a href="#…">`
//      internal link. A nav with no internal links is non-
//      functional for keyboard-only users.
//   3. Every `<details>` element has a direct `<summary>` child.
//      Collapsible sections without a summary label are unnavigable
//      to screen readers.
//   4. Every `<img>` element carries an `alt=` attribute (possibly
//      empty for decorative images, per HTML5 rules; the attribute
//      presence is what screen readers key on).
//   5. Every interactive form control (input / select / textarea /
//      button) has EITHER a visible label (via `<label for="id">`
//      or an enclosing `<label>`), an `aria-label`, or an
//      `aria-labelledby` attribute. The shipped variables-filter
//      input uses an enclosing `<label>`; other controls may use
//      different shapes.
//   6. No custom-role element (`role="button"`, `role="link"`,
//      etc.) without a matching `aria-label`. Custom roles without
//      accessible names are one of the most common F25-class
//      regressions and can appear anywhere a design-first
//      contributor adds a clickable `<div>`.
//
// None of these are sufficient for full WCAG compliance — they are
// the cheap-to-check structural floor. Phase 8's audit record
// layers the semantic checks on top.
func TestA11ySmoke(t *testing.T) {
	root := repoRoot(t)
	input := filepath.Join(root, "testdata", "example2")
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "report.html")

	cmd := exec.Command(binPath, "--out", out, input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("my-gather exit non-zero: %v\nstderr:\n%s", err, stderr.String())
	}
	html, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	rawDoc := string(html)

	// Strip `<script>…</script>` and `<style>…</style>` BODIES
	// before running structural regexes. Those blocks contain Go
	// template source, JS comments, and CSS rule text that can
	// literally include strings like "<details>" in prose — a bare
	// tag-finding regex would treat them as real HTML elements and
	// report spurious mismatches with the actual `<summary>` count.
	// The stripping keeps the opening / closing tags themselves (so
	// `<script src=…>` still surfaces in later invariants) but
	// replaces the body text between them with an empty placeholder.
	stripBodyRE := regexp.MustCompile(`(?s)(<(?:script|style)\b[^>]*>).*?(</(?:script|style)>)`)
	doc := stripBodyRE.ReplaceAllString(rawDoc, "$1$2")

	// Invariant 1: at least one `<nav …>` element.
	navRE := regexp.MustCompile(`<nav\b[^>]*>`)
	navs := navRE.FindAllStringIndex(doc, -1)
	if len(navs) == 0 {
		t.Fatalf("rendered HTML has no <nav> element; keyboard-only users have no site map")
	}

	// Invariant 2: at least one nav contains `<a href="#…">`.
	// Scan each nav's body (from its opening tag to the matching
	// `</nav>`) for at least one internal anchor.
	anchorInternalRE := regexp.MustCompile(`<a\s+[^>]*href="#[^"]+"`)
	foundInternal := false
	for _, loc := range navs {
		start := loc[1] // past the opening tag
		closeRel := strings.Index(doc[start:], "</nav>")
		if closeRel == -1 {
			continue
		}
		body := doc[start : start+closeRel]
		if anchorInternalRE.MatchString(body) {
			foundInternal = true
			break
		}
	}
	if !foundInternal {
		t.Errorf(`no <nav> element contains an <a href="#…"> internal link; FR-031 nav rail requires anchor links to section IDs`)
	}

	// Invariant 3: every `<details>` has a `<summary>` child. Count
	// `<details>` openings and `<summary>` openings; the latter must
	// be at least as many. (A `<details>` without a `<summary>`
	// displays "Details" in Chromium and nothing in Firefox — both
	// are screen-reader hostile.)
	detailsOpenRE := regexp.MustCompile(`<details\b[^>]*>`)
	summaryOpenRE := regexp.MustCompile(`<summary\b[^>]*>`)
	detailsCount := len(detailsOpenRE.FindAllString(doc, -1))
	summaryCount := len(summaryOpenRE.FindAllString(doc, -1))
	if detailsCount == 0 {
		t.Fatalf("rendered HTML has no <details> elements; FR-031 requires collapsible sections")
	}
	if summaryCount < detailsCount {
		t.Errorf("rendered HTML has %d <details> but only %d <summary>; every <details> must carry a <summary>", detailsCount, summaryCount)
	}

	// Invariant 4: every `<img>` has `alt=`. Uses a tag-scoped scan
	// so a `<img>` without the attribute surfaces the actual tag
	// text in the error message.
	imgTagRE := regexp.MustCompile(`<img\b[^>]*>`)
	for _, tag := range imgTagRE.FindAllString(doc, -1) {
		if !strings.Contains(tag, " alt=") && !strings.Contains(tag, "\talt=") {
			t.Errorf("<img> without alt=: %q (decorative images MUST carry alt=\"\" per HTML5)", tag)
		}
	}

	// Invariant 5: every interactive control has a label. Build a
	// set of all `id` values referenced by `<label for="id">`; for
	// each interactive control, require EITHER:
	//   - the control's `id` is in the label-for set, OR
	//   - the control carries `aria-label=` / `aria-labelledby=`,
	//     OR
	//   - the control is enclosed by a `<label>…</label>` (handled
	//     by checking a 200-char window of context).
	labelForRE := regexp.MustCompile(`<label\b[^>]*\bfor="([^"]+)"`)
	labelFor := map[string]struct{}{}
	for _, m := range labelForRE.FindAllStringSubmatch(doc, -1) {
		labelFor[m[1]] = struct{}{}
	}
	controlRE := regexp.MustCompile(`<(input|select|textarea|button)\b[^>]*>`)
	controlMatches := controlRE.FindAllStringIndex(doc, -1)
	for _, loc := range controlMatches {
		tag := doc[loc[0]:loc[1]]
		// Skip <input type="hidden"> — not interactive, not
		// visible, labelling is a no-op.
		if strings.Contains(tag, `type="hidden"`) {
			continue
		}
		// ARIA attrs?
		if strings.Contains(tag, "aria-label=") || strings.Contains(tag, "aria-labelledby=") {
			continue
		}
		// <label for=…>?
		idRE := regexp.MustCompile(`\bid="([^"]+)"`)
		if m := idRE.FindStringSubmatch(tag); m != nil {
			if _, ok := labelFor[m[1]]; ok {
				continue
			}
		}
		// Enclosing <label>? Check a window of preceding + trailing
		// text. A control whose 200-char preceding window contains
		// `<label` without an intervening `</label>` is considered
		// labelled by enclosure.
		windowStart := loc[0] - 200
		if windowStart < 0 {
			windowStart = 0
		}
		preceding := doc[windowStart:loc[0]]
		lastLabel := strings.LastIndex(preceding, "<label")
		lastLabelClose := strings.LastIndex(preceding, "</label>")
		if lastLabel > lastLabelClose {
			continue
		}
		// <button> elements with text content between the opening
		// and closing tags count as self-labelled — the text IS the
		// accessible name. Check the next 200 chars for a
		// `>non-empty text</button>` pattern.
		if strings.HasPrefix(tag, "<button") {
			end := loc[1] + 500
			if end > len(doc) {
				end = len(doc)
			}
			rest := doc[loc[1]:end]
			closeRel := strings.Index(rest, "</button>")
			if closeRel > 0 {
				inner := rest[:closeRel]
				stripped := strings.TrimSpace(stripTags(inner))
				if stripped != "" {
					continue
				}
			}
		}
		t.Errorf("interactive control without a label: %q — add aria-label, aria-labelledby, <label for=…>, or enclose in <label>", tag)
	}

	// Invariant 6: no `role="..."` without a matching `aria-label`
	// or `aria-labelledby`. Semantic HTML elements (a, button, nav,
	// input) are self-identifying; a custom role on a neutral
	// element (`<div role="button">`) must carry an accessible
	// name.
	customRoleRE := regexp.MustCompile(`<(div|span|section)\b[^>]*\brole="([^"]+)"[^>]*>`)
	for _, m := range customRoleRE.FindAllStringSubmatchIndex(doc, -1) {
		tagStart, tagEnd := m[0], m[1]
		tag := doc[tagStart:tagEnd]
		if strings.Contains(tag, "aria-label=") || strings.Contains(tag, "aria-labelledby=") {
			continue
		}
		t.Errorf("custom role on neutral element without accessible name: %q — add aria-label or aria-labelledby", tag)
	}
}

// stripTags removes anything between `<` and `>` from s so the
// remaining text can be trimmed to test whether a <button> had a
// visible label. Crude but sufficient for the accessible-name
// check; a real HTML parser isn't warranted for a single-use
// helper.
func stripTags(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
