package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/parse"
)

// buildEnvironmentSection constructs the render-ready EnvironmentSection
// from a Collection. Consumes Collection.RawEnvSidecars — a map of
// suffix → raw file contents populated once at parse.Discover time — so
// the render layer is a pure function of the in-memory model (no
// filesystem reads here; determinism contract preserved).
//
// Host panel is nil when no host-side sidecar produced any signal; MySQL
// panel is nil when no -variables snapshot parsed. Both nil means the
// Environment section will render as "missing" in the badge.
func buildEnvironmentSection(c *model.Collection) *model.EnvironmentSection {
	if c == nil {
		return nil
	}
	sec := &model.EnvironmentSection{}
	contents := c.RawEnvSidecars // may be nil; all lookups below are nil-safe

	// ----- Host panel ---------------------------------------------
	host := &model.HostEnv{}
	populated := false
	if s := contents["hostname"]; s != "" {
		if h := parse.ParseEnvHostname(s); h != "" {
			host.Hostname = h
			populated = true
		}
	}
	if host.Hostname == "" && c.Hostname != "" {
		host.Hostname = c.Hostname
		populated = true
	}
	if s := contents["sysctl"]; s != "" {
		keys := parse.ParseSysctl(s)
		// Kernel line: osrelease + version when available.
		parts := []string{}
		if v := keys["kernel.osrelease"]; v != "" {
			parts = append(parts, v)
		}
		if v := keys["kernel.version"]; v != "" {
			parts = append(parts, v)
		}
		if k := strings.Join(parts, " "); k != "" {
			host.Kernel = k
			populated = true
		}
		// Architecture: tail of kernel.osrelease after last dot.
		if v := keys["kernel.osrelease"]; v != "" {
			if idx := strings.LastIndex(v, "."); idx >= 0 && idx+1 < len(v) {
				host.Architecture = v[idx+1:]
				populated = true
			}
		}
		// OS best-effort via crypto.fips_name (RHEL/Rocky/OL hint).
		if v := keys["crypto.fips_name"]; v != "" {
			host.OS = v
			populated = true
		}
		if v := keys["vm.swappiness"]; v != "" {
			host.Swappiness = v
			populated = true
		}
		if v := keys["vm.dirty_ratio"]; v != "" {
			host.DirtyRatio = v
			populated = true
		}
		if v := keys["vm.dirty_background_ratio"]; v != "" {
			host.DirtyBackgroundRatio = v
			populated = true
		}
		if v := keys["fs.file-max"]; v != "" {
			host.FileMax = v
			populated = true
		}
	}
	// Secondary OS hint: grep -output for distro strings if we don't have one.
	if host.OS == "" {
		if s := contents["output"]; s != "" {
			if guess := guessOSFromOutput(s); guess != "" {
				host.OS = guess
				populated = true
			}
		}
	}
	if s := contents["procstat"]; s != "" {
		if ps := parse.ParseProcStat(s); ps != nil {
			if ps.LogicalCPUs > 0 {
				host.LogicalCPUs = ps.LogicalCPUs
				populated = true
			}
			// OS uptime — btime vs capture timestamp (last snapshot).
			if ps.BTime > 0 && len(c.Snapshots) > 0 {
				last := c.Snapshots[len(c.Snapshots)-1]
				if !last.Timestamp.IsZero() {
					diff := last.Timestamp.Unix() - ps.BTime
					if diff > 0 {
						host.OSUptimeSeconds = diff
						populated = true
					}
				}
			}
		}
	}
	if s := contents["top"]; s != "" {
		if th := parse.ParseTopHeader(s); th != nil {
			host.LoadAvg1 = th.Loadavg1
			host.LoadAvg5 = th.Loadavg5
			host.LoadAvg15 = th.Loadavg15
			if th.Loadavg1 != 0 || th.Loadavg5 != 0 || th.Loadavg15 != 0 {
				populated = true
			}
		}
	}
	if s := contents["meminfo"]; s != "" {
		if m := parse.ParseEnvMeminfo(s); m != nil {
			host.Meminfo = m
			populated = true
		}
	}
	if s := contents["df"]; s != "" {
		if fs := parse.ParseDFSnapshot(s, 5); len(fs) > 0 {
			host.Filesystems = fs
			populated = true
		}
	}

	// ----- MySQL panel --------------------------------------------
	sec.MySQL = buildMySQLEnv(c)
	// Timezone belongs to the host panel but is sourced from a MySQL
	// variable. Attach it only when the host panel already has some
	// signal of its own — otherwise a capture with only -variables
	// would spuriously mark the host panel present and mask the
	// "mysql only" badge outcome.
	if populated && sec.MySQL != nil {
		if tz := lookupVar(c, "system_time_zone"); tz != "" {
			host.Timezone = tz
		}
	}

	if populated {
		sec.Host = host
	}
	return sec
}

