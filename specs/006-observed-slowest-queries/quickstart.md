# Quickstart: Observed Slowest Processlist Queries

## Validate tests

```bash
go test ./parse ./render ./model
go test ./...
```

## Generate the motivating report

```bash
my-gather --overwrite \
  -o /tmp/report-CS0060148.html \
  /Users/matias/Documents/Incidents/CS0060148/eu-hrznp-d003/pt-stalk
```

## Manual validation

Open `/tmp/report-CS0060148.html`, then inspect:

Database Usage -> Thread states -> Slowest observed queries
Advisor -> Query Shape

Expected behavior:

- `Sleep` and `Daemon` rows do not appear as slow queries.
- Long-running active `Query` rows waiting on table metadata lock are visible.
- Repeated sightings of the same query shape are grouped.
- Query snippets are bounded.
- The slowest observed queries panel can collapse independently.
- User, database, state, and query/fingerprint filters narrow the table.
- Advisor marks metadata-lock slow observed queries as Critical.
- The page remains fully functional offline.
