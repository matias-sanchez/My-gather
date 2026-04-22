package integration_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestE2EExample2: T032 / FR-005 / FR-020 / Principle I.
//
// Run the compiled my-gather binary against `testdata/example2/`
// and verify the end-to-end contract: exit 0, the output HTML exists
// at the requested path, and the rendered document contains the
// three top-level section headers plus the verbatim passthrough
// banner required by FR-026.
//
// This is the canonical "does the binary work" integration test. A
// regression that broke parsing, rendering, or the CLI entry point
// would fail here regardless of which specific layer broke.
func TestE2EExample2(t *testing.T) {
	root := repoRoot(t)
	input := filepath.Join(root, "testdata", "example2")
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "report.html")

	cmd := exec.Command(binPath, "--out", out, input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("my-gather exit non-zero: %v\nstderr:\n%s", err, stderr.String())
	}

	// Output file exists and has non-zero size.
	st, err := os.Stat(out)
	if err != nil {
		t.Fatalf("output not written at %q: %v", out, err)
	}
	if st.Size() == 0 {
		t.Fatalf("output at %q is empty", out)
	}

	html, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	htmlStr := string(html)

	// The three top-level <details> sections MUST be present. Using
	// the `id="sec-…"` anchors (rather than a header-text match) so
	// the test doesn't couple to human-readable copy.
	for _, id := range []string{`id="sec-os"`, `id="sec-variables"`, `id="sec-db"`} {
		if !strings.Contains(htmlStr, id) {
			t.Errorf("rendered HTML missing %q; T032 requires all three top-level sections", id)
		}
	}

	// FR-026 verbatim-passthrough banner (header text).
	if !strings.Contains(htmlStr, `Verbatim passthrough:`) {
		t.Errorf(`rendered HTML missing "Verbatim passthrough:" banner; FR-026 requires it`)
	}
}
