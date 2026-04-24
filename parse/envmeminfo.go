package parse

import (
	"strconv"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

// ParseEnvMeminfo extracts the scalar fields used by the Environment
// panel from a raw -meminfo file contents string.
//
// The file is the concatenation of one or more /proc/meminfo dumps
// separated by `TS <epoch> …` boundary lines. This parser scans each
// sample INDEPENDENTLY and returns the newest sample whose MemTotal is
// set (the canonical indicator of a non-truncated /proc/meminfo dump),
// falling back to the newest sample with any keys when none qualifies.
// Keys from different samples are never mixed: if the last sample is
// truncated past MemTotal the render path sees the prior complete
// sample rather than a Frankenstein combination.
//
// Never returns an error: a malformed or empty input yields nil,
// which the template renders as "—".
func ParseEnvMeminfo(content string) *model.EnvMeminfo {
	var (
		samples []*model.EnvMeminfo
		cur     *model.EnvMeminfo
		curAny  bool
	)
	flush := func() {
		if cur != nil && curAny {
			samples = append(samples, cur)
		}
		cur = nil
		curAny = false
	}
	setInt := func(dst *int64, rest string) {
		// "MemTotal:       32654396 kB" — after the colon, fields are
		// [number, unit]. HugePages_Total has no unit so we just take
		// the first numeric field.
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return
		}
		v, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return
		}
		*dst = v
		curAny = true
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "TS ") {
			flush()
			cur = &model.EnvMeminfo{}
			continue
		}
		if cur == nil {
			// Input without any TS boundaries — treat the whole file
			// as a single sample.
			cur = &model.EnvMeminfo{}
		}
		idx := strings.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		key := line[:idx]
		rest := line[idx+1:]
		switch key {
		case "MemTotal":
			setInt(&cur.MemTotalKB, rest)
		case "MemFree":
			setInt(&cur.MemFreeKB, rest)
		case "MemAvailable":
			setInt(&cur.MemAvailableKB, rest)
		case "Buffers":
			setInt(&cur.BuffersKB, rest)
		case "Cached":
			setInt(&cur.CachedKB, rest)
		case "SwapTotal":
			setInt(&cur.SwapTotalKB, rest)
		case "SwapFree":
			setInt(&cur.SwapFreeKB, rest)
		case "HugePages_Total":
			setInt(&cur.HugePagesTotal, rest)
		case "AnonHugePages":
			setInt(&cur.AnonHugePagesKB, rest)
		}
	}
	flush()

	// Prefer the newest sample whose MemTotal is populated. Samples
	// that are truncated before the MemTotal line are treated as
	// unusable; the prior full sample is returned instead.
	for i := len(samples) - 1; i >= 0; i-- {
		if samples[i].MemTotalKB > 0 {
			return samples[i]
		}
	}
	if len(samples) == 0 {
		return nil
	}
	return samples[len(samples)-1]
}
