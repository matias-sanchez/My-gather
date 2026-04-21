package parse

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// tsLine matches the sample-boundary marker pt-stalk writes at the top
// of each processlist snapshot:
//
//	TS 1776790303.009325313 2026-04-21 16:51:43
var tsLine = regexp.MustCompile(`^TS\s+(\d+(?:\.\d+)?)\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})`)

// parseProcesslist reads pt-stalk -processlist output (repeated
// SHOW FULL PROCESSLIST \G captures) and returns one
// ThreadStateSample per sample, each with a per-state count bucket
// (spec FR-017).
//
// Each snapshot begins with a TS line:
//
//	TS <unix.time> YYYY-MM-DD HH:MM:SS
//
// followed by vertical-format rows:
//
//	*************************** 1. row ***************************
//	         Id: 6
//	       User: …
//	        ...
//	      State: <label>
//	        ...
//
// Threads with an empty State or an unknown state bucket into "Other".
func parseProcesslist(r io.Reader, sourcePath string) (*model.ProcesslistData, []model.Diagnostic) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)

	var diagnostics []model.Diagnostic

	type sampleBuild struct {
		t          time.Time
		stateCount map[string]int
	}
	var samples []sampleBuild
	var current *sampleBuild
	statesSet := map[string]struct{}{}

	startNewSampleIfNeeded := func(t time.Time) *sampleBuild {
		if current != nil {
			samples = append(samples, *current)
		}
		current = &sampleBuild{
			t:          t,
			stateCount: map[string]int{},
		}
		return current
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if m := tsLine.FindStringSubmatch(line); m != nil {
			epoch, _ := strconv.ParseFloat(m[1], 64)
			t := time.Unix(int64(math.Floor(epoch)), 0).UTC()
			startNewSampleIfNeeded(t)
			continue
		}
		if strings.Contains(line, "*** ") && strings.Contains(line, ". row ***") {
			// Row-separator; no action (each block's State: field is what
			// we care about).
			continue
		}
		// Vertical-format field. The field name is everything up to the
		// first ":" stripped of leading space.
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "State" && current != nil {
			label := val
			if label == "" || label == "NULL" {
				label = "Other"
			}
			current.stateCount[label]++
			statesSet[label] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Severity:   model.SeverityError,
			Message:    fmt.Sprintf("processlist read: %v", err),
		})
	}
	if current != nil {
		samples = append(samples, *current)
	}

	if len(samples) == 0 {
		return nil, diagnostics
	}

	// Canonical States order: alphabetical, "Other" last.
	var states []string
	hasOther := false
	for s := range statesSet {
		if s == "Other" {
			hasOther = true
			continue
		}
		states = append(states, s)
	}
	sort.Strings(states)
	if hasOther {
		states = append(states, "Other")
	}

	// Samples sorted by timestamp ascending (they usually already are,
	// but be defensive).
	sort.SliceStable(samples, func(i, j int) bool {
		return samples[i].t.Before(samples[j].t)
	})

	out := make([]model.ThreadStateSample, len(samples))
	for i, s := range samples {
		out[i] = model.ThreadStateSample{
			Timestamp:   s.t,
			StateCounts: s.stateCount,
		}
	}

	return &model.ProcesslistData{
		ThreadStateSamples: out,
		States:             states,
	}, diagnostics
}
