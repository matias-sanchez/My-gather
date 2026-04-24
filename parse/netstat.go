package parse

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// parseNetstat reads one pt-stalk -netstat file and produces ONE
// NetstatSocketsSample per `TS <epoch> …` poll block. pt-stalk appends
// `netstat -antp` (or `ss -tan`) output to the same file on every
// poll, so a single file typically carries many polls; without
// splitting on TS, socket-state counts would be summed across polls
// and the Recv-Q / Send-Q flags would sticky-latch after a single
// earlier backlogged poll.
//
// Counted dimensions for v1:
//   - StateCounts: per-TCP-state histogram plus a "UDP" pseudo-state
//     aggregating every UDP socket regardless of local/foreign.
//   - RecvQNonZero / SendQNonZero: set true when any single row in
//     THAT poll had a non-zero Recv-Q or Send-Q. Granular per-socket
//     queue tracking is intentionally out of scope for the initial
//     Network subview; a rolled-up flag is sufficient to drive the
//     advisor's queue-saturation rule.
//
// Files with no TS header at all (single-poll captures or simplified
// fixtures) are treated as one sample timestamped at snapshotStart so
// older fixtures keep rendering. Malformed rows are logged as
// diagnostics and skipped. Returns (nil, diagnostics) only when no
// usable rows were found.
func parseNetstat(r io.Reader, snapshotStart time.Time, sourcePath string) ([]*model.NetstatSocketsSample, []model.Diagnostic) {
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

	var (
		samples    []*model.NetstatSocketsSample
		curTS      = snapshotStart
		curStates  map[string]int
		curRecvNZ  bool
		curSendNZ  bool
		curPopular bool // at least one recognised socket row in the block
	)
	flush := func() {
		if curStates != nil && curPopular {
			samples = append(samples, &model.NetstatSocketsSample{
				Timestamp:    curTS,
				StateCounts:  curStates,
				RecvQNonZero: curRecvNZ,
				SendQNonZero: curSendNZ,
			})
		}
		curStates = nil
		curRecvNZ = false
		curSendNZ = false
		curPopular = false
	}

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if m := reTimestampLine.FindStringSubmatch(line); m != nil {
			flush()
			curTS = epochToTime(m[1], snapshotStart)
			curStates = map[string]int{}
			continue
		}
		// Skip header and banner lines; we only care about rows
		// starting with a known protocol token.
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		proto := fields[0]
		isTCP := strings.HasPrefix(proto, "tcp")
		isUDP := strings.HasPrefix(proto, "udp")
		if !isTCP && !isUDP {
			continue
		}
		// No TS seen yet — treat the whole file as one sample
		// timestamped at snapshotStart. Keeps backward compat with
		// single-poll fixtures that omit the TS header.
		if curStates == nil {
			curStates = map[string]int{}
			curTS = snapshotStart
		}

		// `netstat -an` row:
		//   tcp 0 0 local foreign STATE pid/prog
		// `netstat -tulpn` can also appear; same first columns.
		// Recv-Q and Send-Q are columns 1 and 2 (0-indexed).
		if len(fields) < 5 {
			addDiag(lineNum, fmt.Sprintf("unexpected netstat row with %d fields: %q", len(fields), line))
			continue
		}
		if fields[1] != "0" {
			curRecvNZ = true
		}
		if fields[2] != "0" {
			curSendNZ = true
		}

		// UDP rows have no State column; bucket them as "UDP".
		var state string
		switch {
		case isUDP:
			state = "UDP"
		case len(fields) >= 6:
			state = fields[5]
		default:
			addDiag(lineNum, fmt.Sprintf("tcp row missing state column: %q", line))
			continue
		}
		curStates[state]++
		curPopular = true
	}
	flush()
	if err := scanner.Err(); err != nil {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Severity:   model.SeverityWarning,
			Message:    fmt.Sprintf("netstat read: %v", err),
		})
	}
	if len(samples) == 0 {
		return nil, diagnostics
	}
	return samples, diagnostics
}

// epochToTime parses a pt-stalk TS-line epoch token (e.g.
// "1769702259.004572779") into a UTC time.Time. Falls back to the
// supplied default on parse failure so callers can anchor a block
// that carried a malformed header without losing the whole capture.
func epochToTime(epoch string, fallback time.Time) time.Time {
	v, err := strconv.ParseFloat(epoch, 64)
	if err != nil {
		return fallback
	}
	secs := int64(math.Floor(v))
	ns := int64(math.Round((v - float64(secs)) * 1e9))
	return time.Unix(secs, ns).UTC()
}

// canonicalStateOrder returns a sort.Interface-friendly comparator for
// state labels: alphabetical, with "Other" always last when present.
// Matches the ThreadStateSample ordering convention used by
// -processlist (see model doc).
func canonicalStateOrder(states []string) []string {
	out := append([]string(nil), states...)
	sort.Slice(out, func(i, j int) bool {
		if out[i] == "Other" {
			return false
		}
		if out[j] == "Other" {
			return true
		}
		return out[i] < out[j]
	})
	return out
}
