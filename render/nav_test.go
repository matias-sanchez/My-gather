package render_test

import (
	"bytes"
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/parse"
	"github.com/matias-sanchez/My-gather/render"
	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// TestNavCoverage: T035 / SC-009 / FR-031.
//
// SC-009 requires every entry in `Report.Navigation` to resolve to a
// real HTML anchor (`id="..."`) in the rendered document, and every
// anchor in the nav rail (`<a href="#...">`) to point at an
// addressable target. A mismatch in either direction breaks deep-
// linking: either a nav entry points at a dead anchor, or a real
// anchor is unreachable from the rail.
//
// This test runs the full parse.Discover + render pipeline against
// `testdata/example2/` and scans the rendered HTML for the two halves
// of the invariant:
//
//  1. Every `href="#..."` inside the `<nav class="index">` block has
//     a matching `id="..."` somewhere in the document.
//  2. No duplicate `href="#..."` references in the nav rail (a
//     duplicate href would be a rendering bug or a genuine
//     ambiguous link).
//
// The parity check is intentionally asymmetric: a document CAN have
// more `id="..."` anchors than nav entries (sub-subviews, chart
// containers, etc.) — the spec only requires every nav entry point
// to resolve, not the reverse.
func TestNavCoverage(t *testing.T) {
	root := goldens.RepoRoot(t)
	fixtureDir := filepath.Join(root, "testdata", "example2")
	c, err := parse.Discover(context.Background(), fixtureDir, parse.DiscoverOptions{})
	if err != nil {
		t.Fatalf("parse.Discover: %v", err)
	}

	var buf bytes.Buffer
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("render.Render: %v", err)
	}
	html := buf.String()

	// Extract the `<nav class="index">…</nav>` block so the href
	// scan doesn't pick up anchors from the document body (which
	// would be a weaker scope).
	navStart := strings.Index(html, `<nav class="index"`)
	if navStart == -1 {
		t.Fatalf("rendered HTML has no `<nav class=\"index\">` block; FR-031 requires a nav rail")
	}
	navEnd := strings.Index(html[navStart:], `</nav>`)
	if navEnd == -1 {
		t.Fatalf("unterminated `<nav class=\"index\">` block in rendered HTML")
	}
	navBlock := html[navStart : navStart+navEnd]

	// Collect every `href="#..."` inside the nav. Empty-hash hrefs
	// (`href="#"`) are skipped — they are used for no-op links and
	// don't point at a target.
	hrefRE := regexp.MustCompile(`href="#([^"]+)"`)
	hrefs := hrefRE.FindAllStringSubmatch(navBlock, -1)
	if len(hrefs) == 0 {
		t.Fatalf("nav block has no `href=\"#...\"` entries; SC-009 requires nav entries to point at anchors")
	}

	// Invariant 2: no duplicates in the nav. A duplicate href is a
	// rendering bug (buildNavigation produces a flat list, not a
	// set; duplicates would indicate a genuine collision).
	seen := map[string]int{}
	for _, h := range hrefs {
		seen[h[1]]++
	}
	for id, count := range seen {
		if count > 1 {
			t.Errorf("nav rail carries %d duplicate `href=\"#%s\"` entries; buildNavigation should produce a set", count, id)
		}
	}

	// Invariant 1: every href target has a matching `id="..."` in
	// the document body. Build a set of all `id="..."` attributes in
	// the rendered HTML first, then check each href resolves.
	idRE := regexp.MustCompile(`id="([^"]+)"`)
	allIDs := map[string]struct{}{}
	for _, m := range idRE.FindAllStringSubmatch(html, -1) {
		allIDs[m[1]] = struct{}{}
	}
	var missing []string
	for id := range seen {
		if _, ok := allIDs[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		// Cap the error list so a big regression doesn't spam
		// terminal output.
		report := missing
		if len(report) > 10 {
			report = report[:10]
		}
		t.Errorf("nav rail href targets have no matching `id=\"...\"` in the rendered document: %d missing; first 10: %v",
			len(missing), report)
	}
}
