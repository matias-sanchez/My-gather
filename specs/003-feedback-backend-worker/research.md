# Phase 0 Research — Feedback Backend Worker

## R1: GitHub App auth — `octokit` library vs hand-rolled?

**Decision**: hand-rolled client using `jose` for JWT signing + `fetch` for the two GitHub calls per Submit (installation-token exchange + GraphQL `createIssue` mutation). The deployed Worker also performs a paginated `repository.labels` GraphQL lookup behind a 24 h KV cache to resolve label NAMES to label IDs, since `createIssue` takes `labelIds`, not names. See `feedback-worker/src/github-app.ts`.

**Rationale**:
- The Worker makes exactly 2 GitHub calls per Submit (token exchange + GraphQL mutation). Pulling in all of `@octokit/app` (~400 KB) to save ~80 LOC is an enormous bundle tax on a Worker with a 1 MB size limit and a 10 ms CPU budget.
- `jose` is 8 KB, widely used, and provides the RS256 JWT signing we need against Web Crypto.
- The deployed App private key is PKCS#8 (`BEGIN PRIVATE KEY`), which `jose` imports natively.

**Alternatives considered**:
- `@octokit/app`: rejected. 50× the size for zero ergonomic gain.
- Bring GitHub's JWT signing into the Worker via a full JWT library (`jsonwebtoken`, etc.): rejected. None of them are Workers-compatible out of the box (they need Node.js crypto); `jose` is Web-Crypto-native.
- REST `POST /repos/{owner}/{repo}/issues`: considered, but GraphQL `createIssue` was preferred because the same Worker also pages through `repository.labels` to resolve label NAMES to IDs (`createIssue` takes `labelIds`, not names) — bundling the label-list + create flow on the same GraphQL endpoint keeps the API surface consistent. The one extra setup secret this requires is `GITHUB_REPO_ID` (the repository node ID), already part of the deployed secret set.

## R2: Attachment hosting — GitHub's user-content endpoint vs Cloudflare R2?

**Decision**: **upload to R2** and embed public URLs in the markdown body.

**Rationale**:
- GitHub's image upload endpoint used by the web form (`POST https://uploads.github.com/upload/...`) is **undocumented**. Depending on it is a liability.
- R2 is free to 10 GB. One Worker binding. Signed public URLs are trivial. Files live forever (or until explicit deletion).
- R2 objects can have metadata; we attach `{submitted_at, content_hash}` for audit.

**Alternatives considered**:
- Embed base64 data URIs in the markdown body: **rejected**. GitHub strips `data:` URIs from markdown for security. Only `https://` URLs render as images.
- Upload to GitHub via the REST API (`POST /repos/{owner}/{repo}/issues/{number}/attachments`): **rejected — does not exist**. GitHub does not have a public REST attachments API.
- Use the feature-002 existing "clipboard + download" UX: regressive. User experience has to be one-click.
- Host on a dedicated `matias-sanchez/My-gather-feedback-assets` GitHub repo via the Contents API: possible. Worker commits the blob as a file, links to the raw URL. Simpler than R2 but pulls every attachment into a repo forever. Size bloat risk.

### Chosen flow

