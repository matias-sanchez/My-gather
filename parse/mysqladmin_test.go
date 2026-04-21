package parse

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// TestPtMextFixture validates the pt-mext correctness anchor committed
// under testdata/pt-mext/. See research R8 and spec FR-028.
//
// We use structural comparison (F10 remediation): parse both the Go
// parser output and the frozen expected.txt into
//
//	map[varname] -> {total, min, max, avg}
//
// and compare element-wise. Whitespace in the expected file is not
// authoritative — only the numeric aggregates are.
//
// Aggregate semantics match pt-mext's C++ reference exactly:
//   - total = sum of deltas for samples 2..N
//   - min   = smallest delta in samples 2..N (using col==2 to seed)
//   - max   = largest delta in samples 2..N
//   - avg   = integer division of total by total-sample-count N
//     (matches pt-mext line 53: `stats[vname][3] = stats[vname][0] / col`)
func TestPtMextFixture(t *testing.T) {
	root := findRepoRootFromTest(t)
	inputPath := filepath.Join(root, "testdata", "pt-mext", "input.txt")
	expectedPath := filepath.Join(root, "testdata", "pt-mext", "expected.txt")

	f, err := os.Open(inputPath)
	if err != nil {
		t.Fatalf("open input: %v", err)
	}
	defer f.Close()
	d, _ := parseMysqladmin(f, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), inputPath)
	if d == nil {
		t.Fatal("parseMysqladmin returned nil")
	}

	got := aggregatePtMextStyle(d)
	want := parseExpectedFile(t, expectedPath)

	// The Go parser distinguishes counters from gauges (research R8
	// improvement A); pt-mext treats every variable as a counter.
	// The fixture's Acl_* variables are semantically gauges (a count
	// of currently-defined grants), so our classifier skips them.
	// Restrict the cross-check to variables present in both sides.
	matched := 0
	for _, name := range sortedKeys(want) {
		w := want[name]
		g, ok := got[name]
		if !ok {
			t.Logf("%s: not reporting (classified as gauge, deliberate per R8-A)", name)
			continue
		}
		matched++
		if g.total != w.total {
			t.Errorf("%s.total = %d, want %d", name, g.total, w.total)
		}
		if g.min != w.min {
			t.Errorf("%s.min = %d, want %d", name, g.min, w.min)
		}
		if g.max != w.max {
			t.Errorf("%s.max = %d, want %d", name, g.max, w.max)
		}
		if g.avg != w.avg {
			t.Errorf("%s.avg = %d, want %d", name, g.avg, w.avg)
		}
	}
	if matched < 1 {
		t.Fatalf("no pt-mext-counter rows matched between got/want")
	}
}

type ptMextAgg struct {
	total, min, max, avg int64
}

// aggregatePtMextStyle computes per-counter aggregates using pt-mext's
// exact arithmetic: integer values, pt-mext's `stats[vname][0] / col`
// where col is the total sample count (not the delta count).
func aggregatePtMextStyle(d *model.MysqladminData) map[string]ptMextAgg {
	out := map[string]ptMextAgg{}
	for name, isCounter := range d.IsCounter {
		if !isCounter {
			continue
		}
		vs := d.Deltas[name]
		if len(vs) < 2 {
			continue
		}
		var total int64
		var minv, maxv int64
		initialised := false
		for i, v := range vs {
			if i == 0 || math.IsNaN(v) {
				continue
			}
			iv := int64(v)
			total += iv
			if !initialised {
				minv, maxv = iv, iv
				initialised = true
				continue
			}
			if iv < minv {
				minv = iv
			}
			if iv > maxv {
				maxv = iv
			}
		}
		// pt-mext's avg: total / total-sample-count (integer division).
		col := int64(len(vs))
		var avg int64
		if col > 0 {
			avg = total / col
		}
		out[name] = ptMextAgg{total: total, min: minv, max: maxv, avg: avg}
	}
	return out
}

// parseExpectedFile loads the expected aggregates from the committed
// fixture. Each line has the form
//
//	<name><WS><initial><WS><delta1><WS><delta2><WS>| total: N min: N max: N avg: N
func parseExpectedFile(t *testing.T, path string) map[string]ptMextAgg {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	out := map[string]ptMextAgg{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, "|")
		if idx < 0 {
			continue
		}
		nameAndDeltas := strings.Fields(line[:idx])
		if len(nameAndDeltas) == 0 {
			continue
		}
		name := nameAndDeltas[0]
		stats := line[idx+1:]
		var total, minv, maxv, avg int64
		_, err := fmt.Sscanf(stats, " total: %d min: %d max: %d avg: %d", &total, &minv, &maxv, &avg)
		if err != nil {
			t.Logf("skipping unparseable expected line: %q (err=%v)", line, err)
			continue
		}
		out[name] = ptMextAgg{total, minv, maxv, avg}
	}
	if len(out) == 0 {
		t.Fatalf("parsed zero expected entries from %s", path)
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	xs := make([]string, 0, len(m))
	for k := range m {
		xs = append(xs, k)
	}
	sort.Strings(xs)
	return xs
}

func findRepoRootFromTest(t *testing.T) string {
	t.Helper()
	wd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	for dir := wd; dir != "/" && dir != ""; dir = filepath.Dir(dir) {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			return dir
		}
	}
	t.Fatalf("no go.mod from %s", wd)
	return ""
}
