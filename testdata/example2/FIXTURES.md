# testdata/example2/ fixtures — provenance

Fixtures used by the goldens, render tests, and end-to-end coverage
suites in this repository. Every file here is anonymised and
small-enough-to-commit; real-world pt-stalk dumps are stored under
`_references/examples/` (not checked-in at v1 scale).

Fixture | Snapshots present | Provenance
---|---|---
`2026_04_21_16_51_41-hostname` | yes | Captured (anonymised)
`2026_04_21_16_51_41-innodbstatus1` | yes | Captured (anonymised)
`2026_04_21_16_51_41-iostat` | yes | Captured (anonymised)
`2026_04_21_16_51_41-meminfo` | yes | **Synthetic** (Linux-kernel `/proc/meminfo` format, plausible values; no pt-stalk capture of this collector exists in the source dump that seeded example2)
`2026_04_21_16_51_41-mysqladmin` | yes | Captured (anonymised)
`2026_04_21_16_51_41-processlist` | yes | Captured (anonymised)
`2026_04_21_16_51_41-top` | yes | Captured (anonymised)
`2026_04_21_16_51_41-variables` | yes | Captured (anonymised)
`2026_04_21_16_51_41-vmstat` | yes | Captured (anonymised)
`2026_04_21_16_52_11-*` | yes | Same as above; second snapshot in the pair for multi-snapshot coverage
`pt-summary.out` | once per collection | Captured (anonymised)
`pt-mysql-summary.out` | once per collection | Captured (anonymised)

## Why this matters

- **Captured** fixtures exercise the real-world format surface of the
  supported pt-stalk release (research R2). When bumping the supported
  pt-stalk version, these are the files that must be refreshed from a
  current dump (plus re-anonymisation via
  `scripts/anonymise-fixtures.sh`).
- **Synthetic** fixtures are hand-written to exercise a specific parser
  path the real sample didn't include. They do NOT represent the
  format surface any pt-stalk release ships — if a parser's format
  drifts, the synthetic fixture won't catch it. Synthetic fixtures
  should be replaced with captured ones when a real dump including
  that collector becomes available.

The `-meminfo` collector is synthetic because the original pt-stalk
dump that seeded `example2/` predates the `-meminfo` wiring. When a
newer dump including `-meminfo` is anonymised, replace these two
files and regenerate the meminfo golden with
`go test ./parse/... -update`.

## Anonymisation policy

See `scripts/anonymise-fixtures.sh` (research R6) for the applied
rules: hostnames → `example-db-01` et al., IPv4 → reserved-for-docs
space, query text scrubbed, MySQL user names and database names
replaced with neutral labels.
