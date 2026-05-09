package render

import (
	"strings"
	"testing"
)

// TestLoadMySQLDefaultsVersioned asserts the embedded JSON is parsed
// into the versioned shape with every column listed in `versions`
// represented by at least one variable-level entry. Catches malformed
// JSON or a schema drift (e.g. dropping a column) at build time.
func TestLoadMySQLDefaultsVersioned(t *testing.T) {
	defaults, versions := loadMySQLDefaults()
	if len(versions) == 0 {
		t.Fatalf("versions list empty; expected at least one supported version")
	}
	if len(defaults) == 0 {
		t.Fatalf("defaults map empty; expected many variable entries")
	}
	seen := map[string]bool{}
	for _, perVersion := range defaults {
		for v := range perVersion {
			seen[v] = true
		}
	}
	for _, v := range versions {
		if !seen[v] {
			t.Errorf("version %q listed in `versions` but no variable defines a default for it", v)
		}
	}
}

// TestClassifyVariableVersionDivergent exercises the three version
// columns (5.7, 8.0, 8.4) on variables whose documented default
// actually differs across versions. A value that matches 8.0 must be
// flagged as modified against the 5.7 default, and vice versa.
func TestClassifyVariableVersionDivergent(t *testing.T) {
	defaults, supported := loadMySQLDefaults()

	tests := []struct {
		name     string
		version  string
		variable string
		observed string
		want     string
	}{
		// character_set_server: latin1 on 5.7, utf8mb4 on 8.0+.
		{"5.7 latin1 is default", "5.7.44", "character_set_server", "latin1", "default"},
		{"5.7 utf8mb4 is modified", "5.7.44", "character_set_server", "utf8mb4", "modified"},
		{"8.0 utf8mb4 is default", "8.0.32", "character_set_server", "utf8mb4", "default"},
		{"8.0 latin1 is modified", "8.0.32", "character_set_server", "latin1", "modified"},
		{"8.4 utf8mb4 is default", "8.4.0", "character_set_server", "utf8mb4", "default"},

		// explicit_defaults_for_timestamp: OFF on 5.7, ON on 8.0+.
		{"5.7 OFF is default", "5.7.44", "explicit_defaults_for_timestamp", "OFF", "default"},
		{"8.0 ON is default", "8.0.32", "explicit_defaults_for_timestamp", "ON", "default"},
		{"5.7 ON is modified", "5.7.44", "explicit_defaults_for_timestamp", "ON", "modified"},

		// innodb_io_capacity: 200 on 5.7/8.0, 10000 on 8.4.
		{"5.7 io_capacity 200 default", "5.7.44", "innodb_io_capacity", "200", "default"},
		{"8.0 io_capacity 200 default", "8.0.36", "innodb_io_capacity", "200", "default"},
		{"8.4 io_capacity 10000 default", "8.4.0", "innodb_io_capacity", "10000", "default"},
		{"8.4 io_capacity 200 modified", "8.4.0", "innodb_io_capacity", "200", "modified"},

		// innodb_autoinc_lock_mode: 1 on 5.7, 2 on 8.0+.
		{"5.7 autoinc 1 default", "5.7.44", "innodb_autoinc_lock_mode", "1", "default"},
		{"8.0 autoinc 2 default", "8.0.32", "innodb_autoinc_lock_mode", "2", "default"},
		{"8.0 autoinc 1 modified", "8.0.32", "innodb_autoinc_lock_mode", "1", "modified"},

		// Unknown version falls through to "unknown" rather than
		// reporting a false "modified" against an unchecked column.
		{"no version string unknown", "", "character_set_server", "utf8mb4", "unknown"},
		{"unparseable version unknown", "custom-build", "character_set_server", "utf8mb4", "unknown"},

		// Unknown variable returns "unknown".
		{"unknown var", "8.0.32", "nonexistent_variable_xyz", "any", "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyVariable(defaults, supported, tc.version, tc.variable, tc.observed)
			if got != tc.want {
				t.Errorf("classifyVariable(version=%q, %q, %q) = %q; want %q",
					tc.version, tc.variable, tc.observed, got, tc.want)
			}
		})
	}
}

