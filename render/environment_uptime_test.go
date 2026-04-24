package render

import (
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// TestLatestUptimeSecondsAnchorsToVariablesSnapshot — regression
// guard for the P2 codex finding on bf7e8fc/55bb4f3. When the
// mysqladmin path produces nothing and the function falls back to
// the Uptime variable, the returned anchor must be the timestamp of
// the snapshot that actually supplied -variables, NOT the last
// snapshot in the collection. Otherwise StartTimeUTC drifts forward
// by the inter-snapshot gap on partial captures.
func TestLatestUptimeSecondsAnchorsToVariablesSnapshot(t *testing.T) {
	older := time.Date(2026, 4, 21, 16, 51, 41, 0, time.UTC)
	newer := older.Add(30 * time.Second)

	// Older snapshot has -variables with Uptime=12345; newer snapshot
	// has nothing parseable. The fallback path must pick Uptime from
	// the older snapshot AND return the older snapshot's timestamp.
	c := &model.Collection{
		Snapshots: []*model.Snapshot{
			{
				Timestamp: older,
				SourceFiles: map[model.Suffix]*model.SourceFile{
					model.SuffixVariables: {
						Parsed: &model.VariablesData{
							Entries: []model.VariableEntry{
								{Name: "Uptime", Value: "12345"},
							},
						},
					},
				},
			},
			{
				Timestamp:   newer,
				SourceFiles: map[model.Suffix]*model.SourceFile{
					// No -variables here — partial capture.
				},
			},
		},
	}

	uptime, ts := latestUptimeSeconds(c)
	if uptime != 12345 {
		t.Errorf("uptime: want 12345, got %d", uptime)
	}
	if !ts.Equal(older) {
		t.Errorf("anchor timestamp: want %s (variables snapshot), got %s — would shift StartTimeUTC forward by the snapshot gap",
			older.Format(time.RFC3339), ts.Format(time.RFC3339))
	}
}
