package findings

import (
	"fmt"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

// ruleConfigMaxConnectionsHigh flags `max_connections` settings that
// are large enough to be a foot-gun on under-sized hosts. A high
// max_connections is not a problem on its own — it only becomes one
// when the buffer pool is small (per-thread overhead can outweigh
// pool memory) or when concurrent runners spike.
//
// Trip conditions:
//
//	max_connections > 5000
//	  AND innodb_buffer_pool_size < 1 GiB per 1000 max_connections
//	  → WARN
//
//	max_connections > 5000
//	  AND innodb_buffer_pool_size < 256 MiB per 1000 max_connections
//	  → CRIT
//
// Per-thread overhead (sort_buffer, join_buffer, thread stack) is the
// usual 4-12 MiB ballpark, so a 1 GiB-per-1000 floor keeps the host
// from running out of memory under any plausible concurrency surge.
func ruleConfigMaxConnectionsHigh(r *model.Report) Finding {
	const (
		threshold     = 5000.0
		bytesPer1000W = 1024.0 * 1024.0 * 1024.0 // 1 GiB / 1k connections
		bytesPer1000C = 256.0 * 1024.0 * 1024.0  // 256 MiB / 1k connections
	)
	maxConns, ok := variableFloat(r, "max_connections")
	if !ok || maxConns <= threshold {
		return Finding{Severity: SeveritySkip}
	}
	bpSize, ok := variableFloat(r, "innodb_buffer_pool_size")
	if !ok || bpSize <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	perThousand := bpSize / (maxConns / 1000)

	sev := SeverityOK
	summary := fmt.Sprintf("max_connections = %s with %s per 1k slots — within the 1 GiB-per-1k headroom band.",
		formatNum(maxConns), humanBytes(perThousand))
	switch {
	case perThousand < bytesPer1000C:
		sev = SeverityCrit
		summary = fmt.Sprintf("max_connections = %s but innodb_buffer_pool_size is only %s (%s per 1k slots) — concurrent thread overhead will starve the pool under load.",
			formatNum(maxConns), humanBytes(bpSize), humanBytes(perThousand))
	case perThousand < bytesPer1000W:
		sev = SeverityWarn
		summary = fmt.Sprintf("max_connections = %s with %s buffer pool — that's only %s per 1k slots, well below the 1 GiB headroom guideline.",
			formatNum(maxConns), humanBytes(bpSize), humanBytes(perThousand))
	}
	threadsRunning, hasRun := gaugeMax(r, "Threads_running")
	metrics := []MetricRef{
		{Name: "max_connections", Value: maxConns, Unit: "count"},
		{Name: "innodb_buffer_pool_size", Value: bpSize, Unit: "bytes"},
		{Name: "buffer pool / 1k slots", Value: perThousand, Unit: "bytes"},
	}
	if hasRun {
		metrics = append(metrics, MetricRef{Name: "Threads_running (max)", Value: threadsRunning, Unit: "count"})
	}
	return Finding{
		ID:        "config.max_connections_high",
		Subsystem: "Configuration",
		Title:     "max_connections vs buffer pool sizing",
		Severity:  sev,
		Summary:   summary,
		Explanation: "max_connections caps the number of concurrent threads MySQL will accept. Each open thread reserves stack, " +
			"sort_buffer, join_buffer and other per-session memory before it ever runs a query. When max_connections is large " +
			"and the buffer pool is small, a connection burst can either OOM the host or starve the pool of headroom for hot " +
			"data. The 1 GiB-per-1000-slots heuristic leaves room for ~4-12 MiB per thread plus stable buffer-pool capacity.",
		FormulaText: "max_connections > 5000  AND  innodb_buffer_pool_size / (max_connections / 1000) < {1 GiB warn, 256 MiB crit}",
		FormulaComputed: fmt.Sprintf("max_connections=%s, innodb_buffer_pool_size=%s, ratio=%s/1k",
			formatNum(maxConns), humanBytes(bpSize), humanBytes(perThousand)),
		Metrics: metrics,
		Recommendations: []string{
			"Lower max_connections to a value that reflects real concurrency (typical OLTP fleets sit at 200-2000).",
			"If the application genuinely needs thousands of pooled connections, place a proxy (ProxySQL, MySQL Router) in front so the server only sees actual runners.",
			"Raise innodb_buffer_pool_size so the pool retains at least 1 GiB of headroom per 1000 max_connections after per-thread allocations.",
		},
		Source: "Rosetta Stone — Connections (capacity vs concurrency)",
	}
}

// ruleConfigSyncBinlogNotOne flags durability-relevant settings of
// `sync_binlog` that drift from the canonical safe value of 1.
//
// Trip conditions:
//
//	sync_binlog == 0  AND replication is configured  → CRIT
//	sync_binlog == 0  (no replication / unknown)     → WARN
//	sync_binlog > 1                                  → WARN (durability concern under crash)
//	log_bin = OFF                                    → INFO (binlog disabled — different scenario)
//
// "Replication is configured" is detected via `server_id != 0` AND
// (`gtid_mode != OFF` OR a non-empty `binlog_format`); both are
// captured in pt-stalk's variables snapshot. False negatives are
// safer than false positives: the WARN tier still flags a mis-set
// sync_binlog independently.
func ruleConfigSyncBinlogNotOne(r *model.Report) Finding {
	logBinRaw, hasLogBin := variableRaw(r, "log_bin")
	if hasLogBin {
		logBin := strings.ToUpper(strings.TrimSpace(logBinRaw))
		if logBin == "OFF" || logBin == "0" {
			return Finding{
				ID:        "config.sync_binlog_not_one",
				Subsystem: "Configuration",
				Title:     "Binary logging is disabled",
				Severity:  SeverityInfo,
				Summary:   "log_bin = OFF — binary logging is disabled; sync_binlog has no effect on this server.",
				Explanation: "With log_bin = OFF the server does not record write events to a binary log, so neither replication " +
					"nor point-in-time recovery is possible from this instance. This may be intentional for a read replica " +
					"with semi-sync disabled or a development host, but is otherwise a serious recoverability gap.",
				FormulaText:     "log_bin = OFF",
				FormulaComputed: "log_bin = " + logBinRaw,
				Metrics: []MetricRef{
					{Name: "log_bin", Value: 0, Unit: "bool", Note: "raw: " + logBinRaw},
				},
				Recommendations: []string{
					"If this is a primary or a server that may ever be promoted, enable log_bin and pick a sensible binlog retention.",
				},
				Source: "Rosetta Stone — Configuration §Binlog (durability)",
			}
		}
	}

	syncBL, ok := variableFloat(r, "sync_binlog")
	if !ok {
		return Finding{Severity: SeveritySkip}
	}
	if syncBL == 1 {
		return Finding{Severity: SeveritySkip}
	}

	replicationLikely := isReplicationConfigured(r)

	switch {
	case syncBL == 0 && replicationLikely:
		return Finding{
			ID:        "config.sync_binlog_not_one",
			Subsystem: "Configuration",
			Title:     "sync_binlog disabled with replication configured",
			Severity:  SeverityCrit,
			Summary:   "sync_binlog = 0 on a replication-configured server — a crash can lose binlog events that have already been streamed to replicas.",
			Explanation: "sync_binlog = 0 means the server never fsync()s the binary log; it is flushed only when the OS decides. " +
				"On a primary serving replicas, a crash can leave replicas with events that no longer exist on the primary's disk, " +
				"breaking replication and risking data loss. The canonical safe value on any replication-active primary is 1.",
			FormulaText:     "sync_binlog = 0  AND  (server_id != 0  AND  binlog/gtid configured)",
			FormulaComputed: fmt.Sprintf("sync_binlog = %s, replication_configured = true", formatNum(syncBL)),
			Metrics: []MetricRef{
				{Name: "sync_binlog", Value: syncBL, Unit: "count"},
			},
			Recommendations: []string{
				"Set sync_binlog = 1 on the primary so every group commit fsyncs the binary log.",
				"Pair with innodb_flush_log_at_trx_commit = 1 for full ACID durability.",
				"If write throughput drops unacceptably, batch with binlog_group_commit_sync_delay before relaxing sync_binlog.",
			},
			Source: "Rosetta Stone — Configuration §Binlog (durability)",
		}
	case syncBL == 0:
		return Finding{
			ID:        "config.sync_binlog_not_one",
			Subsystem: "Configuration",
			Title:     "sync_binlog is 0",
			Severity:  SeverityWarn,
			Summary:   "sync_binlog = 0 — binary log is never fsync'd; a server crash can lose binlog events.",
			Explanation: "Even without replication, sync_binlog = 0 makes the binary log unreliable for point-in-time recovery: " +
				"after a crash, the on-disk binlog may not contain the last group of committed transactions.",
			FormulaText:     "sync_binlog = 0",
			FormulaComputed: fmt.Sprintf("sync_binlog = %s", formatNum(syncBL)),
			Metrics: []MetricRef{
				{Name: "sync_binlog", Value: syncBL, Unit: "count"},
			},
			Recommendations: []string{
				"Set sync_binlog = 1 unless you have explicitly accepted the recoverability gap.",
				"Verify your backup / PITR strategy does not depend on the binary log if you keep sync_binlog = 0.",
				"Document the deviation in the server's runbook so the next operator knows the trade-off.",
			},
			Source: "Rosetta Stone — Configuration §Binlog (durability)",
		}
	default:
		// sync_binlog > 1: writes fsync every Nth commit, durability gap of N transactions on crash.
		return Finding{
			ID:        "config.sync_binlog_not_one",
			Subsystem: "Configuration",
			Title:     "sync_binlog > 1",
			Severity:  SeverityWarn,
			Summary:   fmt.Sprintf("sync_binlog = %s — up to %s transactions can be lost on crash.", formatNum(syncBL), formatNum(syncBL-1)),
			Explanation: "sync_binlog > 1 fsyncs the binary log only every Nth group commit. On a crash the most recent N-1 " +
				"transactions may be missing from the binlog, breaking PITR and (on a primary) potentially making replicas " +
				"diverge. The safe value is 1.",
			FormulaText:     "sync_binlog > 1",
			FormulaComputed: fmt.Sprintf("sync_binlog = %s", formatNum(syncBL)),
			Metrics: []MetricRef{
				{Name: "sync_binlog", Value: syncBL, Unit: "count"},
			},
			Recommendations: []string{
				"Set sync_binlog = 1 for full durability.",
				"If write throughput is the concern, use binlog_group_commit_sync_delay (microseconds) — it batches without losing fsync semantics.",
				"Re-evaluate hardware: a healthy NVMe should sustain sync_binlog = 1 well past 10k commits/s.",
			},
			Source: "Rosetta Stone — Configuration §Binlog (durability)",
		}
	}
}

// isReplicationConfigured reports whether the captured variables
// suggest the server is part of a replication topology. The detector
// is intentionally conservative — both server_id != 0 AND a binlog/
// GTID indicator must be present — so the CRIT escalation in
// ruleConfigSyncBinlogNotOne does not fire on standalone servers.
func isReplicationConfigured(r *model.Report) bool {
	serverID, ok := variableFloat(r, "server_id")
	if !ok || serverID == 0 {
		return false
	}
	if g, ok := variableRaw(r, "gtid_mode"); ok {
		if v := strings.ToUpper(strings.TrimSpace(g)); v != "" && v != "OFF" {
			return true
		}
	}
	if f, ok := variableRaw(r, "binlog_format"); ok {
		if strings.TrimSpace(f) != "" {
			return true
		}
	}
	return false
}

// init registers the two new configuration rules added in this PR.
func init() {
	register(RuleDefinition{
		ID:                 "config.max_connections_high",
		Subsystem:          "Configuration",
		Title:              "max_connections vs buffer pool sizing",
		FormulaText:        "max_connections > 5000  AND  innodb_buffer_pool_size / (max_connections/1000) < {1 GiB warn, 256 MiB crit}",
		MinRecommendations: 3,
		Severity:           SeverityHintVariable,
		Run:                ruleConfigMaxConnectionsHigh,
	})
	register(RuleDefinition{
		ID:                 "config.sync_binlog_not_one",
		Subsystem:          "Configuration",
		Title:              "sync_binlog vs replication durability",
		FormulaText:        "sync_binlog != 1  (CRIT if replication; INFO if log_bin=OFF; WARN otherwise)",
		MinRecommendations: 3,
		Severity:           SeverityHintVariable,
		Run:                ruleConfigSyncBinlogNotOne,
	})
}

