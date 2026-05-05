// Package coverage_test enforces Constitution Principle VIII and spec
// FR-021: every supported collector Suffix MUST have at least one
// fixture under testdata/ and at least one parser golden under
// testdata/golden/.
package coverage_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/tests/goldens"
)

func TestEverySuffixHasFixture(t *testing.T) {
	root := goldens.RepoRoot(t)
	testdata := filepath.Join(root, "testdata")

	for _, suffix := range model.KnownSuffixes {
		suffix := suffix
		t.Run(string(suffix), func(t *testing.T) {
			found := false
			_ = filepath.WalkDir(testdata, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				// Match filenames of the form
				// "YYYY_MM_DD_HH_MM_SS-<suffix>" (the pt-stalk convention)
				// or ending in "-<suffix>".
				if strings.HasSuffix(d.Name(), "-"+string(suffix)) {
					found = true
					return filepath.SkipAll
				}
				return nil
			})
			if !found {
				t.Errorf("no fixture under testdata/ for Suffix %q (Principle VIII / FR-021)",
					suffix)
			}
		})
	}
}

func TestEverySuffixHasParserGolden(t *testing.T) {
	root := goldens.RepoRoot(t)
	goldenDir := filepath.Join(root, "testdata", "golden")
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("read %s: %v", goldenDir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}

	for _, suffix := range model.KnownSuffixes {
		suffix := suffix
		t.Run(string(suffix), func(t *testing.T) {
			token := parserGoldenToken(suffix)
			for _, name := range names {
				if strings.HasPrefix(name, token+".") && strings.HasSuffix(name, ".json") {
					return
				}
			}
			t.Fatalf("no parser golden under testdata/golden/ for Suffix %q (Principle VIII / FR-021)", suffix)
		})
	}
}

func parserGoldenToken(suffix model.Suffix) string {
	switch suffix {
	case model.SuffixInnodbStatus:
		return "innodbstatus"
	default:
		return string(suffix)
	}
}
