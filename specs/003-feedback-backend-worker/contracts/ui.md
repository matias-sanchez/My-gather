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

```html
<dialog id="feedback-dialog"
        data-feedback-worker-url="{{ .Feedback.WorkerURL }}"
        data-feedback-report-version="{{ .Feedback.ReportVersion }}"
        aria-labelledby="feedback-title"
        aria-describedby="feedback-desc">

  <!-- One of these regions is visible at a time; the others have hidden=""  -->

  <section id="feedback-form" data-state="form">
    <h2 id="feedback-title">Report feedback</h2>
    <p id="feedback-desc">Tell the maintainer what's broken or what could be better.</p>
    <form>
      <label>Title    <input  name="title" required maxlength="200"></label>
      <label>Body     <textarea name="body" maxlength="10240"></textarea></label>
      <label>Category
        <select name="category">
          <option value="">(none)</option>
          <option value="UI">UI</option>
          <option value="Parser">Parser</option>
          <option value="Advisor">Advisor</option>
          <option value="Other">Other</option>
        </select>
      </label>
      <label>Image    <input type="file" name="image" accept="image/png,image/jpeg,image/gif,image/webp"></label>
      <label>Voice    <button type="button" data-action="record">Record</button></label>
      <button type="submit" data-action="submit">Submit</button>
      <button type="button" data-action="cancel">Cancel</button>
    </form>
    <p class="feedback-inline-note" id="feedback-fallback-note" hidden></p>
  </section>

  <section id="feedback-submitting" data-state="submitting" hidden>
    <p>Posting feedback&hellip;</p>
    <progress aria-label="Submit in progress"></progress>
  </section>

  <section id="feedback-success" data-state="success" hidden>
    <h2>Feedback posted</h2>
    <p>Thanks. View on GitHub:
       <a id="feedback-success-link" href="" rel="noopener" target="_blank"></a></p>
    <button type="button" data-action="dismiss">Close</button>
  </section>

  <section id="feedback-error" data-state="error" hidden>
    <h2 tabindex="-1">Could not submit</h2>
    <p class="feedback-error-message"></p>
    <button type="button" data-action="retry">Try again</button>
    <button type="button" data-action="dismiss">Close</button>
  </section>

  <section id="feedback-throttle" data-state="throttle" hidden>
    <h2 tabindex="-1">Rate limit reached</h2>
    <p>Try again in <span class="feedback-countdown"></span>.</p>
    <button type="button" data-action="dismiss">Close</button>
  </section>

</dialog>
```

The `<section data-state="…">` regions are mutually exclusive: exactly
one is visible at any time. The state machine below governs which.

## State machine

```text
                    cancel/dismiss
                  ┌────────────────────────────── closed ──┐
                  │                                        │
            ┌─────▼─────┐  submit       ┌─────────────┐    │
  open ──►  │   form    │ ─────────────►│ submitting  │    │
            └─────▲─────┘               └──────┬──────┘    │
                  │ retry / fix                │           │
                  │                            │           │
        ┌─────────┴────────┐                   │           │
        │                  │                   │           │
        │        ┌─────────┴────┐    400/422   │           │
        │        │ form (with   │ ◄────────────┤           │
        │        │  inline err) │              │           │
        │        └──────────────┘              │ 200       │
        │                                      ▼           │
        │                                  ┌────────┐      │
        │                                  │success │ ─────┘
        │                                  └────────┘
        │   429
        ├──────────────────► throttle ─────────────────────┘
        │   5xx / network / timeout / 504
        └──────────────────► fallback (window.open) ──► form (inline note)
```

### State responsibilities

| State | Trigger | Mandatory behavior |
|---|---|---|
| `form` | Initial open; cancel from elsewhere | Focus moves to `input[name="title"]`. The fallback inline-note is hidden unless we just degraded from a Worker failure. |
| `submitting` | Submit click with valid form | Disable Submit button; show spinner; start 5s `AbortController` timer; mint `idempotencyKey` once and keep it for any retry. |
| `success` | Worker returned 200 | Hydrate `#feedback-success-link[href]` with `issueUrl` and `textContent` with the issue number (`#NN`). Auto-focus the link. NEVER auto-close. |
| `form` (with inline validation errors) | Worker returned 400/422 with a known validation `error` code, OR client-side validation failed pre-Submit | Surface inline messages adjacent to the offending field (title / body / image / voice). Re-enable Submit. Do NOT fall back. The `#feedback-error` region defined in the DOM is **reserved** in this Phase 1 contract — it has no JS-driven entry path and MUST remain `hidden` on every 400/422 response. |
| `throttle` | Worker returned 429 | Read `retryAfterSeconds`, render countdown updating each second. Disable Submit until 0; then return to `form`. |
| Fallback | Worker returned 5xx/504/network/timeout | Call the feature-002 `window.open(prefillURL, "_blank", "noopener,noreferrer")` exactly once with the same payload. The third argument is mandatory — without `noopener` the GitHub tab gets `window.opener` access to the report (a known security regression). Return to `form` with `#feedback-fallback-note` set to "Backend unavailable — opened GitHub with pre-filled form." |

