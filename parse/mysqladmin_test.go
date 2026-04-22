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
	"github.com/matias-sanchez/My-gather/tests/goldens"
)

// mysqladminSnapshotStart anchors the FR-028 golden and the
// multi-snapshot tests against a fixed virtual wall-clock. The parser
// synthesises per-column timestamps from this base (1-second spacing),
// so the value flows into the committed golden — leaving it floating
// would make the file non-deterministic across developers.
func mysqladminSnapshotStart() time.Time {
	return time.Date(2026, 4, 21, 16, 51, 41, 0, time.UTC)
}

// mysqladminSnapshotStartSecond matches the second committed fixture's
// filename (16:52:11 UTC) and is used by TestSnapshotBoundaryReset to
// simulate the render-layer's multi-Snapshot merge.
func mysqladminSnapshotStartSecond() time.Time {
	return time.Date(2026, 4, 21, 16, 52, 11, 0, time.UTC)
}

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
	root := goldens.RepoRoot(t)
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

// TestMysqladminGolden: T064 / FR-028 / Principle VIII.
//
// Parse `testdata/example2/2026_04_21_16_51_41-mysqladmin` and
// snapshot-compare the resulting *MysqladminData plus diagnostics
// against a committed golden.
//
// The parser emits `math.NaN()` for post-boundary first slots
// (FR-030) and drift slots (R8.C); encoding/json rejects non-finite
// floats, so the Deltas map is routed through the shared
// `goldens.ScrubFloats` helper before marshalling. Finite slots flow
// through as JSON numbers; NaN / ±Inf become `null`.
//
// Inline invariants cover the shape contract from data-model.md:
//   - VariableNames is sorted alphabetically and deduplicated;
//   - IsCounter has the same key set as VariableNames;
//   - every Deltas[v] slice has length == SampleCount;
//   - Timestamps has length == SampleCount, strictly non-decreasing;
//   - single-file parse has SnapshotBoundaries == [0];
//   - the counter/gauge classifier agrees with a small declared
//     allowlist spot-check so a refactor of classifyAsCounter can't
//     silently reclassify a well-known counter as a gauge (or vice
//     versa).
func TestMysqladminGolden(t *testing.T) {
	root := goldens.RepoRoot(t)
	fixture := filepath.Join(root, "testdata", "example2", "2026_04_21_16_51_41-mysqladmin")
	goldenPath := filepath.Join(root, "testdata", "golden", "mysqladmin.example2.2026_04_21_16_51_41.json")

	f, err := os.Open(fixture)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	data, diags := parseMysqladmin(f, mysqladminSnapshotStart(), fixture)
	if data == nil {
		t.Fatalf("parseMysqladmin returned nil data (diagnostics: %+v)", diags)
	}

	// VariableNames strictly sorted with no duplicates. Rendering
	// iterates this slice directly (per the data-model.md note on
	// MysqladminData), so drift here flips chart legends silently.
	if len(data.VariableNames) == 0 {
		t.Fatalf("expected non-empty VariableNames")
	}
	for i := 1; i < len(data.VariableNames); i++ {
		if data.VariableNames[i-1] >= data.VariableNames[i] {
			t.Errorf("VariableNames not strictly sorted: %q >= %q at index %d",
				data.VariableNames[i-1], data.VariableNames[i], i)
		}
	}
	// IsCounter keys == VariableNames as a set. A mismatch points at
	// classifyAsCounter being called on a name that isn't registered
	// (or vice versa), which would leave a chart either mis-typed or
	// unrenderable.
	if got, want := len(data.IsCounter), len(data.VariableNames); got != want {
		t.Errorf("IsCounter has %d keys; want %d (one per VariableNames entry)", got, want)
	}
	for _, name := range data.VariableNames {
		if _, ok := data.IsCounter[name]; !ok {
			t.Errorf("IsCounter missing entry for VariableNames[%q]", name)
		}
	}
	// Every Deltas slice has length SampleCount, and SampleCount ==
	// len(Timestamps). Together these say "every variable has one
	// value per observed column", which is the core invariant the
	// render layer depends on.
	if got, want := len(data.Timestamps), data.SampleCount; got != want {
		t.Errorf("Timestamps has %d entries; want SampleCount=%d", got, want)
	}
	for _, name := range data.VariableNames {
		if got, want := len(data.Deltas[name]), data.SampleCount; got != want {
			t.Errorf("Deltas[%q] has %d entries; want SampleCount=%d", name, got, want)
		}
	}
	// Timestamps strictly non-decreasing. Not strictly increasing,
	// because future multi-file synthesis could produce equal ticks
	// at a boundary; within a single file they come from a
	// 1-second-spaced ladder.
	for i := 1; i < len(data.Timestamps); i++ {
		if data.Timestamps[i].Before(data.Timestamps[i-1]) {
			t.Errorf("Timestamps[%d] %s is before [%d] %s — must be non-decreasing",
				i, data.Timestamps[i].Format(time.RFC3339Nano),
				i-1, data.Timestamps[i-1].Format(time.RFC3339Nano))
		}
	}
	// Single-file parse: boundaries MUST be exactly [0]. Multi-file
	// concatenation is the render layer's responsibility and is
	// exercised by TestSnapshotBoundaryReset below.
	if len(data.SnapshotBoundaries) != 1 || data.SnapshotBoundaries[0] != 0 {
		t.Errorf("single-file parse SnapshotBoundaries = %v; want [0]", data.SnapshotBoundaries)
	}

	// Counter/gauge classification spot-check. The allowlist in
	// parse/mysqladmin.go::classifyAsCounter is the source of truth;
	// this table pins a handful of well-known names so an accidental
	// reclassification fails with a specific message instead of a
	// wide golden-JSON diff. Only names present in the fixture are
	// checked — others are skipped with a tolerant Logf so a fixture
	// refresh doesn't require this table to stay in lockstep.
	classifierSpotCheck := []struct {
		name     string
		counter  bool
		category string
	}{
		// Regex-matched families (mysqld_exporter global_status.go:37).
		{"Com_select", true, "counter: com family"},
		{"Handler_read_key", true, "counter: handler family"},
		{"Connection_errors_accept", true, "counter: connection_errors family"},
		{"Connection_errors_max_connections", true, "counter: connection_errors family"},
		{"Performance_schema_accounts_lost", true, "counter: performance_schema family"},
		{"Performance_schema_digest_lost", true, "counter: performance_schema family"},
		{"Innodb_rows_read", true, "counter: innodb_rows family"},
		{"Innodb_rows_inserted", true, "counter: innodb_rows family"},
		// innodb_buffer_pool_pages sub-switch (mysqld_exporter
		// global_status.go:147-163).
		{"Innodb_buffer_pool_pages_flushed", true, "counter: innodb_buffer_pool_pages sub-switch default"},
		{"Innodb_buffer_pool_pages_lru_flushed", true, "counter: innodb_buffer_pool_pages sub-switch default"},
		{"Innodb_buffer_pool_pages_made_young", true, "counter: innodb_buffer_pool_pages sub-switch default"},
		{"Innodb_buffer_pool_pages_made_not_young", true, "counter: innodb_buffer_pool_pages sub-switch default"},
		{"Innodb_buffer_pool_pages_data", false, "gauge: innodb_buffer_pool_pages state"},
		{"Innodb_buffer_pool_pages_dirty", false, "gauge: innodb_buffer_pool_pages state"},
		{"Innodb_buffer_pool_pages_free", false, "gauge: innodb_buffer_pool_pages state"},
		{"Innodb_buffer_pool_pages_misc", false, "gauge: innodb_buffer_pool_pages state"},
		{"Innodb_buffer_pool_pages_old", false, "gauge: innodb_buffer_pool_pages state"},
		{"Innodb_buffer_pool_pages_total", false, "gauge: buffer pool capacity (skipped by exporter)"},
		// Explicit-override counters (JSON allowlist).
		{"Bytes_sent", true, "counter: json override"},
		{"Bytes_received", true, "counter: json override"},
		{"Questions", true, "counter: json override"},
		{"Queries", true, "counter: json override"},
		{"Connections", true, "counter: json override"},
		{"Slow_queries", true, "counter: json override"},
		{"Threads_created", true, "counter: json override"},
		{"Table_open_cache_misses", true, "counter: json override"},
		{"Aborted_clients", true, "counter: json override"},
		// Explicit-override gauges (JSON allowlist).
		{"Threads_running", false, "gauge: json override"},
		{"Threads_connected", false, "gauge: json override"},
		{"Threads_cached", false, "gauge: json override"},
		{"Uptime", false, "gauge: json override (monotonic but delta is meaningless)"},
		{"Open_tables", false, "gauge: json override"},
		{"Open_files", false, "gauge: json override (regression fix — was misclassified as counter)"},
		{"Innodb_row_lock_current_waits", false, "gauge: json override"},
		{"Innodb_num_open_files", false, "gauge: json override"},
		{"Prepared_stmt_count", false, "gauge: json override"},
		{"Max_used_connections", false, "gauge: json override (peak since reset)"},
	}
	for _, c := range classifierSpotCheck {
		got, present := data.IsCounter[c.name]
		if !present {
			t.Logf("%s: not in fixture — skipping allowlist check (%s)", c.name, c.category)
			continue
		}
		if got != c.counter {
			t.Errorf("IsCounter[%q] = %v; want %v (%s)", c.name, got, c.counter, c.category)
		}
	}

	// Route Deltas through ScrubFloats before marshalling. Finite
	// values pass through unchanged; NaN/±Inf become JSON null. This
	// is the opt-in NaN path documented on
	// tests/goldens/goldens.go::MarshalDeterministic.
	got := goldens.MarshalDeterministic(t, struct {
		VariableNames      any `json:"variableNames"`
		SampleCount        any `json:"sampleCount"`
		Timestamps         any `json:"timestamps"`
		Deltas             any `json:"deltas"`
		IsCounter          any `json:"isCounter"`
		SnapshotBoundaries any `json:"snapshotBoundaries"`
		Diagnostics        any `json:"diagnostics"`
	}{
		VariableNames:      data.VariableNames,
		SampleCount:        data.SampleCount,
		Timestamps:         data.Timestamps,
		Deltas:             goldens.ScrubFloats(data.Deltas),
		IsCounter:          data.IsCounter,
		SnapshotBoundaries: data.SnapshotBoundaries,
		Diagnostics:        diags,
	})
	goldens.Compare(t, goldenPath, got)
}

