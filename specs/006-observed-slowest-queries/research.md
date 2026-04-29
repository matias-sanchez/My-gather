# Phase 0 Research: Observed Slowest Processlist Queries

## Decision: Report observed in-flight processlist queries

**Decision**: Treat the feature as "slowest observed active processlist
queries" rather than slow-query-log or pt-query-digest analysis.

**Rationale**: The current pt-stalk collections supported by the report already
include `-processlist` snapshots. The inspected incident folder contains no
slow-query-log or pt-query-digest artifacts, so processlist rows are the
available query-level evidence. `Time_ms` and `Time` describe the current row's
observed age at capture time, not completed execution duration.

**Alternatives considered**:

- Slow-query-log parser: rejected for this feature because the artifact is not
  present in the current supported collector set.
- pt-query-digest parser: rejected for this feature because no digest artifact
  is present in the capture and it would introduce a separate collector scope.

## Decision: Exclude idle and background rows from ranking

**Decision**: Eligible rows must have non-empty query text and active SQL
commands. Rows with `Sleep`, `Daemon`, or empty commands are excluded from the
slowest-query ranking.

**Rationale**: Long ages on sleeping sessions or background daemons are not
slow queries. In the motivating capture, `event_scheduler` daemon rows have the
largest age and would otherwise drown out blocked SQL evidence.

**Alternatives considered**:

- Rank every row by age: rejected because it misclassifies idle/background
  state as query slowness.
- Exclude only `Sleep`: rejected because daemon/background rows can also have
  very large non-query ages.

## Decision: Group by normalized query fingerprint

**Decision**: Normalize query text by trimming whitespace, collapsing internal
whitespace, lowercasing, and replacing quoted strings and numeric literals with
placeholders. Compute a stable hash from that normalized form.

**Rationale**: Processlist captures can see the same query across many
seconds. Grouping repeated and literal-varied sightings keeps the report
bounded and highlights query shapes instead of duplicates.

**Alternatives considered**:

- Group by raw query text: rejected because literal-only differences create too
  many rows.
- Full SQL parser: rejected because standard-library text normalization is
  sufficient for a bounded incident report and avoids a new dependency.

## Decision: Keep query text bounded but visible

**Decision**: Render a compact snippet for each top query summary and a stable
fingerprint. Do not render every raw processlist row.

**Rationale**: Support engineers need enough identity to act, but the previous
processlist feature intentionally avoided a raw SQL dump. A bounded top-N table
keeps the feature useful while respecting the report's privacy posture.

**Alternatives considered**:

- Hide query text entirely: rejected because fingerprints alone are hard to act
  on during an incident.
- Render full SQL for all sightings: rejected because it expands sensitive data
  exposure and makes the Processlist subview noisy.

## Decision: Extend the existing processlist model

**Decision**: Add bounded query summary fields to the existing
`ProcesslistData` model and compute summaries during parsing and render-layer
concatenation.

**Rationale**: This keeps one canonical processlist path and satisfies the
constitution's canonical-code-path requirement. The existing parser already
extracts the fields needed for eligibility, age, row counters, and query text.

**Alternatives considered**:

- Separate query parser: rejected because it would duplicate processlist
  parsing.
- Browser-side grouping from all raw rows: rejected because it would inflate
  the report payload and expose more raw query text than necessary.
