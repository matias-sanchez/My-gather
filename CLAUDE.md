<!-- SPECKIT START -->
Active feature: **001-ptstalk-report-mvp**

For technologies, project structure, constitution gates, data model,
CLI and package contracts, and shell commands, read the current plan:

- Plan: `specs/001-ptstalk-report-mvp/plan.md`
- Spec: `specs/001-ptstalk-report-mvp/spec.md`
- Research (Phase 0): `specs/001-ptstalk-report-mvp/research.md`
- Data model (Phase 1): `specs/001-ptstalk-report-mvp/data-model.md`
- Contracts (Phase 1): `specs/001-ptstalk-report-mvp/contracts/cli.md`,
  `specs/001-ptstalk-report-mvp/contracts/packages.md`
- Quickstart (Phase 1): `specs/001-ptstalk-report-mvp/quickstart.md`
- Constitution: `.specify/memory/constitution.md`
<!-- SPECKIT END -->

## Language convention

All artifacts that live inside this repository MUST be written in English:

- Source code, comments, and identifiers.
- Commit messages, branch names, tags.
- Pull request titles, descriptions, and review comments.
- `specs/`, `.specify/`, `README.md`, and any other checked-in documentation.
- GitHub issues opened from this repo.

Out-of-band conversation with the user (chat/UI) may be in any language
the user chooses — only the durable, shared artifacts are required to
be English so contributors and tooling have a single consistent language.

## Repo-local Claude Code tooling

- `/pr-review-trigger-my-gather` (skill at
  `.claude/skills/pr-review-trigger-my-gather/`) — start an external
  Codex + Copilot review cycle on the current PR. Pre-flights with
  the constitution guard, preserves resolved bot threads from prior
  cycles, and posts trigger comments that cite the constitution and
  list the My-gather-specific anti-patterns so bots can self-filter.
  **Use this in this repo, not the global `/pr-review-trigger`.**
- `/pr-review-fix-my-gather` (skill at `.claude/skills/pr-review-fix-my-gather/`)
  — the My-gather variant of the PR review-fix workflow. Walks the 13
  principles in `.specify/memory/constitution.md`, uses Go-native
  validation, and marks review threads resolved on GitHub. **Use this
  in this repo, not the global `/pr-review-fix`** — the global one
  targets Python projects and would apply the wrong rules here.
- `@agent-pre-review-constitution-guard` (agent at
  `.claude/agents/pre-review-constitution-guard.md`) — local pre-push
  reviewer that emits P1/P2/P3 findings against the constitution
  before you push.
- Pre-push hook (`scripts/hooks/pre-push-constitution-guard.sh`) wired
  via `.claude/settings.json` — mechanical checks that block pushes
  violating Principles I, VIII, or X.
