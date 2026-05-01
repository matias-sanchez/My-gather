---

description: "Task list for Feedback Backend Worker implementation"
---

# Tasks: Feedback Backend Worker

**Input**: `specs/003-feedback-backend-worker/`
**Prerequisites**: spec.md, plan.md, research.md, data-model.md, contracts/api.md, quickstart.md

## Format: `[ID] [P?] [Story] Description`

---

## ✅ DONE — Phase 1: One-time setup (HUMAN — cannot be automated)

**All five tasks were completed before 2026-04-24; the App, labels, and Cloudflare resources are live. Re-do only when forking.**

- [x] T001 Create the `My-gather Feedback` GitHub App per quickstart 1.1. Record App ID + Installation ID + download the `.pem` private key.
- [x] T002 Install the App on the `matias-sanchez/My-gather` repo; confirm "Issues: Read and write" permission in the install summary.
- [x] T003 Create the six labels (`user-feedback`, `needs-triage`, `area/ui`, `area/parser`, `area/advisor`, `area/other`) in the repo per quickstart 1.2. Verify with `gh label list`.
- [x] T004 Sign up for Cloudflare (if no account); run `wrangler login` locally.
- [x] T005 Create the two KV namespaces and the R2 bucket per quickstart 1.3; record their IDs and the R2 public URL prefix.

**Checkpoint**: 6 values in hand (App ID, Installation ID, Repo node ID, 2 × KV IDs, R2 URL) + the `.pem` file locally + 6 labels created on the repo. All in place since 2026-04-24.

---

## ✅ DONE (commit 636a332) — Phase 2: Constitution check + repo scaffolding

**Purpose**: verify the constitutional precondition and land the empty Worker skeleton before any logic is written.

- [x] T006 Verify `.specify/memory/constitution.md` is at version **1.3.0** with Principle IX's "Named exceptions" subsection citing feature 003. The amendment was ratified 2026-04-24 and is already on main; this task became a redundant guard once v1.3.0 landed on main, but is preserved here as the historical gate.
- [x] T007 [P] Add `feedback-worker/` directory with `package.json`, `tsconfig.json`, `wrangler.toml` (concrete, not `.template`), `src/index.ts`, `vitest.config.ts`, and `.gitignore` excluding `node_modules/` and `.wrangler/`.
- [x] T008 [P] Update root `.gitignore` to include `feedback-worker/node_modules/` and `feedback-worker/.wrangler/`.
- [x] T009 Commit (shipped as 636a332). The constitution amendment itself already shipped in a prior commit on main at v1.3.0.

---

## ✅ DONE (commit 636a332) — Phase 3: Worker logic

**Purpose**: implement the Worker handler with validation, rate-limit, idempotency, GitHub App auth, and the GraphQL `createIssue` mutation. Exhaustive unit tests. The shipped Worker source inventory is 9 modules (`index.ts`, `env.ts`, `github-app.ts`, `idempotency.ts`, `log.ts`, `r2-upload.ts`, `ratelimit.ts`, `validate.ts`, `body.ts`) and 4 test files (`body.test.ts`, `github-app.test.ts`, `ratelimit.test.ts`, `validate.test.ts`).

