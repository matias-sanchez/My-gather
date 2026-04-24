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
- [ ] T002 Install the App on the `matias-sanchez/My-gather` repo; confirm "Discussions: Read and write" permission in the install summary.
- [ ] T003 Fetch `GITHUB_REPO_ID` and `GITHUB_CATEGORY_ID` via the GraphQL query in quickstart 1.2; record both.
- [ ] T004 Sign up for Cloudflare (if no account); run `wrangler login` locally.
- [ ] T005 Create the two KV namespaces and the R2 bucket per quickstart 1.3; record their IDs and the R2 public URL prefix.

**Checkpoint**: 7 values in hand (App ID, Installation ID, Repo ID, Category ID, 2 × KV IDs, R2 URL) + the `.pem` file locally. Agent work below does not start until these exist.

---

## Phase 2: Constitution amendment + repo scaffolding

**Purpose**: land the constitutional change and empty Worker skeleton before any logic is written, so the amendment is reviewable in isolation.

- [ ] T006 Update `.specify/memory/constitution.md` per plan.md's amendment text: add "Named exceptions" section to Principle IX, bump version to 1.3.0, add the standard Sync Impact Report comment at the top. Re-run `grep` to confirm no other principle was changed.
- [ ] T007 [P] Add `feedback-worker/` directory with `package.json`, `tsconfig.json`, `wrangler.toml.template`, an empty `src/index.ts` returning 501 Not Implemented, `vitest.config.ts`, and `.gitignore` excluding `node_modules/` and `.wrangler/`.
- [ ] T008 [P] Update root `.gitignore` to include `feedback-worker/node_modules/` and `feedback-worker/.wrangler/`.
- [ ] T009 Commit "feedback: constitution amendment + worker scaffold" as one commit. This is the first artifact that lands the PR's claim.

---

## Phase 3: Worker logic (agent-parallel opportunity)

**Purpose**: implement the Worker handler with validation, rate-limit, idempotency, GitHub App auth, and the GraphQL call. Exhaustive unit tests.

- [ ] T010 [P] [US1] Implement `feedback-worker/src/validate.ts`: payload schema validation per `contracts/api.md`. Return typed errors.
- [ ] T011 [P] [US3] Implement `feedback-worker/src/ratelimit.ts`: fixed-window KV counter. Returns `{allowed, retryAfterSeconds}`.
- [ ] T012 [P] [US1] Implement `feedback-worker/src/idempotency.ts`: check + record cached responses for a key.
- [ ] T013 [P] [US1] Implement `feedback-worker/src/github-app.ts`: `jose`-based RS256 JWT signing, installation-token exchange, and a typed `createDiscussion` GraphQL caller.
- [ ] T014 [P] [US2] Implement `feedback-worker/src/r2-upload.ts`: decode base64 blob, compute SHA-256, PUT to R2 at `attachments/<sha256>.<ext>`, return public URL. Skip if blob already exists (by key).
- [ ] T015 [US1] Implement `feedback-worker/src/index.ts` main handler composing all the above: parse JSON, validate, check rate-limit, check idempotency, upload attachments, call GitHub, cache idempotency, return response.
- [ ] T016 [US1] CORS + `OPTIONS /feedback` + `GET /health` endpoints per contract.
- [ ] T017 [P] [US1] `feedback-worker/test/validate.test.ts`: exhaustive payload-validator tests (good + malformed + edge sizes).
- [ ] T018 [P] [US3] `feedback-worker/test/ratelimit.test.ts`: simulated KV, 5 + 1 requests, verify 429 on 6th with `Retry-After`.
- [ ] T019 [P] [US1] `feedback-worker/test/github-app.test.ts`: mock fetch, verify JWT is well-formed, token-exchange flow, GraphQL payload.
- [ ] T020 [US1] `feedback-worker/test/e2e.test.ts`: full happy-path with all mocks, asserts the response body + log shape.

---

## Phase 4: Browser-side client (sequential, single file)

**Purpose**: rewire `doSubmit` to call the Worker, handle all response paths, keep the feature-002 fallback.

- [ ] T021 [US1] Add `WorkerURL string` field to `FeedbackView` in `render/feedback.go`; populate from a new `feedbackWorkerURL` package-level constant (placeholder value until Phase 5 deploy). Add godoc per Principle VI.
- [ ] T022 [US1] Add `data-feedback-worker-url="{{ .Feedback.WorkerURL }}"` to the `<dialog>` element in `render/templates/report.html.tmpl`.
- [ ] T023 [US1] In `render/assets/app.js` → `doSubmit`:
  - Build JSON payload via a new `buildPayload()` helper (base64-encode blobs via `FileReader.readAsDataURL`).
  - Mint `idempotencyKey` from `crypto.randomUUID()` once per Submit-click.
  - `fetch(workerURL, {method:"POST", body: JSON.stringify(...), signal: AbortController.signal})` with 5s timeout.
  - On 200: render success state (success message + link to `discussionUrl`). Do NOT auto-close.
  - On 429: show throttle message with countdown from `retryAfterSeconds`. Do not fall back.
  - On 400: show field-specific error; do not fall back.
  - On 5xx / network error / timeout: fall back to feature-002 `window.open` flow, with a small inline note "Backend unavailable — opened GitHub".
- [ ] T024 [US1] Add a new success-state markup region in the dialog's template (hidden by default, `id="feedback-success"`), per contracts/ui.md. Extend CSS for `.feedback-success` + green accent.
- [ ] T025 [P] [US1] Update `render/feedback_test.go`: assert `WorkerURL` field is the documented constant.
- [ ] T026 [P] [US1] Update `render/report_feedback_test.go`: assert the dialog has `data-feedback-worker-url` attribute rendered with the expected value; assert the success region exists.

---

## Phase 5: Deploy + end-to-end

- [ ] T027 Fill `feedback-worker/wrangler.toml` from the template using the IDs from Phase 1. Commit the concrete `wrangler.toml`.
- [ ] T028 `wrangler secret put` for each of the 6 secrets listed in data-model.md.
- [ ] T029 `cd feedback-worker && npm install && npm test && npm run deploy`. Record the Worker URL.
- [ ] T030 Update `render/feedback.go`'s `feedbackWorkerURL` constant to the real deployed URL. Rebuild. Regenerate goldens.
- [ ] T031 Walk all 5 Quickstart scenarios manually in a browser. Check each off.

---

## Phase 6: Polish + ship

- [ ] T032 Full-suite sweep: `go vet`, `go test ./...`, `cd feedback-worker && npm test`.
- [ ] T033 Run `@agent-pre-review-constitution-guard`. Expect READY TO PUSH (the Principle IX amendment makes the network call compliant).
- [ ] T034 Commit sequence: one commit per phase checkpoint. Push.
- [ ] T035 Open PR with `/pr-review-trigger-my-gather`. Address findings with `/pr-review-fix-my-gather`.

---

## Dependencies

- Phase 1 (human setup) → Phase 2 needs the IDs to fill the constitutional amendment's citation.
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

Ship text-only first: skip R2 and attachment uploading in the Worker (T014), skip image/voice base64 in the client payload. Once the "Submit → success" flow is proven, a second iteration adds attachments (T014 + client work). Halves the surface of the first PR and gets the constitutional amendment in sooner.

### Full feature in one PR (if preferred)

All phases, one PR, ~1 week of work including the human-setup handoff.
