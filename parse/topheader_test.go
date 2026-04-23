package parse

import "testing"

func TestParseTopHeader_ExtractsLoadAverages(t *testing.T) {
	input := `top - 15:57:38 up 5 days, 10:43, 10 users,  load average: 0.41, 0.49, 0.64
Tasks: 310 total,   3 running, 307 sleeping,   0 stopped,   0 zombie
`
	got := ParseTopHeader(input)
	if got == nil {
		t.Fatalf("ParseTopHeader returned nil")
	}
	if got.Loadavg1 != 0.41 || got.Loadavg5 != 0.49 || got.Loadavg15 != 0.64 {
		t.Errorf("loadavg mismatch: got %+v", got)
	}
}

func TestParseTopHeader_NoHeaderReturnsNil(t *testing.T) {
	if got := ParseTopHeader("nothing here\n"); got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}
