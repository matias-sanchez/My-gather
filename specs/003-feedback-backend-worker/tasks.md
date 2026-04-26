---

description: "Task list for Feedback Backend Worker implementation"
---

# Tasks: Feedback Backend Worker

**Input**: `specs/003-feedback-backend-worker/`
**Prerequisites**: spec.md, plan.md, research.md, data-model.md, contracts/api.md, quickstart.md

## Format: `[ID] [P?] [Story] Description`

---

## Phase 1: One-time setup (HUMAN — cannot be automated)

**These five items require the user's hands on external services. Total time: ~15 minutes.**

- [ ] T001 Create the `My-gather Feedback` GitHub App per quickstart 1.1. Record App ID + Installation ID + download the `.pem` private key.
- [ ] T002 Install the App on the `matias-sanchez/My-gather` repo; confirm "Issues: Read and write" permission in the install summary.
- [ ] T003 Create the five labels (`feedback`, `area:ui`, `area:parser`, `area:advisor`, `area:other`) in the repo per quickstart 1.2. Verify with `gh label list`.
- [ ] T004 Sign up for Cloudflare (if no account); run `wrangler login` locally.
- [ ] T005 Create the two KV namespaces and the R2 bucket per quickstart 1.3; record their IDs and the R2 public URL prefix.

**Checkpoint**: 5 values in hand (App ID, Installation ID, 2 × KV IDs, R2 URL) + the `.pem` file locally + 5 labels created on the repo. Agent work below does not start until these exist.

---

## Phase 2: Constitution check + repo scaffolding

**Purpose**: verify the constitutional precondition and land the empty Worker skeleton before any logic is written.

- [ ] T006 Verify `.specify/memory/constitution.md` is at version **1.3.0** with Principle IX's "Named exceptions" subsection citing feature 003. The amendment was ratified 2026-04-24 and is already on main; this task is a guard, not an edit. Fail loudly if the constitution version has regressed.
- [ ] T007 [P] Add `feedback-worker/` directory with `package.json`, `tsconfig.json`, `wrangler.toml.template`, an empty `src/index.ts` returning 501 Not Implemented, `vitest.config.ts`, and `.gitignore` excluding `node_modules/` and `.wrangler/`.
- [ ] T008 [P] Update root `.gitignore` to include `feedback-worker/node_modules/` and `feedback-worker/.wrangler/`.
- [ ] T009 Commit "feedback(003): worker scaffold + constitution check" as one commit. The constitution amendment itself already shipped in a prior commit on main at v1.3.0.

---

## Phase 3: Worker logic (agent-parallel opportunity)

**Purpose**: implement the Worker handler with validation, rate-limit, idempotency, GitHub App auth, and the REST call to `POST /repos/.../issues`. Exhaustive unit tests.

- [ ] T010 [P] [US1] Implement `feedback-worker/src/validate.ts`: payload schema validation per `contracts/api.md`. Return typed errors.
- [ ] T011 [P] [US3] Implement `feedback-worker/src/ratelimit.ts`: fixed-window KV counter. Returns `{allowed, retryAfterSeconds}`.
- [ ] T012 [P] [US1] Implement `feedback-worker/src/idempotency.ts`: check + record cached responses for a key.
- [ ] T013 [P] [US1] Implement `feedback-worker/src/github-app.ts`: `jose`-based RS256 JWT signing (local crypto, no fetch), installation-token exchange (`POST /app/installations/{id}/access_tokens`), and a typed `createIssue` REST caller (`POST /repos/:owner/:repo/issues` with `labels` array). Both GitHub fetches (token exchange + createIssue) MUST use `AbortController` with a per-call timeout of 10 000 ms (FR-014); on timeout, surface a typed error so the index handler returns HTTP 504. Returns `{issueUrl, issueNumber, issueId}` on success.
- [ ] T014 [P] [US2] Implement `feedback-worker/src/r2-upload.ts`: decode base64 blob, compute SHA-256, PUT to R2 at `attachments/<sha256>.<ext>`, return public URL. Skip if blob already exists (by key).
- [ ] T015 [US1] Implement `feedback-worker/src/index.ts` main handler composing all the above: parse JSON, validate, check rate-limit, check idempotency, upload attachments, build label list (`feedback` + `area:<lower(category)>` if set), call GitHub `createIssue`, cache idempotency, return response.
- [ ] T016 [US1] CORS + `OPTIONS /feedback` + `GET /health` endpoints per contract.
- [ ] T017 [P] [US1] `feedback-worker/test/validate.test.ts`: exhaustive payload-validator tests (good + malformed + edge sizes).
- [ ] T018 [P] [US3] `feedback-worker/test/ratelimit.test.ts`: simulated KV, 5 + 1 requests, verify 429 on 6th with `Retry-After`.
- [ ] T019 [P] [US1] `feedback-worker/test/github-app.test.ts`: mock fetch, verify JWT is well-formed, token-exchange flow, REST request body (path, headers, `labels` array including `feedback` plus the `area:*` mapping for each category enum value).
- [ ] T020 [US1] `feedback-worker/test/e2e.test.ts`: full happy-path with all mocks, asserts the response body + log shape.

---

## Phase 4: Browser-side client (sequential, single file)

