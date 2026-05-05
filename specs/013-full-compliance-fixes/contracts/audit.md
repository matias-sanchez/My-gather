# Compliance Fix Audit Contract

| ID | Severity | Finding | Required Resolution |
|----|----------|---------|---------------------|
| C01 | HIGH | Completed feature remained active on `main`. | Final checked-in agent blocks say no active feature; `.specify/feature.json` points at latest shipped feature. |
| C02 | HIGH | Old Spec Kit feature creation script remained beside extension script. | Delete old core script and manifest entry. |
| C03 | HIGH | Feature path resolution could silently fall back to branch prefix. | Fail clearly unless canonical pointer or explicit override is present. |
| C04 | MEDIUM | Env meminfo quiet wrapper discarded diagnostics. | First-party callers use `ParseEnvMeminfoWithDiagnostics`; quiet wrapper removed. |
| C05 | MEDIUM | Netstat malformed timestamp parse fell back silently. | Emit structured diagnostic before using snapshot timestamp. |
| C06 | MEDIUM | InnoDB kept duplicate comma formatter. | Use `reportutil` formatting helper. |
| C07 | MEDIUM | Worker tests were configured for Node despite Workers-runtime docs. | Use Workers Vitest pool. |
| C08 | MEDIUM | Parser golden enforcement wording and tests were misaligned. | Hard coverage test for fixtures and parser goldens. |
| C09 | MEDIUM | Historical feature 001 and 003 specs had contradictory wording. | Align wording to shipped behavior without changing runtime code. |
| C10 | LOW | Principle XV gate missed some governed source extensions. | Include PowerShell and HTML implementation files. |

## Acceptance

All required resolutions are complete only when validation in
`quickstart.md` passes and canonical-path searches find no confirmed old path.

## Validation Log

- Spec Kit prerequisites: passed for `specs/013-full-compliance-fixes`.
- Full Go suite: `go test -count=1 ./...` passed after final no-active
  metadata.
- Go lint gate: `make lint` passed.
- Worker gates: `npm run typecheck` and `npm test` passed with
  `@cloudflare/vitest-pool-workers`.
- Constitution guard: `CONSTITUTION_GUARD_CI=1
  CONSTITUTION_GUARD_RANGE=origin/main..HEAD
  scripts/hooks/pre-push-constitution-guard.sh` passed.
- Canonical path searches: no remaining source matches for
  `ParseEnvMeminfo(`, `formatThousands`, `find_feature_dir_by_prefix`, or
  `legacy fallback`; deleted core Spec Kit branch-creation path is absent from
  the working tree.
