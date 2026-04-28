# Implementation Plan: Agent Context Alignment

**Branch**: `004-agent-context-alignment` | **Date**: 2026-04-27 | **Spec**: `specs/004-agent-context-alignment/spec.md`
**Input**: Feature specification from `specs/004-agent-context-alignment/spec.md`

## Summary

Align Codex and Claude repository context by making `AGENTS.md`,
`CLAUDE.md`, and `.specify/feature.json` agree on the active feature,
adding Codex-visible wrappers for the My-gather PR review skills, and
adding a focused Go test that catches future drift.

## Technical Context

**Language/Version**: Go 1.24 for repository tests; Markdown/YAML/JSON for
agent artifacts
**Primary Dependencies**: Go standard library only
**Storage**: Repository files
**Testing**: `go test ./tests/coverage -run AgentAlignment -count=1`
**Target Platform**: Local development and CI on macOS/Linux
**Project Type**: CLI/library repository with Spec Kit governance
**Performance Goals**: Alignment test completes in under one second in the
coverage package
**Constraints**: No changes to Claude runtime worktrees or logs; all durable
artifacts in English; no new Go dependencies
**Scale/Scope**: Three context pointers, two repository skill trees, the
Codex startup skill tree when available, one focused test

## Constitution Check

- **I. Single Static Binary**: Not affected; no production binary behavior
  changes.
- **II. Read-Only Inputs**: Not affected; no pt-stalk input handling changes.
- **III. Graceful Degradation**: Not affected.
- **IV. Deterministic Output**: Alignment test uses sorted directory listings
  for deterministic failures.
- **V. Self-Contained HTML Reports**: Not affected.
- **VI. Library-First Architecture**: Not affected; test code remains under
  `tests/coverage`.
- **VII. Typed Errors**: Not affected.
- **VIII. Reference Fixtures & Golden Tests**: Not affected; no parser change.
- **IX. Zero Network at Runtime**: Not affected; no production network code.
- **X. Minimal Dependencies**: Passes; Go standard library only.
- **XI. Reports Optimized for Humans Under Pressure**: Not affected.
- **XII. Pinned Go Version**: Passes; `go.mod` is unchanged.
- **XIII. Canonical Code Path**: Passes; Codex PR review skills are wrappers
  that delegate to the canonical My-gather workflow text instead of copying
  the full procedure.
- **XIV. English-Only Durable Artifacts**: Passes; all added artifacts are in
  English.

## Project Structure

### Documentation (this feature)

```text
specs/004-agent-context-alignment/
|-- spec.md
|-- plan.md
|-- research.md
|-- data-model.md
|-- quickstart.md
`-- tasks.md
```

### Source Code (repository root)

```text
AGENTS.md
CLAUDE.md
.specify/feature.json
.agents/skills/pr-review-trigger-my-gather/SKILL.md
.agents/skills/pr-review-fix-my-gather/SKILL.md
.agents/skills/pr-review-loop-my-gather/SKILL.md
tests/coverage/agent_alignment_test.go
README.md
~/.codex/skills/pr-review-*-my-gather/SKILL.md  # machine-local coverage
```

**Structure Decision**: Keep agent-specific entry points separate while
testing the invariants that must remain shared.

## Complexity Tracking

No constitution violations require justification.
