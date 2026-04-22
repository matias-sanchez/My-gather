package render

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"html/template"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/matias-sanchez/My-gather/findings"
	"github.com/matias-sanchez/My-gather/model"
)

//go:embed templates/*.tmpl
var embeddedTemplates embed.FS

//go:embed assets/chart.min.js
var embeddedChartJS string

//go:embed assets/chart.min.css
var embeddedChartCSS string

//go:embed assets/app.js
var embeddedAppJS string

//go:embed assets/app.css
var embeddedAppCSS string

//go:embed assets/mysql-defaults.json
var embeddedMySQLDefaultsJSON []byte

//go:embed assets/mysqladmin-categories.json
var embeddedMysqladminCategoriesJSON []byte

//go:embed assets/logo.png
var embeddedLogoPNG []byte

type mysqladminCategoryDef struct {
	Key             string   `json:"key"`
	Label           string   `json:"label"`
	Description     string   `json:"description"`
	Matchers        []string `json:"matchers"`
	ExcludeMatchers []string `json:"exclude_matchers"`
	Members         []string `json:"members"`
}

var (
	mysqladminCategories     []mysqladminCategoryDef
	mysqladminCategoriesOnce sync.Once
)

func loadMysqladminCategories() []mysqladminCategoryDef {
	mysqladminCategoriesOnce.Do(func() {
		var v struct {
			Categories []mysqladminCategoryDef `json:"categories"`
		}
		// Embedded at build time — a parse error is a programmer
		// error, not a user-runtime condition. Fail loudly instead
		// of silently producing an uncategorised chart view.
		if err := json.Unmarshal(embeddedMysqladminCategoriesJSON, &v); err != nil {
			panic(fmt.Sprintf("render/assets/mysqladmin-categories.json: malformed embedded JSON: %v", err))
		}
		mysqladminCategories = v.Categories
	})
	return mysqladminCategories
}

// classifyMysqladminCategory returns the slugs of every category that
// claims `name`. Matching is case-insensitive on prefixes. Explicit
// `members` take precedence; `exclude_matchers` carve out a subset of
// a broader matcher (e.g., buffer-pool excludes the read/write
// subviews so they live in InnoDB Reads / Writes instead).
func classifyMysqladminCategory(cats []mysqladminCategoryDef, name string) []string {
	lower := strings.ToLower(name)
	var hits []string
	for _, c := range cats {
		// Members list — highest priority.
		matched := false
		for _, m := range c.Members {
			if strings.EqualFold(m, name) {
				matched = true
				break
			}
		}
		if !matched {
			for _, mp := range c.Matchers {
				if strings.HasPrefix(lower, strings.ToLower(mp)) {
					matched = true
					break
				}
			}
			if matched {
				for _, ex := range c.ExcludeMatchers {
					if strings.HasPrefix(lower, strings.ToLower(ex)) {
						matched = false
						break
					}
				}
			}
		}
		if matched {
			hits = append(hits, c.Key)
		}
	}
	return hits
}

// mysqlDefaults is the parsed default-values map, populated on first use.
var (
	mysqlDefaults     map[string]string
	mysqlDefaultsOnce sync.Once
)

func loadMySQLDefaults() map[string]string {
	mysqlDefaultsOnce.Do(func() {
		var v struct {
			Defaults map[string]string `json:"defaults"`
		}
		// Embedded at build time — a parse error is a programmer
		// error. Fail loudly so the "non-default" badges can't
		// silently lose their reference map.
		if err := json.Unmarshal(embeddedMySQLDefaultsJSON, &v); err != nil {
			panic(fmt.Sprintf("render/assets/mysql-defaults.json: malformed embedded JSON: %v", err))
		}
		mysqlDefaults = v.Defaults
	})
	return mysqlDefaults
}

// RenderOptions controls optional aspects of rendering. The zero value
// is valid and uses the current UTC time and empty Version/GitCommit.
type RenderOptions struct {
	// GeneratedAt is the single explicitly non-deterministic field in
	// the rendered HTML (Constitution Principle IV, spec FR-006).
	// Tests pass a fixed value to make golden comparisons stable.
	GeneratedAt time.Time

	// Version is the tool's semver string ("v0.1.0").
	Version string

	// GitCommit is the short git SHA.
	GitCommit string

	// BuiltAt is the build timestamp (injected via -ldflags). Purely
	// informational; never rendered in a way that affects layout.
	BuiltAt string
}

// Render writes a self-contained HTML report for c to w. All CSS,
// JavaScript, fonts, and data are embedded inline; the resulting file
// makes zero network requests at view time (Constitution Principle V,
// spec FR-004).
//
// Render is deterministic: given the same Collection and RenderOptions
// (including GeneratedAt), two invocations MUST write byte-identical
// output to w (Constitution Principle IV, spec FR-006).
//
// Render returns an error only for I/O failures against w or for
// fatal template parsing errors. A Collection with missing or failed
// per-collector data is never an error — the corresponding section
// renders its "data not available" banner.
func Render(w io.Writer, c *model.Collection, opts RenderOptions) error {
	if c == nil {
		return fmt.Errorf("render: nil Collection")
	}
	if opts.GeneratedAt.IsZero() {
		opts.GeneratedAt = time.Now().UTC()
	}

	report := buildReport(c, opts)
	view, err := buildView(report, c)
	if err != nil {
		return fmt.Errorf("render: build view: %w", err)
	}

	tmpl, err := template.ParseFS(embeddedTemplates, "templates/*.tmpl")
	if err != nil {
		return fmt.Errorf("render: parse templates: %w", err)
	}

	// Render into a buffer first so a failure doesn't leave the caller's
	// writer partially written.
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "report.html.tmpl", view); err != nil {
		return fmt.Errorf("render: execute template: %w", err)
	}
	if _, err := io.Copy(w, &buf); err != nil {
		return fmt.Errorf("render: write output: %w", err)
	}
	return nil
}

// --- Internal view construction ---------------------------------------------

// buildReport converts a Collection into the typed Report envelope.
func buildReport(c *model.Collection, opts RenderOptions) *model.Report {
	rpt := &model.Report{
		Title:       collectionTitle(c),
		Version:     opts.Version,
		GitCommit:   opts.GitCommit,
		BuiltAt:     opts.BuiltAt,
		GeneratedAt: opts.GeneratedAt.UTC(),
		Collection:  c,
		ReportID:    CanonicalReportID(c),
	}
	rpt.OSSection = buildOSSection(c)
	rpt.VariablesSection = buildVariablesSection(c)
	rpt.DBSection = buildDBSection(c)
	rpt.Navigation = buildNavigation(rpt)
	return rpt
}

func collectionTitle(c *model.Collection) string {
	if c.Hostname != "" {
		return c.Hostname
	}
	if len(c.Snapshots) > 0 {
		return c.Snapshots[0].Prefix
	}
	return "unknown-collection"
}

// buildOSSection pulls OS-related parsed payloads out of the
// Collection and merges them across Snapshots onto a single time axis
// per FR-018. SnapshotBoundaries on each returned *Data struct records
// the sample indexes at which a new Snapshot's first sample sits; the
// chart layer draws a vertical boundary marker at each corresponding
// timestamp (FR-030 renderer requirement).
//
// A subview is "missing" only if EVERY Snapshot lacked that collector
// (or every present file failed to parse).
func buildOSSection(c *model.Collection) *model.OSSection {
	sec := &model.OSSection{}
	var ios []*model.IostatData
	var tops []*model.TopData
	var vms []*model.VmstatData
	for _, snap := range c.Snapshots {
		if sf, ok := snap.SourceFiles[model.SuffixIostat]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.IostatData); ok {
				ios = append(ios, v)
			}
		}
		if sf, ok := snap.SourceFiles[model.SuffixTop]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.TopData); ok {
				tops = append(tops, v)
			}
		}
		if sf, ok := snap.SourceFiles[model.SuffixVmstat]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.VmstatData); ok {
				vms = append(vms, v)
			}
		}
	}
	sec.Iostat = concatIostat(ios)
	sec.Top = concatTop(tops)
	sec.Vmstat = concatVmstat(vms)
	if sec.Iostat == nil {
		sec.Missing = append(sec.Missing, "-iostat")
	}
	if sec.Top == nil {
		sec.Missing = append(sec.Missing, "-top")
	}
	if sec.Vmstat == nil {
		sec.Missing = append(sec.Missing, "-vmstat")
	}
	sort.Strings(sec.Missing)
	return sec
}

