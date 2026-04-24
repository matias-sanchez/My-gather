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

func TestParseProcStat_TruncatedFirstSamplePicksMax(t *testing.T) {
	// First TS block was truncated after cpu1; the second block is a
	// complete four-CPU dump. LogicalCPUs must reflect the max count
	// across samples — otherwise the Environment panel misreports
	// host CPU topology as 2 cores.
	input := `TS 1769702259.004572779 2026-01-29 15:57:39
cpu  21657958 1610091 3598697 159402657 661411 495206 263199 0 0 0
cpu0 5293821 406093 892989 39989928 163997 118774 69458 0 0 0
cpu1 5347444 380927 894305 39900540 165996 133274 71661 0 0 0
TS 1769702289.004572779 2026-01-29 15:58:09
cpu  21757958 1620091 3698697 159502657 671411 505206 273199 0 0 0
cpu0 5393821 416093 902989 40089928 173997 128774 79458 0 0 0
cpu1 5447444 390927 904305 40000540 175996 143274 81661 0 0 0
cpu2 5622963 418799 918469 39838871 173971 132570 72071 0 0 0
cpu3 5593729 424271 912932 39873316 177444 130587 70007 0 0 0
btime 1769231675
`
	got := ParseProcStat(input)
	if got == nil {
		t.Fatalf("ParseProcStat returned nil")
	}
	if got.LogicalCPUs != 4 {
		t.Errorf("LogicalCPUs: want 4 (max across samples), got %d", got.LogicalCPUs)
	}
}
