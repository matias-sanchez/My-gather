# Phase 0 Research — Report Feedback Button

Resolves the handful of browser-API questions the feature surfaces, with decisions and rejected alternatives.

## R1: How to open a new tab with a pre-filled GitHub Discussion?

**Decision**: Use the URL pattern `https://github.com/matias-sanchez/My-gather/discussions/new?category=ideas&title=<enc>&body=<enc>`. Trigger navigation inside a synchronous click/keydown handler via `window.open(url, "_blank", "noopener,noreferrer")`. If the return value is `null` (popup blocker tripped), render an inline `<a href=... target="_blank" rel="noopener noreferrer">` the user can click manually.

**Rationale**:
- Pre-fill URL is GitHub's own documented behaviour (stable for multiple years).
- User-initiated `window.open` is the only combo that reliably escapes popup blockers across Chromium, Firefox, and Safari.
- `noopener,noreferrer` prevents the opened tab from accessing the report's `window.opener` (defence against navigation hijacks even though the origin is trusted).

**Alternatives considered**:
- **GitHub GraphQL API (create discussion)** — rejected: requires an auth token, which forces us either to ship a token (leak) or prompt the user for one (scope creep). Spec explicitly ruled both out.
- **`<form method=GET action=...>` auto-submit** — rejected: GitHub does not accept a GET form submission at the new-discussion URL; it expects the same URL as a link target.
- **Custom proxy endpoint in the binary** — rejected: violates Principle IX (zero network at runtime).

## R2: How should the dialog be implemented?

**Decision**: Native HTML5 `<dialog>` element, opened via `dialog.showModal()` and closed via `dialog.close()`. Use its built-in `::backdrop` pseudo-element for the modal overlay. Focus trap is native. Escape-key close is native.

**Rationale**:
- Supported in Chrome 37+, Firefox 98+, Safari 15.4+ — all versions older than "two most recent releases" per spec (SC-006).
- Built-in focus trap satisfies FR-010 without a custom trap; built-in Escape-close satisfies FR-005.
- Zero JS needed for open/close baseline behaviour; keeps the asset footprint small (Principle X).

**Alternatives considered**:
- **Role-based div with custom focus trap** — rejected: reinvents three accessibility behaviours the native dialog provides for free.
- **Dedicated page / route** — rejected: violates the single-HTML-file contract (Principle V).

## R3: How to capture a pasted image?

**Decision**: Attach a `paste` event listener on the dialog element. Inside the handler, iterate `event.clipboardData.items` and pick the first item with `type.startsWith("image/")`. Call `item.getAsFile()` to get a `Blob`. Render a thumbnail via `URL.createObjectURL(blob)` into an `<img>` preview. Revoke the object URL in the Remove handler and on dialog close (to free memory).

**Rationale**:
- `paste` events carry clipboard image data in every evergreen browser.
- `URL.createObjectURL` is the idiomatic zero-copy thumbnail path; no base64 round-trip.

**Alternatives considered**:
- **Async clipboard read (`navigator.clipboard.read`)** — rejected: requires an explicit user gesture + permission prompt, which is a worse UX than "just paste".
- **File input** — rejected: paste-to-capture is the point; file picker is a worse UX for screenshots that already live on the clipboard.

## R4: How to record voice?

**Decision**: Use `navigator.mediaDevices.getUserMedia({ audio: true })` + `MediaRecorder`. Start/stop buttons update a single recording state machine. Collect `dataavailable` chunks into an array, concatenate on stop into a single Blob. Recording cap at 120 seconds enforced by `setTimeout` triggering the stop logic. Elapsed-time counter driven by `requestAnimationFrame` (monotonic, not `setInterval`).

**Format**: accept whatever `MediaRecorder` default produces — `audio/webm;codecs=opus` on Chromium/Firefox, `audio/mp4` on Safari. Don't pass a `mimeType` option; trust the default. The download filename extension matches the recorder's `mimeType` (`.webm` or `.mp4`).

**Rationale**:
- MediaRecorder is the only offline voice-capture primitive in browsers.
- GitHub's discussion body accepts both `.webm` and `.mp4` drag-drops.
- Not pinning a codec avoids a hard dependency on Opus availability on Safari.

**Alternatives considered**:
- **Bundle a JS audio codec (e.g. recorder.js)** — rejected: bloats the bundle and adds a dependency (Principle X).
- **WebAudio ScriptProcessor → raw WAV download** — rejected: larger files for worse quality; codec-less WAV upload is just as good but 10× bigger.

