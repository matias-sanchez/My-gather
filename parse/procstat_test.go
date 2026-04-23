package parse

import "testing"

func TestParseProcStat_CountsCPUAndReadsBTime(t *testing.T) {
	input := `TS 1769702259.004572779 2026-01-29 15:57:39
cpu  21657958 1610091 3598697 159402657 661411 495206 263199 0 0 0
cpu0 5293821 406093 892989 39989928 163997 118774 69458 0 0 0
cpu1 5347444 380927 894305 39900540 165996 133274 71661 0 0 0
cpu2 5522963 408799 908469 39738871 163971 122570 62071 0 0 0
cpu3 5493729 414271 902932 39773316 167444 120587 60007 0 0 0
ctxt 1377460535
btime 1769231675
processes 1248205
`
	got := ParseProcStat(input)
	if got == nil {
		t.Fatalf("ParseProcStat returned nil")
	}
	if got.LogicalCPUs != 4 {
		t.Errorf("LogicalCPUs: want 4, got %d", got.LogicalCPUs)
	}
	if got.BTime != 1769231675 {
		t.Errorf("BTime: want 1769231675, got %d", got.BTime)
	}
}

func TestParseProcStat_EmptyReturnsNil(t *testing.T) {
	if got := ParseProcStat(""); got != nil {
		t.Errorf("expected nil for empty input, got %+v", got)
	}
}
