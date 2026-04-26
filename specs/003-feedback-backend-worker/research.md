# Phase 0 Research — Feedback Backend Worker

## R1: GitHub App auth — `octokit` library vs hand-rolled?

**Decision**: hand-rolled client using `jose` for JWT signing + `fetch` for the two GitHub calls we make (installation token exchange + `POST /repos/{owner}/{repo}/issues`). ~80 LOC.

**Rationale**:
- We make exactly 2 GitHub calls per Submit. Pulling in all of `@octokit/app` (~400 KB) to save 80 LOC is an enormous bundle tax on a Worker with a 1 MB size limit and a 10ms CPU budget.
- `jose` is 8 KB, widely used, only provides the RS256 JWT signing we need.
- `octokit` handles caching of installation tokens, which we don't need (each request re-exchanges; cost is negligible at our rate).

**Alternatives considered**:
- `@octokit/app`: rejected. 50× the size for zero ergonomic gain.
- Bring GitHub's JWT signing into the Worker via a full JWT library (`jsonwebtoken`, etc.): rejected. None of them are Workers-compatible out of the box (they need Node.js crypto); `jose` is Web-Crypto-native.
- GitHub GraphQL `createIssue` mutation: rejected. Requires fetching `repositoryId` once at deploy time and passing it as a node ID. REST takes `:owner/:repo` directly in the URL — one fewer setup step, one fewer secret. REST returns `html_url` and `number` directly, which is exactly what the Worker passes to the dialog.

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
4. Worker creates the issue via `POST /repos/{owner}/{repo}/issues` with `labels: ["feedback", "area:<category>"]` (the second label only if `category` is set).

R2 bucket is publicly readable. Privacy note: these assets *are* the user's feedback content already being posted publicly in a GitHub Issue; R2's public read is equivalent.

## R3: Rate-limiting — fixed window vs sliding window?

**Decision**: **fixed window** (simpler) keyed by `<ip>:<current-hour-UTC>`.

**Rationale**:
- Sliding-window requires storing N timestamps per IP; fixed-window stores one counter.
- At our rate-limit threshold (5/hour), the worst edge case of fixed-window is "a user gets 10 submits across a minute that spans :59 → :00". Not a problem for actual attack scenarios.
- Cloudflare KV TTLs are per-key; one `SET rl:<ip>:<hour> ttl=3600` per increment (key prefix matches data-model.md).

