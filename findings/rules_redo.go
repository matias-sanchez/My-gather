package findings

import (
	"fmt"

	"github.com/matias-sanchez/My-gather/model"
)

// ruleRedoCheckpointAge evaluates checkpoint_age / checkpoint_max_age.
// Percona Server exposes these as status variables directly; stock
// MySQL does not, in which case the rule returns Skip.
// See Rosetta Stone — Redo Log §Utilization (capacity).
func ruleRedoCheckpointAge(r *model.Report) Finding {
	const (
		warnThreshold = 0.5
		critThreshold = 0.8
	)
	age, ok1 := gaugeMax(r, "Innodb_checkpoint_age")
	max, ok2 := gaugeLast(r, "Innodb_checkpoint_max_age")
	if !ok1 || !ok2 || max <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	ratio := age / max
	sev := SeverityOK
	summary := fmt.Sprintf("Checkpoint age peaked at %.1f %% of max — plenty of redo-log headroom.", ratio*100)
	switch {
	case ratio >= critThreshold:
		sev = SeverityCrit
		summary = fmt.Sprintf("Checkpoint age reached %.1f %% of max during the capture — redo log is near saturation.", ratio*100)
	case ratio >= warnThreshold:
		sev = SeverityWarn
		summary = fmt.Sprintf("Checkpoint age peaked at %.1f %% of max — redo log is filling up.", ratio*100)
	}
	return Finding{
		ID:        "redo.checkpoint_age",
		Subsystem: "Redo Log",
		Title:     "Redo log checkpoint age",
		Severity:  sev,
		Summary:   summary,
		Explanation: "InnoDB cannot reuse redo log space while it still contains unflushed changes. " +
			"When checkpoint_age approaches checkpoint_max_age (~7/8 of redo log size), InnoDB throttles writes " +
			"to force a checkpoint, which appears as a latency stall.",
		FormulaText:     "checkpoint_age / checkpoint_max_age",
		FormulaComputed: fmt.Sprintf("%s / %s = %.2f %%", formatNum(age), formatNum(max), ratio*100),
		Metrics: []MetricRef{
			{Name: "Innodb_checkpoint_age (max)", Value: age, Unit: "bytes"},
			{Name: "Innodb_checkpoint_max_age", Value: max, Unit: "bytes"},
		},
		Recommendations: []string{
			"Increase the redo log size (innodb_redo_log_capacity on 8.0, or innodb_log_file_size × innodb_log_files_in_group on 5.7).",
			"If I/O allows it, raise innodb_io_capacity to make checkpoint flushing faster.",
			"Check whether a long-running write workload is issuing many small transactions — batching reduces checkpoint pressure.",
		},
		Source: "Rosetta Stone — Redo Log §Utilization (capacity) — Percona Server only",
	}
}

// ruleRedoPendingWrites flags any observation where the number of
// pending OS log writes was > 0.
// See Rosetta Stone — Redo Log §Saturation (throughput).
func ruleRedoPendingWrites(r *model.Report) Finding {
	peak, ok := gaugeMax(r, "Innodb_os_log_pending_writes")
	if !ok {
		return Finding{Severity: SeveritySkip}
	}
	if peak <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	return Finding{
		ID:        "redo.pending_writes",
		Subsystem: "Redo Log",
		Title:     "Redo log pending writes",
		Severity:  SeverityCrit,
		Summary:   fmt.Sprintf("Innodb_os_log_pending_writes peaked at %s — the storage cannot keep up with the redo write rate.", formatNum(peak)),
		Explanation: "This gauge counts redo writes queued at the OS level waiting for the disk to accept them. " +
			"Sustained non-zero values mean the storage layer is the bottleneck for commits.",
		FormulaText:     "max(Innodb_os_log_pending_writes) > 0",
		FormulaComputed: fmt.Sprintf("max = %s > 0", formatNum(peak)),
		Metrics: []MetricRef{
			{Name: "Innodb_os_log_pending_writes (max)", Value: peak, Unit: "count"},
		},
		Recommendations: []string{
			"Move redo logs to faster storage, or verify that the current storage's real IOPS budget is sized for the workload.",
			"Review innodb_flush_log_at_trx_commit — if it's 1 (safe) and you can tolerate 1-second data loss, 2 halves the fsync rate.",
			"Check for contention between redo and data writes on the same device.",
		},
		Source: "Rosetta Stone — Redo Log §Saturation (throughput)",
	}
}

