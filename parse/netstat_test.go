package parse

import (
	"strings"
	"testing"
	"time"
)

func TestParseNetstat_EmitsOneSamplePerTSBlock(t *testing.T) {
	// Two polls in one file. The old parser summed these into a
	// single sample inflating TIME_WAIT from 2 to 3 and leaving
	// RecvQNonZero sticky-set from the first poll even though the
	// second poll had clean queues.
	input := `TS 1769702259.000000000 2026-01-29 15:57:39
tcp        0      0 10.0.0.1:1234          10.0.0.2:8080           TIME_WAIT   -
tcp       42      0 10.0.0.1:5678          10.0.0.3:80             ESTABLISHED 1/foo
tcp        0      0 10.0.0.1:1235          10.0.0.2:8080           TIME_WAIT   -
TS 1769702289.000000000 2026-01-29 15:58:09
tcp        0      0 10.0.0.1:9999          10.0.0.2:8080           TIME_WAIT   -
tcp        0      0 0.0.0.0:22             0.0.0.0:*               LISTEN      2/bar
`
	snapshotStart := time.Unix(1769702259, 0).UTC()
	samples, diags := parseNetstat(strings.NewReader(input), snapshotStart, "test-netstat")
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if len(samples) != 2 {
		t.Fatalf("want 2 samples (one per TS block), got %d", len(samples))
	}
	if samples[0].StateCounts["TIME_WAIT"] != 2 {
		t.Errorf("first poll TIME_WAIT: want 2, got %d", samples[0].StateCounts["TIME_WAIT"])
	}
	if samples[0].StateCounts["ESTABLISHED"] != 1 {
		t.Errorf("first poll ESTABLISHED: want 1, got %d", samples[0].StateCounts["ESTABLISHED"])
	}
	if !samples[0].RecvQNonZero {
		t.Errorf("first poll RecvQNonZero: want true (row had Recv-Q=42)")
	}
	if samples[1].StateCounts["TIME_WAIT"] != 1 {
		t.Errorf("second poll TIME_WAIT: want 1, got %d", samples[1].StateCounts["TIME_WAIT"])
	}
	if samples[1].StateCounts["LISTEN"] != 1 {
		t.Errorf("second poll LISTEN: want 1, got %d", samples[1].StateCounts["LISTEN"])
	}
	// RecvQNonZero must not carry over from poll 1 — a clean second
	// poll should read as clean.
	if samples[1].RecvQNonZero {
		t.Errorf("second poll RecvQNonZero: sticky-latched across TS boundary")
	}
	// Timestamps come from each TS header, not snapshotStart.
	if samples[0].Timestamp.Unix() != 1769702259 {
		t.Errorf("first sample timestamp: want epoch 1769702259, got %d", samples[0].Timestamp.Unix())
	}
	if samples[1].Timestamp.Unix() != 1769702289 {
		t.Errorf("second sample timestamp: want epoch 1769702289, got %d", samples[1].Timestamp.Unix())
	}
}

func TestParseNetstat_NoTSFallsBackToSnapshotStart(t *testing.T) {
	// Fixture-style single-sample file with no TS header — stays
	// one sample anchored at snapshotStart for backward compat.
	input := `Active Internet connections (servers and established)
Proto Recv-Q Send-Q Local Address  Foreign Address State
tcp        0      0 0.0.0.0:22     0.0.0.0:*       LISTEN
udp        0      0 0.0.0.0:53     0.0.0.0:*
`
	snapshotStart := time.Unix(1769702259, 0).UTC()
	samples, _ := parseNetstat(strings.NewReader(input), snapshotStart, "test-netstat")
	if len(samples) != 1 {
		t.Fatalf("want 1 sample, got %d", len(samples))
	}
	if samples[0].Timestamp != snapshotStart {
		t.Errorf("no-TS fallback: want snapshotStart %v, got %v", snapshotStart, samples[0].Timestamp)
	}
	if samples[0].StateCounts["LISTEN"] != 1 || samples[0].StateCounts["UDP"] != 1 {
		t.Errorf("unexpected counts: %+v", samples[0].StateCounts)
	}
}

