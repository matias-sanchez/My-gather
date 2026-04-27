package findings

import (
	"strings"
	"testing"
)

func TestConfigMaxConnectionsHigh(t *testing.T) {
	cases := []struct {
		name     string
		maxConns string
		bpBytes  string
		wantSev  Severity
		wantSubs string
	}{
		// Below threshold: skipped.
		{"below_threshold", "1000", "8589934592" /*8 GiB*/, SeveritySkip, ""},
		// Big pool, big max_connections: OK.
		{"ok_big_pool", "10000", "21474836480" /*20 GiB*/, SeverityOK, "within"},
		// 6000 connections + 2 GiB pool: 333 MiB / 1k slots → WARN band (< 1 GiB but > 256 MiB).
		{"warn_under_1gib_per_1k", "6000", "2147483648" /*2 GiB*/, SeverityWarn, "below the 1 GiB"},
		// 10000 connections + 1 GiB pool: 100 MiB / 1k slots → CRIT band (< 256 MiB).
		{"crit_under_256mib_per_1k", "10000", "1073741824" /*1 GiB*/, SeverityCrit, "starve the pool"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := newBuilder().
				variable("max_connections", tc.maxConns).
				variable("innodb_buffer_pool_size", tc.bpBytes)
			got := Analyze(b.build())
			f := findByID(got, "config.max_connections_high")
			if tc.wantSev == SeveritySkip {
				if f != nil {
					t.Fatalf("expected skip, got %+v", f)
				}
				return
			}
			if f == nil {
				t.Fatalf("config.max_connections_high not present")
			}
			if f.Severity != tc.wantSev {
				t.Errorf("severity: got %v, want %v (summary: %q)", f.Severity, tc.wantSev, f.Summary)
			}
			if tc.wantSubs != "" && !strings.Contains(f.Summary, tc.wantSubs) {
				t.Errorf("summary %q missing %q", f.Summary, tc.wantSubs)
			}
		})
	}
}

func TestConfigMaxConnectionsHigh_SkipsWhenBPMissing(t *testing.T) {
	// 6000 max_connections but no innodb_buffer_pool_size variable
	// captured: rule should skip rather than fire on partial data.
	b := newBuilder().variable("max_connections", "6000")
	if findByID(Analyze(b.build()), "config.max_connections_high") != nil {
		t.Fatal("expected skip when innodb_buffer_pool_size is absent")
	}
}

func TestConfigSyncBinlogNotOne(t *testing.T) {
	t.Run("skip_when_safe", func(t *testing.T) {
		b := newBuilder().
			variable("log_bin", "ON").
			variable("sync_binlog", "1")
		if findByID(Analyze(b.build()), "config.sync_binlog_not_one") != nil {
			t.Fatal("expected skip when sync_binlog=1")
		}
	})

	t.Run("info_when_log_bin_off", func(t *testing.T) {
		b := newBuilder().
			variable("log_bin", "OFF").
			variable("sync_binlog", "0")
		f := findByID(Analyze(b.build()), "config.sync_binlog_not_one")
		if f == nil || f.Severity != SeverityInfo {
			t.Fatalf("expected Info when log_bin=OFF, got %+v", f)
		}
		if !strings.Contains(f.Summary, "log_bin = OFF") {
			t.Errorf("summary should mention log_bin: %q", f.Summary)
		}
	})

	t.Run("warn_when_zero_no_replication", func(t *testing.T) {
		b := newBuilder().
			variable("log_bin", "ON").
			variable("sync_binlog", "0").
			variable("server_id", "0") // no replication
		f := findByID(Analyze(b.build()), "config.sync_binlog_not_one")
		if f == nil || f.Severity != SeverityWarn {
			t.Fatalf("expected Warn for sync_binlog=0 standalone, got %+v", f)
		}
	})

	t.Run("crit_when_zero_with_replication", func(t *testing.T) {
		b := newBuilder().
			variable("log_bin", "ON").
			variable("sync_binlog", "0").
			variable("server_id", "1").
			variable("gtid_mode", "ON")
		f := findByID(Analyze(b.build()), "config.sync_binlog_not_one")
		if f == nil || f.Severity != SeverityCrit {
			t.Fatalf("expected Crit when replication is configured, got %+v", f)
		}
	})

	t.Run("warn_when_greater_than_one", func(t *testing.T) {
		b := newBuilder().
			variable("log_bin", "ON").
			variable("sync_binlog", "100")
		f := findByID(Analyze(b.build()), "config.sync_binlog_not_one")
		if f == nil || f.Severity != SeverityWarn {
			t.Fatalf("expected Warn for sync_binlog=100, got %+v", f)
		}
		if !strings.Contains(f.Summary, "100") {
			t.Errorf("summary should mention the value: %q", f.Summary)
		}
	})

	t.Run("skip_when_variable_absent", func(t *testing.T) {
		b := newBuilder() // no variables at all
		if findByID(Analyze(b.build()), "config.sync_binlog_not_one") != nil {
			t.Fatal("expected skip when sync_binlog absent and log_bin absent")
		}
	})
}