**Purpose**: rewire `doSubmit` to call the Worker, handle all response paths, keep the feature-002 fallback.

- [ ] T021 [US1] Extend `FeedbackView` in `render/feedback.go` with two `string` fields: `WorkerURL` (populated from a new `feedbackWorkerURL` package-level constant — placeholder until Phase 5 deploy) and `ReportVersion` (populated from the same build-time version string the binary already prints in `--version`, so the dialog and the binary stay in lockstep). Add godoc on both per Principle VI. The `ReportVersion` field is what the dialog ships in the Submit payload (FR-002, contracts/ui.md `data-feedback-report-version`); no `window` globals required.
- [ ] T022 [US1] Add `data-feedback-worker-url="{{ .Feedback.WorkerURL }}"` and `data-feedback-report-version="{{ .Feedback.ReportVersion }}"` to the `<dialog>` element in `render/templates/report.html.tmpl`.
- [ ] T023 [US1] In `render/assets/app.js` → `doSubmit`:
  - Build JSON payload via a new `buildPayload()` helper (base64-encode blobs via `FileReader.readAsDataURL`).
  - Mint `idempotencyKey` from `crypto.randomUUID()` once per Submit-click.
  - `fetch(workerURL, {method:"POST", body: JSON.stringify(...), signal: AbortController.signal})` with 5s timeout.
  - On 200: render success state (success message + link to `issueUrl`, label text `#<issueNumber>`). Do NOT auto-close.
  - On 429: show throttle message with countdown from `retryAfterSeconds`. Do not fall back.
  - On 400: show field-specific error; do not fall back.
  - On 5xx / network error / timeout: fall back to feature-002 `window.open` flow, with a small inline note "Backend unavailable — opened GitHub".
- [ ] T024 [US1] Add the four extra state regions in the dialog's template (`#feedback-submitting`, `#feedback-success`, `#feedback-error`, `#feedback-throttle`, all `hidden` by default), per `contracts/ui.md`. Extend CSS for `.feedback-inline-note`, `.feedback-error-message`, `.feedback-countdown`, and the per-state accent borders.
- [ ] T025 [P] [US1] Update `render/feedback_test.go`: assert `WorkerURL` field is the documented constant.
- [ ] T026 [P] [US1] Update `render/report_feedback_test.go`: assert the dialog has BOTH `data-feedback-worker-url` AND `data-feedback-report-version` rendered with their documented constants (per `contracts/ui.md` test contract items 1 and 2); assert all five state regions from `contracts/ui.md` (`form`, `submitting`, `success`, `error`, `throttle`) exist with the correct `data-state` attribute; assert `form` is the only region rendered without `hidden` and the other four are `hidden` by default; assert the throttle heading carries `tabindex="-1"` (per the Accessibility contract).

---

## Phase 5: Deploy + end-to-end

- [ ] T027 Fill `feedback-worker/wrangler.toml` from the template using the IDs from Phase 1. Commit the concrete `wrangler.toml`.
- [ ] T028 `wrangler secret put` for each of the 4 secrets listed in data-model.md (`GITHUB_APP_ID`, `GITHUB_INSTALLATION_ID`, `GITHUB_APP_PRIVATE_KEY`, `R2_PUBLIC_URL_PREFIX`). The three non-secret config values (`GITHUB_REPO`, `FEEDBACK_LABEL`, `AREA_LABEL_PREFIX`) live in `wrangler.toml [vars]` and are committed in T027.
- [ ] T029 `cd feedback-worker && npm install && npm test && npm run deploy`. Record the Worker URL. Then `wrangler tail --format=json` while issuing 3 sample submits to capture `cpuTime` per request from the Tail logs; if the median exceeds 9 ms (margin under the 10 ms free-tier cap, see research R9 + spec.md Assumption), STOP — either move to the paid tier ($5/month) or implement the KV-cached installation-token mitigation in `github-app.ts` before shipping.
- [ ] T030 Update `render/feedback.go`'s `feedbackWorkerURL` constant to the real deployed URL. Rebuild. Regenerate goldens.
- [ ] T031 Walk all 5 Quickstart scenarios manually in a browser. Check each off. The dialog state machine documented in `contracts/ui.md` (form / submitting / success / error / throttle / fallback transitions, focus moves, AbortController timeout, ARIA roles) is exercised here — it has no headless-browser unit coverage in this repo, so this manual walkthrough is the regression net.

---

## Phase 6: Polish + ship

- [ ] T032 Full-suite sweep: `go vet`, `go test ./...`, `cd feedback-worker && npm test`. Then run a secret-leak check on the built binary per SC-005: `make build && grep -aE 'ghp_|github_pat_|ghs_|-----BEGIN [A-Z ]*PRIVATE KEY-----' bin/my-gather` MUST return zero matches. Fail the task if any of those patterns appear in the binary.
- [ ] T033 Run `@agent-pre-review-constitution-guard`. Expect READY TO PUSH (the Principle IX amendment makes the network call compliant).
- [ ] T034 Commit sequence: one commit per agent-phase checkpoint (Phase 2, 3, 4, 5, 6). Phase 1 is human-only and produces no commits. Push.
- [ ] T035 Open PR with `/pr-review-trigger-my-gather`. Address findings with `/pr-review-fix-my-gather`.

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