- [x] T010 [P] [US1] Implement `feedback-worker/src/validate.ts`: payload schema validation per `contracts/api.md`. Returns typed errors including `report_version_invalid`. Voice cap is 15 728 640 bytes (15 MB).
- [x] T011 [P] [US3] Implement `feedback-worker/src/ratelimit.ts`: fixed UTC-hour window keyed `rl:<ipHash>:<UTC-hour>:<random>` (unique key per request, counted via `KV.list({prefix}).keys.length`). App ID is the salt. Returns `{allowed, retryAfterSeconds}`.
- [x] T012 [P] [US1] Implement `feedback-worker/src/idempotency.ts`: two-state reservation pattern under `idem:<uuid>` — `{status:"inflight"}` with 60 s TTL (Cloudflare KV minimum), upgraded to `{status:"done", issueUrl, issueNumber}` with 300 s TTL after success. No release helper (KV has no compare-and-delete).
- [x] T013 [P] [US1] Implement `feedback-worker/src/github-app.ts`: `jose`-based RS256 JWT signing (PKCS#8 `BEGIN PRIVATE KEY`), installation-token exchange (`POST /app/installations/{id}/access_tokens`), GraphQL `createIssue` mutation, and `repository.labels` paginated lookup (KV-cached 24 h). Returns `{issueUrl, issueNumber, issueId}` on success. The FR-014 `AbortController` 10 000 ms timeout per GitHub fetch (with `GitHubTimeoutError` → HTTP 504 `github_timeout` in `index.ts`) was wired up post-deploy in commit `b168a71` on this branch — see also Phase 7 / T041.
- [x] T014 [P] [US2] Implement `feedback-worker/src/r2-upload.ts`: decode base64 blob, compute SHA-256, PUT to R2 at `attachments/<sha256>.<ext>`, return public URL. Skip if blob already exists (by key).
- [x] T015 [US1] Implement `feedback-worker/src/index.ts` main handler composing all the above: parse JSON, validate, check rate-limit, check idempotency, upload attachments, build label list (`user-feedback` + `needs-triage` always, plus `area/<lower(category)>` if set), call GitHub `createIssue`, cache idempotency, return response. Body composition lives in `feedback-worker/src/body.ts` (category blockquote, `### Attached screenshot`, `### Attached voice note`, bare audio URL, footer).
- [x] T016 [US1] CORS (`POST, OPTIONS, GET`) + `OPTIONS /feedback` + `GET /health` endpoints per contract.
- [x] T017 [P] [US1] `feedback-worker/test/validate.test.ts`: exhaustive payload-validator tests (good + malformed + edge sizes).
- [x] T018 [P] [US3] `feedback-worker/test/ratelimit.test.ts`: simulated KV, 5 + 1 requests, verify 429 on 6th with `Retry-After`.
- [x] T019 [P] [US1] `feedback-worker/test/github-app.test.ts`: mock fetch, verify JWT is well-formed, token-exchange flow, GraphQL request body (`createIssue` mutation + `repository.labels` lookup, `labelIds` array including `user-feedback` + `needs-triage` plus the `area/*` mapping for each category enum value).
- [x] T020 [US1] `feedback-worker/test/body.test.ts` covers issue-body composition (category blockquote, screenshot section, voice section, footer). Note: no `e2e.test.ts` shipped in 636a332 — happy-path coverage lives across the four module tests above; smoke test issues #25 / #26 provide live e2e validation.

---

## ✅ DONE (commit 636a332, partial) — Phase 4: Browser-side client

**Purpose**: rewire `doSubmit` to call the Worker, handle all response paths, keep the feature-002 fallback. The deployed shape diverges from the original task list in two places — the planned `ReportVersion` Go field was not added; instead `app.js` reads `reportVersion` from the document's `<meta name="generator">` tag.

- [x] T021 [US1] Extend `FeedbackView` in `render/feedback.go`. **Deviation**: the deployed `FeedbackView` carries `WorkerURL`, `GitHubURL`, and `Categories`. `ReportVersion` was NOT added as a Go field — `app.js` reads `reportVersion` from `<meta name="generator">` instead, which keeps the dialog and the binary in lockstep without a new field.
- [x] T022 [US1] Add `data-feedback-worker-url` to the dialog. **Deviations**: the deployed template renders `data-feedback-worker-url` and the fallback URL `data-feedback-url`. `data-feedback-report-version` was NOT added (per the T021 deviation above — the version is read from the `<meta name="generator">` tag, not a dialog data-attr).
- [x] T023 [US1] In `render/assets/app.js` → `doSubmit`:
  - Builds JSON payload via the `buildPayload()` helper (base64-encode blobs via `FileReader.readAsDataURL`).
  - Mints `idempotencyKey` from `crypto.randomUUID()` once per Submit-click.
  - `fetch(workerURL, …)` with `AbortController` 15 s timeout.
  - On 200: renders success state (success message + link to `issueUrl`, label text `#<issueNumber>`). Does NOT auto-close.
  - On 429: shows throttle message with countdown from `retryAfterSeconds`. Does not fall back.
  - On 400: shows field-specific error; does not fall back.
  - On 5xx / network error / timeout: falls back to feature-002 `window.open` flow, with a small inline note "Backend unavailable — opened GitHub".
- [x] T024 [US1] Dialog regions in the deployed template: two regions (`<form id="feedback-form">` and `<div id="feedback-success">`) plus inline transient blocks `#feedback-error` and `#feedback-fallback`. The original task asked for `#feedback-submitting` and `#feedback-throttle` as separate regions; the deployed shape collapses transient states into inline blocks instead — see `contracts/ui.md` for the reconciled list.
- [x] T025 [P] [US1] `render/feedback_test.go`: asserts `WorkerURL` field is the documented constant.
- [x] T026 [P] [US1] `render/report_feedback_test.go`: asserts `data-feedback-worker-url` and `data-feedback-url` are rendered with their documented constants; asserts the deployed regions (`feedback-form`, `feedback-success`, `feedback-error`, `feedback-fallback`) per the reconciled `contracts/ui.md`. The `data-feedback-report-version` assertion from the original task is dropped per T021/T022 deviation.

---

## ✅ DONE (Worker URL https://my-gather-feedback.mati-orfeo.workers.dev/feedback) — Phase 5: Deploy + end-to-end

- [x] T027 `feedback-worker/wrangler.toml` ships as a concrete file (no `.template`) with the deployed KV / R2 IDs, the `nodejs_compat` flag, and `[observability] enabled = true`. No `[vars]` block — label names live in `feedback-worker/src/index.ts`.
- [x] T028 `wrangler secret put` ran for the 5 secrets actually needed by the deployed Worker: `GITHUB_APP_ID`, `GITHUB_INSTALLATION_ID`, `GITHUB_APP_PRIVATE_KEY`, `GITHUB_REPO_ID` (GraphQL `createIssue` needs the repository node ID), `R2_PUBLIC_URL_PREFIX`. The original task's `GITHUB_REPO` / `FEEDBACK_LABEL` / `AREA_LABEL_PREFIX` `[vars]` were obsolete by deploy time.
- [x] T029 `cd feedback-worker && npm install && npm test && npm run deploy` — shipped as commit 636a332. Worker URL recorded. **Caveat**: the formal `wrangler tail --format=json` cpuTime sampling for the SC-004 gate has not been captured into a commit message; smoke test issues #25 and #26 indicate normal free-tier operation (no CPU-exhaustion errors) over several days. If a future change pushes cpuTime above 9 ms median, the research R9 mitigation ladder (KV-cached installation token, then paid tier) still applies.
- [x] T030 `render/feedback.go`'s `feedbackWorkerURL` constant set to `https://my-gather-feedback.mati-orfeo.workers.dev/feedback`. Goldens regenerated.
- [x] T031 The 5 Quickstart scenarios were walked manually in a browser before merging PR #29. The dialog state machine in `contracts/ui.md` (form / success / error / fallback transitions, focus moves, AbortController timeout, ARIA roles) was exercised in that walkthrough.

---

## ✅ DONE — Phase 6: Polish + ship (PR #29 merged)

- [x] T032 Full-suite sweep: `go vet`, `go test ./...`, `cd feedback-worker && npm test`. Secret-leak check on the built binary per SC-005 ran clean.
- [x] T033 `@agent-pre-review-constitution-guard` returned READY TO PUSH (the Principle IX named exception at v1.3.0 makes the network call compliant).
- [x] T034 Commits pushed across the agent-phase checkpoints. Worker shipped in commit 636a332 (2026-04-24).
- [x] T035 PR #29 opened with `/pr-review-trigger-my-gather`. Bots reviewed over 5 rounds; 27 findings closed via `/pr-review-fix-my-gather`. PR merged.

---

## Phase 7: Reconciliation gaps closed in PR #30 (this branch `003a-spec-reality-reconcile`)

- [x] T036 Reconcile `spec.md` against deployed reality (commit `fb0aba8`).
- [x] T037 Reconcile `data-model.md` against deployed reality (commit `fb0aba8`).
- [x] T038 Reconcile `contracts/api.md` against deployed reality (commit `fb0aba8`).
- [x] T039 Reconcile `contracts/ui.md` against deployed reality (commit `fb0aba8`).
- [x] T040 Reconcile `research.md`, `plan.md`, `quickstart.md`, `tasks.md`, `checklists/requirements.md` against deployed reality (this commit).
- [x] T041 (commit `b168a71`) Add `AbortController` 10 000 ms timeout to all GitHub fetches in `feedback-worker/src/github-app.ts` via a `fetchWithTimeout` helper. On timeout throw `GitHubTimeoutError` (sibling — not subclass — of `GitHubError`); `feedback-worker/src/index.ts` catches it and returns HTTP 504 with `error: "github_timeout"` per FR-014 + contracts/api.md §504. Tests: `feedback-worker/test/github-app.test.ts` +2 cases (signal-passed-through + AbortError → GitHubTimeoutError conversion). `npm test` 43/43 PASS.
- [x] T042 PR #30 review cycle via `/pr-review-trigger-my-gather` and `/pr-review-fix-my-gather` until clean; merged as PR #30 on 2026-04-27.

---

## Phase 8: R2 attachment cleanup hardening

**Purpose**: keep the automatic Worker → Cloudflare R2 → GitHub Issue flow, while preventing public orphaned attachments when R2 upload succeeds but GitHub issue creation fails before an issue URL exists.

- [x] T043 [US2] Update `specs/003-feedback-backend-worker/{spec.md,data-model.md,research.md,contracts/api.md,tasks.md}` with the pre-create R2 cleanup contract and content-hash ownership rule.
- [x] T044 [P] [US2] Add `feedback-worker/test/index.test.ts` coverage for "new R2 object uploaded, GitHub pre-create failure, object deleted before returning 503".
- [x] T045 [P] [US2] Add `feedback-worker/test/index.test.ts` coverage for "pre-existing content-hash object reused, GitHub pre-create failure, object not deleted".
- [x] T046 [US2] Update `feedback-worker/src/r2-upload.ts` so `uploadAttachment()` returns `created`, and add a deletion helper for request-owned cleanup.
- [x] T047 [US2] Update `feedback-worker/src/index.ts` to track uploaded attachment metadata and run best-effort cleanup in every pre-create failure branch before returning 503/504/500.
- [x] T048 [US2] Run `cd feedback-worker && npm run typecheck && npm test`, then run root `go test ./...`, `make lint`, and `git diff --check`.

---

## Dependencies

- Phase 1 (human setup) → Phase 2 needs the IDs and the R2 URL. The constitution amendment is already on main at v1.3.0; T006 is a verification guard, not an edit.
- Phase 2 → Phase 3 (Worker can't be written without the scaffold).
- Phase 3 can start in parallel once T007 has produced the scaffold. Agents can fork: one per module (validate, ratelimit, idempotency, github-app, r2, index).
- Phase 4 does NOT depend on Phase 3 being done — the client change can proceed in parallel using a stub Worker URL. Agents can fork Go-side and Worker-side in parallel.
- Phase 5 (deploy) depends on Phase 3 (Worker working) AND Phase 4 (client expecting the Worker).
- Phase 6 depends on 5.

## Parallel opportunities

- **Within Phase 3**: T010–T014 (validate, ratelimit, idempotency, github-app, r2) each in its own file; agent-parallel safe. Test files likewise.
- **Phase 4 concurrent with Phase 3**: different agents, different files, different languages.

## Implementation strategy

### MVP: T001–T020 + T021–T026 text-only

Ship text-only first: skip R2 and attachment uploading in the Worker (T014), skip image/voice base64 in the client payload. Once the "Submit → success" flow is proven, a second iteration adds attachments (T014 + client work). Halves the surface of the first PR and lets the maintainer review the wiring without the attachment plumbing in the same diff.

### Full feature in one PR (if preferred)

All phases, one PR, ~1 week of work including the human-setup handoff.
