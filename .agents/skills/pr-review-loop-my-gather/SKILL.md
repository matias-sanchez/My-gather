---
name: pr-review-loop-my-gather
description: "Drive the current My-gather PR toward a clean Codex review by composing `/pr-review-fix-my-gather` and `/pr-review-trigger-my-gather` with bounded iterations. Use when the user says `/pr-review-loop-my-gather`, `loop the reviews`, `drive the PR to green`, or asks for unattended review convergence."
---

# PR Review Loop (My-gather, Codex)

This is the Codex-facing entry point for the My-gather review-loop
workflow. The canonical procedure is maintained in:

`.claude/skills/pr-review-loop-my-gather/SKILL.md`

Before executing, read that file and follow its steps with these Codex
adaptations:

- Compose the Codex-visible wrappers in `.agents/skills/` for trigger and
  fix semantics. Do not reimplement their procedures inside the loop.
- Treat references to `CLAUDE.md` as agent-specific context references;
  Codex-facing durable guidance lives in `AGENTS.md`.
- If the Claude-only scheduling primitive named by the canonical workflow
  is unavailable in the current Codex runtime, stop after the current
  iteration and report the next manual command to run instead of sleeping
  or spinning in-process.
- Keep every artifact produced by the loop in English.

This wrapper exists so Codex skill discovery sees the same repo-local
review workflow as Claude without duplicating the implementation text.
