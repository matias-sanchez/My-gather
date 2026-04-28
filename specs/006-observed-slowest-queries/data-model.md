# Phase 1 Data Model: Observed Slowest Processlist Queries

`ProcesslistData` remains the typed payload for merged processlist data. The
feature extends it with a bounded list of grouped observed query summaries.

## ObservedProcesslistQuery

Represents one grouped query shape observed in active processlist rows.

| Field | Type | Meaning |
|-------|------|---------|
| `Fingerprint` | `string` | Stable identifier derived from normalized query shape |
| `Snippet` | `string` | Bounded human-readable query snippet |
| `FirstSeen` | `time.Time` | Earliest sample timestamp for this fingerprint |
| `LastSeen` | `time.Time` | Latest sample timestamp for this fingerprint |
| `SeenSamples` | `int` | Number of eligible sightings grouped into this summary |
| `MaxTimeMS` | `float64` | Largest observed row age in milliseconds |
| `HasTimeMetric` | `bool` | Whether `MaxTimeMS` is observed rather than default |
| `MaxRowsExamined` | `float64` | Largest observed `Rows_examined` value |
| `HasRowsExaminedMetric` | `bool` | Whether rows examined was observed |
| `MaxRowsSent` | `float64` | Largest observed `Rows_sent` value |
| `HasRowsSentMetric` | `bool` | Whether rows sent was observed |
| `User` | `string` | User from the slowest sighting |
| `DB` | `string` | Database from the slowest sighting |
| `Command` | `string` | Command from the slowest sighting |
| `State` | `string` | State from the slowest sighting |

## Validation and Ranking Rules

- Eligible rows require non-empty, non-`NULL` query text.
- Eligible rows exclude `Sleep`, `Daemon`, and empty command values.
- Age uses valid `Time_ms` first, then valid `Time` seconds converted to
  milliseconds.
- Query snippets are whitespace-collapsed and bounded.
- Fingerprints are stable for normalized query shapes.
- Summaries are sorted by max age descending, then rows examined descending,
  then rows sent descending, then fingerprint ascending.
- The rendered report keeps only a bounded top list.

## Payload Shape

The existing `processlist` chart payload adds:

```json
{
  "slowQueries": [
    {
      "fingerprint": "q_1234567890abcdef",
      "snippet": "select count(*) from table where id = ?",
      "firstSeen": 1769702258,
      "lastSeen": 1769702719,
      "seenSamples": 12,
      "maxTimeSeconds": 3723.6,
      "hasTimeMetric": true,
      "maxRowsExamined": 0,
      "hasRowsExamined": true,
      "maxRowsSent": 0,
      "hasRowsSent": true,
      "user": "app",
      "db": "appdb",
      "command": "Query",
      "state": "Waiting for table metadata lock"
    }
  ]
}
```

The existing `timestamps`, `dimensions`, `metrics`, and `snapshotBoundaries`
fields remain unchanged.
