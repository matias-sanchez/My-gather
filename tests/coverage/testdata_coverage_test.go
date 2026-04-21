// Package coverage_test enforces Constitution Principle VIII and spec
// FR-021: every supported collector Suffix MUST have at least one
// fixture under testdata/ and is expected to have a golden output file
// once its parser lands.
//
// In Phase 2 (this point in implementation) only fixtures are
// mandatory. Goldens land per-parser in Phase 4–6 (tasks T046, T057,
// T069). The golden check is currently a soft warning; it will be
// promoted to a hard failure by task T080.
package coverage_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/model"
)

func TestEverySuffixHasFixture(t *testing.T) {
	root := findRepoRoot(t)
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

// findRepoRoot ascends from the test's working directory until it
// finds a go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs cwd: %v", err)
	}
	for dir := wd; dir != "/" && dir != ""; dir = filepath.Dir(dir) {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			return dir
		}
	}
	t.Fatalf("could not find go.mod starting from %s", wd)
	return ""
}
