# Reconciliation Cleanup Audit Contract

| ID | Severity | Finding | Required Resolution |
|----|----------|---------|---------------------|
| C01 | MEDIUM | Non-numeric or negative netstat `TS` headers bypass malformed timestamp diagnostics. | Emit diagnostics and preserve deterministic sample boundaries for any malformed `TS` token. |
| C02 | MEDIUM | Spec Kit git feature scripts keep duplicate/fallback helper loading paths. | Remove hidden fallback helper paths and keep one canonical helper source per shell family. |
| C03 | MEDIUM | Feature 001 still contains stale current-state wording for section count, parser diagnostics, and Go version. | Align normative-looking text to current shipped behavior. |
| C04 | LOW | Feature 001 still references the older 12-principle model. | Update to the current 15-principle constitution model where text is current-state guidance. |
| C05 | LOW | Recent spec artifacts include trailing whitespace. | Clean touched feature artifacts and verify diff whitespace. |

## Acceptance

All required resolutions are complete only when validation in `quickstart.md`
passes and no confirmed old helper or parser diagnostic gap remains.

## Validation Summary

- Multi-agent reconciliation audit: completed; confirmed drift findings were
  fixed.
- Spec Kit prerequisites: passed while `014-reconciliation-cleanup` was active.
- Go validation: `go test -count=1 ./...` passed; `make lint` passed.
- Worker repo health: `npm run typecheck` and `npm test` passed under
  `feedback-worker/`.
- Constitution guard: `scripts/hooks/pre-push-constitution-guard.sh` passed.
- Canonical-path searches: retired helper patterns returned no live matches in
  `.specify/extensions/git/scripts`; Bash `git-common.sh` is absent.
- Source-size search: no governed first-party source file exceeded 1000 lines.
