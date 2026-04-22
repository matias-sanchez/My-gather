package findings

import (
	"strings"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// builder helps construct a *model.Report with just the fields
// findings rules read. Pad deltas by prepending a zero at index 0
// because counter helpers skip index 0 by design (pt-mext raw tally
// convention; see inputs.go::counterTotal).
type builder struct {
	counters    map[string][]float64
	gauges      map[string][]float64
	isCounter   map[string]bool
	vars        map[string]string
	windowSecs  float64
}

func newBuilder() *builder {
	return &builder{
		counters:   map[string][]float64{},
		gauges:     map[string][]float64{},
		isCounter:  map[string]bool{},
		vars:       map[string]string{},
		windowSecs: 30,
	}
}

// counter adds a counter variable with the given per-sample delta
// total distributed evenly across the sample window.
func (b *builder) counter(name string, total float64) *builder {
	// 11 samples: index 0 is the raw initial tally (ignored), indices
	// 1..10 each carry total/10 so sum == total.
	const n = 11
	arr := make([]float64, n)
	arr[0] = 999999 // ignored by counterTotal
	each := total / (n - 1)
	for i := 1; i < n; i++ {
		arr[i] = each
	}
	b.counters[name] = arr
	b.isCounter[name] = true
	return b
}

// gauge adds a gauge variable. Last value is `last`; previous samples
// scale linearly from max/2 up to `last`.
func (b *builder) gauge(name string, last float64) *builder {
	const n = 11
	arr := make([]float64, n)
	for i := 0; i < n; i++ {
		arr[i] = last
	}
	b.gauges[name] = arr
	b.isCounter[name] = false
	return b
}

// gaugeMax sets a gauge whose maximum across the window is `max`
// (used by rules that consume gaugeMax rather than gaugeLast).
func (b *builder) gaugeMax(name string, max float64) *builder {
	const n = 11
	arr := make([]float64, n)
	arr[n/2] = max
	b.gauges[name] = arr
	b.isCounter[name] = false
	return b
}

func (b *builder) variable(name, value string) *builder {
	b.vars[name] = value
	return b
}

func (b *builder) withWindow(sec float64) *builder {
	b.windowSecs = sec
	return b
}

func (b *builder) build() *model.Report {
	deltas := map[string][]float64{}
	for k, v := range b.counters {
		deltas[k] = v
	}
	for k, v := range b.gauges {
		deltas[k] = v
	}
	names := make([]string, 0, len(deltas))
	for k := range deltas {
		names = append(names, k)
	}
	n := 0
	for _, v := range deltas {
		if len(v) > n {
			n = len(v)
		}
	}
	timestamps := make([]time.Time, n)
	start := time.Unix(1_700_000_000, 0).UTC()
	step := time.Duration(0)
	if n > 1 && b.windowSecs > 0 {
		step = time.Duration(b.windowSecs*float64(time.Second)) / time.Duration(n-1)
	}
	for i := 0; i < n; i++ {
		timestamps[i] = start.Add(time.Duration(i) * step)
	}

	m := &model.MysqladminData{
		VariableNames: names,
		Timestamps:    timestamps,
		Deltas:        deltas,
		IsCounter:     map[string]bool{},
		SampleCount:   n,
	}
	for k, v := range b.isCounter {
		m.IsCounter[k] = v
	}

	entries := make([]model.VariableEntry, 0, len(b.vars))
	for k, v := range b.vars {
		entries = append(entries, model.VariableEntry{Name: k, Value: v})
	}
	vs := &model.VariablesSection{
		PerSnapshot: []model.SnapshotVariables{
			{
				SnapshotPrefix: "snap1",
				Timestamp:      start,
				Data:           &model.VariablesData{Entries: entries},
			},
		},
	}
	return &model.Report{
		DBSection:        &model.DBSection{Mysqladmin: m},
		VariablesSection: vs,
	}
}

func findByID(fs []Finding, id string) *Finding {
	for i := range fs {
		if fs[i].ID == id {
			return &fs[i]
		}
	}
	return nil
}

func TestAnalyze_SkipsWhenNoMysqladmin(t *testing.T) {
	got := Analyze(&model.Report{})
	if len(got) != 0 {
		t.Fatalf("expected no findings for empty report; got %d", len(got))
	}
}

func TestBPHitRatio(t *testing.T) {
	cases := []struct {
		name     string
		reads    float64
		requests float64
		wantSev  Severity
		wantSubs string
	}{
		{"healthy", 10, 1_000_000, SeverityOK, "healthy"},
		{"warn", 5_000, 1_000_000, SeverityWarn, "degraded"},
		{"crit", 50_000, 1_000_000, SeverityCrit, "LOW"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := newBuilder().
				counter("Innodb_buffer_pool_reads", tc.reads).
				counter("Innodb_buffer_pool_read_requests", tc.requests)
			got := Analyze(b.build())
			f := findByID(got, "bp.hit_ratio")
			if f == nil {
				t.Fatalf("bp.hit_ratio not found")
			}
			if f.Severity != tc.wantSev {
				t.Errorf("severity: got %v, want %v", f.Severity, tc.wantSev)
			}
			if !strings.Contains(f.Summary, tc.wantSubs) {
				t.Errorf("summary %q does not contain %q", f.Summary, tc.wantSubs)
			}
		})
	}
}

