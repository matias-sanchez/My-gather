# Feature Specification: Richer Processlist Metrics

**Feature Branch**: `005-richer-processlist-metrics`  
**Created**: 2026-04-28  
**Status**: Complete  
**Input**: User description: "Implement richer processlist metrics so support engineers can see active versus sleeping threads, longest thread age, peak Rows_examined/Rows_sent, and query text presence trends from pt-stalk processlist captures."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Spot active thread pressure quickly (Priority: P1)

A support engineer opens a generated report from a pt-stalk collection and
needs to know whether the server was overloaded by active work or merely had
many idle client sessions. The Processlist subview must make this distinction
visible without requiring the engineer to inspect raw `-processlist` files.

**Why this priority**: The incident that motivated this feature was triggered
by high `Threads_connected`. Active versus sleeping split is the fastest way
to tell whether that trigger represents real workload pressure or connection
pool/idleness pressure.

**Independent Test**: Generate a report from a fixture containing mixed
`Sleep`, `Query`, and other command rows. The Processlist subview shows active
threads, sleeping threads, and total threads over time with deterministic
summary values.

**Acceptance Scenarios**:

1. **Given** a processlist sample containing sleeping and non-sleeping
   commands, **When** the report renders, **Then** the Processlist subview
   shows total, active, and sleeping thread counts for that sample.
2. **Given** several processlist samples with changing command mix, **When**
   the engineer views the Processlist subview, **Then** the active/sleeping
   trend is visible across the capture window and can be compared with the
   existing state/user/host/command/db breakdowns.
3. **Given** a processlist row with an empty or missing command, **When** the
   sample is parsed, **Then** the row is counted as active unless the command is
   explicitly `Sleep`, and no parser failure is emitted solely for that row.

---

### User Story 2 - Identify long-running and high-row work (Priority: P2)

A support engineer needs to find whether the processlist contains work that
has been running for a long time or scanning/sending unusually many rows. The
report must surface the largest `Time_ms`, `Rows_examined`, and `Rows_sent`
signals per sample so expensive work is visible even when thread state labels
are generic.

**Why this priority**: Long-running queries and high row counts are common
root-cause signals for MySQL incidents. The raw processlist already contains
these fields, but the current report does not promote them.

**Independent Test**: Generate a report from a fixture with known `Time_ms`,
`Rows_examined`, and `Rows_sent` values. The parser exposes per-sample maxima,
and the rendered report shows those maxima with stable formatting.

**Acceptance Scenarios**:

1. **Given** a processlist sample with `Time_ms` values, **When** the report
   renders, **Then** the Processlist subview shows the longest thread age for
   the sample in seconds.
2. **Given** a processlist sample with `Rows_examined` and `Rows_sent` values,
   **When** the report renders, **Then** the Processlist subview shows the peak
   rows examined and peak rows sent for the sample.
3. **Given** a processlist sample where one or more of these numeric fields are
   absent or non-numeric, **When** the sample is parsed, **Then** valid fields
   from the same row and sample are still preserved, and a valid `Time` value
   is used when `Time_ms` is missing or invalid.

---

### User Story 3 - Understand query-text availability safely (Priority: P3)

A support engineer needs to know whether processlist rows include query text
without forcing the report to expose full SQL prominently. The report must
show query-text presence as a count so the engineer can decide whether raw
files need deeper review.

**Why this priority**: Query text can be sensitive. A compact count gives
useful visibility while avoiding a new prominent SQL dump in the report.

**Independent Test**: Generate a report from a fixture with a mix of `Info:
NULL`, empty `Info`, and non-empty `Info` rows. The report shows query-text
presence counts over time and does not add a raw SQL table.

**Acceptance Scenarios**:

1. **Given** a processlist sample containing rows with non-empty, non-`NULL`
   `Info`, **When** the report renders, **Then** the Processlist subview shows
   the count of rows with query text for that sample.
2. **Given** a row whose `Info` value is `NULL` or empty, **When** the parser
   reads the row, **Then** the row is not counted as having query text.
