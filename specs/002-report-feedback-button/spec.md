# Feature Specification: Report Feedback Button

**Feature Branch**: `002-report-feedback-button`
**Created**: 2026-04-24
**Status**: Draft
**Input**: User description: "Report Feedback Button — a small 'Report feedback' control anchored in the top-right of the pt-stalk diagnostic report header. Clicking it opens a self-contained floating dialog inside the report (no CDN, no external assets) with a short form: title, body, optional category tag. On submit, the browser opens GitHub's 'new discussion' pre-fill URL for the 'Ideas' category of matias-sanchez/My-gather in a new tab, with the title and body pre-populated; the user finishes posting on GitHub. Goal: zero-friction path for report readers (support engineers) to drop ideas into the project's Ideas discussion category without leaving the report. Non-goals: no token handling, no API proxy, no comment threads inside the report."

## Clarifications (2026-04-24)

- **Scope**: image paste (US4) and voice record (US5) ship in the **same feature** as the text flow (US1), not split across a v1/v2. One design review, one PR.
- **Keyboard submit**: pressing **Cmd/Ctrl+Enter** inside the dialog submits, matching GitHub's own editor shortcut. The Submit button remains available; both paths MUST be blocked under the same conditions (empty title, URL length budget exceeded).
- **Category marker placement**: the chosen category is prepended to the body as a single leading quoted line `> Category: <name>` followed by a blank line, then the user's body text. This renders as a quoted aside on GitHub — visible for triage, unobtrusive on short bodies.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Report reader drops an idea mid-triage (Priority: P1)

A support engineer is looking at a rendered pt-stalk report during an incident. They notice something the report could do better — for example, "this chart needs a log-scale toggle" or "advisor should flag X when Y". Instead of context-switching to a browser tab, hunting for the project repo, clicking Discussions → Ideas → New, and re-typing their thought from memory, they click a "Report feedback" control in the report's top-right and type the idea immediately. Their attention returns to the incident within seconds.

**Why this priority**: This is the entire reason the feature exists. Without P1, the feature has no value. Every other slice below is an enhancement.

**Independent Test**: Open a rendered report HTML file with no network connection. Click "Report feedback". A form opens with title + body fields. Fill both. Click submit. Verify that a browser tab opens to `https://github.com/matias-sanchez/My-gather/discussions/new?category=ideas&title=...&body=...` with the typed values URL-encoded in the query string. Close the report; no trace is left in the filesystem.

**Acceptance Scenarios**:

1. **Given** a rendered report is open in a browser with no network, **When** the user clicks "Report feedback" in the top-right of the header, **Then** a floating dialog appears inside the report with title and body inputs.
2. **Given** the dialog is open and the user has typed a title ("Log-scale toggle for IOPS chart") and a body ("Would help compare idle vs. peak..."), **When** the user clicks Submit, **Then** a new browser tab opens to the GitHub "new discussion" URL for the Ideas category with those values pre-filled.
3. **Given** the dialog is open, **When** the user presses Escape or clicks outside the dialog, **Then** the dialog closes and focus returns to the previously focused element. No data is sent anywhere.
4. **Given** the dialog is open with an empty title, **When** the user clicks Submit, **Then** the Submit button is disabled (or the form refuses to submit) and the title field is highlighted, because GitHub's pre-fill URL without a title drops the user into an unlabelled discussion.

---

### User Story 2 — Keyboard-first reader can open and submit without touching the mouse (Priority: P2)

An engineer using a keyboard-only workflow opens a report and wants to capture an idea. They press Tab through the header, reach the feedback control, press Enter to open the dialog, Tab between fields, and Ctrl/Cmd+Enter to submit.

**Why this priority**: Support engineers live in terminals. A mouse-only control is a papercut. Not the main value, but raises the adoption rate.

**Independent Test**: Same as Story 1 but the entire flow — open, fill, submit, cancel — is executed without using a mouse. Verify focus ring is visible on the feedback control, Enter opens the dialog, Tab moves between fields, Escape closes, Ctrl/Cmd+Enter submits.

**Acceptance Scenarios**:

1. **Given** the report has keyboard focus anywhere in the page, **When** the user presses Tab repeatedly, **Then** the "Report feedback" control receives a visible focus ring before the report body content.
2. **Given** the feedback control has focus, **When** the user presses Enter or Space, **Then** the dialog opens and keyboard focus moves to the title input.
3. **Given** the dialog is open, **When** the user presses Escape, **Then** the dialog closes and focus returns to the feedback control.

