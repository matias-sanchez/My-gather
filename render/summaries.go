package render

import (
	"fmt"
	"math"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

func isNaNOrInf(v float64) bool {
	return math.IsNaN(v) || math.IsInf(v, 0)
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
	sum.PeakUtil = formatFloat(maxUtil, 1) + "%"
	sum.PeakDevice = fallback(maxUtilDev, "–")
	sum.PeakAqusz = formatFloat(maxAqu, 2)
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
		avgs = append(avgs, formatFloat(avg, 1))
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
	sum.PeakRunqueue = formatFloat(peakForMetric("runqueue"), 0)
	sum.PeakBlocked = formatFloat(peakForMetric("blocked"), 0)
	sum.PeakIowait = formatFloat(peakForMetric("cpu_iowait"), 0)
	return sum
}

func summariseMeminfo(d *model.MeminfoData) *meminfoSummaryView {
	sum := &meminfoSummaryView{}
	// Helpers: for "pressure floor" we want the MIN value across
	// samples (i.e. the headroom at its worst); for backlog metrics
	// we want the MAX.
	minSeries := func(name string) (float64, bool) {
		for _, s := range d.Series {
			if s.Metric != name {
				continue
			}
			sum.SampleCount = maxInt(sum.SampleCount, len(s.Samples))
			if len(s.Samples) == 0 {
				return 0, false
			}
			min := s.Samples[0].Measurements[name]
			for _, sp := range s.Samples {
				if v := sp.Measurements[name]; v < min {
					min = v
				}
			}
			return min, true
		}
		return 0, false
	}
	maxSeries := func(name string) (float64, bool) {
		for _, s := range d.Series {
			if s.Metric != name {
				continue
			}
			sum.SampleCount = maxInt(sum.SampleCount, len(s.Samples))
			if len(s.Samples) == 0 {
				return 0, false
			}
			var peak float64
			for _, sp := range s.Samples {
				if v := sp.Measurements[name]; v > peak {
					peak = v
				}
			}
			return peak, true
		}
		return 0, false
	}
	if v, ok := minSeries("mem_available"); ok {
		sum.MinAvailable = formatFloat(v, 2)
	}
	if v, ok := maxSeries("anon_pages"); ok {
		sum.MaxAnonPages = formatFloat(v, 2)
	}
	if v, ok := maxSeries("dirty"); ok {
		sum.MaxDirty = formatFloat(v, 3)
	}
	if v, ok := maxSeries("swap_used"); ok {
		sum.MaxSwapUsed = formatFloat(v, 2)
	}
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

// summariseNetwork produces the chip-strip atop the Network subview.
// Inputs come from both -netstat_s (rate-derived peaks) and -netstat
// (socket-state absolute peaks). Accepts nil inputs and degrades
// gracefully (fields stay empty → template omits the chip).
func summariseNetwork(counters *model.NetstatCountersData, sockets *model.NetstatSocketsData) *networkSummaryView {
	sum := &networkSummaryView{}

	peakRate := func(name string) float64 {
		if counters == nil {
			return 0
		}
		arr, ok := counters.Deltas[name]
		if !ok {
			return 0
		}
		var peak, gap float64
		for i := 1; i < len(arr); i++ {
			if isNaNOrInf(arr[i]) {
				continue
			}
			gap = counters.Timestamps[i] - counters.Timestamps[i-1]
			if gap <= 0 {
				continue
			}
			rate := arr[i] / gap
			if rate > peak {
				peak = rate
			}
		}
		return peak
	}
	if r := peakRate("tcp_retransmits"); r > 0 {
		sum.PeakRetransmits = fmt.Sprintf("%.2f", r)
	}
	if r := peakRate("tcp_listen_overflows"); r > 0 {
		sum.PeakListenOverflows = fmt.Sprintf("%.2f", r)
	}

	if sockets != nil {
		var peakTW, peakCW int
		for _, s := range sockets.Samples {
			if v := s.StateCounts["TIME_WAIT"]; v > peakTW {
				peakTW = v
			}
			if v := s.StateCounts["CLOSE_WAIT"]; v > peakCW {
				peakCW = v
			}
		}
		if peakTW > 0 {
			sum.PeakTimeWait = fmt.Sprintf("%d", peakTW)
		}
		if peakCW > 0 {
			sum.PeakCloseWait = fmt.Sprintf("%d", peakCW)
		}
		sum.SampleCount = len(sockets.Samples)
	}
	if sum.SampleCount == 0 && counters != nil {
		sum.SampleCount = len(counters.Timestamps)
	}
	return sum
}

func summariseProcesslist(d *model.ProcesslistData) *processlistSummaryView {
	sum := &processlistSummaryView{}
	if d == nil {
		return sum
	}
	var peakActive, peakSleeping, peakQueryTextRows int
	var longestAgeMS, peakRowsExamined, peakRowsSent float64
	for _, s := range d.ThreadStateSamples {
		sum.SampleCount++
		if s.ActiveThreads > peakActive {
			peakActive = s.ActiveThreads
		}
		if s.SleepingThreads > peakSleeping {
			peakSleeping = s.SleepingThreads
		}
		if s.MaxTimeMS > longestAgeMS {
			longestAgeMS = s.MaxTimeMS
		}
		if s.MaxRowsExamined > peakRowsExamined {
			peakRowsExamined = s.MaxRowsExamined
		}
		if s.MaxRowsSent > peakRowsSent {
			peakRowsSent = s.MaxRowsSent
		}
		if s.RowsWithQueryText > peakQueryTextRows {
			peakQueryTextRows = s.RowsWithQueryText
		}
	}
	sum.PeakActive = fmt.Sprintf("%d", peakActive)
	sum.PeakSleeping = fmt.Sprintf("%d", peakSleeping)
	sum.LongestAge = formatFloat(longestAgeMS/1000, 1)
	sum.PeakRowsExamined = formatFloat(peakRowsExamined, 0)
	sum.PeakRowsSent = formatFloat(peakRowsSent, 0)
	sum.PeakQueryTextRows = fmt.Sprintf("%d", peakQueryTextRows)
	return sum
}
