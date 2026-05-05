# Phase 1 Quickstart — Feedback Backend Worker

End-to-end walkthrough to deploy the Worker, wire the report, and verify.

## Prerequisites

- Cloudflare account (free tier — no credit card needed).
- GitHub account that owns `matias-sanchez/My-gather`.
- Node.js 22.x and `npm`.
- The `wrangler` CLI: `npm install -g wrangler` (or use the npx form).

## Phase 1 — One-time setup (human intervention required)

### Step 1.1 — Create the GitHub App

**Status**: this App was created on 2026-04-24 and is installed on `matias-sanchez/My-gather`. Re-do this step only when forking the project or rotating credentials.

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

### Step 1.2 — Create the six labels in the repo

The Worker resolves labels by *name* via the GraphQL `repository.labels` lookup (cached 24 h in KV) and passes the resolved label IDs to `createIssue`. The names must already exist on the repo or the lookup yields no ID. One-time, with a PAT that has `repo` scope (or run from the GitHub UI):

```bash
gh label create user-feedback   --color "0E8A16" --description "Submitted via Report Feedback dialog"
gh label create needs-triage    --color "FBCA04" --description "Awaiting maintainer review"
gh label create area/ui         --color "1D76DB" --description "Category: UI"
gh label create area/parser     --color "1D76DB" --description "Category: Parser"
gh label create area/advisor    --color "1D76DB" --description "Category: Advisor"
gh label create area/other      --color "1D76DB" --description "Category: Other"
```

Verify with `gh label list | grep -E '^(user-feedback|needs-triage|area/)'` — six rows expected.

### Step 1.3 — Cloudflare setup

**Status**: deployed since 2026-04-24. The two KV namespaces and the R2 bucket below already exist and are wired into `feedback-worker/wrangler.toml`. Re-do these steps only when forking.

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
wrangler secret put GITHUB_REPO_ID              # paste repository node ID (GraphQL createIssue requires it)
wrangler secret put R2_PUBLIC_URL_PREFIX        # paste R2 public URL
wrangler secret put GITHUB_APP_PRIVATE_KEY < ~/Downloads/my-gather-feedback.*.pem
```

Five secrets total. The deployed Worker uses GraphQL `createIssue`, which takes a repository **node ID** (`R_kgDO…`) — not an `:owner/:repo` slug — so `GITHUB_REPO_ID` is required. Look it up once with `gh api graphql -f query='{ repository(owner:"matias-sanchez", name:"My-gather") { id } }'`. The private key must be PKCS#8 (`BEGIN PRIVATE KEY`) — `jose` imports that natively. Label names live in `feedback-worker/src/index.ts`, not in `[vars]`.

### Step 1.5 — `wrangler.toml`

The deployed `feedback-worker/wrangler.toml` is a concrete file checked into the repo (no `.template`). It carries the KV + R2 bindings, the `nodejs_compat` flag, and observability — and **no `[vars]` block**: label names live in `feedback-worker/src/index.ts` and are not configurable per-deploy. The shipped contents:

```toml
name = "my-gather-feedback"
main = "src/index.ts"
compatibility_date = "2026-04-24"
compatibility_flags = ["nodejs_compat"]
workers_dev = true

[[kv_namespaces]]
binding = "FEEDBACK_RATELIMIT"
id = "a0424545a9164b70bd0b8852ffd4de32"

[[kv_namespaces]]
binding = "FEEDBACK_IDEMP"
id = "e9d7dfd96131442d8d908c2b44691aa2"

[[r2_buckets]]
binding = "FEEDBACK_ATTACHMENTS"
bucket_name = "feedback-attachments"

[observability]
enabled = true
```

A fork that repoints to its own repo edits `index.ts` (label names) and `render/assets/feedback-contract.json` (Worker URL and GitHub fallback URL), then re-runs Steps 1.3 + 1.4 to provision its own KV / R2 / secrets — there is no `[vars]` shortcut.

### Step 1.6 — First deploy

```bash
cd feedback-worker/
npm install
npm run test        # unit tests pass
npm run deploy      # wrangler deploy, prints the Worker URL
```

Record the Worker URL (e.g., `https://my-gather-feedback.<account>.workers.dev`). This is the value that goes into `render/assets/feedback-contract.json` as `workerUrl`.

## Phase 2 — Wire the report

1. Update `render/assets/feedback-contract.json` → `workerUrl = "<Worker URL>"`.
2. Rebuild: `make build`.
3. Regenerate goldens if needed: `go test ./render/... -update` (review the diff — it should only reflect the embedded `data-feedback-contract` value).

## Phase 3 — End-to-end verification

### Scenario 1 — Happy path text-only

1. `bin/my-gather -root testdata/example2 -out /tmp/report-feedback.html`.
2. Open `/tmp/report-feedback.html` in Chrome.
3. Click "Report feedback". Fill title + body. Submit.
4. Within 3 seconds, dialog shows success with a link.
5. Click the link → GitHub shows the new issue with the exact title and body, tagged `user-feedback` and `needs-triage` (plus `area/<lower(category)>` when a category was selected).
6. Close (or delete, if you have the rights) the test issue when done.

### Scenario 2 — With image + voice

Same as 1 but paste a screenshot and record 5 seconds of audio before Submit. Verify the issue on GitHub shows the image inline and a `### Attached voice note` section with the bare R2 audio URL on its own line. GitHub renders that URL as an inline player when the rendering surface supports it; otherwise the URL remains clickable (see spec.md US2 acceptance scenario 2 + research R2 chosen flow).

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
POST with a 16 MB decoded voice blob (base64 of garbage) → 400 `voice_too_large` (the deployed cap is 15 728 640 bytes / 15 MB).
POST with `voice.mime: "audio/midi"` → 400 `voice_bad_mime`.
POST with `category: "Networking"` (not in the enum) → 400 `category_invalid`.
POST with `idempotencyKey: "not-a-uuid"` → 400 `idempotency_key_invalid`.
POST with `reportVersion: ""` (or missing) → 400 `report_version_invalid`.
POST with a 35 MB image → 413 `payload_too_large` (streaming size gate trips before validation).
POST with malformed JSON → 400 `malformed_payload`.

## Tear-down (if abandoning the feature)

1. Delete the Worker: `wrangler delete my-gather-feedback`.
2. Delete KV namespaces + R2 bucket in the Cloudflare dashboard.
3. Uninstall the GitHub App from the repo.
4. Revert the feedback contract changes that point at the Worker URL.

## Operational tasks (steady-state)

- **Rotate GitHub App private key**: yearly. Regenerate on App settings page, `wrangler secret put GITHUB_APP_PRIVATE_KEY`, redeploy.
- **Monitor Worker error rate**: Cloudflare dashboard → Workers → Logs. If error rate > 5% over an hour, investigate.
- **GitHub API quota**: ~5000 req/hr per installation. Nowhere near our traffic; no alerting needed.
