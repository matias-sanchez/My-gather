package render

import (
	"fmt"
	"math"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/parse"
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
		if r.OSSection.NetCounters != nil {
			payload["network-counters"] = networkCountersChartPayload(r.OSSection.NetCounters)
		}
		if r.OSSection.NetSockets != nil {
			payload["network-sockets"] = networkSocketsChartPayload(r.OSSection.NetSockets)
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
// are skipped, mirroring aggregateInnoDBMetrics. Returns nil only when
// NO populated snapshots exist (so the sparkline container stays empty
// and the scalar callout is all the reader sees). With one populated
// snapshot renderHLLSparkline in app.js draws its single-sample fallback
// (centred dot + value label); with two+ it draws the proper spline.
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

	activitySeries := []map[string]any{
		{
			"label": "Active",
			"values": processlistMetricValues(d, func(s model.ThreadStateSample) float64 {
				return float64(s.ActiveThreads)
			}),
		},
		{
			"label": "Sleeping",
			"values": processlistMetricValues(d, func(s model.ThreadStateSample) float64 {
				return float64(s.SleepingThreads)
			}),
		},
		{
			"label": "Total",
			"values": processlistMetricValues(d, func(s model.ThreadStateSample) float64 {
				return float64(s.TotalThreads)
			}),
		},
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
		{
			"key":    "activity",
			"label":  "Activity",
			"unit":   "threads",
			"series": activitySeries,
		},
	}

	// Primary `series` stays pointed at the State dimension so the
	// existing renderTimeSeries fallback path remains functional.
	return map[string]any{
		"timestamps":  timestamps,
		"series":      dimensions[0]["series"],
		"dimensions":  dimensions,
		"slowQueries": processlistSlowQueryPayload(d.ObservedQueries),
		"metrics": map[string]any{
			"maxTimeSeconds": processlistMetricValues(d, func(s model.ThreadStateSample) float64 {
				return s.MaxTimeMS / 1000
			}),
			"hasMaxTimeSeconds": processlistBoolValues(d, func(s model.ThreadStateSample) bool {
				return s.HasTimeMetric
			}),
			"maxRowsExamined": processlistMetricValues(d, func(s model.ThreadStateSample) float64 {
				return s.MaxRowsExamined
			}),
			"hasRowsExamined": processlistBoolValues(d, func(s model.ThreadStateSample) bool {
				return s.HasRowsExaminedMetric
			}),
			"maxRowsSent": processlistMetricValues(d, func(s model.ThreadStateSample) float64 {
				return s.MaxRowsSent
			}),
			"hasRowsSent": processlistBoolValues(d, func(s model.ThreadStateSample) bool {
				return s.HasRowsSentMetric
			}),
			"rowsWithQueryText": processlistMetricValues(d, func(s model.ThreadStateSample) float64 {
				return float64(s.RowsWithQueryText)
			}),
			"hasQueryText": processlistBoolValues(d, func(s model.ThreadStateSample) bool {
				return s.HasQueryTextMetric
			}),
		},
		"snapshotBoundaries": d.SnapshotBoundaries,
	}
}

func processlistSlowQueryPayload(queries []model.ObservedProcesslistQuery) []map[string]any {
	out := make([]map[string]any, 0, len(queries))
	for _, q := range queries {
		out = append(out, map[string]any{
			"fingerprint":     q.Fingerprint,
			"snippet":         q.Snippet,
			"firstSeen":       q.FirstSeen.Unix(),
			"lastSeen":        q.LastSeen.Unix(),
			"seenSamples":     q.SeenSamples,
			"maxTimeSeconds":  q.MaxTimeMS / 1000,
			"hasTimeMetric":   q.HasTimeMetric,
			"maxRowsExamined": q.MaxRowsExamined,
			"hasRowsExamined": q.HasRowsExaminedMetric,
			"maxRowsSent":     q.MaxRowsSent,
			"hasRowsSent":     q.HasRowsSentMetric,
			"user":            q.User,
			"db":              q.DB,
			"command":         q.Command,
			"state":           q.State,
		})
	}
	return out
}

func processlistMetricValues(d *model.ProcesslistData, pick func(model.ThreadStateSample) float64) []float64 {
	values := make([]float64, len(d.ThreadStateSamples))
	for i, s := range d.ThreadStateSamples {
		values[i] = pick(s)
	}
	return values
}

func processlistBoolValues(d *model.ProcesslistData, pick func(model.ThreadStateSample) bool) []bool {
	values := make([]bool, len(d.ThreadStateSamples))
	for i, s := range d.ThreadStateSamples {
		values[i] = pick(s)
	}
	return values
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

// networkCountersChartPayload emits per-sample rates for the curated
// TCP/UDP counters. Raw Deltas are divided by the per-slot wall-clock
// gap so the chart axis reads as "events/s" rather than raw deltas
// (inflated by longer snapshot gaps). Slot 0 stays NaN (no baseline);
// the frontend treats NaN as "no sample".
func networkCountersChartPayload(d *model.NetstatCountersData) map[string]any {
	n := len(d.Timestamps)
	if n == 0 {
		return nil
	}
	gapSec := make([]float64, n)
	for i := 1; i < n; i++ {
		dt := d.Timestamps[i] - d.Timestamps[i-1]
		if dt > 0 {
			gapSec[i] = dt
		}
	}
	series := make([]map[string]any, 0, len(d.Labels))
	for _, name := range d.Labels {
		raw := d.Deltas[name]
		// Emit a JSON-safe value stream: NaN → nil (the frontend
		// treats nil as "no sample", same convention used by
		// mysqladminChartPayload).
		vals := make([]any, n)
		vals[0] = nil
		for i := 1; i < n; i++ {
			if math.IsNaN(raw[i]) || gapSec[i] == 0 {
				vals[i] = nil
			} else {
				vals[i] = raw[i] / gapSec[i]
			}
		}
		series = append(series, map[string]any{
			"label":  parse.NetstatSDisplayName(name),
			"values": vals,
		})
	}
	return map[string]any{
		"timestamps":         d.Timestamps,
		"series":             series,
		"snapshotBoundaries": d.SnapshotBoundaries,
	}
}

// networkSocketsChartPayload emits one series per observed socket
// state — each a count-per-snapshot timeseries rendered as a stacked
// bar chart showing socket-state composition across the capture.
func networkSocketsChartPayload(d *model.NetstatSocketsData) map[string]any {
	n := len(d.Samples)
	if n == 0 {
		return nil
	}
	timestamps := make([]float64, n)
	for i, s := range d.Samples {
		timestamps[i] = float64(s.Timestamp.Unix())
	}
	series := make([]map[string]any, 0, len(d.States))
	for _, st := range d.States {
		vals := make([]float64, n)
		for i, s := range d.Samples {
			vals[i] = float64(s.StateCounts[st])
		}
		series = append(series, map[string]any{
			"label":  st,
			"values": vals,
		})
	}
	return map[string]any{
		"timestamps":         timestamps,
		"series":             series,
		"snapshotBoundaries": d.SnapshotBoundaries,
	}
}
