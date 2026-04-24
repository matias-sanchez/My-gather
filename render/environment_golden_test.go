package render_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// TestEnvironmentGoldenHTML — Principle VIII: the Environment section
// is a top-level `<details id="sec-environment">` block driven by
// real env sidecars (-hostname, -sysctl, -procstat, -top, -meminfo,
// -df, -output) plus the MySQL -variables. Before this test only
// the view-builder was exercised by an in-memory unit test, so a
// template typo, a missing {{if}} branch, or a field-name mismatch
// between envView and the template would render silently wrong.
//
// Drives parse.Discover on testdata/example2/ (which carries the env
// sidecars committed in 826f2c2) and keeps only -variables so the
// MySQL panel has real data; the host panel is populated from the
// always-present env sidecar map on the Collection. Extracts the
// sec-environment block and snapshot-compares against a golden.
func TestEnvironmentGoldenHTML(t *testing.T) {
	root := goldens.RepoRoot(t)
	goldenPath := filepath.Join(root, "testdata", "golden", "environment.example2.html")

	html := renderGolden(t, model.SuffixVariables)
	section := extractDetailsSection(t, html, "sec-environment")

	goldens.Compare(t, goldenPath, []byte(section))
}

// TestEnvironmentSectionEmittedInEveryRender — sec-environment must
// appear in every report regardless of which collectors are present.
// The section header is rendered unconditionally in report.html.tmpl;
// this test pins that contract so a future refactor can't silently
// hide the section when the host or MySQL panels happen to be empty.
func TestEnvironmentSectionEmittedInEveryRender(t *testing.T) {
	// No collectors kept — both panels should degrade to their
	// "missing" banner, but the outer `<details id="sec-environment">`
	// must still be emitted so the nav rail's href resolves.
	html := renderGolden(t /* no suffixes */)
	if !strings.Contains(html, `id="sec-environment"`) {
		t.Errorf(`sec-environment not found in an empty-collector render; Environment header must be unconditional so the nav rail href always resolves`)
	}
}
