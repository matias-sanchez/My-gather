<!-- SPECKIT START -->
Active feature: **003-feedback-backend-worker**

For technologies, project structure, constitution gates, data model,
API contract, and shell commands, read the current plan:

- Plan: `specs/003-feedback-backend-worker/plan.md`
- Spec: `specs/003-feedback-backend-worker/spec.md`
- Research (Phase 0): `specs/003-feedback-backend-worker/research.md`
- Data model (Phase 1): `specs/003-feedback-backend-worker/data-model.md`
- Contract (Phase 1): `specs/003-feedback-backend-worker/contracts/api.md`
- Quickstart (Phase 1): `specs/003-feedback-backend-worker/quickstart.md`
- Tasks (Phase 2): `specs/003-feedback-backend-worker/tasks.md`
- Constitution: `.specify/memory/constitution.md`

Prior features (shipped):
- **002-report-feedback-button** — `specs/002-report-feedback-button/plan.md`
- **001-ptstalk-report-mvp** — `specs/001-ptstalk-report-mvp/plan.md`
<!-- SPECKIT END -->

## Language convention

English-only for all checked-in artifacts (Principle XIV of
`.specify/memory/constitution.md`). Out-of-band chat with the user is
not an artifact and may be in any language.

## Repo-local Claude Code tooling

- `/pr-review-trigger-my-gather` (skill at
  `.claude/skills/pr-review-trigger-my-gather/`) — start an external
  Codex + Copilot review cycle on the current PR. Pre-flights with
  the constitution guard, preserves resolved bot threads from prior
  cycles, and posts trigger comments that cite the constitution and
  list the My-gather-specific anti-patterns so bots can self-filter.
  **Use this in this repo, not the global `/pr-review-trigger`.**
- `/pr-review-fix-my-gather` (skill at `.claude/skills/pr-review-fix-my-gather/`)
  — the My-gather variant of the PR review-fix workflow. Walks the 14
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