## R5: How to hand off media from the dialog to the new GitHub tab?

**Decision**: Two-path handoff, both triggered from within the same Submit click handler (which also calls `window.open`):

1. **Image**: call `navigator.clipboard.write([new ClipboardItem({ "image/png": blob })])`. If the promise rejects (permission, API unsupported, or Firefox on HTTP), render an inline `<a href="<object URL>" download="...">Download image</a>` next to a one-line note.
2. **Voice**: create an anchor `<a href="<object URL>" download="feedback-voice-<short>.webm">` and programmatically `.click()` it. Do this inside the same user-gesture handler so browsers don't block the download.

**Rationale**:
- Clipboard API is the only way to put rich (non-text) data on the clipboard. It requires a user gesture — the Submit click provides that.
- Programmatic download via `<a download>` is a well-established pattern; it requires a user gesture on all current browsers and the Submit click satisfies it.

**Alternatives considered**:
- **Base64 inline in the URL body** — rejected: GitHub caps URLs around 8 KB; even a 300 × 200 screenshot blows past that.
- **Drag-drop from the dialog into the new tab** — rejected: cross-window drag-drop is unreliable across browsers and breaks when the tab is in a different browser profile or window.
- **Server-side upload + markdown link in body** — rejected: requires a server (Principle IX).

## R6: What is a short stable hash for the voice filename?

**Decision**: Compute a 6-character random id at download-time using `crypto.randomUUID().slice(0, 8)`. This id is *only* in the download filename — **it does not appear in any markup rendered by the Go binary**, so it doesn't affect HTML determinism (Principle IV). The id's only purpose is to avoid filename collisions when a user records several notes in a row on the same machine.

**Rationale**:
- Filename is browser-local, user-facing, and incidental. Not an artifact of the report.
- Deterministic HTML output is preserved — the report HTML contains no recording id at build time; the id is minted only when the user clicks Submit.

**Alternatives considered**:
- **Fixed filename** — rejected: collision risk on re-record.
- **Wall-clock timestamp** — rejected: leaks clock on the download filename, minor privacy concern; also less unique across parallel browser tabs.

## R7: Category taxonomy

**Decision**: Small fixed list shipped in the binary as a package-level `var` in `render/feedback.go`:

```go
var categoriesInOrder = []string{
    "UI",
    "Parser",
    "Advisor",
    "Other",
}
```

Template ranges over the slice with `{{range $i, $c := .Categories}}...{{end}}` — integer index iteration, deterministic order. JS uses the same list injected via a `data-*` attribute that is itself constant across runs.

**Rationale**:
- Four buckets cover the bulk of feedback types observed in the project's discussions so far.
- A `var` slice keeps enforcement on the Go side; avoids JSON decoding on the client.

**Alternatives considered**:
- **Free-text category** — rejected: defeats the triage-marker goal; users will invent overlapping buckets.
- **Derive categories from GitHub labels** — rejected: requires an API call (Principle IX).

## R8: Focus-ring and keyboard accessibility expectations

**Decision**:
- Button is a real `<button type="button">` element (native focusable; Tab order is natural).
- Dialog uses native `<dialog>` (native focus trap).
- Keyboard shortcuts:
  - Enter on the button → open dialog.
  - Escape inside dialog → close dialog (native).
  - Enter inside the title `<input>` → submit (form convention).
  - Cmd/Ctrl+Enter anywhere in dialog → submit (GitHub-editor parity, per clarification A).
  - Enter inside `<textarea>` → newline (native textarea convention).

**Rationale**:
- All behaviours are either native or a single key handler on `keydown`. No reimplementation of focus management.

**Alternatives considered**:
- **Custom focus trap** — rejected (see R2).

## R9: Accessibility labelling

**Decision**:
- Button: `aria-label="Report feedback — file an idea on GitHub"` and a short visible label "Report feedback". Visible label is primary; aria-label reinforces for screen readers.
- Dialog: `aria-labelledby` pointing at the dialog's `<h2>` title and `aria-describedby` pointing at the short help text.

**Rationale**: Matches WAI-ARIA Authoring Practices for dialogs. No surprises.

## Outcome

All unknowns are resolved. No `NEEDS CLARIFICATION` markers remain. Feature is implementation-ready and constitution-clean.
