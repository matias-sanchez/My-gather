# Contract: Author field on Report Feedback

This contract is the single source of truth for the Author field
behaviour across the dialog UI, the JSON payload sent to the worker,
the worker's validation, and the issue body composition.

## C1: Shared contract JSON (`render/assets/feedback-contract.json`)

The `limits` object gains exactly one new field:

```json
{
  "limits": {
    "...": "...existing fields unchanged...",
    "authorMaxChars": 80
  }
}
```

- Type: positive integer.
- Both `render/feedback.go` and `feedback-worker/src/feedback-contract.ts`
  MUST consume it from this file. Neither side may hard-code the
  value.

## C2: Dialog markup (`render/templates/report.html.tmpl`)

A new field block is inserted IMMEDIATELY before the existing
`feedback-field-title` block:

```html
<div class="feedback-field">
  <label for="feedback-field-author">Author <span class="feedback-field-req">required</span></label>
  <input id="feedback-field-author" name="author" type="text"
         autocomplete="name" placeholder="Your name" required
         maxlength="{{ .Feedback.AuthorMaxChars }}">
</div>
```

- The element ID `feedback-field-author` is part of the UI contract
  and stable (the JS wiring keys off it).
- `maxlength` is rendered from the contract value via the new
  `FeedbackView.AuthorMaxChars` field.
- The `required` HTML attribute is present so screen readers announce
  the requirement; the canonical Submit gate is JS, the canonical
  rejection gate is the worker.

## C3: Go view (`render/feedback.go`)

`FeedbackView` gains one exported field:

```go
type FeedbackView struct {
    Categories      []string
    ContractJSON    string
    AuthorMaxChars  int
}
```

`feedbackContract.Limits` gains `AuthorMaxChars int` with JSON tag
`authorMaxChars`. `loadFeedbackContract` extends the positivity
check to include the new field. `BuildFeedbackView` populates
`AuthorMaxChars` from the parsed contract.

## C4: Worker payload (`feedback-worker/src/validate.ts`)

`ValidatedPayload` gains a required `author: string` field.
`ValidationError` gains two new literals: `"author_required"` and
`"author_too_long"`.

Validation order: `author` is validated immediately after `title`
(both required strings) and before `body`.

Rejection rules:
- Missing field, non-string, empty string, or whitespace-only string:
  `fail("author_required", "Author is required.")`.
- `value.length > feedbackContract.limits.authorMaxChars`:
  `fail("author_too_long", "Author exceeds N characters.")`.
- Otherwise: stored as `value.trim()` on the validated payload.

## C5: Worker contract type (`feedback-worker/src/feedback-contract.ts`)

The `FeedbackContract.limits` interface gains
`authorMaxChars: number`. The existing `assertPositiveInteger` walk
covers the new field automatically because it iterates
`Object.entries(contract.limits)`.

## C6: Worker body composer (`feedback-worker/src/body.ts`)

The first lines of `buildIssueBody` output become:

```text
Submitted by: <author>

```

(That is: the literal `Submitted by: ` prefix + the validated author +
newline + blank line.) The remainder of the composition is unchanged.

The line is ALWAYS present (Author is required by C4).

## C7: Frontend payload (`render/assets/app-js/04.js`)

`doSubmit` adds exactly one line to the payload object construction:

```js
payload.author = authorInput.value.trim();
```

The field is required; there is no conditional inclusion.

## C8: Frontend Submit gate (`render/assets/app-js/03.js`)

`updateSubmitEnabled` adds an Author check immediately after the
existing Title trim-empty check:

- If `authorInput.value.trim().length === 0`, disable Submit.
- If `authorInput.value.length > LIMITS.authorMaxChars`, disable Submit.

The `closeDialog` reset assigns `authorInput.value = ""` (or the
persisted value if any — see C10).

The `onFormContentChange` event wiring includes `authorInput` so
edits invalidate the in-flight `idempotencyKey`, mirroring how
`titleInput` / `bodyInput` / `catSelect` already do.

## C9: Legacy fallback URL builder (`render/assets/app-js/03.js`)

`maybePrefixBody` is extended so its output is:

```text
Submitted by: <author>

[> Category: <cat>\n\n   if category]
<user-typed body>
```

(Same `Submitted by:` line shape as C6.) The Author value is read
from the same `authorInput` ref. Because Author is the canonical
required field, the fallback path also has Author present at submit
time (the Submit button is disabled otherwise — see C8).

## C10: localStorage persistence (`render/assets/app-js/03.js`)

Two helpers, both wrapped in `try/catch` so a `localStorage`
exception silently degrades to "no persistence":

- `loadPersistedAuthor()`: returns the stored string truncated to
  `LIMITS.authorMaxChars`, or `""` on absence/error.
- `savePersistedAuthor(value)`: writes the trimmed value if non-empty;
  no-op otherwise.

Lifecycle (R5, R6):
- `openDialog`: if `authorInput.value === ""`, set it from
  `loadPersistedAuthor()`.
- Worker success arm of `doSubmit` (`renderSuccess` callsite): call
  `savePersistedAuthor(authorInput.value.trim())`.

The localStorage key is the literal string
`mygather.feedback.lastAuthor`.

## C11: Backwards compatibility

This contract change is forward-only. Reports built before this
feature lands do not POST `author` and will receive 400
`author_required` from the worker after this contract ships. The
existing legacy `window.open` fallback handles those clients
gracefully with a pre-filled GitHub URL (which lacks the
`Submitted by:` line because the older binary doesn't know about
Author — that's acceptable graceful degradation, not a forbidden
fallback, since the binary itself is the boundary).
