package parse

import (
	"os"
	"path/filepath"
	"testing"

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
