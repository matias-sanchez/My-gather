package parse

import "testing"

func TestParseEnvMeminfo_PicksLastSampleValues(t *testing.T) {
	input := `TS 1769702259.004572779 2026-01-29 15:57:39
MemTotal:       32654396 kB
MemFree:        11634540 kB
MemAvailable:   28222432 kB
Buffers:           13864 kB
Cached:         16704284 kB
SwapTotal:      33554428 kB
SwapFree:       33554428 kB
HugePages_Total:       0
AnonHugePages:   1000000 kB
TS 1769702289.004572779 2026-01-29 15:58:09
MemTotal:       32654396 kB
MemFree:        11000000 kB
MemAvailable:   27000000 kB
Buffers:           14000 kB
Cached:         16800000 kB
SwapTotal:      33554428 kB
SwapFree:       33554428 kB
HugePages_Total:       2
AnonHugePages:   1100000 kB
`
	got := ParseEnvMeminfo(input)
	if got == nil {
		t.Fatalf("ParseEnvMeminfo returned nil")
	}
	if got.MemTotalKB != 32654396 {
		t.Errorf("MemTotalKB: want 32654396, got %d", got.MemTotalKB)
	}
	if got.MemAvailableKB != 27000000 {
		t.Errorf("MemAvailableKB should be from last sample: want 27000000, got %d", got.MemAvailableKB)
	}
	if got.HugePagesTotal != 2 {
		t.Errorf("HugePagesTotal last sample: want 2, got %d", got.HugePagesTotal)
	}
	if got.AnonHugePagesKB != 1100000 {
		t.Errorf("AnonHugePagesKB last sample: want 1100000, got %d", got.AnonHugePagesKB)
	}
}

func TestParseEnvMeminfo_EmptyReturnsNil(t *testing.T) {
	if got := ParseEnvMeminfo(""); got != nil {
		t.Errorf("empty input should yield nil; got %+v", got)
	}
}
