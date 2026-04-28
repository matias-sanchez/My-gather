# Quickstart: Agent Context Alignment

## Verify Active Feature Alignment

```bash
go test ./tests/coverage -run AgentAlignmentActiveFeaturePointers -count=1
```

Expected result: the test passes when `AGENTS.md`, `CLAUDE.md`, and
`.specify/feature.json` all name `004-agent-context-alignment`.

## Verify Skill Mirror Alignment

```bash
go test ./tests/coverage -run AgentAlignmentSkillMirrors -count=1
```

Expected result: every top-level `.claude/skills/<name>/` directory has a
Codex exposure path in either `.agents/skills/<name>/` or
`~/.codex/skills/<name>/`, and no Codex skill slug exists in both places.

## Full Alignment Check

```bash
go test ./tests/coverage -run AgentAlignment -count=1
```

Expected result: active-feature pointers, Codex exposure paths,
duplicate-slug checks, and agent-specific Spec Kit context targets all pass.
On a machine with `~/.codex/skills` configured, this also verifies that the
My-gather PR review skills are in Codex's startup skill path.

## Manual Smoke Check

1. Open `AGENTS.md` and verify the active feature block points at
   `specs/004-agent-context-alignment/`.
2. Open `CLAUDE.md` and verify the same active feature block and prior
   feature list are present.
3. Confirm `.agents/skills/pr-review-trigger-my-gather/`,
   `.agents/skills/pr-review-fix-my-gather/`, and
   `.agents/skills/pr-review-loop-my-gather/` do not exist.
4. Confirm `~/.codex/skills/pr-review-trigger-my-gather/SKILL.md`,
   `~/.codex/skills/pr-review-fix-my-gather/SKILL.md`, and
   `~/.codex/skills/pr-review-loop-my-gather/SKILL.md` exist locally.
5. Confirm `.claude/worktrees/` and `.claude/pre-review.log` are not staged.