func TestBPHitRatio_Skip(t *testing.T) {
	b := newBuilder() // no counters
	got := Analyze(b.build())
	if findByID(got, "bp.hit_ratio") != nil {
		t.Fatal("expected bp.hit_ratio to be skipped when counters absent")
	}
}

func TestBPWaitFree(t *testing.T) {
	b := newBuilder().counter("Innodb_buffer_pool_wait_free", 10)
	got := Analyze(b.build())
	f := findByID(got, "bp.wait_free")
	if f == nil || f.Severity != SeverityCrit {
		t.Fatalf("expected Crit; got %+v", f)
	}
}

func TestThreadCacheHitRatio(t *testing.T) {
	cases := []struct {
		name        string
		created     float64
		connections float64
		wantSev     Severity
	}{
		{"healthy", 1, 1_000, SeverityOK},
		{"warn_below_90", 200, 1_000, SeverityWarn},
		{"crit_below_70", 500, 1_000, SeverityCrit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := newBuilder().
				counter("Threads_created", tc.created).
				counter("Connections", tc.connections).
				variable("thread_cache_size", "9")
			got := Analyze(b.build())
			f := findByID(got, "threadcache.hit_ratio")
			if f == nil {
				t.Fatalf("rule not triggered")
			}
			if f.Severity != tc.wantSev {
				t.Errorf("got %v, want %v", f.Severity, tc.wantSev)
			}
		})
	}
}

func TestTableCacheOverflows(t *testing.T) {
	b := newBuilder().counter("Table_open_cache_overflows", 30) // 30/30s = 1/s → Warn
	got := Analyze(b.build())
	f := findByID(got, "tablecache.overflows")
	if f == nil || f.Severity != SeverityWarn {
		t.Fatalf("expected Warn, got %+v", f)
	}

	b2 := newBuilder().counter("Table_open_cache_overflows", 300) // 10/s → Crit
	got2 := Analyze(b2.build())
	f2 := findByID(got2, "tablecache.overflows")
	if f2 == nil || f2.Severity != SeverityCrit {
		t.Fatalf("expected Crit, got %+v", f2)
	}
}

func TestTableCacheMissRatio(t *testing.T) {
	cases := []struct {
		hits, misses float64
		want         Severity
	}{
		{1_000_000, 1_000, SeverityOK},   // 0.1 %
		{1_000, 30, SeverityWarn},         // ~2.9 %
		{100, 30, SeverityCrit},           // ~23 %
	}
	for _, tc := range cases {
		b := newBuilder().
			counter("Table_open_cache_hits", tc.hits).
			counter("Table_open_cache_misses", tc.misses)
		got := Analyze(b.build())
		f := findByID(got, "tablecache.miss_ratio")
		if f == nil {
			t.Fatalf("rule not triggered (hits=%v misses=%v)", tc.hits, tc.misses)
		}
		if f.Severity != tc.want {
			t.Errorf("hits=%v misses=%v: got %v, want %v", tc.hits, tc.misses, f.Severity, tc.want)
		}
	}
}

func TestTmpDiskRatio(t *testing.T) {
	b := newBuilder().
		counter("Created_tmp_tables", 100).
		counter("Created_tmp_disk_tables", 60)
	got := Analyze(b.build())
	f := findByID(got, "tmp.disk_ratio")
	if f == nil || f.Severity != SeverityCrit {
		t.Fatalf("expected Crit at 60%%, got %+v", f)
	}
}

