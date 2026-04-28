# Phase 1 Package Contract: Richer Processlist Metrics

## `parse` package

### `parseProcesslist`

Input: an `io.Reader` containing pt-stalk `-processlist` output in vertical
`SHOW FULL PROCESSLIST \G` format.

Output: `*model.ProcesslistData` with one `ThreadStateSample` per `TS` block.

Additional contract for this feature:

- Parse `Time`, `Time_ms`, `Rows_sent`, `Rows_examined`, and `Info` fields when
  present.
- Preserve existing state/user/host/command/db bucketing behavior.
- Do not fail the sample when optional numeric fields are missing or malformed.
- Compute the richer metrics on row flush so each completed row contributes at
  most once.
- Leave `SnapshotBoundaries` unset; merged boundaries remain render-owned.

## `model` package

### `ThreadStateSample`

`ThreadStateSample` adds exported fields for:

- `TotalThreads`
- `ActiveThreads`
- `SleepingThreads`
- `MaxTimeMS`
- `MaxRowsExamined`
- `MaxRowsSent`
- `RowsWithQueryText`

Each exported field must have a godoc comment and must marshal
deterministically in existing golden tests.

## `render` package

### `concatProcesslist`

Merging multiple `ProcesslistData` values must preserve the richer per-sample
fields exactly while continuing to union label sets and set
`SnapshotBoundaries`.

### `processlistChartPayload`

The payload must keep the existing `timestamps`, `dimensions`, and
`snapshotBoundaries` keys. It must add:

- an `Activity` dimension containing active, sleeping, and total thread counts
- a `metrics` object containing longest age in seconds, peak rows examined,
  peak rows sent, and query-text row count arrays

### `summariseProcesslist`

The summary helper must return formatted peak values for the Processlist
subview callouts. It must accept empty data and return an empty summary rather
than panicking.
