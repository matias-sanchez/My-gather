# Feature Specification: Agent Context Alignment

**Feature Branch**: `004-agent-context-alignment`
**Created**: 2026-04-27
**Status**: Draft
**Input**: User description: "Open a new PR/branch and work on this alignment to ensure 100% side-by-side work between Codex and Claude."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Codex resolves the same active feature as Claude (Priority: P1)

A maintainer opens the repository in Codex after Claude has generated or
updated Spec Kit artifacts. Codex reads `AGENTS.md`, Claude reads
`CLAUDE.md`, and Spec Kit scripts read `.specify/feature.json`. All three
sources point at the same active feature, so neither agent starts from a
stale spec.

**Why this priority**: If the active feature pointers disagree, all later
alignment is unreliable because agents may edit different specs.

**Independent Test**: Run the agent alignment test and verify it fails if
any one of `AGENTS.md`, `CLAUDE.md`, or `.specify/feature.json` names a
different active feature.

**Acceptance Scenarios**:

1. **Given** Codex opens the repository, **When** it reads `AGENTS.md`,
   **Then** the active feature matches `.specify/feature.json`.
2. **Given** Claude opens the repository, **When** it reads `CLAUDE.md`,
   **Then** the active feature matches `.specify/feature.json`.
3. **Given** either agent changes the active feature, **When** tests run,
   **Then** mismatched context files fail the build.

---

### User Story 2 - Codex discovers repo-local review workflows (Priority: P2)

A maintainer asks Codex to trigger, fix, or loop PR review for My-gather.
Codex can discover My-gather-specific skill entries under `.agents/skills/`
instead of falling back to generic global workflows that target the wrong
project conventions.

**Why this priority**: Review automation is the highest-risk place for a
generic workflow to violate this repository's constitution.

**Independent Test**: Compare top-level skill directories under
`.claude/skills/` and `.agents/skills/`; every Claude skill has a Codex
counterpart.

**Acceptance Scenarios**:

1. **Given** `.claude/skills/pr-review-trigger-my-gather/` exists, **When**
   Codex scans `.agents/skills/`, **Then** a matching
   `pr-review-trigger-my-gather` skill exists.
2. **Given** Claude adds a new repo-local skill, **When** tests run without
   a matching `.agents/skills/` directory, **Then** the build fails.

---

### User Story 3 - Future drift is caught mechanically (Priority: P3)

A contributor updates Claude-facing or Codex-facing instructions months
later. A focused test catches active-feature drift and missing skill mirrors
before the PR merges.

**Why this priority**: Documentation-only conventions decay unless they are
checked by CI.

**Independent Test**: Run `go test ./tests/coverage -run AgentAlignment`
and inspect the failure message after intentionally introducing a local
drift.

**Acceptance Scenarios**:

1. **Given** `.agents/skills/speckit-plan/SKILL.md` exists, **When** tests
   run, **Then** it must target `AGENTS.md`.
2. **Given** `.claude/skills/speckit-plan/SKILL.md` exists, **When** tests
   run, **Then** it must target `CLAUDE.md`.

### Edge Cases

- Existing Claude runtime files such as `.claude/pre-review.log`,
  `.claude/scheduled_tasks.lock`, and `.claude/worktrees/` are runtime
  state and MUST NOT be required for Codex alignment.
- Codex wrappers MUST NOT duplicate the full Claude workflow text; they
  should delegate to the canonical My-gather procedure and document only
  Codex-specific adaptations.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `AGENTS.md`, `CLAUDE.md`, and `.specify/feature.json` MUST
  agree on the active feature.
- **FR-002**: `AGENTS.md` MUST document the side-by-side agent contract,
  including which paths are Codex-facing and Claude-facing.
- **FR-003**: Every top-level skill directory under `.claude/skills/` MUST
  have a matching top-level directory under `.agents/skills/`.
- **FR-004**: Codex-visible wrappers MUST exist for
  `pr-review-trigger-my-gather`, `pr-review-fix-my-gather`, and
  `pr-review-loop-my-gather`.
- **FR-005**: A repository test MUST fail on active-feature pointer drift.
- **FR-006**: A repository test MUST fail when a Claude skill lacks a Codex
  skill directory.
- **FR-007**: Agent-specific Spec Kit plan skills MUST continue to target
  their own context files: Codex updates `AGENTS.md`, Claude updates
  `CLAUDE.md`.

### Key Entities

- **Agent Context File**: Durable repo guidance file consumed by a specific
  assistant runtime, either `AGENTS.md` for Codex or `CLAUDE.md` for Claude.
- **Skill Mirror**: A Codex-visible `.agents/skills/<name>/` directory that
  corresponds to a Claude-visible `.claude/skills/<name>/` directory.
- **Active Feature Pointer**: The current feature name recorded in human
  context files and `.specify/feature.json`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `go test ./tests/coverage -run AgentAlignment -count=1`
  passes.
- **SC-002**: `git diff --name-only origin/main...HEAD` shows no edits to
  Claude runtime state under `.claude/worktrees/` or `.claude/pre-review.log`.
- **SC-003**: A future mismatch between `AGENTS.md` and `CLAUDE.md` active
  feature names produces a clear test failure.
- **SC-004**: A future Claude-only skill addition produces a clear test
  failure until a Codex-visible counterpart is added.

## Assumptions

- Codex and Claude can share durable repository guidance, but their runtime
  tool invocation details may differ.
- `.claude/skills/` remains the canonical location for the existing
  My-gather PR review workflow text.
- `.agents/skills/` is the correct local discovery path for Codex-visible
  repo skills in this workspace.
