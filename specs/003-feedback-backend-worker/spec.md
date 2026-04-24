# Feature Specification: Feedback Backend Worker

**Feature Branch**: `iterations_2304b` (continues current branch; cut a feature branch at implement-time if we want review isolation)
**Created**: 2026-04-24
**Status**: Draft
**Input**: User description: "Cuando el user hace Submit en el report-feedback dialog, la discussion se tiene que registrar sola en GitHub Discussions del repo matias-sanchez/My-gather (categoría Ideas), sin que el user tenga que pegar nada ni clickear más. El user ve un success inline y listo. Image y voice tienen que quedar también en la discussion. No se puede shipping un token de GitHub en el HTML (cualquier lector lo extrae)."

## Clarifications (2026-04-24)

Resolved before starting this spec via interactive discussion:

- **Transport**: the report POSTs JSON to a Cloudflare Worker controlled by the project. The Worker holds a GitHub App credential and calls GitHub's GraphQL API to create the discussion. The token never reaches the browser.
- **Backend**: Cloudflare Workers (free tier). Chosen over Lambda/Vercel/others because: (a) no cold-start at the scale expected, (b) no credit card required, (c) Durable Objects / KV provide simple rate-limiting primitives, (d) familiar TypeScript surface.
- **GitHub credential**: a GitHub App installed in `matias-sanchez/My-gather` with `Discussions: write` permission. Preferred over a PAT because: (a) Apps can be revoked per-installation, (b) tokens are short-lived (1h) and generated per-request, (c) audit trail on GitHub shows the App author on each discussion.
- **Attachments**: image + voice inline in the discussion body via GitHub's markdown (images as base64 data URIs are rejected by GitHub; we upload via the GraphQL `uploadSignedAssets`-style endpoint OR upload to a Worker-managed R2 bucket and link). The exact path is a research question — see `research.md` R2.
- **Fallback when Worker is down**: the report silently falls back to the current `window.open` pre-fill URL flow from feature 002. No user-visible error states about "backend unavailable" — just a degraded path that still delivers value.
- **Constitution impact**: introduces runtime network from the report. Requires an amendment to Principle IX (Zero Network at Runtime) documenting a named exception. Amendment MUST land in the same change; it is not a deferred follow-up.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Submit posts the discussion without user post-action (Priority: P1)

A support engineer opens the rendered report, clicks "Report feedback", types title + body, hits Submit. The dialog replaces its form with a success state: "Feedback posted · View on GitHub [link]". The engineer closes the dialog and returns to triage. No extra click, no new tab to visit, no paste into GitHub.

