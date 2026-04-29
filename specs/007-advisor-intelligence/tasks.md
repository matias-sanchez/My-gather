# Tasks: Advisor Intelligence

**Input**: Design documents from `specs/007-advisor-intelligence/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Test tasks are included because the specification and quickstart
require focused Advisor, render, and golden validation.

**Organization**: Tasks are grouped by user story to enable independently
testable increments.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel with other marked tasks in the same phase
  because it touches different files and has no dependency on incomplete tasks.
- **[Story]**: User story label for story phases only.
- Every task includes exact file paths.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare durable planning and baseline context before shared code changes.

- [x] T001 Create the Rosetta Stone topic coverage skeleton in `specs/007-advisor-intelligence/coverage-map.md`
- [x] T002 [P] Record current Advisor baseline commands and expected green tests in `specs/007-advisor-intelligence/quickstart.md`
- [x] T003 [P] Add a short implementation scope note for deferred Rosetta topics in `specs/007-advisor-intelligence/coverage-map.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Add shared Advisor metadata, evidence, and render plumbing required by every story.

**Critical**: No user story work can begin until this phase is complete.

- [x] T004 Add diagnostic category, evidence kind, evidence strength, confidence, recommendation kind, and correlation types in `findings/findings.go`
- [x] T005 Extend `Finding` with category, confidence, evidence bundle, recommendation metadata, related finding IDs, and coverage topic in `findings/findings.go`
- [x] T006 Extend `RuleDefinition` with category and coverage metadata in `findings/register.go`
- [x] T007 Add coverage map types and deterministic exported coverage accessors in `findings/coverage.go`
- [x] T008 Update Advisor view structs for category, confidence, evidence, recommendations, relations, and source topic in `render/view.go`
- [x] T009 Update `buildFindingViews` mapping for the new Advisor fields in `render/advisor_view.go`
- [x] T010 [P] Add rule metadata quality assertions for category and coverage topic in `findings/rules_quality_test.go`
- [x] T011 [P] Add coverage map quality tests for unique topics and valid statuses in `findings/coverage_test.go`

**Checkpoint**: Shared Advisor metadata compiles, rule quality tests cover required metadata, and story work can begin.

---

## Phase 3: User Story 1 - Explain Incident Drivers Clearly (Priority: P1)

**Goal**: Advisor findings explain likely subsystem pressure with evidence,
interpretation, and concrete next checks.

**Independent Test**: Generate or build a report with known buffer pool, redo,
table cache, connection, and query-shape signals; Advisor findings show
subsystem, severity, evidence, interpretation, and next checks.

### Tests for User Story 1

- [x] T012 [P] [US1] Add finding evidence and interpretation tests for major subsystem signals in `findings/findings_test.go`
- [x] T013 [P] [US1] Add Advisor card rendering tests for evidence bundles and next checks in `render/render_test.go`

### Implementation for User Story 1

- [x] T014 [P] [US1] Populate evidence, interpretation, confidence, and next checks for buffer pool findings in `findings/rules_bufferpool.go`
- [x] T015 [P] [US1] Populate evidence, interpretation, confidence, and next checks for redo findings in `findings/rules_redo.go`
- [x] T016 [P] [US1] Populate evidence, interpretation, confidence, and next checks for table cache findings in `findings/rules_tablecache.go`
- [x] T017 [P] [US1] Populate evidence, interpretation, confidence, and next checks for connection findings in `findings/rules_connections.go`
- [x] T018 [P] [US1] Populate evidence, interpretation, confidence, and next checks for query-shape findings in `findings/rules_queryshape.go`
- [x] T019 [US1] Render evidence bundles and recommendation groups in `render/templates/advisor.html.tmpl`
- [x] T020 [US1] Style evidence bundles, category chips, confidence chips, and recommendation groups in `render/assets/app.css`
- [x] T021 [US1] Update Advisor golden output for evidence and recommendations in `testdata/golden/findings.example2.json`

**Checkpoint**: User Story 1 is independently functional and the Advisor can explain top incident drivers with evidence.

---

## Phase 4: User Story 2 - Classify Signals Using Expert Diagnostic Framing (Priority: P1)

**Goal**: Findings consistently classify signals as utilization, saturation,
error, or combined according to the Rosetta Stone framing.