1. Worker receives payload with base64 image + voice.
2. Worker decodes; puts each blob into R2 under a content-hashed key `attachments/<sha256>.<ext>`. Idempotent by content — same image posted twice hits the same R2 key.
3. Worker builds the issue body markdown with `![image](https://feedback-assets.cf/attachments/<hash>.png)` for images and `🔊 Voice note ([audio/webm](https://feedback-assets.cf/attachments/<hash>.webm))` for audio. (`<audio>` tags are stripped by GitHub's issue-body HTML sanitizer; a plain Markdown link is the most that survives. The reader clicks through and the browser plays the file natively.)
4. Worker creates the issue via the GraphQL `createIssue` mutation with `labelIds` resolved from the names `["user-feedback", "needs-triage"]` plus `area/<lower(category)>` (the third label only if `category` is set).

R2 bucket is publicly readable. Privacy note: these assets *are* the user's feedback content already being posted publicly in a GitHub Issue; R2's public read is equivalent.

## R3: Rate-limiting — fixed window vs sliding window?

**Decision**: **fixed window** keyed by `rl:<ipHash>:<UTC-hour>:<random>`. The deployed Worker writes a unique key per request under a per-IP / per-hour prefix and counts via `KV.list({prefix: "rl:<ipHash>:<UTC-hour>:"}).keys.length`. App ID is the salt for the IP hash. See `feedback-worker/src/ratelimit.ts`.

**Rationale**:
- Sliding-window requires storing N timestamps per IP; fixed-window stores one counter — but a single shared counter would force read-modify-write under bursts, which KV does not serialize. Writing a unique random-suffixed key per request and counting via `list().keys.length` removes the read-modify-write race entirely.
- At our rate-limit threshold (5 / hour), the worst edge case of fixed-window is "a user gets 10 submits across a minute that spans :59 → :00". Not a problem for actual attack scenarios.
- Each per-request key has a TTL just past the end of the current UTC hour, so KV self-cleans without an explicit sweep.

**Alternatives considered**:
- Sliding-window: marginal benefit, complex implementation.
- Durable Objects: strong consistency guarantees but not free tier. Overkill.
- Turnstile (Cloudflare's CAPTCHA): adds user friction, reintroducing the "clicks" problem we're trying to eliminate.

## R4: Idempotency — how long to cache?

**Decision**: **5 minutes** TTL on the terminal `done` state, with an additional **30 second** `inflight` reservation written before the GitHub mutation. Two states are stored under the same `idem:<uuid>` key:
- `{status: "inflight"}` (TTL 30 s) — written immediately after rate-limit, BEFORE any side effects, so a concurrent retry sees the reservation and short-circuits.
- `{status: "done", issueUrl, issueNumber}` (TTL 300 s = 5 min) — written after `createIssue` succeeds. Subsequent retries with the same key replay this response.

See `feedback-worker/src/idempotency.ts`.

**Rationale**:
- 5 minutes is long enough to cover network retries and browser double-click; short enough that a deliberate re-submit with a fresh UUID is never blocked.
- The 30 s `inflight` reservation closes the race where two near-simultaneous requests with the same idempotency key both pass the "is it cached?" check and end up creating two issues.
- There is intentionally no `releaseReservation` helper. Cloudflare KV has no compare-and-delete primitive, so any release path admits the race "caller A's `done` is overwritten by caller B's stale `release`". Instead, the `inflight` record simply expires after 30 s if the request crashes mid-flight; the next retry will then proceed.

## R4b: Category → labels mapping

**Decision**: ship six labels in the repo: `user-feedback` (always applied), `needs-triage` (always applied), and four `area/<x>` labels (`area/ui`, `area/parser`, `area/advisor`, `area/other`) — the `area/*` label is applied iff `category` is set, lower-cased. See `feedback-worker/src/index.ts`.

**Rationale**:
- Issues do not have categories. The original input asked for an "Ideas category" in Discussions (verbatim Spanish wording preserved on `spec.md:6`). Labels are the closest Issue equivalent and survive triage moves better than categories.
- A fixed `user-feedback` label lets the maintainer filter the inbox: `is:issue label:user-feedback` is the bot-submitted bucket, and human-filed issues stay separate.
- A fixed `needs-triage` label is added on every Submit so the maintainer's "to be reviewed" view never misses one.
- Secondary `area/<x>` labels mirror the `category` enum in `render/feedback.go` (`UI` / `Parser` / `Advisor` / `Other`), lower-cased and slash-separated, keeping the user's intent visible without polluting the issue body.
- The deployed Worker resolves these label NAMES to GraphQL label IDs once per 24 h via the cached `repository.labels` page-walk in `github-app.ts`. The labels just need to exist on the repo — see quickstart Step 1.2 for the one-time `gh label create` commands.

**Alternatives considered**:
- No labels (plain Issue): rejected. The maintainer's inbox already has manually-filed issues; no way to triage Worker-submitted ones separately.
- Issue Form template (`.github/ISSUE_TEMPLATE/*.yml`): rejected. The Worker bypasses the form anyway (it POSTs JSON directly), and the form's structure would lock the Worker payload shape to the form's question schema.
- Labels by ID baked into a `GITHUB_LABEL_IDS` secret: rejected. Renames or recreations of a label would silently break the mapping; the deployed name → ID lookup with a 24 h cache survives renames automatically.

## R5: CORS — allowlist or `*`?

**Decision**: `Access-Control-Allow-Origin: *` with `Access-Control-Allow-Methods: POST, OPTIONS, GET` — the deployed Worker ships exactly that string. `POST` covers the feedback endpoint, `OPTIONS` covers preflight, and `GET` covers the `/health` endpoint, intentionally readable cross-origin so monitoring can poll without same-origin gymnastics (and so a browser visiting `/health` directly auto-renders any audio URL Cloudflare returns). Request `Content-Type` restricted to `application/json`.

**Rationale**:
- Reports are shipped to arbitrary machines; we don't know the origins. The report may be served from `file://`, which presents `Origin: null`.
- Allowing any origin is fine because the endpoint is already public (anyone can POST with curl). CORS adds no real security — rate-limit + payload validation + App-scoped token do.

**Alternatives considered**:
- Allowlist `https://*.my-gather.dev` + `null`: restricts to our reports but makes embeddable third-party usage impossible. Marginal security win.
- No CORS headers: breaks browsers silently. Bad.

## R6: How does the browser build the payload?

**Decision**: after the existing `buildURL` helper in `render/assets/app.js`, add a `buildPayload()` helper that assembles `{title, body, category, image: base64, voice: base64, idempotencyKey: uuid, reportVersion: "0.3.x"}`. Base64-encode blobs via `FileReader.readAsDataURL()` (the URI prefix is stripped before sending).

**Rationale**:
- Base64 is the only JSON-safe blob representation without a separate multipart flow.
- Expanded size (~33% overhead) is acceptable at the caps (5 MB image becomes 6.6 MB JSON — well under practical body size limits, still fast on broadband).

**Alternatives considered**:
- `multipart/form-data`: more efficient but requires manual boundary management in Workers (no built-in `FormData` parser). Not worth the complexity.

## R7: Where does the Worker URL live?

**Decision**: a Go `const feedbackWorkerURL` in `render/feedback.go`. The deployed value is `https://my-gather-feedback.mati-orfeo.workers.dev/feedback`. The URL is a public endpoint, picked at GitHub-App setup time by the project owner.

**Rationale**:
- Build-time constant → Principle IV determinism preserved.
- One place to change if we redeploy the Worker (which is rare).

**Alternatives considered**:
- `-ldflags -X` at build time: more flexible but complicates dev builds. The URL is effectively permanent; a constant is simpler.
- Environment variable read at runtime: **rejected**. The Go binary runs on arbitrary customer jump hosts; it can't reach a config service.

## R8: Fallback trigger — what counts as "Worker failure"?

**Decision**: fall back to `window.open` on any of:
- Network error (fetch rejected).
- Timeout > 15 s (implemented via `AbortController` in the browser; see `render/assets/app.js`).
- HTTP 5xx response.
- HTTP 4xx that is NOT a validation error that the user can fix (422, 429).

The fallback URL is the static constant `https://github.com/matias-sanchez/My-gather/issues/new?labels=user-feedback,needs-triage`. The `?labels=…` query string is part of the constant itself, not appended at click time, so the fallback issue still lands in the same triage bucket as Worker-created issues.

For 429 (rate-limit), show the throttle message; don't fall back — the user will retry after the cooldown.
For 400 (validation error), show the field-specific error inline; the user fixes it and retries.

**Rationale**: fallback protects availability. Validation errors are the user's data shape issue; fallback would silently paper over them.

## R9: Worker cold-start impact

**Decision**: acceptable for wall-clock (≤3s budget per SC-001). Cloudflare Workers have essentially zero cold-start (<5ms isolate boot). `jose` JWT signing adds ~20ms — this is **on-CPU work that counts against the free tier's 10ms-per-request CPU budget**. GitHub GraphQL call (`createIssue` mutation, plus an amortised `repository.labels` paginated lookup behind a 24h KV cache) ~200ms p95 — wall-clock only; the isolate is suspended during I/O and consumes no CPU budget. Total wall-clock p95 ~300ms on a warm path.

**CPU caveat**: the 20ms JWT estimate may exceed the 10ms free-tier CPU cap. In production the deployed Worker has run on the free tier since 2026-04-24 without CPU-exhaustion errors (see smoke test issues #25 and #26), so the gate is now a verification step, not a blocking measurement. The cost-gate ladder, per SC-004 + T029, remains normative and ordered — mitigations are NOT a free choice — but step 1 is currently observed to hold:

1. Verify cpuTime stays under 9 ms median in `wrangler tail` during a 5-request smoke. The deployed Worker has been observed in this band over several days of free-tier operation. If a future change moves the median above 9 ms, proceed to step 2.
2. If measured median CPU > 9 ms: implement the KV-cached installation-token mitigation in `github-app.ts` (cache the token in KV across requests with a 1h TTL, saving the JWT-signing cost on all but the first request per hour). Re-deploy and re-measure.
3. If the second measurement still shows median CPU > 9 ms: only THEN move to the paid tier ($5/month, 30s CPU/req — see spec.md SC-004 + Assumptions). Document the measurement evidence in the deploy commit.

Skipping straight to the paid tier without trying step 2 violates SC-004's AND-gate. Skipping step 1's measurement and assuming step 2 is needed also violates SC-004 (the team must have evidence).

**Alternatives considered**: pre-warming (Durable Object that pings itself periodically) — unnecessary at this latency.

## R10: Observability — how do we know Submits are working?

**Decision**: Worker logs structured JSON (no user content — just `{timestamp, status, issue_id, issue_number, ip_hash, duration_ms}`) to Cloudflare's Tail. Alarm if error rate > 10% over 1 hour.

**Rationale**:
- Free tier includes logs.
- Never log user text — Principle-adjacent privacy preservation.

## Outcome

All unknowns resolved. Feature is ready for Phase 1 (data-model + contracts).