func TestBinlogCacheDiskUse(t *testing.T) {
	b := newBuilder().counter("Binlog_cache_disk_use", 6) // 0.2/s → Warn
	got := Analyze(b.build())
	f := findByID(got, "binlog.cache_disk_use")
	if f == nil || f.Severity != SeverityWarn {
		t.Fatalf("expected Warn, got %+v", f)
	}
}

func TestRedoLogWaits(t *testing.T) {
	b := newBuilder().counter("Innodb_log_waits", 50) // ~1.67/s → Crit
	got := Analyze(b.build())
	f := findByID(got, "redo.log_waits")
	if f == nil || f.Severity != SeverityCrit {
		t.Fatalf("expected Crit, got %+v", f)
	}
}

func TestRedoCheckpointAge(t *testing.T) {
	b := newBuilder().
		gaugeMax("Innodb_checkpoint_age", 850_000).
		gauge("Innodb_checkpoint_max_age", 1_000_000)
	got := Analyze(b.build())
	f := findByID(got, "redo.checkpoint_age")
	if f == nil || f.Severity != SeverityCrit {
		t.Fatalf("expected Crit at 85%%, got %+v", f)
	}
}

func TestRedoPendingWrites(t *testing.T) {
	b := newBuilder().gaugeMax("Innodb_os_log_pending_writes", 3)
	got := Analyze(b.build())
	f := findByID(got, "redo.pending_writes")
	if f == nil || f.Severity != SeverityCrit {
		t.Fatalf("expected Crit, got %+v", f)
	}
}

func TestAbortedConnectsRate(t *testing.T) {
	b := newBuilder().counter("Aborted_connects", 10) // 10/30s ≈ 0.33/s → Warn
	got := Analyze(b.build())
	f := findByID(got, "connections.aborted_rate")
	if f == nil || f.Severity != SeverityWarn {
		t.Fatalf("expected Warn, got %+v", f)
	}
}

func TestFullScanSelectScan(t *testing.T) {
	b := newBuilder().counter("Select_scan", 30) // 1/s → Warn (need > 10 for Crit)
	got := Analyze(b.build())
	f := findByID(got, "queryshape.select_scan")
	if f == nil || f.Severity != SeverityWarn {
		t.Fatalf("expected Warn, got %+v", f)
	}
}

func TestBPFreePagesLow(t *testing.T) {
	// Canonical paths: (1) idle reads → Skip; (2) free > threshold → OK;
	// (3) free <= threshold AND reads > 0 → Warn; (4) also LRU flushing → Crit.
	t.Run("skip_when_idle", func(t *testing.T) {
		b := newBuilder().
			gauge("Innodb_buffer_pool_pages_free", 10).
			variable("innodb_lru_scan_depth", "1024").
			variable("innodb_buffer_pool_instances", "8").
			counter("Innodb_buffer_pool_reads", 0)
		f := findByID(Analyze(b.build()), "bp.free_pages_low")
		if f != nil {
			t.Fatalf("expected Skip (no finding), got %+v", f)
		}
	})
	t.Run("ok_when_headroom", func(t *testing.T) {
		b := newBuilder().
			gauge("Innodb_buffer_pool_pages_free", 50_000).
			variable("innodb_lru_scan_depth", "1024").
			variable("innodb_buffer_pool_instances", "8").
			counter("Innodb_buffer_pool_reads", 30)
		f := findByID(Analyze(b.build()), "bp.free_pages_low")
		if f == nil || f.Severity != SeverityOK {
			t.Fatalf("expected OK, got %+v", f)
		}
	})
	t.Run("warn_when_pressured", func(t *testing.T) {
		b := newBuilder().
			gauge("Innodb_buffer_pool_pages_free", 100).
			variable("innodb_lru_scan_depth", "1024").
			variable("innodb_buffer_pool_instances", "8").
			counter("Innodb_buffer_pool_reads", 30)
		f := findByID(Analyze(b.build()), "bp.free_pages_low")
		if f == nil || f.Severity != SeverityWarn {
			t.Fatalf("expected Warn, got %+v", f)
		}
	})
	t.Run("crit_when_pressured_and_lru_flushing", func(t *testing.T) {
		b := newBuilder().
			gauge("Innodb_buffer_pool_pages_free", 100).
			variable("innodb_lru_scan_depth", "1024").
			variable("innodb_buffer_pool_instances", "8").
			counter("Innodb_buffer_pool_reads", 30).
			counter("Innodb_buffer_pool_pages_LRU_flushed", 30)
		f := findByID(Analyze(b.build()), "bp.free_pages_low")
		if f == nil || f.Severity != SeverityCrit {
			t.Fatalf("expected Crit, got %+v", f)
		}
	})
}