**Independent Test**: Advisor output for rule scenarios includes the expected
diagnostic category and does not overstate utilization-only signals.

### Tests for User Story 2

- [x] T022 [P] [US2] Add diagnostic category table tests for registered rules in `findings/rules_quality_test.go`
- [x] T023 [P] [US2] Add category-specific rule behavior tests for utilization, saturation, and error examples in `findings/findings_test.go`

### Implementation for User Story 2

- [x] T024 [P] [US2] Classify buffer pool findings by diagnostic category in `findings/rules_bufferpool.go`
- [x] T025 [P] [US2] Classify redo findings by diagnostic category in `findings/rules_redo.go`
- [x] T026 [P] [US2] Classify flushing findings by diagnostic category in `findings/rules_flushing.go`
- [x] T027 [P] [US2] Classify table cache findings by diagnostic category in `findings/rules_tablecache.go`
- [x] T028 [P] [US2] Classify thread cache findings by diagnostic category in `findings/rules_threadcache.go`
- [x] T029 [P] [US2] Classify thread queue findings by diagnostic category in `findings/rules_threadqueue.go`
- [x] T030 [P] [US2] Classify connection findings by diagnostic category in `findings/rules_connections.go`
- [x] T031 [P] [US2] Classify temporary table findings by diagnostic category in `findings/rules_tmp.go`
- [x] T032 [P] [US2] Classify binary log findings by diagnostic category in `findings/rules_binlog.go`
- [x] T033 [P] [US2] Classify query-shape findings by diagnostic category in `findings/rules_queryshape.go`
- [x] T034 [P] [US2] Classify semaphore findings by diagnostic category in `findings/rules_semaphores.go`
- [x] T035 [US2] Complete Rosetta Stone coverage rows for covered, partial, deferred, and excluded topics in `findings/coverage.go`
- [x] T036 [US2] Update the feature coverage map with implementation coverage status in `specs/007-advisor-intelligence/coverage-map.md`
- [x] T037 [US2] Update Advisor golden output for diagnostic category metadata in `testdata/golden/findings.example2.json`

**Checkpoint**: User Story 2 is independently functional and every visible finding has an auditable diagnostic category.

---

## Phase 5: User Story 3 - Prioritize The Most Actionable Findings (Priority: P1)

**Goal**: Advisor highlights the top suspected incident drivers and explains
relationships between correlated findings without hiding important evidence.

**Independent Test**: A report with correlated symptoms ranks dominant
findings ahead of weaker related symptoms and cross-references relationships.

### Tests for User Story 3

- [x] T038 [P] [US3] Add deterministic top-driver ranking tests in `findings/findings_test.go`
- [x] T039 [P] [US3] Add correlated finding render tests in `render/render_test.go`

### Implementation for User Story 3

- [x] T040 [US3] Implement deterministic finding correlation helpers in `findings/findings.go`
- [x] T041 [US3] Add related-finding IDs for redo and flushing relationships in `findings/rules_flushing.go`
- [x] T042 [US3] Add related-finding IDs for metadata-lock and query-shape relationships in `findings/rules_queryshape.go`
- [x] T043 [US3] Add related-finding IDs for table-cache relationships in `findings/rules_tablecache.go`
- [x] T044 [US3] Derive top suspected drivers from visible findings in `render/advisor_view.go`
- [x] T045 [US3] Render the Advisor top-driver summary and related finding references in `render/templates/advisor.html.tmpl`
- [x] T046 [US3] Style top-driver summary and related finding references in `render/assets/app.css`
- [x] T047 [US3] Update Advisor golden output for top-driver summary in `testdata/golden/findings.example2.json`
- [x] T048 [US3] Update DB HTML golden output for top-driver summary in `testdata/golden/db.example2.html`

**Checkpoint**: User Story 3 is independently functional and correlated findings are prioritized and cross-referenced.

---

## Phase 6: User Story 4 - Guide Deeper Investigation Per Subsystem (Priority: P2)

**Goal**: Each supported subsystem finding recommends focused confirmation and
investigation steps that match the evidence shown.

**Independent Test**: Trigger each supported subsystem family and verify the
finding recommends relevant checks without unrelated advice.

### Tests for User Story 4