func TestParseNetstatS_EmitsOneSamplePerTSBlock(t *testing.T) {
	// Two polls; counters monotonically increase across polls. The
	// old parser overwrote values as it walked the file and returned
	// only the final reading, so concatNetstatS saw a single point
	// per file and could not compute per-poll deltas.
	input := `TS 1769702259.000000000 2026-01-29 15:57:39
Tcp:
    50 active connection openings
    30 passive connection openings
    5000 segments received
    4500 segments sent out
    10 segments retransmitted
Udp:
    120 packets received
TS 1769702289.000000000 2026-01-29 15:58:09
Tcp:
    60 active connection openings
    35 passive connection openings
    6000 segments received
    5400 segments sent out
    12 segments retransmitted
Udp:
    130 packets received
`
	snapshotStart := time.Unix(1769702259, 0).UTC()
	samples, _ := parseNetstatS(strings.NewReader(input), snapshotStart, "test-netstat_s")
	if len(samples) != 2 {
		t.Fatalf("want 2 samples (one per TS block), got %d", len(samples))
	}
	if samples[0].Values["tcp_segs_in"] != 5000 {
		t.Errorf("poll 1 tcp_segs_in: want 5000, got %v", samples[0].Values["tcp_segs_in"])
	}
	if samples[1].Values["tcp_segs_in"] != 6000 {
		t.Errorf("poll 2 tcp_segs_in: want 6000, got %v", samples[1].Values["tcp_segs_in"])
	}
	if samples[0].Timestamp.Unix() != 1769702259 {
		t.Errorf("poll 1 timestamp: want epoch 1769702259, got %d", samples[0].Timestamp.Unix())
	}
	if samples[1].Timestamp.Unix() != 1769702289 {
		t.Errorf("poll 2 timestamp: want epoch 1769702289, got %d", samples[1].Timestamp.Unix())
	}
}

func TestParseNetstat_ParsesSSTANRows(t *testing.T) {
	// `ss -tan` captures: first column is the connection state, not
	// the proto. The old parser gated on "first column starts with
	// tcp/udp" and silently dropped every row, producing an empty
	// Network sockets view.
	input := `State      Recv-Q Send-Q Local Address:Port         Peer Address:Port        Process
LISTEN     0      128    0.0.0.0:22                 0.0.0.0:*
ESTAB      0      0      10.0.0.1:5678              10.0.0.2:80
ESTAB      42     0      10.0.0.1:5679              10.0.0.2:443
TIME-WAIT  0      0      10.0.0.1:1234              10.0.0.2:8080
`
	snapshotStart := time.Unix(1769702259, 0).UTC()
	samples, _ := parseNetstat(strings.NewReader(input), snapshotStart, "test-ss")
	if len(samples) != 1 {
		t.Fatalf("want 1 sample, got %d", len(samples))
	}
	s := samples[0]
	if s.StateCounts["LISTEN"] != 1 {
		t.Errorf("LISTEN: want 1, got %d", s.StateCounts["LISTEN"])
	}
	// ss "ESTAB" must normalise to "ESTABLISHED" so mixed ss/netstat
	// captures combine onto the same bucket.
	if s.StateCounts["ESTABLISHED"] != 2 {
		t.Errorf("ESTABLISHED: want 2 (two ESTAB rows), got %d", s.StateCounts["ESTABLISHED"])
	}
	// "TIME-WAIT" must normalise to "TIME_WAIT".
	if s.StateCounts["TIME_WAIT"] != 1 {
		t.Errorf("TIME_WAIT: want 1, got %d", s.StateCounts["TIME_WAIT"])
	}
	if !s.RecvQNonZero {
		t.Errorf("RecvQNonZero: want true (one ESTAB row had Recv-Q=42)")
	}
}

