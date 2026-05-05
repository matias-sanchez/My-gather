# Implementation Plan: Feedback Backend Worker

**Branch**: `003-feedback-backend-worker` | **Date**: 2026-04-24 (revised 2026-04-26) | **Spec**: [spec.md](./spec.md)

## Summary

A Cloudflare Worker receives the report's feedback payload via HTTPS POST and creates a GitHub Issue on the user's behalf, authenticated as a GitHub App with `Issues: write` scope. The Worker has been live since 2026-04-24 (commit 636a332) at https://my-gather-feedback.mati-orfeo.workers.dev. Each issue is tagged with `user-feedback` + `needs-triage` (always) and `area/<lower(category)>` (only when the user picked a category). The report's Submit button calls `fetch(workerURL, …)` instead of opening GitHub in a new tab. On success the dialog shows an inline success state with a link. On failure the dialog falls back to feature-002's `window.open` flow, so we never regress below the current experience.

This is a dual-repo change. Most work lives in the `feedback-worker/` directory deployed to Cloudflare (its artifact is a live URL, not a binary). A small JS change lives in the embedded report app asset. The constitution's Principle IX named exception for this endpoint is already on main.

## Technical Context

**Language/Version**: Go version per current `go.mod` (unchanged by this feature). TypeScript 5.x + Wrangler 4.x for the Worker. Browser JS ES2017+ (unchanged).
**Primary Dependencies**:
- Go stdlib only (unchanged).
- Worker: `@cloudflare/workers-types` and a hand-rolled GitHub GraphQL client (decision R1 — `createIssue` mutation + `repository.labels` paginated lookup + token exchange). One runtime JWT library (`jose` — ~8 KB) for signing App JWTs.
- Browser: native `fetch` API only. No polyfill.

Worker direct dependency justification:

| Dependency | Scope | Justification | Transitive footprint |
|------------|-------|---------------|----------------------|
| `jose` | runtime | Signs GitHub App JWTs in the Worker without hand-rolling crypto primitives. | Small pure JS dependency tree; no browser/report dependency. |
| `@cloudflare/workers-types` | dev | Type definitions for Cloudflare Workers bindings and runtime APIs. | Type-only; not bundled into the Worker. |
| `typescript` | dev | Type-checks Worker source before deploy. | Build-time only. |
| `vitest` | dev | Worker unit test runner. | Test-only. |
| `@cloudflare/vitest-pool-workers` | dev | Runs Vitest against a Workers-compatible runtime. | Test-only; mirrors Cloudflare runtime behavior. |
| `wrangler` | dev/deploy | Cloudflare deployment CLI for the Worker. | Deploy-time only; not part of the report or Go binary. |
**Storage**: Cloudflare KV namespace `FEEDBACK_RATELIMIT` (rate-limit counters) and `FEEDBACK_IDEMP` (idempotency cache). No persistent storage of user content.
**Testing**:
- Go side: add one test asserting the canonical feedback contract is embedded in the rendered HTML and the JS reads it. Reuse feature-002's determinism test — the contract is build-time data, so determinism holds.
- Worker side: `vitest` + `@cloudflare/vitest-pool-workers` for unit tests of the payload validator and rate-limiter. `wrangler dev` + manual curl for end-to-end.
**Target Platform**: Cloudflare Workers edge network (global). Report itself renders in any modern browser.
**Project Type**: Go CLI + sidecar edge Worker. First time the repo ships a non-Go deployable artifact.
**Performance Goals**: p95 end-to-end Submit → success state ≤ 3s. Worker CPU per request ≤ 10ms (free-tier limit).
**Constraints**: zero user-secret in the shipped HTML. Fallback to `window.open` on any Worker failure. $0/month infra cost.
**Scale/Scope**: realistic ceiling ~100 submits/day across all users of all reports. Free tier covers 1000× this.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Re-read the constitution at `.specify/memory/constitution.md`. Relevant principles:

