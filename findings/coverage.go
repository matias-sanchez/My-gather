package findings

import "sort"

// CoverageStatus identifies whether a Rosetta Stone topic is
// implemented by Advisor rules in this feature.
type CoverageStatus string

const (
	// CoverageCovered means the topic has native Advisor rule coverage.
	CoverageCovered CoverageStatus = "Covered"

	// CoveragePartial means some signals are covered but the topic
	// needs additional parsed evidence for complete coverage.
	CoveragePartial CoverageStatus = "Partial"

	// CoverageDeferred means the topic is intentionally deferred.
	CoverageDeferred CoverageStatus = "Deferred"

	// CoverageExcluded means the topic is outside this feature's scope.
	CoverageExcluded CoverageStatus = "Excluded"
)

// CoverageTopic maps one Rosetta Stone topic to Advisor coverage.
type CoverageTopic struct {
	Topic          string
	Subsystem      string
	Category       DiagnosticCategory
	CoverageStatus CoverageStatus
	Signals        []string
	Reason         string
}

// CoverageMap returns a deterministic copy of Rosetta Stone topic
// coverage for the Advisor.
func CoverageMap() []CoverageTopic {
	topics := []CoverageTopic{
		{Topic: "Buffer Pool", Subsystem: "Buffer Pool", Category: CategoryCombined, CoverageStatus: CoverageCovered, Signals: []string{"hit ratio", "free pages", "LRU flushing", "wait_free", "dirty pages"}},
		{Topic: "Redo Log", Subsystem: "Redo Log", Category: CategoryCombined, CoverageStatus: CoverageCovered, Signals: []string{"checkpoint age", "pending writes", "pending fsyncs", "log waits"}},
		{Topic: "Flushing and Dirty Pages", Subsystem: "Buffer Pool", Category: CategorySaturation, CoverageStatus: CoverageCovered, Signals: []string{"InnoDB pending writes", "single-page flushes", "flush-list backlog"}},
		{Topic: "Table Open Cache", Subsystem: "Table Open Cache", Category: CategoryCombined, CoverageStatus: CoverageCovered, Signals: []string{"Open_tables", "Table_open_cache_misses", "Table_open_cache_overflows"}},
		{Topic: "Thread Cache", Subsystem: "Thread Cache", Category: CategorySaturation, CoverageStatus: CoverageCovered, Signals: []string{"Threads_created", "Connections"}},
		{Topic: "Connections", Subsystem: "Connections", Category: CategoryCombined, CoverageStatus: CoverageCovered, Signals: []string{"Threads_connected", "max_connections", "Aborted_connects"}},
		{Topic: "Temporary Tables", Subsystem: "Temp Tables", Category: CategorySaturation, CoverageStatus: CoverageCovered, Signals: []string{"Created_tmp_tables", "Created_tmp_disk_tables"}},
		{Topic: "Query Shape", Subsystem: "Query Shape", Category: CategoryCombined, CoverageStatus: CoverageCovered, Signals: []string{"Select_scan", "Select_full_join", "Handler_read_rnd_next", "observed processlist queries"}},
		{Topic: "Semaphores", Subsystem: "InnoDB Semaphores", Category: CategorySaturation, CoverageStatus: CoverageCovered, Signals: []string{"SHOW ENGINE INNODB STATUS semaphore waits"}},
		{Topic: "Binary Log Caches", Subsystem: "Binlog Cache", Category: CategorySaturation, CoverageStatus: CoverageCovered, Signals: []string{"Binlog_cache_disk_use", "Binlog_stmt_cache_disk_use"}},
		{Topic: "Metadata and DDL Contention", Subsystem: "Query Shape", Category: CategoryCombined, CoverageStatus: CoveragePartial, Signals: []string{"metadata lock processlist states", "Com_alter_table"}, Reason: "Prepared-statement reprepare errors require error-log evidence not parsed by this feature."},
		{Topic: "Change Buffer", Subsystem: "Buffer Pool", Category: CategorySaturation, CoverageStatus: CoverageDeferred, Reason: "Requires additional InnoDB status change-buffer parsing beyond current Advisor inputs."},
		{Topic: "Adaptive Hash Index", Subsystem: "Buffer Pool", Category: CategoryUtilization, CoverageStatus: CoverageDeferred, Reason: "Requires additional InnoDB status or INNODB_METRICS inputs not available in current reports."},
		{Topic: "Data Dictionary", Subsystem: "Table Open Cache", Category: CategoryCombined, CoverageStatus: CoverageDeferred, Reason: "Requires error-log and version-specific dictionary wait evidence not currently normalized."},
	}
	sort.SliceStable(topics, func(i, j int) bool {
		if topics[i].Subsystem != topics[j].Subsystem {
			return subsystemOrder(topics[i].Subsystem) < subsystemOrder(topics[j].Subsystem)
		}
		return topics[i].Topic < topics[j].Topic
	})
	return topics
}

func inferCategory(id, subsystem, title, formula string) DiagnosticCategory {
	if id == "queryshape.observed_slow_processlist" {
		return CategoryCombined
	}
	text := id + " " + title + " " + formula
	switch {
	case containsAny(text, "error", "aborted"):
		return CategoryError
	case containsAny(text, "wait", "pending", "overflow", "miss", "dirty", "full_join", "select_scan", "scan", "rnd_next", "disk", "metadata", "flushing", "saturation", "semaphore", "queue", "processlist"):
		return CategorySaturation
	case containsAny(text, "checkpoint_age", "ratio", "usage", "size", "cache", "utilization"):
		return CategoryUtilization
	default:
		return CategoryCombined
	}
}

func defaultCoverageTopic(subsystem, title string) string {
	if title == "" {
		return subsystem
	}
	return subsystem + " - " + title
}
