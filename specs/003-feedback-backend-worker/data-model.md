# Phase 1 Data Model — Feedback Backend Worker

## Browser-side (transient)

### Submit payload (JSON)

```typescript
interface FeedbackPayload {
  title:          string;   // 1-200 chars
  body:           string;   // 0-10240 bytes UTF-8
  category?:      "UI" | "Parser" | "Advisor" | "Other";  // matches render/feedback.go
  image?:         {
    mime: string;           // "image/png", "image/jpeg", etc.
    base64: string;         // <= 7 MB base64-encoded (5 MB raw × 1.37 overhead)
  };
  voice?:         {
    mime: string;           // "audio/webm", "audio/mp4"
    base64: string;         // <= 14 MB base64-encoded (10 MB raw × 1.37 overhead)
  };
  idempotencyKey: string;   // crypto.randomUUID()
  reportVersion:  string;   // "0.3.x-g<commit>" (my-gather version) — for debugging, not for auth
}
```

### Browser response handling

```typescript
interface WorkerOkResponse {
  ok: true;
  issueUrl: string;          // "https://github.com/matias-sanchez/My-gather/issues/NN"
  issueNumber: number;       // duplicates the trailing /NN for clients that want a numeric handle
}

interface WorkerErrorResponse {
  ok: false;
  error: string;             // machine-readable code
  message: string;           // human-readable, already localized in English
  retryAfterSeconds?: number; // only on rate-limit (429)
}
```

Error codes the Worker may emit (full list — must stay in sync with `contracts/api.md`):

Validation (HTTP 400):
- `title_required` / `title_too_long`
- `body_too_long`
- `image_too_large` / `image_bad_mime`
- `voice_too_large` / `voice_bad_mime`
- `category_invalid`
- `idempotency_key_invalid`
- `report_version_invalid`
- `malformed_payload`

Operational:
- `payload_too_large` (HTTP 413 — total request body > 30 MB; trips the streaming size gate before JSON parse)
- `duplicate_inflight` (HTTP 409 — same `idempotencyKey` is already being processed by another in-flight request; the inflight TTL is 30s)
- `rate_limit` (HTTP 429, `retryAfterSeconds` present)
- `github_api_error` (HTTP 503 — GitHub returned a non-2xx HTTP status or a GraphQL `errors` field)
- `github_timeout` (HTTP 504 — a per-call AbortController fired at 10 000 ms on a GitHub fetch, see FR-014)
- `worker_error` (HTTP 500 — bug on our side; the catch-all for an unexpected throw)
- `cache_write_failed` (HTTP 200 — the issue was created but the idempotency-cache write retries exhausted; the response includes the real `issueUrl`/`issueNumber` and the client never sees the failure, but observability logs the diagnostic so an operator can investigate persistent KV writes failing)
- `not_found` (HTTP 404 — any path other than `POST /feedback`, `OPTIONS /feedback`, `GET /health`)

The dialog picks its UI state (success, validation error, throttle, fallback) from `{ok, error}`.

## Worker-side (transient)

### Cloudflare KV — `FEEDBACK_RATELIMIT`

Each request writes its OWN unique key under a per-IP/per-hour prefix; the count is the number of keys returned by `KV.list({prefix})`. This avoids the read-modify-write race the prior counter pattern admitted (concurrent same-IP requests would read the same value and overwrite each other with the same `next`, silently allowing more than `LIMIT` submissions per window).

Key format: `rl:<ipHash>:<UTC-hour>:<random-suffix>`
- `<ipHash>` = `SHA-256(ip + GITHUB_APP_ID)` hex (32 bytes → 64 hex chars). The App ID is the per-deployment salt; never reveal IPs in logs.
- `<UTC-hour>` = `YYYY-MM-DDTHH` (zero-padded; UTC always).
- `<random-suffix>` = 12 random bytes hex (24 chars).

Value: `"1"` (literal — the count comes from `list().keys.length`, not the value).

TTL: 3600 seconds — KV expires entries on its own at hour rollover.

