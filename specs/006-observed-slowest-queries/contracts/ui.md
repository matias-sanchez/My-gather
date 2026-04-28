# Phase 1 UI Contract: Observed Slowest Processlist Queries

## Scope

This contract extends the existing Database Usage Processlist subview. It does
not add a new top-level report section.

## Processlist subview additions

When processlist data contains eligible active query sightings, the subview
must render a compact "Slowest observed queries" table below the thread-state
chart.

The table must include:

- maximum observed age
- first seen timestamp
- last seen timestamp
- sighting count
- user
- database
- state
- peak rows examined when available
- peak rows sent when available
- stable fingerprint
- bounded query snippet

## Empty state

When processlist data exists but there are no eligible active rows with query
text, the subview must render a short empty-state message instead of an empty
table.

## Privacy and size constraints

- The table must be bounded to a small top-N list.
- Query snippets must be length-bounded.
- The report must not render every raw processlist row.
- The report must remain self-contained and work offline.

## Sorting

Rows must appear in deterministic rank order:

1. maximum observed age descending
2. peak rows examined descending
3. peak rows sent descending
4. fingerprint ascending
