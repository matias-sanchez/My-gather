package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// fixtureDir resolves the committed testdata/example2/ directory from
// anywhere inside the repo. Tests use it as the canonical input for
// CLI invocations so the exit codes and stderr output match what a
// user running the binary against the reference fixture would see.
func fixtureDir(t *testing.T) string {
	t.Helper()
	wd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	for dir := wd; ; {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "testdata", "example2")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod from %s", wd)
		}
		dir = parent
	}
}

// invokeRun calls run(args, stdout, stderr) and returns exit code +
// captured stdio. Tests use the returned buffers to assert both the
// normative exit code (contracts/cli.md) and the stderr format
// (FR-027 / F30 resolution).
func invokeRun(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// TestOutputInsideInputRejected: T030 / FR-029 / Principle II.
//
// The CLI MUST refuse to write the output file inside the input
// directory tree, because that would mutate an otherwise read-only
// input (Principle II) and create a dependency cycle on re-runs
// (the next invocation would see the report.html file as a pt-stalk
// file and try to parse it). Exit 7 is reserved for this case in
// contracts/cli.md.
func TestOutputInsideInputRejected(t *testing.T) {
	input := fixtureDir(t)
	// Point --out at a path INSIDE the input directory. The resolver
	// in run() detects this via a path-is-under check and returns
	// exitOutputInsideIn (7) before opening any file.
	inside := filepath.Join(input, "report.html")
	code, _, stderr := invokeRun("--out", inside, input)
	if code != 7 {
		t.Errorf("exit code = %d; want 7 (exitOutputInsideIn); stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "resolves inside input directory") {
		t.Errorf("stderr does not mention the inside-input refusal; got:\n%s", stderr)
	}
	// Belt-and-suspenders: the path should NOT have been written.
	if _, err := os.Stat(inside); err == nil {
		t.Errorf("output file was written at %q despite the rejection", inside)
		_ = os.Remove(inside) // clean up to not poison the fixture
	}
}

// TestOverwriteGuard: T031 / FR-002.
//
// A second invocation with the same --out path MUST exit 6 (output
// exists) unless --overwrite is passed. Guards against silently
// clobbering a report the user may still be reviewing.
func TestOverwriteGuard(t *testing.T) {
	input := fixtureDir(t)
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "report.html")

	// First invocation: should succeed.
	if code, _, stderr := invokeRun("--out", out, input); code != 0 {
		t.Fatalf("first run exit = %d; want 0; stderr:\n%s", code, stderr)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("first run did not write %q: %v", out, err)
	}

	// Second invocation without --overwrite: must exit 6.
	code, _, stderr := invokeRun("--out", out, input)
	if code != 6 {
		t.Errorf("second run exit = %d; want 6 (exitOutputExists); stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("stderr does not mention overwrite guard; got:\n%s", stderr)
	}

	// Third invocation WITH --overwrite: must succeed. Guards against
	// a regression where the flag stopped being honoured.
	if code, _, stderr := invokeRun("--out", out, "--overwrite", input); code != 0 {
		t.Errorf("run with --overwrite exit = %d; want 0; stderr:\n%s", code, stderr)
	}
}

// TestOutputParentMissing: T086 / F8 resolution.
//
// --out pointing at a non-existent parent directory MUST fail cleanly
// with exitInputPath (3) and a message naming the missing directory.
// Auto-mkdir is explicitly NOT done (per F8 resolution in
// specs/tasks.md); the user keeps control of where report files land.
//
// Without this early check the atomic-write path would fall through
// to `os.CreateTemp(dir, …)` which fails with a "no such file or
// directory" error and mapped to exitInternal (70), which is a
// confusing exit code for a recoverable user error.
func TestOutputParentMissing(t *testing.T) {
	input := fixtureDir(t)
	tmpDir := t.TempDir()
	// A path whose parent does not exist.
	out := filepath.Join(tmpDir, "does-not-exist", "report.html")

	code, _, stderr := invokeRun("--out", out, input)
	if code != 3 {
		t.Errorf("exit code = %d; want 3 (exitInputPath); stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "does not exist") {
		t.Errorf("stderr does not mention the missing output parent; got:\n%s", stderr)
	}
}

// TestOutputFileSingletonNoSideCar: T088 / FR-002 (F27 remediation).
//
// A successful run MUST leave exactly one new file in the output
// parent directory — the target report. No `.tmp`, `.bak`, `.part`,
// or lock-file residue. Walks the parent dir before and after the
// run and diffs.
func TestOutputFileSingletonNoSideCar(t *testing.T) {
	input := fixtureDir(t)
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "report.html")

	beforeSet := dirEntries(t, tmpDir)

	if code, _, stderr := invokeRun("--out", out, input); code != 0 {
		t.Fatalf("exit = %d; want 0; stderr:\n%s", code, stderr)
	}

	afterSet := dirEntries(t, tmpDir)

	// Diff: every entry in `after` not present in `before` is a new
	// file. The only acceptable new entry is `report.html`.
	newEntries := map[string]struct{}{}
	for name := range afterSet {
		if _, wasBefore := beforeSet[name]; !wasBefore {
			newEntries[name] = struct{}{}
		}
	}
	if len(newEntries) != 1 {
		t.Errorf("expected exactly 1 new file in output parent; got %d: %v", len(newEntries), keysOf(newEntries))
	}
	if _, ok := newEntries["report.html"]; !ok {
		t.Errorf("the single new file should be report.html; got: %v", keysOf(newEntries))
	}
	// Explicit names to guard against a regression where the
	// atomic-write temp file is left behind after a panic / signal.
	for _, suffix := range []string{".tmp", ".bak", ".part", ".lock"} {
		for name := range newEntries {
			if strings.Contains(name, suffix) {
				t.Errorf("new file %q contains %q suffix; FR-002 requires a single output file with no side-cars",
					name, suffix)
			}
		}
	}
}

// TestStderrSilentOnSuccess: T091 / FR-027 (F30 resolution 1/3).
//
// A successful run against a clean fixture MUST leave stderr empty —
// SeverityInfo diagnostics are never mirrored (F13), so if the
// fixture produces only Info diagnostics the run is silent. The
// example2 fixture is clean enough to meet this bar.
func TestStderrSilentOnSuccess(t *testing.T) {
	input := fixtureDir(t)
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "report.html")

	code, _, stderr := invokeRun("--out", out, input)
	if code != 0 {
		t.Fatalf("exit = %d; want 0; stderr:\n%s", code, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr should be empty on a clean success; got:\n%s", stderr)
	}
}

// TestStderrWarningMirrored: T092 / FR-027 (F30 resolution 2/3).
//
// When a source file is unreadable (truncated / missing), the parser
// emits a SeverityWarning diagnostic and the CLI MUST mirror it to
// stderr in the format
//
//     [warning] <basename>: <message>
//
// The test truncates one of example2's source files, invokes the
// CLI, and asserts a warning line matching the format is emitted.
// We tolerate additional warnings from other files — the invariant
// is "at least one warning line in the required format is present".
func TestStderrWarningMirrored(t *testing.T) {
	input := fixtureDir(t)
	// Copy example2 to a tempdir so we can truncate one source file
	// without damaging the committed fixture.
	scratch := t.TempDir()
	copyDir(t, input, scratch)
	// Truncate the iostat file to half its length to force a Warning.
	// iostat is the canonical partial-recovery test target
	// (parse/iostat_test.go::TestIostatPartialRecovery already pins
	// that the parser emits a Warning on a mid-row cut).
	target := filepath.Join(scratch, "2026_04_21_16_51_41-iostat")
	truncateFile(t, target)

	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "report.html")

	code, _, stderr := invokeRun("--out", out, scratch)
	if code != 0 {
		t.Fatalf("exit = %d; want 0 (partial recovery per Principle III); stderr:\n%s", code, stderr)
	}
	warningRE := regexp.MustCompile(`\[warning\]\s+\S+:\s+.+`)
	if !warningRE.MatchString(stderr) {
		t.Errorf("stderr does not contain a line matching %q; got:\n%s", warningRE, stderr)
	}
}

// TestStderrVerboseProgress: T093 / FR-027 (F30 resolution 3/3).
//
// With -v, stderr MUST carry `[parse] …`, `[render] writing …`, and
// `[done] …` progress lines in that order. SeverityInfo diagnostics
// do NOT appear on stderr (F13 behaviour) — the verbose channel is
// a separate surface that only carries phase markers.
func TestStderrVerboseProgress(t *testing.T) {
	input := fixtureDir(t)
	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "report.html")

	code, _, stderr := invokeRun("-v", "--out", out, input)
	if code != 0 {
		t.Fatalf("exit = %d; want 0; stderr:\n%s", code, stderr)
	}
	// Order-preserving substring scan.
	idxParse := strings.Index(stderr, "[parse] ")
	idxRender := strings.Index(stderr, "[render] writing ")
	idxDone := strings.Index(stderr, "[done]")
	if idxParse == -1 {
		t.Errorf("stderr missing `[parse] …` line; got:\n%s", stderr)
	}
	if idxRender == -1 {
		t.Errorf("stderr missing `[render] writing …` line; got:\n%s", stderr)
	}
	if idxDone == -1 {
		t.Errorf("stderr missing `[done] …` line; got:\n%s", stderr)
	}
	if idxParse != -1 && idxRender != -1 && idxParse >= idxRender {
		t.Errorf("[parse] must precede [render] in stderr; parse@%d render@%d\n%s",
			idxParse, idxRender, stderr)
	}
	if idxRender != -1 && idxDone != -1 && idxRender >= idxDone {
		t.Errorf("[render] must precede [done] in stderr; render@%d done@%d\n%s",
			idxRender, idxDone, stderr)
	}
}

