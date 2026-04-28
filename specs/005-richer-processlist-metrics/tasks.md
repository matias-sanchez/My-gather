# Tasks: Richer Processlist Metrics

**Input**: `specs/005-richer-processlist-metrics/`  
**Prerequisites**: spec.md, plan.md, research.md, data-model.md, contracts/

## Phase 1: Setup

- [x] T001 Create feature branch `005-richer-processlist-metrics`.
- [x] T002 Create `specs/005-richer-processlist-metrics/spec.md`.
- [x] T003 Create planning artifacts in `specs/005-richer-processlist-metrics/`.

## Phase 2: Foundational Tests

- [x] T004 [P] Add parser assertions for active/sleeping totals, longest age, row maxima, and query-text counts in `parse/processlist_test.go`.
- [x] T005 [P] Add render payload assertions for the richer processlist payload in `render/render_test.go`.
- [x] T006 [P] Add Processlist subview markup assertions for summary callouts and Activity control in `render/db_test.go`.
- [x] T007 Update the processlist golden fixture output in `testdata/golden/processlist.example2.2026_04_21_16_51_41.json`.

## Phase 3: User Story 1 - Spot active thread pressure quickly (Priority: P1)

**Goal**: Show active, sleeping, and total thread counts in the existing
Processlist subview.

**Independent Test**: Parser and render tests verify active/sleeping/total
counts from a mixed processlist fixture.

- [x] T008 [US1] Extend `model.ThreadStateSample` in `model/model.go` with thread total, active, and sleeping fields.
- [x] T009 [US1] Update `parse/processlist.go` to classify `Command == "Sleep"` as sleeping and all other rows as active.
- [x] T010 [US1] Update `render/payloads.go` so `processlistChartPayload` adds an Activity dimension with Active, Sleeping, and Total series.
- [x] T011 [US1] Update `render/assets/app.js` so the Processlist toolbar exposes the Activity dimension with the existing chart path.

## Phase 4: User Story 2 - Identify long-running and high-row work (Priority: P2)

**Goal**: Surface longest thread age and peak row counts per sample and as
summary callouts.

**Independent Test**: Parser tests verify numeric derivation; render tests
verify summary values and payload arrays.

- [x] T012 [US2] Extend `model.ThreadStateSample` in `model/model.go` with age and row-count metric fields.
- [x] T013 [US2] Update `parse/processlist.go` to parse `Time_ms`, fallback `Time`, `Rows_examined`, and `Rows_sent`.
- [x] T014 [US2] Update `render/view.go`, `render/summaries.go`, and `render/build_view.go` with Processlist summary callouts.
- [x] T015 [US2] Update `render/templates/db.html.tmpl` to render Processlist summary callouts.
- [x] T016 [US2] Update `render/payloads.go` with processlist metric arrays for age and row counts.

## Phase 5: User Story 3 - Understand query-text availability safely (Priority: P3)

**Goal**: Show query-text presence counts without adding a raw SQL table.

**Independent Test**: Parser tests verify `Info` counting; render tests verify
the count is surfaced as an aggregate metric only.

- [x] T017 [US3] Extend `model.ThreadStateSample` in `model/model.go` with query-text count.
- [x] T018 [US3] Update `parse/processlist.go` to count non-empty, non-`NULL` `Info` rows.
- [x] T019 [US3] Update `render/summaries.go`, `render/templates/db.html.tmpl`, and `render/payloads.go` to expose query-text counts.

## Phase 6: Polish and Verification

- [x] T020 Update `AGENTS.md`, `CLAUDE.md`, and `.specify/feature.json` so active feature pointers remain aligned.
- [x] T021 Update `tests/coverage/agent_alignment_test.go` so it validates active feature pointers generically and still accepts no-active-feature main state.
- [x] T022 Run `go test ./...`.
- [x] T023 Run `make lint`.
- [x] T024 Run quickstart validation against `/Users/matias/Documents/Incidents/CS0060148/eu-hrznp-d003/pt-stalk`.
- [ ] T025 Open a GitHub pull request for branch `005-richer-processlist-metrics`.

## Dependencies

- T004-T007 must be written before implementation tasks.
- T008-T011 deliver the MVP.
- T012-T016 depend on the model/parser shape from T008-T009.
- T017-T019 depend on the same row-flush parser path.
- T020-T025 happen after implementation.

## Implementation Strategy

Deliver US1 first because active versus sleeping threads is the incident-triage
signal. Then add age/row metrics, then query-text presence. Each story remains
testable through parser assertions and rendered payload/markup checks.
