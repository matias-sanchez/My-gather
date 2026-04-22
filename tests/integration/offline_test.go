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

// TestOfflineRender: T033 / FR-022 / Principle IX.
//
// FR-022 / Principle IX require that the rendered HTML report run
// entirely offline — no `src=`, `href=`, or `url(…)` reference may
// resolve to a network location, and no `<script>` may carry a
// remote `src`. A single accidental CDN link (Chart.js, a font, an
// analytics beacon) would break offline operation silently: the
// page would render, but a subset of features would degrade
// depending on which asset failed to load.
//
// The `tests/lint` package already forbids network imports at the
// Go source level; this is the runtime-output equivalent. We run
// the compiled binary against the committed fixture and scan the
// emitted HTML for any of the four network-URL probe patterns:
//
//   - `"http://`, `"https://`  — absolute URLs in any attribute.
//   - `"//`                    — protocol-relative URLs (these are
//     network-bound in a file:// context, which is the canonical
//     offline target).
//   - `url(http`, `url(https`, `url(//` — CSS external resources.
//
// False-positive guard: the `mygather:` storage key prefix and
// similar non-URL substrings can contain `://` accidentally. The
// probes here include the opening quote or the `url(` prefix so
// they only fire on actual URL-bearing contexts.
//
// Verbatim-passthrough context: FR-026 copies banner text that may
// contain URLs like `https://perconadev.atlassian.net/…` verbatim
// into the report. Those references live inside `<div
// class="verbatim">`-wrapped `<pre>` blocks (not `src=`/`href=`
// attributes), so the attribute-scoped probes below won't match
// them. A regression that moved the passthrough text into an
// interactive element would rightly fail this test.
func TestOfflineRender(t *testing.T) {
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
	htmlStr := string(html)

	// Attribute probes: src= / href= values must not be network URLs.
	// Embedding the opening quote eliminates false positives from
	// prose like `https://example.com` inside a verbatim <pre> block
	// (which wouldn't sit inside an attribute value anyway).
	attrProbes := []string{
		`src="http://`,
		`src="https://`,
		`src="//`,
		`href="http://`,
		`href="https://`,
		`href="//`,
	}
	for _, probe := range attrProbes {
		if strings.Contains(htmlStr, probe) {
			t.Errorf("rendered HTML contains %q; FR-022 / Principle IX require offline operation (no network resources on interactive elements)", probe)
		}
	}

	// CSS url() probes: inline stylesheets must not pull external
	// fonts / images. The `@font-face { src: url(data:…) }` pattern
	// (base64-embedded) is still offline-safe, so we only flag
	// `url(http|https|//` prefixes.
	cssProbes := []*regexp.Regexp{
		regexp.MustCompile(`url\(\s*['"]?https?://`),
		regexp.MustCompile(`url\(\s*['"]?//[^/]`), // protocol-relative; skip `url(/…)` which is file-relative
	}
	for _, re := range cssProbes {
		if m := re.FindString(htmlStr); m != "" {
			t.Errorf("rendered HTML contains CSS external reference %q; FR-022 expects all stylesheet assets inline or embedded", m)
		}
	}

	// <script src=…> must never carry ANY src attribute — every
	// script block in the report is inline. A src that resolves to
	// a bare filename would still break offline operation if the
	// file is shipped separately; the shipped design is
	// single-file, all-inline.
	scriptSrcRE := regexp.MustCompile(`<script\b[^>]*\bsrc=`)
	if m := scriptSrcRE.FindString(htmlStr); m != "" {
		t.Errorf("rendered HTML contains a <script> element with a src attribute (%q); the single-file offline design requires every script to be inline", m)
	}

	// <link rel="stylesheet"> is likewise forbidden — the
	// stylesheet is concatenated into a single inline <style>
	// block by render.go.
	linkCSSRE := regexp.MustCompile(`<link\b[^>]*\brel="stylesheet"`)
	if m := linkCSSRE.FindString(htmlStr); m != "" {
		t.Errorf("rendered HTML contains a <link rel=\"stylesheet\"> element (%q); the single-file offline design requires the stylesheet to be inline", m)
	}
}