| Principle | Verdict | Rationale / Mitigation |
|---|---|---|
| I. Single Static Binary | PASS | Go binary unchanged; Worker is a separate deploy artifact, not a Go build output. |
| II. Read-Only Inputs | PASS | Worker never sees pt-stalk input trees. Report itself still reads nothing at view-time. |
| III. Graceful Degradation | PASS | Every Worker failure mode falls back to the feature-002 `window.open` path. No feature-level panic. |
| IV. Deterministic Output | PASS | Worker/GitHub URLs and limits come from canonical build-time JSON in `render/assets/feedback-contract.json` (research R7, reconciled by feature 012). |
| V. Self-Contained HTML Reports | PASS (transitively, via Principle IX named exception) | The deployed report still embeds all assets (CSS/JS/fonts) inline. No new CDN, no view-time resource fetch except the Submit POST itself, which is the same call permitted by IX's named exception. V's "no remote resource at view-time" is a strict subset of IX's network rule, so the IX exception covers V transitively; no separate V-level exception is needed. The deployed Worker URL ships inside the embedded feedback contract and is the only cross-origin endpoint the report contacts. |
| VI. Library-First Architecture | PASS | The Worker URL is canonical build-time data in `render/assets/feedback-contract.json`. No new flag on `cmd/my-gather`. |
| VII. Typed Errors | PASS | No new Go error paths. Worker errors are JSON-typed, not Go-typed. |
| VIII. Reference Fixtures & Golden Tests | PASS | No new parser. Render tests assert the dialog embeds the canonical feedback contract. |
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
├── feedback.go                  # MODIFIED — embed canonical feedback contract in FeedbackView
├── feedback_test.go             # MODIFIED — assert canonical contract shape
├── templates/
│   └── report.html.tmpl         # MODIFIED — dialog gets data-feedback-contract attr
└── assets/
    ├── feedback-contract.json   # CANONICAL — endpoints, categories, and limits
    └── app.js                   # MODIFIED — doSubmit() uses the embedded contract with window.open fallback

feedback-worker/                 # Cloudflare Worker deploy artifact (live)
├── package.json
├── tsconfig.json
├── wrangler.toml                # concrete file — real KV/R2 IDs, nodejs_compat, observability enabled
├── src/
│   ├── index.ts                 # Main Worker entry
│   ├── env.ts                   # Env binding types
│   ├── github-app.ts            # JWT signing + installation-token exchange + GraphQL createIssue + repository.labels lookup
│   ├── idempotency.ts           # KV-backed two-state idempotency (inflight 60s / done 300s)
│   ├── log.ts                   # Structured-log helper
│   ├── r2-upload.ts             # Base64 decode + SHA-256 + R2 PUT
│   ├── ratelimit.ts             # KV per-request unique-key fixed UTC-hour window + local prefix lock
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

**Structure Decision**: Worker lives at repo root in `feedback-worker/` (sibling of `cmd/`, `render/`, etc.). Not as a Go subpackage because it's not Go. Not as a git submodule because the coupling to the Go side is tight enough to keep in-tree for atomic commits. As of feature 012, endpoints, categories, and validation limits share one canonical JSON contract at `render/assets/feedback-contract.json`.

### Files touched (exhaustive)

- `render/assets/feedback-contract.json`: canonical endpoints, categories, and limits used by Go, browser JS, and the Worker.
- `render/feedback.go`: `FeedbackView` carries `ContractJSON` and `Categories` derived from the canonical contract (no `ReportVersion` — `app.js` reads it from `<meta name="generator">` instead). See commit 636a332 for the original implementation and feature 012 for canonicalization.
- `render/feedback_test.go`: asserts the canonical feedback contract shape.
- `render/templates/report.html.tmpl`: dialog renders `data-feedback-contract`. Two regions present: `<form id="feedback-form">` and `<div id="feedback-success">`. Inline transients `#feedback-error` and `#feedback-fallback`.
- `render/assets/app.js`: `doSubmit` performs POST → success / fallback path using the embedded feedback contract; reads `reportVersion` from the `<meta name="generator">` tag.
- `feedback-worker/` (live deploy artifact; see commit 636a332 for the original implementation; this PR (003a) adds `AbortController` + 504 in `feedback-worker/src/{github-app,index,validate}.ts` per FR-014 — see commit 3 of this branch).
- `CLAUDE.md`: active-feature pointer.

(`.specify/memory/constitution.md` is NOT touched by this feature's
implementation PR — the Principle IX named exception was ratified
in a separate commit at v1.3.0 and is already on main.)

## Post-Design Re-check

*Filled after Phase 1 artifacts; verified in production after the 2026-04-24 deploy.*

- **Determinism preserved**: the feedback contract is embedded build-time data. The rendered HTML has been verified byte-identical across two builds in production; the `data-feedback-contract` attribute is stable, and smoke test issues #25 and #26 confirm the end-to-end Worker path.
- **Fallback path preserved**: the feature-002 `window.open` code in `doSubmit` is NOT removed. It is invoked when the Worker path fails and builds the fallback URL from the canonical contract's GitHub URL.
- **No secrets in shipped HTML**: only public feedback endpoints and validation metadata ship in the embedded contract. App ID, installation ID, repo ID, R2 prefix, and private key live in Cloudflare Worker Secrets.

Constitution Check passes pre and post, with the Principle IX amendment (already on main at v1.3.0) as the enabling change.
