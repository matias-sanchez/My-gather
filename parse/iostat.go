package parse

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// parseIostat reads the output of `iostat -x 1` as emitted by pt-stalk
// and returns per-device time-series for utilisation (%util) and
// average queue size (aqu-sz).
//
// Each iostat sample is separated by blank lines and begins with a
// "Device …" header row. Samples do not carry per-sample timestamps;
// we synthesise them as snapshotStart + sample_index * 1s, matching
// pt-stalk's default `iostat -x 1` invocation.
//
// Numbers may use either "," or "." as the decimal separator depending
// on the collecting host's locale. We accept both.
//
// The first line of the file is the Linux banner
// ("Linux 5.14.0-… (host) …"); it is skipped.
//
// Malformed sample rows are recorded as SeverityWarning diagnostics
// and skipped; the parser continues with remaining samples. Returns
// (nil, diagnostics) only when no usable samples are present at all.
func parseIostat(r io.Reader, snapshotStart time.Time, sourcePath string) (*model.IostatData, []model.Diagnostic) {
	scanner := newLineScanner(r)

	var diagnostics []model.Diagnostic
	addDiag := func(line int, msg string) {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Location:   fmt.Sprintf("line %d", line),
			Severity:   model.SeverityWarning,
			Message:    msg,
		})
	}

	type samplePoint struct {
		util  float64
		aqusz float64
	}
	// device -> ordered list of samples. Final device ordering is
	// alphabetical (see sort.Strings below) — insertion order is
	// intentionally not preserved, since stable-by-name matches what
	// data-model.md promises to the render layer.
	deviceSamples := map[string][]samplePoint{}

	var (
		utilIdx  = -1
		aqszIdx  = -1
		inSample = false
	)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)

		// Blank line ends the current sample block.
		if line == "" {
			inSample = false
			continue
		}

		// Skip the Linux banner line (e.g., "Linux 5.14.0-… (host) …").
		if !inSample && strings.HasPrefix(line, "Linux ") {
			continue
		}

		// New-sample header row. Resolve the column positions for the
		// two fields we care about.
		if strings.HasPrefix(line, "Device") {
			cols := strings.Fields(line)
			utilIdx = indexOf(cols, "%util")
			if utilIdx < 0 {
				utilIdx = indexOf(cols, "util")
			}
			aqszIdx = indexOf(cols, "aqu-sz")
			if aqszIdx < 0 {
				aqszIdx = indexOf(cols, "avgqu-sz")
			}
			if utilIdx < 0 || aqszIdx < 0 {
				addDiag(lineNum, fmt.Sprintf("iostat header missing expected columns (cols=%v)", cols))
				inSample = false
				continue
			}
			inSample = true
			continue
		}

		if !inSample {
			continue
		}

		// Device row. Split on whitespace; replace European decimal
		// separator before parsing.
		cols := strings.Fields(line)
		if len(cols) <= utilIdx || len(cols) <= aqszIdx {
			addDiag(lineNum, fmt.Sprintf("iostat row has %d columns (need at least %d)", len(cols), maxInt(utilIdx, aqszIdx)+1))
			continue
		}
		device := cols[0]
		util, errU := parseLocalizedFloat(cols[utilIdx])
		aqusz, errA := parseLocalizedFloat(cols[aqszIdx])
		if errU != nil || errA != nil {
			addDiag(lineNum, fmt.Sprintf("iostat row %q: could not parse %s/%s", device, cols[utilIdx], cols[aqszIdx]))
			continue
		}

		deviceSamples[device] = append(deviceSamples[device], samplePoint{
			util:  util,
			aqusz: aqusz,
		})
	}
	if err := scanner.Err(); err != nil {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Severity:   model.SeverityError,
			Message:    fmt.Sprintf("iostat read: %v", err),
		})
	}

	// Post-process: timestamp each sample by its position within its
	// device's slice. For a well-formed iostat file every device appears
	// in every sample in the same order, so re-indexing by position
	// within each device's slice gives the correct sample index.
	var devices []string
	for d := range deviceSamples {
		devices = append(devices, d)
	}
	sort.Strings(devices)

	if len(devices) == 0 {
		return nil, diagnostics
	}

	data := &model.IostatData{}
	for _, d := range devices {
		points := deviceSamples[d]
		utilSamples := make([]model.Sample, 0, len(points))
		aqszSamples := make([]model.Sample, 0, len(points))
		for i, p := range points {
			t := snapshotStart.Add(time.Duration(i) * time.Second)
			utilSamples = append(utilSamples, model.Sample{
				Timestamp:    t,
				Measurements: map[string]float64{"util_percent": p.util},
			})
			aqszSamples = append(aqszSamples, model.Sample{
				Timestamp:    t,
				Measurements: map[string]float64{"avgqu_sz": p.aqusz},
			})
		}
		data.Devices = append(data.Devices, model.DeviceSeries{
			Device: d,
			Utilization: model.MetricSeries{
				Metric: "util_percent", Unit: "%", Subject: d, Samples: utilSamples,
			},
			AvgQueueSize: model.MetricSeries{
				Metric: "avgqu_sz", Unit: "count", Subject: d, Samples: aqszSamples,
			},
		})
	}
	return data, diagnostics
}

// parseLocalizedFloat accepts either "." or "," as decimal separator.
func parseLocalizedFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, ",") && !strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ",", ".")
	}
	return strconv.ParseFloat(s, 64)
}

func indexOf(xs []string, target string) int {
	for i, x := range xs {
		if x == target {
			return i
		}
	}
	return -1
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
