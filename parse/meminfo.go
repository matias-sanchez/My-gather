package parse

import (
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// meminfoSeries is the declared series order for the MeminfoData
// payload. Each entry pairs a /proc/meminfo key with the canonical
// Metric name the chart surfaces. Values are converted from kB to
// gigabytes (÷ 1,048,576) so the Y-axis reads directly as GB.
//
// The curated list prioritises signal for DB workloads:
//   - mem_available / mem_free: headroom.
//   - cached / buffers: page/block cache (typically dominant on DB hosts).
//   - anon_pages: process memory (mysqld buffer pool accounts for most of this).
//   - dirty: fsync pressure — correlates with redo-log / binlog saturation.
//   - slab: kernel-object cache — unusually large values can starve userspace.
//   - swap_used: swap pressure (derived as SwapTotal − SwapFree).
//
// Anything else in /proc/meminfo is intentionally left out so the
// chart stays readable; operators who need a specific field can
// still grep the raw file.
var meminfoSeries = []struct {
	key    string
	metric string
}{
	{"MemAvailable", "mem_available"},
	{"MemFree", "mem_free"},
	{"Cached", "cached"},
	{"Buffers", "buffers"},
	{"AnonPages", "anon_pages"},
	{"Dirty", "dirty"},
	{"Slab", "slab"},
	{"__swap_used", "swap_used"}, // synthesised: SwapTotal − SwapFree
}

// meminfoValueLine matches a /proc/meminfo "Key:  value kB" row.
// The unit is always kB on Linux; the parser assumes that and
// converts to GB at emit time.
var meminfoValueLine = regexp.MustCompile(`^([A-Za-z0-9_()]+):\s+(\d+)\s*kB\s*$`)

const kbPerGB = 1024.0 * 1024.0

// parseMeminfo reads pt-stalk -meminfo output — repeated
// /proc/meminfo dumps, each preceded by a TS boundary line — and
// returns one MeminfoData whose Series is built from meminfoSeries
// in declared order. Values are in gigabytes.
func parseMeminfo(r io.Reader, sourcePath string) (*model.MeminfoData, []model.Diagnostic) {
	scanner := newLineScanner(r)

	var diagnostics []model.Diagnostic

	type sampleBuild struct {
		t    time.Time
		vals map[string]float64
	}
	var samples []sampleBuild
	var current *sampleBuild

	startNewSample := func(t time.Time) {
		if current != nil {
			samples = append(samples, *current)
		}
		current = &sampleBuild{t: t, vals: map[string]float64{}}
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if m := reTimestampLine.FindStringSubmatch(line); m != nil {
			epoch, _ := strconv.ParseFloat(m[1], 64)
			secs := int64(math.Floor(epoch))
			ns := int64(math.Round((epoch - float64(secs)) * 1e9))
			t := time.Unix(secs, ns).UTC()
			startNewSample(t)
			continue
		}
		if m := meminfoValueLine.FindStringSubmatch(line); m != nil {
			if current == nil {
				// Canonical pt-stalk writes a `TS <epoch> …` header
				// before each /proc/meminfo dump. A value line before
				// any TS means the file is truncated or corrupted;
				// reject the whole input so the caller doesn't render
				// misattributed data.
				diagnostics = append(diagnostics, model.Diagnostic{
					SourceFile: sourcePath,
					Severity:   model.SeverityError,
					Message:    "meminfo: value line before first TS marker; input is not canonical pt-stalk output",
				})
				return nil, diagnostics
			}
			// m[2] is the (\d+) capture group from meminfoValueLine; it
			// is digits-only by construction, so ParseFloat cannot fail
			// within the physical-memory range. Discard the error.
			v, _ := strconv.ParseFloat(m[2], 64)
			current.vals[m[1]] = v
		}
	}
	if err := scanner.Err(); err != nil {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Severity:   model.SeverityError,
			Message:    fmt.Sprintf("meminfo read: %v", err),
		})
	}
	if current != nil {
		samples = append(samples, *current)
	}
	if len(samples) == 0 {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Severity:   model.SeverityError,
			Message:    "meminfo: no samples found; input is empty or lacks any TS header",
		})
		return nil, diagnostics
	}

	// Derive SwapUsed = SwapTotal − SwapFree. Negative or missing
	// inputs collapse to zero rather than being emitted as NaN.
	for i := range samples {
		total := samples[i].vals["SwapTotal"]
		free := samples[i].vals["SwapFree"]
		used := total - free
		if used < 0 {
			used = 0
		}
		samples[i].vals["__swap_used"] = used
	}

	data := &model.MeminfoData{}
	for _, s := range meminfoSeries {
		// MED #3: detect declared series that never saw a single sample
		// across the whole capture window. We still emit the series
		// (filled with 0 GB) so the chart layout stays stable, but we
		// attach an informational SeverityWarning diagnostic so the
		// reader can see *why* a line is flat instead of silently
		// getting a zero series.
		seen := false
		// __swap_used is always synthesised above from SwapTotal/Free;
		// skip the "never seen" check — its presence is guaranteed.
		if s.key != "__swap_used" {
			for _, samp := range samples {
				if _, ok := samp.vals[s.key]; ok {
					seen = true
					break
				}
			}
			if !seen {
				diagnostics = append(diagnostics, model.Diagnostic{
					SourceFile: sourcePath,
					Severity:   model.SeverityWarning,
					Message: fmt.Sprintf(
						"meminfo: declared series %q (/proc/meminfo key %q) never appeared in any sample; chart line will be flat at 0 GB",
						s.metric, s.key),
				})
			}
		}
		out := make([]model.Sample, len(samples))
		for j, samp := range samples {
			kb := samp.vals[s.key]
			out[j] = model.Sample{
				Timestamp:    samp.t,
				Measurements: map[string]float64{s.metric: kb / kbPerGB},
			}
		}
		data.Series = append(data.Series, model.MetricSeries{
			Metric:  s.metric,
			Unit:    "GB",
			Subject: "",
			Samples: out,
		})
	}
	return data, diagnostics
}
