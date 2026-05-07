# Data Model: Required Author field on Report Feedback

## Entities

### FeedbackContract.limits.authorMaxChars (NEW field)

| Field | Type | Constraints | Description |
|---|---|---|---|
| `authorMaxChars` | positive integer | > 0 | Maximum length of the Author display name in characters. Single source of truth for both the frontend Submit gate and the worker validator. |

**Storage**: `render/assets/feedback-contract.json` (existing
embedded asset, consumed by both Go and TypeScript).

**Value (R1)**: `80`.

**Validation**:
- `render/feedback.go#loadFeedbackContract` MUST panic at startup if
  `authorMaxChars <= 0` (mirrors the existing positivity check on the
  other limits).
- `feedback-worker/src/feedback-contract.ts#assertFeedbackContract`
  MUST throw at module load if `authorMaxChars` is not a positive
  integer (mirrors the existing `assertPositiveInteger` walk over
  `limits`).

### FeedbackPayload.author (NEW required field)

| Field | Type | Constraints | Description |
|---|---|---|---|
| `author` | string (required) | `trim(value).length >= 1`, `value.length <= contract.limits.authorMaxChars` | Display name of the user submitting the feedback. Worker uses it to compose the `Submitted by: <name>` line. |

**Wire location**: Top-level field on the JSON body POSTed to
`/feedback`.

**Frontend gate** (`render/assets/app-js/03.js#updateSubmitEnabled`):
- If `authorInput.value.trim().length === 0`, Submit is disabled.
- If `authorInput.value.length > LIMITS.authorMaxChars`, Submit is
  disabled and the inline error reads "Author must be N characters or
  fewer" (matches the title-too-long pattern).

**Worker validation** (`feedback-worker/src/validate.ts#validatePayload`):
- Adds `author_required` and `author_too_long` to `ValidationError`.
- Adds `author: string` to `ValidatedPayload`.
- Rejects payloads where `author` is missing, not a string, an empty
  string, or trims to empty: `fail("author_required", "Author is required.")`.
- Rejects payloads where `author.length > authorMaxChars`:
  `fail("author_too_long", "Author exceeds N characters.")`.
- The validated `author` carried forward is `value.trim()` (no
  internal-whitespace collapsing).

### PersistedAuthor (browser localStorage entry)

| Field | Type | Constraints | Description |
|---|---|---|---|
| key | string literal | `mygather.feedback.lastAuthor` | Stable per-origin storage key. |
| value | string | `length >= 1`, `length <= contract.limits.authorMaxChars` (read-time enforced) | The trimmed Author value from the most recent successful submission. |

**Lifecycle**:
- WRITE: in the success arm of `doSubmit` (right next to
  `renderSuccess`), after the worker returns 200. The value written is
  `authorInput.value.trim()`.
- READ: in `openDialog`, before `dialog.showModal()`. If a stored
  value exists and is within `authorMaxChars`, assign to
  `authorInput.value`. If it exceeds the cap (e.g. cap was lowered in
  a later release), truncate to the cap silently rather than refuse to
  load.
- ABSENT (private window, quota error, browser policy): caught with
  `try/catch` around both `getItem` and `setItem`; the field stays
  empty (read failure) or the persistence is skipped (write failure).
  No user-visible error.

### Issue Body shape (extension to existing composition)

The `feedback-worker/src/body.ts#buildIssueBody` output gains one
leading line + a blank separator. New shape:

```text
Submitted by: <author>

> Category: <category>          (only if category present)

<user-typed body>

### Attached screenshot         (only if image)

![screenshot](<imageUrl>)

### Attached voice note         (only if voice)

<voiceUrl>

---
_Submitted via my-gather Report Feedback (v<reportVersion>)._
```

**Ordering rationale (R3)**: Author attribution is the most-relevant
context for triage routing; it earns the top slot. Category quote
moves down by exactly one block. The existing trailer is unchanged.

### Legacy fallback URL body (extension to existing composition)

The `render/assets/app-js/03.js#maybePrefixBody` output gains the
same leading `Submitted by: <author>` line + blank separator,
followed by the existing optional `> Category:` block, followed by
the user-typed body. This is the body GitHub sees when the worker
path is unavailable and the report falls back to `window.open` on a
GitHub new-issue URL with title+body query parameters.

**Same observable shape** as the worker path's body for the same
input (Principle XIII canonical observable behaviour). The only
difference is that the URL-pre-fill body cannot embed attachments;
the existing clipboard/download handoff covers that.