- [x] T049 [P] [US4] Add recommendation coverage tests for subsystem families in `findings/findings_test.go`
- [x] T050 [P] [US4] Add recommendation rendering tests for grouped confirmation and investigation steps in `render/render_test.go`

### Implementation for User Story 4

- [x] T051 [P] [US4] Add focused recommendations for buffer pool findings in `findings/rules_bufferpool.go`
- [x] T052 [P] [US4] Add focused recommendations for redo findings in `findings/rules_redo.go`
- [x] T053 [P] [US4] Add focused recommendations for flushing findings in `findings/rules_flushing.go`
- [x] T054 [P] [US4] Add focused recommendations for table cache findings in `findings/rules_tablecache.go`
- [x] T055 [P] [US4] Add focused recommendations for thread cache findings in `findings/rules_threadcache.go`
- [x] T056 [P] [US4] Add focused recommendations for thread queue findings in `findings/rules_threadqueue.go`
- [x] T057 [P] [US4] Add focused recommendations for connection findings in `findings/rules_connections.go`
- [x] T058 [P] [US4] Add focused recommendations for temporary table findings in `findings/rules_tmp.go`
- [x] T059 [P] [US4] Add focused recommendations for binary log findings in `findings/rules_binlog.go`
- [x] T060 [P] [US4] Add focused recommendations for query-shape findings in `findings/rules_queryshape.go`
- [x] T061 [P] [US4] Add focused recommendations for semaphore findings in `findings/rules_semaphores.go`
- [x] T062 [US4] Render recommendation kind labels and scoped cautions in `render/templates/advisor.html.tmpl`
- [x] T063 [US4] Update Advisor golden output for recommendation groups in `testdata/golden/findings.example2.json`

**Checkpoint**: User Story 4 is independently functional and every non-OK finding points to relevant next checks.

---

## Phase 7: User Story 5 - Preserve Trust Through Transparency And Restraint (Priority: P2)

**Goal**: Advisor clearly distinguishes direct evidence from inference, handles
missing inputs without unsupported warnings, and keeps sparse captures useful.

**Independent Test**: Sparse, partial, and malformed-input report models keep
valid findings while skipping or downgrading unsupported findings.

### Tests for User Story 5

- [x] T064 [P] [US5] Add sparse and partial capture behavior tests in `findings/findings_test.go`
- [x] T065 [P] [US5] Add missing-input and inference display tests in `render/render_test.go`

### Implementation for User Story 5

- [x] T066 [US5] Add missing-input and inference representation helpers in `findings/inputs.go`
- [x] T067 [US5] Update config rules that currently warn on weak evidence to skip, downgrade, or mark low confidence in `findings/rules_config_max_connections.go`
- [x] T068 [US5] Update query-shape rules to avoid unsupported findings on sparse captures in `findings/rules_queryshape.go`
- [x] T069 [US5] Render missing-input context, inference labels, and low-confidence labels in `render/templates/advisor.html.tmpl`
- [x] T070 [US5] Style missing-input context and low-confidence labels in `render/assets/app.css`
- [x] T071 [US5] Update Advisor golden output for sparse-input and confidence behavior in `testdata/golden/findings.example2.json`

**Checkpoint**: User Story 5 is independently functional and unsupported findings are skipped or clearly downgraded.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Finalize documentation, goldens, and repo-wide quality gates.

