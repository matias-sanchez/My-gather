# Implementation Plan: Feedback Backend Worker

**Branch**: `003-feedback-backend-worker` | **Date**: 2026-04-24 (revised 2026-04-26) | **Spec**: [spec.md](./spec.md)

## Summary

A Cloudflare Worker receives the report's feedback payload via HTTPS POST and creates a GitHub Issue on the user's behalf, authenticated as a GitHub App with `Issues: write` scope. The Worker has been live since 2026-04-24 (commit 636a332) at https://my-gather-feedback.mati-orfeo.workers.dev. Each issue is tagged with `user-feedback` + `needs-triage` (always) and `area/<lower(category)>` (only when the user picked a category). The report's Submit button calls `fetch(workerURL, …)` instead of opening GitHub in a new tab. On success the dialog shows an inline success state with a link. On failure the dialog falls back to feature-002's `window.open` flow, so we never regress below the current experience.

This is a dual-repo change. Most work lives in the `feedback-worker/` directory deployed to Cloudflare (its artifact is a live URL, not a binary). A small JS change lives in `render/assets/app.js`. The constitution's Principle IX named exception for this endpoint is already on main at v1.3.0.

## Technical Context

**Language/Version**: Go 1.22 (unchanged). TypeScript 5.x + Wrangler 3.x for the Worker. Browser JS ES2017+ (unchanged).
**Primary Dependencies**:
- Go stdlib only (unchanged).
- Worker: `@cloudflare/workers-types` and a hand-rolled GitHub GraphQL client (decision R1 — `createIssue` mutation + `repository.labels` paginated lookup + token exchange). One runtime JWT library (`jose` — ~8 KB) for signing App JWTs.
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

Re-read the constitution at `.specify/memory/constitution.md` (currently v1.3.0). Relevant principles:

| Principle | Verdict | Rationale / Mitigation |
|---|---|---|
| I. Single Static Binary | PASS | Go binary unchanged; Worker is a separate deploy artifact, not a Go build output. |
| II. Read-Only Inputs | PASS | Worker never sees pt-stalk input trees. Report itself still reads nothing at view-time. |
| III. Graceful Degradation | PASS | Every Worker failure mode falls back to the feature-002 `window.open` path. No feature-level panic. |
| IV. Deterministic Output | PASS | Worker URL is a Go const in `render/feedback.go` (research R7). |
| V. Self-Contained HTML Reports | PASS (transitively, via Principle IX named exception) | The deployed report still embeds all assets (CSS/JS/fonts) inline. No new CDN, no view-time resource fetch except the Submit POST itself, which is the same call permitted by IX's named exception. V's "no remote resource at view-time" is a strict subset of IX's network rule, so the IX exception covers V transitively; no separate V-level exception is needed. The deployed Worker URL ships in `render/feedback.go` as a Go const and is the only cross-origin endpoint the report contacts. |
| VI. Library-First Architecture | PASS | The Worker URL is a Go-side constant in `render/feedback.go` (existing file). No new flag on `cmd/my-gather`. |
| VII. Typed Errors | PASS | No new Go error paths. Worker errors are JSON-typed, not Go-typed. |
| VIII. Reference Fixtures & Golden Tests | PASS | No new parser. Render golden updates by the single Worker-URL line in the dialog's data attribute. |
| IX. Zero Network at Runtime | PASS (via named exception) | The Submit button performs an outbound fetch. The named exception is on main at constitution v1.3.0 (ratified 2026-04-24). No further constitution change is required to merge this feature. See `.specify/memory/constitution.md` Principle IX. |
| X. Minimal Dependencies | PASS | Go side: zero new deps. Worker side: a JWT lib for GitHub App auth — justified as a crypto boundary. |
| XI. Reports Optimized for Humans Under Pressure | PASS | The feature sits exactly where feature 002 put it (small button in header margin). Dialog behaviour is enhanced, not expanded. |
| XII. Pinned Go Version | PASS | No Go version change. |
| XIII. Canonical Code Path (NON-NEGOTIABLE) | PASS | Single submit handler. The fallback to `window.open` is on the failure path only (Principle III); it's not a dual-code-path for the happy case. |
| XIV. English-Only Durable Artifacts | PASS | All code, comments, commit messages, docs in English. |

### Constitution Amendment (ratified — already on main at v1.3.0)

