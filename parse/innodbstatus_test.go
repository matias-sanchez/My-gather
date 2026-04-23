package parse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// TestInnoDBStatusGolden: T063 / FR-014 / Principle VIII.
//
// Parse `testdata/example2/2026_04_21_16_51_41-innodbstatus1` and
// snapshot-compare the resulting *InnodbStatusData plus diagnostics
// against a committed golden. Unlike the time-series collectors, this
// is a point-in-time snapshot — `-innodbstatus1` captures one SHOW
// ENGINE INNODB STATUS per pt-stalk pass — so the golden is a small
// record of scalars rather than a sample array.
//
// Inline invariants stay conservative: SemaphoreCount / PendingReads /
// PendingWrites / HistoryListLength are non-negative counts; the AHI
// rates are non-negative floats. The golden-compare covers every
// other shape change.
func TestInnoDBStatusGolden(t *testing.T) {
	root := goldens.RepoRoot(t)
	fixture := filepath.Join(root, "testdata", "example2", "2026_04_21_16_51_41-innodbstatus1")
	goldenPath := filepath.Join(root, "testdata", "golden", "innodbstatus.example2.2026_04_21_16_51_41.json")

	f, err := os.Open(fixture)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	data, diags := parseInnodbStatus(f, fixture)
	if data == nil {
		t.Fatalf("parseInnodbStatus returned nil data (diagnostics: %+v)", diags)
	}

	// Non-negativity: each of these fields is a count / rate, never a
	// signed value. A sign flip would almost certainly indicate a
	// regex or integer-parse regression; surface it with a direct
	// message rather than a JSON diff.
	if data.SemaphoreCount < 0 {
		t.Errorf("SemaphoreCount = %d; must be non-negative", data.SemaphoreCount)
	}
	if data.PendingReads < 0 {
		t.Errorf("PendingReads = %d; must be non-negative", data.PendingReads)
	}
	if data.PendingWrites < 0 {
		t.Errorf("PendingWrites = %d; must be non-negative", data.PendingWrites)
	}
	if data.HistoryListLength < 0 {
		t.Errorf("HistoryListLength = %d; must be non-negative", data.HistoryListLength)
	}
	if data.AHIActivity.HashTableSize < 0 {
		t.Errorf("AHIActivity.HashTableSize = %d; must be non-negative", data.AHIActivity.HashTableSize)
	}
	if data.AHIActivity.SearchesPerSec < 0 {
		t.Errorf("AHIActivity.SearchesPerSec = %f; must be non-negative", data.AHIActivity.SearchesPerSec)
	}
	if data.AHIActivity.NonHashSearchesPerSec < 0 {
		t.Errorf("AHIActivity.NonHashSearchesPerSec = %f; must be non-negative", data.AHIActivity.NonHashSearchesPerSec)
	}

	got := goldens.MarshalDeterministic(t, struct {
		Data        any `json:"data"`
		Diagnostics any `json:"diagnostics"`
	}{
		Data:        data,
		Diagnostics: diags,
	})
	goldens.Compare(t, goldenPath, got)
}

// TestInnoDBSemaphoreCanonicalPair exercises the single canonical
// SEMAPHORES shape — a `--Thread ...` line immediately followed by
// its paired `Mutex at ... Mutex <NAME> created ...` partner — and
// asserts that both the thread count and the resulting SemaphoreSites
// entry are populated, that sum(SemaphoreSites.WaitCount) equals
// SemaphoreCount, and that no invariant-violation Warning is emitted.
func TestInnoDBSemaphoreCanonicalPair(t *testing.T) {
	body := "SEMAPHORES\n" +
		"--Thread 140148200982272 has waited at ibuf0ibuf.cc line 3922 for 0 seconds the semaphore:\n" +
		"Mutex at 0x7f0000000000, Mutex IBUF created ibuf0ibuf.cc:612, locked by...\n"

	data, diags := parseInnodbStatus(strings.NewReader(body), "unit-test")
	if data == nil {
		t.Fatalf("parseInnodbStatus returned nil data (diags: %+v)", diags)
	}
	if data.SemaphoreCount != 1 {
		t.Errorf("SemaphoreCount = %d; want 1", data.SemaphoreCount)
	}
	if got := len(data.SemaphoreSites); got != 1 {
		t.Fatalf("len(SemaphoreSites) = %d; want 1", got)
	}
	if got := data.SemaphoreSites[0].MutexName; got != "IBUF" {
		t.Errorf("SemaphoreSites[0].MutexName = %q; want %q", got, "IBUF")
	}
	sum := 0
	for _, s := range data.SemaphoreSites {
		sum += s.WaitCount
	}
	if sum != data.SemaphoreCount {
		t.Errorf("SemaphoreSites wait-count sum = %d; want %d (invariant)", sum, data.SemaphoreCount)
	}
	for _, d := range diags {
		if d.Severity == model.SeverityWarning &&
			strings.Contains(d.Message, "SemaphoreSites wait-count sum") {
			t.Errorf("unexpected invariant-violation Warning on canonical input: %q", d.Message)
		}
	}
}