**Alternatives considered**:
- Sliding-window: marginal benefit, complex implementation.
- Durable Objects: strong consistency guarantees but not free tier. Overkill.
- Turnstile (Cloudflare's CAPTCHA): adds user friction, reintroducing the "clicks" problem we're trying to eliminate.

## R4: Idempotency — how long to cache?

**Decision**: **5 minutes** TTL.

**Rationale**:
- Long enough to cover network retries and browser double-click.
- Short enough that if the user legitimately wants to submit a second issue with identical title + body (not generating a new idempotency key), they can within 5 minutes. (In practice JS generates a new UUID per Submit-click, so this isn't a concern.)

## R4b: Category → labels mapping

**Decision**: ship five labels in the repo: `feedback` (always applied) and four `area:<x>` (applied iff `category` is set).

**Rationale**:
- Issues do not have categories. The original input asked for an "Ideas category" in Discussions (verbatim Spanish wording preserved on `spec.md:6`). Labels are the closest Issue equivalent and survive triage moves better than categories.
- A fixed `feedback` label lets the maintainer filter the inbox: `is:issue label:feedback` is the bot-submitted bucket, and human-filed issues stay separate.
- Secondary `area:<x>` labels mirror the `category` enum in `render/feedback.go` (`UI` / `Parser` / `Advisor` / `Other`), keeping the user's intent visible without polluting the issue body.
- Using REST means we pass labels by *name* in the request body. No setup-time GraphQL ID lookup, no `GITHUB_LABEL_IDS` secret. The labels just need to exist — see quickstart Step 1.2 for the one-time `gh label create` commands.

**Alternatives considered**:
- No labels (plain Issue): rejected. The maintainer's inbox already has manually-filed issues; no way to triage Worker-submitted ones separately.
- Issue Form template (`.github/ISSUE_TEMPLATE/*.yml`): rejected. The Worker bypasses the form anyway (it POSTs JSON directly), and the form's structure would lock the Worker payload shape to the form's question schema.
- Labels by ID via GraphQL `addLabelsToLabelable`: rejected. Adds one more network round-trip per Submit and one more setup-time lookup. REST does it in the same call as `createIssue`.

## R5: CORS — allowlist or `*`?

**Decision**: `Access-Control-Allow-Origin: *` with method scoped to `POST` (the feedback endpoint), `OPTIONS` (preflight), and `GET` (the `/health` endpoint, intentionally readable cross-origin so monitoring can poll without same-origin gymnastics). Request `Content-Type` restricted to `application/json`.

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

**Decision**: a Go `const feedbackWorkerURL` in `render/feedback.go`. The URL is chosen at GitHub-App setup time by the project owner and is a public endpoint.

**Rationale**:
- Build-time constant → Principle IV determinism preserved.
- One place to change if we redeploy the Worker (which is rare).

**Alternatives considered**:
- `-ldflags -X` at build time: more flexible but complicates dev builds. The URL is effectively permanent; a constant is simpler.
- Environment variable read at runtime: **rejected**. The Go binary runs on arbitrary customer jump hosts; it can't reach a config service.

## R8: Fallback trigger — what counts as "Worker failure"?

**Decision**: fall back to `window.open` on any of:
- Network error (fetch rejected).
- Timeout > 5s (implemented via `AbortController`).
- HTTP 5xx response.
- HTTP 4xx that is NOT a validation error that the user can fix (422, 429).

For 429 (rate-limit), show the throttle message; don't fall back — the user will retry after the cooldown.
For 400 (validation error), show the field-specific error inline; the user fixes it and retries.

**Rationale**: fallback protects availability. Validation errors are the user's data shape issue; fallback would silently paper over them.

## R9: Worker cold-start impact

**Decision**: acceptable for wall-clock (≤3s budget per SC-001). Cloudflare Workers have essentially zero cold-start (<5ms isolate boot). `jose` JWT signing adds ~20ms — this is **on-CPU work that counts against the free tier's 10ms-per-request CPU budget**. GitHub REST call (`POST /repos/.../issues`) ~200ms p95 — wall-clock only; the isolate is suspended during I/O and consumes no CPU budget. Total wall-clock p95 ~300ms on a warm path.

**CPU caveat**: the 20ms JWT estimate may exceed the 10ms free-tier CPU cap. Measurement at deploy is required (T029 of tasks.md records the actual CPU usage from `wrangler tail`). The cost-gate ladder, per SC-004 + T029, is normative and ordered — mitigations are NOT a free choice:

1. If measured median CPU ≤ 9 ms: ship on the free tier. Done.
2. If measured median CPU > 9 ms: implement the KV-cached installation-token mitigation in `github-app.ts` (cache the token in KV across requests with a 1h TTL, saving the JWT-signing cost on all but the first request per hour). Re-deploy and re-measure.
3. If the second measurement still shows median CPU > 9 ms: only THEN move to the paid tier ($5/month, 30s CPU/req — see spec.md SC-004 + Assumptions). Document the measurement evidence in the deploy commit.

Skipping straight to the paid tier without trying step 2 violates SC-004's AND-gate. Skipping step 1's measurement and assuming step 2 is needed also violates SC-004 (the team must have evidence). The post-launch-mitigation framing the earlier draft of this section used was too permissive; this ladder replaces it.

**Alternatives considered**: pre-warming (Durable Object that pings itself periodically) — unnecessary at this latency.

## R10: Observability — how do we know Submits are working?

**Decision**: Worker logs structured JSON (no user content — just `{timestamp, status, issue_id, issue_number, ip_hash, duration_ms}`) to Cloudflare's Tail. Alarm if error rate > 10% over 1 hour.

**Rationale**:
- Free tier includes logs.
- Never log user text — Principle-adjacent privacy preservation.

## Outcome

All unknowns resolved. Feature is ready for Phase 1 (data-model + contracts).
