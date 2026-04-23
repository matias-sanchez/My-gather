package render

import (
	"fmt"
	"math"

	"github.com/matias-sanchez/My-gather/model"
)

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
		if r.OSSection.Meminfo != nil {
			payload["meminfo"] = meminfoChartPayload(r.OSSection.Meminfo)
		}
	}
	if r.DBSection != nil {
		if r.DBSection.Mysqladmin != nil {
			payload["mysqladmin"] = mysqladminChartPayload(r.DBSection.Mysqladmin)
		}
		if r.DBSection.Processlist != nil {
			payload["processlist"] = processlistChartPayload(r.DBSection.Processlist)
		}
		if hll := innoDBHLLSparklinePayload(r.DBSection.InnoDBPerSnapshot); hll != nil {
			payload["innodb-hll"] = hll
		}
	}
	return payload
}

// innoDBHLLSparklinePayload emits the per-snapshot History List Length
// series consumed by the small sparkline below the aggregated History
// list callout. Snapshots whose Data is nil (file absent / unparseable)
// are skipped, mirroring aggregateInnoDBMetrics. Returns nil when fewer
// than one populated snapshot exists so the template/JS falls back to
// the scalar-only display.
func innoDBHLLSparklinePayload(snaps []model.SnapshotInnoDB) map[string]any {
	var ts []float64
	var vals []float64
	var prefixes []string
	for _, si := range snaps {
		if si.Data == nil {
			continue
		}
		ts = append(ts, float64(si.Timestamp.Unix()))
		vals = append(vals, float64(si.Data.HistoryListLength))
		prefixes = append(prefixes, si.SnapshotPrefix)
	}
	if len(vals) == 0 {
		return nil
	}
	return map[string]any{
		"timestamps": ts,
		"values":     vals,
		"prefixes":   prefixes,
	}
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

func meminfoChartPayload(d *model.MeminfoData) map[string]any {
	if len(d.Series) == 0 {
		return nil
	}
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