**Why this priority**: this is the entire reason this feature exists. Feature 002 left the user with one extra step (paste text into GitHub's pre-filled page) and that step turned out to be enough friction for the feature to feel broken. P1 is zero-step submit.

**Independent Test**: Open a rendered report with network on. Click "Report feedback". Fill title "Log-scale toggle for IOPS chart" and body "Would help compare idle vs peak." Click Submit. Within 3 seconds, the dialog shows success inline with a link to the created discussion. Click the link, verify the discussion exists on github.com/matias-sanchez/My-gather/discussions/... with the title and body posted.

**Acceptance Scenarios**:

1. **Given** network on and Worker up, **When** Submit is clicked with a valid title + body, **Then** dialog shows success state within 3s with a link to the new discussion on GitHub.
2. **Given** network on and Worker returns 500, **When** Submit is clicked, **Then** dialog falls back to the feature-002 `window.open` flow (GitHub pre-fill URL) and shows a small inline note "Backend unavailable — opened GitHub with pre-filled form".
3. **Given** network off at Submit-click time, **When** Submit is clicked, **Then** dialog falls back to `window.open` (the browser still tries to open; if blocked, the feature-002 popup-blocker fallback kicks in).

---

### User Story 2 — Image and voice attachments land in the discussion body (Priority: P2)

A support engineer pastes a screenshot (image on clipboard) and records a 10-second voice note. Submit. The resulting discussion on GitHub contains both: the image rendered inline, the voice note embedded as an `<audio>` the discussion reader can play without downloading.

**Why this priority**: text-only Submit already delivers real value (P1). Attachments are the "much better" that justifies building this backend.

**Independent Test**: Open the dialog, paste an image via Cmd/Ctrl+V, record a voice note, hit Submit. Open the resulting discussion. Verify the image renders inline and the audio player is visible and playable.

**Acceptance Scenarios**:

1. **Given** an image is attached, **When** Submit succeeds, **Then** the discussion body on GitHub displays the image inline (as a `![](url)` markdown, hosted on GitHub's CDN or the Worker's R2).
2. **Given** a voice note is attached, **When** Submit succeeds, **Then** the discussion body contains an embedded audio player or a link that renders as one, playable in-page on GitHub.
3. **Given** both attachments are present, **When** Submit succeeds, **Then** the discussion body contains, in order: the category marker line (if chosen), the user's body text, the image, the audio.

---

### User Story 3 — Rate limiting prevents abuse (Priority: P2)

The Worker enforces a rate limit: maximum 5 Submit requests per IP address per hour. A sixth request within the hour returns HTTP 429 with a `Retry-After` header. The dialog shows "Rate limit reached — try again in N minutes" and keeps the draft intact.

**Why this priority**: without rate-limit the Worker is an open relay to create GitHub discussions as the App's identity. Any attacker who scrapes the Worker URL from a published report can spam the repo.

**Independent Test**: Using a curl loop, hit `POST /feedback` 10 times in a row from the same IP. Requests 1–5 succeed (HTTP 200). Requests 6–10 return HTTP 429 with a `Retry-After` header ≤ 3600 seconds.

**Acceptance Scenarios**:

1. **Given** the same IP has submitted 5 times in the last hour, **When** a sixth Submit arrives, **Then** the Worker returns HTTP 429 with `Retry-After` header and the dialog shows the throttle message.
2. **Given** an hour has passed since the oldest request in the rate-limit window, **When** a new Submit arrives, **Then** it is accepted (sliding or fixed-window — see research R3).

---

### User Story 4 — Spam/abuse content is rejected at the Worker (Priority: P3)

The Worker validates every incoming payload: title non-empty (≤ 200 chars), body ≤ 10 KB of text, image ≤ 5 MB, voice ≤ 10 MB. Over-limit or malformed requests return HTTP 400 with a structured error. A basic profanity/blocklist filter rejects obvious spam payloads.

**Why this priority**: enhances robustness but isn't core to the user story.

**Independent Test**: POST oversized payloads (title of 1000 chars, body of 50 KB, image of 20 MB). Each returns HTTP 400 with a clear error. Dialog surfaces the error inline next to the offending field.

**Acceptance Scenarios**:

1. **Given** a payload with title > 200 chars, **When** submitted, **Then** Worker returns 400 with `{"error": "title_too_long"}` and dialog highlights the title field.
2. **Given** a payload with image > 5 MB, **When** submitted, **Then** Worker returns 400 with `{"error": "image_too_large"}` and dialog clears the image attachment with a visible note.

---

### Edge Cases

- **Cold-start latency**: first request after Worker idle may take 100–300ms. Dialog shows a spinner for up to 5s before declaring timeout.
- **User closes dialog mid-submit**: request continues in the background (the Worker doesn't know the client is gone). If it succeeds, the discussion is still posted. Document this — it's a minor duplicate risk if the user re-submits identical content.
- **Rate-limit cache eviction**: Cloudflare KV is eventually consistent (~1 min to propagate globally). Under extreme burst, a user could briefly exceed the limit. Accept.
- **GitHub App installation removed**: `POST /feedback` returns 503 with a clear error. Dialog falls back to the feature-002 pre-fill URL path.
- **GitHub API rate-limit hit by the App**: 5000 req/hr per installation. We are many orders of magnitude under that. If exceeded, fallback.
- **Multiple simultaneous Submits**: each gets its own Worker invocation. No ordering guarantees on GitHub. Idempotency is not guaranteed (a network retry could create a duplicate discussion). See data-model for the idempotency-key approach.
- **Report opened from `file://`**: CORS treats file:// origin as `null`. Worker must accept `Origin: null` (or lack of Origin header) for the same-binary scenario.
- **Old reports (pre-Worker)**: old reports keep calling `window.open` — they don't know the Worker exists. No regression.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The report MUST POST a structured JSON payload to a Worker endpoint on Submit. The endpoint URL MUST be embedded in the generated HTML at build time (not fetched at runtime).
- **FR-002**: The payload MUST include `{title, body, category?, image?, voice?, idempotencyKey}`. `image` and `voice` are base64-encoded blobs with MIME type hints.
- **FR-003**: The Worker MUST authenticate as a GitHub App (JWT → installation access token) and call GitHub's `createDiscussion` GraphQL mutation with the user's payload.
- **FR-004**: The Worker MUST NOT persist the payload beyond the lifetime of the request (no logs of user text beyond standard rate-limit counters; no storage in KV except rate-limit metadata).
- **FR-005**: Attachments (image, voice) MUST be uploaded to a storage Koordinate durable enough to survive the discussion's lifetime. Options: (a) GitHub's user-content upload endpoint used by web issue form, (b) Cloudflare R2 bucket with public read URL. Research task R2 picks between them; FR-005 is agnostic to the choice.
- **FR-006**: On success, the Worker MUST return `{ok: true, discussionUrl: "https://github.com/..."}`. On failure, `{ok: false, error: "<code>", message: "<human-readable>"}`.
- **FR-007**: The dialog MUST display a success state inline on `ok: true` response, containing the discussion URL as a visible clickable link. It MUST NOT auto-close the dialog — the user dismisses it.
- **FR-008**: On any non-OK response from the Worker (500, 503, network error, timeout > 5s), the dialog MUST fall back to the feature-002 `window.open` flow with the same pre-fill URL, and surface a small neutral note about the fallback.
- **FR-009**: The Worker MUST rate-limit by IP at 5 requests per rolling hour. A sixth request returns HTTP 429 with `Retry-After` header. Implementation via Cloudflare KV with TTL.
- **FR-010**: The Worker MUST validate payload limits: title ≤ 200 chars, body ≤ 10 KB UTF-8, image ≤ 5 MB, voice ≤ 10 MB. Violations return HTTP 400.
- **FR-011**: CORS: the Worker MUST respond with `Access-Control-Allow-Origin: *` (or allowlist), `Access-Control-Allow-Methods: POST, OPTIONS`, and handle `OPTIONS` preflight. Reports opened from `file://` have origin `null` — the Worker MUST accept this.
- **FR-012**: The Worker MUST include an idempotency check: requests with the same `idempotencyKey` within 5 minutes that produced a successful discussion return the cached `{ok: true, discussionUrl}` response instead of creating a duplicate.
- **FR-013**: The report's dialog JS MUST generate `idempotencyKey` as `crypto.randomUUID()` at Submit-click time and reuse it on retry.
- **FR-014**: The Worker MUST set a reasonable request timeout on its GitHub API calls (≤ 10s). If GitHub is slow, the Worker returns 504 and the dialog falls back to `window.open`.
- **FR-015**: The generated report's HTML MUST NOT contain any secret credential. The Worker URL is public (anyone can POST to it — protected by rate-limit + validation).
- **FR-016**: The constitutional amendment (Principle IX exception) MUST be part of the change set that lands this feature. Merging the Worker without the amendment is prohibited.

### Key Entities

- **Feedback payload**: in-transit JSON `{title, body, category, image, voice, idempotencyKey, reportVersion}`. Never persisted.
- **Rate-limit record**: KV entry `ratelimit:<ip>:<hour-window>` → count. TTL 1 hour. Never contains user content.
- **Idempotency record**: KV entry `idemp:<key>` → `{discussionUrl}`. TTL 5 minutes. Contains only the resulting URL, not the original payload.
- **GitHub App credentials**: App ID, Installation ID, and Private Key stored as Cloudflare Worker Secrets. Never in code, never in logs.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 95% of successful Submits complete within 3 seconds end-to-end (click to success-state-visible) on a broadband connection.
- **SC-002**: With Worker down, 100% of Submits fall back to `window.open` cleanly (no error UI confusing the user; no data loss).
- **SC-003**: Rate-limit rejection rate under normal usage (5 submits per user per hour budget) is < 1%. Under a simulated attack (100 submits/min from same IP), 100% of requests above the limit return HTTP 429 within 50ms.
- **SC-004**: Monthly Cloudflare cost: $0. Monthly GitHub cost: $0. If either exceeds $0, alarm surfaces in the dashboard.
- **SC-005**: Zero secrets in the shipped binary or generated HTML. Verified by grep of the release artifact for known patterns (`ghp_`, `github_pat_`, `ghs_`, PEM markers).
- **SC-006**: First-click adoption: within the first month after ship, at least 50% of Submits succeed via the Worker path (not the fallback). Measures whether the feature is working in practice.

## Assumptions

- User has Cloudflare account and can deploy Workers. If not, a one-time setup is needed (Phase 1 of the implementation plan).
- User controls `matias-sanchez/My-gather` GitHub org/repo and can create + install a GitHub App there.
- The project's Discussions feature remains enabled with an "Ideas" category.
- GitHub's `createDiscussion` GraphQL mutation remains stable (documented, not beta).
- GitHub's rate-limit budget of 5000 requests/hour per installation is far above expected traffic. If exceeded, fallback is acceptable.
- Cloudflare Workers free tier (100k req/day, 10ms CPU) is sufficient for the projected traffic (< 100 req/day).
- Corporate networks hosting the report viewers allow outbound HTTPS to `*.workers.dev`. If blocked, fallback flow still works via `window.open` to github.com (usually allowed).
