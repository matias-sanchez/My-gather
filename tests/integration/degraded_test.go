package integration_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestOneMissingCollector: T094 / SC-004 / FR-008.
//
// SC-004 requires the report to degrade gracefully when one
// collector's output is missing from a pt-stalk run: the other six
// sections still render with data, and the missing collector
// surfaces a visible "data not available" banner naming the
// absent file. This matches the field reality — a pt-stalk run
// can fail to collect one or two files (privilege issues, a
// transient mysqladmin connection failure, etc.) without the
// whole report becoming useless.
//
// Parametrised across the seven supported suffixes. For each
// suffix the test:
//
//   1. Copies `testdata/example2/` to an isolated tempdir.
//   2. Deletes every file in that copy whose name ends in
//      `-<suffix>` from BOTH snapshots (so the collector is
//      absent from the whole collection, not just one snapshot
//      — this is what exercises the "no <suffix> files were
//      present in this collection" banner the templates emit).
//   3. Runs the compiled binary against the tempdir copy.
//   4. Asserts exit 0 (the missing file is a degraded, not
//      fatal, condition — FR-008).
//   5. Asserts the rendered HTML carries a missing-file banner
//      that names the absent suffix.
//   6. Asserts the three top-level section anchors still exist
//      (`sec-os`, `sec-variables`, `sec-db`) so navigation is
//      intact — the banner appears inside a surviving section,
//      it doesn't REPLACE the section.
//
// Seven subtests, one task — a parametrised table keeps the
// intent visible and the failures per-suffix actionable.
func TestOneMissingCollector(t *testing.T) {
	suffixes := []string{
		"iostat",
		"top",
		"vmstat",
		"variables",
		"innodbstatus1",
		"mysqladmin",
		"processlist",
	}

	for _, suffix := range suffixes {
		suffix := suffix
		t.Run(suffix, func(t *testing.T) {
			root := repoRoot(t)
			src := filepath.Join(root, "testdata", "example2")

			dst := t.TempDir()
			fixtureCopy := filepath.Join(dst, "example2")
			if err := copyDir(src, fixtureCopy); err != nil {
				t.Fatalf("copyDir: %v", err)
			}

			// Delete every file ending in `-<suffix>` in the copy.
			// Both snapshots carry the same suffix set (except
			// iostat, which only exists in snapshot 1 by fixture
			// design). Walking the tree and matching by suffix
			// handles both cases uniformly.
			removed := 0
			entries, err := os.ReadDir(fixtureCopy)
			if err != nil {
				t.Fatalf("read fixture copy: %v", err)
			}
			want := "-" + suffix
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				if strings.HasSuffix(e.Name(), want) {
					if err := os.Remove(filepath.Join(fixtureCopy, e.Name())); err != nil {
						t.Fatalf("remove %s: %v", e.Name(), err)
					}
					removed++
				}
			}
			if removed == 0 {
				t.Fatalf("deleted zero files with suffix %q from %s; fixture layout changed?", want, fixtureCopy)
			}

			out := filepath.Join(dst, "report.html")
			cmd := exec.Command(binPath, "--out", out, fixtureCopy)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("my-gather exit non-zero with %q missing: %v\nstderr:\n%s",
					suffix, err, stderr.String())
			}

			html, err := os.ReadFile(out)
			if err != nil {
				t.Fatalf("read output: %v", err)
			}
			htmlStr := string(html)

			// The three top-level section anchors must still exist:
			// a missing collector degrades ONE subview, not a whole
			// top-level section. `sec-os` survives iostat/top/vmstat
			// going missing because it's a composite section.
			for _, id := range []string{`id="sec-os"`, `id="sec-variables"`, `id="sec-db"`} {
				if !strings.Contains(htmlStr, id) {
					t.Errorf("with %q missing, rendered HTML is missing top-level section %s; SC-004 requires a missing collector to degrade only its own subview",
						suffix, id)
				}
			}

			// The missing-file banner must name the absent suffix.
			// The two shipped banner forms are:
			//
			//   "Data not available — <code>-{suffix}</code> was
			//    missing or unparseable in this collection."
			//
			//   "Data not available — no <code>-{suffix}</code>
			//    files were present in any snapshot of this
			//    collection." (variables form, per-collection)
			//
			// Either form counts — the invariant is that the banner
			// appears AND carries the suffix name.
			want1 := fmt.Sprintf(`<code>-%s</code>`, suffix)
			if !strings.Contains(htmlStr, want1) {
				t.Errorf("with %q missing, rendered HTML does not contain a banner referencing %q; SC-004 requires a visible banner naming the absent file",
					suffix, want1)
			}
			if !strings.Contains(htmlStr, "Data not available") {
				t.Errorf("with %q missing, rendered HTML does not contain the 'Data not available' banner text; SC-004 requires the banner phrasing",
					suffix)
			}
		})
	}
}

// copyDir recursively copies srcDir into dstDir. Used by the
// degraded-fixture tests to produce an isolated tempdir copy
// before deleting one collector's files. Preserves file modes.
// A single-level directory copy is sufficient for the flat
// pt-stalk fixture layout; nested directories aren't expected.
func copyDir(srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dstDir, err)
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read %s: %w", srcDir, err)
	}
	for _, e := range entries {
		srcPath := filepath.Join(srcDir, e.Name())
		dstPath := filepath.Join(dstDir, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

// copyFile copies a single file with its mode preserved.
func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", srcPath, err)
	}
	defer src.Close()

	fi, err := src.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", srcPath, err)
	}

	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode())
	if err != nil {
		return fmt.Errorf("create %s: %w", dstPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", srcPath, dstPath, err)
	}
	return nil
}
