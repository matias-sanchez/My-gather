# Feature Specification: Observed Slowest Processlist Queries

**Feature Branch**: `006-observed-slowest-queries`  
**Created**: 2026-04-28  
**Status**: Complete
**Input**: User description: "Detect the slowest queries obtained or reported in pt-stalk captures and add this to the report using the spec-driven workflow."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Find slow in-flight queries quickly (Priority: P1)

A support engineer opens a generated report from a pt-stalk collection and
needs to see which active SQL statements were observed running the longest
during the capture window. The report must show this without requiring the
engineer to manually scan every raw `-processlist` file.

**Why this priority**: Processlist captures often hold the only query-level
evidence available during an incident. Surfacing the slowest observed active
queries turns the current aggregate processlist metrics into actionable
diagnostic evidence.

**Independent Test**: Generate a report from a fixture containing several
active `Query` rows with different `Time_ms` values. The Processlist subview
shows the highest-aged query fingerprints in deterministic order.

**Acceptance Scenarios**:

1. **Given** a processlist capture with active `Query` rows containing query
   text and `Time_ms`, **When** the report renders, **Then** the Processlist
   subview lists the slowest observed query fingerprints with their maximum
   observed age.
2. **Given** repeated sightings of the same query text across adjacent
   processlist samples, **When** the report renders, **Then** the repeated
   sightings are grouped into one row with first seen, last seen, sample count,
   and maximum observed age.
3. **Given** a capture containing `Sleep` or `Daemon` rows with very large
   ages, **When** slowest queries are ranked, **Then** those idle or background
   rows do not outrank active SQL statements.

---

### User Story 2 - Preserve row-count and wait-state evidence (Priority: P2)

A support engineer needs to distinguish a long-running query that is blocked
from a query that is actively scanning or sending many rows. The slowest-query
summary must keep the strongest row-count and state evidence seen for each
query fingerprint.

**Why this priority**: Long age alone can be misleading. Row counters and
thread state clarify whether the query is blocked, scanning, sending results,
or waiting on a lock.

**Independent Test**: Generate a report from a fixture where the same query
fingerprint appears with changing states and row counters. The report shows
the highest rows examined, highest rows sent, and representative state.

**Acceptance Scenarios**:

1. **Given** repeated sightings of the same query with increasing
   `Rows_examined`, **When** the summary groups the query, **Then** the row
   shows the peak `Rows_examined` value across all sightings.
2. **Given** repeated sightings of the same query with multiple states,
   **When** the report renders, **Then** the row shows the state associated
   with the slowest observed sighting and keeps a count of total sightings.
3. **Given** malformed or missing optional row counters, **When** the capture
   is parsed, **Then** valid age and query evidence from the same row remains
   available.

---

### User Story 3 - Keep query text useful but controlled (Priority: P3)

A support engineer needs enough query identity to act, but the report must not
turn into an unbounded raw SQL dump. The report must show compact query
snippets and stable fingerprints while bounding the amount of SQL text exposed.

**Why this priority**: The existing richer processlist feature intentionally
avoids a raw SQL table because query text can be sensitive. This feature should
add targeted evidence without weakening that privacy posture.

**Independent Test**: Generate a report from a fixture with long query text and
repeated whitespace/literal variants. The report shows bounded snippets,
stable fingerprints, and grouped variants deterministically.

**Acceptance Scenarios**:

1. **Given** a processlist row with long query text, **When** the report
   renders, **Then** the visible query snippet is bounded to a fixed maximum
   length.
2. **Given** two query texts that differ only by whitespace or literal values,
   **When** the summary computes fingerprints, **Then** they group together as
   the same query shape.
3. **Given** a generated report, **When** the engineer opens the Processlist
   subview, **Then** only the bounded top query summaries are shown, not every
   raw processlist row.

---

### User Story 4 - Work the slow-query list interactively (Priority: P1)

A support engineer needs to narrow the slowest observed query table while
reviewing an incident. The panel must be easy to collapse when the chart is the
focus, and easy to filter when query-level evidence is the focus.

**Why this priority**: The slow-query table can contain wide SQL snippets and
incident context. Clear collapse and filtering controls keep the report usable
without removing the evidence.

**Independent Test**: Render a report with several observed query rows and
assert that the Processlist subview includes an independently collapsible query
panel plus search/filter controls targeting user, database, state, and query
shape fields.

**Acceptance Scenarios**:

1. **Given** slowest observed queries exist, **When** the Processlist subview
   renders, **Then** the slow-query panel is an independently collapsible
   section with a clear summary label and count.
2. **Given** multiple slowest observed queries with different users,
   databases, states, and query snippets, **When** the engineer enters filter
   terms, **Then** only rows matching all populated filters remain visible.
3. **Given** no rows match the active filters, **When** the table updates,
   **Then** a clear filtered-empty state is shown without deleting the full
   server-rendered table.

---

### User Story 5 - Surface impactful observed queries in Advisor (Priority: P1)

A support engineer needs the report to call out when the observed slowest
queries themselves are likely incident drivers, especially long metadata-lock
waits or long-running active statements that appeared throughout the capture.

**Why this priority**: The table provides evidence, but the Advisor should
convert that evidence into a diagnostic finding so the report is smart enough
to guide triage.

**Independent Test**: Build a report with an observed processlist query older
than the configured threshold and assert that the Advisor emits a Query Shape
finding with the expected severity, metrics, and remediation guidance.

**Acceptance Scenarios**:

1. **Given** an observed query remains active for at least 60 seconds, **When**
   Advisor findings are computed, **Then** a Query Shape warning identifies the
   slowest observed query fingerprint and age.
