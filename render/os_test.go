package render_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// TestOSGoldenHTML: T046 / FR-009..011 / Principle VIII.
//
// Render a Collection populated by parse.Discover on
// testdata/example2/ but filtered to expose only the three OS
// parsers (iostat, top, vmstat). Extract the top-level
// `<details id="sec-os">…</details>` block from the rendered HTML
// and snapshot-compare against a committed golden fragment.
//
// Why section-only instead of full-doc: the OS section is the
// surface this test protects; the Diagnostics / Variables / DB
// sections render their "data not available" banner in this
// configuration and are covered by their own golden tests. Shipping
// per-section goldens keeps review diffs focused — a rendering
// regression in DB would otherwise dirty the OS golden too.
func TestOSGoldenHTML(t *testing.T) {
	root := goldens.RepoRoot(t)
	goldenPath := filepath.Join(root, "testdata", "golden", "os.example2.html")

	html := renderGolden(t, model.SuffixIostat, model.SuffixTop, model.SuffixVmstat)
	section := extractDetailsSection(t, html, "sec-os")

	goldens.Compare(t, goldenPath, []byte(section))
}

// TestOSSubviewAnchors: T096 / SC-005 / FR-031.
//
// The OS Usage section renders four subviews (iostat, top, vmstat,
// meminfo). SC-005 requires each subview to be addressable both by
// an HTML anchor AND by a corresponding `Report.Navigation` entry so
// a user can deep-link to the specific chart.
//
// The real subview IDs emitted by render/templates/os.html.tmpl are
// `sub-os-iostat`, `sub-os-top`, `sub-os-vmstat`, `sub-os-meminfo`
// (the task description in tasks.md used aspirational names that
// never landed; this test pins what the template actually emits). A
// template edit that dropped one anchor — or a nav-rail refactor
// that stopped linking to it — would fail here.
func TestOSSubviewAnchors(t *testing.T) {
	html := renderGolden(t, model.SuffixIostat, model.SuffixTop, model.SuffixVmstat, model.SuffixMeminfo)
	section := extractDetailsSection(t, html, "sec-os")

	subviews := []string{"sub-os-iostat", "sub-os-top", "sub-os-vmstat", "sub-os-meminfo"}

	// Every subview anchor lives inside the sec-os <details> block.
	assertAnchorsContained(t, section, subviews, `id="%s"`,
		"OS section missing %q anchor; SC-005 requires every OS subview to carry an HTML id for deep-linking")

	// Every subview must also be addressable from Report.Navigation,
	// which serialises into <nav class="index"> at the document top
	// (outside sec-os).
	assertAnchorsContained(t, html, subviews, `href="#%s"`,
		"Report.Navigation missing href=\"#%s\"; SC-005 requires every OS subview anchor to be reachable from the nav rail")
}

// assertAnchorsContained asserts that every anchor in `anchors`
// appears inside `content` when formatted through `marker` (e.g.
// `id="%s"` or `href="#%s"`). Extracted so both halves of
// TestOSSubviewAnchors — "anchor lives in section HTML" and "nav rail
// links to anchor" — drive through a single loop instead of repeating
// the Contains scaffold.
func assertAnchorsContained(t *testing.T, content string, anchors []string, marker, missingFmt string) {
	t.Helper()
	for _, anchor := range anchors {
		want := fmt.Sprintf(marker, anchor)
		if !strings.Contains(content, want) {
			t.Errorf(missingFmt, anchor)
		}
	}
}
