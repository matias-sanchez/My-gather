# Tasks: Compliance Closure

**Input**: Design documents from `specs/015-compliance-closure/`
**Prerequisites**: plan.md, spec.md, contracts/audit.md, quickstart.md

**Tests**: Required. This feature closes confirmed audit findings.

## Canonical Path Metadata *(Principle XIII; include when changing existing behavior)*

- **Canonical owner/path**: render state, reportutil formatting, Spec Kit
  metadata, and shipped feature artifacts.
- **Old path treatment**: update stale text/metadata; do not preserve duplicate
  paths.
- **External degradation evidence**: unsupported pt-stalk versions are rendered
  as visible unsupported states.

## Phase 1: Setup

- [x] T001 Create feature artifacts under `specs/015-compliance-closure/`.
- [x] T002 Point active feature metadata at `015-compliance-closure`.

## Phase 2: Specs And Metadata

- [x] T003 Align Feature 001 status, backlog, and deterministic timestamp wording.
- [x] T004 Align Feature 002 supersession and 10-minute voice-cap wording.
- [x] T005 Mark Feature 012 complete.
- [x] T006 Align `.specify/extensions.yml` with enabled git extension state.
- [x] T007 Align agent/skill guidance and stale command-doc paths.

## Phase 3: Source Fixes And Tests

- [x] T008 Add render test coverage for unsupported collector version state.
- [x] T009 Render unsupported collector versions as unsupported-version states.
- [x] T010 Route InnoDB callout formatting through `reportutil`.
- [x] T011 Update stale collector-count source comments.
- [x] T012 Add high-signal Worker route-level tests.

## Phase 4: Validation

- [x] T013 Run `go test -count=1 ./...`.
- [x] T014 Run `make lint`.
- [x] T015 Run `scripts/hooks/pre-push-constitution-guard.sh`.
- [x] T016 Run `cd feedback-worker && npm run typecheck && npm test`.
- [x] T017 Run source-size and coverage checks.
- [x] T018 Repeat audit summary and package PR.