func TestBPDirtyPct(t *testing.T) {
	// dirty/total > innodb_max_dirty_pages_pct → Crit when over, Warn at 90% of limit.
	b := newBuilder().
		gaugeMax("Innodb_buffer_pool_pages_dirty", 950).
		gauge("Innodb_buffer_pool_pages_total", 1_000).
		variable("innodb_max_dirty_pages_pct", "90")
	f := findByID(Analyze(b.build()), "bp.dirty_pct")
	if f == nil || f.Severity != SeverityCrit {
		t.Fatalf("expected Crit (95%% of total vs 90%% limit), got %+v", f)
	}
}

func TestBPLRUFlushing(t *testing.T) {
	b := newBuilder().counter("Innodb_buffer_pool_pages_LRU_flushed", 60) // 2/s → Warn
	f := findByID(Analyze(b.build()), "bp.lru_flushing")
	if f == nil || f.Severity != SeverityWarn {
		t.Fatalf("expected Warn, got %+v", f)
	}
}

func TestBinlogStmtCacheDiskUse(t *testing.T) {
	b := newBuilder().counter("Binlog_stmt_cache_disk_use", 6) // 0.2/s → Warn
	f := findByID(Analyze(b.build()), "binlog.stmt_cache_disk_use")
	if f == nil || f.Severity != SeverityWarn {
		t.Fatalf("expected Warn, got %+v", f)
	}
}

func TestFullScanSelectFullJoin(t *testing.T) {
	b := newBuilder().counter("Select_full_join", 30) // 1/s → Warn
	f := findByID(Analyze(b.build()), "queryshape.select_full_join")
	if f == nil || f.Severity != SeverityWarn {
		t.Fatalf("expected Warn, got %+v", f)
	}
}

func TestFullScanHandlerReadRndNext(t *testing.T) {
	// Crit threshold is 100_000 rows/s. 30-second window with total =
	// 3.5M delta ⇒ 116_666/s, well above critRate.
	b := newBuilder().counter("Handler_read_rnd_next", 3_500_000)
	f := findByID(Analyze(b.build()), "queryshape.handler_read_rnd_next")
	if f == nil || f.Severity != SeverityCrit {
		t.Fatalf("expected Crit, got %+v", f)
	}
}

func TestRedoPendingFsyncs(t *testing.T) {
	b := newBuilder().gaugeMax("Innodb_os_log_pending_fsyncs", 2)
	f := findByID(Analyze(b.build()), "redo.pending_fsyncs")
	if f == nil || f.Severity != SeverityCrit {
		t.Fatalf("expected Crit, got %+v", f)
	}
}

func TestAnalyze_DeterministicOrder(t *testing.T) {
	// Build a report that triggers multiple findings across subsystems.
	b := newBuilder().
		counter("Innodb_buffer_pool_reads", 50_000).
		counter("Innodb_buffer_pool_read_requests", 1_000_000).
		counter("Innodb_buffer_pool_wait_free", 100).
		counter("Threads_created", 400).
		counter("Connections", 1_000)
	a := Analyze(b.build())
	bRep := Analyze(b.build())
	if len(a) != len(bRep) {
		t.Fatalf("len mismatch: %d vs %d", len(a), len(bRep))
	}
	for i := range a {
		if a[i].ID != bRep[i].ID || a[i].Severity != bRep[i].Severity {
			t.Fatalf("ordering drift at %d: %v vs %v", i, a[i], bRep[i])
		}
	}
	// Buffer Pool findings must come before Thread Cache.
	var seenTC bool
	for _, f := range a {
		if f.Subsystem == "Thread Cache" {
			seenTC = true
		}
		if seenTC && f.Subsystem == "Buffer Pool" {
			t.Fatalf("Buffer Pool finding %q rendered after Thread Cache — ordering broken", f.ID)
		}
	}
}