Example: `rl:e3b0c44…:2026-04-26T08:7a91f3c2…` → `"1"`

Limit: 5 entries per `<ipHash>:<hour>` prefix. The 6th request's `list()` sees 5 keys and returns `allowed: false` without writing a new entry.

KV is eventually consistent (~100 ms global propagation); a short same-second burst can leak one or two extra writes before the count stabilises. Acceptable at a 5/hour limit; full linearisability would require Durable Objects (out of scope per Principle X — minimal infra surface).

### Cloudflare KV — `FEEDBACK_IDEMP`

Two-state discriminated union, written under one key:

Key format: `idem:<idempotency-key>` (full UUID v4)

Value when reservation is in flight: `{"status": "inflight"}` — TTL **30 seconds**.
Value once the issue exists on GitHub: `{"status": "done", "issueUrl": "<url>", "issueNumber": <n>}` — TTL **300 seconds** (5 minutes).

The reservation pattern (`reserveResponse`) writes the `inflight` marker BEFORE any GitHub side effect; on success the `cacheResponse` write upgrades it to `done` with last-write-wins semantics. A retry of an already-`done` key replays the cached response without hitting GitHub. A retry while the reservation is still `inflight` returns 409 `duplicate_inflight` (deterministic; the inflight TTL self-cleans after 30 s if the original handler crashes mid-flight). There is intentionally NO `releaseReservation` helper — KV has no compare-and-delete primitive, so a release path admits a race that could wipe another caller's `done` record. Letting the 30 s TTL handle cleanup is correct and simpler.

Example evolution of one key over a Submit's lifetime:

```
t+0    PUT idem:01234… → {"status":"inflight"}              (TTL 30s)
t+250ms PUT idem:01234… → {"status":"done", issueUrl:…, issueNumber: 67} (TTL 300s)
t+5min  GET idem:01234… → null (TTL expired)
```

Also stored in this same KV namespace under separate key prefixes: `labels:<repoNodeId>` (TTL 86400s, value = JSON map of `name → labelId`) — the GraphQL label-resolution cache. Sharing a namespace is intentional: one KV binding, two key families, both bounded by short-to-medium TTL.

### Cloudflare R2 — `FEEDBACK_ATTACHMENTS`

Key format: `attachments/<sha256>.<ext>`
Value: binary
Public read: yes (Worker-managed public bucket URL).
No TTL — attachments live as long as the R2 bucket does.

## GitHub-side (persistent, user-visible)

### Issue body markdown composition

Assembled by the Worker, in this exact order:

```markdown
<if category set:>
> Category: <category>
                                  <!-- blank line -->
<user body text, as-is>

<if image attached:>
                                  <!-- blank line -->
### Attached screenshot
                                  <!-- blank line -->
![screenshot](<R2 public URL>)

<if voice attached:>
                                  <!-- blank line -->
### Attached voice note
                                  <!-- blank line -->
<R2 public URL>
                                  <!-- blank line; bare URL on its own line so GitHub auto-renders it as an inline player when the rendering surface supports it -->
                                  <!-- blank line -->
---
_Submitted via my-gather Report Feedback (v<reportVersion>)._
```

The `category` field appears BOTH as a `> Category: <X>` blockquote at the top of the body AND as an `area/<lower(X)>` label on the issue. The intentional duplication serves two surfaces: the blockquote is searchable in the issue body via GitHub's text search, the label is filterable in the issue list view via `label:area/ui`. The labels `user-feedback` and `needs-triage` are always applied unconditionally; `area/<lower>` is added only when `category` is set.

The footer attribution is a single line that helps the maintainer distinguish Worker-submitted issues from manually-filed ones (same signal as `is:issue label:user-feedback`, but visible inside the rendered issue too).

### GitHub GraphQL mutation