The named exception lives on main at constitution v1.3.0 (ratified 2026-04-24). No further constitution edit is required for this PR. See `.specify/memory/constitution.md` Principle IX for the full text.

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
│   ├── api.md           # Phase 1 — Worker HTTP API contract
│   └── ui.md            # Phase 1 — Report Feedback dialog UI contract
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

feedback-worker/                 # Cloudflare Worker deploy artifact (live)
├── package.json
├── tsconfig.json
├── wrangler.toml                # concrete file — real KV/R2 IDs, nodejs_compat, observability enabled
├── src/
│   ├── index.ts                 # Main Worker entry
│   ├── env.ts                   # Env binding types
│   ├── github-app.ts            # JWT signing + installation-token exchange + GraphQL createIssue + repository.labels lookup
│   ├── idempotency.ts           # KV-backed two-state idempotency (inflight 30s / done 300s)
│   ├── log.ts                   # Structured-log helper
│   ├── r2-upload.ts             # Base64 decode + SHA-256 + R2 PUT
│   ├── ratelimit.ts             # KV per-request unique-key fixed UTC-hour window
│   ├── validate.ts              # Payload validation (incl. report_version_invalid)
│   └── body.ts                  # Issue body composition with category/screenshot/voice sections
└── test/
    ├── body.test.ts
    ├── github-app.test.ts
    ├── ratelimit.test.ts
    └── validate.test.ts

CLAUDE.md                        # MODIFIED — active feature → 003

.specify/memory/constitution.md   # NOT TOUCHED by this PR — already at v1.3.0 with the Principle IX named exception
```

**Status as of this reconciliation**: every entry in the Source Code tree above already exists on main except where noted. The Worker landed in commit 636a332 (2026-04-24) and is live at https://my-gather-feedback.mati-orfeo.workers.dev. The 9 source modules and 4 test files listed under `feedback-worker/` are the real shipped inventory; no `wrangler.toml.template` exists — `wrangler.toml` is the concrete checked-in file.

**Structure Decision**: Worker lives at repo root in `feedback-worker/` (sibling of `cmd/`, `render/`, etc.). Not as a Go subpackage because it's not Go. Not as a git submodule because the coupling to the Go side (shared constant: Worker URL) is tight enough to keep in-tree for atomic commits.

### Files touched (exhaustive)

- `render/feedback.go`: `FeedbackView` carries `WorkerURL`, `GitHubURL`, `Categories` (no `ReportVersion` — `app.js` reads it from `<meta name="generator">` instead). See commit 636a332 for the original implementation.
- `render/feedback_test.go`: asserts the WorkerURL shape against the documented constant.
- `render/templates/report.html.tmpl`: dialog renders `data-feedback-worker-url` and the fallback `data-feedback-url`. Two regions present: `<form id="feedback-form">` and `<div id="feedback-success">`. Inline transients `#feedback-error` and `#feedback-fallback`.
- `render/assets/app.js`: `doSubmit` performs POST → success / fallback path; reads `reportVersion` from the `<meta name="generator">` tag.
- `feedback-worker/` (live deploy artifact; see commit 636a332 for the original implementation; this PR (003a) adds `AbortController` + 504 in `feedback-worker/src/{github-app,index,validate}.ts` per FR-014 — see commit 3 of this branch).
- `CLAUDE.md`: active-feature pointer.

(`.specify/memory/constitution.md` is NOT touched by this feature's
implementation PR — the Principle IX named exception was ratified
in a separate commit at v1.3.0 and is already on main.)

## Post-Design Re-check

*Filled after Phase 1 artifacts; verified in production after the 2026-04-24 deploy.*

- **Determinism preserved**: the Worker URL is a package-level Go const. The rendered HTML has been verified byte-identical across two builds in production; the `data-feedback-worker-url` attribute is stable, and smoke test issues #25 and #26 confirm the end-to-end Worker path.
- **Fallback path preserved**: the feature-002 `window.open` code in `doSubmit` is NOT removed. It is invoked when the Worker path fails (see template `data-feedback-url` for the fallback URL constant).
- **No secrets in shipped HTML**: only the Worker URL ships (a public endpoint). App ID, installation ID, repo ID, R2 prefix, and private key live in Cloudflare Worker Secrets.

Constitution Check passes pre and post, with the Principle IX amendment (already on main at v1.3.0) as the enabling change.
