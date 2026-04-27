# Phase 1 Contract — Report Feedback Dialog UI

This contract is the canonical source of truth for the dialog's DOM
shape, state machine, and CSS hooks. It is the spec the Go-side render
tests assert against (`render/report_feedback_test.go`) and the spec
the browser-side `app.js` is implemented from. Changes here must
land together with their template + JS counterparts in the same PR.

## Scope

The Report Feedback dialog is the existing feature-002 dialog,
extended to call the Worker on Submit (this feature, 003). The
fallback path (feature-002 `window.open`) MUST remain reachable and
functionally identical when the Worker call fails.

## DOM structure

The dialog has TWO top-level visible regions: the form (default,
contains all inputs and inline error/fallback nodes) and the success
view (shown after a 200 from the Worker). The form is a single
`<form>` element; transient states (submitting, validation error,
throttle, fallback) are surfaced INLINE inside the form via
hidden-by-default `<p>` elements that JS toggles. There is no
separate `<section>` per state — the design is intentionally flat,
because every error path returns control to the same form so the user
can fix and retry without re-entering the dialog.

```html
<dialog id="feedback-dialog"
        class="feedback-dialog"
        data-feedback-url="{{ .Feedback.GitHubURL }}"
        data-feedback-worker-url="{{ .Feedback.WorkerURL }}"
        aria-labelledby="feedback-title"
        aria-describedby="feedback-help">

  <!-- FORM region: the default, visible until a 200 lands. -->
  <form id="feedback-form" method="dialog">
    <header class="feedback-dialog-header">
      <h2 id="feedback-title">Report feedback</h2>
      <button type="button" class="feedback-close-x" id="feedback-cancel" aria-label="Close">×</button>
    </header>
    <p id="feedback-help" class="feedback-help">
      Posts a new issue to GitHub. You can paste an image and record a voice note to attach them.
    </p>

    <div class="feedback-field">
      <label for="feedback-field-title">Title</label>
      <input id="feedback-field-title" name="title" type="text" autocomplete="off"
             placeholder="Short summary…" required>
    </div>

    <div class="feedback-field">
      <label for="feedback-field-body">Body</label>
      <textarea id="feedback-field-body" name="body" rows="5"
                placeholder="Describe the idea or issue. Markdown supported on GitHub."></textarea>
    </div>

    <div class="feedback-field feedback-field--row">
      <label for="feedback-field-category">
        Category <span class="feedback-field-opt">optional</span>
      </label>
      <select id="feedback-field-category" name="category">
        <option value="">—</option>
        {{- range $i, $c := .Feedback.Categories }}
        <option value="{{ $c }}">{{ $c }}</option>
        {{- end }}
      </select>
    </div>

    <!-- Image + voice attachments (paste or record) — feature 002. -->
    <div class="feedback-attachments" id="feedback-attachments"></div>

    <!-- Inline transient slots, all hidden by default. -->
    <p class="feedback-hint"  id="feedback-hint"  hidden></p>
    <p class="feedback-error" id="feedback-error" hidden></p>
    <p class="feedback-fallback" id="feedback-fallback" hidden>
      <a id="feedback-fallback-link" href="#" target="_blank" rel="noopener noreferrer">
        Open GitHub manually
      </a>
    </p>

    <footer class="feedback-actions">
      <button type="button" class="feedback-record" id="feedback-record">🎙 Record voice</button>
      <span class="feedback-actions-spacer"></span>
      <button type="submit" class="feedback-submit" id="feedback-submit" disabled>
        Post to GitHub
      </button>
    </footer>
  </form>

  <!-- SUCCESS region: shown after a 200 response, hidden by default. -->
  <div class="feedback-success" id="feedback-success" hidden>
    <div class="feedback-success-icon" aria-hidden="true">✓</div>
    <h2>Feedback posted</h2>
    <p>Your issue has been created on GitHub.</p>
    <a id="feedback-success-link" class="feedback-success-link"
       href="#" target="_blank" rel="noopener noreferrer">View issue on GitHub →</a>
    <button type="button" class="feedback-success-close" id="feedback-success-close">Close</button>
  </div>

</dialog>
```

The two visible regions are mutually exclusive: when the Worker
returns 200 the JS hides `#feedback-form` and unhides
`#feedback-success`. Cancel/close from either region closes the
dialog.

## State machine

