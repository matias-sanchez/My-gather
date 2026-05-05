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
	addDiag := func(line int, sev model.Severity, msg string) {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Location:   fmt.Sprintf("line %d", line),
			Severity:   sev,
			Message:    msg,
		})
	}

	// pendingSample buffers one TS block's counts until the end of
	// the file. ESTAB classification is deferred because ss -tan
	// (TCP) and ss -uan (UDP) share the same row shape — we need
	// evidence collected across the WHOLE file (not just the current
	// block) to decide which bucket those ESTAB rows belong to.
	// Otherwise an ss -uan capture where every block contains only
	// connected UDP sockets (all ESTAB, no UNCONN) would be misread
	// as TCP.
	type pendingSample struct {
		ts                 time.Time
		states             map[string]int
		recvNZ             bool
		sendNZ             bool
		pendingESTAB       int
		pendingESTABRecvNZ bool
		pendingESTABSendNZ bool
		popular            bool
	}

	var (
		pending    []pendingSample
		curTS      = snapshotStart
		curStates  map[string]int
		curRecvNZ  bool
		curSendNZ  bool
		curPopular bool // at least one recognised socket row in the block

		// Per-block pending ESTAB; resolved in a final pass after all
		// blocks are read so file-level evidence (below) is available.
		pendingESTAB       int
		pendingESTABRecvNZ bool
		pendingESTABSendNZ bool

		// FILE-level fingerprint evidence for the state-first shape.
		// ss -uan never emits TCP-only states; ss -tan never emits
		// UNCONN. When either fingerprint appears anywhere in the
		// file we can classify all deferred ESTAB rows accordingly.
		fileSawUNCONN  bool
		fileSawTCPOnly bool

		sawTS                  bool
		usedImplicitSampleTime bool
		pendingESTABRows       int
		firstPendingESTABLine  int
	)
	ensureSample := func(line int) {
		if curStates == nil {
			curStates = map[string]int{}
			curTS = snapshotStart
			if !sawTS && !usedImplicitSampleTime {
				addDiag(line, model.SeverityInfo,
					"netstat: no TS marker found; using snapshot start for single-sample compatibility")
				usedImplicitSampleTime = true
			}
		}
	}
	flush := func() {
		if curStates == nil && pendingESTAB == 0 {
			return
		}
		pending = append(pending, pendingSample{
			ts:                 curTS,
			states:             curStates,
			recvNZ:             curRecvNZ,
			sendNZ:             curSendNZ,
			pendingESTAB:       pendingESTAB,
			pendingESTABRecvNZ: pendingESTABRecvNZ,
			pendingESTABSendNZ: pendingESTABSendNZ,
			popular:            curPopular,
		})
		curStates = nil
		curRecvNZ = false
		curSendNZ = false
		curPopular = false
		pendingESTAB = 0
		pendingESTABRecvNZ = false
		pendingESTABSendNZ = false
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
			sawTS = true
			var ok bool
			curTS, ok = epochToTime(m[1], snapshotStart)
			if !ok {
				addDiag(lineNum, model.SeverityWarning,
					"netstat: malformed TS epoch; using snapshot start")
			}
			curStates = map[string]int{}
			continue
		}
		// We recognise three row shapes:
		//
		//   1. `netstat -an` / `netstat -antp`:
		//         tcp 0 0 local foreign STATE pid/prog
		//      → col[0] = "tcp"/"tcp6"/"udp"/"udp6"
		//
		//   2. `ss -nap`:
		//         tcp LISTEN 0 128 local foreign users:("sshd",pid=1)
		//      → col[0] = proto, col[1] = state, col[2]/[3] = q
		//
		//   3. `ss -tan` / `ss -uan`:
		//         LISTEN 0 128 local foreign
		//      → col[0] = state (proto implied), col[1]/[2] = q
		//
		// We sniff the shape from the first token and fall through to
		// the right branch. Unknown first tokens are skipped.
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		var (
			state              string
			tcpRowNoState      bool
			recvQIdx, sendQIdx = 1, 2
		)
		first := fields[0]
		switch {
		case strings.HasPrefix(first, "tcp"):
			// ss -nap: "tcp LISTEN 0 128 ..." — state at col[1], queues
			// at col[2]/col[3].
			// netstat -an: "tcp 0 0 local foreign STATE pid/prog" —
			// queues at col[1]/col[2], state at col[5].
			if canon, _, ok := normalizeSSState(fields[1]); ok {
				state = canon
				recvQIdx, sendQIdx = 2, 3
			} else if len(fields) >= 6 {
				state = fields[5]
			} else {
				tcpRowNoState = true
			}
		case strings.HasPrefix(first, "udp"):
			// ss -nap/-uap: "udp UNCONN 0 0 ..." — queues at col[2]/col[3].
			// netstat -an: "udp 0 0 ..." — queues at col[1]/col[2]. UDP
			// has no TCP state column so the bucket is always "UDP".
			if _, _, ok := normalizeSSState(fields[1]); ok {
				recvQIdx, sendQIdx = 2, 3
			}
			state = "UDP"
		default:
			// `ss -tan` / `ss -uan` rows — first column is the state,
			// queues are col[1]/col[2] (already the default).
			canon, _, ok := normalizeSSState(first)
			if !ok {
				continue // not a socket row (header, blank, etc.)
			}
			// Defer ESTAB — it's ambiguous across ss -tan (TCP) and
			// ss -uan (connected UDP). Record queue-flag evidence now
			// so the flush-time bucketing still gets it.
			if first == "ESTAB" || first == "ESTABLISHED" {
				ensureSample(lineNum)
				if len(fields) > sendQIdx {
					if fields[recvQIdx] != "0" {
						pendingESTABRecvNZ = true
					}
					if fields[sendQIdx] != "0" {
						pendingESTABSendNZ = true
					}
				}
				pendingESTAB++
				pendingESTABRows++
				if firstPendingESTABLine == 0 {
					firstPendingESTABLine = lineNum
				}
				continue
			}
			// Track file-flavour evidence for ESTAB disambiguation.
			if first == "UNCONN" {
				fileSawUNCONN = true
			} else {
				// Every other token normalizeSSState accepts (LISTEN,
				// TIME-WAIT, CLOSE-WAIT, FIN-WAIT-*, SYN-*, LAST-ACK,
				// CLOSING, CLOSED) is TCP-only.
				fileSawTCPOnly = true
			}
			state = canon
		}

		// No TS seen yet — treat the whole file as one sample
		// timestamped at snapshotStart. Keeps backward compat with
		// single-poll fixtures that omit the TS header.
		ensureSample(lineNum)

		if len(fields) <= sendQIdx {
			addDiag(lineNum, model.SeverityWarning, fmt.Sprintf("netstat row missing Recv-Q/Send-Q: %q", line))
			continue
		}
		if fields[recvQIdx] != "0" {
			curRecvNZ = true
		}
		if fields[sendQIdx] != "0" {
			curSendNZ = true
		}
		if tcpRowNoState {
			addDiag(lineNum, model.SeverityWarning, fmt.Sprintf("tcp row missing state column: %q", line))
			continue
		}
		if state == "" {
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

	// Finalise deferred ESTAB counts against file-level evidence.
	// UNCONN-only across the whole file → ss -uan → ESTAB goes to
	// the UDP bucket. Any TCP-only token anywhere in the file → ss
	// -tan → ESTAB is TCP ESTABLISHED. When neither fingerprint is
	// present (e.g. an all-ESTAB capture on either flavour), we
	// default to TCP for backward compatibility — uncommon on a
	// real host; if it matters the capture tool should be fixed to
	// emit proto-prefixed rows.
	estabBucket := "ESTABLISHED"
	if fileSawUNCONN && !fileSawTCPOnly {
		estabBucket = "UDP"
	}
	if pendingESTABRows > 0 && !fileSawUNCONN && !fileSawTCPOnly {
		addDiag(firstPendingESTABLine, model.SeverityWarning,
			fmt.Sprintf("netstat: %d state-first ESTAB row(s) could not be disambiguated as tcp or udp; defaulted to ESTABLISHED for compatibility", pendingESTABRows))
	}
	samples := make([]*model.NetstatSocketsSample, 0, len(pending))
	for _, p := range pending {
		popular := p.popular
		states := p.states
		recvNZ := p.recvNZ
		sendNZ := p.sendNZ
		if p.pendingESTAB > 0 {
			if states == nil {
				states = map[string]int{}
			}
			states[estabBucket] += p.pendingESTAB
			if p.pendingESTABRecvNZ {
				recvNZ = true
			}
			if p.pendingESTABSendNZ {
				sendNZ = true
			}
			popular = true
		}
		if states == nil || !popular {
			continue
		}
		samples = append(samples, &model.NetstatSocketsSample{
			Timestamp:    p.ts,
			StateCounts:  states,
			RecvQNonZero: recvNZ,
			SendQNonZero: sendNZ,
		})
	}
	if len(samples) == 0 {
		return nil, diagnostics
	}
	return samples, diagnostics
}