3. **Given** a generated report, **When** the engineer opens the Processlist
   subview, **Then** the new query-text metric is visible as a count only; the
   feature does not introduce a new prominent raw SQL listing.

### Edge Cases

- Processlist rows may omit `Time_ms`, `Rows_examined`, `Rows_sent`, or `Info`
  while still containing state/user/host/command fields.
- `Time_ms` may be present as an integer or decimal value; the rendered age is
  shown in seconds.
- `Time` may be present with or without `Time_ms`; `Time_ms` is preferred when
  it is valid, and `Time` is used when `Time_ms` is missing or invalid.
- Empty, `NULL`, or whitespace-only `Info` values do not count as query text.
- Very large row counters remain numeric and deterministic in the embedded
  report payload.
- Existing processlist breakdowns by state/user/host/command/db must continue
  to render when new richer metrics are absent.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The report MUST expose active, sleeping, and total thread counts
  for every parsed processlist sample.
- **FR-002**: A processlist row MUST be classified as sleeping only when its
  command is `Sleep`; all other commands count as active.
- **FR-003**: The report MUST expose the longest thread age per processlist
  sample, using `Time_ms` when present and valid and falling back to `Time`
  when `Time_ms` is missing or invalid.
- **FR-004**: The report MUST expose peak `Rows_examined` and peak `Rows_sent`
  per processlist sample.
- **FR-005**: The report MUST expose the count of rows with non-empty query
  text per processlist sample, treating empty and `NULL` `Info` values as no
  query text.
- **FR-006**: The richer metrics MUST be embedded in the existing self-contained
  HTML report and MUST work offline with no external network or asset fetch.
- **FR-007**: Existing Processlist state/user/host/command/db breakdowns MUST
  remain available and deterministic.
- **FR-008**: Missing or malformed optional numeric processlist fields MUST NOT
  make the collection fail; valid processlist data from the same sample MUST
  still render.
- **FR-009**: The Processlist subview MUST include compact summary callouts for
  the peak active threads, peak sleeping threads, longest thread age, peak rows
  examined, peak rows sent, and peak query-text rows across the capture window.
- **FR-010**: The feature MUST include parser-level and render-level tests that
  cover all new metrics and protect the existing processlist breakdown.
- **FR-011**: Optional metric callouts MUST be omitted when their source fields
  are unavailable, while real zero values from available source fields MUST
  remain renderable as `0`.

### Key Entities

- **Processlist sample metrics**: Per-sample totals derived from all rows in a
  single processlist timestamp block. Attributes include timestamp, total
  threads, active threads, sleeping threads, longest age in milliseconds, peak
  rows examined, peak rows sent, and query-text row count.
- **Processlist summary metrics**: Capture-window maxima derived from all
  processlist samples. Attributes include peak active threads, peak sleeping
  threads, longest age, peak rows examined, peak rows sent, and peak query-text
  rows.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For a fixture containing mixed active and sleeping processlist
  rows, the generated report shows active, sleeping, and total counts matching
  the fixture exactly.
- **SC-002**: For a fixture containing known `Time_ms`, `Rows_examined`, and
  `Rows_sent` values, parser tests verify the expected per-sample maxima.
- **SC-003**: Rendering the same fixture twice produces byte-identical HTML
  output except for the existing explicitly allowed generated timestamp.
- **SC-004**: A real 20-snapshot pt-stalk collection with processlist files
  continues to generate a report successfully and includes the richer
  processlist payload.
- **SC-005**: No new runtime network dependency, external asset fetch, or write
  inside the input pt-stalk directory is introduced.

## Assumptions

- Support engineers need aggregate processlist signals first; full SQL text
  remains available in raw pt-stalk files and is not promoted into a new report
  table by this feature.
- `Time_ms` is the preferred age source because it has higher precision than
  `Time`; `Time` is acceptable as a fallback when `Time_ms` is missing or
  invalid.
- A command value of `Sleep` is the only sleeping classification for the
  active/sleeping split.
- The existing Processlist subview is the correct home for these metrics; no
  new top-level report section is required.
