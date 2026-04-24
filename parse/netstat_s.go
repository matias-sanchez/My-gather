package parse

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// parseNetstatS reads one pt-stalk -netstat_s file and extracts the
// curated set of TCP counters we surface in the Network subview.
//
// The file is the verbatim output of `netstat -s` captured at the
// snapshot moment, with a "TS <epoch>" header on the first line added
// by pt-stalk. Each indented line under a section header is a named
// counter: either
//
//	"    12345 active connection openings"
//	"    TCPListenOverflows: 23"
//
// We recognise both forms — leading-number-then-label and
// label-colon-number. Only curated counters (netstatSCounters below)
// are extracted; every other line is skipped. This keeps the emitted
// map bounded and deterministic regardless of kernel-version
// variation in the `netstat -s` report.
//
// One call parses ONE sample; the merge across snapshots happens in
// concatNetstatS (render/concat.go) where per-sample counters become
// deltas on the collection's timeline.
func parseNetstatS(r io.Reader, snapshotStart time.Time, sourcePath string) (*model.NetstatCountersSample, []model.Diagnostic) {
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

	values := map[string]float64{}
	any := false
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		// Skip section headers; labels tie directly to counter
		// keywords so we don't need the section context.
		if !strings.HasPrefix(raw, " ") && !strings.HasPrefix(raw, "\t") {
			continue
		}

		// Try "LABEL: N"
		if idx := strings.Index(line, ":"); idx >= 0 {
			label := strings.TrimSpace(line[:idx])
			rest := strings.TrimSpace(line[idx+1:])
			toks := strings.Fields(rest)
			if len(toks) == 1 {
				if name, ok := netstatSLookup[label]; ok {
					if v, err := strconv.ParseFloat(toks[0], 64); err == nil {
						values[name] = v
						any = true
						continue
					} else {
						addDiag(lineNum, fmt.Sprintf("non-numeric value for %q: %q", label, toks[0]))
					}
				}
				continue
			}
		}

		// Try "N words of label text"
		first := strings.Fields(line)[0]
		v, err := strconv.ParseFloat(first, 64)
		if err != nil {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, first))
		if name, ok := netstatSLookup[rest]; ok {
			values[name] = v
			any = true
		}
	}
	if err := scanner.Err(); err != nil {
		diagnostics = append(diagnostics, model.Diagnostic{
			SourceFile: sourcePath,
			Severity:   model.SeverityWarning,
			Message:    fmt.Sprintf("netstat_s read: %v", err),
		})
	}
	if !any {
		return nil, diagnostics
	}
	return &model.NetstatCountersSample{Timestamp: snapshotStart, Values: values}, diagnostics
}

// netstatSLookup maps the raw `netstat -s` label (as it appears in
// the file) to the canonical name we render. We curate only counters
// that have clear operational meaning and are stable across supported
// kernels.
var netstatSLookup = func() map[string]string {
	m := map[string]string{}
	for raw, canon := range netstatSRawToCanon {
		m[raw] = canon
	}
	return m
}()

// netstatSRawToCanon declares the canonical counter names and the
// raw-label forms we translate from. Keep the ordering deterministic
// by populating netstatSCounters as the render-side display order.
var netstatSRawToCanon = map[string]string{
	// TCP section
	"active connection openings":  "tcp_active_opens",
	"passive connection openings": "tcp_passive_opens",
	"failed connection attempts":  "tcp_failed_conns",
	"connection resets received":  "tcp_resets_recv",
	"segments received":           "tcp_segs_in",
	"segments sent out":           "tcp_segs_out",
	"segments retransmitted":      "tcp_retransmits",
	"bad segments received":       "tcp_bad_segs",
	"resets sent":                 "tcp_resets_sent",
	// TcpExt section
	"TCPListenOverflows": "tcp_listen_overflows",
	"TCPBacklogDrop":     "tcp_backlog_drop",
	"SyncookiesSent":     "tcp_syncookies_sent",
	"TCPSynRetrans":      "tcp_syn_retrans",
	// UDP section
	"packets received":      "udp_pkts_in",
	"packets sent":          "udp_pkts_out",
	"packet receive errors": "udp_recv_errors",
	"receive buffer errors": "udp_rcvbuf_errors",
	"send buffer errors":    "udp_sndbuf_errors",
}

// NetstatSCounters is the canonical render order. The Network chart
// iterates this slice rather than ranging the map for deterministic
// output.
var NetstatSCounters = []string{
	// Throughput
	"tcp_segs_in",
	"tcp_segs_out",
	// Errors & re-work
	"tcp_retransmits",
	"tcp_syn_retrans",
	"tcp_resets_recv",
	"tcp_resets_sent",
	"tcp_failed_conns",
	"tcp_bad_segs",
	// Backlog saturation
	"tcp_listen_overflows",
	"tcp_backlog_drop",
	"tcp_syncookies_sent",
	// Connection rate
	"tcp_active_opens",
	"tcp_passive_opens",
	// UDP
	"udp_pkts_in",
	"udp_pkts_out",
	"udp_recv_errors",
	"udp_rcvbuf_errors",
	"udp_sndbuf_errors",
}

// netstatSDisplayName maps canonical counter names to the labels the
// chart shows. Kept separate from the lookup so we can rename on-screen
// labels without touching parsing.
var netstatSDisplayName = map[string]string{
	"tcp_segs_in":          "TCP segs in/s",
	"tcp_segs_out":         "TCP segs out/s",
	"tcp_retransmits":      "TCP retransmits/s",
	"tcp_syn_retrans":      "TCP SYN retransmits/s",
	"tcp_resets_recv":      "RST received/s",
	"tcp_resets_sent":      "RST sent/s",
	"tcp_failed_conns":     "Failed connect/s",
	"tcp_bad_segs":         "Bad segments/s",
	"tcp_listen_overflows": "Listen overflows/s",
	"tcp_backlog_drop":     "Backlog drops/s",
	"tcp_syncookies_sent":  "SYN cookies/s",
	"tcp_active_opens":     "Active opens/s",
	"tcp_passive_opens":    "Passive opens/s",
	"udp_pkts_in":          "UDP in/s",
	"udp_pkts_out":         "UDP out/s",
	"udp_recv_errors":      "UDP recv errors/s",
	"udp_rcvbuf_errors":    "UDP rcvbuf errors/s",
	"udp_sndbuf_errors":    "UDP sndbuf errors/s",
}

// NetstatSDisplayName resolves a canonical counter name to its
// display label. Exported for the render layer. Unknown names pass
// through unchanged so the chart still renders sane labels if the
// canon list ever grows out-of-sync with netstatSDisplayName.
func NetstatSDisplayName(name string) string {
	if d, ok := netstatSDisplayName[name]; ok {
		return d
	}
	return name
}

// reNetstatSTSLine keeps the netstat_s detector robust; see detector
// in version.go. Unused beyond the detector, but colocated with the
// parser for clarity.
var reNetstatSTSLine = regexp.MustCompile(`^TS \d`)
