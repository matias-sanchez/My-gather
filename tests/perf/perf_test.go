// Package perf_test exercises the spec's performance envelopes:
//
//   - SC-007: cold-start run against a 50 MB fixture < 10 s.
//   - SC-001: end-to-end run against a 100 MB fixture < 60 s.
//
// Both numbers are wall-clock targets on a stock CI runner. The
// tests build a synthetic pt-stalk-shaped fixture at the requested
// size by duplicating the committed example2 mysqladmin file (the
// biggest collector in real pt-stalk output, ~4 MB per snapshot).
// Duplication preserves the pipe-table structure so parse.Discover
// still runs the real mysqladmin parsing path.
//
// The tests that exceed a few-seconds budget live behind
// `testing.Short()` so `go test -short ./...` stays fast; the full
// suite runs them at their spec'd sizes. A sanity 5 MB variant
// always runs to catch a gross regression (e.g., O(n²) parser) at
// low cost.
package perf_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/parse"
	"github.com/matias-sanchez/My-gather/render"
)

// TestPerfSmall: always-on sanity. A 5 MB mysqladmin fixture must
// complete a full parse + render in < 2 s. Proportional to the
// 50 MB / 10 s SC-007 envelope — a 10× slower run here is a
// direct indicator that SC-007 is likely to fail at full scale.
func TestPerfSmall(t *testing.T) {
	runPerf(t, 5*1024*1024, 2*time.Second, "sanity 5 MB")
}

// TestPerfSC007: SC-007 gate. 50 MB fixture, 10 s wall-clock.
// Skipped under -short.
func TestPerfSC007(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SC-007 50 MB perf test under -short")
	}
	runPerf(t, 50*1024*1024, 10*time.Second, "SC-007 50 MB")
}

// TestPerfSC001: SC-001 gate. 100 MB fixture, 60 s wall-clock.
// Skipped under -short.
func TestPerfSC001(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SC-001 100 MB perf test under -short")
	}
	runPerf(t, 100*1024*1024, 60*time.Second, "SC-001 100 MB")
}

// runPerf builds a synthetic fixture of approximately `targetBytes`
// size, runs the full parse + render pipeline, and asserts wall
// clock under `limit`. The label surfaces in the error message.
func runPerf(t *testing.T, targetBytes int64, limit time.Duration, label string) {
	t.Helper()
	root := repoRoot(t)

	tmp := t.TempDir()
	if err := buildSyntheticFixture(root, tmp, targetBytes); err != nil {
		t.Fatalf("build synthetic fixture: %v", err)
	}

	// Cold start: include both parse + render in the measured
	// span; SC-007 / SC-001 both target end-to-end wall clock.
	start := time.Now()
	c, err := parse.Discover(context.Background(), tmp, parse.DiscoverOptions{})
	if err != nil {
		t.Fatalf("parse.Discover: %v", err)
	}
	if c == nil {
		t.Fatalf("parse.Discover returned nil Collection")
	}
	var buf bytes.Buffer
	opts := render.RenderOptions{
		GeneratedAt: time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC),
		Version:     "v0.0.1-perf",
	}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("render.Render: %v", err)
	}
	elapsed := time.Since(start)

	t.Logf("%s: parse + render in %s, output %d bytes", label, elapsed, buf.Len())
	if elapsed > limit {
		t.Errorf("%s: elapsed %s exceeds limit %s", label, elapsed, limit)
	}
}

// buildSyntheticFixture fills dstDir with a minimal pt-stalk layout
// whose combined mysqladmin-file size across snapshots approximates
// `targetBytes` by duplicating the committed example2 mysqladmin
// fixture. Other suffixes are copied once per snapshot at their
// original size.
//
// Per-file size is capped at 150 MB so the resulting fixture stays
// under the shipped per-file 200 MB guard (FR-005 / parse.Discover);
// larger targets spread across additional synthetic snapshots. A
// 1 GB target becomes ~7 snapshots of 150 MB each — the real
// memory pressure at SC-008 scale lives in concat-across-snapshots
// anyway, which is actually what this test wants to exercise.
func buildSyntheticFixture(repoRoot, dstDir string, targetBytes int64) error {
	srcDir := filepath.Join(repoRoot, "testdata", "example2")
	const basePrefix = "2026_04_21_16_51_41"
	const maxPerFile = 150 * 1024 * 1024 // stay under the 200 MB guard

	// Read the real mysqladmin fixture once; we'll duplicate its
	// pipe-table body to reach the per-snapshot target.
	mysqladminData, err := os.ReadFile(filepath.Join(srcDir, basePrefix+"-mysqladmin"))
	if err != nil {
		return err
	}

	// Decide the synthetic shape: N snapshots each carrying a
	// mysqladmin file of (targetBytes/N) bytes, capped at
	// maxPerFile. Use as few snapshots as possible so the render-
	// layer concat code does less work than the pathological case.
	perSnap := targetBytes
	snapshots := 1
	if perSnap > maxPerFile {
		snapshots = int((targetBytes + maxPerFile - 1) / maxPerFile)
		perSnap = targetBytes / int64(snapshots)
	}

	// Every supported suffix must be present per snapshot so
	// parse.Discover doesn't skip it. iostat is snapshot-1-only
	// in the real fixture; replicate that shape.
	suffixes := []string{"top", "vmstat", "variables", "innodbstatus1", "mysqladmin", "processlist"}

	// Pre-build the large mysqladmin body once and reuse across
	// snapshots — saves repeated concatenation work on large
	// targets.
	var largeMysqladmin []byte
	if int64(len(mysqladminData)) < perSnap {
		times := (perSnap + int64(len(mysqladminData)) - 1) / int64(len(mysqladminData))
		var buf bytes.Buffer
		buf.Grow(int(int64(len(mysqladminData)) * times))
		for i := int64(0); i < times; i++ {
			buf.Write(mysqladminData)
		}
		largeMysqladmin = buf.Bytes()
	} else {
		largeMysqladmin = mysqladminData
	}

	for snap := 0; snap < snapshots; snap++ {
		prefix := basePrefix
		if snap > 0 {
			// Synthesise unique timestamp prefixes. The pt-stalk
			// convention is YYYY_MM_DD_HH_MM_SS; bump the seconds
			// by 30 per synthetic snapshot so Discover sorts them
			// correctly.
			prefix = fmt.Sprintf("2026_04_21_16_51_%02d", 41+30*snap)
		}
		// Per-snapshot hostname (needed for Collection hostname).
		{
			src := filepath.Join(srcDir, basePrefix+"-hostname")
			dst := filepath.Join(dstDir, prefix+"-hostname")
			data, err := os.ReadFile(src)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dst, data, 0o644); err != nil {
				return err
			}
		}
		if snap == 0 {
			// iostat only in snapshot 1 to mirror real pt-stalk.
			src := filepath.Join(srcDir, basePrefix+"-iostat")
			dst := filepath.Join(dstDir, prefix+"-iostat")
			data, err := os.ReadFile(src)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dst, data, 0o644); err != nil {
				return err
			}
		}
		for _, s := range suffixes {
			src := filepath.Join(srcDir, basePrefix+"-"+s)
			dst := filepath.Join(dstDir, prefix+"-"+s)
			var data []byte
			if s == "mysqladmin" {
				data = largeMysqladmin
			} else {
				var err error
				data, err = os.ReadFile(src)
				if err != nil {
					return err
				}
			}
			if err := os.WriteFile(dst, data, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// repoRoot returns the directory containing go.mod by walking upward
// from the test's working directory.
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
