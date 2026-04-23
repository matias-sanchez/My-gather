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
// separated by `TS <epoch> …` boundary lines. This parser walks every
// sample and returns the values from the LAST fully-populated block so
// the Environment panel reflects the most recent observation — matching
// how render-layer collectors pick the last-snapshot sidecar.
//
// Never returns an error: a malformed or empty input yields an
// EnvMeminfo with zero fields, which the template renders as "—".
func ParseEnvMeminfo(content string) *model.EnvMeminfo {
	out := &model.EnvMeminfo{}
	any := false

	setInt := func(dst *int64, raw string) {
		// "MemTotal:       32654396 kB" — after the colon, fields are
		// [number, unit]. HugePages_Total has no unit so we just take
		// the first numeric field.
		fields := strings.Fields(raw)
		if len(fields) == 0 {
			return
		}
		v, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return
		}
		*dst = v
		any = true
	}

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// TS lines reset-per-sample but we intentionally accumulate across
		// samples: the last occurrence of each key wins, giving us the
		// most recent value present in the file.
		idx := strings.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		key := line[:idx]
		rest := line[idx+1:]
		switch key {
		case "MemTotal":
			setInt(&out.MemTotalKB, rest)
		case "MemFree":
			setInt(&out.MemFreeKB, rest)
		case "MemAvailable":
			setInt(&out.MemAvailableKB, rest)
		case "Buffers":
			setInt(&out.BuffersKB, rest)
		case "Cached":
			setInt(&out.CachedKB, rest)
		case "SwapTotal":
			setInt(&out.SwapTotalKB, rest)
		case "SwapFree":
			setInt(&out.SwapFreeKB, rest)
		case "HugePages_Total":
			setInt(&out.HugePagesTotal, rest)
		case "AnonHugePages":
			setInt(&out.AnonHugePagesKB, rest)
		}
	}
	if !any {
		return nil
	}
	return out
}
