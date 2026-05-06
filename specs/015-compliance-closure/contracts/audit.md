# Compliance Closure Audit Contract

| ID | Severity | Finding | Required Resolution |
|----|----------|---------|---------------------|
| C01 | HIGH | Feature 001 status/backlog is ambiguous. | Mark as shipped/historical with tracked backlog or split backlog clearly. |
| C02 | HIGH | Feature 001 deterministic timestamp wording contradicts current strict determinism. | Align generated-at wording to deterministic render options. |
| C03 | HIGH | Feature 002 plan still describes superseded GitHub Discussions path. | Mark submit path superseded by Feature 003. |
| C04 | MEDIUM | Feature 002 voice cap mixes current 10-minute guidance with old two-minute wording. | Normalize current guidance to 10 minutes. |
| C05 | MEDIUM | Feature 012 status is still `Ready for review`. | Mark complete. |
| C06 | HIGH | Unsupported known collector versions render as generic missing/unparseable states. | Render unsupported-version state naming source file. |
| C07 | MEDIUM | InnoDB callout formatting bypasses `reportutil`. | Use canonical formatting helpers. |
| C08 | MEDIUM | `.specify/extensions.yml` installed state conflicts with registry. | Align installed extension metadata. |
| C09 | MEDIUM | Agent/skill guidance has stale one-sided update and command-doc paths. | Align guidance to canonical repo paths. |
| C10 | MEDIUM | Worker route-level behavior lacks high-signal tests. | Add tests for existing routes and edge states. |
| C11 | LOW | Source comments still say seven collectors where ten are known. | Update comments. |

## Acceptance

All rows are complete when validation in `quickstart.md` passes and repeated
audit finds no confirmed high or medium issue from this contract.
