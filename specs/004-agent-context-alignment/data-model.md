# Data Model: Agent Context Alignment

## AgentContext

Represents a durable context file consumed by an assistant runtime.

- `path`: `AGENTS.md` or `CLAUDE.md`
- `runtime`: `codex` or `claude`
- `activeFeature`: feature slug parsed from the `<!-- SPECKIT START -->`
  block
- `skillRoot`: `.agents/skills` or `.claude/skills`

## CodexStartupSkillRoot

Represents the machine-local skill directory Codex reads at session start.

- `path`: `~/.codex/skills`
- `expectedTarget`: `$HOME/git/my_memories/skills/codex` when symlinked by
  `setup-agent-skills.sh`
- `requiredSkills`: the My-gather PR review skills that must load
  automatically in new Codex sessions

## ActiveFeaturePointer

Represents the machine-readable current feature selection.

- `path`: `.specify/feature.json`
- `featureDirectory`: relative path under `specs/`
- `featureSlug`: basename of `featureDirectory`

## SkillMirror

Represents one skill name that should exist for both assistant runtimes.

- `name`: top-level skill directory name
- `claudePath`: `.claude/skills/<name>/SKILL.md`
- `codexPath`: `.agents/skills/<name>/SKILL.md`
- `mode`: `native` for agent-specific skills, `wrapper` for Codex wrappers
  delegating to canonical Claude procedure text

## Validation Rules

- `AgentContext.activeFeature` for Codex MUST equal
  `ActiveFeaturePointer.featureSlug`.
- `AgentContext.activeFeature` for Claude MUST equal
  `ActiveFeaturePointer.featureSlug`.
- Every top-level directory in `.claude/skills/` MUST have a matching
  top-level directory in `.agents/skills/`.
- Codex `speckit-plan` text MUST reference `AGENTS.md`.
- Claude `speckit-plan` text MUST reference `CLAUDE.md`.
- If `~/.codex/skills` exists, it MUST contain every required My-gather PR
  review skill.
