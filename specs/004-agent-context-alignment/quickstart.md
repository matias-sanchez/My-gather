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
matching `.agents/skills/<name>/` directory.

## Full Alignment Check

```bash
go test ./tests/coverage -run AgentAlignment -count=1
```

Expected result: active-feature pointers, skill mirrors, and agent-specific
Spec Kit context targets all pass.

## Manual Smoke Check

1. Open `AGENTS.md` and verify the active feature block points at
   `specs/004-agent-context-alignment/`.
2. Open `CLAUDE.md` and verify the same active feature block and prior
   feature list are present.
3. Confirm `.agents/skills/pr-review-trigger-my-gather/SKILL.md`,
   `.agents/skills/pr-review-fix-my-gather/SKILL.md`, and
   `.agents/skills/pr-review-loop-my-gather/SKILL.md` exist.
4. Confirm `.claude/worktrees/` and `.claude/pre-review.log` are not staged.
