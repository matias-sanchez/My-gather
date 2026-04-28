# Tasks: Observed Slowest Processlist Queries

**Input**: Design documents from `specs/006-observed-slowest-queries/`  
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`

**Tests**: Tests are required by FR-010 and must be written before the matching
implementation tasks.

## Phase 1: Setup

**Purpose**: Establish feature artifacts and active spec context.

- [x] T001 Create feature branch `006-observed-slowest-queries`.
- [x] T002 Create `specs/006-observed-slowest-queries/spec.md`.
- [x] T003 Create planning artifacts in `specs/006-observed-slowest-queries/`.

---

## Phase 2: Foundational Model and Parser Tests

**Purpose**: Define the slow-query summary contract before implementation.

- [x] T004 [P] Add model ranking and merge tests in `model/processlist_query_test.go`.
- [x] T005 [P] Add parser tests for eligible active rows, idle/background exclusion, fingerprint grouping, and bounded snippets in `parse/processlist_test.go`.

---

## Phase 3: User Story 1 - Find slow in-flight queries quickly (Priority: P1)

**Goal**: Show the top slowest observed active processlist query fingerprints.

**Independent Test**: Parser tests identify grouped top query fingerprints and
render tests show them in the Processlist subview.

- [x] T006 [US1] Extend `model/model.go` with observed query summary fields and deterministic helper functions.
- [x] T007 [US1] Update `parse/processlist.go` to collect eligible active query sightings and emit grouped summaries.
- [x] T008 [US1] Add render markup tests in `render/db_test.go`.
- [x] T009 [US1] Update `render/templates/db.html.tmpl` and `render/assets/app.css` to show the bounded slowest-query table.

---

## Phase 4: User Story 2 - Preserve row-count and wait-state evidence (Priority: P2)

**Goal**: Preserve peak row counters and context from the slowest sighting.

**Independent Test**: Repeated sightings of the same fingerprint retain the
highest row counters and state from the slowest sighting.

- [x] T010 [P] [US2] Add merge tests for cross-snapshot query summaries in `render/concat_test.go`.
- [x] T011 [US2] Update `render/concat.go` to merge observed query summaries across snapshots.
- [x] T012 [US2] Update `render/payloads.go` and `render/render_test.go` to embed `slowQueries`.

---

## Phase 5: User Story 3 - Keep query text useful but controlled (Priority: P3)

**Goal**: Bound text exposure while preserving useful identity.

**Independent Test**: Long query text renders as a bounded snippet and repeated
literal variants group by fingerprint.

- [x] T013 [P] [US3] Add snippet and fingerprint tests in `model/processlist_query_test.go`.
- [x] T014 [US3] Ensure `model/model.go` normalizes query text, computes stable fingerprints, and bounds snippets.
- [x] T015 [US3] Add empty-state render coverage in `render/db_test.go`.

---

## Phase 6: Polish and Verification

**Purpose**: Validate the full workflow against tests and a real incident
capture.

- [x] T016 Run `gofmt` on modified Go files.
- [x] T017 Run `go test ./parse ./render ./model`.
- [x] T018 Run `go test ./...`.
- [x] T019 Generate `/tmp/report-CS0060148.html` from the motivating incident and validate the slowest observed queries appear.
- [x] T020 Update `AGENTS.md` active feature block to `006-observed-slowest-queries`.

## Dependencies & Execution Order

- Phase 1 must complete first.
- Phase 2 tests must be written before model/parser implementation.
- US1 is the MVP and must complete before US2/US3 render polish.
- US2 depends on the model fields from US1.
- US3 depends on the fingerprint/snippet helpers from US1.
- Phase 6 runs after all user stories are complete.