// TestSnapshotBoundaryReset: T066 / FR-030 / Principle VIII.
//
// Multi-snapshot mysqladmin parses concatenate pt-stalk snapshots
// into one sample stream for boundary handling. The delta arithmetic
// MUST reset at every snapshot boundary — carrying the previous
// snapshot's final counter value into the next snapshot's first
// sample would produce a nonsense delta (often enormous) that would
// distort every counter chart. (The merged Timestamps slice is not
// part of this test's contract: the internal parser's `finish`
// synthesises 1-second-spaced timestamps from its most recently
// installed `timestampBase`, so a true cross-snapshot wall-clock
// merge is the render layer's responsibility — T054 covers that.)
//
// Assertions (per FR-030):
//   - SnapshotBoundaries contains 0 plus every additional boundary
//     index at the exact position where the second file's first
//     column sits;
//   - for every counter variable, Deltas[v][boundaryIdx] is
//     math.NaN() (which the render layer surfaces as a visible chart
//     gap);
//   - one SeverityInfo diagnostic is emitted per non-zero boundary;
//   - gauges are unaffected (raw values at boundary indices are
//     finite, not NaN).
//
// This exercises the internal parser API
// (newMysqladminParser + parseOneFile + ResetForNewSnapshot +
// finish) because parseMysqladmin is a single-file convenience
// wrapper. Note the integration path in the MVP: render/concatMysqladmin
// merges finished per-Snapshot *MysqladminData records rather than
// calling the internal API, so this test is the direct coverage for
// the ResetForNewSnapshot boundary-marking behaviour; concat's own
// cross-Snapshot merge is covered by render-layer tests (T054).
func TestSnapshotBoundaryReset(t *testing.T) {
	root := goldens.RepoRoot(t)
	first := filepath.Join(root, "testdata", "example2", "2026_04_21_16_51_41-mysqladmin")
	second := filepath.Join(root, "testdata", "example2", "2026_04_21_16_52_11-mysqladmin")

	p := newMysqladminParser(first+"+"+second, mysqladminSnapshotStart())

	f1, err := os.Open(first)
	if err != nil {
		t.Fatalf("open first fixture: %v", err)
	}
	p.parseOneFile(f1)
	f1.Close()

	// Record the column count at boundary time — the next ingested
	// column should be marked as a boundary at this 0-based index.
	boundaryExpectedAt := p.col

	p.ResetForNewSnapshot(mysqladminSnapshotStartSecond())

	f2, err := os.Open(second)
	if err != nil {
		t.Fatalf("open second fixture: %v", err)
	}
	p.parseOneFile(f2)
	f2.Close()

	data, diags := p.finish()
	if data == nil {
		t.Fatalf("finish returned nil data (diagnostics: %+v)", diags)
	}

	// SnapshotBoundaries must include the exact column index at which
	// the second file began. [0] plus one more entry is the minimum
	// shape for a two-file concat.
	if len(data.SnapshotBoundaries) < 2 {
		t.Fatalf("SnapshotBoundaries = %v; expected at least [0, %d] for a two-file concat", data.SnapshotBoundaries, boundaryExpectedAt)
	}
	if data.SnapshotBoundaries[0] != 0 {
		t.Errorf("SnapshotBoundaries[0] = %d; must be 0 (first sample of first snapshot)", data.SnapshotBoundaries[0])
	}
	sawExpected := false
	for _, b := range data.SnapshotBoundaries {
		if b == boundaryExpectedAt {
			sawExpected = true
			break
		}
	}
	if !sawExpected {
		t.Errorf("SnapshotBoundaries = %v; missing the %d marker captured at ResetForNewSnapshot time", data.SnapshotBoundaries, boundaryExpectedAt)
	}

	// Every non-zero boundary must land on a NaN in every counter and
	// a finite value in every gauge. This is the algorithmic contract
	// of FR-030: counter deltas can't be computed across a boundary,
	// but gauges are per-sample absolute values that remain defined.
	for _, b := range data.SnapshotBoundaries {
		if b == 0 {
			continue
		}
		for _, name := range data.VariableNames {
			slot := data.Deltas[name][b]
			if data.IsCounter[name] {
				if !math.IsNaN(slot) {
					t.Errorf("counter %q at boundary %d = %v; FR-030 requires NaN (reset cannot produce a delta)", name, b, slot)
				}
			} else {
				if math.IsNaN(slot) {
					t.Errorf("gauge %q at boundary %d = NaN; gauges carry raw values across boundaries (drift-NaN would indicate variable drift, not boundary reset)", name, b)
				}
			}
		}
	}

	// One SeverityInfo diagnostic per non-zero boundary (ResetForNewSnapshot
	// emits exactly one). Warnings (drift) and Errors (read failure) are
	// allowed alongside — this is an equality check on the Info count
	// only.
	nonZeroBoundaries := 0
	for _, b := range data.SnapshotBoundaries {
		if b != 0 {
			nonZeroBoundaries++
		}
	}
	infoCount := 0
	for _, d := range diags {
		if d.Severity == model.SeverityInfo {
			infoCount++
		}
	}
	if infoCount != nonZeroBoundaries {
		t.Errorf("expected %d SeverityInfo diagnostics (one per non-zero boundary); got %d (diagnostics: %+v)",
			nonZeroBoundaries, infoCount, diags)
	}
}

