# Phase 1 Quickstart — Feedback Backend Worker

End-to-end walkthrough to deploy the Worker, wire the report, and verify.

## Prerequisites

- Cloudflare account (free tier — no credit card needed).
- GitHub account that owns `matias-sanchez/My-gather`.
- Node.js 20.x and `npm`.
- The `wrangler` CLI: `npm install -g wrangler` (or use the npx form).

## Phase 1 — One-time setup (human intervention required)

### Step 1.1 — Create the GitHub App

1. Go to `https://github.com/settings/apps/new`.
2. Fill:
   - **App name**: `My-gather Feedback` (unique across GitHub — add a suffix if taken)
   - **Homepage URL**: `https://github.com/matias-sanchez/My-gather`
   - **Webhook**: **uncheck "Active"** — we don't need webhooks
   - **Repository permissions → Issues**: `Read and write`
   - **Where can this GitHub App be installed**: **Only on this account**
3. Click **Create GitHub App**. Note the `App ID` (numeric, e.g. `234567`) shown on the settings page.
4. Scroll down, click **Generate a private key**. A `.pem` file downloads. Keep it — we'll upload it to Cloudflare in Step 1.4.
5. Click **Install App** in the left sidebar. Install on `matias-sanchez` → "Only select repositories" → pick `My-gather`.
6. After install, the URL is `https://github.com/settings/installations/<ID>`. Note the `<ID>` — this is the `GITHUB_INSTALLATION_ID`.

### Step 1.2 — Create the five labels in the repo

The Worker applies labels by *name* in `POST /repos/.../issues`. The names must already exist on the repo or GitHub returns 422. One-time, with a PAT that has `repo` scope (or run from the GitHub UI):

```bash
gh label create feedback        --color "0E8A16" --description "Submitted via Report Feedback dialog"
gh label create area:ui         --color "1D76DB" --description "Category: UI"
gh label create area:parser     --color "1D76DB" --description "Category: Parser"
gh label create area:advisor    --color "1D76DB" --description "Category: Advisor"
gh label create area:other      --color "1D76DB" --description "Category: Other"
```

Verify with `gh label list | grep -E '^(feedback|area:)'` — five rows expected.

### Step 1.3 — Cloudflare setup

1. Sign up at `https://dash.cloudflare.com/sign-up` (if needed).
2. `wrangler login` — opens browser OAuth flow.
3. `wrangler kv namespace create FEEDBACK_RATELIMIT` — note the returned `id`.
4. `wrangler kv namespace create FEEDBACK_IDEMP` — note the `id`.
5. `wrangler r2 bucket create feedback-attachments` — enable public access in the R2 dashboard, record the public URL prefix (e.g. `https://pub-abc123.r2.dev`).

### Step 1.4 — Worker secrets

From inside `feedback-worker/`:

```bash
wrangler secret put GITHUB_APP_ID               # paste App ID
wrangler secret put GITHUB_INSTALLATION_ID      # paste install ID
wrangler secret put R2_PUBLIC_URL_PREFIX        # paste R2 public URL
wrangler secret put GITHUB_APP_PRIVATE_KEY < ~/Downloads/my-gather-feedback.*.pem
```

Four secrets total. The previous design also stored `GITHUB_REPO_ID` and `GITHUB_CATEGORY_ID` (GraphQL node IDs); the REST flow does not need either — the repo is identified by `:owner/:repo` in the URL (config var, see next step) and the labels are matched by name.

### Step 1.5 — `wrangler.toml`

Update `feedback-worker/wrangler.toml` with the KV + R2 bindings and the public config vars:

```toml
name = "my-gather-feedback"
main = "src/index.ts"
compatibility_date = "2026-04-24"

[vars]
GITHUB_REPO       = "matias-sanchez/My-gather"
FEEDBACK_LABEL    = "feedback"
AREA_LABEL_PREFIX = "area:"

[[kv_namespaces]]
binding = "FEEDBACK_RATELIMIT"
id = "<from step 1.3>"

[[kv_namespaces]]
binding = "FEEDBACK_IDEMP"
id = "<from step 1.3>"

[[r2_buckets]]
binding = "FEEDBACK_ATTACHMENTS"
bucket_name = "feedback-attachments"
```

