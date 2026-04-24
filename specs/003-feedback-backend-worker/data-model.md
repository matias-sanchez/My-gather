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
  discussionUrl: string;     // "https://github.com/matias-sanchez/My-gather/discussions/NN"
}

interface WorkerErrorResponse {
  ok: false;
  error: string;             // machine-readable code
  message: string;           // human-readable, already localized in English
  retryAfterSeconds?: number; // only on rate-limit (429)
}
```

Error codes the Worker may emit:
- `title_required` / `title_too_long`
- `body_too_long`
- `image_too_large` / `image_bad_mime`
- `voice_too_large` / `voice_bad_mime`
- `rate_limit` (HTTP 429, `retryAfterSeconds` present)
- `github_api_error` (HTTP 503)
- `worker_error` (HTTP 500 — bug on our side)

The dialog picks its UI state (success, validation error, throttle, fallback) from `{ok, error}`.

## Worker-side (transient)

### Cloudflare KV — `FEEDBACK_RATELIMIT`

Key format: `rl:<ip>:<hour>`
Value: integer counter (1..5)
TTL: 3600 seconds

Example: `rl:203.0.113.42:2026-04-24T08` → `3`

### Cloudflare KV — `FEEDBACK_IDEMP`

Key format: `idem:<idempotency-key>`
Value: `{discussionUrl: string}` JSON
TTL: 300 seconds

Example: `idem:0196a1b2-3c4d-...` → `{"discussionUrl": "https://github.com/..."}`

### Cloudflare R2 — `FEEDBACK_ATTACHMENTS`

Key format: `attachments/<sha256>.<ext>`
Value: binary
Public read: yes (Worker-managed public bucket URL).
No TTL — attachments live as long as the R2 bucket does.

## GitHub-side (persistent, user-visible)

### Discussion body markdown composition

Assembled by the Worker, in this exact order:

```markdown
> Category: <category, if present>

<user body text, as-is>

<if image attached:>
![Attached image](<R2 public URL>)

<if voice attached:>

https://<worker-host>/attachments/<hash>.<ext>

<footer, single line>
---
_Submitted via my-gather Report Feedback (v<reportVersion>)._
```

The footer is a small attribution line that helps the maintainer distinguish Worker-submitted discussions from manually-posted ones.

### GitHub GraphQL mutation

```graphql
mutation CreateFeedback($repoId: ID!, $categoryId: ID!, $title: String!, $body: String!) {
  createDiscussion(input: {
    repositoryId: $repoId,
    categoryId:   $categoryId,
    title:        $title,
    body:         $body
  }) {
    discussion { id number url }
  }
}
```

The Worker knows `repoId` and `categoryId` as Worker Secrets (fetched once at deploy time via a one-off setup script).

## Worker Secrets (never in code)

| Secret | Source | Usage |
|---|---|---|
| `GITHUB_APP_ID` | Numeric, from App settings page | JWT `iss` claim |
| `GITHUB_INSTALLATION_ID` | Numeric, from installation URL | Token exchange |
| `GITHUB_APP_PRIVATE_KEY` | PEM, downloaded once on App creation | JWT signing |
| `GITHUB_REPO_ID` | GraphQL node ID, static | `createDiscussion` mutation |
| `GITHUB_CATEGORY_ID` | GraphQL node ID for "Ideas" category, static | `createDiscussion` mutation |
| `R2_PUBLIC_URL_PREFIX` | Public R2 bucket URL | Linking assets in discussion body |

All set via `wrangler secret put`. Stored encrypted at rest by Cloudflare.

## Determinism & idempotency

- **Client-generated idempotency key**: a `crypto.randomUUID()` is minted once per Submit-click and reused on any automatic retry (network error → retry). This is not a "client-deduplication" signal — it's the server's contract: given the same key within 5 minutes, return the same discussion URL.
- **Attachment content-hashing**: two submissions with the same image produce the same R2 key. This is a side-effect property; clients don't need to know.

## Privacy posture

- Worker logs never contain user body text.
- KV values never contain user body text (only counters and URLs).
- R2 contains attachments but those attachments are already public (posted in a public discussion).
- No analytics beacon, no IP geolocation, no device fingerprinting.
- The `ip_hash` in logs is `SHA-256(ip + worker-deploy-salt)` so it's useful for rate-limit correlation but not reversible.
