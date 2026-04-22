// Package coverage_test also enforces the golden half of spec SC-006 /
// Principle VIII: every supported collector Suffix MUST have at least
// one golden output file under testdata/golden/. This is the
// companion hard-failure test that T021 (testdata_coverage_test.go)
// mentioned as "promoted to a hard failure by task T080".
//
// T080 strengthens T021 in two ways:
//
//  1. T021 only asserts a FIXTURE exists; T080 additionally asserts a
//     GOLDEN exists. A parser can drift its output shape silently if
//     neither side is checked — the fixture keeps parsing but the
//     golden never regenerated, so the pipeline stays green while the
//     shipped report diverges from what the committed golden records.
//  2. T080 accepts either a per-suffix parser JSON golden (`<suffix>.
//     example*.<timestamp>.json`) or a render-section HTML golden
//     bundle (`os.example2.html`, `db.example2.html`,
//     `variables.example2.html`) that is known to carry the suffix.
//     A suffix is "covered" if at least one of the two shapes exists.
package coverage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/model"
)

// renderBundles lists the render-section HTML goldens that carry
// multiple collector outputs in one file. A suffix listed as a
// member here is considered "golden-covered" if the named bundle
// golden exists, even without a per-suffix parser JSON golden.
//
// This mirrors the real render layer: `render/templates/db.html.tmpl`
// packs innodbstatus + mysqladmin + processlist into one rendered
// `<details id="sec-db">` section whose golden is `db.example2.html`;
// `render/templates/os.html.tmpl` packs iostat + top + vmstat into
// `os.example2.html`.
var renderBundles = map[model.Suffix]string{
	model.SuffixIostat:       "os",
	model.SuffixTop:          "os",
	model.SuffixVmstat:       "os",
	model.SuffixVariables:    "variables",
	model.SuffixInnodbStatus: "db",
	model.SuffixMysqladmin:   "db",
	model.SuffixProcesslist:  "db",
}

// TestEverySuffixHasGolden asserts at least one golden file under
// testdata/golden/ covers each model.Suffix. "Covers" means either a
// parser JSON golden whose filename starts with the suffix, or a
// render-section HTML golden bundle that is known to carry the
// suffix. Both shapes are accepted so the test does not over-
// prescribe golden layout: a future refactor that merges per-parser
// JSON goldens into a single bundle, or adds a new per-section
// HTML golden, should only need to update `renderBundles`.
func TestEverySuffixHasGolden(t *testing.T) {
	root := findRepoRoot(t)
	goldenDir := filepath.Join(root, "testdata", "golden")

	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("read %s: %v", goldenDir, err)
	}
	if len(entries) == 0 {
		t.Fatalf("testdata/golden/ is empty; SC-006 requires at least one golden per supported suffix")
	}
	var goldenFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		goldenFiles = append(goldenFiles, e.Name())
	}

	for _, suffix := range model.KnownSuffixes {
		suffix := suffix
		t.Run(string(suffix), func(t *testing.T) {
			// Parser JSON golden shape: filename starts with the
			// suffix, e.g. "iostat.example2.2026_04_21_16_51_41.json".
			// The SuffixInnodbStatus constant is "innodbstatus1" but
			// the shipped parser-golden filename is "innodbstatus."
			// without the trailing digit — the parser strips it so
			// the golden name mirrors the logical collector. Match
			// both the raw suffix prefix AND the digit-stripped form
			// to avoid false negatives.
			parserCandidates := []string{
				string(suffix) + ".",
				strings.TrimRight(string(suffix), "0123456789") + ".",
			}
			parserGoldenFound := false
			for _, name := range goldenFiles {
				for _, cand := range parserCandidates {
					if strings.HasPrefix(name, cand) {
						parserGoldenFound = true
						break
					}
				}
				if parserGoldenFound {
					break
				}
			}

			// Render bundle golden shape: the suffix is a member of
			// a render-section bundle whose HTML golden exists under
			// testdata/golden/<bundle>.example*.html.
			bundleGoldenFound := false
			bundleName := renderBundles[suffix]
			if bundleName != "" {
				prefix := bundleName + "."
				for _, name := range goldenFiles {
					if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".html") {
						bundleGoldenFound = true
						break
					}
				}
			}

			if !parserGoldenFound && !bundleGoldenFound {
				t.Errorf("no golden under testdata/golden/ covers Suffix %q — neither a parser JSON golden (filename starting with %q or %q) nor a render bundle HTML golden (%q.example*.html) was found. SC-006 requires at least one.",
					suffix, parserCandidates[0], parserCandidates[1], bundleName)
			}
		})
	}
}
