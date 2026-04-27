package findings

import (
	"testing"
)

func TestThreadMaxQueueDepth(t *testing.T) {
	cases := []struct {
		name     string
		peak     float64
		maxConns string
		wantSev  Severity
	}{
		{"skip_when_zero", 0, "500", SeveritySkip},
		{"ok_low_peak", 50, "500", SeverityOK},      // peak < 250 (max/2)
		{"warn_half", 260, "500", SeverityWarn},     // peak >= 250 (max/2) but < 450 (max*0.9)
		{"crit_near_max", 460, "500", SeverityCrit}, // peak >= 450
		// Fallback path (max_connections absent): warn at 50, crit at 200.
		{"fallback_warn", 75, "", SeverityWarn},
		{"fallback_crit", 250, "", SeverityCrit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := newBuilder().gaugeMax("Threads_running", tc.peak)
			if tc.maxConns != "" {
				b = b.variable("max_connections", tc.maxConns)
			}
			got := Analyze(b.build())
			f := findByID(got, "thread.max_queue_depth")
			if tc.wantSev == SeveritySkip {
				if f != nil {
					t.Fatalf("expected skip, got %+v", f)
				}
				return
			}
			if f == nil {
				t.Fatalf("rule missing")
			}
			if f.Severity != tc.wantSev {
				t.Errorf("severity: got %v, want %v (summary: %q)", f.Severity, tc.wantSev, f.Summary)
			}
		})
	}
}
