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

func TestParseEnvMeminfo_TruncatedLastSampleFallsBack(t *testing.T) {
	// Last sample is truncated before MemTotal. The parser must not
	// mix keys across samples; it should fall back to the last full
	// sample rather than combine an old MemTotal with a newer MemFree.
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
MemFree:        99999999 kB
`
	got := ParseEnvMeminfo(input)
	if got == nil {
		t.Fatalf("ParseEnvMeminfo returned nil")
	}
	if got.MemTotalKB != 32654396 {
		t.Errorf("MemTotalKB: want 32654396, got %d", got.MemTotalKB)
	}
	if got.MemFreeKB != 11634540 {
		t.Errorf("MemFreeKB must come from the same (last complete) sample, not the truncated sample; got %d", got.MemFreeKB)
	}
}

func TestParseEnvMeminfo_TruncatedAfterMemTotalFallsBack(t *testing.T) {
	// Last sample got MemTotal but was truncated before swap lines.
	// Previously this sample won (MemTotal > 0 was the gate) and swap
	// rendered as 0 B — misreporting a swap-enabled host as swapless.
	// The parser must reject this sample and fall back to the prior
	// complete one.
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
`
	got := ParseEnvMeminfo(input)
	if got == nil {
		t.Fatalf("ParseEnvMeminfo returned nil")
	}
	if got.SwapTotalKB != 33554428 {
		t.Errorf("SwapTotalKB must come from the last complete sample; got %d", got.SwapTotalKB)
	}
	if got.MemFreeKB != 11634540 {
		t.Errorf("MemFreeKB must match the last complete sample; got %d", got.MemFreeKB)
	}
}

func TestParseEnvMeminfo_SwaplessHostStillComplete(t *testing.T) {
	// A swapless host legitimately emits SwapTotal: 0 kB. The core
	// "seen" bitmask must not confuse zero with missing.
	input := `TS 1769702259.004572779 2026-01-29 15:57:39
MemTotal:       32654396 kB
MemFree:        11634540 kB
MemAvailable:   28222432 kB
Buffers:           13864 kB
Cached:         16704284 kB
SwapTotal:             0 kB
SwapFree:              0 kB
HugePages_Total:       0
AnonHugePages:   1000000 kB
`
	got := ParseEnvMeminfo(input)
	if got == nil {
		t.Fatalf("ParseEnvMeminfo returned nil")
	}
	if got.MemTotalKB != 32654396 {
		t.Errorf("MemTotalKB: want 32654396, got %d", got.MemTotalKB)
	}
	if got.SwapTotalKB != 0 {
		t.Errorf("SwapTotalKB: want 0 (swapless), got %d", got.SwapTotalKB)
	}
}
