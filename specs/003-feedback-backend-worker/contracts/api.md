# Phase 1 Contract — Worker HTTP API

## `POST /feedback`

### Request

- Method: `POST`
- URL: `https://<worker-subdomain>.workers.dev/feedback` (or custom domain)
- Headers:
  - `Content-Type: application/json`
  - No `Authorization` header — endpoint is public
- Body: JSON matching `FeedbackPayload` from `data-model.md`

### Request constraints

- Title length: 1–200 chars (trimmed)
- Body length: 0–10 240 bytes UTF-8
- `category`, if present: one of `"UI" | "Parser" | "Advisor" | "Other"` (exact case)
- `image`, if present:
  - `mime` matches `/^image\/(png|jpeg|gif|webp)$/`
  - `base64` decodes to ≤ 5 242 880 bytes
- `voice`, if present:
  - `mime` matches `/^audio\/(webm|mp4|ogg|mpeg)(;.*)?$/`
  - `base64` decodes to ≤ 15 728 640 bytes (15 MB — covers a 10-minute Opus recording at the browser's default bitrate with margin)
- `idempotencyKey`: 36-char UUID v4 (`/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i`) — strict v4 shape; the prior loose pattern `/^[0-9a-f-]{36}$/i` accepted 36 dashes or 36 hex digits with no dashes and let malformed values into KV keys.
- `reportVersion`: 1-64 char string. Required (NOT optional). Read by the dialog from `<meta name="generator">` in the report HTML head, sliced to 64 chars.

### Response

#### 200 OK — success

```json
{
  "ok": true,
  "issueUrl": "https://github.com/matias-sanchez/My-gather/issues/123",
  "issueNumber": 123
}
```

#### 400 Bad Request — validation error

```json
{
  "ok": false,
  "error": "image_too_large",
  "message": "Image exceeds 5 MB."
}
```

Error codes:
- `title_required` · `title_too_long`
- `body_too_long`
- `image_too_large` · `image_bad_mime`
- `voice_too_large` · `voice_bad_mime`
- `category_invalid`
- `idempotency_key_invalid`
- `report_version_invalid`
- `malformed_payload` (catch-all for shape errors: not-a-JSON-object, wrong type on a required field, base64 not decodable, body type ≠ string when present, etc.)

#### 409 Conflict — duplicate inflight

```json
{
  "ok": false,
  "error": "duplicate_inflight",
  "message": "A previous submission with the same idempotency key is still being processed. Retry in a few seconds."
}
```

Emitted when an `idempotencyKey` already has an inflight reservation in KV (a concurrent same-key Submit, or a same-key retry within the 30-second inflight TTL after the first call crashed mid-handler). After 30s of inflight TTL the key is auto-released and a fresh attempt proceeds.

#### 413 Payload Too Large

```json
{
  "ok": false,
  "error": "payload_too_large",
  "message": "Request body exceeds 30 MB."
}
```

Emitted in two places: (a) Content-Length pre-flight rejects when the header is present and >30 000 000; (b) the streaming reader aborts once the running byte count crosses the cap, so a header-less oversized upload can't be fully buffered into the isolate. Always JSON-shaped — no bare HTTP error.

#### 429 Too Many Requests

```json
{
  "ok": false,
  "error": "rate_limit",
  "message": "Rate limit reached. Try again in N minutes.",
  "retryAfterSeconds": 1200
}
```

Headers:
- `Retry-After: <seconds>`

#### 503 Service Unavailable — GitHub API problem

```json
{
  "ok": false,
  "error": "github_api_error",
  "message": "GitHub API unavailable. Please retry or use the fallback link."
}
```

#### 504 Gateway Timeout — GitHub call exceeded the Worker's deadline

```json
{
  "ok": false,
  "error": "github_timeout",
  "message": "GitHub took too long to respond. Please retry or use the fallback link."
}
```

The Worker MUST emit 504 when its `AbortController`-bounded GitHub fetch exceeds 10 000 ms (per FR-014 + tasks.md T013). The dialog falls back to `window.open` on 504 the same way it does on 5xx (per FR-008 + `contracts/ui.md` state machine).

#### 500 Internal Server Error — Worker bug

```json
{
  "ok": false,
  "error": "worker_error",
  "message": "Internal error. Please try again later."
}
```

#### 200 OK with `error: "cache_write_failed"` (degraded success)

```json
{
  "ok": true,
  "issueUrl": "https://github.com/matias-sanchez/My-gather/issues/123",
  "issueNumber": 123
}
```

Note: this is still a 200 OK with the same success shape and `ok: true`. The `cache_write_failed` diagnostic is observed only in Tail logs, NOT in the response body. The issue exists on GitHub; the only thing that failed is the post-create idempotency-cache upgrade. Client retries within the 30s inflight window will see 409 `duplicate_inflight` instead of the cached 200, but the user has already received the real issue URL on the first call and never sees this failure mode in the dialog.

#### 404 Not Found — wrong path or method

```json
{
  "ok": false,
  "error": "not_found",
  "message": "Not found."
}
```

Returned for any non-`OPTIONS` request whose method/path is neither `POST /feedback` nor `GET /health`. The Worker handles `OPTIONS` at the top of its dispatch (before pathname matching), so any `OPTIONS` request — `/feedback`, `/health`, or any other path — receives a 204 preflight response, never a 404. This keeps CORS preflight behaviour uniform across the routes the browser cares about and matches `feedback-worker/src/index.ts`. Same JSON shape as other errors so clients have a uniform parser.

## `OPTIONS /feedback`

### Request
- Method: `OPTIONS`
- Headers: any browser-sent preflight headers

### Response
- Status: `204 No Content`
- Headers:
  - `Access-Control-Allow-Origin: *`
  - `Access-Control-Allow-Methods: POST, OPTIONS, GET`
  - `Access-Control-Allow-Headers: Content-Type`
  - `Access-Control-Max-Age: 86400`

## `GET /health`

### Request
- Method: `GET`

### Response
- Status: `200 OK`
- Body: `{"ok": true, "version": "<worker-deploy-sha>"}`

Purpose: allow anyone (including the maintainer's monitoring) to poll without side effects. Never creates issues. Never consumes rate-limit budget.

## CORS contract

All responses MUST include:
- `Access-Control-Allow-Origin: *`
- `Vary: Origin`

This allows reports served from any origin (including `file://` which sends `Origin: null`) to read the response.

## Caching contract

All responses MUST include `Cache-Control: no-store`. We never want intermediate caches returning a stale success (e.g., an issue URL that was later deleted).

## Request size limit

The Worker MUST reject requests with body byte length > `30_000_000` (≈ 20 MB base64 voice + 7 MB base64 image + overhead) with `413 Payload Too Large`. The size check is streamed — the reader aborts once the running byte count crosses the cap so an oversized upload can't be fully buffered into the isolate before we notice. Content-Length (when present) is a cheap fast-reject pre-flight; the authoritative check is the streaming byte count.

## Issue creation contract

On a 200 response the Worker has just created an issue at `matias-sanchez/My-gather` via the GraphQL `createIssue` mutation (NOT the REST endpoint — see data-model.md §"GitHub GraphQL mutation" for the rationale and the exact mutation shape) with:

- `title` set to the user's title (after trim).
- `body` assembled per `data-model.md` §"Issue body markdown composition": optional `> Category:` blockquote, user body, `### Attached screenshot` section if image present, `### Attached voice note` section with bare R2 URL if voice present, footer attribution.
- `labelIds` resolved from the names: `user-feedback` and `needs-triage` always, plus `area/<lower(category)>` (e.g. `category: "UI"` → label `area/ui`) when `category` is present. Missing labels are silently skipped (graceful degradation per Principle III) — a repo without `area/parser` still gets the issue, just without the area routing.
- No assignee, no milestone, no project. Triage is left to the maintainer (the `needs-triage` label is the signal for the maintainer's queue).

The Worker MUST NOT comment on the issue, react, lock it, or modify it after creation. One issue, one mutation, done.

## Idempotency contract

If the same `idempotencyKey` is submitted twice within 5 minutes and the first call produced a 200 response, the second call MUST return the same 200 response (same `issueUrl`, same `issueNumber`) without creating a new issue.

If the first call produced a 4xx/5xx, the second call is treated as a fresh request.

## Versioning

The API is at implicit v1. If/when a breaking change is needed, the endpoint path changes to `/v2/feedback`; old reports continue to POST to `/feedback` and get the v1 behaviour.

## Observability contract

Every request produces a single Tail log entry:

```json
{
  "timestamp":        "2026-04-24T08:30:00.000Z",
  "status":           200,
  "error":            null,
  "duration_ms":      287,
  "ip_hash":          "sha256-...",
  "issue_id":         "I_kwDOPNQ",
  "issue_number":     67,
  "report_version":   "v0.3.1-54-g29734aa",
  "rate_limit_count": 2,
  "has_image":        true,
  "has_voice":        false
}
```

User content (title, body, attachment bytes, IP) is NEVER logged.