```text
                    cancel/dismiss
                  ┌────────────────────────────── closed ──┐
                  │                                        │
            ┌─────▼─────┐                                  │
  open ──►  │   form    │ ─── submit ──► (Worker call)     │
            └─────▲─────┘                    │             │
                  │                          │             │
        ┌─────────┴────────────┐  200        │             │
        │                      │ ◄───────────┤             │
        │ form (success view   │             │             │
        │  takes over)         │             │ 400 / 422   │
        └──────────────────────┘             ▼             │
                                  ┌──────────────────┐     │
                                  │ form (inline     │ ────┘
                                  │  error message)  │
                                  └──────────────────┘
                                                     ▲
                  ┌──────────────────────────────────┘
                  │   429
                  │ form (inline throttle hint, "Try again in N min")
                  │
                  │   5xx / 504 / network / 15s timeout
                  └─► fallback: window.open(url, "_blank", "noopener,noreferrer")
                       + show #feedback-fallback link inline
```

### State responsibilities

| Trigger | Visible region | Behavior |
|---|---|---|
| Dialog `open()` | `#feedback-form` | Focus moves to `#feedback-field-title`. `#feedback-error`, `#feedback-hint`, `#feedback-fallback` start hidden. |
| Submit click (valid form) | `#feedback-form` (no transition) | Disable `#feedback-submit`; mint `idempotencyKey = crypto.randomUUID()` once and keep it for any retry within the click. Optionally show `#feedback-hint` with "Posting…" copy. |
| Worker returned 200 | switch to `#feedback-success` | Hydrate `#feedback-success-link[href]` with `issueUrl` from the response. Auto-focus the link. NEVER auto-close. |
| Worker returned 400/422 | `#feedback-form` (with inline error) | Surface `#feedback-error` with the Worker's `message`. Re-enable `#feedback-submit`. Do NOT fall back. |
| Worker returned 429 | `#feedback-form` (with inline throttle) | Surface `#feedback-error` with "Rate limit reached. Try again in N minutes." (derived from `retryAfterSeconds` or the `Retry-After` header). Re-enable Submit immediately so the user can retry; the Worker enforces the limit server-side. |
| Worker returned 409 `duplicate_inflight` | `#feedback-form` (with inline error) | Surface `#feedback-error` with the Worker's `message` ("A previous submission with the same idempotency key is still being processed. Retry in a few seconds."). Re-enable Submit. |
| Worker returned 5xx (500/503/504), network error, browser timeout | Fallback | Call `window.open(prefillURL, "_blank", "noopener,noreferrer")` exactly once. `prefillURL` is built from the `data-feedback-url` attribute (which already carries the canonical `?labels=user-feedback,needs-triage` query string — see Determinism contract below) by setting `title` and `body` via `new URL(...).searchParams.set()` so duplicate keys are not introduced. If the popup is blocked, unhide `#feedback-fallback` so the user can click `Open GitHub manually`. The third `window.open` argument is mandatory — without `noopener` the GitHub tab gets `window.opener` access to the report (a known security regression). |

## JS contract — `app.js` `doSubmit`

The handler must:

1. Read the build-time constants from the dialog's data attributes:
   - `dialog.dataset.feedbackWorkerUrl` → POST endpoint.
   - `dialog.dataset.feedbackUrl`        → GitHub fallback base URL.
   The `reportVersion` payload field is read separately from
   `<meta name="generator">` in the document head, sliced to 64
   chars (the existing report-wide convention; the dialog does not
   carry its own copy).
2. Validate the form client-side (HTML5 validity + length caps from
   `contracts/api.md`). If invalid, stay in `form` with field-level
   ARIA hints. Do not call the Worker.
3. Build the JSON payload:
   - `title`, `body`, `category` from the form,
   - if image present, base64-encode via
     `await readAsDataURL(file)` and strip the `data:…;base64,` prefix,
   - if voice present, same,
   - `idempotencyKey = crypto.randomUUID()` minted ONCE per
     Submit-click,
   - `reportVersion` from `<meta name="generator">`.
4. `await fetch(workerURL, {method:"POST", body: JSON.stringify(payload),
   headers:{"Content-Type":"application/json"}, signal: ctrl.signal})`
   with `ctrl = new AbortController()` and a **15 000 ms** `setTimeout`
   arming `ctrl.abort()`. The 15s budget brackets the Worker's
   per-call 10s GitHub timeout (FR-014) so a slow GitHub call surfaces
   as a clean Worker-side 504 within the browser's window.