// TestVariableDrift: T067 / research R8 improvement C / Principle VIII.
//
// A counter variable that appears in one snapshot block but not the
// next must (a) produce `math.NaN()` at every drift slot so the chart
// shows a visible gap, and (b) emit exactly one SeverityWarning
// diagnostic naming the variable. The FR-028 / R8.C contract is
// explicitly counter-specific — gauges carry raw values, so their
// "missing sample" shape is trivially the same NaN pad but without
// the counter-arithmetic hazard.
//
// The test builds two in-memory pt-stalk -mysqladmin blocks so the
// drift condition is deterministic and independent of any fixture
// refresh. The first block reports three variables — two surviving
// (Com_select, Bytes_sent) and one drifter (Com_missing, which
// matches the "Com_" prefix in classifyAsCounter and is therefore
// classified as a counter). The second block drops Com_missing
// entirely. After the concat, Deltas[Com_missing] should be NaN at
// every post-drift column, IsCounter[Com_missing] should be true,
// and one Warning should name "Com_missing".
func TestVariableDrift(t *testing.T) {
	// Two pt-stalk mysqladmin files. The first has headers (col 1,
	// col 2), so we see the raw initial tally (col 1) and one delta
	// (col 2). The second block drops Com_missing after
	// ResetForNewSnapshot, leaving it to be NaN-padded at finish()
	// time.
	firstBlock := strings.Join([]string{
		"| Variable_name | Value |",
		"| Com_select | 100 |",
		"| Bytes_sent | 1000 |",
		"| Com_missing | 42 |",
		"| Variable_name | Value |",
		"| Com_select | 110 |",
		"| Bytes_sent | 1200 |",
		"| Com_missing | 50 |",
		"",
	}, "\n")
	// Second block: Com_select and Bytes_sent continue (both also
	// counters, both kept present). Com_missing is absent — the
	// drift scenario we want to exercise.
	secondBlock := strings.Join([]string{
		"| Variable_name | Value |",
		"| Com_select | 200 |",
		"| Bytes_sent | 2000 |",
		"| Variable_name | Value |",
		"| Com_select | 210 |",
		"| Bytes_sent | 2100 |",
		"",
	}, "\n")

	p := newMysqladminParser("drift-test", mysqladminSnapshotStart())
	p.parseOneFile(strings.NewReader(firstBlock))
	p.ResetForNewSnapshot(mysqladminSnapshotStartSecond())
	p.parseOneFile(strings.NewReader(secondBlock))

	data, diags := p.finish()
	if data == nil {
		t.Fatalf("finish returned nil data (diagnostics: %+v)", diags)
	}

	// Sanity: four columns total (2 from first block + 2 from
	// second). If the parser miscounts, every downstream assertion
	// is suspect — fail fast with a direct message.
	if data.SampleCount != 4 {
		t.Fatalf("SampleCount = %d; expected 4 (2 + 2 columns across the two blocks)", data.SampleCount)
	}

	// Guard the counter-classification premise: if a future
	// classifyAsCounter refactor demotes Com_missing to a gauge, this
	// test would silently fall back to exercising the generic
	// missing-slot padding path (which applies to gauges too) and no
	// longer cover R8.C's counter-specific contract. Fail loudly.
	if !data.IsCounter["Com_missing"] {
		t.Fatalf("IsCounter[\"Com_missing\"] = false; the R8.C counter-drift test requires Com_missing be classified as a counter (Com_ prefix matches classifyAsCounter)")
	}

	// Missing slots for Com_missing must be NaN, not zero. Zero would
	// be a perfectly plausible real value and would silently
	// under-report drift.
	cSlots := data.Deltas["Com_missing"]
	if len(cSlots) != 4 {
		t.Fatalf("Deltas[Com_missing] length = %d; want 4 (NaN-padded to SampleCount)", len(cSlots))
	}
	// Columns 0 and 1 were the first block — finite values.
	for i := 0; i < 2; i++ {
		if math.IsNaN(cSlots[i]) {
			t.Errorf("Deltas[Com_missing][%d] = NaN; first block provided a value here", i)
		}
	}
	// Columns 2 and 3 were the second block — drift, must be NaN.
	for i := 2; i < 4; i++ {
		if !math.IsNaN(cSlots[i]) {
			t.Errorf("Deltas[Com_missing][%d] = %v; drift slot must be NaN (R8.C)", i, cSlots[i])
		}
	}

	// Exactly one SeverityWarning naming Com_missing. The parser may
	// emit additional Info diagnostics (the boundary reset), so we
	// filter by severity AND content; other Warnings are fine as
	// long as we hit our drift warning exactly once.
	driftWarnings := 0
	for _, d := range diags {
		if d.Severity != model.SeverityWarning {
			continue
		}
		if strings.Contains(d.Message, "Com_missing") {
			driftWarnings++
		}
	}
	if driftWarnings != 1 {
		t.Errorf("expected exactly 1 SeverityWarning mentioning Com_missing; got %d (diagnostics: %+v)", driftWarnings, diags)
	}
}
