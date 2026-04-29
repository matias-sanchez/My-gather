<!-- SPECKIT START -->
Active feature: **007-advisor-intelligence**

For technologies, project structure, constitution gates, data model,
package/UI contracts, and shell commands, read the current plan:

- Plan: `specs/007-advisor-intelligence/plan.md`
- Spec: `specs/007-advisor-intelligence/spec.md`
- Research: `specs/007-advisor-intelligence/research.md`
- Data model: `specs/007-advisor-intelligence/data-model.md`
- Contracts: `specs/007-advisor-intelligence/contracts/packages.md`,
  `specs/007-advisor-intelligence/contracts/ui.md`
- Quickstart: `specs/007-advisor-intelligence/quickstart.md`
- Constitution: `.specify/memory/constitution.md`

Prior features:
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

## Repo-local Codex tooling

- `/pr-review-trigger-my-gather` (startup skill at
  `~/.codex/skills/pr-review-trigger-my-gather/`) - start an external
  Codex + Copilot review cycle on the current PR. Use this in this repo,
  not the generic `/pr-review-trigger`.
- `/pr-review-fix-my-gather` (startup skill at
  `~/.codex/skills/pr-review-fix-my-gather/`) - process Codex/Copilot
  findings using the My-gather constitution and Go-native validation.
  Use this in this repo, not the generic `/pr-review-fix`.
- `/pr-review-loop-my-gather` (startup skill at
  `~/.codex/skills/pr-review-loop-my-gather/`) - bounded review loop
  that composes the trigger and fix workflows until Codex is clean or a
  safety rail fires.

Claude owns equivalent skill entry points under `.claude/skills/`. Keep
the semantics matched; only the agent-specific context file names and
tool invocation details should differ. Do not duplicate the My-gather PR
review startup skills under `.agents/skills/`; Codex loads them from
`~/.codex/skills`.
