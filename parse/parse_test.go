package parse_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/parse"
	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// TestNotAPtStalkDir — FR-019 + research R5: a directory with no
// timestamped pt-stalk files and no summary file returns
// ErrNotAPtStalkDir.
func TestNotAPtStalkDir(t *testing.T) {
	dir := t.TempDir()
	// Drop some noise so the dir isn't empty.
	if err := os.WriteFile(filepath.Join(dir, "random.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := parse.Discover(context.Background(), dir, parse.DiscoverOptions{})
	if !errors.Is(err, parse.ErrNotAPtStalkDir) {
		t.Fatalf("expected ErrNotAPtStalkDir, got %v", err)
	}
}

// TestEmptyButValid — FR-020 (F18 remediation): recognised directory
// with no supported collectors renders a Collection anyway (the
// renderer will show "data not available" in every subview).
func TestEmptyButValid(t *testing.T) {
	dir := t.TempDir()
	// A timestamped file with an unsupported suffix -hostname, plus
	// a bare pt-summary.out. No supported collectors.
	must := func(path string, content string) {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must("2026_04_21_16_52_11-hostname", "example-db-01\n")
	must("pt-summary.out", "# pt-summary\n")

	c, err := parse.Discover(context.Background(), dir, parse.DiscoverOptions{})
	if err != nil {
		t.Fatalf("expected nil err for empty-but-valid, got %v", err)
	}
	if len(c.Snapshots) == 0 {
		t.Fatalf("expected at least one synthetic Snapshot")
	}
	for _, snap := range c.Snapshots {
		if len(snap.SourceFiles) != 0 {
			t.Errorf("snapshot %q unexpectedly has SourceFiles: %v",
				snap.Prefix, snap.SourceFiles)
		}
	}
	if c.Hostname != "example-db-01" {
		t.Errorf("Hostname %q, want example-db-01", c.Hostname)
	}
}

// TestEnvSidecarFallbackEmitsDiagnostic — Principle III: when the
// newest env sidecar is empty/unreadable and the loader silently
// falls back to an older file, a structured diagnostic MUST be
// attached so the degraded fallback is visible in the report.
func TestEnvSidecarFallbackEmitsDiagnostic(t *testing.T) {
	dir := t.TempDir()
	must := func(path string, content string) {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Two -hostname sidecars: newer one is empty (common truncation
	// mode), older one has content. Also include a pt-summary so the
	// directory is recognised as a pt-stalk dump.
	must("2026_04_21_16_51_41-hostname", "example-db-01\n")
	must("2026_04_21_16_52_11-hostname", "")
	must("pt-summary.out", "# pt-summary\n")

	c, err := parse.Discover(context.Background(), dir, parse.DiscoverOptions{})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	// The loader must have fallen back to the older, non-empty file.
	if got := c.RawEnvSidecars["hostname"]; got != "example-db-01\n" {
		t.Errorf("RawEnvSidecars[hostname] = %q, want fallback content", got)
	}
	// And it MUST have left a diagnostic on the collection so the
	// render layer (or CLI verbose mode) can surface the degradation.
	found := false
	for _, d := range c.Diagnostics {
		if d.Severity == model.SeverityWarning &&
			filepath.Base(d.SourceFile) == "2026_04_21_16_51_41-hostname" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a warning diagnostic naming the older fallback file, got: %+v", c.Diagnostics)
	}
}

// TestEnvMeminfoSidecarFallbackEmitsDiagnostic verifies that
// Discover surfaces env-meminfo compatibility fallbacks in the actual
// report pipeline instead of leaving them visible only to direct
// parser callers.
func TestEnvMeminfoSidecarFallbackEmitsDiagnostic(t *testing.T) {
	dir := t.TempDir()
	must := func(path string, content string) {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must("pt-summary.out", "# pt-summary\n")
	must("2026_04_21_16_51_41-meminfo", `TS 1769702259.004572779 2026-01-29 15:57:39
MemTotal:       32654396 kB
MemFree:        11634540 kB
MemAvailable:   28222432 kB
Buffers:           13864 kB
Cached:         16704284 kB
SwapTotal:      33554428 kB
SwapFree:       33554428 kB
TS 1769702289.004572779 2026-01-29 15:58:09
MemTotal:       32654396 kB
MemFree:        11000000 kB
`)

	c, err := parse.Discover(context.Background(), dir, parse.DiscoverOptions{})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	found := false
	for _, d := range c.Diagnostics {
		if filepath.Base(d.SourceFile) == "2026_04_21_16_51_41-meminfo" &&
			d.Severity == model.SeverityWarning &&
			strings.Contains(d.Message, "newest sample is incomplete") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected env meminfo fallback diagnostic on Collection, got %+v", c.Diagnostics)
	}
	if c.EnvMeminfo == nil {
		t.Fatal("expected Discover to populate typed EnvMeminfo")
	}
	if c.EnvMeminfo.MemAvailableKB != 28222432 {
		t.Fatalf("EnvMeminfo.MemAvailableKB = %d, want 28222432", c.EnvMeminfo.MemAvailableKB)
	}
}

// TestSizeBoundTotalExceeded — FR-025: collection total exceeds limit.
func TestSizeBoundTotalExceeded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "2026_04_21_16_52_11-iostat")
	// 200 KB file, limit 100 KB.
	data := make([]byte, 200*1024)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := parse.Discover(context.Background(), dir, parse.DiscoverOptions{
		MaxCollectionBytes: 100 * 1024,
		MaxFileBytes:       1024 * 1024, // allow big single files
	})
	var sz *parse.SizeError
	if !errors.As(err, &sz) {
		t.Fatalf("expected *SizeError, got %v", err)
	}
	if sz.Kind != parse.SizeErrorTotal {
		t.Errorf("Kind = %v, want SizeErrorTotal", sz.Kind)
	}
}

// TestSizeBoundFileExceeded — FR-025: one file exceeds the per-file
// bound.
func TestSizeBoundFileExceeded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "2026_04_21_16_52_11-iostat")
	data := make([]byte, 150*1024)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := parse.Discover(context.Background(), dir, parse.DiscoverOptions{
		MaxCollectionBytes: 1024 * 1024,
		MaxFileBytes:       100 * 1024,
	})
	var sz *parse.SizeError
	if !errors.As(err, &sz) {
		t.Fatalf("expected *SizeError, got %v", err)
	}
	if sz.Kind != parse.SizeErrorFile {
		t.Errorf("Kind = %v, want SizeErrorFile", sz.Kind)
	}
}

// TestPathError — FR-019: missing path surfaces as a *PathError that
// wraps the stdlib os error.
func TestPathError(t *testing.T) {
	_, err := parse.Discover(context.Background(), "/definitely/not/a/real/path", parse.DiscoverOptions{})
	var pe *parse.PathError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *PathError, got %v", err)
	}
	if pe.Op != "stat" {
		t.Errorf("Op %q, want stat", pe.Op)
	}
}

// TestHappyPathAgainstFixtures — integration: run Discover against the
// committed anonymised fixture in testdata/example2/, assert we pick up
// two snapshots and non-zero SourceFiles entries.
func TestHappyPathAgainstFixtures(t *testing.T) {
	root := goldens.RepoRoot(t)
	c, err := parse.Discover(context.Background(),
		filepath.Join(root, "testdata", "example2"), parse.DiscoverOptions{})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(c.Snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(c.Snapshots))
	}
	for _, snap := range c.Snapshots {
		if len(snap.SourceFiles) == 0 {
			t.Errorf("snapshot %q has zero SourceFiles", snap.Prefix)
		}
	}
	if c.Hostname == "" {
		t.Errorf("Hostname empty; expected anonymised value")
	}
}

// TestContextCanceledDuringParsePropagates — regression guard for
// the P2 codex flagged on 31f91da: runParsers returned void, so a
// deadline/cancel that fired AFTER directory enumeration but DURING
// per-collector parsing let Discover return (collection, nil) — a
// silent partial success. Discover now forwards the context error.
func TestContextCanceledDuringParsePropagates(t *testing.T) {
	root := goldens.RepoRoot(t)
	fixtureDir := filepath.Join(root, "testdata", "example2")

	// Cancel immediately so the ctx.Err() check inside runParsers'
	// inner loop fires on the first SourceFile. Directory
	// enumeration ran synchronously before runParsers, so it
	// completes before the check trips.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := parse.Discover(ctx, fixtureDir, parse.DiscoverOptions{})
	if err == nil {
		t.Fatalf("expected cancellation error, got nil (Discover silently returned a partial Collection)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
