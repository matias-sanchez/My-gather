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
   - **Repository permissions → Discussions**: `Read and write`
   - **Where can this GitHub App be installed**: **Only on this account**
3. Click **Create GitHub App**. Note the `App ID` (numeric, e.g. `234567`) shown on the settings page.
4. Scroll down, click **Generate a private key**. A `.pem` file downloads. Keep it — we'll upload it to Cloudflare in Step 1.4.
5. Click **Install App** in the left sidebar. Install on `matias-sanchez` → "Only select repositories" → pick `My-gather`.
6. After install, the URL is `https://github.com/settings/installations/<ID>`. Note the `<ID>` — this is the `GITHUB_INSTALLATION_ID`.

### Step 1.2 — Fetch repo + category GraphQL IDs

```bash
# Run with a PAT that has `read:discussion` scope (this is separate from the App).
# One-time only; not needed at runtime.
gh api graphql -f query='
  query($owner: String!, $name: String!) {
    repository(owner: $owner, name: $name) {
      id
      discussionCategories(first: 20) {
        nodes { id name slug }
      }
    }
  }' -f owner=matias-sanchez -f name=My-gather
```

Record:
- `repository.id` → `GITHUB_REPO_ID`
- `discussionCategories.nodes[n].id` where `slug == "ideas"` → `GITHUB_CATEGORY_ID`

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
wrangler secret put GITHUB_REPO_ID              # paste repository node ID
wrangler secret put GITHUB_CATEGORY_ID          # paste ideas category node ID
wrangler secret put R2_PUBLIC_URL_PREFIX        # paste R2 public URL
wrangler secret put GITHUB_APP_PRIVATE_KEY < ~/Downloads/my-gather-feedback.*.pem
```

### Step 1.5 — `wrangler.toml`

Update `feedback-worker/wrangler.toml` with the KV + R2 bindings:

```toml
name = "my-gather-feedback"
main = "src/index.ts"
compatibility_date = "2026-04-24"

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
5. Click the link → GitHub shows the new discussion with the exact title and body.
6. Delete the test discussion from GitHub when done.

### Scenario 2 — With image + voice

Same as 1 but paste a screenshot and record 5 seconds of audio before Submit. Verify the discussion on GitHub shows the image inline and the audio player.

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