// buildMySQLEnv pulls MySQL-facing env fields from the LAST snapshot's
// -variables + last Uptime status. Returns nil when no -variables file
// parsed successfully.
func buildMySQLEnv(c *model.Collection) *model.MySQLEnv {
	vars := latestVariables(c)
	if vars == nil {
		return nil
	}
	get := func(name string) string {
		for _, e := range vars.Entries {
			if e.Name == name {
				return e.Value
			}
		}
		return ""
	}

	mys := &model.MySQLEnv{
		Version:                get("version"),
		VersionComment:         get("version_comment"),
		CompileOS:              get("version_compile_os"),
		CompileMachine:         get("version_compile_machine"),
		DataDir:                get("datadir"),
		Port:                   get("port"),
		Socket:                 get("socket"),
		PidFile:                get("pid_file"),
		ServerID:               get("server_id"),
		DefaultStorageEngine:   get("default_storage_engine"),
		CharacterSetServer:     get("character_set_server"),
		CollationServer:        get("collation_server"),
		TransactionIsolation:   get("transaction_isolation"),
		InnodbBufferPoolInsts:  get("innodb_buffer_pool_instances"),
		MaxConnections:         get("max_connections"),
		SQLMode:                get("sql_mode"),
		SlowQueryLog:           get("slow_query_log"),
		LongQueryTime:          get("long_query_time"),
		LogBin:                 get("log_bin"),
		BinlogFormat:           get("binlog_format"),
		SyncBinlog:             get("sync_binlog"),
		GTIDMode:               get("gtid_mode"),
		EnforceGTIDConsistency: get("enforce_gtid_consistency"),
		ReadOnly:               get("read_only"),
		SuperReadOnly:          get("super_read_only"),
		PerformanceSchema:      get("performance_schema"),
	}
	// Distribution inference.
	mys.Distribution = inferDistribution(get("wsrep_on"), get("wsrep_cluster_size"), mys.VersionComment, mys.Version)
	// Buffer pool size, human-formatted.
	if raw := get("innodb_buffer_pool_size"); raw != "" {
		if bytes, err := strconv.ParseFloat(raw, 64); err == nil {
			mys.InnodbBufferPoolSize = humanBytesEnv(bytes)
		} else {
			mys.InnodbBufferPoolSize = raw
		}
	}
	// Uptime + StartTime. Uptime comes from the last mysqladmin sample
	// that had a numeric value; we also carry that sample's timestamp
	// through so StartTimeUTC anchors to the right clock — not the
	// last Collection snapshot (which may be a newer snapshot without
	// -mysqladmin, shifting start time forward by the gap).
	uptime, uptimeTS := latestUptimeSeconds(c)
	mys.UptimeSeconds = uptime
	if uptime > 0 && !uptimeTS.IsZero() {
		start := uptimeTS.Add(-time.Duration(uptime) * time.Second).UTC()
		mys.StartTimeUTC = start.Format("2006-01-02T15:04:05Z")
	}
	// Galera / PXC sub-panel.
	if strings.EqualFold(get("wsrep_on"), "ON") {
		mys.Wsrep = &model.WsrepEnv{
			ClusterName:     get("wsrep_cluster_name"),
			ClusterSize:     get("wsrep_cluster_size"),
			ProviderName:    get("wsrep_provider_name"),
			ProviderVersion: get("wsrep_provider_version"),
		}
	}
	return mys
}