// concatIostat merges multiple per-Snapshot *IostatData into one.
//
// All merged DeviceSeries share a **single, unified timestamp axis**:
// for each Snapshot, the per-Snapshot axis is taken from that
// Snapshot's first DeviceSeries (pt-stalk emits all devices at the
// same intervals within one -iostat file, so any device's timestamps
// represent the whole Snapshot). Across the global union of device
// names, a device absent from a given Snapshot gets NaN-padded
// samples at each axis timestamp so every device ends up with the
// same total sample count; uPlot requires matched-length series on
// a shared x-axis, and the chart payload treats
// `Devices[0].Utilization.Samples` as authoritative.
//
// SnapshotBoundaries indexes into that shared axis — equivalently,
// into any merged DeviceSeries' samples, since all of them are
// aligned to the same length.
func concatIostat(ins []*model.IostatData) *model.IostatData {
	nonNil := ins[:0]
	for _, d := range ins {
		if d != nil && len(d.Devices) > 0 && len(d.Devices[0].Utilization.Samples) > 0 {
			nonNil = append(nonNil, d)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	// Union of device names, deterministically sorted.
	nameSet := map[string]bool{}
	for _, d := range nonNil {
		for _, dev := range d.Devices {
			nameSet[dev.Device] = true
		}
	}
	names := make([]string, 0, len(nameSet))
	for n := range nameSet {
		names = append(names, n)
	}
	sort.Strings(names)

	devices := make(map[string]*model.DeviceSeries, len(names))
	for _, n := range names {
		devices[n] = &model.DeviceSeries{
			Device:       n,
			Utilization:  model.MetricSeries{Metric: "util_percent", Unit: "%", Subject: n},
			AvgQueueSize: model.MetricSeries{Metric: "avgqu_sz", Unit: "count", Subject: n},
		}
	}

	boundaries := make([]int, 0, len(nonNil))
	cumulative := 0
	for _, d := range nonNil {
		boundaries = append(boundaries, cumulative)
		// Per-Snapshot axis: take the first device's timestamps in
		// that Snapshot (all devices in one -iostat file share the
		// same sample grid).
		axis := d.Devices[0].Utilization.Samples
		// Quick-lookup per-Snapshot device map so we can pad the
		// absent-devices path without a linear scan per timestamp.
		snapDevices := make(map[string]*model.DeviceSeries, len(d.Devices))
		for i := range d.Devices {
			snapDevices[d.Devices[i].Device] = &d.Devices[i]
		}
		for _, n := range names {
			m := devices[n]
			src, present := snapDevices[n]
			if present {
				m.Utilization.Samples = append(m.Utilization.Samples, src.Utilization.Samples...)
				m.AvgQueueSize.Samples = append(m.AvgQueueSize.Samples, src.AvgQueueSize.Samples...)
				continue
			}
			// Absent in this Snapshot: pad with NaN samples whose
			// timestamps match the Snapshot's axis. The chart
			// payload will surface these as JSON null so uPlot
			// draws a visible gap at those positions.
			for _, a := range axis {
				m.Utilization.Samples = append(m.Utilization.Samples, model.Sample{
					Timestamp:    a.Timestamp,
					Measurements: map[string]float64{"util_percent": math.NaN()},
				})
				m.AvgQueueSize.Samples = append(m.AvgQueueSize.Samples, model.Sample{
					Timestamp:    a.Timestamp,
					Measurements: map[string]float64{"avgqu_sz": math.NaN()},
				})
			}
		}
		cumulative += len(axis)
	}

	out := &model.IostatData{SnapshotBoundaries: boundaries}
	for _, n := range names {
		out.Devices = append(out.Devices, *devices[n])
	}
	return out
}

// concatTop merges multiple per-Snapshot *TopData by unioning
// ProcessSamples across inputs, then recomputing Top3ByAverage over
// the merged stream. SnapshotBoundaries indexes into the first
// (post-recompute) Top3ByAverage series' CPU.Samples slice — the
// primary timestamp axis the renderer uses.
func concatTop(ins []*model.TopData) *model.TopData {
	nonNil := ins[:0]
	for _, d := range ins {
		if d != nil && (len(d.ProcessSamples) > 0 || len(d.Top3ByAverage) > 0) {
			nonNil = append(nonNil, d)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}

	// Concatenate all process samples.
	var all []model.ProcessSample
	for _, d := range nonNil {
		all = append(all, d.ProcessSamples...)
	}

	// Recompute top-3 by average CPUPercent over the merged window. A
	// process absent from a sample contributes zero, matching T049's
	// FR-010 aggregate semantics.
	seriesByPID := map[int]*model.ProcessSeries{}
	samplesAtTS := map[int64]map[int]float64{} // ts-unix -> pid -> cpu
	timestamps := map[int64]time.Time{}
	for _, s := range all {
		ts := s.Timestamp.Unix()
		if _, ok := samplesAtTS[ts]; !ok {
			samplesAtTS[ts] = map[int]float64{}
			timestamps[ts] = s.Timestamp
		}
		samplesAtTS[ts][s.PID] = s.CPUPercent
		if _, ok := seriesByPID[s.PID]; !ok {
			seriesByPID[s.PID] = &model.ProcessSeries{
				PID:     s.PID,
				Command: s.Command,
				CPU:     model.MetricSeries{Metric: "cpu_percent", Unit: "%", Subject: fmt.Sprintf("pid=%d", s.PID)},
			}
		}
	}
	// Ordered timestamps.
	tsList := make([]int64, 0, len(timestamps))
	for ts := range timestamps {
		tsList = append(tsList, ts)
	}
	sort.Slice(tsList, func(i, j int) bool { return tsList[i] < tsList[j] })

	// For each PID, emit one Sample per timestamp (zero if absent).
	pids := make([]int, 0, len(seriesByPID))
	for pid := range seriesByPID {
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	for _, pid := range pids {
		series := seriesByPID[pid]
		series.CPU.Samples = make([]model.Sample, 0, len(tsList))
		for _, ts := range tsList {
			series.CPU.Samples = append(series.CPU.Samples, model.Sample{
				Timestamp:    timestamps[ts],
				Measurements: map[string]float64{"cpu_percent": samplesAtTS[ts][pid]},
			})
		}
	}

	// Rank by average CPU, tie-break by higher PID first (matches T049 spec).
	pidsRanked := make([]int, len(pids))
	copy(pidsRanked, pids)
	sort.SliceStable(pidsRanked, func(i, j int) bool {
		ai := averageCPU(seriesByPID[pidsRanked[i]])
		aj := averageCPU(seriesByPID[pidsRanked[j]])
		if ai != aj {
			return ai > aj
		}
		return pidsRanked[i] > pidsRanked[j]
	})

	out := &model.TopData{ProcessSamples: all}
	limit := 3
	if len(pidsRanked) < limit {
		limit = len(pidsRanked)
	}
	for i := 0; i < limit; i++ {
		out.Top3ByAverage = append(out.Top3ByAverage, *seriesByPID[pidsRanked[i]])
	}
	// Always surface mysqld, even when it isn't in the global top-3.
	// Diagnosing a MySQL incident without seeing the mysqld curve is
	// useless — we'd rather render 4 series than hide the one the
	// reader actually came for. The mysqld series is appended after
	// the top-3 so ranking-by-average still reads left to right.
	mysqldAlreadyIn := false
	for i := 0; i < limit; i++ {
		if isMysqldCommand(seriesByPID[pidsRanked[i]].Command) {
			mysqldAlreadyIn = true
			break
		}
	}
	if !mysqldAlreadyIn {
		for _, pid := range pidsRanked[limit:] {
			if isMysqldCommand(seriesByPID[pid].Command) {
				out.Top3ByAverage = append(out.Top3ByAverage, *seriesByPID[pid])
				break
			}
		}
	}

	// SnapshotBoundaries: sample indexes into tsList where each input's
	// first sample sits.
	boundaries := make([]int, 0, len(nonNil))
	cumulative := 0
	for _, d := range nonNil {
		boundaries = append(boundaries, cumulative)
		seen := map[int64]bool{}
		for _, s := range d.ProcessSamples {
			ts := s.Timestamp.Unix()
			if !seen[ts] {
				seen[ts] = true
				cumulative++
			}
		}
	}
	out.SnapshotBoundaries = boundaries
	return out
}

func averageCPU(ps *model.ProcessSeries) float64 {
	if len(ps.CPU.Samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range ps.CPU.Samples {
		sum += s.Measurements["cpu_percent"]
	}
	return sum / float64(len(ps.CPU.Samples))
}

// concatVmstat merges multiple per-Snapshot *VmstatData by
// concatenating each canonically-ordered Series' Samples in input
// order. SnapshotBoundaries indexes into Series[0].Samples — the
// primary timestamp axis the renderer uses.
func concatVmstat(ins []*model.VmstatData) *model.VmstatData {
	nonNil := ins[:0]
	for _, d := range ins {
		if d != nil && len(d.Series) > 0 {
			nonNil = append(nonNil, d)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	// All inputs SHOULD share the same declared Series order; preserve
	// the first input's order and align by Metric name so a gap in a
	// later input doesn't shift the axis.
	metrics := make([]string, 0, len(nonNil[0].Series))
	for _, s := range nonNil[0].Series {
		metrics = append(metrics, s.Metric)
	}
	seriesByMetric := make(map[string]*model.MetricSeries, len(metrics))
	for _, m := range metrics {
		seriesByMetric[m] = &model.MetricSeries{Metric: m, Unit: "mixed"}
	}

	boundaries := make([]int, 0, len(nonNil))
	cumulative := 0
	for _, d := range nonNil {
		boundaries = append(boundaries, cumulative)
		primaryAdded := 0
		for _, s := range d.Series {
			m := seriesByMetric[s.Metric]
			if m == nil {
				continue // unknown metric in a later input — skip
			}
			m.Samples = append(m.Samples, s.Samples...)
			if s.Metric == metrics[0] {
				primaryAdded = len(s.Samples)
			}
			// Pick up the Unit from the first non-empty source.
			if m.Unit == "mixed" && s.Unit != "" {
				m.Unit = s.Unit
			}
		}
		cumulative += primaryAdded
	}

	out := &model.VmstatData{SnapshotBoundaries: boundaries}
	for _, m := range metrics {
		out.Series = append(out.Series, *seriesByMetric[m])
	}
	return out
}

func buildVariablesSection(c *model.Collection) *model.VariablesSection {
	sec := &model.VariablesSection{}
	for _, snap := range c.Snapshots {
		sv := model.SnapshotVariables{
			SnapshotPrefix: snap.Prefix,
			Timestamp:      snap.Timestamp,
		}
		if sf, ok := snap.SourceFiles[model.SuffixVariables]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.VariablesData); ok {
				sv.Data = v
			}
		}
		sec.PerSnapshot = append(sec.PerSnapshot, sv)
	}
	return sec
}

func buildDBSection(c *model.Collection) *model.DBSection {
	sec := &model.DBSection{}
	var mas []*model.MysqladminData
	var pls []*model.ProcesslistData
	for _, snap := range c.Snapshots {
		si := model.SnapshotInnoDB{
			SnapshotPrefix: snap.Prefix,
			Timestamp:      snap.Timestamp,
		}
		if sf, ok := snap.SourceFiles[model.SuffixInnodbStatus]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.InnodbStatusData); ok {
				si.Data = v
			}
		}
		sec.InnoDBPerSnapshot = append(sec.InnoDBPerSnapshot, si)

		if sf, ok := snap.SourceFiles[model.SuffixMysqladmin]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.MysqladminData); ok {
				mas = append(mas, v)
			}
		}
		if sf, ok := snap.SourceFiles[model.SuffixProcesslist]; ok && sf.Parsed != nil {
			if v, ok := sf.Parsed.(*model.ProcesslistData); ok {
				pls = append(pls, v)
			}
		}
	}

	sec.Mysqladmin = concatMysqladmin(mas)
	sec.Processlist = concatProcesslist(pls)

	if allInnoDBNil(sec.InnoDBPerSnapshot) {
		sec.Missing = append(sec.Missing, "-innodbstatus1")
	}
	if sec.Mysqladmin == nil {
		sec.Missing = append(sec.Missing, "-mysqladmin")
	}
	if sec.Processlist == nil {
		sec.Missing = append(sec.Missing, "-processlist")
	}
	sort.Strings(sec.Missing)
	return sec
}

// concatMysqladmin merges multiple per-Snapshot *MysqladminData onto
// a single time axis. Counters reset at each Snapshot boundary per
// FR-030: the first post-boundary sample's delta is NaN. Gauges just
// concat their raw values.
//
// This function does NOT emit model.Diagnostic records itself; any
// Info-severity diagnostic about the boundary is already attached to
// the relevant SourceFile by the parser at Discover time (the parser
// exposes `ResetForNewSnapshot` for callers that feed a concatenated
// stream; single-file parses record their own boundary info), so
// surfacing it twice would violate FR-038's no-duplication rule.
//
// SnapshotBoundaries indexes into the merged Timestamps slice — the
// primary timestamp axis the renderer uses.
func concatMysqladmin(ins []*model.MysqladminData) *model.MysqladminData {
	nonNil := ins[:0]
	for _, d := range ins {
		if d != nil && d.SampleCount > 0 {
			nonNil = append(nonNil, d)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	if len(nonNil) == 1 {
		// Trust the parser's own SnapshotBoundaries (which contains [0]
		// for a single file, or more if the parser saw concatenated
		// input — it already honours FR-030 in that path).
		return nonNil[0]
	}

	// Union of variable names, sorted.
	nameSet := map[string]bool{}
	for _, d := range nonNil {
		for _, n := range d.VariableNames {
			nameSet[n] = true
		}
	}
	names := make([]string, 0, len(nameSet))
	for n := range nameSet {
		names = append(names, n)
	}
	sort.Strings(names)

	// IsCounter: a variable is a counter iff at least one input flags
	// it as such (pragmatic; conflicts are extremely rare and a
	// per-version allowlist drives the flag deterministically).
	isCounter := make(map[string]bool, len(names))
	for _, n := range names {
		for _, d := range nonNil {
			if d.IsCounter[n] {
				isCounter[n] = true
				break
			}
		}
	}

	// Concatenate timestamps and deltas. Record boundaries as the
	// cumulative sample index where each input starts.
	var (
		timestamps []time.Time
		boundaries = make([]int, 0, len(nonNil))
		cumulative int
		deltas     = make(map[string][]float64, len(names))
	)
	for _, n := range names {
		deltas[n] = make([]float64, 0)
	}

	for inputIdx, d := range nonNil {
		boundaries = append(boundaries, cumulative)
		timestamps = append(timestamps, d.Timestamps...)
		for _, n := range names {
			src, present := d.Deltas[n]
			if !present {
				// Variable absent from this input: fill with NaN so the
				// chart shows a discontinuity rather than a misleading
				// straight line through 0.
				for i := 0; i < d.SampleCount; i++ {
					deltas[n] = append(deltas[n], math.NaN())
				}
				continue
			}
			if inputIdx > 0 && isCounter[n] && len(src) > 0 {
				// FR-030: counters reset at the boundary. The first
				// post-boundary slot is NaN — cross-snapshot deltas are
				// meaningless.
				deltas[n] = append(deltas[n], math.NaN())
				deltas[n] = append(deltas[n], src[1:]...)
			} else {
				deltas[n] = append(deltas[n], src...)
			}
		}
		cumulative += d.SampleCount
	}

	return &model.MysqladminData{
		VariableNames:      names,
		SampleCount:        cumulative,
		Timestamps:         timestamps,
		Deltas:             deltas,
		IsCounter:          isCounter,
		SnapshotBoundaries: boundaries,
	}
}

// concatProcesslist merges multiple per-Snapshot *ProcesslistData by
// concatenating ThreadStateSamples in input order and unioning the
// States slice. SnapshotBoundaries indexes into the merged
// ThreadStateSamples slice.
func concatProcesslist(ins []*model.ProcesslistData) *model.ProcesslistData {
	nonNil := ins[:0]
	for _, d := range ins {
		if d != nil && len(d.ThreadStateSamples) > 0 {
			nonNil = append(nonNil, d)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}

	// Union every dimension's label set across Snapshots, preserving
	// "Other last" semantics from the parser.
	unionLabels := func(pick func(d *model.ProcesslistData) []string) []string {
		set := map[string]bool{}
		hasOther := false
		for _, d := range nonNil {
			for _, s := range pick(d) {
				if s == "Other" {
					hasOther = true
				} else {
					set[s] = true
				}
			}
		}
		out := make([]string, 0, len(set)+1)
		for s := range set {
			out = append(out, s)
		}
		sort.Strings(out)
		if hasOther {
			out = append(out, "Other")
		}
		return out
	}

	out := &model.ProcesslistData{
		States:   unionLabels(func(d *model.ProcesslistData) []string { return d.States }),
		Users:    unionLabels(func(d *model.ProcesslistData) []string { return d.Users }),
		Hosts:    unionLabels(func(d *model.ProcesslistData) []string { return d.Hosts }),
		Commands: unionLabels(func(d *model.ProcesslistData) []string { return d.Commands }),
		Dbs:      unionLabels(func(d *model.ProcesslistData) []string { return d.Dbs }),
	}
	boundaries := make([]int, 0, len(nonNil))
	cumulative := 0
	for _, d := range nonNil {
		boundaries = append(boundaries, cumulative)
		out.ThreadStateSamples = append(out.ThreadStateSamples, d.ThreadStateSamples...)
		cumulative += len(d.ThreadStateSamples)
	}
	out.SnapshotBoundaries = boundaries
	return out
}

func allInnoDBNil(xs []model.SnapshotInnoDB) bool {
	for _, x := range xs {
		if x.Data != nil {
			return false
		}
	}
	return true
}

// buildNavigation produces a flat, deterministic list of navigation
// entries. Level-1 and Level-2 entries interleave in the order the
// templates render them.
func buildNavigation(r *model.Report) []model.NavEntry {
	var nav []model.NavEntry

	// OS section.
	nav = append(nav, model.NavEntry{ID: "sec-os", Title: "OS Usage", Level: 1})
	nav = append(nav, model.NavEntry{ID: "sub-os-iostat", Title: "Disk utilization", Level: 2, ParentID: "sec-os"})
	nav = append(nav, model.NavEntry{ID: "sub-os-top", Title: "Top CPU processes", Level: 2, ParentID: "sec-os"})
	nav = append(nav, model.NavEntry{ID: "sub-os-vmstat", Title: "vmstat saturation", Level: 2, ParentID: "sec-os"})

	// Variables section (one Level-2 per *unique* snapshot — adjacent
	// snapshots with identical variables are collapsed, matching the
	// panels rendered in the Variables body). nil-Data snapshots
	// always get their own entry so partial captures stay visible.
	nav = append(nav, model.NavEntry{ID: "sec-variables", Title: "Variables", Level: 1})
	if r.VariablesSection != nil {
		var lastSig string
		sigValid := false
		for i, sv := range r.VariablesSection.PerSnapshot {
			if sv.Data == nil {
				nav = append(nav, model.NavEntry{
					ID:       variablesSnapshotID(i),
					Title:    sv.SnapshotPrefix,
					Level:    2,
					ParentID: "sec-variables",
				})
				sigValid = false
				continue
			}
			sig := snapshotSignature(sv.Data.Entries)
			if sigValid && sig == lastSig {
				continue
			}
			nav = append(nav, model.NavEntry{
				ID:       variablesSnapshotID(i),
				Title:    sv.SnapshotPrefix,
				Level:    2,
				ParentID: "sec-variables",
			})
			lastSig = sig
			sigValid = true
		}
	}

	// DB section.
	nav = append(nav, model.NavEntry{ID: "sec-db", Title: "Database Usage", Level: 1})
	nav = append(nav, model.NavEntry{ID: "sub-db-innodb", Title: "InnoDB status", Level: 2, ParentID: "sec-db"})
	nav = append(nav, model.NavEntry{ID: "sub-db-mysqladmin", Title: "Counter deltas", Level: 2, ParentID: "sec-db"})
	nav = append(nav, model.NavEntry{ID: "sub-db-processlist", Title: "Thread states", Level: 2, ParentID: "sec-db"})

	// Advisor section — rule-based findings.
	nav = append(nav, model.NavEntry{ID: "sec-advisor", Title: "Advisor", Level: 1})

	return nav
}

func variablesSnapshotID(idx int) string {
	return fmt.Sprintf("sub-var-%d", idx+1)
}

// groupNavigation flattens Report.Navigation into one navGroupView per
// Level-1 entry with its Level-2 children nested. Order is preserved.
func groupNavigation(nav []model.NavEntry) []navGroupView {
	var groups []navGroupView
	idxByID := map[string]int{}
	for _, e := range nav {
		if e.Level == 1 {
			groups = append(groups, navGroupView{ID: e.ID, Title: e.Title})
			idxByID[e.ID] = len(groups) - 1
		} else if e.Level == 2 {
			parentIdx, ok := idxByID[e.ParentID]
			if !ok {
				continue
			}
			groups[parentIdx].Children = append(groups[parentIdx].Children, navChildView{ID: e.ID, Title: e.Title})
		}
	}
	return groups
}

// --- Template view (what the .tmpl files actually see) ----------------------

// reportView is the value passed to the top-level template. It
// flattens the Report into fields the template can consume without
// complex logic.
type reportView struct {
	Title              string
	Hostname           string
	Version            string
	GitCommit          string
	GeneratedAtDisplay string
	SnapshotCount      int
	Navigation         []model.NavEntry
	NavGroups          []navGroupView

	EmbeddedCSS     template.CSS
	EmbeddedChartJS template.JS
	EmbeddedAppJS   template.JS
	DataPayload     template.JS // JSON, emitted inside <script type="application/json">
	LogoDataURI     template.URL

	AdvisorBadge   string
	VariablesBadge string

	// Advisor section payload.
	Findings      []findingView
	AdvisorCounts findingCountsView
	HasFindings   bool

	// OS section payload
	HasIostat     bool
	HasTop        bool
	HasVmstat     bool
	IostatSummary *iostatSummaryView
	TopSummary    *topSummaryView
	VmstatSummary *vmstatSummaryView

	// Variables section payload
	HasVariables      bool
	VariableSnapshots []variableSnapshotView

	// DB section payload
	HasInnoDB     bool
	InnoDBMetrics []innoDBMetricView
	HasMysqladmin       bool
	MysqladminVariables []string
	MysqladminCount     int
	MysqladminSelectID  string
	HasProcesslist bool
}

type variableSnapshotView struct {
	DetailsID     string
	Title         string
	Badge         string
	// RangeNote is the dedup subtitle shown inside the snapshot body
	// when this kept snapshot represents a run of identical snapshots
	// (e.g. "Identical values seen in snapshots #2–#10"). Empty for
	// unique snapshots.
	RangeNote     string
	Count         int
	ModifiedCount int
	// ChangedCount is the number of rows flagged .Changed — useful
	// for showing a "N changed" hint on panels that aren't the first.
	ChangedCount  int
	Entries       []variableRowView
}

// volatileVariables are SHOW VARIABLES entries expected to drift
// between snapshots on a busy server even when the operator has not
// changed anything (e.g. gtid_executed advances on every commit).
// Excluding them from the dedup signature keeps the "collapse
// identical snapshots" behaviour useful on busy replicas. Matching
// is case-insensitive.
var volatileVariables = map[string]struct{}{
	"gtid_executed": {},
	"gtid_purged":   {},
}

type variableRowView struct {
	Name    string
	Value   string
	Status  string // "default" | "modified" | "unknown"
	// Changed marks this variable as different from the previous kept
	// snapshot. Volatile variables (gtid_executed/gtid_purged) are
	// intentionally never marked so the highlight doesn't light up
	// every snapshot on a busy replica.
	Changed bool
}

// innoDBMetricView holds one row in the aggregated InnoDB status
// callout grid: a single "worst" (max) value for the whole capture
// window with a small min/avg/max line below. One view per metric
// replaces the previous per-snapshot card repetition so the reader
// lands on the peak pain point immediately.
type innoDBMetricView struct {
	Label string // "Semaphores" / "Pending I/O" / "AHI" / "History list"
	Hint  string // tiny descriptor shown under Worst (optional)
	Worst string
	Min   string
	Avg   string
	Max   string
	// Semaphores-only: per-site wait breakdowns. When populated,
	// db.html.tmpl renders a collapsible "contention breakdown"
	// <details> under the card with two toggleable views:
	//   - Peak: breakdown from the single snapshot whose count
	//     equals Max ("at peak · snapshot <prefix> · N waits").
	//   - Total: sum of waits over ALL snapshots, by the same
	//     (file, line, mutex) key.
	// Empty when no snapshot ever saw >0 waits.
	BreakdownPeak         []semaphoreSiteRow
	BreakdownPeakSnapshot string
	BreakdownPeakTotal    int
	BreakdownTotal        []semaphoreSiteRow
	BreakdownTotalTotal   int

	// AHI-only: formula breakdown exposed when the card represents
	// the Adaptive Hash Index hit ratio. The text fields are all
	// pre-rendered so the template can concatenate them into a
	// readable "hit ratio = hash / (hash + non-hash) = X / (X + Y)
	// = Z%" line.
	AHIFormula         bool
	AHIFormulaText     string // "hash / (hash + non-hash)"
	AHIWorstSnapshot   string
	AHIWorstHash       string // "3901.21"
	AHIWorstNonHash    string // "8669.42"
	AHIWorstRatio      string // "31.0"
	AHIHashTableSize   string // "34,679"
	AHINoActivity      bool   // true when every snapshot had hash+non-hash == 0
}

// semaphoreSiteRow is one row in the contention-breakdown sub-panel.
// Percent is computed against the view's total (peak-snapshot total
// or window total), so each view's percents sum to 100.
type semaphoreSiteRow struct {
	File      string
	Line      int
	MutexName string
	Count     int
	Percent   string // formatted, e.g. "94.9"
}

// navGroupView is the template-facing shape of one Level-1 nav entry
// plus its Level-2 children. Built from the flat Report.Navigation.
type navGroupView struct {
	ID       string
	Title    string
	Children []navChildView
}

type navChildView struct {
	ID    string
	Title string
}

// findingView is the template-facing shape of a findings.Finding.
type findingView struct {
	ID              string
	Subsystem       string
	Title           string
	SeverityClass   string // "crit" | "warn" | "info" | "ok"
	SeverityLabel   string // "Critical" | "Warning" | "Info" | "OK"
	OpenByDefault   bool   // crit findings start expanded
	Summary         string
	Explanation     string
	FormulaText     string
	FormulaComputed string
	Metrics         []findingMetricView
	Recommendations []string
}

type findingMetricView struct {
	Name  string
	Value string
	Unit  string
	Note  string
}

type findingCountsView struct {
	Crit int
	Warn int
	Info int
	OK   int
	Any  bool
}

type iostatSummaryView struct {
	PeakUtil        string
	PeakDevice      string
	PeakAqusz       string
	PeakAquszDevice string
	DeviceCount     int
	SampleCount     int
}

type topSummaryView struct {
	First       string
	FirstAvg    string
	Second      string
	SecondAvg   string
	Third       string
	ThirdAvg    string
	// MysqldExtra is populated only when mysqld is NOT already one of
	// the top-3 by average — rendering it as a 4th chip (and a 4th
	// series in the chart) so the MySQL process is always visible.
	MysqldExtra    string
	MysqldExtraAvg string
	SampleCount    int
}

type vmstatSummaryView struct {
	PeakRunqueue string
	PeakBlocked  string
	PeakIowait   string
	SampleCount  int
}

// buildView flattens the Report into the template-friendly shape.
func buildView(r *model.Report, c *model.Collection) (*reportView, error) {
	v := &reportView{
		Title:              r.Title,
		Hostname:           c.Hostname,
		Version:            r.Version,
		GitCommit:          r.GitCommit,
		GeneratedAtDisplay: FormatTimestamp(r.GeneratedAt),
		SnapshotCount:      len(c.Snapshots),
		Navigation:         r.Navigation,
		NavGroups:          groupNavigation(r.Navigation),
		EmbeddedCSS:        template.CSS(embeddedChartCSS + "\n" + embeddedAppCSS),
		EmbeddedChartJS:    template.JS(embeddedChartJS),
		EmbeddedAppJS:      template.JS(embeddedAppJS),
		LogoDataURI:        template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(embeddedLogoPNG)),
		MysqladminSelectID: "mysqladmin-select",
	}

	if r.OSSection != nil {
		v.HasIostat = r.OSSection.Iostat != nil
		v.HasTop = r.OSSection.Top != nil
		v.HasVmstat = r.OSSection.Vmstat != nil
		if v.HasIostat {
			v.IostatSummary = summariseIostat(r.OSSection.Iostat)
		}
		if v.HasTop {
			v.TopSummary = summariseTop(r.OSSection.Top)
		}
		if v.HasVmstat {
			v.VmstatSummary = summariseVmstat(r.OSSection.Vmstat)
		}
	}
	totalSnaps := 0
	if r.VariablesSection != nil {
		totalSnaps = len(r.VariablesSection.PerSnapshot)
		defaults := loadMySQLDefaults()
		haveAny := false
		// Dedup adjacent identical snapshots. For each captured
		// snapshot compute a signature over the (name, value) pairs —
		// skipping volatile variables that drift without meaningful
		// change — and compare against the last KEPT snapshot. If
		// identical, extend that snapshot's range; otherwise emit a
		// new view. Nil-Data snapshots emit their own entry so
		// partial captures remain visible.
		lastKept := -1
		var lastSig string
		var lastKeptMap map[string]string // name → value from previous kept snapshot
		for i, sv := range r.VariablesSection.PerSnapshot {
			if sv.Data == nil {
				v.VariableSnapshots = append(v.VariableSnapshots, variableSnapshotView{
					DetailsID: variablesSnapshotID(i),
					Title:     fmt.Sprintf("Snapshot %s", sv.SnapshotPrefix),
					Badge:     fmt.Sprintf("snap #%d", i+1),
				})
				lastKept = -1
				lastSig = ""
				lastKeptMap = nil
				continue
			}
			sig := snapshotSignature(sv.Data.Entries)
			if lastKept >= 0 && sig == lastSig {
				// Identical to the last kept snapshot — extend its range.
				extendRange(&v.VariableSnapshots[lastKept], i+1)
				continue
			}
			vv := variableSnapshotView{
				DetailsID: variablesSnapshotID(i),
				Title:     fmt.Sprintf("Snapshot %s", sv.SnapshotPrefix),
				Badge:     fmt.Sprintf("snap #%d", i+1),
				Count:     len(sv.Data.Entries),
			}
			vv.Entries = make([]variableRowView, 0, len(sv.Data.Entries))
			for _, e := range sv.Data.Entries {
				st := classifyVariable(defaults, e.Name, e.Value)
				if st == "modified" {
					vv.ModifiedCount++
				}
				// Flag the row as Changed when (1) there IS a previous
				// kept snapshot, (2) this variable is NOT in the
				// volatile ignorelist, and (3) its value differs from
				// (or was absent in) the previous snapshot. The first
				// kept panel never highlights — nothing to compare to.
				changed := false
				if lastKeptMap != nil {
					if _, vol := volatileVariables[strings.ToLower(e.Name)]; !vol {
						prev, ok := lastKeptMap[e.Name]
						if !ok || prev != e.Value {
							changed = true
						}
					}
				}
				if changed {
					vv.ChangedCount++
				}
				vv.Entries = append(vv.Entries, variableRowView{
					Name:    e.Name,
					Value:   e.Value,
					Status:  st,
					Changed: changed,
				})
			}
			haveAny = true
			v.VariableSnapshots = append(v.VariableSnapshots, vv)
			lastKept = len(v.VariableSnapshots) - 1
			lastSig = sig
			// Snapshot the name→value map for the next kept comparison.
			lastKeptMap = make(map[string]string, len(sv.Data.Entries))
			for _, e := range sv.Data.Entries {
				lastKeptMap[e.Name] = e.Value
			}
			// Seed the range tracker with the current snapshot number
			// so extendRange can grow it later.
			v.VariableSnapshots[lastKept].RangeNote = formatRangeNote(i+1, i+1)
		}
		v.HasVariables = haveAny
		// RangeNote is only informative when the kept snapshot
		// represents a range; clear the single-snapshot notes.
		for idx := range v.VariableSnapshots {
			if rangeIsSingle(v.VariableSnapshots[idx].RangeNote) {
				v.VariableSnapshots[idx].RangeNote = ""
			}
		}
	}
	if v.HasVariables {
		unique := len(v.VariableSnapshots)
		if unique < totalSnaps {
			v.VariablesBadge = fmt.Sprintf("%d unique of %d", unique, totalSnaps)
		} else {
			v.VariablesBadge = fmt.Sprintf("%d snapshots", unique)
		}
	} else {
		v.VariablesBadge = "missing"
	}

	if r.DBSection != nil {
		v.InnoDBMetrics = aggregateInnoDBMetrics(r.DBSection.InnoDBPerSnapshot)
		v.HasInnoDB = len(v.InnoDBMetrics) > 0
		v.HasMysqladmin = r.DBSection.Mysqladmin != nil
		v.HasProcesslist = r.DBSection.Processlist != nil
		if v.HasMysqladmin {
			v.MysqladminVariables = append(v.MysqladminVariables, r.DBSection.Mysqladmin.VariableNames...)
			v.MysqladminCount = len(r.DBSection.Mysqladmin.VariableNames)
		}
	}
	// Advisor: rule-based findings derived from the captured data.
	fs := findings.Analyze(r)
	v.Findings = buildFindingViews(fs)
	v.AdvisorCounts = summariseFindings(fs)
	v.HasFindings = len(v.Findings) > 0
	v.AdvisorBadge = advisorBadge(v.AdvisorCounts)

	// Build the embedded data payload. Per-chart series are populated
	// by US2-US4 parser integration; for the MVP render skeleton we
	// emit an empty payload with the report ID — enough for app.js to
	// wire localStorage keys and for the charts to render "empty"
	// banners without throwing.
	// The embedded payload drives every interactive feature (charts,
	// filters, localStorage keys). A marshal error here would ship an
	// HTML that looks valid but is silently non-interactive, so fail
	// loudly instead of emitting a broken blob.
	payload, err := json.Marshal(map[string]any{
		"reportID": r.ReportID,
		"charts":   buildChartPayload(r),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal embedded data payload: %w", err)
	}
	v.DataPayload = template.JS(payload)

	return v, nil
}

func buildChartPayload(r *model.Report) map[string]any {
	payload := map[string]any{}
	if r.OSSection != nil {
		if r.OSSection.Iostat != nil {
			payload["iostat"] = iostatChartPayload(r.OSSection.Iostat)
		}
		if r.OSSection.Top != nil {
			payload["top"] = topChartPayload(r.OSSection.Top)
		}
		if r.OSSection.Vmstat != nil {
			payload["vmstat"] = vmstatChartPayload(r.OSSection.Vmstat)
		}
	}
	if r.DBSection != nil {
		if r.DBSection.Mysqladmin != nil {
			payload["mysqladmin"] = mysqladminChartPayload(r.DBSection.Mysqladmin)
		}
		if r.DBSection.Processlist != nil {
			payload["processlist"] = processlistChartPayload(r.DBSection.Processlist)
		}
	}
	return payload
}

func mysqladminChartPayload(d *model.MysqladminData) map[string]any {
	timestamps := make([]float64, len(d.Timestamps))
	for i, t := range d.Timestamps {
		timestamps[i] = float64(t.Unix())
	}
	// app.js reads { variables, timestamps, deltas, isCounter, snapshotBoundaries, defaultVisible, categories, categoryMap }.
	// Replace NaN values with null for JSON encoding (math.NaN is not a
	// valid JSON number).
	deltas := map[string]any{}
	for name, vs := range d.Deltas {
		cleaned := make([]any, len(vs))
		for i, v := range vs {
			if math.IsNaN(v) {
				cleaned[i] = nil
			} else {
				cleaned[i] = v
			}
		}
		deltas[name] = cleaned
	}
	// Default visible: pick up to 5 well-known high-signal counters if
	// they exist; otherwise first 5 counter variables alphabetically.
	defaults := pickDefaultCounters(d)

	// Resolve per-variable category memberships once at render time so
	// the JS doesn't have to evaluate matchers in the browser.
	cats := loadMysqladminCategories()
	categoryMap := map[string][]string{}
	categoryCounts := map[string]int{}
	for _, name := range d.VariableNames {
		hits := classifyMysqladminCategory(cats, name)
		if len(hits) > 0 {
			categoryMap[name] = hits
			for _, h := range hits {
				categoryCounts[h]++
			}
		}
	}
	// Emit category metadata in declared order plus a count of matched
	// variables for each.
	type catView struct {
		Key         string `json:"key"`
		Label       string `json:"label"`
		Description string `json:"description"`
		Count       int    `json:"count"`
	}
	categoryViews := make([]catView, 0, len(cats))
	for _, c := range cats {
		categoryViews = append(categoryViews, catView{
			Key:         c.Key,
			Label:       c.Label,
			Description: c.Description,
			Count:       categoryCounts[c.Key],
		})
	}
	return map[string]any{
		"variables":          d.VariableNames,
		"timestamps":         timestamps,
		"deltas":             deltas,
		"isCounter":          d.IsCounter,
		"snapshotBoundaries": d.SnapshotBoundaries,
		"defaultVisible":     defaults,
		"categories":         categoryViews,
		"categoryMap":        categoryMap,
	}
}

func pickDefaultCounters(d *model.MysqladminData) []string {
	preferred := []string{"Com_select", "Com_insert", "Com_update", "Questions", "Bytes_received", "Bytes_sent"}
	present := map[string]bool{}
	for _, n := range d.VariableNames {
		present[n] = true
	}
	var out []string
	for _, p := range preferred {
		if present[p] && d.IsCounter[p] {
			out = append(out, p)
			if len(out) >= 5 {
				return out
			}
		}
	}
	// Fallback: first 5 counters alphabetically.
	for _, n := range d.VariableNames {
		if len(out) >= 5 {
			break
		}
		if d.IsCounter[n] {
			already := false
			for _, x := range out {
				if x == n {
					already = true
					break
				}
			}
			if !already {
				out = append(out, n)
			}
		}
	}
	return out
}

func processlistChartPayload(d *model.ProcesslistData) map[string]any {
	n := len(d.ThreadStateSamples)
	if n == 0 {
		return nil
	}
	timestamps := make([]float64, n)
	for i, s := range d.ThreadStateSamples {
		timestamps[i] = float64(s.Timestamp.Unix())
	}

	buildSeries := func(labels []string, pick func(s model.ThreadStateSample, label string) int) []map[string]any {
		out := make([]map[string]any, 0, len(labels))
		for _, lbl := range labels {
			vals := make([]float64, n)
			for i, s := range d.ThreadStateSamples {
				vals[i] = float64(pick(s, lbl))
			}
			out = append(out, map[string]any{
				"label":  lbl,
				"values": vals,
			})
		}
		return out
	}

	dimensions := []map[string]any{
		{
			"key":    "state",
			"label":  "State",
			"unit":   "threads",
			"series": buildSeries(d.States, func(s model.ThreadStateSample, l string) int { return s.StateCounts[l] }),
		},
		{
			"key":    "user",
			"label":  "User",
			"unit":   "threads",
			"series": buildSeries(d.Users, func(s model.ThreadStateSample, l string) int { return s.UserCounts[l] }),
		},
		{
			"key":    "host",
			"label":  "Host",
			"unit":   "threads",
			"series": buildSeries(d.Hosts, func(s model.ThreadStateSample, l string) int { return s.HostCounts[l] }),
		},
		{
			"key":    "command",
			"label":  "Command",
			"unit":   "threads",
			"series": buildSeries(d.Commands, func(s model.ThreadStateSample, l string) int { return s.CommandCounts[l] }),
		},
		{
			"key":    "db",
			"label":  "db",
			"unit":   "threads",
			"series": buildSeries(d.Dbs, func(s model.ThreadStateSample, l string) int { return s.DbCounts[l] }),
		},
	}

	// Primary `series` stays pointed at the State dimension so the
	// existing renderTimeSeries fallback path remains functional.
	return map[string]any{
		"timestamps":         timestamps,
		"series":             dimensions[0]["series"],
		"dimensions":         dimensions,
		"snapshotBoundaries": d.SnapshotBoundaries,
	}
}

// chartTimestamps extracts a unified timestamp axis from an arbitrary
// primary series. Timestamps are rendered as unix seconds (float) for
// uPlot's time-scale.
func chartTimestamps(samples []model.Sample) []float64 {
	out := make([]float64, len(samples))
	for i, s := range samples {
		out[i] = float64(s.Timestamp.Unix())
	}
	return out
}

func iostatChartPayload(d *model.IostatData) map[string]any {
	if len(d.Devices) == 0 {
		return nil
	}
	// Use the first device's timestamps as the x-axis. After
	// concatIostat, every DeviceSeries is aligned to this same axis
	// (NaN-padded for Snapshots where a device was absent), so every
	// per-device `values` slice below has the same length as
	// `timestamps`.
	timestamps := chartTimestamps(d.Devices[0].Utilization.Samples)
	var series []map[string]any
	// Emit %util for every device first, then avgqu-sz. NaN values
	// (from the concat NaN-padding path) are serialised as JSON
	// `null` so uPlot draws a visible gap — json.Marshal rejects
	// raw NaN floats.
	for _, dev := range d.Devices {
		vals := make([]any, len(dev.Utilization.Samples))
		for i, s := range dev.Utilization.Samples {
			v := s.Measurements["util_percent"]
			if math.IsNaN(v) {
				vals[i] = nil
			} else {
				vals[i] = v
			}
		}
		series = append(series, map[string]any{
			"label":  dev.Device + " %util",
			"values": vals,
		})
	}
	for _, dev := range d.Devices {
		vals := make([]any, len(dev.AvgQueueSize.Samples))
		for i, s := range dev.AvgQueueSize.Samples {
			v := s.Measurements["avgqu_sz"]
			if math.IsNaN(v) {
				vals[i] = nil
			} else {
				vals[i] = v
			}
		}
		series = append(series, map[string]any{
			"label":  dev.Device + " aqu-sz",
			"values": vals,
		})
	}
	return map[string]any{
		"timestamps":         timestamps,
		"series":             series,
		"snapshotBoundaries": d.SnapshotBoundaries,
	}
}

func topChartPayload(d *model.TopData) map[string]any {
	if len(d.Top3ByAverage) == 0 {
		return nil
	}
	// Use the first top-3 process's timestamps as the x-axis.
	timestamps := chartTimestamps(d.Top3ByAverage[0].CPU.Samples)
	var series []map[string]any
	for _, ps := range d.Top3ByAverage {
		vals := make([]float64, len(ps.CPU.Samples))
		for i, s := range ps.CPU.Samples {
			vals[i] = s.Measurements["cpu_percent"]
		}
		series = append(series, map[string]any{
			"label":  fmt.Sprintf("%s (pid %d)", truncateCommand(ps.Command), ps.PID),
			"values": vals,
		})
	}
	return map[string]any{
		"timestamps":         timestamps,
		"series":             series,
		"snapshotBoundaries": d.SnapshotBoundaries,
	}
}

func vmstatChartPayload(d *model.VmstatData) map[string]any {
	if len(d.Series) == 0 {
		return nil
	}
	// Use the first series' timestamps.
	timestamps := chartTimestamps(d.Series[0].Samples)
	var series []map[string]any
	for _, s := range d.Series {
		vals := make([]float64, len(s.Samples))
		for i, sp := range s.Samples {
			vals[i] = sp.Measurements[s.Metric]
		}
		series = append(series, map[string]any{
			"label":  s.Metric,
			"values": vals,
		})
	}
	return map[string]any{
		"timestamps":         timestamps,
		"series":             series,
		"snapshotBoundaries": d.SnapshotBoundaries,
	}
}

func truncateCommand(s string) string {
	const max = 20
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func summariseIostat(d *model.IostatData) *iostatSummaryView {
	sum := &iostatSummaryView{DeviceCount: len(d.Devices)}
	if len(d.Devices) == 0 {
		return sum
	}
	var maxUtil, maxAqu float64
	var maxUtilDev, maxAquDev string
	sampleCount := 0
	for _, dev := range d.Devices {
		if len(dev.Utilization.Samples) > sampleCount {
			sampleCount = len(dev.Utilization.Samples)
		}
		for _, s := range dev.Utilization.Samples {
			if v := s.Measurements["util_percent"]; v > maxUtil {
				maxUtil = v
				maxUtilDev = dev.Device
			}
		}
		for _, s := range dev.AvgQueueSize.Samples {
			if v := s.Measurements["avgqu_sz"]; v > maxAqu {
				maxAqu = v
				maxAquDev = dev.Device
			}
		}
	}
	sum.PeakUtil = FormatFloat(maxUtil, 1) + "%"
	sum.PeakDevice = fallback(maxUtilDev, "–")
	sum.PeakAqusz = FormatFloat(maxAqu, 2)
	sum.PeakAquszDevice = fallback(maxAquDev, "–")
	sum.SampleCount = sampleCount
	return sum
}

func summariseTop(d *model.TopData) *topSummaryView {
	sum := &topSummaryView{}
	uniq := map[time.Time]struct{}{}
	for _, s := range d.ProcessSamples {
		uniq[s.Timestamp] = struct{}{}
	}
	sum.SampleCount = len(uniq)
	labels := make([]string, 0, 3)
	avgs := make([]string, 0, 3)
	for _, ps := range d.Top3ByAverage {
		var total float64
		for _, s := range ps.CPU.Samples {
			total += s.Measurements["cpu_percent"]
		}
		avg := 0.0
		if sum.SampleCount > 0 {
			avg = total / float64(sum.SampleCount)
		}
		labels = append(labels, truncateCommand(ps.Command)+" (pid "+fmt.Sprintf("%d", ps.PID)+")")
		avgs = append(avgs, FormatFloat(avg, 1))
	}
	if len(labels) > 0 {
		sum.First, sum.FirstAvg = labels[0], avgs[0]
	}
	if len(labels) > 1 {
		sum.Second, sum.SecondAvg = labels[1], avgs[1]
	}
	if len(labels) > 2 {
		sum.Third, sum.ThirdAvg = labels[2], avgs[2]
	}
	// concatTop appends mysqld as a 4th series when it isn't already
	// in the top-3, so surface it as a 4th chip to keep the summary
	// aligned with the chart.
	if len(labels) > 3 {
		sum.MysqldExtra, sum.MysqldExtraAvg = labels[3], avgs[3]
	}
	return sum
}

// snapshotSignature returns a stable hash of a snapshot's variable
// entries (sorted by name, skipping the volatile set). Adjacent
// snapshots with identical signatures are collapsed into a single
// rendered panel.
func snapshotSignature(entries []model.VariableEntry) string {
	filtered := make([]string, 0, len(entries))
	for _, e := range entries {
		if _, skip := volatileVariables[strings.ToLower(e.Name)]; skip {
			continue
		}
		filtered = append(filtered, e.Name+"\x00"+e.Value)
	}
	sort.Strings(filtered)
	h := fnv.New64a()
	for _, s := range filtered {
		h.Write([]byte(s))
		h.Write([]byte{'\n'})
	}
	return fmt.Sprintf("%016x", h.Sum64())
}

// extendRange grows the RangeNote on a kept snapshot view to cover
// the given 1-based snapshot number. RangeNote is re-formatted each
// call so the final text always reflects the full inclusive run.
func extendRange(v *variableSnapshotView, snapNum int) {
	lo, hi := parseRangeNote(v.RangeNote)
	if hi < snapNum {
		hi = snapNum
	}
	if lo > snapNum {
		lo = snapNum
	}
	v.RangeNote = formatRangeNote(lo, hi)
}

func formatRangeNote(lo, hi int) string {
	if lo == hi {
		return fmt.Sprintf("Identical values seen in snapshot #%d", lo)
	}
	return fmt.Sprintf("Identical values seen in snapshots #%d–#%d", lo, hi)
}

// parseRangeNote recovers (lo, hi) from a formatRangeNote string.
// When the note is missing or malformed (shouldn't happen) returns
// (0, 0) and callers can recover gracefully.
func parseRangeNote(note string) (int, int) {
	var lo, hi int
	if _, err := fmt.Sscanf(note, "Identical values seen in snapshot #%d", &lo); err == nil && lo > 0 {
		return lo, lo
	}
	if _, err := fmt.Sscanf(note, "Identical values seen in snapshots #%d–#%d", &lo, &hi); err == nil {
		return lo, hi
	}
	return 0, 0
}

func rangeIsSingle(note string) bool {
	lo, hi := parseRangeNote(note)
	return lo != 0 && lo == hi
}

// aggregateInnoDBMetrics rolls every per-snapshot InnoDB scalar up
// into one "worst-plus-distribution" card per metric. Max is treated
// as "worst" for all four metrics — for Semaphores, Pending I/O, and
// History list, higher means more trouble; for AHI searches/sec we
// also surface the peak because an unexplained spike or collapse is
// what a reader would want to notice first. Snapshots whose Data is
// nil (e.g. a missing -innodbstatus1 file) are simply skipped.
func aggregateInnoDBMetrics(snaps []model.SnapshotInnoDB) []innoDBMetricView {
	var sem, pio, hll []int
	for _, si := range snaps {
		if si.Data == nil {
			continue
		}
		sem = append(sem, si.Data.SemaphoreCount)
		pio = append(pio, si.Data.PendingReads+si.Data.PendingWrites)
		hll = append(hll, si.Data.HistoryListLength)
	}
	if len(sem) == 0 {
		return nil
	}
	semMetric := innoDBIntMetric("Semaphores", "waiting", sem)
	attachSemaphoreBreakdown(&semMetric, snaps)
	return []innoDBMetricView{
		semMetric,
		innoDBIntMetric("Pending I/O", "reads + writes", pio),
		buildAHIMetric(snaps),
		innoDBIntMetric("History list", "HLL (undo depth)", hll),
	}
}

// buildAHIMetric renders the AHI card as a hit-ratio view instead of
// a raw "searches/sec" rate. The hit ratio is the fraction of index
// lookups served directly from the adaptive hash index rather than
// by walking the B-tree, so a low value is the interesting signal —
// it means AHI is contributing little and the memory it occupies
// might be better spent elsewhere. Worst = lowest ratio across
// snapshots that had any activity; min/avg/max mirror that view.
//
// Formula: hit_ratio = hash_searches / (hash_searches + non_hash_searches)
//
// Snapshots with zero activity (hash + non-hash == 0) are skipped —
// dividing 0/0 is meaningless and lets an idle capture collapse the
// ratio to 0 which would misrepresent the server.
func buildAHIMetric(snaps []model.SnapshotInnoDB) innoDBMetricView {
	var ratios []float64
	type active struct {
		prefix        string
		hash, nonHash float64
		ratio         float64
	}
	var activeSnaps []active
	hashTblSize := 0
	for _, si := range snaps {
		if si.Data == nil {
			continue
		}
		if si.Data.AHIActivity.HashTableSize > 0 {
			hashTblSize = si.Data.AHIActivity.HashTableSize
		}
		h := si.Data.AHIActivity.SearchesPerSec
		nh := si.Data.AHIActivity.NonHashSearchesPerSec
		if h+nh <= 0 {
			continue
		}
		r := h * 100.0 / (h + nh)
		ratios = append(ratios, r)
		activeSnaps = append(activeSnaps, active{
			prefix: si.SnapshotPrefix, hash: h, nonHash: nh, ratio: r,
		})
	}
	if len(ratios) == 0 {
		return innoDBMetricView{
			Label: "AHI",
			Hint:  "hit ratio",
			Worst: "–",
			Min:   "–",
			Avg:   "–",
			Max:   "–",
			AHIFormula:       true,
			AHIFormulaText:   "hash / (hash + non-hash)",
			AHIHashTableSize: formatThousands(hashTblSize),
			AHINoActivity:    true,
		}
	}
	mn, mx, total := ratios[0], ratios[0], 0.0
	worstIdx := 0
	for i, r := range ratios {
		if r < mn {
			mn = r
			worstIdx = i
		}
		if r > mx {
			mx = r
		}
		total += r
	}
	avg := total / float64(len(ratios))
	worst := activeSnaps[worstIdx]
	return innoDBMetricView{
		Label: "AHI",
		Hint:  "hit ratio",
		Worst: FormatFloat(mn, 1) + "%",
		Min:   FormatFloat(mn, 1) + "%",
		Avg:   FormatFloat(avg, 1) + "%",
		Max:   FormatFloat(mx, 1) + "%",
		AHIFormula:       true,
		AHIFormulaText:   "hash / (hash + non-hash)",
		AHIWorstSnapshot: worst.prefix,
		AHIWorstHash:     FormatFloat(worst.hash, 2),
		AHIWorstNonHash:  FormatFloat(worst.nonHash, 2),
		AHIWorstRatio:    FormatFloat(worst.ratio, 1),
		AHIHashTableSize: formatThousands(hashTblSize),
	}
}

func formatThousands(n int) string {
	if n == 0 {
		return "–"
	}
	s := fmt.Sprintf("%d", n)
	// Insert commas every 3 digits from the right.
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

// attachSemaphoreBreakdown populates the Semaphores card with two
// contention-breakdown views: the peak-snapshot list and the
// summed-over-window list. No-op when no snapshot saw >0 waits.
func attachSemaphoreBreakdown(m *innoDBMetricView, snaps []model.SnapshotInnoDB) {
	// 1. Find the peak snapshot (first one tying the max) and sum
	//    sites across the whole window using a single pass.
	type siteKey struct {
		file  string
		line  int
		mutex string
	}
	totalByKey := map[siteKey]int{}
	totalAll := 0
	peakIdx := -1
	peakCount := -1
	for i, si := range snaps {
		if si.Data == nil {
			continue
		}
		if si.Data.SemaphoreCount > peakCount {
			peakCount = si.Data.SemaphoreCount
			peakIdx = i
		}
		for _, s := range si.Data.SemaphoreSites {
			totalByKey[siteKey{s.File, s.Line, s.MutexName}] += s.WaitCount
			totalAll += s.WaitCount
		}
	}
	if peakIdx < 0 || peakCount <= 0 {
		return
	}
	peak := snaps[peakIdx].Data
	peakTotal := peak.SemaphoreCount
	m.BreakdownPeak = buildSiteRows(peak.SemaphoreSites, peakTotal)
	m.BreakdownPeakSnapshot = snaps[peakIdx].SnapshotPrefix
	m.BreakdownPeakTotal = peakTotal

	// 2. Window-total breakdown: flatten the map, sort desc, format.
	if totalAll > 0 {
		rows := make([]model.SemaphoreSite, 0, len(totalByKey))
		for k, c := range totalByKey {
			rows = append(rows, model.SemaphoreSite{
				File:      k.file,
				Line:      k.line,
				MutexName: k.mutex,
				WaitCount: c,
			})
		}
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].WaitCount != rows[j].WaitCount {
				return rows[i].WaitCount > rows[j].WaitCount
			}
			if rows[i].File != rows[j].File {
				return rows[i].File < rows[j].File
			}
			if rows[i].Line != rows[j].Line {
				return rows[i].Line < rows[j].Line
			}
			return rows[i].MutexName < rows[j].MutexName
		})
		m.BreakdownTotal = buildSiteRows(rows, totalAll)
		m.BreakdownTotalTotal = totalAll
	}
}

func buildSiteRows(sites []model.SemaphoreSite, total int) []semaphoreSiteRow {
	out := make([]semaphoreSiteRow, 0, len(sites))
	for _, s := range sites {
		pct := 0.0
		if total > 0 {
			pct = float64(s.WaitCount) * 100.0 / float64(total)
		}
		out = append(out, semaphoreSiteRow{
			File:      s.File,
			Line:      s.Line,
			MutexName: s.MutexName,
			Count:     s.WaitCount,
			Percent:   FormatFloat(pct, 1),
		})
	}
	return out
}

func innoDBIntMetric(label, hint string, vals []int) innoDBMetricView {
	mn, mx, total := vals[0], vals[0], 0
	for _, v := range vals {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
		total += v
	}
	avg := float64(total) / float64(len(vals))
	return innoDBMetricView{
		Label: label,
		Hint:  hint,
		Worst: fmt.Sprintf("%d", mx),
		Min:   fmt.Sprintf("%d", mn),
		Avg:   FormatFloat(avg, 1),
		Max:   fmt.Sprintf("%d", mx),
	}
}

func innoDBFloatMetric(label, hint string, vals []float64, precision int) innoDBMetricView {
	mn, mx, total := vals[0], vals[0], 0.0
	for _, v := range vals {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
		total += v
	}
	avg := total / float64(len(vals))
	return innoDBMetricView{
		Label: label,
		Hint:  hint,
		Worst: FormatFloat(mx, precision),
		Min:   FormatFloat(mn, precision),
		Avg:   FormatFloat(avg, precision),
		Max:   FormatFloat(mx, precision),
	}
}

// isMysqldCommand reports whether a top-process Command string refers
// to the mysqld server (not mysqld_safe or another wrapper). Matches
// are case-insensitive and tolerant of leading/trailing whitespace.
func isMysqldCommand(cmd string) bool {
	return strings.EqualFold(strings.TrimSpace(cmd), "mysqld")
}

func summariseVmstat(d *model.VmstatData) *vmstatSummaryView {
	sum := &vmstatSummaryView{}
	peakForMetric := func(name string) float64 {
		for _, s := range d.Series {
			if s.Metric != name {
				continue
			}
			sum.SampleCount = maxInt(sum.SampleCount, len(s.Samples))
			var peak float64
			for _, sp := range s.Samples {
				if v := sp.Measurements[name]; v > peak {
					peak = v
				}
			}
			return peak
		}
		return 0
	}
	sum.PeakRunqueue = FormatFloat(peakForMetric("runqueue"), 0)
	sum.PeakBlocked = FormatFloat(peakForMetric("blocked"), 0)
	sum.PeakIowait = FormatFloat(peakForMetric("cpu_iowait"), 0)
	return sum
}

func fallback(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// classifyVariable compares a captured variable value against the
// documented compiled-in default. Returns:
//   - "default"  — value matches the documented default
//   - "modified" — value differs from the documented default
//   - "unknown"  — no default is documented for this variable
//
// Matching is tolerant: whitespace-trimmed, case-insensitive, and
// comma-separated values (e.g. sql_mode, tls_version) compared as
// sets so member order does not flag a default as modified.
func classifyVariable(defaults map[string]string, name, observed string) string {
	def, ok := defaults[name]
	if !ok {
		return "unknown"
	}
	if normalisedEqual(def, observed) {
		return "default"
	}
	if commaSetsEqual(def, observed) {
		return "default"
	}
	return "modified"
}

func normalisedEqual(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func commaSetsEqual(a, b string) bool {
	if !strings.Contains(a, ",") && !strings.Contains(b, ",") {
		return false
	}
	split := func(s string) []string {
		parts := strings.Split(s, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			out = append(out, strings.ToLower(p))
		}
		sort.Strings(out)
		return out
	}
	xs, ys := split(a), split(b)
	if len(xs) != len(ys) {
		return false
	}
	for i := range xs {
		if xs[i] != ys[i] {
			return false
		}
	}
	return true
}

// --- Advisor / findings view helpers --------------------------------

func buildFindingViews(fs []findings.Finding) []findingView {
	out := make([]findingView, 0, len(fs))
	for _, f := range fs {
		class, label := severityUIFields(f.Severity)
		metrics := make([]findingMetricView, 0, len(f.Metrics))
		for _, m := range f.Metrics {
			metrics = append(metrics, findingMetricView{
				Name:  m.Name,
				Value: findings.FormatNum(m.Value),
				Unit:  m.Unit,
				Note:  m.Note,
			})
		}
		out = append(out, findingView{
			ID:              f.ID,
			Subsystem:       f.Subsystem,
			Title:           f.Title,
			SeverityClass:   class,
			SeverityLabel:   label,
			OpenByDefault:   f.Severity == findings.SeverityCrit,
			Summary:         f.Summary,
			Explanation:     f.Explanation,
			FormulaText:     f.FormulaText,
			FormulaComputed: f.FormulaComputed,
			Metrics:         metrics,
			Recommendations: f.Recommendations,
		})
	}
	return out
}

func severityUIFields(s findings.Severity) (class, label string) {
	switch s {
	case findings.SeverityCrit:
		return "crit", "Critical"
	case findings.SeverityWarn:
		return "warn", "Warning"
	case findings.SeverityInfo:
		return "info", "Info"
	case findings.SeverityOK:
		return "ok", "OK"
	}
	return "info", "Info"
}

func summariseFindings(fs []findings.Finding) findingCountsView {
	c := findings.Summarise(fs)
	return findingCountsView{
		Crit: c.Crit,
		Warn: c.Warn,
		Info: c.Info,
		OK:   c.OK,
		Any:  (c.Crit + c.Warn + c.Info + c.OK) > 0,
	}
}

func advisorBadge(c findingCountsView) string {
	if !c.Any {
		return "no findings"
	}
	parts := []string{}
	if c.Crit > 0 {
		parts = append(parts, fmt.Sprintf("%d critical", c.Crit))
	}
	if c.Warn > 0 {
		parts = append(parts, fmt.Sprintf("%d warning", c.Warn))
	}
	if c.Info > 0 {
		parts = append(parts, fmt.Sprintf("%d info", c.Info))
	}
	if c.OK > 0 {
		parts = append(parts, fmt.Sprintf("%d ok", c.OK))
	}
	return strings.Join(parts, " · ")
}