---

### User Story 4 — Paste a screenshot from the clipboard into the report (Priority: P3)

A support engineer sees something visual in the report (a weird chart gap, a misaligned label). They take a screenshot with the OS screenshot tool (which lands on the clipboard), then paste into the feedback dialog. A thumbnail preview appears inside the dialog. On Submit, the feedback flow copies the image back onto the OS clipboard so the user can paste it into GitHub's new-discussion body after the pre-filled page opens. GitHub then auto-uploads the pasted image and replaces it with a CDN link in the discussion body.

**Why this priority**: Dramatically raises signal for UI/visual feedback where words alone fall flat. P3 because text-only feedback already works; this is an enhancement.

**Independent Test**: Take a screenshot (OS shortcut, e.g., Cmd+Ctrl+Shift+4 on macOS, PrintScreen + region on Windows) so the image lives on the clipboard. Open the feedback dialog. Press Cmd/Ctrl+V anywhere inside the dialog body area. Verify a thumbnail preview appears. Click Submit. Verify (a) a new GitHub tab opens with the title/body pre-filled, and (b) the OS clipboard now contains the pasted image (or the dialog's inline instructions explicitly tell the user how to attach it, if clipboard-write is blocked). Paste into GitHub's body field and observe GitHub auto-upload.

**Acceptance Scenarios**:

1. **Given** the dialog is open with an image on the OS clipboard, **When** the user presses Cmd/Ctrl+V inside the dialog, **Then** a thumbnail of the image appears in a dedicated "Attachments" area with a small "Remove" control.
2. **Given** an image is attached, **When** the user clicks Submit, **Then** a new GitHub tab opens with the pre-filled URL AND the image is on the OS clipboard ready to paste.
3. **Given** an image is attached and the user clicks Remove, **Then** the thumbnail disappears and the image is discarded from dialog memory.
4. **Given** the browser denies clipboard-write permission at Submit, **Then** the dialog displays a visible fallback: a download link for the image plus one-line instructions to drag the file into GitHub's body field.

---

### User Story 5 — Record a short voice note and attach it (Priority: P3)

A support engineer wants to describe something that is easier spoken than typed (a workflow walk-through, a reproduction sequence). They click a "Record voice note" control in the dialog, speak for up to a cap (suggested 2 minutes), and stop. A small audio preview with play/pause appears in the Attachments area. On Submit, the feedback flow triggers a one-click download of the recording file so the user can drag it into the GitHub body field on the new tab.

**Why this priority**: Same as US4 — raises signal, but not essential. P3.

**Independent Test**: Open the dialog. Click "Record voice note". The browser prompts for microphone permission; grant it. Speak for ~5 seconds. Click Stop. Verify a playback control appears. Click Submit. Verify (a) a new GitHub tab opens with title/body pre-filled, and (b) the browser triggers a file download of the recording (e.g., `feedback-voice-<short-hash>.webm`). Drag the downloaded file into GitHub's body and confirm GitHub accepts and auto-uploads it.

**Acceptance Scenarios**:

1. **Given** the dialog is open, **When** the user clicks "Record voice note" and grants microphone permission, **Then** a live recording indicator appears with an elapsed-time counter and a Stop control.
2. **Given** recording is in progress, **When** the user clicks Stop (or the cap is reached), **Then** a playback control and a "Remove" control appear in the Attachments area.
3. **Given** a recording is attached, **When** the user clicks Submit, **Then** a new GitHub tab opens AND the browser downloads the recording file.
4. **Given** the user denies microphone permission, **Then** the dialog displays a clear error message in the Attachments area and the Record control is re-enabled (so permission can be re-requested); the rest of the form remains fully usable.
5. **Given** recording is capped at the duration limit, **When** the cap is reached, **Then** recording stops automatically and the user sees a short notice that the cap triggered.

---

### User Story 3 — Optional category tag pre-populates a visible triage marker (Priority: P3)

When a user categorises their feedback (e.g., "UI", "parser", "advisor"), that category is appended to the body as a short prefix line so the project maintainer can triage the discussion faster. This does not attempt to use GitHub labels directly (labels require a token and would drag the feature into scope-creep).

**Why this priority**: Useful but not essential. P1 already delivers the goal; P3 sharpens triage.

**Independent Test**: Open the dialog, type title + body, pick a category from a short dropdown (e.g., "UI / Parser / Advisor / Other"). Submit. Verify the GitHub new-discussion URL's body parameter starts with a single line like `> Category: UI` (or equivalent text marker) followed by the user's body text.

**Acceptance Scenarios**:

1. **Given** the dialog is open with a category chosen, **When** the user submits, **Then** the pre-filled body on GitHub begins with a visible category marker line and the user's body text follows.
2. **Given** the dialog is open with no category chosen, **When** the user submits, **Then** the body is sent unchanged (no empty category line, no placeholder).

---

### Edge Cases

- **Browser blocks the new-tab open** (popup blocker on some browsers for non-user-initiated navigation): The spec requires that submit is directly driven by the user's click, so browsers treat it as user-initiated. Fallback: if the window did not open (detected by `window.open` returning null), show an inline link the user can click to go to GitHub manually, with the same pre-filled URL.
- **User has no network when clicking Submit**: The report is offline-safe; the Submit action opens a new tab that requires network. If there's no connectivity, GitHub loads an error page — but the user's typed content is lost only from the new tab, not from the form. The form dialog remains open until the user explicitly dismisses it, so they can copy-paste the content if GitHub is unreachable.
- **Very long title or body**: GitHub's URL length limit is roughly 8000 characters in practice; URLs longer than that truncate server-side. The form MUST warn the user when combined title + body approaches this limit, and prevent submit when it exceeds, rather than silently losing content.
- **User accidentally types sensitive data** (hostnames, IPs, database names from the very report they are reading): This is out of scope to automatically redact. The form help text MUST remind the user that the submitted text becomes a public GitHub Discussion.
- **Report opened from `file://` on a locked-down laptop** (no internet-facing DNS): Same as no-network — submit opens a tab, tab fails to load, form stays open for copy-paste recovery.
- **Multiple reports open in different tabs**: Each report's dialog is self-contained within its own HTML; they do not share state. This is implied by the self-contained-report principle.
- **Browser does not grant microphone permission**: recording control shows an error; text-only flow (and image-paste flow) continues to work.
- **Clipboard-write API unsupported** (older browsers, some privacy-focused browsers): fallback download link replaces the clipboard hand-off for images (FR-021).
- **Very large pasted image**: the in-URL flow is unaffected (image does not ride the URL). The clipboard copy may silently fail on some browsers for images above a few MB; if the browser rejects the write, the download fallback kicks in. No compression of the pasted image is attempted in v1 — the raw clipboard bytes are used.
- **Voice recording exceeds the duration cap**: recording stops automatically; the user is informed the cap triggered. Partial content is kept, not discarded.
- **User pastes an image while a different image is already attached**: the new paste replaces the existing thumbnail (one image per draft per FR-016), with a brief inline notice that the previous image was discarded. This keeps the hand-off deterministic (clipboard only holds one image).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The rendered report MUST display a "Report feedback" control in the top-right of its existing header, alongside the existing metadata (host / snapshots / generated / version). The control MUST NOT displace or resize the existing header content.
- **FR-002**: The control MUST be implemented with assets that are already embedded in the binary (per Principle V). It MUST NOT introduce any new external asset at view-time: no CDN, no external font, no remote icon, no analytics.
- **FR-003**: The control's rendered markup MUST be byte-identical across runs for the same input (per Principle IV). It MUST NOT include the current wall-clock time, a random id, or any field that varies between runs.
- **FR-004**: Clicking the control MUST open a floating dialog inside the report with, at minimum, a title input, a body textarea, a Submit button, and a Cancel button.
- **FR-005**: The dialog MUST be dismissible by Escape, by clicking Cancel, and by clicking the backdrop outside the dialog. Dismissal MUST NOT send any data anywhere.
- **FR-006**: On Submit, the report MUST open a new browser tab via a user-initiated `window.open` (or anchor click) to the URL `https://github.com/matias-sanchez/My-gather/discussions/new?category=ideas&title=<URL-encoded title>&body=<URL-encoded body>`.
- **FR-007**: The report MUST NOT transmit the title or body to any origin other than the user's explicit GitHub navigation in FR-006. No telemetry, no analytics, no proxy.
- **FR-008**: The control's tab order MUST place it after the existing header metadata and before the first collapsible report section, so keyboard readers encounter it early but not in place of the report's primary content.
- **FR-009**: The control MUST expose an accessible name ("Report feedback" or equivalent) to assistive technologies via standard ARIA or native HTML semantics.
- **FR-010**: The dialog MUST trap focus while open (Tab cycles within the dialog's interactive elements) and restore focus to the control on close.
- **FR-011**: The Submit action (whether triggered by clicking the Submit button or pressing Cmd/Ctrl+Enter inside the dialog) MUST be blocked while the title field is empty, because a GitHub pre-fill URL without a title lands in an unlabelled discussion.
- **FR-011a**: Pressing Cmd/Ctrl+Enter with focus anywhere inside the dialog MUST submit the form, matching GitHub's own editor convention. Pressing Enter inside the title input MUST also submit (single-line input convention). Pressing Enter inside the body textarea MUST insert a newline (standard textarea convention), not submit.
- **FR-012**: When the combined URL-encoded `title + body` (plus the category prefix, if any, from FR-013) exceeds a conservative length budget (target 7500 characters to leave margin under GitHub's roughly 8 KB URL limit), the Submit button MUST be blocked and the dialog MUST display an inline message asking the user to shorten the body.
- **FR-013**: The dialog MUST offer an optional category selector (UI / Parser / Advisor / Other). When a category is chosen, its marker MUST be prepended to the body as the leading line `> Category: <name>` followed by one blank line and then the user's body text, before URL-encoding. When no category is chosen, the body is sent unchanged (no empty category line, no placeholder).
- **FR-014**: The dialog MUST display a visible reminder that submitted content becomes a public GitHub Discussion, positioned where the user sees it before clicking Submit.
- **FR-015**: If `window.open` returns null (popup blocked) when Submit is clicked, the dialog MUST display a visible fallback link with the same pre-filled URL so the user can click through manually without retyping.
- **FR-016**: The dialog MUST accept an image pasted from the OS clipboard (Cmd/Ctrl+V) when focus is inside the dialog. Pasted image data MUST be rendered as a thumbnail preview in a dedicated Attachments area with a visible Remove control. The dialog MUST accept at most one image attachment per draft (replacing any previous one on a new paste) to keep the hand-off flow simple.
- **FR-017**: The dialog MUST accept an in-browser voice recording via the browser MediaRecorder API with explicit microphone-permission prompting. Recording MUST be capped at a maximum duration (target 2 minutes) with a visible elapsed-time counter. On stop, the recording MUST be exposed as an in-dialog playback control with a Remove action. The dialog MUST accept at most one voice recording per draft.
- **FR-018**: Because GitHub's pre-fill URL does not accept file attachments, on Submit the feedback flow MUST hand media off through the browser's own boundaries, not through the URL: (a) for an attached image, the flow MUST best-effort copy the image onto the OS clipboard as `image/png` via the asynchronous Clipboard API, so the user can paste into GitHub's body; (b) for an attached voice note, the flow MUST trigger a one-click download of the recording file (e.g., `feedback-voice-<stable-hash>.webm`) so the user can drag-drop into GitHub's body.
- **FR-019**: The dialog MUST visibly show a single-line instruction next to the Submit button explaining what will happen to any attached media on submit ("Image will be on your clipboard — paste into GitHub's body", "Voice note will be downloaded — drag it into GitHub's body"). When no media is attached, no such instruction is shown.
- **FR-020**: All media capture and playback MUST work offline: the Clipboard API, MediaRecorder, and audio playback are browser primitives that do not require network. The binary MUST NOT embed any codec library or audio asset as a CDN link to satisfy these requirements (Principle V); only browser-native APIs and `//go:embed`-bundled UI assets are allowed.
- **FR-021**: If the browser blocks clipboard-write at Submit time (permission denied or API unsupported), the dialog MUST degrade to an inline download link for the image alongside a one-line instruction ("Your browser blocked the clipboard copy. Download the image and drag it into GitHub's body."). Submit MUST still open the GitHub pre-filled tab regardless of clipboard-write outcome, so the text half of the hand-off never regresses.
- **FR-022**: Attachments MUST NOT leave dialog memory except via the user-initiated mechanisms above (clipboard-write on Submit, download on Submit). They MUST NOT be transmitted to any network endpoint from the binary or from the report's runtime code.
- **FR-023**: Attachment metadata (image dimensions, audio duration) MAY be computed client-side and shown as inline hints. Such metadata MUST NOT be included in the URL-encoded body unless the user has chosen a category that would naturally include it.

### Key Entities

- **Feedback draft**: a transient in-browser object holding `{ title: string, body: string, category: string | null, imageAttachment: Blob | null, voiceAttachment: Blob | null }`. It lives only in the browser's JavaScript memory. It is never persisted to localStorage, IndexedDB, or sent anywhere except via the user-initiated GitHub URL (FR-006) and the user-initiated clipboard-write / download actions (FR-018).
- **GitHub Ideas discussion URL**: the constant `https://github.com/matias-sanchez/My-gather/discussions/new?category=ideas` with `title` and `body` query parameters appended at submit time. The base URL is embedded at build time and is the only constant that couples the report to the project repo.
- **Attachment hand-off contract**: the documented behaviour of the browser primitives used to carry media across the tab boundary — OS clipboard for images, download-link for audio. This contract is a dependency on browser capabilities, not on any third-party service.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A report reader can open the feedback dialog, type a short title and body, and land on the pre-filled GitHub new-discussion page in under 10 seconds from first glance, without leaving the report tab until they choose to submit.
- **SC-002**: The report remains viewable on an air-gapped laptop. With no network connectivity, clicking the feedback control still opens the dialog and lets the user draft text; only the final Submit requires network (and gracefully falls back to a copy-paste link if the popup is blocked).
- **SC-003**: Determinism is preserved: running the generator twice on the same input directory produces byte-identical report HTML including the feedback control markup.
- **SC-004**: Adoption signal: within the first week after the feature ships, at least one new discussion appears in the Ideas category that was submitted via the in-report control (detectable by the category marker line or by asking the submitter).
- **SC-005**: Accessibility: keyboard-only users can complete the entire flow (open → type → submit → close) without using the mouse, and screen-reader users hear the dialog announced as a dialog with the control's accessible name.
- **SC-006**: Media hand-off: a user who pastes a screenshot and clicks Submit can, on the new GitHub tab, paste the same image into the discussion body with a single Cmd/Ctrl+V and see GitHub upload it — succeeds on at least the two most-used browsers (Chrome/Edge, Firefox) within the two-most-recent releases on macOS and Linux.
- **SC-007**: Offline safety: with the laptop's WiFi turned off, the entire dialog continues to function — open, type, paste image, record voice, preview playback, remove, re-paste. Only the final Submit step (which opens GitHub) requires network, and its failure never corrupts the draft.

## Assumptions

- The project's GitHub repository `matias-sanchez/My-gather` has Discussions enabled with an Ideas category (verified at 2026-04-24 via the URL the user provided).
- GitHub's `discussions/new?category=<slug>&title=...&body=...` pre-fill URL format is stable enough for production use; it has been a supported GitHub URL pattern for several years and changes are rare.
- Readers of the report have a modern browser with standard HTML form, dialog, and `window.open` support (Chrome / Firefox / Safari released within the last two years).
- Reports may be shared by email or filesystem; the control must keep working when the report is opened from a `file://` URL and when it is opened behind a corporate proxy.
- The feature is additive to the existing header: the current header layout has room on the top-right for a small control without reflowing other elements. If measurement during `/speckit.plan` shows the header is already visually full, we will revisit placement (top-right of the page as a floating affordance is acceptable too).
- The target browsers support the asynchronous Clipboard API (`navigator.clipboard.write`) for image writes and the MediaRecorder API for voice capture. Where support is absent, the dialog falls back gracefully per FR-021 (image → download) and FR-017 (voice → permission-error state); the text-only flow is never blocked by the absence of media capabilities.
- GitHub's new-discussion body field auto-uploads pasted images and drag-dropped audio files. This behaviour is observed current GitHub functionality and is the reason the clipboard+download hand-off works. If GitHub changes this (removing auto-upload), FR-016/FR-017/FR-018 would need revision.
- Voice recording format is browser-default MediaRecorder output (`audio/webm;codecs=opus` on Chrome/Firefox, `audio/mp4` on Safari). GitHub accepts both; the dialog does not transcode. No audio codec library is bundled into the Go binary (Principle V, X).
