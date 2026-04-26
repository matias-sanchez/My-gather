# Implementation Plan: Feedback Backend Worker

**Branch**: `003-feedback-backend-worker` | **Date**: 2026-04-24 (revised 2026-04-26) | **Spec**: [spec.md](./spec.md)

## Summary

Add a minimal Cloudflare Worker that receives the report's feedback payload via HTTPS POST and creates a GitHub Issue on the user's behalf, authenticated as a GitHub App with `Issues: write` scope. The Issue is tagged with the `feedback` label and (when the user picked a category) one of `area:ui` / `area:parser` / `area:advisor` / `area:other`. The report's Submit button calls `fetch(workerURL, …)` instead of opening GitHub in a new tab. On success the dialog shows an inline success state with a link. On failure the dialog falls back to feature-002's `window.open` flow, so we never regress below the current experience.

This is a dual-repo change. Most work lives in a new `feedback-worker/` directory that is deployed to Cloudflare (its artifact is a live URL, not a binary). A small JS change lands in `render/assets/app.js`. A constitution amendment adds a named exception to Principle IX (Zero Network at Runtime) for this specific endpoint.

## Technical Context

**Language/Version**: Go 1.22 (unchanged). TypeScript 5.x + Wrangler 3.x for the Worker. Browser JS ES2017+ (unchanged).
**Primary Dependencies**:
- Go stdlib only (unchanged).
- Worker: `@cloudflare/workers-types` and a hand-rolled GitHub REST client (decision R1 — ~30 LOC for `POST /repos/.../issues` + token exchange). One runtime JWT library (`jose` — ~8 KB) for signing App JWTs.
- Browser: native `fetch` API only. No polyfill.
**Storage**: Cloudflare KV namespace `FEEDBACK_RATELIMIT` (rate-limit counters) and `FEEDBACK_IDEMP` (idempotency cache). No persistent storage of user content.
**Testing**:
- Go side: add one test asserting the Worker URL is embedded in the rendered HTML and the JS reads it. Reuse feature-002's determinism test — the Worker URL is a constant, so determinism holds.
- Worker side: `vitest` + `@cloudflare/vitest-pool-workers` for unit tests of the payload validator and rate-limiter. `wrangler dev` + manual curl for end-to-end.
**Target Platform**: Cloudflare Workers edge network (global). Report itself renders in any modern browser.
**Project Type**: Go CLI + sidecar edge Worker. First time the repo ships a non-Go deployable artifact.
**Performance Goals**: p95 end-to-end Submit → success state ≤ 3s. Worker CPU per request ≤ 10ms (free-tier limit).
**Constraints**: zero user-secret in the shipped HTML. Fallback to `window.open` on any Worker failure. $0/month infra cost.
**Scale/Scope**: realistic ceiling ~100 submits/day across all users of all reports. Free tier covers 1000× this.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Re-read the constitution at `.specify/memory/constitution.md` (currently v1.2.0). Relevant principles:

| Principle | Verdict | Rationale / Mitigation |
|---|---|---|
| I. Single Static Binary | PASS | Go binary unchanged; Worker is a separate deploy artifact, not a Go build output. |
| II. Read-Only Inputs | PASS | Worker never sees pt-stalk input trees. Report itself still reads nothing at view-time. |
| III. Graceful Degradation | PASS | Every Worker failure mode falls back to the feature-002 `window.open` path. No feature-level panic. |
| IV. Deterministic Output | PASS | Worker URL is a build-time constant embedded via `-ldflags` or a Go `const`. Rendered markup is byte-identical across runs. |
| V. Self-Contained HTML Reports | PARTIAL — see amendment | The report still embeds all assets (CSS/JS/fonts) inline. No new CDN. But the Submit action performs a runtime `fetch` to a Worker URL, which is a runtime network call. Addressed by the same amendment that covers Principle IX. |
| VI. Library-First Architecture | PASS | The Worker URL is a Go-side constant in `render/feedback.go` (existing file). No new flag on `cmd/my-gather`. |
| VII. Typed Errors | PASS | No new Go error paths. Worker errors are JSON-typed, not Go-typed. |
| VIII. Reference Fixtures & Golden Tests | PASS | No new parser. Render golden updates by the single Worker-URL line in the dialog's data attribute. |
| IX. Zero Network at Runtime | PASS (via named exception) | The Submit button performs an outbound fetch. A named exception is documented under Principle IX and was ratified in constitution **v1.3.0** (2026-04-24, already on main). The amendment text is reproduced below for reference; no further constitution change is required to merge this feature. |
| X. Minimal Dependencies | PASS | Go side: zero new deps. Worker side: a JWT lib for GitHub App auth — justified as a crypto boundary. |
| XI. Reports Optimized for Humans Under Pressure | PASS | The feature sits exactly where feature 002 put it (small button in header margin). Dialog behaviour is enhanced, not expanded. |
| XII. Pinned Go Version | PASS | No Go version change. |
| XIII. Canonical Code Path (NON-NEGOTIABLE) | PASS | Single submit handler. The fallback to `window.open` is on the failure path only (Principle III); it's not a dual-code-path for the happy case. |
| XIV. English-Only Durable Artifacts | PASS | All code, comments, commit messages, docs in English. |

