# Feature Specification: Observed Slowest Processlist Queries

**Feature Branch**: `006-observed-slowest-queries`  
**Created**: 2026-04-28  
**Status**: Draft  
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

## Assumptions

- This feature reports slowest **observed in-flight processlist rows**, not
  completed slow-query-log events.
- The existing Processlist subview is the correct home for the feature; no new
  top-level report section is required.
- Query snippets are useful enough when combined with stable fingerprints and
  context fields.
- The report should default to a small bounded list so query evidence is
  actionable during an incident and does not dominate the page.
