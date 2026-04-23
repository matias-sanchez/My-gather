package parse

import (
	"sort"
	"strconv"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

// ParseDFSnapshot reads a -df file contents string and returns the
// filesystems from its LAST snapshot, sorted by Use% descending. Up to
// `limit` rows are returned (limit <= 0 means "all").
//
// pt-stalk `-df` files are typically the output of `df -P -k` captured
// once at snapshot time. They may include a leading `TS …` marker and
// a header line starting with "Filesystem". Overflowed lines where the
// FS name wrapped onto its own line are rejoined with the following
// row before parsing.
func ParseDFSnapshot(content string, limit int) []model.EnvFilesystem {
	// Split into samples by TS markers; keep the last non-empty one.
	var lastBlock string
	{
		samples := splitDFSamples(content)
		for i := len(samples) - 1; i >= 0; i-- {
			if strings.TrimSpace(samples[i]) != "" {
				lastBlock = samples[i]
				break
			}
		}
	}
	if lastBlock == "" {
		lastBlock = content
	}

	lines := strings.Split(lastBlock, "\n")
	// Rejoin rows where the FS name wrapped: a line with a single field
	// that looks like an FS path followed by a line starting with whitespace.
	joined := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 1 && i+1 < len(lines) {
			next := strings.TrimRight(lines[i+1], "\r")
			joined = append(joined, line+" "+strings.TrimLeft(next, " \t"))
			i++
			continue
		}
		joined = append(joined, line)
	}

	out := make([]model.EnvFilesystem, 0, len(joined))
	for _, line := range joined {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		// Skip header.
		if fields[0] == "Filesystem" {
			continue
		}
		// Canonical df -P columns: FS, 1K-blocks, Used, Available, Use%, Mounted on...
		sizeKB, err1 := strconv.ParseInt(fields[1], 10, 64)
		usedKB, err2 := strconv.ParseInt(fields[2], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		pctRaw := strings.TrimSuffix(fields[4], "%")
		pct, err3 := strconv.Atoi(pctRaw)
		if err3 != nil {
			continue
		}
		mount := strings.Join(fields[5:], " ")
		out = append(out, model.EnvFilesystem{
			FS:     fields[0],
			Mount:  mount,
			SizeKB: sizeKB,
			UsedKB: usedKB,
			UsePct: pct,
		})
	}

	// Sort by UsePct desc, tie-break by Mount ascending for determinism.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UsePct != out[j].UsePct {
			return out[i].UsePct > out[j].UsePct
		}
		return out[i].Mount < out[j].Mount
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// splitDFSamples returns one string per TS-delimited sample block in the
// file. The leading chunk before the first TS marker (if any) is
// included as the first element so callers can fall back to it when the
// file has no TS markers at all.
func splitDFSamples(content string) []string {
	var samples []string
	lines := strings.Split(content, "\n")
	var buf strings.Builder
	flush := func() {
		if buf.Len() == 0 {
			return
		}
		samples = append(samples, buf.String())
		buf.Reset()
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "TS ") {
			flush()
			continue
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	flush()
	return samples
}
