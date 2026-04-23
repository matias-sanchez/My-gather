package parse

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

var reProcStatCPU = regexp.MustCompile(`^cpu[0-9]+$`)

// ParseProcStat extracts logical-CPU count and btime (Unix epoch boot
// time) from a raw -procstat file contents string.
//
// The file is the concatenation of one or more /proc/stat dumps
// separated by `TS <epoch> …` boundary lines. btime is written by the
// kernel in every dump so we take the value from the LAST occurrence.
// Logical CPU count is the number of distinct `cpuN` (N integer) lines
// in a single sample — we take the count from the first sample that
// had any cpuN lines, which is sufficient for the Environment panel
// (physical topology is stable across a capture window).
//
// Returns nil when neither btime nor any cpuN line was found.
func ParseProcStat(content string) *model.EnvProcStat {
	out := &model.EnvProcStat{}
	any := false

	// Walk samples separated by TS lines so we can count cpuN per sample.
	var currentCPUs int
	firstSampleHadCPU := false
	flushSample := func() {
		if !firstSampleHadCPU && currentCPUs > 0 {
			out.LogicalCPUs = currentCPUs
			firstSampleHadCPU = true
			any = true
		}
		currentCPUs = 0
	}

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "TS ") {
			flushSample()
			continue
		}
		// Extract the first whitespace-delimited token.
		sp := strings.IndexByte(line, ' ')
		if sp <= 0 {
			sp = strings.IndexByte(line, '\t')
		}
		if sp <= 0 {
			continue
		}
		head := line[:sp]
		rest := strings.TrimSpace(line[sp:])
		if reProcStatCPU.MatchString(head) {
			currentCPUs++
			continue
		}
		if head == "btime" {
			// Guard against truncated captures where `btime` has no
			// numeric token: `strings.Fields(rest)` would be empty
			// and [0] would panic. Treat missing/unparseable values
			// as "uptime unavailable" (any stays false from btime's
			// perspective, but earlier cpuN lines may still mark it).
			toks := strings.Fields(rest)
			if len(toks) == 0 {
				continue
			}
			if v, err := strconv.ParseInt(toks[0], 10, 64); err == nil {
				out.BTime = v
				any = true
			}
		}
	}
	flushSample()
	if !any {
		return nil
	}
	return out
}