// TestVersionOutput: T095 / FR-023.
//
// --version prints the five-line format from contracts/cli.md to
// stdout (NOT stderr) and exits 0. The test patches the build-time
// injectables (version / commit / builtAt) with known values via
// package-level overrides and asserts each expected line is present
// in order.
func TestVersionOutput(t *testing.T) {
	// Save + override + restore the build-time variables so the
	// assertions are deterministic regardless of how the test binary
	// was built.
	savedVersion, savedCommit, savedBuiltAt := version, commit, builtAt
	defer func() {
		version, commit, builtAt = savedVersion, savedCommit, savedBuiltAt
	}()
	version = "v9.9.9-test"
	commit = "deadbeef"
	builtAt = "2026-04-22T00:00:00Z"

	code, stdout, stderr := invokeRun("--version")
	if code != 0 {
		t.Fatalf("exit = %d; want 0", code)
	}
	if stderr != "" {
		t.Errorf("--version emitted stderr; got:\n%s", stderr)
	}
	// The contracts/cli.md format is five lines:
	//   my-gather <version>
	//     commit:   <commit>
	//     go:       <go-version>
	//     built:    <date>
	//     platform: <GOOS>/<GOARCH>
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines in --version output; got %d:\n%s", len(lines), stdout)
	}
	if !strings.HasPrefix(lines[0], "my-gather v9.9.9-test") {
		t.Errorf("line 0 = %q; want `my-gather v9.9.9-test`", lines[0])
	}
	if !strings.Contains(lines[1], "commit:") || !strings.Contains(lines[1], "deadbeef") {
		t.Errorf("line 1 = %q; want to contain `commit:` and `deadbeef`", lines[1])
	}
	if !strings.Contains(lines[2], "go:") {
		t.Errorf("line 2 = %q; want to contain `go:`", lines[2])
	}
	if !strings.Contains(lines[3], "built:") || !strings.Contains(lines[3], "2026-04-22T00:00:00Z") {
		t.Errorf("line 3 = %q; want to contain `built:` and the injected timestamp", lines[3])
	}
	if !strings.Contains(lines[4], "platform:") || !strings.Contains(lines[4], "/") {
		t.Errorf("line 4 = %q; want to contain `platform: GOOS/GOARCH`", lines[4])
	}
}

