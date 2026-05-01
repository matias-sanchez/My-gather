# Feature Specification: Feedback Backend Worker

**Feature Branch**: `003-feedback-backend-worker`
**Created**: 2026-04-24
**Status**: Complete
**Input**: User description: "Cuando el user hace Submit en el report-feedback dialog, la discussion se tiene que registrar sola en GitHub Discussions del repo matias-sanchez/My-gather (categoría Ideas), sin que el user tenga que pegar nada ni clickear más. El user ve un success inline y listo. Image y voice tienen que quedar también en la discussion. No se puede shipping un token de GitHub en el HTML (cualquier lector lo extrae)." *(Verbatim user input. Pivoted to Issues during 2026-04-26 clarification — see below.)*

## Clarifications (2026-04-24)

Resolved before starting this spec via interactive conversation:

- **Transport**: the report POSTs JSON to a Cloudflare Worker controlled by the project. The Worker holds a GitHub App credential and calls GitHub's REST API to create the issue. The token never reaches the browser.
- **Backend**: Cloudflare Workers (free tier). Chosen over Lambda/Vercel/others because: (a) no cold-start at the scale expected, (b) no credit card required, (c) Durable Objects / KV provide simple rate-limiting primitives, (d) familiar TypeScript surface.
- **GitHub credential**: a GitHub App installed in `matias-sanchez/My-gather` with `Issues: write` permission. Preferred over a PAT because: (a) Apps can be revoked per-installation, (b) tokens are short-lived (1h) and generated per-request, (c) audit trail on GitHub shows the App author on each issue.
- **Attachments**: image + voice inline in the issue body via GitHub's markdown (images as base64 data URIs are rejected by GitHub; we upload to a Worker-managed R2 bucket and link). See `research.md` R2 for the rejected alternatives.
- **Fallback when Worker is down**: the report silently falls back to the current `window.open` pre-fill URL flow from feature 002. No user-visible error states about "backend unavailable" — just a degraded path that still delivers value.
- **Constitution impact**: introduces runtime network from the report. The Principle IX named exception that authorises this was ratified in constitution v1.3.0 (2026-04-24, already on main) — the implementation PR only needs to verify the constitution is at v1.3.0, not amend it.

## Clarifications (2026-04-26)

- **Issues, not Discussions**: pivoted the original input ("GitHub Discussions, Ideas category" — the verbatim Spanish wording is preserved on `spec.md:6`) to GitHub **Issues** with two unconditional labels (`user-feedback`, `needs-triage`) plus an optional `area/<category>` label when the user picked one. Rationale: (a) Issues use the same triage UX the maintainer already runs; (b) `is:issue label:user-feedback` filters the bot-submitted bucket cleanly while `needs-triage` keeps everything visible in the unprocessed queue until reviewed; (c) the App's blast radius is one permission scope (`Issues: write`) rather than `Discussions: write`. The `category` field on the payload (`UI | Parser | Advisor | Other`) maps to lowercase labels `area/ui` / `area/parser` / `area/advisor` / `area/other` (slash separator, not colon — matches GitHub's de-facto convention). The repo MUST have these labels created once before deploy: `user-feedback`, `needs-triage`, plus the four `area/*` — see quickstart Step 1.2.
- **Branch name**: feature work lives on `003-feedback-backend-worker`, conforming to Spec Kit's `NNN-slug` convention. The original draft cited the prior working branch and is corrected here.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Submit posts the issue without user post-action (Priority: P1)

A support engineer opens the rendered report, clicks "Report feedback", types title + body, hits Submit. The dialog replaces its form with a success state: "Feedback posted · View on GitHub [link]". The engineer closes the dialog and returns to triage. No extra click, no new tab to visit, no paste into GitHub.