// TestResolveVersionFallback exercises the "latest supported ≤
// captured" fallback so a capture from an unlisted release (e.g. 8.1
// or 8.3) still resolves to 8.0 rather than "unknown".
func TestResolveVersionFallback(t *testing.T) {
	supported := []string{"5.7", "8.0", "8.4"}
	cases := []struct {
		captured string
		want     string
	}{
		{"5.7.44", "5.7"},
		{"8.0.32", "8.0"},
		{"8.1.0", "8.0"},  // fallback down to 8.0
		{"8.3.99", "8.0"}, // still 8.0 (8.4 > captured)
		{"8.4.0", "8.4"},
		{"8.5.1", "8.4"}, // newer than supported; picks latest ≤ captured
		{"5.6.50", ""},   // older than every column
		{"", ""},
		{"abc", ""},
	}
	for _, c := range cases {
		got := resolveVersion(c.captured, supported)
		if got != c.want {
			t.Errorf("resolveVersion(%q) = %q; want %q", c.captured, got, c.want)
		}
	}
}

// TestMajorVersionMultiDigit guards against the naive raw[:3] slicing
// that used to mis-parse multi-digit majors (10.x) and minors (8.10).
func TestMajorVersionMultiDigit(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"8.0.32-24", "8.0"},
		{"5.7.44-48-log", "5.7"},
		{"8.4.2", "8.4"},
		{"10.4.32-MariaDB", "10.4"},
		{"8.10.1", "8.10"},
		{"11.4.5-LTS", "11.4"},
		{"", ""},
		{"Ver 8", ""},
		{"x.y", ""},
		{"8.", ""},
		{".8", ""},
	}
	for _, c := range cases {
		if got := majorVersion(c.in); got != c.want {
			t.Errorf("majorVersion(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestFeedbackWorkerClientContract(t *testing.T) {
	app := embeddedAppJS
	for _, forbidden := range []string{
		"mysqladmin:selected",
		"mysqladminSelectionKey",
		"LEGACY_KEY",
	} {
		if strings.Contains(app, forbidden) {
			t.Fatalf("app JS still carries legacy mysqladmin persistence path %q", forbidden)
		}
	}
	for _, want := range []string{
		`renderSuccess(data.issueUrl, data.issueNumber)`,
		`successLink.focus()`,
		`res.headers.get("Retry-After")`,
		`View issue #`,
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("app JS missing feedback Worker contract marker %q", want)
		}
	}
}

// TestChartZoomSyncContract asserts the embedded JS contains the
// canonical chart-sync-store and windowed-stats markers from
// specs/017-chart-zoom-sync-stats/contracts/chart-sync.md, and does
// not retain any non-canonical reset-zoom or duplicate mountLegend
// path.
func TestChartZoomSyncContract(t *testing.T) {
	app := embeddedAppJS
	for _, want := range []string{
		// Store identity.
		"window.__chartSyncStore",
		"chartSyncStore.subscribe",
		// Public methods (called from broadcast hooks and reset path).
		"setWindow:",
		"reset: function",
		"applyToChart:",
		// Windowed-stats helper + the canonical pill marker the legend
		// renders for each series.
		"computeWindowedStats",
		"series-pill-stats",
		"setLegendStats",
		// Boot wiring lives in app-js/04.js (initChartSync defined in
		// 05.js); both must ship together.
		"initChartSync()",
		"function initChartSync()",
		// The reset-zoom button click handler routes through the store
		// so every chart resets, not just the one the button is on.
		"window.__chartSyncStore.reset()",
		// The basePlotOpts setScale broadcaster name is canonical.
		"broadcastXScaleChange",
		// 05.js banner comment proves the new ordered part is embedded.
		"app-js/05.js",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("embedded app JS missing chart-sync canonical marker %q", want)
		}
	}
	for _, forbidden := range []string{
		// The old per-chart reset button used to call setScale on a
		// single plot; the new canonical path goes through the store.
		// If anyone reintroduces a direct setScale("x" outside the
		// store, this guard fires.
		`getPlot().setScale("x"`,
		// The mysqladmin per-card zoom button used to call
		// cs.plot.setScale directly; same guard.
		`cs.plot.setScale("x"`,
	} {
		if strings.Contains(app, forbidden) {
			t.Fatalf("embedded app JS still contains non-canonical zoom-reset path %q", forbidden)
		}
	}
	// mountLegend has exactly one definition.
	if got := strings.Count(app, "function mountLegend("); got != 1 {
		t.Fatalf("mountLegend defined %d times in embedded app JS; want exactly 1 (canonical owner)", got)
	}
}

// TestChartSyncWindowedStatsFlow asserts the legend recomputation flow
// is wired end-to-end in the embedded JS: the store subscriber walks
// the registry and calls each chart's setStats with the new window,
// and a chart that mounts post-zoom adopts the current window via
// applyToChart on registration.
func TestChartSyncWindowedStatsFlow(t *testing.T) {
	app := embeddedAppJS
	for _, want := range []string{
		// On every store update, the subscriber pings each chart's
		// legend handle.
		"h.setStats(win)",
		// On registration, a chart that mounts after a zoom adopts
		// the current shared window without an extra interaction.
		"window.__chartSyncStore.applyToChart",
		// Stacked-chart stats source is the un-stacked raw values, not
		// the cumulative `stacked[]` arrays uPlot draws.
		"stacked charts whose drawn values are cumulative",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("embedded app JS missing chart-sync flow marker %q", want)
		}
	}
}