2. **Given** the slowest observed query is waiting for table metadata lock,
   **When** Advisor findings are computed, **Then** the finding is Critical
   because the query is blocked and likely holding or waiting on DDL/MDL.
3. **Given** no observed active query reaches the configured age threshold,
   **When** Advisor findings are computed, **Then** no slow-observed-query
   finding is emitted.

### Edge Cases

- Processlist files may contain no active rows with non-empty query text.
- A row may have `Command: Query` but empty, whitespace-only, or `NULL` `Info`.
- `Time_ms` may be absent or malformed; valid `Time` remains a fallback.
- Background rows such as `event_scheduler` daemon can have very large ages
  but must not be ranked as slow SQL.
- Many repeated sightings of the same query must not produce duplicate report
  rows.
- Query text may contain newlines, extra whitespace, quoted literals, numeric
  literals, or very long statements.
- Large captures must keep the embedded report bounded and deterministic.
- Client-side filters must work without network access and must degrade to the
  fully rendered table when JavaScript is disabled.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The report MUST expose a bounded list of the slowest observed
  active processlist query fingerprints when active query text is present.
- **FR-002**: A processlist row MUST be eligible for slowest-query ranking only
  when it has non-empty, non-`NULL` `Info` text and its command represents
  active SQL work. `Sleep`, `Daemon`, and empty commands MUST be excluded.
- **FR-003**: Slowest-query age MUST use `Time_ms` when valid and fall back to
  `Time` seconds when `Time_ms` is missing or invalid.
- **FR-004**: Repeated sightings of the same query shape MUST be grouped under
  a stable fingerprint rather than rendered as duplicate rows.
- **FR-005**: Each grouped query summary MUST include maximum observed age,
  first seen timestamp, last seen timestamp, sighting count, user, database,
  state from the slowest sighting, peak `Rows_examined`, and peak `Rows_sent`
  when those values are available.
- **FR-006**: Visible query text MUST be bounded to a compact snippet and MUST
  not render every raw query row from the processlist capture.
- **FR-007**: Slowest-query rows MUST be sorted deterministically by maximum
  observed age descending, then peak rows examined descending, then peak rows
  sent descending, then fingerprint ascending.
- **FR-008**: Missing or malformed optional numeric fields MUST NOT make the
  collection fail; valid processlist data from the same row and sample MUST
  still render.
- **FR-009**: The feature MUST be embedded in the existing self-contained HTML
  report and MUST work offline with no external network or asset fetch.
- **FR-010**: The feature MUST include parser-level, merge-level, payload-level,
  and render-level tests that protect grouping, ranking, bounded text, and the
  existing processlist breakdown.
- **FR-011**: When no eligible query rows exist, the Processlist subview MUST
  show a clear empty state instead of an empty table.
- **FR-012**: The slowest-observed-query panel MUST be independently
  collapsible inside the Processlist subview and MUST show the number of
  observed query summaries in the collapsed summary.
- **FR-013**: The report MUST provide client-side filters for slowest observed
  queries by user, database, state, and query text/fingerprint.
- **FR-014**: Slow-query filters MUST combine with AND semantics across
  populated fields and MUST show a filtered-empty state when no rendered rows
  match.
- **FR-015**: The Advisor MUST emit a Query Shape finding when the slowest
  observed active query reaches at least 60 seconds.
- **FR-016**: The slow-observed-query Advisor finding MUST be Critical when the
  slowest observed query state indicates a metadata lock wait; otherwise it
  MUST be Warning for age at or above 60 seconds.
- **FR-017**: The Advisor finding MUST include the slowest query fingerprint,
  maximum age, sighting count, representative user/database/state, and bounded
  query shape in its metrics or explanation.

### Key Entities

- **Observed processlist query sighting**: One eligible active row observed in a
  processlist timestamp block. Attributes include timestamp, command, user,
  database, state, age, row counters, and query text.
- **Observed query summary**: A bounded grouped summary of one normalized query
  shape across the capture window. Attributes include fingerprint, snippet,
  first seen, last seen, sighting count, maximum age, peak row counters, and
  context from the slowest sighting.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For a fixture with known active processlist queries, the report
  lists the expected top query fingerprints in exact deterministic order.
- **SC-002**: For repeated sightings of the same query shape, the report shows
  one grouped row with correct first seen, last seen, sighting count, and
  maximum age.
- **SC-003**: For a fixture containing long-running `Sleep` and `Daemon` rows,
  those rows are excluded from slowest-query ranking.
- **SC-004**: Rendering the same fixture twice produces byte-identical HTML
  output except for the existing explicitly allowed generated timestamp.
- **SC-005**: A real multi-snapshot pt-stalk collection continues to generate a
  report successfully and includes bounded slowest-query summaries when active
  query text exists.
- **SC-006**: Slow-query table controls are present in the generated HTML and
  allow rows to be filtered by user, database, state, and query content without
  requiring any external asset.
- **SC-007**: A capture with a 60-second or older observed query produces an
  Advisor finding; a metadata-lock wait at that age produces a Critical
  finding.

## Assumptions

- This feature reports slowest **observed in-flight processlist rows**, not
  completed slow-query-log events.
- The existing Processlist subview is the correct home for the feature; no new
  top-level report section is required.
- Query snippets are useful enough when combined with stable fingerprints and
  context fields.
- The report should default to a small bounded list so query evidence is
  actionable during an incident and does not dominate the page.
- The slowest-query panel should default open when query evidence exists, but
  remain independently collapsible so readers can hide it while using the chart.
- The first Advisor threshold is intentionally conservative: 60 seconds is
  enough to be actionable in an incident processlist capture, while metadata
  lock waits are considered more severe than generic long-running work.
