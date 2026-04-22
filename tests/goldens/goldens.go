// Package goldens is a tiny, dependency-free helper shared by every
// parser / render golden test in My-gather. It implements:
//
//   - the stdlib-only `-update` convention for regenerating committed
//     goldens (see the flag note below);
//   - deterministic JSON marshalling for per-collector parsed models
//     (sorted keys, stable float precision);
//   - a single Compare entry point that reads the committed golden,
//     compares byte-for-byte, and either fails the test (normal mode)
//     or rewrites the golden (update mode).
//
// The package lives under tests/ rather than parse/ or model/ so no
// production import graph ever sees the `-update` flag. It ships no
// third-party dependencies (Principle X).
//
// Scope of the `-update` flag. Go's flag registration is per-test-
// binary: `-update` is only recognised by the `go test` invocation of
// packages whose test binary imports `tests/goldens`. Running
// `go test ./... -update` from the repo root today fails in packages
// that do not use goldens (render, tests/lint, tests/coverage) with
// `flag provided but not defined: -update`. Regenerate goldens by
// scoping the invocation to goldens-using packages — currently
// `go test ./parse/... -update` — and extend the scope as more
// packages adopt goldens. Deliberately not adding blank-import
// stubs to every test package: that is speculative scaffolding the
// repo does not yet need (Principle X).
package goldens

import (
	"bytes"
	"encoding/json"
	"flag"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// update is the shared flag. The flag is registered only in the test
// binary of packages that import `tests/goldens`; scope the `go test`
// invocation accordingly (see the package godoc's "Scope of the
// `-update` flag" note). Typical regeneration today:
// `go test ./parse/... -update`.
var update = flag.Bool("update", false, "rewrite committed golden files under testdata/golden/ from the current parser / render output")

// MarshalDeterministic renders v as indented JSON suitable for a
// human-reviewed golden file: two-space indent, HTML escaping
// disabled (so `<` / `>` / `&` appear verbatim in rendered-HTML
// goldens). String-keyed maps are sorted by key deterministically by
// `encoding/json`, and our model types use struct fields in declared
// order, so two invocations on the same value produce byte-identical
// output.
//
// NaN handling. `encoding/json` rejects non-finite floats (NaN / Inf)
// with an `UnsupportedValueError`. Parsers that deliberately emit
// NaN — `parse/mysqladmin` at cross-Snapshot boundaries (FR-030) and
// drift slots (research R8) — MUST scrub those slots BEFORE calling
// this helper. Use `ScrubFloats` below, which rewrites NaN / ±Inf in
// a `map[string][]float64` to JSON-encodable nulls. Keeping the
// scrub as an opt-in helper (rather than baking it into
// MarshalDeterministic) avoids a reflection walk that would change
// struct field ordering in already-committed goldens.
//
// A marshalling error fails the test loudly — that indicates either
// a missing ScrubFloats call or a non-serialisable shape in the
// model, both of which we want to surface rather than silently
// paper over.
func MarshalDeterministic(t *testing.T, v any) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		t.Fatalf("MarshalDeterministic: %v (if a NaN reached here, pass the offending map through ScrubFloats first)", err)
	}
	return buf.Bytes()
}

// ScrubFloats returns a copy of m in which every NaN / ±Inf value has
// been rewritten to nil (`null` after JSON encoding). Finite values
// pass through unchanged. The returned map is `map[string][]any`
// because JSON's type system does not have `float64 | null`; slot-
// level `any` is the shape MarshalDeterministic can encode.
//
// Typical use (mysqladmin): `ScrubFloats(data.Deltas)` before
// including in the payload that feeds MarshalDeterministic.
func ScrubFloats(m map[string][]float64) map[string][]any {
	if m == nil {
		return nil
	}
	out := make(map[string][]any, len(m))
	for k, vs := range m {
		scrubbed := make([]any, len(vs))
		for i, v := range vs {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				scrubbed[i] = nil
			} else {
				scrubbed[i] = v
			}
		}
		out[k] = scrubbed
	}
	return out
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
		t.Fatalf("Compare: read golden %s: %v (run `go test ./parse/... -update` to create it — the -update flag is only registered in packages that import tests/goldens, so scope the invocation to goldens-using packages)", goldenPath, err)
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
		"golden mismatch at %s (first differing byte at offset %d):\n  want: %q\n  got:  %q\nRun `go test ./parse/... -update` (or the appropriate goldens-using package scope) to regenerate; review the diff before committing.",
		goldenPath, idx, wantExcerpt, gotExcerpt,
	)
}

// RepoRoot returns the absolute path of the repository root, resolved
// by walking upward from the test's cwd looking for `go.mod`. Every
// golden test uses this to build deterministic paths to
// `testdata/golden/*.json` regardless of what directory `go test`
// happened to cd into.
//
// Terminates when `filepath.Dir(dir) == dir` rather than when the
// string equals "/". Unix roots round-trip through Dir as "/", but on
// Windows `filepath.Dir("C:\\") == "C:\\"` — a naive string compare
// against "/" never terminates and the test hangs. The same equality
// check covers both conventions.
func RepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("RepoRoot: abs cwd: %v", err)
	}
	for dir := wd; ; {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir || parent == "" {
			break
		}
		dir = parent
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