// TestProcesslistStackedDimensionLegendStats is a regression guard for
// PR #59 review finding (P1): the non-activity stacked dimensions
// inside renderProcesslist (State / User / Host / Command / db) used to
// call registerChart with no `data` payload and mountLegend with no
// `statsSource`, silently violating spec 017 FR-007 (windowed Min/Avg/
// Max pills on every synced chart). The fix wires both, mirroring the
// activity stacked path in buildStackedChart. This test asserts the
// canonical wiring inside the renderProcesslist body so a future drift
// is caught at build time.
func TestProcesslistStackedDimensionLegendStats(t *testing.T) {
	app := embeddedAppJS
	idx := strings.Index(app, "function renderProcesslist")
	if idx < 0 {
		t.Fatalf("renderProcesslist not found in embedded app JS")
	}
	end := strings.Index(app[idx:], "\n  function ")
	if end < 0 {
		end = len(app) - idx
	}
	body := app[idx : idx+end]
	for _, want := range []string{
		// statsSource must be passed to mountLegend so the legend pills
		// recompute Min/Avg/Max on every store window change.
		"statsSource: { timestamps: data.timestamps, series: statsSeries }",
		// The stacked-dimension chart must capture the legend handle on
		// the plot so initChartSync's first-pass and the post-zoom
		// applyToChart can find it.
		"plot.__legendHandle = mountLegend(",
		// registerChart must receive a non-nil 4th-arg `data` payload so
		// applyToChart can adopt a pre-existing zoom window.
		"registerChart(plot, el, opts, {",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("renderProcesslist body missing canonical chart-sync wiring %q (PR #59 P1 regression)", want)
		}
	}
}

// TestMountLegendStatsConditionalOnStatsSource is a regression guard for
// PR #59 review finding (Copilot, 00.js:764): the legend rendering used
// to unconditionally append the `series-pill-stats` markup defaulting to
// "–", but `setStats()` is a no-op when `opts.statsSource` is omitted.
// Result: charts that don't wire statsSource showed permanent em-dashes
// in pill stat slots that would never get populated. The fix gates the
// markup append on `statsSource` so non-numeric legends get a clean pill
// and only stats-aware callers render the slot. This test asserts the
// canonical guard is present inside `mountLegend`.
func TestMountLegendStatsConditionalOnStatsSource(t *testing.T) {
	app := embeddedAppJS
	idx := strings.Index(app, "function mountLegend(")
	if idx < 0 {
		t.Fatalf("mountLegend not found in embedded app JS")
	}
	end := strings.Index(app[idx:], "\n  function ")
	if end < 0 {
		end = len(app) - idx
	}
	body := app[idx : idx+end]
	// The pill stats markup must be gated by a `statsSource ?` ternary so
	// it is omitted (not just empty) when the caller did not wire stats.
	statsMarkupIdx := strings.Index(body, "series-pill-stats")
	if statsMarkupIdx < 0 {
		t.Fatalf("mountLegend body missing series-pill-stats markup")
	}
	// Look for the canonical guard `(statsSource\n          ?` immediately
	// preceding the markup. Substring check for `statsSource` within a
	// short window before the markup is sufficient to assert the guard
	// without depending on exact whitespace.
	prelude := body[:statsMarkupIdx]
	lastGuard := strings.LastIndex(prelude, "statsSource")
	if lastGuard < 0 || statsMarkupIdx-lastGuard > 200 {
		t.Fatalf("series-pill-stats markup is not gated on statsSource within mountLegend (PR #59 Copilot regression: empty pills appear when statsSource is omitted)")
	}
	// The statsByIdx capture must also be gated so we don't allocate
	// references to elements that don't exist.
	if !strings.Contains(body, "if (statsSource) {\n        statsByIdx[String(i)] = {") {
		t.Fatalf("statsByIdx capture is not gated on statsSource (PR #59 Copilot regression: querySelector against non-existent elements)")
	}
}

