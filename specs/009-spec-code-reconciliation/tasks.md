# Tasks: Spec And Code Reconciliation

**Input**: Design documents from `specs/009-spec-code-reconciliation/`
**Prerequisites**: spec.md, plan.md, research.md, data-model.md, contracts/reconciliation.md, quickstart.md

**Tests**: Included because this feature closes missing CLI coverage for
documented archive-input behavior.

## Phase 1: Setup

- [x] T001 Create feature branch `009-spec-code-reconciliation`.
- [x] T002 Create reconciliation Spec Kit artifacts under `specs/009-spec-code-reconciliation/`.
- [x] T003 Update `.specify/feature.json`, `AGENTS.md`, and `CLAUDE.md` to the active reconciliation feature.

## Phase 2: Spec And Task Reconciliation

- [x] T004 Mark fully completed features `006`, `007`, and `008` as `Complete` in their `spec.md` files.
- [x] T005 Add a historical-state note to `specs/002-report-feedback-button/tasks.md`.
- [x] T006 Replace the unresolved placeholder task ID in `specs/001-ptstalk-report-mvp/checklists/ux-quality.md`.
- [x] T007 Update feature index blocks in `AGENTS.md` and `CLAUDE.md` so prior features include `008-archive-input`.

## Phase 3: Archive Input Test Coverage

- [x] T008 Add a CLI test for supported archive files with zero pt-stalk roots in `cmd/my-gather/main_test.go`.
- [x] T009 Add a CLI test for unsupported regular input files in `cmd/my-gather/main_test.go`.
- [x] T010 Run `gofmt` on modified Go files.

## Phase 4: Validation And PR

- [x] T011 Run `go test ./cmd/my-gather`.
- [x] T012 Run `go test ./...`.
- [x] T013 Review the final diff for constitution alignment and update this task file to checked state.
- [x] T014 Push the branch and open a PR.
