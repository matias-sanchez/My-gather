# Tasks: Repository Alignment Audit

**Input**: Design documents from `specs/012-repo-alignment-audit/`
**Prerequisites**: spec.md, plan.md, research.md, contracts/audit.md, quickstart.md

**Tests**: Included because this branch changes enforcement and small runtime
contracts.

## Canonical Path Metadata

- **Canonical owner/path**: existing owners for each touched behavior.
- **Old path treatment**: remove only confirmed small competing paths.
- **External degradation evidence**: no new degradation path; existing external
  feedback fallback remains visible and named by the constitution.

## Phase 1: Setup

- [x] T001 Create branch `012-repo-alignment-audit` from `main`.
- [x] T002 Create feature artifacts under `specs/012-repo-alignment-audit/`.
- [x] T003 Update `.specify/feature.json`, `AGENTS.md`, and `CLAUDE.md` to the
  active feature.

## Phase 2: Audit

- [x] T004 Run six parallel audit agents.
- [x] T005 Consolidate findings into `contracts/audit.md`.

## Phase 3: Minimal Fixes

- [x] T006 Fix deterministic render/build metadata and release verification.
- [x] T007 Make the constitution guard usable as a CI-failing gate.
- [x] T008 Remove the legacy mysqladmin localStorage path.
- [x] T009 Align feedback Worker success/throttle/preflight contracts.
- [x] T010 Update stale spec/docs/template metadata found by the audit.
- [x] T011 Document larger source refactors as follow-ups instead of partially
  implementing them.
- [x] T017 Resolve helper duplication with one canonical `reportutil` package.
- [x] T018 Resolve feedback contract duplication with canonical contract JSON.
- [x] T019 Make confirmed parser compatibility fallbacks observable with
  structured diagnostics.

## Phase 4: Validation And PR

- [x] T012 Run `go test -count=1 ./...`.
- [x] T013 Run `cd feedback-worker && npm run typecheck && npm test`.
- [x] T014 Run `scripts/hooks/pre-push-constitution-guard.sh`.
- [x] T015 Run Spec Kit analysis for this feature.
- [x] T016 Commit, push, and open PR.
