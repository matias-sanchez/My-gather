# Tasks: File Size Constitution Guard

**Input**: Design documents from `specs/010-file-size-constitution/`
**Prerequisites**: spec.md, plan.md, research.md, data-model.md, contracts/file-size.md, quickstart.md

**Tests**: Included because the constitution rule must be mechanically enforced.

## Canonical Path Metadata

- **Canonical owner/path**: `render/assets.go` with maintained app parts under
  `render/assets/app-js/` and `render/assets/app-css/`.
- **Old path treatment**: `render/assets/app.js` and `render/assets/app.css` are
  deleted by T006/T007; Chart.js minified assets remain explicit reviewed
  allowlist entries.
- **External degradation evidence**: N/A; T008 preserves the same rendered CSS/JS
  contract.

## Phase 1: Setup

- [x] T001 Create feature branch `010-file-size-constitution`.
- [x] T002 Create feature artifacts under `specs/010-file-size-constitution/`.
- [x] T003 Update `.specify/feature.json`, `AGENTS.md`, and `CLAUDE.md` to the active feature.

## Phase 2: Constitution And Gate

- [x] T004 Add the bounded-file-size constitution principle and mechanical gate in `.specify/memory/constitution.md`.
- [x] T005 Add `tests/coverage/file_size_test.go` for governed source files over 1000 lines.

## Phase 3: Repository Alignment

- [x] T006 Split `render/assets/app.js` into ordered embedded JS parts under `render/assets/app-js/`.
- [x] T007 Split `render/assets/app.css` into ordered embedded CSS parts under `render/assets/app-css/`.
- [x] T008 Update `render/assets.go` to concatenate ordered embedded app JS/CSS parts.
- [x] T009 Run `gofmt` on modified Go files.

## Phase 4: Validation And PR

- [x] T010 Run `go test -count=1 ./tests/coverage -run TestGovernedSourceFileLineLimit`.
- [x] T011 Run `go test -count=1 ./...`.
- [x] T012 Review governed source file scan output.
- [x] T013 Canonical-path audit: verify `render/assets.go` is the only app
  asset embed/concat owner, old monolithic app asset paths are deleted, and no
  fallback or compatibility asset path remains.
- [x] T014 Commit, push, and open a PR.