## JS contract — `app.js` `doSubmit`

The handler must:

1. Validate the form client-side (HTML5 validity + length caps from
   `contracts/api.md`). If invalid, stay in `form` with field-level
   ARIA hints. Do not call the Worker.
2. Call `buildPayload()` (new helper) which:
   - reads `title`, `body`, `category` from the form,
   - if image present, base64-encodes via
     `await readAsDataURL(file)` and strips the `data:…;base64,` prefix,
   - if voice present, same,
   - mints `idempotencyKey = crypto.randomUUID()` ONCE per
     Submit-click (memoized on the dialog state),
   - reads `reportVersion` from the dialog's
     `data-feedback-report-version` attribute (rendered by the Go
     template, alongside `data-feedback-worker-url`). No `window`
     globals — the dialog is the canonical source of build-time
     constants.
3. Transition to `submitting`.
4. `await fetch(workerURL, {method:"POST", body: JSON.stringify(payload),
   headers:{"Content-Type":"application/json"}, signal: ctrl.signal})`
   with `ctrl = new AbortController()` and a 5000ms `setTimeout` arming
   `ctrl.abort()`.
5. Branch on `response.status`:
   - `200`: parse `{ok, issueUrl, issueNumber}`, transition to `success`.
   - `400`: parse `{error, message}`, set field-level hint, transition
     back to `form` with the message inline.
   - `429`: parse `{retryAfterSeconds, message}`, transition to
     `throttle`. Use `Retry-After` header if present (more
     authoritative than the body field).
   - `5xx`, network error, timeout: invoke the fallback path.
6. Reuse `idempotencyKey` if the user clicks Submit again from the
   error state. Mint a new key only on a clean form transition (e.g.,
   after a successful submit, then re-opening the dialog).

## Accessibility contract

- Dialog uses `<dialog>` element with `aria-labelledby` / `aria-describedby`.
- State changes MUST move focus deterministically:
  `form → submitting`: focus stays put (input field).
  `submitting → success`: focus moves to `#feedback-success-link` (an `<a>` element, focusable by default).
  `submitting → form (with inline validation errors)`: focus moves to the first invalid field.
  `submitting → throttle`: focus moves to the throttle heading. The heading MUST carry `tabindex="-1"` so JS can call `.focus()` on it (`<h2>` is not focusable by default).
  Fallback path: focus stays put after `window.open` returns; the inline `#feedback-fallback-note` becomes visible in the existing `form` region.
- Any heading the JS calls `.focus()` on MUST have `tabindex="-1"` in the markup; this includes the throttle heading and the heading inside `#feedback-error` (the latter is reserved in Phase 1 — see state-responsibilities table — but its heading must be focus-ready in case the contract grows in a later phase).
- The progress indicator MUST be `<progress>` (semantic), not a CSS
  spinner-only div.
- All buttons MUST be reachable by Tab in source order.

## CSS hooks

These class names are the public CSS contract; the stylesheet may
restyle freely but must not rename them without a migration:

- `.feedback-inline-note` — neutral notice (gray text, small).
- `.feedback-error-message` — error-state message body.
- `.feedback-countdown` — live-updating throttle countdown.
- `[data-state="success"]` — green accent border.
- `[data-state="error"]` — red accent border.
- `[data-state="throttle"]` — amber accent border.

## Determinism contract

The rendered HTML for the dialog MUST be byte-identical across two
renders of the same Go input. This implies:

- `data-feedback-worker-url` MUST be the build-time constant.
- `data-feedback-report-version` MUST also be a build-time constant
  (same value the binary already exposes via `--version`).
- `idempotencyKey` is generated client-side at click time, NOT
  embedded in the HTML.
- No timestamps, no random IDs, no ordering by Map iteration.

## Test contract

The Go-side render test (`render/report_feedback_test.go`) asserts:

1. The dialog has `data-feedback-worker-url` with the documented constant.
2. The dialog has `data-feedback-report-version` with the build-time version string (the same constant the binary's `--version` exposes); the assertion compares against the documented constant, not against `""` or `undefined`. A regression that drops this attribute breaks `reportVersion` in every Submit payload (FR-002).
3. The five state regions (`form`, `submitting`, `success`, `error`,
   `throttle`) all exist with the expected `data-state` attribute.
4. Initial render: only `#feedback-form` is visible (no `hidden`
   attribute); the other four have `hidden`.
5. The success region contains an `<a id="feedback-success-link">`
   with empty `href` (hydrated at runtime).

The browser-side state-machine behavior is exercised manually via
quickstart Phase 3 scenarios. Automating it would require a headless
browser harness, which the repo currently doesn't carry — out of
scope for this feature.

## Versioning

This contract is at v1. A v2 (e.g. multi-attachment support, draft
saving) would coexist as `contracts/ui-v2.md`; the dialog template
would gain a new `data-ui-version="2"` attribute and the JS would
branch on it. Until then, there is exactly one UI shape.
