# Phase 1 Contract — UI

My-gather has no HTTP or CLI interface for this feature, but it does expose a **UI contract** to human readers. This document specifies the contract so later review rounds can gate on it.

## DOM contract

### The header control

```html
<button type="button"
        class="feedback-btn"
        id="feedback-open"
        aria-haspopup="dialog"
        aria-controls="feedback-dialog"
        aria-label="Report feedback — file an idea on GitHub">
  <span class="feedback-btn-label">Report feedback</span>
</button>
```

- **Position**: inside `<header class="app-header">`, after the `<div class="meta">` child, as the last element of the header flex row.
- **Tab order**: natural — after the header metadata, before the first `<details class="section">`.
- **Visible text**: "Report feedback" (no icon-only; icon-only would hurt discoverability for first-time users during an incident).
- **Deterministic ID**: `id="feedback-open"` — fixed, never generated.

### The dialog

```html
<dialog id="feedback-dialog"
        class="feedback-dialog"
        aria-labelledby="feedback-title"
        aria-describedby="feedback-help">
  <form id="feedback-form" method="dialog">
    <h2 id="feedback-title">Report feedback</h2>
    <p id="feedback-help">
      This opens a new GitHub Discussion in the Ideas category.
      Submitted content is public.
    </p>

    <label for="feedback-field-title">Title</label>
    <input id="feedback-field-title" name="title" type="text" required>

    <label for="feedback-field-body">Body</label>
    <textarea id="feedback-field-body" name="body" rows="6"></textarea>

    <label for="feedback-field-category">Category (optional)</label>
    <select id="feedback-field-category" name="category">
      <option value="">—</option>
      <!-- options injected by template from .Feedback.Categories -->
    </select>

    <div class="feedback-attachments" id="feedback-attachments">
      <!-- populated at runtime by JS; see Attachment contract below -->
    </div>

    <div class="feedback-actions">
      <button type="button" class="feedback-record">Record voice note</button>
      <button type="button" class="feedback-cancel">Cancel</button>
      <button type="submit" class="feedback-submit" disabled>Submit</button>
    </div>

    <p class="feedback-hint" id="feedback-hint" hidden></p>
  </form>
</dialog>
```

- **ID convention**: all IDs start with `feedback-`. No ambiguity with other report sections.
- **Submit button**: starts `disabled`; JS enables when title is non-empty and URL length ≤ 7500.
- **Form method**: `method="dialog"` — pressing Enter inside the title input closes the dialog with the form's submit event, which JS intercepts to trigger the handoff instead.

### Attachments area (runtime-generated, not in template)

```html
<!-- When an image is attached: -->
<div class="feedback-attachment feedback-attachment--image">
  <img class="feedback-thumb" src="blob:...">
  <button type="button" class="feedback-attachment-remove" aria-label="Remove image">×</button>
</div>

<!-- When a voice recording exists: -->
<div class="feedback-attachment feedback-attachment--voice">
  <audio class="feedback-audio" controls src="blob:..."></audio>
  <button type="button" class="feedback-attachment-remove" aria-label="Remove voice note">×</button>
</div>

<!-- When recording is in progress: -->
<div class="feedback-attachment feedback-attachment--recording">
  <span class="feedback-recording-dot" aria-hidden="true"></span>
  <span class="feedback-recording-time">0:07 / 2:00</span>
  <button type="button" class="feedback-record-stop">Stop</button>
</div>
```

## Keyboard contract

| Key | Context | Action |
|---|---|---|
| Enter, Space | focus on `#feedback-open` | open dialog |
| Escape | anywhere in dialog | close dialog (native) |
| Tab | anywhere in dialog | focus traps within dialog (native) |
| Enter | `#feedback-field-title` | submit (if Submit is not disabled) |
| Cmd/Ctrl+Enter | anywhere in dialog | submit (if Submit is not disabled) |
| Enter | `#feedback-field-body` | insert newline (native textarea) |

## Submission contract

On Submit (whether via button click, Cmd/Ctrl+Enter, or Enter-in-title):

1. Validate: title non-empty, URL length after encoding ≤ 7500. On fail, surface inline message, do nothing else.
2. Compute final URL: `GitHubURL + "&title=" + enc(title) + "&body=" + enc(maybePrefixCategory(body, category))`.
3. Call `window.open(url, "_blank", "noopener,noreferrer")`. If result is null, render an inline "Click to open GitHub" anchor and return (do not proceed to clipboard/download — the user hasn't actually landed on GitHub yet).
4. If an image is attached: `await navigator.clipboard.write([new ClipboardItem({"image/png": imageBlob})])`. On rejection, render an inline "Clipboard write blocked — use this download" link to the image blob.
5. If a voice recording is attached: create `<a href="<object URL>" download="feedback-voice-<id>.<ext>">` appended to the dialog, `.click()` it, then `remove()` it. The `<id>` is `crypto.randomUUID().slice(0, 8)`; the `<ext>` matches the blob's MIME type (`webm` or `mp4`).
6. Do not close the dialog automatically. The user may want to copy-paste from the dialog if GitHub's tab didn't land where expected. Leave the dialog open until Cancel / Escape / backdrop click.

## Determinism contract (Go side)

The Go template renders **only** constants and `.Feedback.*` values (themselves constants). No `time.Now`, no `rand`, no map iteration, no unspecified slice order. Verified by:

- `render/feedback_test.go::TestBuildFeedbackViewIsDeterministic` — two calls `DeepEqual`.
- The existing determinism golden suite — same report rendered twice, byte-identical.

## Self-contained contract (Principle V)

No new external asset. The only network reference in the emitted HTML is the `https://github.com/...` link, which is an anchor target for a user-initiated click — not a resource the page fetches. Verified by:

- Manual `grep` in the golden output for `src=`, `href=`, `url(`, `@import`, `fetch(`, `XMLHttpRequest`, `new URL(` against anything other than `blob:`, `data:`, `#`, or the single GitHub URL.
- A lint test in `tests/lint/` can be extended to assert no new external URLs slipped in, if regression risk warrants it (deferred to tasks phase).
