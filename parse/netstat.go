package parse

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// parseNetstat reads one pt-stalk -netstat file and produces a single
// NetstatSocketsSample. The file is the output of `netstat -an` (or
// `ss -tan`) captured at one snapshot moment — there is no per-row
// TS marker, so every socket in the file is attributed to the
// snapshot timestamp.
//
// Counted dimensions for v1:
//   - StateCounts: per-TCP-state histogram plus a "UDP" pseudo-state
//     aggregating every UDP socket regardless of local/foreign.
//   - RecvQNonZero / SendQNonZero: set true when any single row had
//     a non-zero Recv-Q or Send-Q. Granular per-socket queue tracking
//     is intentionally out of scope for the initial Network subview;
//     a rolled-up flag is sufficient to drive the advisor's
//     queue-saturation rule.
//
// Malformed rows are logged as diagnostics and skipped. Returns
// (nil, diagnostics) only when no usable rows were found.
func parseNetstat(r io.Reader, snapshotStart time.Time, sourcePath string) (*model.NetstatSocketsSample, []model.Diagnostic) {
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

	states := map[string]int{}
	var recvQNZ, sendQNZ bool
	any := false
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
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

		// `netstat -an` row:
		//   tcp 0 0 local foreign STATE pid/prog
		// `netstat -tulpn` can also appear; same first columns.
		// Recv-Q and Send-Q are columns 1 and 2 (0-indexed).
		if len(fields) < 5 {
			addDiag(lineNum, fmt.Sprintf("unexpected netstat row with %d fields: %q", len(fields), line))
			continue
		}
		if fields[1] != "0" {
			recvQNZ = true
		}
		if fields[2] != "0" {
			sendQNZ = true
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
		states[state]++
		any = true
	}
	if err := scanner.Err(); err != nil {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Severity:   model.SeverityWarning,
			Message:    fmt.Sprintf("netstat read: %v", err),
		})
	}
	if !any {
		return nil, diagnostics
	}
	return &model.NetstatSocketsSample{
		Timestamp:    snapshotStart,
		StateCounts:  states,
		RecvQNonZero: recvQNZ,
		SendQNonZero: sendQNZ,
	}, diagnostics
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
