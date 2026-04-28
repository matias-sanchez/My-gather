package render

import (
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

func TestSummariseProcesslistNilSafe(t *testing.T) {
	sum := summariseProcesslist(nil)
	if sum == nil {
		t.Fatal("summariseProcesslist(nil) returned nil")
	}
	if sum.SampleCount != 0 {
		t.Errorf("summariseProcesslist(nil).SampleCount = %d, want 0", sum.SampleCount)
	}
}

func TestSummariseProcesslistPreservesAvailableZeroMetrics(t *testing.T) {
	sum := summariseProcesslist(&model.ProcesslistData{
		ThreadStateSamples: []model.ThreadStateSample{
			{
				Timestamp:             time.Date(2026, 4, 21, 16, 52, 0, 0, time.UTC),
				TotalThreads:          1,
				ActiveThreads:         1,
				HasTimeMetric:         true,
				HasRowsExaminedMetric: true,
				HasRowsSentMetric:     true,
				HasQueryTextMetric:    true,
			},
		},
	})

	if !sum.HasLongestAge || sum.LongestAge != "0.0" {
		t.Errorf("LongestAge = (%t, %q), want available 0.0", sum.HasLongestAge, sum.LongestAge)
	}
	if !sum.HasPeakRowsExamined || sum.PeakRowsExamined != "0" {
		t.Errorf("PeakRowsExamined = (%t, %q), want available 0", sum.HasPeakRowsExamined, sum.PeakRowsExamined)
	}
	if !sum.HasPeakRowsSent || sum.PeakRowsSent != "0" {
		t.Errorf("PeakRowsSent = (%t, %q), want available 0", sum.HasPeakRowsSent, sum.PeakRowsSent)
	}
	if !sum.HasPeakQueryTextRows || sum.PeakQueryTextRows != "0" {
		t.Errorf("PeakQueryTextRows = (%t, %q), want available 0", sum.HasPeakQueryTextRows, sum.PeakQueryTextRows)
	}
}