5. Branch on `response.status`:
   - `200`: parse `{ok, issueUrl, issueNumber}`; switch to the
     success view.
   - `400`: parse `{error, message}`; surface `#feedback-error`
     with the message inline; re-enable Submit.
   - `409`: same as 400 — duplicate-inflight is a transient
     application-layer error, not a fatal one.
   - `429`: parse `{retryAfterSeconds, message}`; surface
     `#feedback-error` with the throttle copy. Use `Retry-After`
     header if present (more authoritative than the body field).
   - `5xx`, `504`, network error, timeout: invoke the fallback path.
6. Reuse `idempotencyKey` if the user clicks Submit again from the
   inline-error state. Mint a new key only on a clean form
   transition (e.g., after a successful submit, then re-opening the
   dialog).

## Accessibility contract

- Dialog uses `<dialog>` element with `aria-labelledby` + `aria-describedby`.
- Focus on open: `#feedback-field-title` (the first input).
- Focus on success: `#feedback-success-link` (an `<a>` element, focusable by default — no `tabindex` needed).
- Focus on error: stays on `#feedback-submit` (re-enabled), the user reads `#feedback-error` and either fixes the offending field or hits Submit again.
- All buttons MUST be reachable by Tab in source order.

The dialog does NOT use a separate `<section>` per state, so heading-focus management (`tabindex="-1"` on `<h2>`) is not required — no `<h2>` is ever programmatically focused. Focus moves to `#feedback-field-title` on dialog open, to `#feedback-success-link` (an `<a>`, focusable by default) on success, and stays on `#feedback-submit` on error.

## CSS hooks

These class names are the public CSS contract; the stylesheet may
restyle freely but must not rename them without a migration:

- `.feedback-dialog` — outer dialog container.
- `.feedback-help` — neutral subtitle under the dialog title.
- `.feedback-field` — labelled form input wrapper (one per row).
- `.feedback-field-opt` — small "(optional)" annotation.
- `.feedback-attachments` — container for image preview + voice waveform widgets.
- `.feedback-hint` — neutral inline notice (gray text, small).
- `.feedback-error` — error-state inline message body.
- `.feedback-fallback` — fallback-link container (shown when popup blocked).
- `.feedback-success` — success-state container.
- `.feedback-success-icon` — green ✓ icon.
- `.feedback-success-link` — clickable link to the created issue.

## Determinism contract

The rendered HTML for the dialog MUST be byte-identical across two
renders of the same Go input. This implies:

- `data-feedback-worker-url` MUST be the build-time `feedbackWorkerURL` constant in `render/feedback.go`.
- `data-feedback-url` MUST be the build-time `feedbackGitHubURL` constant in `render/feedback.go` (which already pre-fills `?labels=user-feedback,needs-triage` so the fallback issue lands in the same triage bucket as Worker-posted ones).
- Categories list iterates over `.Feedback.Categories` (a slice deterministically copied from the package-level `feedbackCategories` slice — see `render/feedback.go` `BuildFeedbackView`).
- `idempotencyKey` is generated client-side at click time, NOT embedded in the HTML.
- `reportVersion` is read from `<meta name="generator">` at click time, also NOT embedded in the dialog markup.
- No timestamps, no random IDs, no ordering by Map iteration.

## Test contract

The Go-side render test (`render/report_feedback_test.go`) asserts:

1. The dialog has `data-feedback-worker-url` with the value of the `feedbackWorkerURL` constant.
2. The dialog has `data-feedback-url` with the value of the `feedbackGitHubURL` constant (which contains `labels=user-feedback,needs-triage`).
3. The dialog contains both regions: `#feedback-form` (visible by default — no `hidden` attribute) and `#feedback-success` (with `hidden`).
4. The success region contains an `<a id="feedback-success-link">` with `href="#"` (hydrated at runtime by JS) and `rel="noopener noreferrer"`.
5. The form contains `#feedback-error` and `#feedback-fallback` `<p>` elements, both with `hidden` by default.
6. The category `<select>` contains the four options (`UI`, `Parser`, `Advisor`, `Other`) plus the `—` placeholder, in the order they appear in `feedbackCategories`.

The browser-side state-machine behavior is exercised manually via
quickstart Phase 3 scenarios. Automating it would require a headless
browser harness, which the repo currently doesn't carry — out of
scope for this feature.

## Versioning

This contract is at v1. A v2 (e.g. multi-attachment support, draft
saving, separate transient regions per state) would coexist as
`contracts/ui-v2.md`; the dialog template would gain a new
`data-ui-version="2"` attribute and the JS would branch on it.
Until then, there is exactly one UI shape.
