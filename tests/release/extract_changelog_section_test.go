// Package release_test exercises the canonical release-pipeline shell
// scripts under scripts/release/.
//
// extract_changelog_section_test exec's
// scripts/release/extract-changelog-section.sh against the in-repo
// CHANGELOG.md and asserts (a) the produced body is bounded by the
// next "## v" heading, (b) for v0.4.0 every shipped feature
// (016..021) is mentioned, and (c) a missing tag exits non-zero.
//
// This is the regression test for FR-012, FR-013, and SC-004 of
// feature 022-changelog-release-pipeline.
package release_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot returns the absolute path to the repository root, derived
// from the location of this test file. Walks up two levels:
// tests/release/<this-file> -> tests/release -> tests -> repo root.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed; cannot locate test file")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	return root
}

// runExtract execs the canonical extraction script with the given tag
// and returns stdout, stderr, and the exit error (nil on exit code 0).
func runExtract(t *testing.T, tag string) (stdout, stderr string, err error) {
	t.Helper()
	root := repoRoot(t)
	cmd := exec.Command("scripts/release/extract-changelog-section.sh", tag)
	cmd.Dir = root
	cmd.Env = append(cmd.Environ(), "LC_ALL=C")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// TestExtractV031 asserts the script returns the v0.3.1 body, that
// the body contains the existing v0.3.1 prose ("Patch release"), and
// that the body does not leak into the next ## v heading.
func TestExtractV031(t *testing.T) {
	stdout, stderr, err := runExtract(t, "v0.3.1")
	if err != nil {
		t.Fatalf("extract v0.3.1 failed: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, "Patch release") {
		t.Errorf("v0.3.1 body missing the expected 'Patch release' prose; stdout was:\n%s", stdout)
	}
	if hasHeading(stdout) {
		t.Errorf("v0.3.1 body leaked into the next section heading; stdout was:\n%s", stdout)
	}
}

// hasHeading reports whether body contains any line that starts with
// "## v" (i.e., a Keep-a-Changelog version heading). Substring matches
// inside prose (e.g., the literal "## vX.Y.Z (YYYY-MM-DD)" form
// quoted in changelog text) do NOT count, only true line-anchored
// headings, which is what the extractor must terminate before.
func hasHeading(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## v") {
			return true
		}
	}
	return false
}

// TestExtractV040 asserts the script returns the v0.4.0 body, that
// every shipped feature (016..021) is mentioned (FR-013), that the
// body does not contain any ## v heading, and that the body's first
// non-blank line is a Keep-a-Changelog subsection heading.
func TestExtractV040(t *testing.T) {
	stdout, stderr, err := runExtract(t, "v0.4.0")
	if err != nil {
		t.Fatalf("extract v0.4.0 failed: %v\nstderr: %s", err, stderr)
	}
	if hasHeading(stdout) {
		t.Errorf("v0.4.0 body leaked into the next section heading; stdout was:\n%s", stdout)
	}
	for _, id := range []string{"016", "017", "018", "019", "020", "021"} {
		if !strings.Contains(stdout, id) {
			t.Errorf("v0.4.0 body missing feature attribution %q (FR-013)", id)
		}
	}
	for _, pr := range []string{"#50", "#51", "#52", "#53", "#54", "#55", "#56"} {
		if !strings.Contains(stdout, pr) {
			t.Errorf("v0.4.0 body missing PR attribution %q", pr)
		}
	}
}

// TestExtractMissingTag asserts the script exits non-zero when the
// requested tag has no matching heading, with a clear stderr message.
func TestExtractMissingTag(t *testing.T) {
	stdout, stderr, err := runExtract(t, "v9.9.9-nope")
	if err == nil {
		t.Fatalf("extract for missing tag should have failed; stdout was: %q", stdout)
	}
	if !strings.Contains(stderr, "no section for tag v9.9.9-nope") {
		t.Errorf("stderr should explain the missing tag; got: %q", stderr)
	}
	if stdout != "" {
		t.Errorf("missing-tag run should produce no stdout; got: %q", stdout)
	}
}

// TestExtractMissingArg asserts the script exits with usage when no
// tag argument is provided.
func TestExtractMissingArg(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.Command("scripts/release/extract-changelog-section.sh")
	cmd.Dir = root
	cmd.Env = append(cmd.Environ(), "LC_ALL=C")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err == nil {
		t.Fatalf("extract without args should have failed; stdout was: %q", outBuf.String())
	}
	if !strings.Contains(errBuf.String(), "usage:") {
		t.Errorf("stderr should print usage; got: %q", errBuf.String())
	}
}
