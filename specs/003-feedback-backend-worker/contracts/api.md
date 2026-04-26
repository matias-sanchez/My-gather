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
  - `base64` decodes to ≤ 10 485 760 bytes
- `idempotencyKey`: 36-char UUID v4 (`/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i`)
- `reportVersion`: opaque, ≤ 64 chars

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
- `malformed_payload`

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

On a 200 response the Worker has just created an issue at `matias-sanchez/My-gather` with:

- `title` set to the user's title (after trim).
- `body` assembled per `data-model.md` (user body, then image markdown if present, then audio markdown if present, then footer).
- `labels` set to `["feedback"]` always, plus `"area:<lower(category)>"` when `category` is present in the payload (e.g. `category: "UI"` → label `area:ui`).
- No assignee, no milestone, no project. Triage is left to the maintainer.

The Worker MUST NOT comment on the issue, react, lock it, or modify it after creation. One issue, one POST, done.

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
