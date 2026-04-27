---
description: "Task list for Agent Context Alignment implementation"
---

# Tasks: Agent Context Alignment

**Input**: Design documents from `specs/004-agent-context-alignment/`
**Prerequisites**: spec.md, plan.md, research.md, data-model.md, quickstart.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel when files do not overlap
- **[Story]**: User story covered by the task

## Phase 1: Spec and branch setup

- [x] T001 Create branch `004-agent-context-alignment`.
- [x] T002 Add `specs/004-agent-context-alignment/` design artifacts.

## Phase 2: User Story 1 - Active feature alignment (Priority: P1)

- [x] T003 [US1] Update `AGENTS.md` so its Spec Kit block points at
  `specs/004-agent-context-alignment/`.
- [x] T004 [US1] Update `CLAUDE.md` so its Spec Kit block points at
  `specs/004-agent-context-alignment/`.
- [x] T005 [US1] Update `.specify/feature.json` to
  `specs/004-agent-context-alignment`.

## Phase 3: User Story 2 - Codex skill discovery (Priority: P2)

- [x] T006 [P] [US2] Add
  `.agents/skills/pr-review-trigger-my-gather/SKILL.md`.
- [x] T007 [P] [US2] Add `.agents/skills/pr-review-fix-my-gather/SKILL.md`.
- [x] T008 [P] [US2] Add `.agents/skills/pr-review-loop-my-gather/SKILL.md`.
- [x] T009 [US2] Update `README.md` repository layout to include
  `.agents/`.

## Phase 4: User Story 3 - Mechanical drift detection (Priority: P3)

- [x] T010 [US3] Add `tests/coverage/agent_alignment_test.go` covering
  active feature pointers, skill mirrors, and agent-specific plan targets.
- [x] T011 [US3] Run
  `go test ./tests/coverage -run AgentAlignment -count=1`.
- [x] T012 Run `go test ./... -count=1`.
- [ ] T013 Push branch and open PR.
