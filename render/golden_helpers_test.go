package render_test

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/parse"
	"github.com/matias-sanchez/My-gather/render"
	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// discoverExample2 runs the full parse.Discover pipeline against the
// committed testdata/example2/ fixture and returns the populated
// Collection. Render golden tests use this instead of
// render_test.go's hand-built minimalCollection / twoSnapshotCollection
// helpers because a real parser-driven render is the only way to catch
// regressions where the rendered HTML diverges from what the parsers
// actually produce (a hand-built Collection can drift from parser
// output without the render golden noticing).
func discoverExample2(t *testing.T) *model.Collection {
	t.Helper()
	root := goldens.RepoRoot(t)
	fixtureDir := filepath.Join(root, "testdata", "example2")
	c, err := parse.Discover(context.Background(), fixtureDir, parse.DiscoverOptions{})
	if err != nil {
		t.Fatalf("parse.Discover(%s): %v", fixtureDir, err)
	}
	if c == nil {
		t.Fatalf("parse.Discover returned nil Collection")
	}
	return c
}

// keepOnly returns a shallow copy of c in which every Snapshot's
// SourceFiles map is filtered to retain only the suffixes in `keep`.
// Used by per-section golden tests to render a Collection with just
// one section's parsers wired (the other sections render their
// "data not available" banner). This matches the per-section task
// descriptions in specs/001-.../tasks.md (T046, T057, T069).
//
// The original Collection is not mutated: a fresh Collection with
// fresh Snapshot pointers and fresh SourceFiles maps is returned.
func keepOnly(c *model.Collection, keep ...model.Suffix) *model.Collection {
	keepSet := make(map[model.Suffix]struct{}, len(keep))
	for _, k := range keep {
		keepSet[k] = struct{}{}
	}
	out := &model.Collection{
		RootPath: c.RootPath,
		Hostname: c.Hostname,
		// Env-sidecar data is not keyed by Suffix (it lives outside the
		// Snapshot.SourceFiles tree) and is shared across sections. Pass
		// it through so per-section goldens that touch the Environment
		// section still have the host facts they need.
		RawEnvSidecars:       c.RawEnvSidecars,
		EnvMeminfo:           c.EnvMeminfo,
		EnvSidecarTimestamps: c.EnvSidecarTimestamps,
	}
	for _, s := range c.Snapshots {
		filtered := make(map[model.Suffix]*model.SourceFile, len(keep))
		for suf, sf := range s.SourceFiles {
			if _, ok := keepSet[suf]; ok {
				filtered[suf] = sf
			}
		}
		out.Snapshots = append(out.Snapshots, &model.Snapshot{
			Timestamp:   s.Timestamp,
			Prefix:      s.Prefix,
			SourceFiles: filtered,
		})
	}
	return out
}

// renderGolden runs Render(keepOnly(discover, keep...), fixedOpts)
// and returns the full HTML. Tests typically pass the result to
// extractDetailsSection to narrow down to one `<details id="sec-X">`
// block before calling goldens.Compare.
func renderGolden(t *testing.T, keep ...model.Suffix) string {
	t.Helper()
	c := keepOnly(discoverExample2(t), keep...)
	var buf bytes.Buffer
	opts := render.RenderOptions{GeneratedAt: fixedTime(), Version: "v0.0.1-test"}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("render.Render: %v", err)
	}
	return buf.String()
}

// extractDetailsSection returns the single top-level
// `<details id="{sectionID}" …>…</details>` block from html,
// respecting arbitrarily nested `<details>` children (per FR-032 the
// section `<details>` contains per-subview `<details>` children).
//
// Returns "" and fails the test if the section is not found or is
// unterminated — the render output is deterministic, so either case
// is a real regression rather than a flake.
func extractDetailsSection(t *testing.T, html, sectionID string) string {
	t.Helper()
	anchor := fmt.Sprintf(`<details id=%q`, sectionID)
	// Fall back to attribute-order-tolerant match: a reviewer could
	// reorder id/class/data-* attributes without changing semantics.
	start := strings.Index(html, anchor)
	if start == -1 {
		// Try the single-quote / no-quote variants if the template
		// ever shifts style. Keeping the simple quoted form first
		// because that is what the existing templates emit today.
		alt := fmt.Sprintf(`id=%q`, sectionID)
		idx := strings.Index(html, alt)
		if idx == -1 {
			t.Fatalf("extractDetailsSection: %q not found", sectionID)
		}
		// Walk back to the preceding `<details` on the same element.
		back := strings.LastIndex(html[:idx], "<details")
		if back == -1 {
			t.Fatalf("extractDetailsSection: %q not inside a <details>", sectionID)
		}
		start = back
	}
	// Walk forward, tracking <details> / </details> nesting depth.
	depth := 0
	i := start
	for i < len(html) {
		switch {
		case strings.HasPrefix(html[i:], "<details"):
			depth++
			// Skip to end of the opening tag.
			gt := strings.Index(html[i:], ">")
			if gt == -1 {
				t.Fatalf("extractDetailsSection: unterminated <details at offset %d", i)
			}
			i += gt + 1
		case strings.HasPrefix(html[i:], "</details>"):
			depth--
			i += len("</details>")
			if depth == 0 {
				return html[start:i]
			}
		default:
			i++
		}
	}
	t.Fatalf("extractDetailsSection: section %q is unterminated (depth=%d)", sectionID, depth)
	return ""
}

// extractJSONPayload returns the JSON text embedded in the
// `<script id="report-data" type="application/json">…</script>` tag.
// The chart payload + category map + defaults map all flow through
// this payload; markup tests that need to assert the JSON shape
// (T070 toggle markup, T098..T104 feature tests) use this.
func extractJSONPayload(t *testing.T, html string) string {
	t.Helper()
	start := strings.Index(html, `id="report-data"`)
	if start == -1 {
		t.Fatalf("extractJSONPayload: no id=\"report-data\" found")
	}
	// Skip past the opening `<script …>` tag.
	gt := strings.Index(html[start:], ">")
	if gt == -1 {
		t.Fatalf("extractJSONPayload: unterminated <script> opening tag")
	}
	bodyStart := start + gt + 1
	bodyEnd := strings.Index(html[bodyStart:], "</script>")
	if bodyEnd == -1 {
		t.Fatalf("extractJSONPayload: unterminated <script> body")
	}
	return html[bodyStart : bodyStart+bodyEnd]
}
