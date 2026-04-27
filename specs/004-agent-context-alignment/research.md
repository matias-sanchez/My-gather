# Research: Agent Context Alignment

## R1: Keep separate agent context files, enforce shared active feature

**Decision**: Keep `AGENTS.md` for Codex and `CLAUDE.md` for Claude, but
require both `<!-- SPECKIT START -->` blocks to name the same active feature
as `.specify/feature.json`.

**Rationale**: The two runtimes need different tool details, but they must
not disagree about which spec controls the current work.

**Alternatives considered**:

- Single shared context file: rejected because Claude and Codex have
  different local skill discovery paths and runtime conventions.
- No mechanical enforcement: rejected because this drift already occurred.

## R2: Codex wrappers delegate to canonical My-gather review workflows

**Decision**: Add `.agents/skills/pr-review-*-my-gather/` wrappers that point
to the existing `.claude/skills/` procedures and document only Codex-specific
adaptations.

**Rationale**: This preserves one canonical procedure for each review workflow
while making the skill discoverable to Codex.

**Alternatives considered**:

- Copy full Claude skill bodies into `.agents/skills/`: rejected because the
  duplicated text would drift.
- Leave the skills Claude-only: rejected because Codex would keep falling
  back to generic review workflows.

## R3: Enforce alignment with a Go test

**Decision**: Add a focused coverage-package test that verifies active feature
agreement and skill mirror coverage.

**Rationale**: Existing CI already runs Go tests. A small standard-library
test gives maintainers immediate, local feedback without introducing a new
toolchain.

**Alternatives considered**:

- Shell script only: rejected because it would require a new CI hook or
  manual habit.
- Full JSON/YAML schema validator: rejected as unnecessary for this narrow
  drift class.