// epochToTime parses a pt-stalk TS-line epoch token (e.g.
// "1769702259.004572779") into a UTC time.Time. The boolean return is
// false when the supplied default was used.
func epochToTime(epoch string, fallback time.Time) (time.Time, bool) {
	v, err := strconv.ParseFloat(epoch, 64)
	const maxUnixSeconds = float64(1<<63 - 1)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > maxUnixSeconds {
		return fallback, false
	}
	secs := int64(math.Floor(v))
	ns := int64(math.Round((v - float64(secs)) * 1e9))
	if ns >= 1e9 {
		secs++
		ns -= 1e9
	}
	return time.Unix(secs, ns).UTC(), true
}

// normalizeSSState maps an `ss`-style state token (e.g. "ESTAB",
// "TIME-WAIT", "UNCONN") onto the netstat-style canonical label used
// by the rest of the pipeline ("ESTABLISHED", "TIME_WAIT", "UDP").
// Returns (canonical, isUDP, ok). Unknown tokens return ok=false so
// callers can decide whether to skip the row as a banner/header.
func normalizeSSState(tok string) (string, bool, bool) {
	switch tok {
	case "ESTAB":
		return "ESTABLISHED", false, true
	case "ESTABLISHED":
		return "ESTABLISHED", false, true
	case "TIME-WAIT":
		return "TIME_WAIT", false, true
	case "TIME_WAIT":
		return "TIME_WAIT", false, true
	case "CLOSE-WAIT":
		return "CLOSE_WAIT", false, true
	case "CLOSE_WAIT":
		return "CLOSE_WAIT", false, true
	case "FIN-WAIT-1":
		return "FIN_WAIT1", false, true
	case "FIN_WAIT1":
		return "FIN_WAIT1", false, true
	case "FIN-WAIT-2":
		return "FIN_WAIT2", false, true
	case "FIN_WAIT2":
		return "FIN_WAIT2", false, true
	case "SYN-SENT":
		return "SYN_SENT", false, true
	case "SYN_SENT":
		return "SYN_SENT", false, true
	case "SYN-RECV":
		return "SYN_RECV", false, true
	case "SYN_RECV":
		return "SYN_RECV", false, true
	case "LAST-ACK":
		return "LAST_ACK", false, true
	case "LAST_ACK":
		return "LAST_ACK", false, true
	case "LISTEN":
		return "LISTEN", false, true
	case "CLOSING":
		return "CLOSING", false, true
	case "CLOSED":
		return "CLOSED", false, true
	// UDP has no TCP state — `ss -uan` reports "UNCONN" for every
	// unconnected socket. Map to the same bucket our netstat path
	// uses so mixed captures combine cleanly.
	case "UNCONN":
		return "UDP", true, true
	}
	return "", false, false
}

// canonicalStateOrder returns a sort.Interface-friendly comparator for
// state labels: alphabetical, with "Other" always last when present.
// Matches the ThreadStateSample ordering convention used by
// -processlist (see model doc).
func canonicalStateOrder(states []string) []string {
	out := append([]string(nil), states...)
	// SliceStable so the determinism guarantee is explicit. Today the
	// input is a key-set (no duplicates) so Slice vs SliceStable is a
	// no-op in practice, but any future path that admits duplicates
	// would need the stable order to keep renders byte-identical.
	sort.SliceStable(out, func(i, j int) bool {
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
