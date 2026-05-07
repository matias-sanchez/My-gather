# Feature Specification: Required Author field on Report Feedback

**Feature Branch**: `021-feedback-author-field`
**Created**: 2026-05-07
**Status**: Draft
**Input**: User description: "Add a required \"Author\" input field to the Report Feedback dialog. Submission is blocked with inline validation until the user enters a non-empty author name. The submitted GitHub issue body must include a \"Submitted by: <name>\" line so triagers can identify the reporter. The last-used author value is persisted in browser localStorage and pre-fills the field on subsequent dialog opens."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Sign your feedback before sending it (Priority: P1)

A support engineer viewing a generated report opens the "Report feedback"
dialog to flag a parser issue. The dialog now shows an "Author" field at
the top. The user types their name, fills the Title and Body, and clicks
Submit. The created GitHub issue body carries a "Submitted by: <name>"
line so the triager knows who reported it without scrolling git blame or
opening Slack.

**Why this priority**: This is the entire feature - without an author on
the issue, triagers cannot tell who reported it, which today is the
single concrete pain point closing GitHub issue #56.

**Independent Test**: Open the dialog, type an Author + Title + Body,
Submit. The created GitHub issue body includes a "Submitted by: <name>"
line attributable to the typed value.

**Acceptance Scenarios**:

1. **Given** the report is open and the user has never submitted
   feedback before, **When** they click "Report feedback", **Then** the
   dialog shows an Author field labelled "Author" that is empty and
   marked required.
2. **Given** the dialog is open with all other fields filled, **When**
   the user leaves Author empty, **Then** the Submit button is disabled
   and an inline validation hint indicates Author is required.
3. **Given** the user has typed a non-empty Author plus a non-empty
   Title, **When** they click Submit and the worker accepts the
   request, **Then** the resulting GitHub issue body contains a line
   "Submitted by: <typed-name>".

---

### User Story 2 - Don't retype your name every time (Priority: P2)

A support engineer who already submitted feedback once opens the dialog
again later (same browser, same machine) to file another report. The
Author field is pre-filled with the name they used last time. They can
edit it, clear it, or accept it as-is.

**Why this priority**: Eliminates the "type your own name again on every
submission" papercut for users who file multiple reports per incident.

**Independent Test**: Submit feedback once with a chosen Author. Close
the dialog. Reopen the dialog. Author field is pre-filled with that
value.

**Acceptance Scenarios**:

1. **Given** the user has previously submitted feedback successfully
   with Author "Jane", **When** they reopen the dialog in the same
   browser, **Then** the Author field is pre-filled with "Jane".
2. **Given** the field is pre-filled with "Jane", **When** the user
   edits it to "J. Doe" and submits, **Then** the next dialog open
   shows "J. Doe".
3. **Given** the user's browser blocks or has no localStorage (e.g.,
   private window), **When** they open the dialog, **Then** the Author
   field is empty and the user can still type a value and submit; no
   error is shown about persistence.

---

### Edge Cases

- Author entered as whitespace only (spaces/tabs): treated as empty,
  Submit stays disabled (mirrors the existing Title trim-empty rule).
- Author longer than the field length cap: Submit is disabled with the
  same inline validation pattern used for Title-too-long today.
- localStorage write fails (quota exceeded, security exception): the
  current submission still succeeds; the persistence failure is
  silently ignored (the next dialog open will simply not pre-fill).
- Worker receives a payload missing or empty `author`: rejected with a
  400 validation error (mirrors `title_required`); the frontend gate
  should normally have prevented this but the worker is still the
  authoritative gate.
- Legacy `window.open` GitHub fallback path (when the worker is
  unavailable or fetch is missing): the pre-filled GitHub URL body
  also carries the "Submitted by: <name>" line so both submission
  paths produce equivalent issue bodies.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The Report Feedback dialog MUST display an Author input
  field positioned above the Title field, labelled "Author", marked
  visibly as required.
- **FR-002**: The Submit button MUST be disabled while the Author
  field's trimmed value is empty, in addition to the existing
  Title-empty and length-cap gates.
- **FR-003**: The dialog MUST show inline validation indicating the
  Author field is required when it is empty (consistent with the
  existing inline error surface used for other validation failures -
  no separate alert dialog, no submission attempt).
- **FR-004**: The submitted GitHub issue body composed by the feedback
  worker MUST include a single line "Submitted by: <name>" derived
  from the trimmed Author value, attributable to the dialog input.
- **FR-005**: The last-used non-empty Author value MUST be persisted
  in browser localStorage on successful submission and used to
  pre-fill the Author field the next time the dialog is opened in the
  same browser profile.
- **FR-006**: The Author value MUST be carried in the payload sent to
  the feedback worker as a dedicated, required field - not folded into
  the body string by the frontend - so the worker is the canonical
  composer of the "Submitted by:" line.
