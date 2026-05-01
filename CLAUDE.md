<!-- SPECKIT START -->
Active feature: **009-spec-code-reconciliation**

For technologies, project structure, constitution gates, data model,
contracts, and shell commands, read the current plan:

- Plan: `specs/009-spec-code-reconciliation/plan.md`
- Spec: `specs/009-spec-code-reconciliation/spec.md`
- Research: `specs/009-spec-code-reconciliation/research.md`
- Data model: `specs/009-spec-code-reconciliation/data-model.md`
- Contracts: `specs/009-spec-code-reconciliation/contracts/reconciliation.md`
- Quickstart: `specs/009-spec-code-reconciliation/quickstart.md`
- Tasks: `specs/009-spec-code-reconciliation/tasks.md`
- Constitution: `.specify/memory/constitution.md`

Prior features:
- **008-archive-input** - `specs/008-archive-input/plan.md`
- **007-advisor-intelligence** - `specs/007-advisor-intelligence/plan.md`
- **006-observed-slowest-queries** - `specs/006-observed-slowest-queries/plan.md`
- **005-richer-processlist-metrics** - `specs/005-richer-processlist-metrics/plan.md`
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
- `.specify/feature.json` is the machine-readable feature pointer.
- `.agents/skills/` contains Codex-visible repo-local skills.
- `.claude/skills/` contains Claude-visible repo-local skills.

When `AGENTS.md` and `CLAUDE.md` say there is no active feature,
`.specify/feature.json` may point at the latest shipped feature as a
reference for Spec Kit scripts. When a feature is active, all three
signals must name the same feature before running `/speckit-*` workflows.

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
- `/pr-review-loop-my-gather` (skill at
  `.claude/skills/pr-review-loop-my-gather/`) — bounded convergence
  loop that composes `/pr-review-fix-my-gather` and
  `/pr-review-trigger-my-gather` and self-schedules via
  ScheduleWakeup until Codex reports zero open threads on the PR.
  One-shot invocation; safety rails on max iterations, non-progress,
  guard failure, and Codex response timeout. Use when you want the
  PR driven to green unattended.
- `@agent-pre-review-constitution-guard` (agent at
  `.claude/agents/pre-review-constitution-guard.md`) — local pre-push
  reviewer that emits P1/P2/P3 findings against the constitution
  before you push.
- Pre-push hook (`scripts/hooks/pre-push-constitution-guard.sh`) wired
  via `.claude/settings.json` — mechanical checks that block pushes
  violating Principles I, VIII, or X.