// ruleRedoPendingFsyncs is the fsync counterpart to pending writes.
func ruleRedoPendingFsyncs(r *model.Report) Finding {
	peak, ok := gaugeMax(r, "Innodb_os_log_pending_fsyncs")
	if !ok {
		return Finding{Severity: SeveritySkip}
	}
	if peak <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	return Finding{
		ID:        "redo.pending_fsyncs",
		Subsystem: "Redo Log",
		Title:     "Redo log pending fsyncs",
		Severity:  SeverityCrit,
		Summary:   fmt.Sprintf("Innodb_os_log_pending_fsyncs peaked at %s — fsync latency is the bottleneck for commits.", formatNum(peak)),
		Explanation: "Pending fsyncs on the redo log mean commits are serialised behind the storage's fsync latency. " +
			"Any sustained non-zero value here is felt directly as commit latency.",
		FormulaText:     "max(Innodb_os_log_pending_fsyncs) > 0",
		FormulaComputed: fmt.Sprintf("max = %s > 0", formatNum(peak)),
		Metrics: []MetricRef{
			{Name: "Innodb_os_log_pending_fsyncs (max)", Value: peak, Unit: "count"},
		},
		Recommendations: []string{
			"Validate that the redo log filesystem does not have write-back caching disabled at the hardware level.",
			"Consider batching commits (group commit is on by default but can be tuned further).",
			"As a last resort, lower innodb_flush_log_at_trx_commit to 2 — accepts up to 1 s of data loss on OS crash.",
		},
		Source: "Rosetta Stone — Redo Log §Saturation (throughput)",
	}
}

// ruleRedoLogWaits flags any increment of Innodb_log_waits —
// transactions had to wait because the log buffer was full.
// See Rosetta Stone — Redo Log Buffer §Saturation (capacity).
func ruleRedoLogWaits(r *model.Report) Finding {
	rate, ok := counterRatePerSec(r, "Innodb_log_waits")
	if !ok {
		return Finding{Severity: SeveritySkip}
	}
	if rate <= 0 {
		return Finding{Severity: SeveritySkip}
	}
	sev := SeverityWarn
	if rate > 1 {
		sev = SeverityCrit
	}
	return Finding{
		ID:        "redo.log_waits",
		Subsystem: "Redo Log",
		Title:     "Redo log buffer waits",
		Severity:  sev,
		Summary:   fmt.Sprintf("Innodb_log_waits is incrementing at %s/s — the redo log buffer is undersized.", formatNum(rate)),
		Explanation: "When a transaction needs to write to the log buffer but the buffer is full, it must wait for a flush. " +
			"Any non-zero rate is a signal to raise innodb_log_buffer_size.",
		FormulaText:     "Innodb_log_waits/s > 0",
		FormulaComputed: fmt.Sprintf("%s /s > 0", formatNum(rate)),
		Metrics: []MetricRef{
			{Name: "Innodb_log_waits/s", Value: rate, Unit: "/s"},
		},
		Recommendations: []string{
			"Increase innodb_log_buffer_size (64 MB or 128 MB is a reasonable starting point for busy systems).",
			"Check whether large transactions are producing redo faster than the buffer can drain.",
		},
		Source: "Rosetta Stone — Redo Log Buffer §Saturation (capacity)",
	}
}

// init registers the native RuleDefinition entries for every Redo
// Log rule in this file. Metadata lives next to each Run function
// so the rules catalogue and the rendered Finding stay aligned.
func init() {
	register(RuleDefinition{
		ID:                 "redo.checkpoint_age",
		Subsystem:          "Redo Log",
		Title:              "Redo checkpoint age",
		FormulaText:        "max(Innodb_checkpoint_age) / Innodb_checkpoint_max_age",
		MinRecommendations: 3,
		Severity:           SeverityHintVariable,
		Run:                ruleRedoCheckpointAge,
	})
	register(RuleDefinition{
		ID:                 "redo.pending_writes",
		Subsystem:          "Redo Log",
		Title:              "Redo log pending writes",
		FormulaText:        "max(Innodb_os_log_pending_writes) > 0",
		MinRecommendations: 3,
		Severity:           SeverityHintCritical,
		Run:                ruleRedoPendingWrites,
	})
	register(RuleDefinition{
		ID:                 "redo.pending_fsyncs",
		Subsystem:          "Redo Log",
		Title:              "Redo log pending fsyncs",
		FormulaText:        "max(Innodb_os_log_pending_fsyncs) > 0",
		MinRecommendations: 3,
		Severity:           SeverityHintCritical,
		Run:                ruleRedoPendingFsyncs,
	})
	register(RuleDefinition{
		ID:                 "redo.log_waits",
		Subsystem:          "Redo Log",
		Title:              "Redo log buffer waits",
		FormulaText:        "Innodb_log_waits/s > 0",
		MinRecommendations: 3,
		Severity:           SeverityHintVariable,
		Run:                ruleRedoLogWaits,
	})
}
