package integration_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// binPath is the absolute path to a freshly-built my-gather binary.
// TestMain builds the binary once per package-test invocation and
// points this variable at it; each integration test invokes it via
// exec.Command.
var binPath string

// TestMain builds ./cmd/my-gather into a tempdir once before running
// any test in this package. Building once (rather than per-test)
// keeps the integration suite fast while still exercising the real
// compiled CLI path the user runs — exec-based tests that
// parse stderr / exit codes would be fragile against `go run`'s
// compile-and-forget overhead on each invocation.
//
// If the build fails the package exits 1 immediately; no test body
// runs, which is the right failure mode for an environment that
// can't produce the binary.
func TestMain(m *testing.M) {
	wd, err := filepath.Abs(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: abs cwd: %v\n", err)
		os.Exit(1)
	}
	// Walk up to repo root (go.mod) so the build command resolves
	// the ./cmd/my-gather path regardless of where `go test` is
	// invoked from.
	root := wd
	for {
		if fi, err := os.Stat(filepath.Join(root, "go.mod")); err == nil && !fi.IsDir() {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			fmt.Fprintf(os.Stderr, "TestMain: no go.mod walking up from %s\n", wd)
			os.Exit(1)
		}
		root = parent
	}

	tmp, err := os.MkdirTemp("", "mygather-int-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: mkdirtemp: %v\n", err)
		os.Exit(1)
	}
	binPath = filepath.Join(tmp, "my-gather")
	build := exec.Command("go", "build", "-o", binPath, "./cmd/my-gather")
	build.Dir = root
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: go build failed: %v\n", err)
		_ = os.RemoveAll(tmp)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

// repoRoot returns the repository root (directory containing go.mod)
// resolved by walking upward from the test's cwd. Integration tests
// use this to build deterministic paths to testdata/example2/.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	for dir := wd; ; {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod walking up from %s", wd)
		}
		dir = parent
	}
}