- **FR-007**: The legacy `window.open` GitHub-prefill fallback path
  MUST also include the "Submitted by: <name>" line in the prefilled
  body so both submission paths produce equivalent GitHub issue bodies
  (Principle XIII canonical observable behaviour across the worker and
  the documented external-degradation path).
- **FR-008**: The feedback worker MUST validate Author as a required,
  non-empty, trimmed string with a published maximum character length,
  and MUST reject payloads missing or violating that contract with the
  same 400 error shape used by the existing required-field
  validations.
- **FR-009**: The Author character cap MUST be expressed once, in the
  shared feedback contract JSON consumed by both the Go renderer and
  the worker, so the frontend Submit gate and the worker validator
  enforce identical limits.

### Canonical Path Expectations

- **Canonical owner/path**: The Report Feedback dialog UI lives in
  `render/templates/report.html.tmpl` (markup), `render/feedback.go`
  (Go view + contract loading), `render/assets/feedback-contract.json`
  (single source of truth for limits and categories), and the dialog
  JavaScript split across `render/assets/app-js/03.js` and
  `render/assets/app-js/04.js`. The submit-time worker path lives in
  `feedback-worker/src/validate.ts` (payload validation),
  `feedback-worker/src/feedback-contract.ts` (shared contract import),
  and `feedback-worker/src/body.ts` (issue body composition).
- **Old path treatment**: The current dialog has no Author field. This
  feature adds one canonical Author input + payload field + worker
  validation rule + body-composition line; nothing is replaced or
  duplicated. The legacy GitHub `window.open` fallback's body
  composer (`maybePrefixBody` in `app-js/03.js`) is updated to also
  emit the "Submitted by:" line so the two submission paths converge
  on a single observable body shape; no parallel composer is added.
- **External degradation**: localStorage absence (private window,
  quota error, browser policy) is a genuine external browser-boundary
  outcome - the field still works, the user types their name, the
  submission still succeeds, only the next-time pre-fill is skipped.
  This degradation is observable in the dialog (empty field on next
  open) and explicitly tested.
- **Review check**: Reviewer verifies (a) the Author cap is read from
  the shared contract JSON in both Go and TypeScript, not duplicated;
  (b) the "Submitted by:" line is composed exactly once, on the worker
  body composer, and the legacy fallback URL builder mirrors the same
  line format; (c) no shim defaults the worker payload's `author`
  field to an empty string or "Anonymous" - a missing or empty
  `author` is a 400.

### Key Entities

- **FeedbackPayload (extension)**: The JSON payload POSTed to the
  worker gains a required string field `author` (trimmed,
  non-empty, length-capped per the contract).
- **FeedbackContract (extension)**: The shared contract JSON gains a
  `limits.authorMaxChars` integer used by both the Go renderer's
  Submit-gate validation and the worker's payload validation.
- **PersistedAuthor**: A localStorage entry under a stable key holding
  the last successfully-submitted non-empty Author value as plain text.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of GitHub issues created via the Report Feedback
  dialog after this feature ships carry a "Submitted by: <name>"
  attribution line in the issue body.
- **SC-002**: The dialog blocks Submit when Author is empty in 100%
  of attempts; no submission lacking an Author can reach the worker
  via the normal UI gate.
- **SC-003**: A user who submits feedback once and reopens the dialog
  in the same browser session sees their previous name pre-filled
  without retyping, in 100% of cases where browser localStorage is
  writable.
- **SC-004**: Both the Worker submission path and the legacy GitHub
  `window.open` fallback path produce GitHub issue bodies that contain
  the same "Submitted by: <name>" line for the same input.

## Assumptions

- The display name typed into the Author field is plain text - not
  required to match a GitHub handle, email, or SSO identity. Triagers
  use it as a free-form attribution.
- Author character cap is set to 80 - long enough for "First Last
  (Team)" style names, short enough to fit on the GitHub issue title
  preview row when surfaced inline. Encoded in the contract JSON.
- The localStorage key namespace is `mygather.feedback.lastAuthor`
  (single key, plain text value).
- Persistence happens on definitive submit success (worker 200), not
  on every keystroke and not on dialog cancel - so a half-typed,
  abandoned name does not pollute the next session.
- The dialog already has a single inline error surface
  (`#feedback-error`); no new visual surface is introduced for the
  Author validation message.
- The worker contract version is forward-only: older report binaries
  that lack the Author field will be unable to POST successfully (the
  worker will 400 them) once this contract change ships. There is no
  back-compat shim; this is intentional under Principle XIII.
- File-size budget under Principle XV: the JS files
  `render/assets/app-js/03.js` (~898 lines) and `04.js` (~490 lines)
  are close to the 1000-line cap; the Author wiring is small enough
  to land within budget. If the diff would push 03.js over the cap,
  the planner extracts the dialog wiring into a new ordered
  `render/assets/app-js/05.js` rather than merging a violation.
