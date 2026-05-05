# Tasks: Reconciliation Cleanup

**Input**: Design documents from `specs/014-reconciliation-cleanup/`
**Prerequisites**: plan.md, spec.md, contracts/audit.md, quickstart.md

**Tests**: Required. This feature closes confirmed audit findings.

## Canonical Path Metadata *(Principle XIII; include when changing existing behavior)*

- **Canonical owner/path**: Spec Kit common helper scripts, netstat timestamp
  parsing, and feature 001 current-state documentation.
- **Old path treatment**: remove confirmed fallback helper paths and stale
  contradictory wording.
- **External degradation evidence**: none introduced.

## Phase 1: Setup

- [x] T001 Create feature artifacts under `specs/014-reconciliation-cleanup/`.
- [x] T002 Point active feature metadata at `014-reconciliation-cleanup`.
- [x] T003 Run multi-agent focused reconciliation audit on this branch.

## Phase 2: Tests First

- [x] T004 Add malformed non-numeric/negative netstat `TS` coverage in
  `parse/netstat_test.go`.
- [x] T005 Add or adjust tooling coverage/searches for retired Spec Kit helper
  fallback paths.

## Phase 3: Implementation

- [x] T006 Update netstat/netstat_s timestamp boundary parsing to diagnose any
  malformed `TS` header token.
- [x] T007 Remove duplicate/fallback Bash helper loading in
  `.specify/extensions/git/scripts/bash/create-new-feature.sh`.
- [x] T008 Remove unused duplicate Bash git helper file if no longer referenced.
- [x] T009 Narrow PowerShell git feature helper loading to one canonical live
  helper path without probing absent core paths.
- [x] T010 Align feature 001 stale report-section, diagnostics, Go version, and
  principle-count wording.
- [x] T011 Clean trailing whitespace introduced or touched by this feature.

## Phase 4: Validation

- [x] T012 Run Spec Kit prerequisite/analyze checks while active.
- [x] T013 Run `go test -count=1 ./...`.
- [x] T014 Run `make lint`.
- [x] T015 Run `cd feedback-worker && npm run typecheck && npm test` as
  full-repository health validation, even though this branch does not change
  Worker files.
- [x] T016 Run the constitution guard in CI mode.
- [x] T017 Run canonical-path and source-size searches and summarize results in
  `contracts/audit.md`.
- [ ] T018 Commit, push, open PR, trigger review, fix review findings, and leave
  PR ready.
