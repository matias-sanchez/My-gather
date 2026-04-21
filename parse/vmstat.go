package parse

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// vmstatColumns is the declared series order for the typed payload
// (spec FR-011). 13 series. If a column is absent from an older vmstat
// release, its MetricSeries carries zero samples and renders greyed-
// out.
var vmstatColumns = []struct {
	key    string // column header token as emitted by vmstat
	metric string
	unit   string
}{
	{"r", "runqueue", "count"},
	{"b", "blocked", "count"},
	{"free", "free_kb", "kB"},
	{"buff", "buff_kb", "kB"},
	{"cache", "cache_kb", "kB"},
	{"si", "swap_in", "kB/s"},
	{"so", "swap_out", "kB/s"},
	{"bi", "io_in", "blocks/s"},
	{"bo", "io_out", "blocks/s"},
	{"us", "cpu_user", "%"},
	{"sy", "cpu_sys", "%"},
	{"id", "cpu_idle", "%"},
	{"wa", "cpu_iowait", "%"},
}

// parseVmstat reads `vmstat 1` output as emitted by pt-stalk and
// returns a fixed 13-series time-series (spec FR-011).
//
// Format:
//
//	procs -----------memory---------- ---swap-- -----io---- -system-- ------cpu-----
//	 r  b   swpd   free   buff  cache   si   so    bi    bo   in   cs us sy id wa st
//	12  1      0 104588800 10433708 236728672    0    0     5    93    0    1  3  3 94  0  0
//	...
//
// The "procs" banner may repeat every N rows depending on vmstat
// version — we treat it and the column-label row as markers and skip.
// Every other line that tokenises to 17 integers is a data row.
//
// Samples don't carry per-sample timestamps; we synthesise them as
// snapshotStart + sample_index * 1s.
func parseVmstat(r io.Reader, snapshotStart time.Time, sourcePath string) (*model.VmstatData, []model.Diagnostic) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)

	var diagnostics []model.Diagnostic
	addDiag := func(line int, msg string) {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Location:   fmt.Sprintf("line %d", line),
			Severity:   model.SeverityWarning,
			Message:    msg,
		})
	}

	// Build column-name -> position map from the second header row.
	var headerCols []string
	colIndex := map[string]int{}
	seriesValues := make([][]float64, len(vmstatColumns))
	var timestamps []time.Time

	sampleIdx := 0
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "procs") {
			// Banner line repeats every N rows; skip.
			continue
		}
		// Column-label line always contains "r" and "b" as the first
		// two tokens alongside memory/swap fields. Detect it by
		// looking for a non-numeric first token.
		fields := strings.Fields(line)
		if _, err := strconv.Atoi(fields[0]); err != nil {
			// Header. Record column positions.
			headerCols = fields
			for i, c := range headerCols {
				if _, seen := colIndex[c]; !seen {
					colIndex[c] = i
				}
			}
			continue
		}
		// Data row.
		if len(headerCols) == 0 {
			addDiag(lineNum, "vmstat data row before any header")
			continue
		}
		row := make(map[string]float64, len(vmstatColumns))
		for _, c := range vmstatColumns {
			idx, ok := colIndex[c.key]
			if !ok {
				continue
			}
			if idx >= len(fields) {
				continue
			}
			v, err := strconv.ParseFloat(fields[idx], 64)
			if err != nil {
				continue
			}
			row[c.key] = v
		}
		t := snapshotStart.Add(time.Duration(sampleIdx) * time.Second)
		timestamps = append(timestamps, t)
		for i, c := range vmstatColumns {
			seriesValues[i] = append(seriesValues[i], row[c.key])
		}
		sampleIdx++
	}
	if err := scanner.Err(); err != nil {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Severity:   model.SeverityError,
			Message:    fmt.Sprintf("vmstat read: %v", err),
		})
	}

	if sampleIdx == 0 {
		return nil, diagnostics
	}

	data := &model.VmstatData{}
	for i, c := range vmstatColumns {
		samples := make([]model.Sample, len(timestamps))
		for j, t := range timestamps {
			samples[j] = model.Sample{
				Timestamp:    t,
				Measurements: map[string]float64{c.metric: seriesValues[i][j]},
			}
		}
		data.Series = append(data.Series, model.MetricSeries{
			Metric:  c.metric,
			Unit:    c.unit,
			Subject: "", // system-wide
			Samples: samples,
		})
	}
	return data, diagnostics
}