// latestVariables returns the VariablesData from the last snapshot that
// has a parsed -variables file, or nil.
func latestVariables(c *model.Collection) *model.VariablesData {
	for i := len(c.Snapshots) - 1; i >= 0; i-- {
		sf, ok := c.Snapshots[i].SourceFiles[model.SuffixVariables]
		if !ok || sf == nil || sf.Parsed == nil {
			continue
		}
		if v, ok := sf.Parsed.(*model.VariablesData); ok {
			return v
		}
	}
	return nil
}

func lookupVar(c *model.Collection, name string) string {
	v := latestVariables(c)
	if v == nil {
		return ""
	}
	for _, e := range v.Entries {
		if e.Name == name {
			return e.Value
		}
	}
	return ""
}

// latestUptimeSeconds returns the last Uptime value observed by
// mysqladmin across any snapshot AND the timestamp of the snapshot the
// value came from. Falls back to the "Uptime" field in -variables when
// no mysqladmin counter exists (timestamp = last snapshot in that
// case). Returns (0, zeroTime) when unknown.
func latestUptimeSeconds(c *model.Collection) (int64, time.Time) {
	// Walk snapshots newest-first; take the last non-NaN Uptime slot
	// from the mysqladmin delta series. The caller uses the snapshot's
	// own timestamp as the clock anchor for StartTimeUTC so a newer
	// snapshot without -mysqladmin doesn't shift start time forward.
	for i := len(c.Snapshots) - 1; i >= 0; i-- {
		sn := c.Snapshots[i]
		sf, ok := sn.SourceFiles[model.SuffixMysqladmin]
		if !ok || sf == nil || sf.Parsed == nil {
			continue
		}
		ma, ok := sf.Parsed.(*model.MysqladminData)
		if !ok {
			continue
		}
		if slots, ok := ma.Deltas["Uptime"]; ok {
			for j := len(slots) - 1; j >= 0; j-- {
				if !math.IsNaN(slots[j]) {
					return int64(slots[j]), sn.Timestamp
				}
			}
		}
	}
	// Fallback: variable dump sometimes carries Uptime via -variables.
	// Anchor to the last snapshot's timestamp — variables are a
	// point-in-time dump so we don't have a per-sample timestamp.
	if raw := lookupVar(c, "Uptime"); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			var ts time.Time
			if len(c.Snapshots) > 0 {
				ts = c.Snapshots[len(c.Snapshots)-1].Timestamp
			}
			return v, ts
		}
	}
	return 0, time.Time{}
}

// inferDistribution resolves the presentation label for the MySQL
// distribution. Decision order matches the task spec:
//  1. Galera / PXC when wsrep_on == ON and cluster_size > 0.
//  2. Percona Server when version_comment contains "Percona".
//  3. MariaDB when version_comment or version string says so.
//  4. MySQL Community otherwise.
func inferDistribution(wsrepOn, clusterSize, comment, version string) string {
	if strings.EqualFold(wsrepOn, "ON") {
		if n, err := strconv.Atoi(strings.TrimSpace(clusterSize)); err == nil && n > 0 {
			return "Percona XtraDB Cluster (Galera)"
		}
	}
	if comment != "" {
		lc := strings.ToLower(comment)
		if strings.Contains(lc, "percona") {
			return "Percona Server"
		}
		if strings.Contains(lc, "mariadb") {
			return "MariaDB"
		}
	}
	if strings.Contains(strings.ToLower(version), "mariadb") {
		return "MariaDB"
	}
	if version != "" || comment != "" {
		return "MySQL Community"
	}
	return ""
}

// guessOSFromOutput greps a -output dump for distro-name lines. Best
// effort — returns "" if nothing obvious is found.
func guessOSFromOutput(content string) string {
	needles := []string{
		"Red Hat Enterprise Linux",
		"Rocky Linux",
		"Oracle Linux",
		"CentOS",
		"Ubuntu",
		"Debian",
		"Amazon Linux",
		"SUSE",
		"AlmaLinux",
	}
	for _, line := range strings.Split(content, "\n") {
		for _, n := range needles {
			if strings.Contains(line, n) {
				return strings.TrimSpace(line)
			}
		}
	}
	return ""
}