// TestStackedChartStatsSourceUsesUnbucketedSeries is a regression guard
// for PR #59 review finding (Codex P1, 01.js:201): buildStackedChart
// built statsSeries from the bucketized `series` (length n_buckets) but
// wired data.timestamps (length n_samples) into statsSource. The
// computeWindowedStats helper indexes by timestamps, so once the user
// zoomed beyond the first n_buckets indexes it read mostly undefined
// values and Min/Avg/Max collapsed to "–". The fix iterates over
// `data.series` (the un-bucketed raw values, aligned to data.timestamps)
// per the chart-sync contract §1 (rawSeries semantics).
func TestStackedChartStatsSourceUsesUnbucketedSeries(t *testing.T) {
	app := embeddedAppJS
	idx := strings.Index(app, "function buildStackedChart(")
	if idx < 0 {
		t.Fatalf("buildStackedChart not found in embedded app JS")
	}
	end := strings.Index(app[idx:], "\n  function ")
	if end < 0 {
		end = len(app) - idx
	}
	body := app[idx : idx+end]
	// Canonical wiring: iterate over data.series (un-bucketed raw),
	// matching data.timestamps.
	if !strings.Contains(body, "for (var sk = data.series.length - 1; sk >= 0; sk--)") {
		t.Fatalf("buildStackedChart statsSeries loop must iterate over data.series (un-bucketed) — PR #59 Codex P1 regression: bucketed series mis-aligned with data.timestamps")
	}
	if !strings.Contains(body, "data.series[sk].values") {
		t.Fatalf("buildStackedChart statsSeries must pull values from data.series[sk] (un-bucketed) — PR #59 Codex P1 regression")
	}
}

// TestHLLSparklineExemptFromSync preserves the canonical opt-out: the
// HLL sparkline must NOT call registerChart, so it is never reached by
// the chart-sync broadcaster or the windowed-stats subscriber.
func TestHLLSparklineExemptFromSync(t *testing.T) {
	app := embeddedAppJS
	idx := strings.Index(app, "function renderHLLSparkline")
	if idx < 0 {
		t.Fatalf("renderHLLSparkline not found in embedded app JS")
	}
	end := strings.Index(app[idx:], "\n  function ")
	if end < 0 {
		end = len(app) - idx
	}
	body := app[idx : idx+end]
	// Strip line comments so explanatory mentions of `registerChart()`
	// in prose do not register as call sites. The check is for an
	// actual call, not the literal substring.
	for _, line := range strings.Split(body, "\n") {
		code := line
		if i := strings.Index(code, "//"); i >= 0 {
			code = code[:i]
		}
		if strings.Contains(code, "registerChart(") {
			t.Fatalf("renderHLLSparkline must not call registerChart — that is the canonical opt-out from chart sync")
		}
	}
}

// TestEmbeddedAppJSHasIIFEClosed asserts every top-level IIFE in
// the embedded JS opens and closes — opens-count must equal
// closes-count, and the bundle must end with `})();`. The chart
// helper IIFE (00.js → 05.js) and the theme module IIFE (06.js)
// are independent top-level wrappers; both must be balanced.
func TestEmbeddedAppJSHasIIFEClosed(t *testing.T) {
	app := embeddedAppJS
	if !strings.HasSuffix(strings.TrimRight(app, "\n"), "\n})();") {
		t.Fatalf("expected an IIFE close `})();` at start of the last non-blank line in embedded app JS")
	}
	useStrictOpens := strings.Count(app, "(function () {\n  \"use strict\";")
	if useStrictOpens < 1 {
		t.Fatalf("expected at least one strict-mode IIFE opener in embedded app JS")
	}
	opens := strings.Count(app, "\n(function () {")
	closes := strings.Count(app, "\n})();")
	if opens != closes {
		t.Fatalf("top-level IIFE opens (%d) and closes (%d) must match", opens, closes)
	}
	if opens < 1 {
		t.Fatalf("expected at least one top-level IIFE in embedded app JS")
	}
}

// TestResolveVersionMultiDigitSort ensures resolveVersion sorts
// numerically (not lexicographically) so "10.4" is later than "9.0".
func TestResolveVersionMultiDigitSort(t *testing.T) {
	supported := []string{"8.0", "8.4", "9.0", "10.0", "10.4"}
	cases := []struct{ captured, want string }{
		{"10.4.1", "10.4"},
		{"10.5.0", "10.4"}, // fallback to latest ≤ captured
		{"10.3.9", "10.0"}, // 10.3 < 10.4 so fall back to 10.0
		{"9.5.2", "9.0"},
		{"8.4.0", "8.4"},
	}
	for _, c := range cases {
		if got := resolveVersion(c.captured, supported); got != c.want {
			t.Errorf("resolveVersion(%q) = %q; want %q", c.captured, got, c.want)
		}
	}
}
