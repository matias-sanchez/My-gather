package render

import (
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/model"
)

func TestBuildEnvironmentView_FormatsHumanUnits(t *testing.T) {
	sec := &model.EnvironmentSection{
		Host: &model.HostEnv{
			Hostname:        "host-01",
			LogicalCPUs:     4,
			LoadAvg:         &model.EnvTopHeader{Loadavg1: 0.41, Loadavg5: 0.49, Loadavg15: 0.64},
			OSUptimeSeconds: 5*86400 + 3*3600 + 17*60,
			Meminfo: &model.EnvMeminfo{
				MemTotalKB:     32654396,
				MemAvailableKB: 28222432,
				SwapTotalKB:    33554428,
				SwapFreeKB:     33554428,
			},
		},
		MySQL: &model.MySQLEnv{
			Version:              "8.0.42-33",
			VersionComment:       "Percona Server (GPL), Release 33, Revision 9dc49998",
			Distribution:         "Percona Server",
			InnodbBufferPoolSize: "128 MiB",
			MaxConnections:       "5000",
		},
	}
	v := buildEnvironmentView(&model.Report{EnvironmentSection: sec})
	if !v.HasHost || !v.HasMySQL {
		t.Fatalf("HasHost/HasMySQL not set: %+v", v)
	}
	if got := v.LoadAverage; got != "0.41 / 0.49 / 0.64" {
		t.Errorf("LoadAverage = %q", got)
	}
	if !strings.Contains(v.OSUptime, "5d") {
		t.Errorf("OSUptime missing day component: %q", v.OSUptime)
	}
	if !strings.Contains(v.MemTotal, "GiB") {
		t.Errorf("MemTotal should be GiB-formatted, got %q", v.MemTotal)
	}
	if v.SwapUsed != "0 B" {
		t.Errorf("SwapUsed for fully-free swap should be 0 B, got %q", v.SwapUsed)
	}
	if v.Distribution != "Percona Server" {
		t.Errorf("Distribution = %q", v.Distribution)
	}
	if v.BufferPoolSize != "128 MiB" {
		t.Errorf("BufferPoolSize = %q", v.BufferPoolSize)
	}
	if v.MaxConnections != "5000" {
		t.Errorf("MaxConnections = %q", v.MaxConnections)
	}
}

func TestInferDistribution(t *testing.T) {
	cases := []struct {
		name                    string
		wsrep, size, comment, v string
		want                    string
	}{
		{"galera", "ON", "3", "Percona XtraDB Cluster", "8.0.32-24.2", "Percona XtraDB Cluster (Galera)"},
		{"percona", "OFF", "0", "Percona Server (GPL), Release 33, Revision 9dc49998", "8.0.42-33", "Percona Server"},
		{"mariadb comment", "OFF", "0", "MariaDB Server", "10.4.30-MariaDB", "MariaDB"},
		{"mariadb version only", "OFF", "0", "", "10.4.30-MariaDB", "MariaDB"},
		{"community fallback", "OFF", "0", "MySQL Community Server - GPL", "8.0.36", "MySQL Community"},
		{"empty", "", "", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := inferDistribution(c.wsrep, c.size, c.comment, c.v)
			if got != c.want {
				t.Errorf("inferDistribution(%q,%q,%q,%q)=%q want %q",
					c.wsrep, c.size, c.comment, c.v, got, c.want)
			}
		})
	}
}