### Constitution Amendment (ratified — already on main at v1.3.0)

The named exception cited in the table above lives at
`.specify/memory/constitution.md` (Principle IX). It was ratified
2026-04-24 specifically for this feature and was bumped from v1.2.0
to v1.3.0 (MINOR — additive, named exception, no existing principle
redefined). No further constitution edit is required to merge the
Worker; only the implementation of this plan's tasks remains.

The relevant text on main reads:

```
**Named exceptions**: The following runtime network calls are allowed
when the user has an explicit intent expressed by a UI action:

- **Feedback submission (ratified 2026-04-24 for feature
  003-feedback-backend-worker)**: On the user clicking the "Submit"
  button of the Report Feedback dialog, the report MAY perform a
  single outbound `POST` to a project-controlled HTTPS endpoint.
  [...full text in constitution.md, Principle IX...]
```

### Complexity Tracking

| Complexity added | Why needed | Simpler alternative rejected because |
|---|---|---|
| Cloudflare Worker (new deploy target) | Only way to hold a GitHub App credential without shipping it to the browser | Shipping a token in the HTML — reviewed, rejected for security |
| GitHub App (new secret to rotate) | Per-installation revocation + short-lived tokens | PAT with full user scope — worse blast radius if leaked |
| Constitution amendment IX-E1 | Current wording forbids any runtime network | Silently violating it — rejected; constitution is the non-negotiable baseline |

## Project Structure

### Documentation (this feature)

```text
specs/003-feedback-backend-worker/
├── plan.md              # This file
├── spec.md              # Feature spec
├── research.md          # Phase 0 decisions
├── data-model.md        # Phase 1 — payload shapes + KV schema
├── quickstart.md        # Phase 1 — deploy + test walkthrough
├── contracts/
│   └── api.md           # Phase 1 — Worker HTTP API contract
├── checklists/
│   └── requirements.md  # Quality checklist
└── tasks.md             # Phase 2 — generated by /speckit.tasks
```

### Source Code (repository root)

```text
render/
├── feedback.go                  # MODIFIED — add WorkerURL constant to FeedbackView
├── feedback_test.go             # MODIFIED — assert WorkerURL is the documented constant
├── templates/
│   └── report.html.tmpl         # MODIFIED — dialog gets data-feedback-worker-url attr
└── assets/
    └── app.js                   # MODIFIED — doSubmit() uses fetch(workerURL) with window.open fallback

feedback-worker/                 # NEW — Cloudflare Worker deploy artifact
├── package.json
├── tsconfig.json
├── wrangler.toml
├── src/
│   ├── index.ts                 # Main Worker entry
│   ├── github-app.ts            # JWT signing + installation-token exchange
│   ├── ratelimit.ts             # KV-backed sliding window
│   ├── idempotency.ts           # KV-backed idempotency cache
│   └── validate.ts              # Payload validation
└── test/
    ├── validate.test.ts
    ├── ratelimit.test.ts
    └── e2e.test.ts              # vitest with workers pool

.specify/memory/
└── constitution.md              # MODIFIED — add named exception to Principle IX, bump 1.2.0 → 1.3.0

CLAUDE.md                        # MODIFIED — active feature → 003
```

**Structure Decision**: Worker lives at repo root in `feedback-worker/` (sibling of `cmd/`, `render/`, etc.). Not as a Go subpackage because it's not Go. Not as a git submodule because the coupling to the Go side (shared constant: Worker URL) is tight enough to keep in-tree for atomic commits.

### Files touched (exhaustive)

- `render/feedback.go` (+~5 LOC): add `WorkerURL string` field to `FeedbackView`, populate from `feedbackWorkerURL` constant.
- `render/feedback_test.go` (+~15 LOC): assert the WorkerURL shape.
- `render/templates/report.html.tmpl` (+~1 LOC): dialog gets `data-feedback-worker-url="{{ .Feedback.WorkerURL }}"`.
- `render/assets/app.js` (~40 LOC changed in `doSubmit`): try POST first, fall back to `window.open` on any non-ok response or network error; render success state on ok.
- `render/report_feedback_test.go` (+~30 LOC): add test for Worker-URL presence + fallback behaviour assertions.
- `feedback-worker/` (new, ~400 LOC TypeScript + tests).
- `.specify/memory/constitution.md`: amendment + version bump to 1.3.0.
- `CLAUDE.md`: active-feature pointer.

## Post-Design Re-check

*Filled after Phase 1 artifacts.*

- **Determinism preserved**: the Worker URL is a package-level `const` in Go. Two renders of the same input produce byte-identical HTML including the `data-feedback-worker-url` attribute.
- **Fallback path preserved**: the feature-002 `window.open` code in `doSubmit` is NOT removed. It is invoked when the Worker path fails.
- **No secrets in shipped HTML**: only the Worker URL ships (a public endpoint). App ID, installation ID, and private key live in Cloudflare Worker Secrets.

Constitution Check passes pre and post, with the Principle IX amendment as the enabling change.
