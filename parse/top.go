package parse

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// topHeaderLine matches the batch header emitted by `top -b -n N`:
//
//	top - HH:MM:SS up …
//
// We use the captured time to stamp each sample.
var topHeaderLine = regexp.MustCompile(`^top - (\d{1,2}):(\d{2}):(\d{2})\b`)

// parseTop reads `top -b` output as emitted by pt-stalk and returns
// per-process CPU-percent samples plus the top-3 processes ranked by
// average CPUPercent across all samples (absent-in-sample counts as 0
// — spec FR-010 / F7 resolution).
//
// Each sample begins with a "top - HH:MM:SS …" header line and ends
// at the next such header or EOF. Between them sits Tasks/CPU/Mem/Swap
// summary lines, a blank line, a "PID USER PR NI …" header, and zero
// or more process rows.
//
// Process rows are fixed-column but we split on whitespace; `%CPU` is
// always the 9th column (index 8). We preserve the command as written
// by top — truncated commands end with "+".
func parseTop(r io.Reader, snapshotStart time.Time, sourcePath string) (*model.TopData, []model.Diagnostic) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)

	var diagnostics []model.Diagnostic
	addDiag := func(line int, sev model.Severity, msg string) {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Location:   fmt.Sprintf("line %d", line),
			Severity:   sev,
			Message:    msg,
		})
	}

	var samples []model.ProcessSample

	snapshotDate := snapshotStart.In(time.UTC)
	currentTime := snapshotStart

	state := stateAwaitHeader
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if m := topHeaderLine.FindStringSubmatch(line); m != nil {
			hh, _ := strconv.Atoi(m[1])
			mm, _ := strconv.Atoi(m[2])
			ss, _ := strconv.Atoi(m[3])
			currentTime = time.Date(
				snapshotDate.Year(), snapshotDate.Month(), snapshotDate.Day(),
				hh, mm, ss, 0, time.UTC,
			)
			state = stateAwaitPIDHeader
			continue
		}

		switch state {
		case stateAwaitPIDHeader:
			if strings.HasPrefix(strings.TrimSpace(line), "PID ") {
				state = stateInProcRows
			}
		case stateInProcRows:
			if strings.TrimSpace(line) == "" {
				state = stateAwaitHeader
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 12 {
				// Not a process row; probably the next sample's Tasks line.
				state = stateAwaitHeader
				continue
			}
			pid, err := strconv.Atoi(fields[0])
			if err != nil {
				addDiag(lineNum, model.SeverityWarning, "top: unparseable PID column")
				continue
			}
			cpu, err := strconv.ParseFloat(fields[8], 64)
			if err != nil {
				addDiag(lineNum, model.SeverityWarning, fmt.Sprintf("top: unparseable %%CPU for pid=%d", pid))
				continue
			}
			// The COMMAND column is the 12th field and may contain spaces
			// in pathological cases; join remainder back together.
			cmd := strings.Join(fields[11:], " ")

			samples = append(samples, model.ProcessSample{
				Timestamp:  currentTime,
				PID:        pid,
				Command:    cmd,
				CPUPercent: cpu,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Severity:   model.SeverityError,
			Message:    fmt.Sprintf("top read: %v", err),
		})
	}

	if len(samples) == 0 {
		return nil, diagnostics
	}

	// Stable canonical order for process samples: by Timestamp then PID.
	sort.SliceStable(samples, func(i, j int) bool {
		if !samples[i].Timestamp.Equal(samples[j].Timestamp) {
			return samples[i].Timestamp.Before(samples[j].Timestamp)
		}
		return samples[i].PID < samples[j].PID
	})

	// Top-3 by AVERAGE CPUPercent across ALL samples (absent = 0),
	// per spec FR-010 and the F7 analyze-pass resolution. Total
	// samples = number of distinct Timestamps.
	timestampSet := map[time.Time]struct{}{}
	for _, s := range samples {
		timestampSet[s.Timestamp] = struct{}{}
	}
	totalSamples := len(timestampSet)
	if totalSamples == 0 {
		return &model.TopData{ProcessSamples: samples}, diagnostics
	}

	type agg struct {
		pid     int
		cmd     string
		sumCPU  float64
		samples []model.Sample
	}
	byPID := map[int]*agg{}
	for _, s := range samples {
		a := byPID[s.PID]
		if a == nil {
			a = &agg{pid: s.PID, cmd: s.Command}
			byPID[s.PID] = a
		}
		a.sumCPU += s.CPUPercent
		// keep the most-recent (longest) command line in case top
		// truncation changed.
		if len(s.Command) > len(a.cmd) {
			a.cmd = s.Command
		}
		a.samples = append(a.samples, model.Sample{
			Timestamp:    s.Timestamp,
			Measurements: map[string]float64{"cpu_percent": s.CPUPercent},
		})
	}
	// Rank by average = sumCPU / totalSamples.
	ranked := make([]*agg, 0, len(byPID))
	for _, a := range byPID {
		ranked = append(ranked, a)
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		ai := ranked[i].sumCPU / float64(totalSamples)
		aj := ranked[j].sumCPU / float64(totalSamples)
		if ai != aj {
			return ai > aj
		}
		// Tiebreaker: higher PID first (deterministic per data-model.md).
		return ranked[i].pid > ranked[j].pid
	})
	if len(ranked) > 3 {
		ranked = ranked[:3]
	}

	top3 := make([]model.ProcessSeries, 0, len(ranked))
	for _, a := range ranked {
		sort.SliceStable(a.samples, func(i, j int) bool {
			return a.samples[i].Timestamp.Before(a.samples[j].Timestamp)
		})
		top3 = append(top3, model.ProcessSeries{
			PID:     a.pid,
			Command: a.cmd,
			CPU: model.MetricSeries{
				Metric:  "cpu_percent",
				Unit:    "%",
				Subject: fmt.Sprintf("pid=%d %s", a.pid, a.cmd),
				Samples: a.samples,
			},
		})
	}
	return &model.TopData{
		ProcessSamples: samples,
		Top3ByAverage:  top3,
	}, diagnostics
}

// top-file parse state machine.
const (
	stateAwaitHeader = iota
	stateAwaitPIDHeader
	stateInProcRows
)
