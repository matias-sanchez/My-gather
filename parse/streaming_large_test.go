package parse_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/parse"
)

// TestDiscoverStreamingLargeCollection — feature
// 016-remove-collection-size-cap regression test.
//
// Pins two invariants together:
//
//  1. parse.Discover returns no *parse.SizeError for a synthetic
//     pt-stalk capture whose total size exceeds 1.1 GiB. The
//     historical 1 GiB total-collection refusal path is gone.
//  2. Per-collector parsers stream their input rather than buffer
//     entire files. We assert this by measuring peak in-process heap
//     delta during the Discover call: if any stage slurped a whole
//     ~190 MiB collector file (or worse, the whole >1.1 GiB
//     collection) into memory, the delta would blow past the
//     ceiling.
//
// Synthetic content design (important): each collector file is
// padded with sparse filler lines that the iostat parser silently
// skips (anything not matching "Device" / "Linux " / a data row
// inside a sample block is dropped without emitting a diagnostic
// and without retaining state). That keeps the parsed-model size
// proportional to the small handful of real samples, not to the
// raw byte size — so the heap-delta ceiling actually catches
// buffering regressions.
//
// The test writes its synthetic capture chunk-by-chunk so the
// generator itself never holds a whole file (let alone the whole
// collection) in memory.
//
// Skipped under `go test -short` because allocating a >1.1 GiB
// tempdir takes a few seconds and ~1.14 GiB of disk.
func TestDiscoverStreamingLargeCollection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-GB streaming regression in -short mode")
	}

	dir := t.TempDir()

	// Six iostat snapshots at distinct prefixes. Iostat is used for
	// every file so we exercise a single parser with high confidence
	// in the silent-skip behavior of its filler lines. Different
	// prefixes give Discover six distinct snapshots to group.
	prefixes := []string{
		"2026_05_07_12_00_00",
		"2026_05_07_12_01_00",
		"2026_05_07_12_02_00",
		"2026_05_07_12_03_00",
		"2026_05_07_12_04_00",
		"2026_05_07_12_05_00",
	}
	const perFile = int64(190 << 20) // 190 MiB, just under DefaultMaxFileBytes (200 MiB).

	for _, p := range prefixes {
		path := filepath.Join(dir, fmt.Sprintf("%s-iostat", p))
		if err := writeIostatPadded(path, perFile); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	// Sanity-check: total directory size really is > 1.1 GiB.
	var total int64
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		fi, err := e.Info()
		if err != nil {
			t.Fatalf("stat %s: %v", e.Name(), err)
		}
		total += fi.Size()
	}
	if total < int64(1100)<<20 {
		t.Fatalf("synthetic capture is only %d bytes; want > 1.1 GiB", total)
	}

	// Sample heap before Discover. Force a GC so the baseline is
	// stable.
	runtime.GC()
	var msBefore runtime.MemStats
	runtime.ReadMemStats(&msBefore)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	c, err := parse.Discover(ctx, dir, parse.DiscoverOptions{})
	if err != nil {
		var sz *parse.SizeError
		if errors.As(err, &sz) {
			t.Fatalf("Discover returned *SizeError despite cap removal: %v", err)
		}
		t.Fatalf("Discover failed on %d-byte capture: %v", total, err)
	}
	if c == nil {
		t.Fatal("Discover returned nil collection without error")
	}
	if len(c.Snapshots) == 0 {
		t.Fatal("Discover returned a collection with zero snapshots")
	}

	// Sample heap after Discover. Force a GC so transient allocations
	// from per-collector parsers are reclaimed before we measure.
	runtime.GC()
	var msAfter runtime.MemStats
	runtime.ReadMemStats(&msAfter)

	// HeapAlloc is uint64; convert carefully to a signed delta.
	var delta int64
	if msAfter.HeapAlloc >= msBefore.HeapAlloc {
		delta = int64(msAfter.HeapAlloc - msBefore.HeapAlloc)
	} else {
		// GC reclaimed more than we allocated — this is fine,
		// streaming worked great.
		delta = 0
	}

	const heapDeltaCeiling int64 = 256 << 20 // 256 MiB
	if delta > heapDeltaCeiling {
		t.Fatalf("heap delta during Discover = %d bytes (%d MiB); ceiling %d MiB. A parser stage is buffering, not streaming.",
			delta, delta>>20, heapDeltaCeiling>>20)
	}

	// Reach into the parsed model to make sure the parser actually
	// did work — otherwise the heap-bound assertion is trivially
	// satisfied by an early-exit bug. We only need one snapshot to
	// have at least one parsed source file.
	var parsedAny bool
	for _, snap := range c.Snapshots {
		for _, sf := range snap.SourceFiles {
			if sf != nil && sf.Parsed != nil {
				parsedAny = true
				break
			}
		}
		if parsedAny {
			break
		}
	}
	if !parsedAny {
		t.Fatal("Discover returned a collection but no SourceFile was parsed")
	}

	t.Logf("Discover parsed %d-byte capture with heap delta %d MiB (ceiling %d MiB)",
		total, delta>>20, heapDeltaCeiling>>20)
}

// writeIostatPadded writes a small valid iostat sample at the head
// of path, then pads the file to size bytes with filler lines that
// the iostat parser silently skips (lines outside an active sample
// block that do not start with "Device" or "Linux " hit the
// `if !inSample { continue }` arm of parseIostat without emitting a
// diagnostic and without retaining any per-line state).
//
// The writer uses an io.Reader-backed 1 MiB chunk and io.CopyN so
// the generator itself streams; it never holds more than chunkSize
// bytes of filler in memory.
func writeIostatPadded(path string, size int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	header := "Linux 5.15.0-105-generic (synthetic-host) \t05/07/2026 \t_x86_64_\t(8 CPU)\n\n" +
		"avg-cpu:  %user   %nice %system %iowait  %steal   %idle\n" +
		"           1.50    0.00    0.50    0.10    0.00   97.90\n\n" +
		"Device             tps    kB_read/s    kB_wrtn/s    kB_dscd/s    kB_read    kB_wrtn    kB_dscd %util  aqu-sz\n" +
		"sda                3.21        12.34        56.78         0.00     123456     789012          0  10.00   0.50\n\n"
	if _, err := f.WriteString(header); err != nil {
		return err
	}

	written := int64(len(header))

	// Filler line: 99 bytes plus '\n'. It does not start with
	// "Device" or "Linux " and is therefore silently dropped by
	// parseIostat outside an active sample block.
	const fillerLine = "# pad-line ........................................................................................... \n"
	if len(fillerLine) < 100 {
		return fmt.Errorf("filler line length is %d, want >= 100", len(fillerLine))
	}

	// Pre-build a 1 MiB chunk of filler so each Write flushes a
	// meaningful amount of data without per-line allocations.
	const chunkSize = 1 << 20 // 1 MiB
	chunk := bytes.Repeat([]byte(fillerLine), chunkSize/len(fillerLine))
	r := bytes.NewReader(chunk)

	for written < size {
		remaining := size - written
		toWrite := int64(len(chunk))
		if remaining < toWrite {
			toWrite = remaining
		}
		if _, err := r.Seek(0, io.SeekStart); err != nil {
			return err
		}
		n, err := io.CopyN(f, r, toWrite)
		written += n
		if err != nil {
			return err
		}
	}
	return nil
}
