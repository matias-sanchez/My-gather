# Quickstart: Required Author field on Report Feedback

This is the implementer's checklist. Steps are ordered to keep the
build green at each step. Tests come first per repo convention.

## Prereqs

- Branch `021-feedback-author-field` (already created).
- `go test ./...` passes on `main`.
- `cd feedback-worker && npm test` passes on `main`.

## Step 1 — Extend the canonical contract

1. Edit `render/assets/feedback-contract.json`: add
   `"authorMaxChars": 80` inside `limits`.
2. Edit `render/feedback.go`:
   - Add `AuthorMaxChars int` (JSON tag `authorMaxChars`) to
     `feedbackContract.Limits`.
   - Extend the positivity panic in `loadFeedbackContract` to include
     `limits.AuthorMaxChars <= 0`.
   - Add `AuthorMaxChars int` to `FeedbackView`.
   - Set it in `BuildFeedbackView`.
3. Edit `feedback-worker/src/feedback-contract.ts`: add
   `authorMaxChars: number;` to `FeedbackContract.limits` type.

## Step 2 — Worker validation + body composition

4. Edit `feedback-worker/src/validate.ts`:
   - Add `author_required` and `author_too_long` to `ValidationError`.
   - Add `author: string` to `ValidatedPayload`.
   - In `validatePayload`, immediately after the title block, add an
     author block: required + length-cap, returning the trimmed value
     on success.
5. Edit `feedback-worker/src/body.ts`:
   - Prepend `Submitted by: <author>` followed by a blank line as the
     first two lines of the output.

## Step 3 — Worker tests

6. Edit `feedback-worker/test/validate.test.ts`:
   - Cover missing `author` => `author_required`.
   - Cover empty / whitespace-only `author` => `author_required`.
   - Cover over-cap `author` => `author_too_long`.
   - Cover happy path stores trimmed author.
7. Edit `feedback-worker/test/body.test.ts`:
   - Assert the first body line is `Submitted by: <name>`.
   - Assert exactly one blank line separates it from the next block.
8. Update any existing fixtures in
   `feedback-worker/test/{validate,body,index}.test.ts` to include a
   valid `author` field (otherwise existing tests now fail
   `author_required`).

## Step 4 — Frontend dialog markup + view test

9. Edit `render/templates/report.html.tmpl`: insert the new
   `feedback-field-author` block immediately before
   `feedback-field-title`.
10. Edit `render/feedback_test.go` (or add the assertion to the
    existing test) to verify `BuildFeedbackView().AuthorMaxChars`
    matches the contract value.
11. Edit `render/report_feedback_test.go`: add an HTML assertion that
    a tag with `id="feedback-field-author"` is present and that it
    appears before the `feedback-field-title` tag in the rendered
    output.

## Step 5 — Frontend wiring

12. Edit `render/assets/app-js/03.js#initFeedbackDialog`:
    - Add `var authorInput = document.getElementById("feedback-field-author");`.
    - In `updateSubmitEnabled`, gate Submit on
      `authorInput.value.trim().length > 0` AND
      `authorInput.value.length <= LIMITS.authorMaxChars`.
    - In `closeDialog`, reset `authorInput.value = ""`.
    - Add `authorInput.addEventListener("input", onFormContentChange);`.
    - Add `loadPersistedAuthor()` and `savePersistedAuthor()` helpers
      (both `try/catch` wrapped).
    - In `openDialog`, after the existing reset of other fields, if
      `authorInput.value === ""` call `loadPersistedAuthor()` and
      assign.
    - Update `maybePrefixBody` to prepend the `Submitted by: <name>`
      line.
13. Edit `render/assets/app-js/04.js#doSubmit`:
    - In the payload-building block, add
      `payload.author = authorInput.value.trim();`.
    - In the success arm (right where `renderSuccess` is called), call
      `savePersistedAuthor(payload.author);`.
    - To get a reference to `authorInput` from `04.js`, hoist it (it's
      a closure-captured local in the same `initFeedbackDialog` scope
      as `titleInput`, so it is already in scope inside `doSubmit`).

## Step 6 — Cross-cutting checks

14. Run `go test ./...` from the repo root. The
    `tests/coverage/file_size_test.go` will fail loudly if 03.js or
    04.js exceeds 1000 lines. The determinism test will fail if the
    new DOM is non-deterministic. The godoc coverage will fail if the
    new `AuthorMaxChars` field on `FeedbackView` is missing a doc
    comment.
15. From `feedback-worker/`, run `npm test`.
16. Run the pre-push guard manually:
    `bash scripts/hooks/pre-push-constitution-guard.sh`.
17. Update CLAUDE.md, AGENTS.md, and `.specify/feature.json` to point
    at this feature.

## Acceptance walk

1. Build the binary, generate a report against any pt-stalk fixture,
   open the report in a browser.
2. Click "Report feedback" — the new Author field appears at the top,
   marked required, empty (no prior submission). Submit is disabled.
3. Type a name; Submit becomes enabled (Title is also required so
   complete it too).
4. Submit successfully. Verify the GitHub issue body starts with
   `Submitted by: <name>`.
5. Close the dialog, reopen it — the Author field is pre-filled with
   the value you typed.
6. Open a private window, generate the same report, open dialog —
   Author is empty (private mode blocks localStorage). Type, submit,
   verify the worker still creates the issue.