**Why this priority**: this is the entire reason this feature exists. Feature 002 left the user with one extra step (paste text into GitHub's pre-filled page) and that step turned out to be enough friction for the feature to feel broken. P1 is zero-step submit.

**Independent Test**: Open a rendered report with network on. Click "Report feedback". Fill title "Log-scale toggle for IOPS chart" and body "Would help compare idle vs peak." Click Submit. Within 3 seconds, the dialog shows success inline with a link to the created issue. Click the link, verify the issue exists on github.com/matias-sanchez/My-gather/issues/... with the title and body posted.

**Acceptance Scenarios**:

1. **Given** network on and Worker up, **When** Submit is clicked with a valid title + body, **Then** dialog shows success state within 3s with a link to the new issue on GitHub.
2. **Given** network on and Worker returns 500, **When** Submit is clicked, **Then** dialog falls back to the feature-002 `window.open` flow (GitHub pre-fill URL) and shows a small inline note "Backend unavailable — opened GitHub with pre-filled form".
3. **Given** network off at Submit-click time, **When** Submit is clicked, **Then** dialog falls back to `window.open` (the browser still tries to open; if blocked, the feature-002 popup-blocker fallback kicks in).

---

### User Story 2 — Image and voice attachments land in the issue body (Priority: P2)

A support engineer pastes a screenshot (image on clipboard) and records a 10-second voice note. Submit. The resulting issue on GitHub contains both: the image rendered inline, the voice note as a clearly-labelled clickable link to the audio file.

**Why this priority**: text-only Submit already delivers real value (P1). Attachments are the "much better" that justifies building this backend.

**Independent Test**: Open the dialog, paste an image via Cmd/Ctrl+V, record a voice note, hit Submit. Open the resulting issue. Verify the image renders inline (under an `### Attached screenshot` heading) and the voice section shows the bare R2 URL on its own line (under an `### Attached voice note` heading) — GitHub auto-renders such audio URLs as inline players, see `feedback-worker/src/body.ts` and `data-model.md` body composition.

**Acceptance Scenarios**:

1. **Given** an image is attached, **When** Submit succeeds, **Then** the issue body on GitHub displays the image inline (as a `![](url)` markdown, hosted on the Worker's R2 bucket per FR-005).
2. **Given** a voice note is attached, **When** Submit succeeds, **Then** the issue body contains a `### Attached voice note` section with the audio URL on its own line. GitHub auto-renders bare audio URLs as inline players, so the reader sees a playable widget without leaving the issue page; if a particular GitHub UI surface does NOT auto-render the URL (mobile, RSS feed, third-party clients), the URL is still a clickable link to the audio file.
3. **Given** both attachments are present, **When** Submit succeeds, **Then** the issue body contains, in order: an optional `> Category: <X>` blockquote (when set), the user's body text, an `### Attached screenshot` section with the image markdown, an `### Attached voice note` section with the audio URL on its own line (GitHub renders bare audio URLs as inline players), and a footer attribution line. The `category` field also becomes an `area/<lower>` label on the issue alongside the always-applied `user-feedback` and `needs-triage` labels.
4. **Given** an attachment upload succeeds but the GitHub issue is not created, **When** the Worker returns a pre-create failure response, **Then** any R2 object newly created by that failed request is deleted before the response is returned; an R2 object that already existed before the request is never deleted.

---

### User Story 3 — Rate limiting prevents abuse (Priority: P2)

The Worker enforces a rate limit: maximum 5 Submit requests per IP address per hour. A sixth request within the hour returns HTTP 429 with a `Retry-After` header. The dialog shows "Rate limit reached — try again in N minutes" and keeps the draft intact.

**Why this priority**: without rate-limit the Worker is an open relay to create GitHub issues as the App's identity. Any attacker who scrapes the Worker URL from a published report can spam the repo.

**Independent Test**: Using a curl loop, hit `POST /feedback` 10 times in a row from the same IP. Requests 1–5 succeed (HTTP 200). Requests 6–10 return HTTP 429 with a `Retry-After` header ≤ 3600 seconds.

**Acceptance Scenarios**:

1. **Given** the same IP has submitted 5 times in the current UTC-hour window, **When** a sixth Submit arrives in that same window, **Then** the Worker returns HTTP 429 with `Retry-After` header and the dialog shows the throttle message.
2. **Given** the UTC-hour boundary has rolled over since the previous window's submissions, **When** a new Submit arrives in the new window, **Then** it is accepted (fixed-window per research R3).

---

### User Story 4 — Oversized or malformed payloads are rejected at the Worker (Priority: P3)

The Worker validates every incoming payload: title non-empty (≤ 200 chars), body ≤ 10 KB of text, image ≤ 5 MB, voice ≤ 15 MB (decoded — the 15 MB cap fits ~10 min of Opus at the browser's default bitrate with margin; matches `feedback-worker/src/validate.ts` `VOICE_MAX_BYTES = 15_728_640`). Over-limit or malformed requests return HTTP 400 with a structured error. (Spam/abuse content per se is not filtered at the Worker — the rate limit + the GitHub App's auditable identity carry the abuse story; a profanity filter would be a separate, opt-in feature with its own spec.)

**Why this priority**: enhances robustness but isn't core to the user story.

**Independent Test**: POST oversized payloads (title of 1000 chars, body of 50 KB, image of 20 MB). Each returns HTTP 400 with a clear error. Dialog surfaces the error inline next to the offending field.

**Acceptance Scenarios**:

1. **Given** a payload with title > 200 chars, **When** submitted, **Then** Worker returns 400 with `{"error": "title_too_long"}` and dialog highlights the title field.
2. **Given** a payload with image > 5 MB, **When** submitted, **Then** Worker returns 400 with `{"error": "image_too_large"}` and dialog clears the image attachment with a visible note.

---

### Edge Cases

- **First-request total latency**: typical end-to-end is ~300ms p95 (Worker isolate boot <5ms, JWT signing ~20ms, GitHub GraphQL call ~200ms — per research R9). Dialog shows a spinner for up to 15s before the browser-side AbortController fires (FR-008); the Worker's own GitHub fetches are individually bounded by a 10s AbortController per FR-014.
- **User closes dialog mid-submit**: request continues in the background (the Worker doesn't know the client is gone). If it succeeds, the issue is still posted. Document this — it's a minor duplicate risk if the user re-submits identical content.
- **Rate-limit cache eviction**: Cloudflare KV is eventually consistent (~1 min to propagate globally). Under extreme burst, a user could briefly exceed the limit. Accept.
- **GitHub App installation removed**: `POST /feedback` returns 503 with a clear error. Dialog falls back to the feature-002 pre-fill URL path.
- **GitHub API rate-limit hit by the App**: 5000 req/hr per installation. We are many orders of magnitude under that. If exceeded, fallback.
- **Multiple simultaneous Submits**: each gets its own Worker invocation. No ordering guarantees on GitHub. Idempotency is not guaranteed (a network retry could create a duplicate issue). See data-model for the idempotency-key approach.
- **Browser timeout vs Worker timeout asymmetry**: the browser fetch aborts at 15s (FR-008), the Worker's per-GitHub-call timeout is 10s (FR-014). The 15s browser budget brackets the 10s Worker timeout (with ~5s buffer for KV writes, R2 uploads, and the final response), so a slow GitHub call surfaces as a clean Worker-side 504 within the browser's window — the dialog falls back to `window.open` based on the 504, not on a browser timeout. A genuine browser-side 15s timeout only fires if the Worker itself is unreachable or stuck; in that case the `window.open` fallback may post a duplicate after a slow Worker eventually creates the issue too, but the idempotency-key replay (5-minute KV cache) catches sequential same-key retries on the Worker side so only the very first racing pair of calls can produce the duplicate.
- **Report opened from `file://`**: CORS treats file:// origin as `null`. Worker must accept `Origin: null` (or lack of Origin header) for the same-binary scenario.
- **Old reports (pre-Worker)**: old reports keep calling `window.open` — they don't know the Worker exists. No regression.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The report MUST POST a structured JSON payload to a Worker endpoint on Submit. The endpoint URL MUST be embedded in the generated HTML at build time (not fetched at runtime).
- **FR-002**: The payload MUST include `{title, body, category?, image?, voice?, idempotencyKey, reportVersion}`. `image` and `voice` are base64-encoded blobs with MIME type hints. `reportVersion` is an opaque string (≤ 64 chars), a build-time constant from the my-gather binary that produced the report; it is used only for observability + the issue body footer, never for authentication or authorisation.
- **FR-003**: The Worker MUST authenticate as a GitHub App (JWT → installation access token) and call GitHub's GraphQL `createIssue` mutation with the user's payload, applying the `user-feedback` and `needs-triage` labels unconditionally and (if `category` is set) the matching `area/<lower(category)>` label. Labels are referenced by their GraphQL node IDs, resolved once via a paginated `repository.labels` query and cached in KV for 24h to amortise the lookup across requests; labels that do not exist in the repo are silently skipped (graceful degradation, Principle III). The Worker stores `GITHUB_REPO_ID` (the repository node ID) as a secret, populated once at deploy time from `gh api graphql -f query='{ repository(owner:"...",name:"..."){ id } }'`.
- **FR-004**: The Worker MUST NOT persist the payload beyond the lifetime of the request (no logs of user text beyond standard rate-limit counters; no storage in KV except rate-limit metadata).
- **FR-005**: Attachments (image, voice) MUST be uploaded to storage durable enough to survive the issue's lifetime. Decision (research R2): Cloudflare R2 bucket with public read URL — GitHub's user-content upload endpoint is undocumented and fragile. The Worker MUST track whether each attachment object was newly created by the current request; if the request fails before the GitHub issue exists, the Worker MUST delete only those newly-created R2 objects before returning the error. Pre-existing content-hash objects MUST remain untouched because another issue may already reference them.
- **FR-006**: On success, the Worker MUST return `{ok: true, issueUrl: "https://github.com/...", issueNumber: <number>}`. On failure, `{ok: false, error: "<code>", message: "<human-readable>"}`. Both `issueUrl` and `issueNumber` are required on success — the dialog uses `issueNumber` to render `#NN` in the success-state link, and the pair lets the maintainer correlate Tail logs (which carry `issue_number`) with the actual GitHub issue. The Worker-side idempotency KV cache is described in FR-012 + data-model.md and is server-only — `issueNumber` is not used to build it.
- **FR-007**: The dialog MUST display a success state inline on `ok: true` response, containing the issue URL as a visible clickable link. It MUST NOT auto-close the dialog — the user dismisses it.
- **FR-008**: On any non-OK response from the Worker (500, 503, 504, network error, timeout > 15s on the browser side), the dialog MUST fall back to the feature-002 `window.open(prefillURL, "_blank", "noopener,noreferrer")` flow with the same pre-fill URL (which itself includes `labels=user-feedback,needs-triage` so the fallback issue lands in the same triage bucket as Worker-posted ones), and surface a small neutral note about the fallback.
- **FR-009**: The Worker MUST rate-limit by IP at 5 requests per fixed UTC-hour window. A sixth request returns HTTP 429 with `Retry-After` header. Implementation via Cloudflare KV with TTL keyed `rl:<ip>:<hour>` (see research R3 for the fixed-vs-sliding decision).
- **FR-010**: The Worker MUST validate payload limits: title ≤ 200 chars, body ≤ 10 KB UTF-8, image ≤ 5 MB, voice ≤ 15 MiB decoded (15,728,640 bytes). Violations return HTTP 400.
- **FR-011**: CORS: the Worker MUST respond with `Access-Control-Allow-Origin: *` (or allowlist), `Access-Control-Allow-Methods: POST, OPTIONS, GET` (POST for `/feedback`, GET for `/health` cross-origin polling, OPTIONS for preflight), and handle `OPTIONS` preflight. Reports opened from `file://` have origin `null` — the Worker MUST accept this.
- **FR-012**: The Worker MUST include an idempotency check: requests with the same `idempotencyKey` within 5 minutes that produced a successful issue return the cached `{ok: true, issueUrl, issueNumber}` response instead of creating a duplicate.
- **FR-013**: The report's dialog JS MUST generate `idempotencyKey` as `crypto.randomUUID()` at Submit-click time and reuse it on retry.
- **FR-014**: The Worker MUST set a 10 000 ms `AbortController` timeout on each GitHub API fetch (installation-token exchange, label resolution, `createIssue` mutation). On any per-call timeout the Worker MUST return HTTP 504 with `error: "github_timeout"` and the dialog falls back to `window.open` (per FR-008). The 10s Worker-side timeout sits inside the 15s browser-side budget so a single GitHub stall surfaces as a clean Worker-side 504 rather than a browser AbortError.
- **FR-015**: The generated report's HTML MUST NOT contain any secret credential. The Worker URL is public (anyone can POST to it — protected by rate-limit + validation).
- **FR-016**: The constitutional amendment (Principle IX named exception) is a precondition for this feature. As of constitution v1.3.0 (ratified 2026-04-24, already on main) the exception is in force; the implementation PR MUST verify the constitution is at v1.3.0 and MUST NOT attempt to land a Worker without that exception present.

### Key Entities

- **Feedback payload**: in-transit JSON `{title, body, category, image, voice, idempotencyKey, reportVersion}`. Never persisted.
- **Rate-limit record**: KV entry `rl:<ip>:<hour>` → count. TTL 1 hour. Never contains user content. (Key shape matches FR-009 and data-model.md.)
- **Idempotency record**: KV entry `idem:<idempotency-key>` → `{issueUrl, issueNumber}`. TTL 5 minutes. Contains only the resulting URL + number, not the original payload.
- **GitHub App credentials**: App ID, Installation ID, and Private Key stored as Cloudflare Worker Secrets. Never in code, never in logs.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 95% of successful Submits complete within 3 seconds end-to-end (click to success-state-visible) on a broadband connection.
- **SC-002**: With Worker down, 100% of Submits fall back to `window.open` cleanly (no error UI confusing the user; no data loss).
- **SC-003**: Rate-limit rejection rate under normal usage (5 submits per user per hour budget) is < 1%. Under a simulated attack (100 submits/min from same IP), 100% of requests above the limit return HTTP 429 within 50ms.
- **SC-004**: Monthly Cloudflare cost target is **$0** on the free tier. The Workers paid tier ($5/month) is acceptable iff (a) the deploy-time CPU measurement (research R9, tasks T029) shows >9 ms isolate CPU per request and (b) the KV-cached installation-token mitigation is also unable to bring it under budget; in that case the success criterion shifts to "≤ $5/month, documented in the deploy commit message". Monthly GitHub cost: $0. Any Cloudflare cost >$5 OR any GitHub cost >$0 raises an alarm in the dashboard.
- **SC-005**: Zero secrets in the shipped binary or generated HTML. Verified by grep of the release artifact for known patterns (`ghp_`, `github_pat_`, `ghs_`, PEM markers).
- **SC-006**: First-click adoption: within the first month after ship, at least 50% of Submits succeed via the Worker path (not the fallback). Measures whether the feature is working in practice.

## Assumptions

- User has Cloudflare account and can deploy Workers. If not, a one-time setup is needed (Phase 1 of the implementation plan).
- User controls `matias-sanchez/My-gather` GitHub org/repo and can create + install a GitHub App there.
- The repo has the labels `user-feedback`, `needs-triage`, `area/ui`, `area/parser`, `area/advisor`, `area/other` created (slash-separated, lowercase). One-time setup, see quickstart Step 1.2. Labels that are missing at runtime are silently skipped (graceful degradation, Principle III), so a repo with only the first two labels still gets feedback issues — they just lose the area routing.
- GitHub's GraphQL `createIssue` mutation remains stable (documented under `https://docs.github.com/en/graphql/reference/mutations#createissue`).
- GitHub's rate-limit budget of 5000 requests/hour per installation is far above expected traffic. If exceeded, fallback is acceptable.
- Cloudflare Workers free tier provides 100k req/day and 10ms isolate CPU per request. Wall-clock latency (the ~200ms GitHub call) does NOT count against the CPU budget — the isolate is suspended during I/O. The dominant on-CPU cost is `jose` JWT signing (research R9 estimates ~20ms; needs measurement before deploy). If measured CPU per request reliably exceeds 10ms, the paid tier ($5/month, 30s CPU/req) is the escape hatch — at &lt;100 req/day projected traffic the cost is negligible. Ship-blocker only if measurement shows we need paid tier and the user does not authorise the $5/month spend.
- Corporate networks hosting the report viewers allow outbound HTTPS to `*.workers.dev`. If blocked, fallback flow still works via `window.open` to github.com (usually allowed).