- [x] T072 [P] Update package contracts with final implementation decisions in `specs/007-advisor-intelligence/contracts/packages.md`
- [x] T073 [P] Update UI contract with final Advisor labels and empty-state behavior in `specs/007-advisor-intelligence/contracts/ui.md`
- [x] T074 [P] Update implementation notes and validation commands in `specs/007-advisor-intelligence/quickstart.md`
- [x] T075 Run gofmt on `findings/findings.go`, `findings/register.go`, `findings/coverage.go`, and `render/advisor_view.go`
- [x] T076 Run focused Advisor validation from quickstart and record any intentional golden updates in `specs/007-advisor-intelligence/quickstart.md`
- [x] T077 Run full local validation commands from quickstart in `specs/007-advisor-intelligence/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 Setup**: No dependencies.
- **Phase 2 Foundational**: Depends on Phase 1 and blocks every user story.
- **Phase 3 US1**: Depends on Phase 2 and is the MVP.
- **Phase 4 US2**: Depends on Phase 2; may run after or alongside US1 once shared fields compile.
- **Phase 5 US3**: Depends on US1 and US2 because it ranks and correlates enriched findings.
- **Phase 6 US4**: Depends on US1 and US2 because recommendations rely on evidence and categories.
- **Phase 7 US5**: Depends on US1 and US2 because trust controls rely on evidence and category fields.
- **Phase 8 Polish**: Depends on desired user stories being complete.

### User Story Dependencies

- **US1 (P1)**: MVP; no dependency on other stories after Phase 2.
- **US2 (P1)**: Can start after Phase 2; required before complete coverage claims.
- **US3 (P1)**: Depends on enriched and categorized findings from US1 and US2.
- **US4 (P2)**: Depends on enriched and categorized findings from US1 and US2.
- **US5 (P2)**: Depends on enriched and categorized findings from US1 and US2.

### Parallel Opportunities

- T002 and T003 can run in parallel after T001 creates `specs/007-advisor-intelligence/coverage-map.md`.
- T010 and T011 can run in parallel during the foundational phase.
- T014 through T018 can run in parallel after T012 and T013 fail for the expected missing fields.
- T024 through T034 can run in parallel after T022 and T023 fail for the expected missing categories.
- T049 and T050 can run in parallel because they touch different test files.
- T051 through T061 can run in parallel after recommendation test expectations are established.
- T064 and T065 can run in parallel because they touch different test files.
- T072 through T074 can run in parallel during polish.

---

## Parallel Example: User Story 1

```bash
Task: "T012 [US1] Add finding evidence and interpretation tests in findings/findings_test.go"
Task: "T013 [US1] Add Advisor card rendering tests in render/render_test.go"
Task: "T014 [US1] Populate buffer pool evidence in findings/rules_bufferpool.go"
Task: "T015 [US1] Populate redo evidence in findings/rules_redo.go"
Task: "T016 [US1] Populate table cache evidence in findings/rules_tablecache.go"
Task: "T017 [US1] Populate connection evidence in findings/rules_connections.go"
Task: "T018 [US1] Populate query-shape evidence in findings/rules_queryshape.go"
```

## Parallel Example: User Story 2

```bash
Task: "T024 [US2] Classify buffer pool findings in findings/rules_bufferpool.go"
Task: "T025 [US2] Classify redo findings in findings/rules_redo.go"
Task: "T026 [US2] Classify flushing findings in findings/rules_flushing.go"
Task: "T027 [US2] Classify table cache findings in findings/rules_tablecache.go"
Task: "T028 [US2] Classify thread cache findings in findings/rules_threadcache.go"
Task: "T029 [US2] Classify thread queue findings in findings/rules_threadqueue.go"
Task: "T030 [US2] Classify connection findings in findings/rules_connections.go"
Task: "T031 [US2] Classify temporary table findings in findings/rules_tmp.go"
Task: "T032 [US2] Classify binary log findings in findings/rules_binlog.go"
Task: "T033 [US2] Classify query-shape findings in findings/rules_queryshape.go"
Task: "T034 [US2] Classify semaphore findings in findings/rules_semaphores.go"
```

## Implementation Strategy

### MVP First

1. Complete Phase 1 and Phase 2.
2. Complete Phase 3 for US1.
3. Stop and validate:
   - `go test ./findings -count=1`
   - `go test ./render -run Advisor -count=1`
4. Review the generated report Advisor section manually before expanding scope.

### Incremental Delivery

1. US1 explains incident drivers.
2. US2 adds Rosetta Stone diagnostic categories and coverage traceability.
3. US3 adds top-driver prioritization and correlations.
4. US4 deepens subsystem-specific recommendations.
5. US5 hardens sparse-input and evidence-confidence behavior.

### Final Validation

Before review or merge, run:

```bash
go vet ./...
go test ./... -count=1
make lint
scripts/hooks/pre-push-constitution-guard.sh
```

## Notes

- Keep the native `findings` registry as the only Advisor dispatch path.
- Do not add runtime network access or local Rosetta Stone file reads.
- Regenerate goldens only through explicit reviewed test commands.
- Preserve deterministic ordering for findings, evidence, recommendations, and coverage rows.
