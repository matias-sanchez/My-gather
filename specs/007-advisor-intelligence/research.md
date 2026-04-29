# Phase 0 Research: Advisor Intelligence

## Decision: Use the existing native Advisor registry as the only rule engine

**Rationale**: The repository already has a deterministic native registry in
`findings/register.go`, Rosetta-derived subsystem files in `findings/rules_*.go`,
quality tests, and a rendered Advisor section. Extending this path satisfies
the canonical-code-path requirement and keeps rule authorship local to the
package that already owns Advisor logic.

**Alternatives considered**:

- Add a second data-driven rules engine: rejected because it would duplicate
  behavior and create ordering and fallback risks.
- Load the Rosetta Stone text at runtime: rejected because reports must be
  self-contained and runtime behavior must not depend on a local Downloads
  file.
- Keep only ad hoc rule text in each rule: rejected because it does not provide
  coverage traceability or consistent diagnostic framing.

## Decision: Represent Rosetta Stone coverage as durable feature metadata

**Rationale**: The source document contains expert topics across Buffer Pool,
Redo Log, Change Buffer, Semaphores, Data Dictionary, Table Open Cache, Thread
Cache, Binlog Cache, temporary tables, connection errors, and query-shape
signals. Some topics are already covered, some are partially supported by
current inputs, and some lack capture evidence. A coverage map lets reviewers
see what is covered, deferred, or intentionally excluded without depending on
the original file.

**Alternatives considered**:

- Treat the source file as the coverage map: rejected because it is outside the
  repository and not available to every contributor.
- Add coverage comments only in code: rejected because reviewers need a
  feature-level view before implementation.
- Require full Rosetta Stone coverage in one PR: rejected because some topics
  require inputs not currently parsed.

## Decision: Add explicit diagnostic category and evidence strength to findings

**Rationale**: The user-facing goal is better intelligence, not just more
rules. Categorizing findings as utilization, saturation, error, or a documented
combination mirrors expert MySQL triage and helps analysts understand whether a
finding indicates capacity use, actual pressure, or failed work. Evidence
strength prevents weak signals from being presented as urgent findings.

**Alternatives considered**:

- Infer category from subsystem name: rejected because one subsystem can have
  utilization, saturation, and error findings.
- Encode category only in prose: rejected because sorting, summaries, tests,
  and future rule quality checks need structured metadata.
- Treat every threshold crossing as warning: rejected because this overstates
  utilization-only signals.

## Decision: Preserve deterministic severity ordering, then add driver ranking

**Rationale**: Existing Advisor output sorts deterministically by subsystem,
severity, and rule ID. The enhanced experience needs a compact top-driver
summary without destabilizing the full card list. The top-driver view should be
derived from visible findings using deterministic severity, evidence strength,
category, and stable ID tie-breakers.

**Alternatives considered**:

- Replace the full Advisor ordering with global impact order: rejected because
  subsystem grouping is easier to scan and already matches the Rosetta Stone
  model.
- Use probabilistic scoring: rejected because it is hard to test and explain.
- Let every rule choose its own ranking text: rejected because cross-rule
  prioritization needs consistent inputs.

## Decision: Correlate related findings through explicit relationships

**Rationale**: Many MySQL symptoms are related: dirty-page pressure can support
flushing and redo pressure, metadata-lock waits can support DDL and table-cache
findings, and table scans can support slow observed queries. Explicit related
finding IDs let the Advisor cross-reference supporting evidence without hiding
the underlying findings or inventing a root cause.

**Alternatives considered**:

- Suppress secondary findings automatically: rejected because analysts may need
  the supporting evidence.
- Merge all related rules into one large rule: rejected because it would make
  rules harder to test and maintain.
- Leave correlation entirely to the reader: rejected because the feature goal is
  more intelligent triage.

## Decision: Keep Advisor UI in the existing section with richer card anatomy

**Rationale**: Principle XI requires reports to stay useful under pressure. The
Advisor should gain a compact executive summary, top suspected drivers,
diagnostic category labels, evidence bundles, and next checks inside the
current Advisor section. This avoids a new top-level report section while
making findings more actionable.

**Alternatives considered**:

- Add a new "Expert Advisor" section: rejected because it duplicates the
  existing Advisor surface.
- Render an exhaustive rules table first: rejected because it crowds out the
  primary narrative.
- Hide detailed evidence behind an external link: rejected because the report
  must work offline.

## Decision: Use focused synthetic scenarios plus existing goldens for testing

**Rationale**: Existing fixtures remain the source of full-report regression
coverage. Some expert scenarios need narrow synthetic report models to produce
specific combinations of metrics that may not exist in committed pt-stalk
fixtures. Focused unit tests can exercise rule classification, missing-input
behavior, correlation, and ranking, while goldens protect rendered output.

**Alternatives considered**:

- Only update the full HTML golden: rejected because failures would be too wide
  to diagnose.
- Require new real pt-stalk fixtures for every rule: rejected because many rule
  scenarios are combinations of counters, states, and variables that are easier
  and clearer to model directly.
- Skip render tests for UI additions: rejected because Advisor rendering is a
  user-facing contract.
