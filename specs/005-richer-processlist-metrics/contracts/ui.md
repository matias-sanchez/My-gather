# Phase 1 UI Contract: Processlist Subview

## Scope

This contract extends the existing Database Usage Processlist subview. It does
not add a new top-level report section.

## Summary callouts

When processlist data exists, the subview must render compact callouts before
the chart:

- peak active threads
- peak sleeping threads
- longest thread age
- peak rows examined
- peak rows sent
- peak query-text rows
- samples

Each callout is omitted only if the underlying metric is unavailable. A metric
with a real zero value may render as `0`.

Optional metrics are available only when at least one sample contains the
corresponding source field:

- longest thread age requires a valid `Time_ms` or `Time`
- peak rows examined requires a valid `Rows_examined`
- peak rows sent requires a valid `Rows_sent`
- peak query-text rows requires an `Info` field

## Chart controls

The existing Processlist chart dimension toolbar must continue to provide:

- State
- User
- Host
- Command
- db

The feature adds:

- Activity

The Activity view shows active, sleeping, and total thread counts on the same
timeline as the other Processlist dimensions. It uses the same offline chart
library and embedded payload as the existing views.

## Payload contract

The browser receives all values from the existing `report-data` JSON script.
The report must not fetch any processlist data at view time.

The `processlist.metrics` object is reserved for non-stacked metric arrays. The
first implementation may use it for summaries and hover/detail text; future UI
work can render it as a separate mini chart without changing the parser model.

## Empty and degraded states

- If `-processlist` is absent or unparseable, the existing missing-data banner
  remains the only visible Processlist body.
- If richer optional fields are absent, the Processlist chart still renders the
  existing dimensions.
- The subview must not expose a new raw SQL table.
