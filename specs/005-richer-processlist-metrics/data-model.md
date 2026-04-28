# Phase 1 Data Model: Richer Processlist Metrics

## Existing entity: ProcesslistData

`ProcesslistData` remains the typed payload for a merged `-processlist`
collector. It continues to own:

- `ThreadStateSamples`
- label orders for `States`, `Users`, `Hosts`, `Commands`, and `Dbs`
- `SnapshotBoundaries`

No new top-level processlist model is introduced.

## Extended entity: ThreadStateSample

Each `ThreadStateSample` represents one timestamp block in a processlist file.
It continues to include the existing count maps:

- `StateCounts`
- `UserCounts`
- `HostCounts`
- `CommandCounts`
- `DbCounts`

The feature adds these aggregate fields:

| Field | Type | Meaning | Source field |
|-------|------|---------|--------------|
| `TotalThreads` | `int` | Number of rows seen in the sample | row count |
| `ActiveThreads` | `int` | Rows whose command is not `Sleep` | `Command` |
| `SleepingThreads` | `int` | Rows whose command is exactly `Sleep` | `Command` |
| `MaxTimeMS` | `float64` | Longest row age in milliseconds | `Time_ms`, fallback `Time` |
| `HasTimeMetric` | `bool` | Whether `MaxTimeMS` is an observed metric, not a default zero | `Time_ms`, fallback `Time` |
| `MaxRowsExamined` | `float64` | Largest rows-examined value in the sample | `Rows_examined` |
| `HasRowsExaminedMetric` | `bool` | Whether `MaxRowsExamined` is an observed metric, not a default zero | `Rows_examined` |
| `MaxRowsSent` | `float64` | Largest rows-sent value in the sample | `Rows_sent` |
| `HasRowsSentMetric` | `bool` | Whether `MaxRowsSent` is an observed metric, not a default zero | `Rows_sent` |
| `RowsWithQueryText` | `int` | Rows with non-empty, non-`NULL` `Info` | `Info` |
| `HasQueryTextMetric` | `bool` | Whether `RowsWithQueryText` is an observed metric, not a default zero | `Info` |

### Validation and derivation rules

- `TotalThreads` increments for every completed processlist row that contains
  at least one core row field (`State`, `User`, `Host`, `Command`, or `db`).
  Optional-only fragments (`Time`, `Time_ms`, `Rows_*`, or `Info` without a
  core row field) do not create rows or contribute metrics.
- `SleepingThreads` increments only when `Command == "Sleep"`.
- `ActiveThreads` increments for all other completed rows, including missing or
  empty command values.
- `MaxTimeMS` uses `Time_ms` when present and valid. If `Time_ms` is absent or
  invalid, a valid `Time` value is converted from seconds to milliseconds.
- `MaxRowsExamined` and `MaxRowsSent` use only valid non-negative numeric
  values. Missing or malformed values do not emit diagnostics.
- `RowsWithQueryText` increments when `Info` is neither empty nor `NULL` after
  trimming whitespace.
- `Has*Metric` flags are true only when at least one source field for that
  metric was observed and parsed, so renderers can distinguish unavailable
  optional data from a real zero value.

## Template entity: ProcesslistSummary

The template view receives a compact summary derived from all samples:

| Field | Meaning |
|-------|---------|
| `PeakActive` | Maximum `ActiveThreads` across samples |
| `PeakSleeping` | Maximum `SleepingThreads` across samples |
| `LongestAge` | Maximum `MaxTimeMS`, formatted in seconds |
| `HasLongestAge` | Whether the longest-age callout is available |
| `PeakRowsExamined` | Maximum `MaxRowsExamined`, formatted as a count |
| `HasPeakRowsExamined` | Whether the rows-examined callout is available |
| `PeakRowsSent` | Maximum `MaxRowsSent`, formatted as a count |
| `HasPeakRowsSent` | Whether the rows-sent callout is available |
| `PeakQueryTextRows` | Maximum `RowsWithQueryText` across samples |
| `HasPeakQueryTextRows` | Whether the query-text count callout is available |
| `SampleCount` | Number of processlist samples |

## Embedded chart payload

The existing `processlist` payload keeps the current shape and adds:

```json
{
  "activity": {
    "series": [
      {"label": "Active", "values": [0]},
      {"label": "Sleeping", "values": [0]},
      {"label": "Total", "values": [0]}
    ]
  },
  "metrics": {
    "maxTimeSeconds": [0],
    "hasMaxTimeSeconds": [true],
    "maxRowsExamined": [0],
    "hasRowsExamined": [true],
    "maxRowsSent": [0],
    "hasRowsSent": [true],
    "rowsWithQueryText": [0],
    "hasQueryText": [true]
  }
}
```

The existing `timestamps`, `dimensions`, and `snapshotBoundaries` fields remain
unchanged so current chart behavior stays compatible.
