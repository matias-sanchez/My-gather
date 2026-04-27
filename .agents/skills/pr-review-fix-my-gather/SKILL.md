---
name: pr-review-fix-my-gather
description: "Triage Codex/Copilot findings on the current My-gather PR, verify them against live code, apply only constitution-compliant fixes, resolve processed GitHub threads, and push the result. Use in this repo for `address review findings`, `fix codex comments`, `fix copilot comments`, `resolve PR review`, or `/pr-review-fix-my-gather`."
---

# PR Review Fix (My-gather, Codex)

This is the Codex-facing entry point for the My-gather review-fix
workflow. The canonical procedure is maintained in:

`.claude/skills/pr-review-fix-my-gather/SKILL.md`

Before executing, read that file and follow its steps with these Codex
adaptations:

- Treat references to `CLAUDE.md` as agent-specific context references;
  Codex-facing durable guidance lives in `AGENTS.md`.
- Prefer GitHub MCP tools when available; otherwise use `gh` after
  verifying `gh auth status`.
- If the Claude-only `@agent-pre-review-constitution-guard` agent is not
  invocable in the current Codex runtime, run
  `scripts/hooks/pre-push-constitution-guard.sh` as the local mechanical
  guard before committing or pushing.
- Resolve only bot-authored review threads named in the canonical
  workflow. Never resolve human review threads automatically.
- Stage specific paths only. Do not use `git add -A`.
- Keep every PR reply, log line, and commit message in English.

This wrapper exists so Codex skill discovery sees the same repo-local
review workflow as Claude without duplicating the implementation text.