The `[vars]` block is checked into the repo. None of these values are secrets (the repo is public; the labels are visible in the UI). Keeping them out of source means a fork can repoint the Worker to its own repo with one line and no rebuild.

### Step 1.6 — First deploy

```bash
cd feedback-worker/
npm install
npm run test        # unit tests pass
npm run deploy      # wrangler deploy, prints the Worker URL
```

Record the Worker URL (e.g., `https://my-gather-feedback.<account>.workers.dev`). This is the value that goes into `render/feedback.go` as `feedbackWorkerURL`.

## Phase 2 — Wire the report

1. Update `render/feedback.go` → `feedbackWorkerURL = "<Worker URL>"`.
2. Rebuild: `make build`.
3. Regenerate goldens if needed: `go test ./render/... -update` (review the diff — should be one line: the new `data-feedback-worker-url` attribute).

## Phase 3 — End-to-end verification

### Scenario 1 — Happy path text-only

1. `bin/my-gather -root testdata/example2 -out /tmp/report-feedback.html`.
2. Open `/tmp/report-feedback.html` in Chrome.
3. Click "Report feedback". Fill title + body. Submit.
4. Within 3 seconds, dialog shows success with a link.
5. Click the link → GitHub shows the new issue with the exact title and body, tagged `feedback` (and `area:<x>` if a category was selected).
6. Close (or delete, if you have the rights) the test issue when done.

### Scenario 2 — With image + voice

Same as 1 but paste a screenshot and record 5 seconds of audio before Submit. Verify the issue on GitHub shows the image inline and the audio player.

### Scenario 3 — Worker down (fallback)

`wrangler dev` stopped → deploy an erroring version temporarily (`throw new Error("test")` at top of handler) → Submit from the report → expect the dialog to fall back to the `window.open` pre-fill URL flow from feature 002. Revert the Worker.

### Scenario 4 — Rate limit

```bash
for i in 1 2 3 4 5 6; do
  curl -X POST https://<worker-url>/feedback \
    -H 'Content-Type: application/json' \
    -d '{"title":"test '$i'","body":"","idempotencyKey":"'$(uuidgen)'","reportVersion":"test"}'
  echo
done
```

Requests 1–5: `{"ok": true, ...}`. Request 6: `{"ok": false, "error": "rate_limit", ...}` with `Retry-After` header.

### Scenario 5 — Validation errors

POST with `title: ""` → 400 `title_required`.
POST with a 20 MB image (base64 of garbage) → 400 `image_too_large`.
POST with `image.mime: "image/bmp"` → 400 `image_bad_mime`.
POST with a 30 MB voice blob (base64 of garbage) → 400 `voice_too_large`.
POST with `voice.mime: "audio/midi"` → 400 `voice_bad_mime`.
POST with `category: "Networking"` (not in the enum) → 400 `category_invalid`.
POST with `idempotencyKey: "not-a-uuid"` → 400 `idempotency_key_invalid`.
POST with malformed JSON → 400 `malformed_payload`.

## Tear-down (if abandoning the feature)

1. Delete the Worker: `wrangler delete my-gather-feedback`.
2. Delete KV namespaces + R2 bucket in the Cloudflare dashboard.
3. Uninstall the GitHub App from the repo.
4. Revert the Go-side changes that point at the Worker URL.

## Operational tasks (steady-state)

- **Rotate GitHub App private key**: yearly. Regenerate on App settings page, `wrangler secret put GITHUB_APP_PRIVATE_KEY`, redeploy.
- **Monitor Worker error rate**: Cloudflare dashboard → Workers → Logs. If error rate > 5% over an hour, investigate.
- **GitHub API quota**: ~5000 req/hr per installation. Nowhere near our traffic; no alerting needed.
