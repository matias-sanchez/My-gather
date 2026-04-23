package render

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

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
	nonNil := make([]*model.IostatData, 0, len(ins))
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
	nonNil := make([]*model.TopData, 0, len(ins))
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

// metricSeriesHolder adapts *VmstatData / *MeminfoData for
// mergeMetricSeriesByName so one helper can walk both.
type metricSeriesHolder struct {
	series []model.MetricSeries
}

// mergeMetricSeriesByName concatenates MetricSeries across per-Snapshot
// inputs, aligning by Metric name (not by slice index), so a later
// snapshot that omits a series or emits it in a different order does
// not shift the shared x-axis. Returns the merged series list and the
// per-input boundary offsets.
//
// `adoptUnitFromSource` controls the Unit policy:
//   - true (vmstat): each output series starts with Unit="mixed" and
//     is promoted to the first non-empty Unit seen across inputs.
//   - false (meminfo): each output series starts with the supplied
//     defaultUnit and is never rewritten.
//
// The "primary axis" is the FIRST declared metric of the first input,
// which matches both callers' previous semantics.
func mergeMetricSeriesByName(inputs []metricSeriesHolder, defaultUnit string, adoptUnitFromSource bool) ([]model.MetricSeries, []int) {
	if len(inputs) == 0 {
		return nil, nil
	}
	metrics := make([]string, 0, len(inputs[0].series))
	for _, s := range inputs[0].series {
		metrics = append(metrics, s.Metric)
	}
	seriesByMetric := make(map[string]*model.MetricSeries, len(metrics))
	for _, m := range metrics {
		seriesByMetric[m] = &model.MetricSeries{Metric: m, Unit: defaultUnit}
	}

	boundaries := make([]int, 0, len(inputs))
	cumulative := 0
	for _, in := range inputs {
		boundaries = append(boundaries, cumulative)
		primaryAdded := 0
		for _, s := range in.series {
			m := seriesByMetric[s.Metric]
			if m == nil {
				continue // unknown metric in a later input — skip
			}
			m.Samples = append(m.Samples, s.Samples...)
			if s.Metric == metrics[0] {
				primaryAdded = len(s.Samples)
			}
			if adoptUnitFromSource && m.Unit == defaultUnit && s.Unit != "" {
				m.Unit = s.Unit
			}
		}
		cumulative += primaryAdded
	}

	out := make([]model.MetricSeries, 0, len(metrics))
	for _, m := range metrics {
		out = append(out, *seriesByMetric[m])
	}
	return out, boundaries
}

// concatVmstat merges multiple per-Snapshot *VmstatData by
// concatenating each canonically-ordered Series' Samples in input
// order. SnapshotBoundaries indexes into Series[0].Samples — the
// primary timestamp axis the renderer uses.
func concatVmstat(ins []*model.VmstatData) *model.VmstatData {
	nonNil := make([]*model.VmstatData, 0, len(ins))
	for _, d := range ins {
		if d != nil && len(d.Series) > 0 {
			nonNil = append(nonNil, d)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	holders := make([]metricSeriesHolder, len(nonNil))
	for i, d := range nonNil {
		holders[i] = metricSeriesHolder{series: d.Series}
	}
	merged, boundaries := mergeMetricSeriesByName(holders, "mixed", true)
	return &model.VmstatData{Series: merged, SnapshotBoundaries: boundaries}
}

// concatMeminfo merges multiple per-Snapshot *MeminfoData the same
// way concatVmstat does, aligning by Metric so a later snapshot that
// happens to emit a different subset of series doesn't shift the
// axis. Unit is always "GB" — no need for the "mixed" fallback
// vmstat carries.
func concatMeminfo(ins []*model.MeminfoData) *model.MeminfoData {
	nonNil := make([]*model.MeminfoData, 0, len(ins))
	for _, d := range ins {
		if d != nil && len(d.Series) > 0 {
			nonNil = append(nonNil, d)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	holders := make([]metricSeriesHolder, len(nonNil))
	for i, d := range nonNil {
		holders[i] = metricSeriesHolder{series: d.Series}
	}
	merged, boundaries := mergeMetricSeriesByName(holders, "GB", false)
	return &model.MeminfoData{Series: merged, SnapshotBoundaries: boundaries}
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
