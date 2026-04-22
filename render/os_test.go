package render_test

import (
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
// The OS Usage section renders three subviews (iostat, top, vmstat).
// SC-005 requires each subview to be addressable both by an HTML
// anchor AND by a corresponding `Report.Navigation` entry so a user
// can deep-link to the specific chart.
//
// The real subview IDs emitted by render/templates/os.html.tmpl are
// `sub-os-iostat`, `sub-os-top`, `sub-os-vmstat` (the task description
// in tasks.md used aspirational `os-disk-util`/`os-top-cpu`/`os-vmstat`
// names that never landed; this test pins what the template actually
// emits). A template edit that dropped one anchor — or a nav-rail
// refactor that stopped linking to it — would fail here.
func TestOSSubviewAnchors(t *testing.T) {
	html := renderGolden(t, model.SuffixIostat, model.SuffixTop, model.SuffixVmstat)
	section := extractDetailsSection(t, html, "sec-os")

	subviews := []string{"sub-os-iostat", "sub-os-top", "sub-os-vmstat"}

	// Every subview anchor lives inside the sec-os <details> block.
	for _, anchor := range subviews {
		want := `id="` + anchor + `"`
		if !strings.Contains(section, want) {
			t.Errorf("OS section missing %q anchor; SC-005 requires every OS subview to carry an HTML id for deep-linking", anchor)
		}
	}

	// Every subview must also be addressable from Report.Navigation,
	// which serialises into <nav class="index"> at the document top
	// (outside sec-os).
	for _, anchor := range subviews {
		wantHref := `href="#` + anchor + `"`
		if !strings.Contains(html, wantHref) {
			t.Errorf("Report.Navigation missing href=\"#%s\"; SC-005 requires every OS subview anchor to be reachable from the nav rail", anchor)
		}
	}
}
