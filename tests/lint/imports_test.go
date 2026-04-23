// Package lint_test enforces Constitution Principle IX (zero network
// at runtime) and Principle X (minimal dependencies) via a stdlib-only
// import-graph audit. It walks every .go file under the shipped
// packages (parse/, model/, render/, cmd/) and fails the build if any
// of them imports a network-capable stdlib package or an unjustified
// third-party module.
//
// Test-only files (*_test.go) and the tests/ tree itself are exempt.
package lint_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// bannedImports is the set of stdlib packages whose presence in the
// shipped binary would violate Principle IX.
var bannedImports = map[string]string{
	"net":        "would enable socket I/O (Principle IX)",
	"net/http":   "would enable HTTP I/O (Principle IX)",
	"net/rpc":    "would enable RPC I/O (Principle IX)",
	"net/smtp":   "would enable SMTP I/O (Principle IX)",
	"crypto/tls": "would enable TLS I/O (Principle IX)",
}

// repoModule is the module path; third-party imports are anything that
// isn't stdlib (no dot in the first path segment) and isn't us.
const repoModule = "github.com/matias-sanchez/My-gather"

// shippedDirs are the top-level directories whose import graph must be
// clean. Test files under any directory are skipped.
var shippedDirs = []string{"cmd", "parse", "model", "render"}

func TestNoForbiddenImports(t *testing.T) {
	// Walk from repo root. The test binary's CWD is the package dir,
	// so ascend until we find go.mod.
	root := findRepoRoot(t)

	offenders := map[string][]string{} // file -> list of bad imports

	fset := token.NewFileSet()
	for _, dir := range shippedDirs {
		full := filepath.Join(root, dir)
		_ = filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			for _, imp := range f.Imports {
				raw := imp.Path.Value
				pkg := strings.Trim(raw, `"`)
				if reason, banned := bannedImports[pkg]; banned {
					offenders[path] = append(offenders[path], pkg+": "+reason)
					continue
				}
				// Third-party module audit: non-stdlib imports must be
				// this module or an explicitly justified external
				// module. For v1 we allow zero external modules.
				if isExternalModule(pkg) && !strings.HasPrefix(pkg, repoModule) {
					offenders[path] = append(offenders[path],
						pkg+": unjustified non-stdlib dependency (Principle X)")
				}
			}
			return nil
		})
	}

	if len(offenders) == 0 {
		return
	}

	paths := make([]string, 0, len(offenders))
	for p := range offenders {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		sort.Strings(offenders[p])
		for _, msg := range offenders[p] {
			t.Errorf("%s: %s", p, msg)
		}
	}
}

// isExternalModule reports whether pkg looks like a non-stdlib,
// non-relative import path. Stdlib packages don't contain a '.' in
// their first path segment ("fmt", "net/http", "encoding/json");
// external modules always do (github.com/foo/bar, golang.org/x/…).
func isExternalModule(pkg string) bool {
	first, _, _ := strings.Cut(pkg, "/")
	return strings.Contains(first, ".")
}

// findRepoRoot ascends from the test's working directory until it
// finds a go.mod. Tests run with CWD == the package dir, which is
// several levels under the repo root.
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

// _ references fs to keep the import after edits; io/fs is needed by
// filepath.WalkDir's callback signature (fs.DirEntry).
var _ fs.FileInfo = nil