// humanBytesEnv formats a byte count for the Environment panel. Mirrors
// findings/rules_bufferpool.go's humanBytes but kept local so the render
// layer does not depend on the findings package for a formatter.
func humanBytesEnv(v float64) string {
	const (
		KiB = 1024.0
		MiB = KiB * 1024
		GiB = MiB * 1024
		TiB = GiB * 1024
	)
	switch {
	case v >= TiB:
		return fmt.Sprintf("%.2f TiB", v/TiB)
	case v >= GiB:
		return fmt.Sprintf("%.2f GiB", v/GiB)
	case v >= MiB:
		return fmt.Sprintf("%.0f MiB", v/MiB)
	case v >= KiB:
		return fmt.Sprintf("%.0f KiB", v/KiB)
	default:
		return fmt.Sprintf("%.0f B", v)
	}
}

// humanKBBytes wraps humanBytesEnv with a kB input.
func humanKBBytes(kb int64) string {
	if kb <= 0 {
		return ""
	}
	return humanBytesEnv(float64(kb) * 1024.0)
}

// envView is the flattened, string-only shape the environment template
// consumes. Empty strings become "—" at render time.
type envView struct {
	HasHost  bool
	HasMySQL bool

	// Host
	Hostname             string
	OS                   string
	Kernel               string
	Architecture         string
	LogicalCPUs          string
	LoadAverage          string
	OSUptime             string
	MemTotal             string
	MemAvailable         string
	MemFree              string
	BuffersCached        string
	SwapTotal            string
	SwapUsed             string
	Timezone             string
	Swappiness           string
	DirtyRatio           string
	DirtyBackgroundRatio string
	FileMax              string
	Filesystems          []envFsRow

	// MySQL
	MysqlVersion         string
	VersionComment       string
	Distribution         string
	CompileOS            string
	CompileMachine       string
	MysqlUptime          string
	MysqlStartTime       string
	DataDir              string
	Port                 string
	Socket               string
	PidFile              string
	ServerID             string
	DefaultStorageEngine string
	CharsetCollation     string
	TransactionIsolation string
	BufferPoolSize       string
	BufferPoolInstances  string
	MaxConnections       string
	SQLMode              string
	SlowQueryLog         string
	LongQueryTime        string
	BinaryLogging        string
	GTID                 string
	ReadOnly             string
	PerformanceSchema    string
	HasWsrep             bool
	WsrepClusterName     string
	WsrepClusterSize     string
	WsrepProviderName    string
	WsrepProviderVersion string
}

type envFsRow struct {
	Mount string
	Used  string
	Total string
	Pct   string
}

