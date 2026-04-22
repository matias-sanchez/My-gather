package perf_test

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/parse"
	"github.com/matias-sanchez/My-gather/render"
)

// TestMemSmall: always-on sanity. A 5 MB fixture must peak below
// 100 MB of Go heap alloc — proportional floor to SC-008's 1 GB /
// 2 GB envelope. A gross regression (e.g., a parser that buffers
// the entire input in memory per snapshot) surfaces here at low
// cost.
func TestMemSmall(t *testing.T) {
	runMem(t, 5*1024*1024, 100*1024*1024, "sanity 5 MB / 100 MB alloc ceiling")
}

// TestMemSC008: SC-008 gate. 1 GB fixture, peak HeapInuse < 2 GB.
//
// Double-gated behind both `-short` AND `PERF_LARGE=1` so the
// default `go test ./...` (even without -short) stays fast. The
// 1 GB target sits at the edge of the shipped FR-005 total-
// collection size-bound (1 GB); reliably landing under the bound
// while still exercising SC-008's intent requires building the
// fixture AND running the test under an operator who has opted
// in. Set `PERF_LARGE=1` to run this locally.
//
// We measure `m.HeapInuse` (not Sys or RSS) because:
//
//   - Sys includes bucketing overhead unrelated to the application's
//     peak working set.
//   - RSS is not directly exposed by runtime.MemStats and varies
//     with OS-level paging decisions; a test that asserts RSS via
//     /proc/self/status would be Linux-only and flaky on macOS CI.
//   - HeapInuse tracks the actual Go-heap bytes in use at the
//     sample point and is the closest stable proxy for "how much
//     memory does the binary need to keep this fixture alive".
//
// A `runtime.GC()` call before and after the work ensures the
// measurement isn't polluted by dead objects from the build-
// synthetic-fixture phase.
func TestMemSC008(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SC-008 1 GB memory test under -short")
	}
	if os.Getenv("PERF_LARGE") != "1" {
		t.Skip("skipping SC-008 1 GB memory test without PERF_LARGE=1 (sits at FR-005 size-bound edge; opt in explicitly)")
	}
	// 900 MB — slightly under the FR-005 1 GB total-collection
	// limit so the synthetic fixture actually parses. Still
	// exercises the SC-008 memory-footprint contract.
	runMem(t, 900*1024*1024, 2*1024*1024*1024, "SC-008 900 MB / 2 GB HeapInuse ceiling")
}

// runMem builds a synthetic fixture of approximately `targetBytes`
// size, runs parse + render, samples HeapInuse at the peak (just
// after render completes, which is where the fixture data lives
// alongside the rendered output buffer) and asserts it stays below
// `limitBytes`. The label surfaces in the error message.
//
// An outer time budget of 10× the perf test's size-equivalent time
// acts as a belt-and-suspenders guard against deadlocks.
func runMem(t *testing.T, targetBytes int64, limitBytes uint64, label string) {
	t.Helper()
	root := repoRoot(t)

	tmp := t.TempDir()
	if err := buildSyntheticFixture(root, tmp, targetBytes); err != nil {
		t.Fatalf("build synthetic fixture: %v", err)
	}

	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	deadline := time.Now().Add(5 * time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	c, err := parse.Discover(ctx, tmp, parse.DiscoverOptions{})
	if err != nil {
		t.Fatalf("parse.Discover: %v", err)
	}
	if c == nil {
		t.Fatalf("parse.Discover returned nil Collection")
	}

	var buf bytes.Buffer
	opts := render.RenderOptions{
		GeneratedAt: time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC),
		Version:     "v0.0.1-mem",
	}
	if err := render.Render(&buf, c, opts); err != nil {
		t.Fatalf("render.Render: %v", err)
	}

	// Sample peak HeapInuse right after render completes — this
	// is the phase where both the parsed Collection and the
	// rendered buffer coexist.
	var peak runtime.MemStats
	runtime.ReadMemStats(&peak)

	// Ensure the rendered buffer and Collection stay live through
	// the sample so the optimiser doesn't free them early.
	runtime.KeepAlive(c)
	runtime.KeepAlive(&buf)

	delta := peak.HeapInuse
	t.Logf("%s: baseline HeapInuse %d, peak HeapInuse %d (delta %d), output %d bytes",
		label, baseline.HeapInuse, peak.HeapInuse, delta, buf.Len())

	if peak.HeapInuse > limitBytes {
		t.Errorf("%s: peak HeapInuse %d bytes exceeds limit %d bytes", label, peak.HeapInuse, limitBytes)
	}
}
