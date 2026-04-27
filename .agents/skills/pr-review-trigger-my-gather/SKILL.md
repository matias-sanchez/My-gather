---
name: pr-review-trigger-my-gather
description: "Start a fresh Codex + Copilot review cycle on the current My-gather PR using the repo-local constitution-aware workflow. Use in this repo whenever the user says `/pr-review-trigger-my-gather`, `trigger review`, `ask codex to review`, `start a review round`, `request PR review`, or wants external reviewers to inspect the current PR."
---

# PR Review Trigger (My-gather, Codex)

This is the Codex-facing entry point for the My-gather review-trigger
workflow. The canonical procedure is maintained in:

`.claude/skills/pr-review-trigger-my-gather/SKILL.md`

Before executing, read that file and follow its steps with these Codex
adaptations:

- Treat references to `CLAUDE.md` as agent-specific context references;
  Codex-facing durable guidance lives in `AGENTS.md`.
- Prefer GitHub MCP tools when available; otherwise use `gh` after
  verifying `gh auth status`.
- If the Claude-only `@agent-pre-review-constitution-guard` agent is not
  invocable in the current Codex runtime, run
  `scripts/hooks/pre-push-constitution-guard.sh` as the local mechanical
  guard and report that substitution in the trigger summary.
- Do not delete resolved review threads. Delete only stale trigger issue
  comments as described by the canonical workflow.
- Keep every PR comment, log line, and commit message in English.

This wrapper exists so Codex skill discovery sees the same repo-local
review workflow as Claude without duplicating the implementation text.