func TestParseNetstat_ParsesSSNAPRows(t *testing.T) {
	// `ss -nap`: proto is col[0] AND state is col[1]. Must parse the
	// state from col[1], not fall through to col[5] like netstat.
	// Also: queue columns shift to col[2]/col[3] — reading them from
	// col[1]/col[2] would treat the state token (e.g. "LISTEN") as
	// Recv-Q and sticky-set RecvQNonZero on every capture.
	// All Recv-Q / Send-Q columns are 0 so the no-false-positive
	// queue-flag assertion below is unambiguous. (Real ss -nap often
	// shows somaxconn in Send-Q for LISTEN rows, which is distinct
	// from an actual backlog but would still register as non-zero.)
	input := `Netid State   Recv-Q Send-Q Local Address:Port    Peer Address:Port
tcp    LISTEN  0      0      0.0.0.0:22            0.0.0.0:*
tcp    ESTAB   0      0      10.0.0.1:5678         10.0.0.2:80
udp    UNCONN  0      0      0.0.0.0:68            0.0.0.0:*
`
	snapshotStart := time.Unix(1769702259, 0).UTC()
	samples, _ := parseNetstat(strings.NewReader(input), snapshotStart, "test-ss-nap")
	if len(samples) != 1 {
		t.Fatalf("want 1 sample, got %d", len(samples))
	}
	s := samples[0]
	if s.StateCounts["LISTEN"] != 1 {
		t.Errorf("LISTEN: want 1, got %d", s.StateCounts["LISTEN"])
	}
	if s.StateCounts["ESTABLISHED"] != 1 {
		t.Errorf("ESTABLISHED: want 1, got %d", s.StateCounts["ESTABLISHED"])
	}
	if s.StateCounts["UDP"] != 1 {
		t.Errorf("UDP: want 1, got %d", s.StateCounts["UDP"])
	}
	// Every queue column in the fixture is "0"; RecvQNonZero must
	// stay false. If the parser was still reading col[1] as Recv-Q
	// it would see the state token "LISTEN" and flip this to true.
	if s.RecvQNonZero {
		t.Errorf("RecvQNonZero: parser read state token as Recv-Q")
	}
	if s.SendQNonZero {
		t.Errorf("SendQNonZero: parser read state token as Send-Q")
	}
}

func TestParseNetstat_SSNAPDetectsRealQueueBacklog(t *testing.T) {
	// With ss-nap queue indices correct, a genuinely backlogged row
	// still sets RecvQNonZero / SendQNonZero — i.e., the fix is
	// surgical (shifted columns), not a blanket suppression.
	input := `tcp    ESTAB   99     0      10.0.0.1:5678    10.0.0.2:80
udp    UNCONN  0      17     0.0.0.0:514      0.0.0.0:*
`
	snapshotStart := time.Unix(1769702259, 0).UTC()
	samples, _ := parseNetstat(strings.NewReader(input), snapshotStart, "test-ss-nap-backlog")
	if len(samples) != 1 {
		t.Fatalf("want 1 sample, got %d", len(samples))
	}
	if !samples[0].RecvQNonZero {
		t.Errorf("RecvQNonZero: want true (ESTAB row has Recv-Q=99)")
	}
	if !samples[0].SendQNonZero {
		t.Errorf("SendQNonZero: want true (UNCONN row has Send-Q=17)")
	}
}

func TestParseNetstatS_NoTSFallsBackToSnapshotStart(t *testing.T) {
	input := `Tcp:
    50 active connection openings
    5000 segments received
`
	snapshotStart := time.Unix(1769702259, 0).UTC()
	samples, _ := parseNetstatS(strings.NewReader(input), snapshotStart, "test-netstat_s")
	if len(samples) != 1 {
		t.Fatalf("want 1 sample, got %d", len(samples))
	}
	if samples[0].Timestamp != snapshotStart {
		t.Errorf("no-TS fallback: want snapshotStart %v, got %v", snapshotStart, samples[0].Timestamp)
	}
	if samples[0].Values["tcp_segs_in"] != 5000 {
		t.Errorf("unexpected value: %v", samples[0].Values["tcp_segs_in"])
	}
}
