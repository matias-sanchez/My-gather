# Feature Specification: Advisor Intelligence

**Feature Branch**: `007-advisor-intelligence`  
**Created**: 2026-04-29  
**Status**: Draft  
**Input**: User description: "Create a spec to enhance Advisor intelligence using the expert insights from `MySQL Rosetta Stone.txt` in the Downloads folder so analysts can understand database problems with highest-quality diagnosis."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Explain incident drivers clearly (Priority: P1)

A database support engineer opens a generated report during or after a MySQL
incident and needs the Advisor to explain the most likely pressure points in
plain diagnostic language, with enough evidence to decide what to inspect next.

**Why this priority**: The report already gathers many metrics, but raw
signals are not enough under pressure. The Advisor must connect observed
symptoms to likely database subsystems and make the first triage path obvious.

**Independent Test**: Generate a report from a capture with known buffer pool,
redo, table cache, connection, and query-shape signals. The Advisor lists the
expected findings with subsystem, severity, evidence, interpretation, and next
checks.

**Acceptance Scenarios**:

1. **Given** a capture with clear subsystem pressure, **When** the report is
   opened, **Then** the Advisor identifies the affected subsystem and states
   why the signal matters.
2. **Given** a finding depends on multiple related metrics, **When** the
   Advisor renders the finding, **Then** it shows the metrics together as an
   evidence bundle instead of isolated counters.
3. **Given** a finding has a plausible operational next step, **When** the
   analyst reads it, **Then** the finding includes concrete checks that can
   confirm or disprove the hypothesis.

---

### User Story 2 - Classify signals using expert diagnostic framing (Priority: P1)

A support engineer wants findings to follow the same mental model used by
experienced MySQL practitioners: utilization, saturation, and errors for each
database subsystem.

**Why this priority**: The Rosetta Stone notes encode expert triage structure.
Using that structure makes Advisor output easier to audit, extend, and trust.

**Independent Test**: Review Advisor output for captures that trigger each
signal family. Every finding is categorized as utilization, saturation, error,
or a documented combination, and the category matches the evidence shown.

**Acceptance Scenarios**:

1. **Given** a metric indicates capacity usage without pressure, **When** the
   Advisor evaluates it, **Then** the finding is categorized as utilization and
   does not overstate urgency.
2. **Given** a metric indicates queueing, waiting, flushing pressure, cache
   misses, or blocked work, **When** the Advisor evaluates it, **Then** the
   finding is categorized as saturation and receives an appropriate warning or
   critical severity.
3. **Given** an observed signal indicates failed work or explicit MySQL errors,
   **When** the Advisor evaluates it, **Then** the finding is categorized as an
   error and clearly explains the failed operation.

---

### User Story 3 - Prioritize the most actionable findings (Priority: P1)

A support engineer needs the Advisor to avoid a flat list of noisy warnings.
The highest-impact issues must appear first, and duplicate symptoms from the
same root pressure must be grouped or cross-referenced.

**Why this priority**: A busy incident report can contain many correlated
signals. Better prioritization helps the analyst focus on root causes instead
of chasing every secondary symptom.

**Independent Test**: Generate a report from a capture with correlated signals,
such as dirty-page pressure plus redo pressure, or metadata waits plus DDL
activity. The Advisor ranks the dominant finding ahead of weaker correlated
signals and avoids contradictory recommendations.

**Acceptance Scenarios**:

1. **Given** multiple findings from the same subsystem, **When** the Advisor
   sorts findings, **Then** critical and higher-confidence findings appear
   before informational or low-confidence findings.
2. **Given** two findings are likely related, **When** both are shown, **Then**
   the Advisor explains the relationship or uses one as supporting evidence for
   the other.
3. **Given** a finding has insufficient evidence, **When** the Advisor renders
   the final report, **Then** it is omitted or shown as informational rather
   than escalated as a warning.

---

### User Story 4 - Guide deeper investigation per subsystem (Priority: P2)

A support engineer needs findings to recommend focused follow-up checks for the
specific subsystem involved, such as checking table-cache misses for Opening
tables, semaphore wait locations for InnoDB contention, or row scan evidence
for query-shape problems.

**Why this priority**: A high-quality Advisor should not only say that
something is wrong; it should tell the analyst where to look next and what
evidence would confirm the diagnosis.

