package render

import "testing"

func TestSummariseProcesslistNilSafe(t *testing.T) {
	sum := summariseProcesslist(nil)
	if sum == nil {
		t.Fatal("summariseProcesslist(nil) returned nil")
	}
	if sum.SampleCount != 0 {
		t.Errorf("summariseProcesslist(nil).SampleCount = %d, want 0", sum.SampleCount)
	}
}
