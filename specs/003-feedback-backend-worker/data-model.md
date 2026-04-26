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
Value: `{issueUrl: string, issueNumber: number}` JSON
TTL: 300 seconds

Example: `idem:0196a1b2-3c4d-...` → `{"issueUrl": "https://github.com/matias-sanchez/My-gather/issues/67", "issueNumber": 67}`

### Cloudflare R2 — `FEEDBACK_ATTACHMENTS`

Key format: `attachments/<sha256>.<ext>`
Value: binary
Public read: yes (Worker-managed public bucket URL).
No TTL — attachments live as long as the R2 bucket does.

## GitHub-side (persistent, user-visible)

### Issue body markdown composition

Assembled by the Worker, in this exact order:

```markdown
<user body text, as-is>

<if image attached:>
![Attached image](<R2 public URL>)

<if voice attached:>
<audio src="<R2 public URL>" controls></audio>

<footer, single line>
---
_Submitted via my-gather Report Feedback (v<reportVersion>)._
```

The `category` field is conveyed by an `area:<category>` label on the issue, NOT by a body header. Reasons: labels are filterable in the GitHub UI; a body header line would clutter the rendered issue and be redundant with the label chip.

The footer is a small attribution line that helps the maintainer distinguish Worker-submitted issues from manually-filed ones (same effect as `is:issue label:feedback`, but visible inside the issue too).

### GitHub REST call

```http
POST /repos/matias-sanchez/My-gather/issues
Authorization: Bearer <installation-access-token>
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
Content-Type: application/json

{
  "title":  "<title>",
  "body":   "<assembled markdown>",
  "labels": ["feedback", "area:ui"]   // second label only if category is set
}
```

Successful response (201 Created) includes `html_url` (→ `issueUrl`), `number` (→ `issueNumber`), `node_id` (→ `issue_id` in observability log). The Worker uses these three fields and ignores the rest.

The Worker hard-codes `matias-sanchez/My-gather` in the URL via a `wrangler.toml` `[vars]` entry — see Worker Secrets table below. It is not a secret (the repo is public) but it lives in config, not source, so a future fork can repoint without a code edit.

## Worker Secrets (never in code)

| Secret | Source | Usage |
|---|---|---|
| `GITHUB_APP_ID` | Numeric, from App settings page | JWT `iss` claim |
| `GITHUB_INSTALLATION_ID` | Numeric, from installation URL | Token exchange |
| `GITHUB_APP_PRIVATE_KEY` | PEM, downloaded once on App creation | JWT signing |
| `R2_PUBLIC_URL_PREFIX` | Public R2 bucket URL | Linking assets in issue body |

All set via `wrangler secret put`. Stored encrypted at rest by Cloudflare.

### Worker non-secret config (`[vars]` in `wrangler.toml`)

| Var | Value | Usage |
|---|---|---|
| `GITHUB_REPO` | `matias-sanchez/My-gather` | `:owner/:repo` path segment in `POST /repos/.../issues` |
| `FEEDBACK_LABEL` | `feedback` | Always-applied label |
| `AREA_LABEL_PREFIX` | `area:` | Prefix for `category` → label mapping |

These are not secrets (the repo is public; the labels are visible in the GitHub UI), but keeping them in config rather than source lets a fork repoint to a different repo without a Worker code edit.

## Determinism & idempotency

- **Client-generated idempotency key**: a `crypto.randomUUID()` is minted once per Submit-click and reused on any automatic retry (network error → retry). This is not a "client-deduplication" signal — it's the server's contract: given the same key within 5 minutes, return the same `{issueUrl, issueNumber}`.
- **Attachment content-hashing**: two submissions with the same image produce the same R2 key. This is a side-effect property; clients don't need to know.

## Privacy posture

- Worker logs never contain user body text.
- KV values never contain user body text (only counters and URLs).
- R2 contains attachments but those attachments are already public (posted in a public issue).
- No analytics beacon, no IP geolocation, no device fingerprinting.
- The `ip_hash` in logs is `SHA-256(ip + worker-deploy-salt)` so it's useful for rate-limit correlation but not reversible.