**Independent Test**: For each supported subsystem family, trigger a finding
and verify that the recommendation names the relevant follow-up checks and
does not recommend unrelated work.

**Acceptance Scenarios**:

1. **Given** table-cache pressure is detected, **When** the finding renders,
   **Then** it points to open-table misses, opening-table states, cache sizing,
   and file-descriptor constraints as relevant follow-up checks.
2. **Given** query-shape scan pressure is detected, **When** the finding
   renders, **Then** it points to expensive running queries, rows examined vs.
   rows sent, table structure, and execution-plan validation.
3. **Given** semaphore or metadata contention is detected, **When** the finding
   renders, **Then** it points to wait locations, related DDL activity, lock
   waits, and blocking transactions.

---

### User Story 5 - Preserve trust through transparency and restraint (Priority: P2)

A support engineer needs to trust that the Advisor is evidence-based, not
guessing. It should state when evidence is missing and avoid recommendations
that cannot be supported by the capture.

**Why this priority**: Overconfident findings can mislead incident response.
The Advisor must be explicit about evidence strength and missing inputs.

**Independent Test**: Generate reports from sparse captures, partial captures,
and captures with malformed optional metrics. The Advisor degrades gracefully,
keeps valid findings, and does not invent unavailable evidence.

**Acceptance Scenarios**:

1. **Given** a capture lacks a metric needed for a rule, **When** the Advisor
   evaluates that rule, **Then** it skips or downgrades the finding and does
   not fabricate a value.
2. **Given** a finding uses an inferred relationship, **When** it renders,
   **Then** the evidence text distinguishes direct measurements from inferred
   interpretation.
3. **Given** only partial evidence exists, **When** the Advisor renders, **Then**
   it still presents valid lower-confidence guidance without blocking the
   entire report.

### Edge Cases

- Captures may contain only a subset of MySQL collectors, leaving some
  subsystem checks without inputs.
- Some counters may be zero, unchanged, missing, malformed, or available only
  as point-in-time values rather than rates.
- Related symptoms may appear in multiple places, such as redo pressure,
  dirty-page pressure, and buffer pool free-page pressure.
- Very short capture windows may show misleading rates or insufficient trend
  confidence.
- Workloads with no active queries should not trigger query-shape findings.
- Healthy systems should still show useful OK or informational context without
  hiding critical findings from other subsystems.
- Findings must remain readable when many rules trigger at once.
- The Advisor must not require external references, network access, or manual
  access to the Rosetta Stone source file to explain a report.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The Advisor MUST evaluate MySQL findings using an explicit
  diagnostic frame of utilization, saturation, and errors.
- **FR-002**: Each visible finding MUST identify its subsystem, diagnostic
  category, severity, evidence, interpretation, and recommended next checks.
- **FR-003**: The Advisor MUST support expert-guided findings for at least
  these subsystem families: Buffer Pool, Redo Log, Flushing and Dirty Pages,
  Table Open Cache, Thread and Connection Handling, Temporary Tables, Query
  Shape, Semaphores and InnoDB Contention, Metadata and DDL Contention, and
  Binary Log Caches.
- **FR-004**: Buffer Pool findings MUST distinguish capacity pressure, read
  miss pressure, free-page pressure, and evidence of background flushing.
- **FR-005**: Redo Log findings MUST distinguish redo write volume, pending
  redo writes or fsyncs, checkpoint-age pressure when available, and error
  evidence related to redo capacity.
- **FR-006**: Flushing findings MUST correlate dirty-page pressure, free-page
  pressure, and write or checkpoint symptoms when the capture contains enough
  evidence.
- **FR-007**: Table Open Cache findings MUST consider table-open misses,
  overflows, opened-table growth, and processlist states related to opening
  tables.
- **FR-008**: Thread and Connection findings MUST consider connection pressure,
  thread creation churn, thread-cache effectiveness, aborted connections, and
  connection error categories when available.
- **FR-009**: Temporary Table findings MUST distinguish total temporary-table
  creation from disk temporary-table pressure and relate disk pressure to I/O
  symptoms when evidence exists.
- **FR-010**: Query Shape findings MUST consider full scans, full joins, high
  rows-examined-to-rows-sent signals, and slow observed active queries.