// buildEnvironmentView flattens the typed EnvironmentSection into the
// string-only shape the template renders. Formatting is deterministic:
// byte counts go through humanBytesEnv, durations through formatDuration,
// empty strings through the "—" template helper.
func buildEnvironmentView(sec *model.EnvironmentSection) envView {
	v := envView{}
	if sec == nil {
		return v
	}
	if h := sec.Host; h != nil {
		v.HasHost = true
		v.Hostname = h.Hostname
		v.OS = h.OS
		v.Kernel = h.Kernel
		v.Architecture = h.Architecture
		if h.LogicalCPUs > 0 {
			v.LogicalCPUs = strconv.Itoa(h.LogicalCPUs)
		}
		if h.LoadAvg1 != 0 || h.LoadAvg5 != 0 || h.LoadAvg15 != 0 {
			v.LoadAverage = fmt.Sprintf("%.2f / %.2f / %.2f", h.LoadAvg1, h.LoadAvg5, h.LoadAvg15)
		}
		if h.OSUptimeSeconds > 0 {
			v.OSUptime = formatDurationHuman(h.OSUptimeSeconds)
		}
		if m := h.Meminfo; m != nil {
			v.MemTotal = humanKBBytes(m.MemTotalKB)
			v.MemAvailable = humanKBBytes(m.MemAvailableKB)
			v.MemFree = humanKBBytes(m.MemFreeKB)
			if m.BuffersKB > 0 || m.CachedKB > 0 {
				v.BuffersCached = humanKBBytes(m.BuffersKB + m.CachedKB)
			}
			v.SwapTotal = humanKBBytes(m.SwapTotalKB)
			used := m.SwapTotalKB - m.SwapFreeKB
			if used > 0 {
				v.SwapUsed = humanKBBytes(used)
			} else if m.SwapTotalKB > 0 {
				v.SwapUsed = "0 B"
			}
		}
		v.Timezone = h.Timezone
		v.Swappiness = h.Swappiness
		v.DirtyRatio = h.DirtyRatio
		v.DirtyBackgroundRatio = h.DirtyBackgroundRatio
		v.FileMax = h.FileMax
		for _, fs := range h.Filesystems {
			v.Filesystems = append(v.Filesystems, envFsRow{
				Mount: fs.Mount,
				Used:  humanKBBytes(fs.UsedKB),
				Total: humanKBBytes(fs.SizeKB),
				Pct:   strconv.Itoa(fs.UsePct) + "%",
			})
		}
	}
	if m := sec.MySQL; m != nil {
		v.HasMySQL = true
		v.MysqlVersion = m.Version
		v.VersionComment = m.VersionComment
		v.Distribution = m.Distribution
		v.CompileOS = m.CompileOS
		v.CompileMachine = m.CompileMachine
		if m.UptimeSeconds > 0 {
			v.MysqlUptime = formatDurationHuman(m.UptimeSeconds)
		}
		v.MysqlStartTime = m.StartTimeUTC
		v.DataDir = m.DataDir
		v.Port = m.Port
		v.Socket = m.Socket
		v.PidFile = m.PidFile
		v.ServerID = m.ServerID
		v.DefaultStorageEngine = m.DefaultStorageEngine
		v.CharsetCollation = joinNonEmpty(" / ", m.CharacterSetServer, m.CollationServer)
		v.TransactionIsolation = m.TransactionIsolation
		v.BufferPoolSize = m.InnodbBufferPoolSize
		v.BufferPoolInstances = m.InnodbBufferPoolInsts
		v.MaxConnections = m.MaxConnections
		v.SQLMode = m.SQLMode
		v.SlowQueryLog = m.SlowQueryLog
		v.LongQueryTime = m.LongQueryTime
		v.BinaryLogging = joinBinlog(m.LogBin, m.BinlogFormat, m.SyncBinlog)
		v.GTID = joinGTID(m.GTIDMode, m.EnforceGTIDConsistency)
		v.ReadOnly = joinReadOnly(m.ReadOnly, m.SuperReadOnly)
		v.PerformanceSchema = m.PerformanceSchema
		if m.Wsrep != nil {
			v.HasWsrep = true
			v.WsrepClusterName = m.Wsrep.ClusterName
			v.WsrepClusterSize = m.Wsrep.ClusterSize
			v.WsrepProviderName = m.Wsrep.ProviderName
			v.WsrepProviderVersion = m.Wsrep.ProviderVersion
		}
	}
	return v
}

func joinNonEmpty(sep string, parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}

func joinBinlog(logBin, format, sync string) string {
	if logBin == "" && format == "" && sync == "" {
		return ""
	}
	parts := []string{}
	if logBin != "" {
		parts = append(parts, "log_bin="+logBin)
	}
	if format != "" {
		parts = append(parts, "format="+format)
	}
	if sync != "" {
		parts = append(parts, "sync_binlog="+sync)
	}
	return strings.Join(parts, ", ")
}

func joinGTID(mode, enforce string) string {
	if mode == "" && enforce == "" {
		return ""
	}
	parts := []string{}
	if mode != "" {
		parts = append(parts, "gtid_mode="+mode)
	}
	if enforce != "" {
		parts = append(parts, "enforce_gtid_consistency="+enforce)
	}
	return strings.Join(parts, ", ")
}

func joinReadOnly(ro, sro string) string {
	if ro == "" && sro == "" {
		return ""
	}
	parts := []string{}
	if ro != "" {
		parts = append(parts, "read_only="+ro)
	}
	if sro != "" {
		parts = append(parts, "super_read_only="+sro)
	}
	return strings.Join(parts, ", ")
}

// formatDurationHuman renders a seconds count as "Nd HHh MMm" (or the
// largest reasonable form). Deterministic; UTC-agnostic.
func formatDurationHuman(secs int64) string {
	if secs <= 0 {
		return ""
	}
	d := secs / 86400
	h := (secs % 86400) / 3600
	m := (secs % 3600) / 60
	s := secs % 60
	switch {
	case d > 0:
		return fmt.Sprintf("%dd %dh %dm", d, h, m)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
