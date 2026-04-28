# Phase 0 Research: Richer Processlist Metrics

## Decision: Reuse the existing Processlist subview

**Decision**: Add richer metrics to the existing Database Usage Processlist
subview rather than creating a new top-level section.

**Rationale**: The metrics are derived entirely from `-processlist` and answer
the same operational question as the current thread-state chart. Keeping them
together satisfies Principle XI by avoiding a second place to look for the
same collector.

**Alternatives considered**:

- New top-level "Query Activity" section: rejected because it would duplicate
  Processlist context and overstate the scope of a single-collector feature.
- Raw SQL table: rejected because query text can be sensitive and the feature
  only needs presence counts.

## Decision: Extend the existing typed sample model

**Decision**: Add richer metric fields to `model.ThreadStateSample`.

**Rationale**: The current parser already emits one `ThreadStateSample` per
processlist timestamp block. The new values are also per-sample aggregates, so
adding fields keeps the model simple and avoids a parallel processlist metrics
type that would need separate merge/render logic.

**Alternatives considered**:

- Separate `ProcesslistMetricsData`: rejected because it would duplicate
  timestamp and boundary ownership.
- Store raw rows in the model: rejected because it would inflate reports and
  make sensitive query text more prominent.

## Decision: Prefer Time_ms, fall back to Time

**Decision**: Compute longest thread age from `Time_ms` when available. If a
row has no valid `Time_ms`, use `Time` seconds converted to milliseconds.

**Rationale**: MySQL vertical processlist captures commonly include both. The
millisecond field is more precise, while the second-level field preserves useful
signal in older or reduced outputs.

**Alternatives considered**:

- Use `Time` only: rejected because it discards available precision.
- Emit diagnostics for malformed optional numeric fields: rejected because
  malformed optional counters are common in real captures and should not crowd
  diagnostics when the rest of the row is valid.

## Decision: Add an Activity dimension and separate metrics payload

**Decision**: Render active, sleeping, and total threads as a new Processlist
chart dimension. Expose longest age, rows examined, rows sent, and query-text
row count as a separate non-stacked metrics payload with summary callouts.

**Rationale**: Active/sleeping/total are thread counts and fit the existing
stacked-bar Processlist chart. Age and row counts use different units, so they
should not be stacked with state/user/host dimensions. Summary callouts surface
their peak values immediately.

**Alternatives considered**:

- Put all metrics into the existing stacked chart: rejected because mixing
  threads, milliseconds, and row counts in one stacked chart is misleading.
- Build a second full chart in the first iteration: rejected to keep the visual
  change compact; the payload leaves room for a future small multiple if needed.

## Decision: Use minimized synthetic tests plus updated existing golden

**Decision**: Add focused synthetic parser tests for edge cases and update the
existing processlist golden to include the new fields.

**Rationale**: The real incident folder is useful for manual validation but is
not safe to commit as a fixture because it contains hostnames, paths, users,
and query text. A minimized fixture gives deterministic coverage without
importing sensitive incident data.

**Alternatives considered**:

- Commit raw incident processlist files: rejected for privacy and repository
  size reasons.
- Rely only on manual validation against the incident folder: rejected because
  the constitution requires fixtures and golden coverage.
