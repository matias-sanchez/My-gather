# Tasks: Full Compliance Fixes

**Input**: Design documents from `specs/013-full-compliance-fixes/`
**Prerequisites**: plan.md, spec.md, contracts/audit.md, quickstart.md

**Tests**: Required. This feature closes compliance findings and must prove the
mechanical gates catch the intended source/spec surfaces.

## Canonical Path Metadata *(Principle XIII; include when changing existing behavior)*

- **Canonical owner/path**: `.specify/feature.json`, git extension feature
  scripts, `ParseEnvMeminfoWithDiagnostics`, `reportutil`, Workers Vitest pool.
- **Old path treatment**: delete or remove all confirmed old/fallback paths by
  T004, T005, T008, T010, and T012.
- **External degradation evidence**: T015 documents feedback degradation as
  visible and accepted.

## Phase 1: Setup

- [x] T001 Create feature artifacts under `specs/013-full-compliance-fixes/`.
- [x] T002 Point active feature metadata at `013-full-compliance-fixes` while
  the branch is under implementation.

## Phase 2: Spec Kit Canonical Paths

- [x] T003 Add/adjust tests for no-active-feature pointer behavior and governed
  source extensions in `tests/coverage/`.
- [x] T004 Delete `.specify/scripts/bash/create-new-feature.sh` and remove it
  from the Spec Kit manifest.
- [x] T005 Remove branch-name feature-directory fallback from
  `.specify/scripts/bash/common.sh`.

## Phase 3: Source Canonical Paths

- [x] T006 Add netstat malformed timestamp diagnostic coverage in
  `parse/netstat_test.go`.
- [x] T007 Make `parse/netstat.go` and `parse/netstat_s.go` emit diagnostics on
  malformed `TS` epochs.
- [x] T008 Remove quiet env meminfo wrapper usage and make first-party callers
  use `ParseEnvMeminfoWithDiagnostics`.
- [x] T009 Add `reportutil` integer-or-placeholder formatting coverage.
- [x] T010 Replace InnoDB local comma formatting with `reportutil`.

## Phase 4: Enforcement and Spec Drift

- [x] T011 Harden parser fixture/golden coverage in `tests/coverage/`.
- [x] T012 Switch Worker tests to the Workers-compatible Vitest pool.
- [x] T013 Align feature 001 section/navigation wording.
- [x] T014 Align feature 003 fallback, rate-limit, and CPU-measurement wording.
- [x] T015 Update constitution/enforcement wording that lagged implementation.
- [x] T016 Mark completed feature state as no active feature for post-merge
  `main` while keeping `.specify/feature.json` pointed at this latest shipped
  feature.

## Phase 5: Validation

- [x] T017 Run Spec Kit prerequisite/analyze checks before final no-active
  metadata is applied.
- [x] T018 Run `go test -count=1 ./...`.
- [x] T019 Run `make lint`.
- [x] T020 Run `cd feedback-worker && npm run typecheck && npm test`.
- [ ] T021 Run the constitution guard in CI mode.
- [x] T022 Run canonical-path searches for retired paths and summarize the
  result in `contracts/audit.md`.
- [ ] T023 Commit, push, open PR, and trigger review.
