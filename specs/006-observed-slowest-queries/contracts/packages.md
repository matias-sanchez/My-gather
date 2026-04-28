# Phase 1 Package Contracts: Observed Slowest Processlist Queries

## `parseProcesslist`

Input: an `io.Reader` containing pt-stalk `-processlist` output in vertical
format.

Output: `*model.ProcesslistData` plus structured diagnostics.

Additional contract:

- Continue parsing existing thread-state dimensions.
- Track eligible active query sightings while reading rows.
- Exclude `Sleep`, `Daemon`, and empty commands from slowest-query ranking.
- Ignore empty or `NULL` `Info` values for slowest-query ranking.
- Use valid `Time_ms`, falling back to valid `Time`, for observed age.
- Preserve valid row data when optional numeric fields are malformed.
- Emit deterministic grouped query summaries in `ProcesslistData`.

## `concatProcesslist`

Input: zero or more per-snapshot `*model.ProcesslistData` values.

Output: one merged `*model.ProcesslistData`.

Additional contract:

- Preserve existing sample concatenation and snapshot-boundary behavior.
- Merge observed query summaries from all inputs by fingerprint.
- Recompute first seen, last seen, sighting count, max age, and peak row
  counters across inputs.
- Sort and bound summaries deterministically after merging.

## `processlistChartPayload`

Input: merged `*model.ProcesslistData`.

Output: JSON-compatible map embedded in the report.

Additional contract:

- Continue emitting existing processlist chart fields.
- Add a bounded `slowQueries` array.
- Use seconds for rendered age values in the payload.
- Use Unix timestamps for first/last seen values.