The Worker uses GraphQL `createIssue` rather than REST `POST /repos/.../issues`. Rationale: the same Worker also pages through `repository.labels` (GraphQL only) to resolve label NAMES to label IDs (`createIssue` takes `labelIds`, not names), and bundling label-list + create on the same endpoint keeps the API surface consistent. The setup cost is one extra secret (`GITHUB_REPO_ID`, the repo's GraphQL node ID, fetched once via `gh api graphql -f query='{repository(owner:"…",name:"…"){id}}'`).

```graphql
mutation CreateFeedbackIssue($input: CreateIssueInput!) {
  createIssue(input: $input) {
    issue { id number url }
  }
}
```

Variables:
```json
{
  "input": {
    "repositoryId": "<GITHUB_REPO_ID secret>",
    "title":        "<user title>",
    "body":         "<assembled markdown>",
    "labelIds":     ["<id of user-feedback>", "<id of needs-triage>", "<id of area/ui>"]
  }
}
```

`labelIds` is the resolved list (Worker `resolveLabelIds()` looks up names in a KV-cached `name→id` map; missing labels are skipped silently per Principle III).

Successful response: `data.createIssue.issue` has `id` (node ID → `issue_id` in observability log), `number` (→ `issueNumber` in the Worker response + `issue_number` in logs), `url` (→ `issueUrl` in the Worker response). The Worker uses these three fields and ignores the rest.

Authentication: `Authorization: Bearer <installation-access-token>` (same token type as REST), plus the standard `Accept: application/vnd.github+json`, `User-Agent: my-gather-feedback-worker`, `X-GitHub-Api-Version: 2022-11-28` headers.

## Worker Secrets (never in code)

| Secret | Source | Usage |
|---|---|---|
| `GITHUB_APP_ID` | Numeric, from App settings page | JWT `iss` claim AND IP-hash salt for `ratelimit.ts` |
| `GITHUB_INSTALLATION_ID` | Numeric, from installation URL | Token exchange |
| `GITHUB_APP_PRIVATE_KEY` | PEM (PKCS#8 — `BEGIN PRIVATE KEY`), downloaded once on App creation | JWT signing via `jose.importPKCS8` + RS256. PKCS#1 keys (`BEGIN RSA PRIVATE KEY`) MUST be converted with `openssl pkcs8 -topk8 -nocrypt` before upload |
| `GITHUB_REPO_ID` | GraphQL node ID, fetched once via `gh api graphql -f query='{repository(owner:"…",name:"…"){id}}'` | Pinned `repositoryId` arg in `createIssue` mutation; also keyed in the labels-cache KV entry as `labels:<GITHUB_REPO_ID>` |
| `R2_PUBLIC_URL_PREFIX` | Public R2 bucket URL (`https://pub-…r2.dev`) | Linking R2-hosted attachments in the issue body |

All set via `wrangler secret put`. Stored encrypted at rest by Cloudflare. Total: **5 secrets**.

The hard-coded labels (`user-feedback`, `needs-triage`, `area/<lower>`) and the always-applied set live in `feedback-worker/src/index.ts` (function `categoryLabel` + the unconditional array). They are NOT secrets and NOT `[vars]` — repointing a fork at a different repo just needs different label names in source, since the labels themselves are tied to the repo's triage convention. Keeping them in source means the labels track the spec without an extra config indirection.

## Determinism & idempotency

- **Client-generated idempotency key**: a `crypto.randomUUID()` is minted once per Submit-click and reused on any automatic retry (network error → retry). This is not a "client-deduplication" signal — it's the server's contract: given the same key within 5 minutes, return the same `{issueUrl, issueNumber}`.
- **Attachment content-hashing**: two submissions with the same image produce the same R2 key. This is a side-effect property; clients don't need to know.

## Privacy posture

- Worker logs never contain user body text.
- KV values never contain user body text (only counters and URLs).
- R2 contains attachments but those attachments are already public (posted in a public issue).
- No analytics beacon, no IP geolocation, no device fingerprinting.
- The `ip_hash` in logs is `SHA-256(ip + worker-deploy-salt)` so it's useful for rate-limit correlation but not reversible.