// --- helpers ---------------------------------------------------------

// dirEntries returns the immediate children of dir as a set.
func dirEntries(t *testing.T, dir string) map[string]struct{} {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %q: %v", dir, err)
	}
	out := make(map[string]struct{}, len(es))
	for _, e := range es {
		out[e.Name()] = struct{}{}
	}
	return out
}

// keysOf returns the keys of a string-set as a sorted slice for
// readable error messages.
func keysOf(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Cheap in-place sort; keeps the test dependency surface small.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// copyDir recursively copies src into dst. Both paths must exist;
// dst is overwritten file-by-file. Used by TestStderrWarningMirrored
// to avoid poisoning the committed fixture.
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("copyDir: readdir %q: %v", src, err)
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				t.Fatalf("copyDir: mkdir %q: %v", dstPath, err)
			}
			copyDir(t, srcPath, dstPath)
			continue
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("copyDir: read %q: %v", srcPath, err)
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			t.Fatalf("copyDir: write %q: %v", dstPath, err)
		}
	}
}

// truncateFile chops a file to half its byte length. Inside any
// line-based text format (like pt-stalk outputs), this leaves the
// last surviving row mid-line, forcing a Warning diagnostic rather
// than a clean end-of-file.
func truncateFile(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("truncate: read %q: %v", path, err)
	}
	if len(data) < 200 {
		t.Fatalf("truncate: file %q too small (%d bytes) to truncate meaningfully", path, len(data))
	}
	if err := os.WriteFile(path, data[:len(data)/2], 0o644); err != nil {
		t.Fatalf("truncate: write %q: %v", path, err)
	}
}
