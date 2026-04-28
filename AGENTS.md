<!-- SPECKIT START -->
Active feature: **004-agent-context-alignment**

For technologies, project structure, constitution gates, data model,
agent context contracts, and shell commands, read the current plan:

- Plan: `specs/004-agent-context-alignment/plan.md`
- Spec: `specs/004-agent-context-alignment/spec.md`
- Research (Phase 0): `specs/004-agent-context-alignment/research.md`
- Data model (Phase 1): `specs/004-agent-context-alignment/data-model.md`
- Quickstart (Phase 1): `specs/004-agent-context-alignment/quickstart.md`
- Tasks (Phase 2): `specs/004-agent-context-alignment/tasks.md`
- Constitution: `.specify/memory/constitution.md`

Prior features:
- **003-feedback-backend-worker** - `specs/003-feedback-backend-worker/plan.md`
- **002-report-feedback-button** - `specs/002-report-feedback-button/plan.md`
- **001-ptstalk-report-mvp** - `specs/001-ptstalk-report-mvp/plan.md`
<!-- SPECKIT END -->

## Language convention

English-only for all checked-in artifacts (Principle XIV of
`.specify/memory/constitution.md`). Out-of-band chat with the user is
not an artifact and may be in any language.

## Side-by-side agent contract

Codex and Claude are expected to work on this repository side by side.
Keep the following files aligned whenever a Spec Kit workflow changes
the active feature or agent instructions:

- `AGENTS.md` is the Codex-facing context file.
- `CLAUDE.md` is the Claude-facing context file.
- `.specify/feature.json` is the machine-readable active feature pointer.
- `.agents/skills/` contains Codex-visible repo-local skills that are not
  already present in Codex's startup skill directory.
- `.claude/skills/` contains Claude-visible local skills.

The `<!-- SPECKIT START -->` blocks in `AGENTS.md` and `CLAUDE.md` MUST
name the same active feature as `.specify/feature.json`. If those three
signals disagree, stop before running `/speckit-*` workflows and reconcile
the drift first.

When adding a repo-local Claude skill that should be invocable by Codex,
expose it in exactly one Codex skill path: either `.agents/skills/` for a
repository-local skill or `~/.codex/skills` for an automatic startup skill.
Do not use the same skill name in both places; Codex shows duplicate picker
entries when the slug is loaded twice. The alignment test in
`tests/coverage/agent_alignment_test.go` enforces this.

Codex startup skills are loaded from `~/.codex/skills` on this machine,
which is expected to be a symlink into
`$HOME/git/my_memories/skills/codex`. The My-gather PR review skills are
startup skills, not `.agents/skills` entries, so they load automatically
without duplicate picker rows. The alignment test checks that
machine-local coverage when `~/.codex/skills` exists.

## Repo-local Codex tooling

- `/pr-review-trigger-my-gather` (skill at
  `~/.codex/skills/pr-review-trigger-my-gather/`) - start an external
  Codex + Copilot review cycle on the current PR. Use this in this repo,
  not the generic `/pr-review-trigger`.
- `/pr-review-fix-my-gather` (skill at
  `~/.codex/skills/pr-review-fix-my-gather/`) - process Codex/Copilot
  findings using the My-gather constitution and Go-native validation.
  Use this in this repo, not the generic `/pr-review-fix`.
- `/pr-review-loop-my-gather` (skill at
  `~/.codex/skills/pr-review-loop-my-gather/`) - bounded review loop that
  composes the trigger and fix workflows until Codex is clean or a safety
  rail fires.

Claude owns equivalent skill entry points under `.claude/skills/`. Keep
the semantics matched; only the agent-specific context file names and
tool invocation details should differ.
