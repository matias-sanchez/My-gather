# Phase 1 UI Contract: Observed Slowest Processlist Queries

## Scope

This contract extends the existing Database Usage Processlist subview. It does
not add a new top-level report section.

## Processlist subview additions

When processlist data contains eligible active query sightings, the subview
must render a compact, independently collapsible "Slowest observed queries"
panel below the thread-state chart.

The panel must:

- use native `<details>`/`<summary>` semantics
- default open when query rows exist
- show the total number of observed query summaries in the summary
- remain readable when JavaScript is disabled

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

## Filters

The panel must include client-side filters for:

- query shape or fingerprint
- user
- database
- state

Filtering must:

- combine populated fields with AND semantics
- use data attributes on each rendered row instead of scraping column layout
- update a visible row count
- reveal a filtered-empty state when no row matches
- keep the full server-rendered table in the HTML for no-JavaScript fallback

## Empty state

When processlist data exists but there are no eligible active rows with query
text, the subview must render a short empty-state message instead of an empty
table.

## Advisor contract

The Advisor must add a Query Shape finding when the slowest observed processlist
query is at least 60 seconds old. The finding is:

- Warning for generic long-running observed active queries
- Critical for metadata-lock wait states

The finding must include the fingerprint, max age, sighting count, user,
database, state, and bounded query shape.

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
