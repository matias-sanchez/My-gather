package model

// EnvironmentSection is the render-ready view of the pt-stalk "Environment"
// panel pair — one panel for host-level facts and another for MySQL-level
// facts. Either panel may be nil when the corresponding sources could not
// be resolved from the capture; templates render "—" in that case.
type EnvironmentSection struct {
	Host  *HostEnv
	MySQL *MySQLEnv
}

// HostEnv carries host-level environment facts sourced from sidecar
// pt-stalk collector files (hostname, sysctl, procstat, top, meminfo, df)
// and — for timezone only — from the parsed MySQL variables.
//
// Fields are exported strings where the environment template expects a
// pre-formatted value. Zero values render as "—". Nested scalar payloads
// are kept typed (EnvMeminfo, EnvProcStat, …) so render/environment.go
// can format them deterministically at view-build time.
type HostEnv struct {
	Hostname        string
	OS              string
	Kernel          string
	Architecture    string
	LogicalCPUs     int
	LoadAvg1        float64
	LoadAvg5        float64
	LoadAvg15       float64
	OSUptimeSeconds int64 // capture-timestamp minus btime; 0 when unknown
	Meminfo         *EnvMeminfo
	Timezone        string

	// Selected sysctl tunings. Missing keys carry the empty string.
	Swappiness           string
	DirtyRatio           string
	DirtyBackgroundRatio string
	FileMax              string

	// Top N filesystems (sorted by Use% descending, capped by the
	// collector that populated this slice — typically 5).
	Filesystems []EnvFilesystem
}

// MySQLEnv carries MySQL-level environment facts sourced from the
// parsed -variables + last-sample Uptime status on the DBSection.
type MySQLEnv struct {
	Version                string
	VersionComment         string
	Distribution           string
	CompileOS              string
	CompileMachine         string
	UptimeSeconds          int64
	StartTimeUTC           string // "2006-01-02T15:04:05Z" when derivable
	DataDir                string
	Port                   string
	Socket                 string
	PidFile                string
	ServerID               string
	DefaultStorageEngine   string
	CharacterSetServer     string
	CollationServer        string
	TransactionIsolation   string
	InnodbBufferPoolSize   string // pre-formatted via humanBytes
	InnodbBufferPoolInsts  string
	MaxConnections         string
	SQLMode                string
	SlowQueryLog           string
	LongQueryTime          string
	LogBin                 string
	BinlogFormat           string
	SyncBinlog             string
	GTIDMode               string
	EnforceGTIDConsistency string
	ReadOnly               string
	SuperReadOnly          string
	PerformanceSchema      string
	Wsrep                  *WsrepEnv // non-nil iff wsrep_on is ON
}

// WsrepEnv is the Galera / PXC cluster sub-panel, rendered only when
// the captured MySQL has wsrep_on = ON.
type WsrepEnv struct {
	ClusterName     string
	ClusterSize     string
	ProviderName    string
	ProviderVersion string
}

// EnvMeminfo is the scalar memory view rendered in the Environment
// panel. Values are in kB — the same unit /proc/meminfo reports — so
// the render layer can decide on human units (GiB, MiB, …).
type EnvMeminfo struct {
	MemTotalKB      int64
	MemFreeKB       int64
	MemAvailableKB  int64
	BuffersKB       int64
	CachedKB        int64
	SwapTotalKB     int64
	SwapFreeKB      int64
	HugePagesTotal  int64
	AnonHugePagesKB int64
}

// EnvProcStat is the scalar /proc/stat-derived view: logical CPU count
// and boot time (seconds since Unix epoch).
type EnvProcStat struct {
	LogicalCPUs int
	BTime       int64
}

// EnvTopHeader carries the three load averages from a -top file's first
// line. Any value that could not be parsed is left at 0 and handled as
// "—" by the render layer.
type EnvTopHeader struct {
	Loadavg1  float64
	Loadavg5  float64
	Loadavg15 float64
}

// EnvFilesystem is one mount-point row from a -df snapshot.
type EnvFilesystem struct {
	FS     string
	Mount  string
	SizeKB int64
	UsedKB int64
	UsePct int // 0..100 — parsed from the "Use%" column
}
