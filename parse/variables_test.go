package parse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// TestVariablesGolden: T056 / FR-012 / Principle VIII.
//
// Parse the committed `-variables` fixture from testdata/example2/
// and compare the resulting *VariablesData against a committed
// golden JSON under testdata/golden/. Any drift in
// deduplication / sort / value extraction fails the test with a
// readable pointer to the first differing byte; `go test ./... -update`
// rewrites the golden for human review.
func TestVariablesGolden(t *testing.T) {
	root := goldens.RepoRoot(t)
	fixture := filepath.Join(root, "testdata", "example2", "2026_04_21_16_51_41-variables")
	goldenPath := filepath.Join(root, "testdata", "golden", "variables.example2.2026_04_21_16_51_41.json")

	f, err := os.Open(fixture)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	data, diags := parseVariables(f, fixture)
	if data == nil {
		t.Fatalf("parseVariables returned nil data (diagnostics: %+v)", diags)
	}

	// Shape invariants that golden-comparison alone would not catch
	// clearly — fail fast on the easy-to-interpret ones so the golden
	// diff is never the first thing a reviewer has to read.
	if len(data.Entries) == 0 {
		t.Fatalf("expected at least one VariableEntry; got zero")
	}
	for i := 1; i < len(data.Entries); i++ {
		if data.Entries[i-1].Name >= data.Entries[i].Name {
			t.Errorf("entries not strictly sorted by Name (FR-012): %q >= %q at index %d",
				data.Entries[i-1].Name, data.Entries[i].Name, i)
		}
	}
	seen := map[string]bool{}
	for _, e := range data.Entries {
		if seen[e.Name] {
			t.Errorf("duplicate entry after parse (FR-012): %q", e.Name)
		}
		seen[e.Name] = true
	}

	// Snapshot-compare the full parsed struct so any future drift in
	// value extraction, quoting, whitespace handling, or the set of
	// retained keys surfaces as a reviewable diff.
	got := goldens.MarshalDeterministic(t, struct {
		Entries     any `json:"entries"`
		Diagnostics any `json:"diagnostics"`
	}{
		Entries:     data.Entries,
		Diagnostics: diags,
	})
	goldens.Compare(t, goldenPath, got)
}