- **FR-011**: Semaphores and InnoDB Contention findings MUST surface wait
  volume, wait duration, and source-location evidence when present.
- **FR-012**: Metadata and DDL Contention findings MUST connect DDL activity,
  table-definition or metadata waits, metadata-lock waits, and prepared
  statement reprepare symptoms when evidence exists.
- **FR-013**: Binary Log Cache findings MUST surface transactional and
  statement cache disk use, capacity errors, and memory-risk context when the
  supporting metrics are present.
- **FR-014**: Each finding MUST list the concrete metrics or observed report
  facts it consumed so the analyst can verify the conclusion.
- **FR-015**: Each finding MUST include at least one recommended confirmation
  step and at least one recommended mitigation or investigation direction when
  the evidence supports one.
- **FR-016**: The Advisor MUST avoid duplicate or contradictory findings by
  grouping related evidence or cross-referencing correlated findings.
- **FR-017**: Finding severity MUST reflect both impact and evidence strength:
  critical for blocked or failed work, warning for active pressure, information
  for notable but non-urgent signals, and OK only when the evaluated signal is
  healthy.
- **FR-018**: The Advisor MUST skip or downgrade findings when required inputs
  are missing, while preserving other valid findings from the same report.
- **FR-019**: The Advisor MUST provide a compact executive summary that counts
  critical, warning, informational, and OK findings and highlights the top
  suspected incident drivers.
- **FR-020**: Advisor output MUST remain deterministic for the same input
  capture, including finding order, wording, severity labels, and evidence
  ordering.
- **FR-021**: The Advisor MUST remain usable in a self-contained report without
  external network access or links required to understand the finding.
- **FR-022**: The Advisor MUST document the expert-source coverage so future
  reviewers can trace which Rosetta Stone topics are covered, deferred, or
  intentionally excluded.

### Key Entities

- **Advisor Signal**: A raw or derived observation from the capture that can
  support a finding. Key attributes include subsystem, diagnostic category,
  metric name or observed fact, value, threshold band, timestamp range, and
  confidence.
- **Diagnostic Finding**: A user-visible Advisor conclusion. Key attributes
  include subsystem, category, severity, title, explanation, evidence bundle,
  confidence, related findings, and next checks.
- **Evidence Bundle**: The grouped facts that justify a finding. It may contain
  rates, point-in-time values, processlist states, row-count ratios, wait
  locations, or observed error text.
- **Recommendation**: A focused next action tied to a finding. Key attributes
  include confirmation check, investigation direction, mitigation hint, and any
  scope caution.
- **Expert Coverage Map**: The traceable mapping from Rosetta Stone topics to
  Advisor coverage status. Key attributes include topic, subsystem, diagnostic
  category, covered signals, deferred signals, and rationale.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For a curated set of expert scenarios, at least 90% of expected
  subsystem findings appear with the correct subsystem, category, severity, and
  evidence.
- **SC-002**: In sparse or partial captures, 100% of findings that lack required
  evidence are skipped or downgraded rather than rendered as unsupported
  warnings.
- **SC-003**: For captures with correlated symptoms, at least 80% of duplicated
  or secondary symptoms are grouped, cross-referenced, or ranked below the
  dominant suspected incident driver.
- **SC-004**: A support engineer can identify the top suspected incident driver
  and the next two confirmation checks within 60 seconds of opening the report.
- **SC-005**: Rendering the same capture twice produces byte-identical Advisor
  output.
- **SC-006**: Every visible non-OK finding includes at least two concrete
  evidence items or one direct error signal plus one recommended next check.
- **SC-007**: The expert coverage map accounts for 100% of topics selected from
  the Rosetta Stone source for this feature as covered, deferred, or excluded.

## Assumptions

- The first enhancement scope is the Advisor output in generated MySQL reports,
  not a new collector or an interactive external service.
- The Rosetta Stone source is treated as expert guidance for diagnostic
  framing; final findings must still be supported by evidence available in the
  capture.
- Existing report data may not cover every Rosetta Stone topic, so the feature
  will explicitly document deferred topics instead of inventing signals.
- The Advisor should prefer fewer, stronger findings over a large list of weak
  warnings.
- Thresholds may use conservative defaults when exact values are not available,
  provided the finding explains the evidence and confidence level.
- Durable artifacts for this feature are written in English.
