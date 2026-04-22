// Package goldens is a tiny, dependency-free helper shared by every
// parser / render golden test in My-gather. It implements:
//
//   - the stdlib-only `-update` convention (rerun with
//     `go test ./... -update` to regenerate committed goldens);
//   - deterministic JSON marshalling for per-collector parsed models
//     (sorted keys, stable float precision);
//   - a single Compare entry point that reads the committed golden,
//     compares byte-for-byte, and either fails the test (normal mode)
//     or rewrites the golden (update mode).
//
// The package lives under tests/ rather than parse/ or model/ so no
// production import graph ever sees the `-update` flag. It ships no
// third-party dependencies (Principle X).
package goldens

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update is the shared flag: `go test ./... -update` rewrites every
// golden file referenced in this test run. Normal `go test ./...`
// runs compare against the committed golden and fail on any drift.
var update = flag.Bool("update", false, "rewrite committed golden files under testdata/golden/ from the current parser / render output")

// MarshalDeterministic renders v as indented JSON with sorted keys,
// suitable for a human-reviewed golden file. Fails the test on any
// marshalling error (that would indicate a non-serialisable shape
// in the model, which is a bug we want to surface loudly).
func MarshalDeterministic(t *testing.T, v any) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		t.Fatalf("MarshalDeterministic: %v", err)
	}
	return buf.Bytes()
}

// Compare reads the committed golden file at goldenPath and compares
// it against got.
//
// In normal mode, a mismatch fails the test with a readable diff
// excerpt pointing at the first differing byte.
//
// When `-update` is passed, Compare writes got to goldenPath (creating
// parent directories as needed) and logs a one-line confirmation so
// reviewers can diff the regenerated goldens before committing.
func Compare(t *testing.T, goldenPath string, got []byte) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("Compare: mkdir %s: %v", filepath.Dir(goldenPath), err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("Compare: write %s: %v", goldenPath, err)
		}
		t.Logf("goldens: rewrote %s (%d bytes)", goldenPath, len(got))
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Compare: read golden %s: %v (run `go test ./... -update` to create it)", goldenPath, err)
	}
	if bytes.Equal(want, got) {
		return
	}
	// Short excerpt around the first differing byte so the failure
	// message is actionable without needing to open a diff tool.
	idx := firstDiffIndex(want, got)
	wantExcerpt := excerpt(want, idx, 80)
	gotExcerpt := excerpt(got, idx, 80)
	t.Fatalf(
		"golden mismatch at %s (first differing byte at offset %d):\n  want: %q\n  got:  %q\nRun `go test ./... -update` to regenerate; review the diff before committing.",
		goldenPath, idx, wantExcerpt, gotExcerpt,
	)
}

// RepoRoot returns the absolute path of the repository root, resolved
// by walking upward from the test's cwd looking for `go.mod`. Every
// golden test uses this to build deterministic paths to
// `testdata/golden/*.json` regardless of what directory `go test`
// happened to cd into.
func RepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("RepoRoot: abs cwd: %v", err)
	}
	for dir := wd; dir != "/" && dir != ""; dir = filepath.Dir(dir) {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			return dir
		}
	}
	t.Fatalf("RepoRoot: could not find go.mod starting from %s", wd)
	return ""
}

func firstDiffIndex(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

func excerpt(buf []byte, idx, pad int) string {
	start := idx - pad
	if start < 0 {
		start = 0
	}
	end := idx + pad
	if end > len(buf) {
		end = len(buf)
	}
	return string(buf[start:end])
}
