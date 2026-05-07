# Tasks: Required Author field on Report Feedback

**Feature**: 021-feedback-author-field
**Branch**: `021-feedback-author-field`
**Plan**: [plan.md](plan.md) | **Spec**: [spec.md](spec.md) | **Contracts**: [contracts/author-field.md](contracts/author-field.md)

---

## Phase 1 — Setup

(No project initialization needed; the renderer and worker exist.)

- [ ] T001 Confirm clean baseline: `go test ./...` and `cd feedback-worker && npm test` both pass on the current branch tip.

---

## Phase 2 — Foundational (blocking prerequisites)

These wire the new Author cap into the canonical contract JSON
consumed by both Go and TypeScript. Both user stories depend on
this.

- [ ] T010 Add `"authorMaxChars": 80` inside `limits` in `render/assets/feedback-contract.json` (canonical single source of truth, contract C1).
- [ ] T011 Extend `feedbackContract.Limits` in `render/feedback.go` with `AuthorMaxChars int` (JSON tag `authorMaxChars`); extend the positivity panic in `loadFeedbackContract`; add `AuthorMaxChars int` (godoc-commented) to `FeedbackView`; populate it in `BuildFeedbackView` (contract C3).
- [ ] T012 [P] Extend `FeedbackContract.limits` interface in `feedback-worker/src/feedback-contract.ts` with `authorMaxChars: number` (the existing `assertPositiveInteger` walk catches it automatically) (contract C5).

---

## Phase 3 — User Story 1: Sign your feedback before sending it (Priority: P1)

**Goal**: An Author field appears in the dialog, gates Submit, rides
on the worker payload, and produces a `Submitted by: <name>` line in
the GitHub issue body. Both the worker and the legacy
`window.open` fallback paths emit the same line.

**Independent Test**: Open the dialog, type Author + Title + Body,
Submit. The created GitHub issue body's first line is
`Submitted by: <typed-name>`. With Author empty, Submit is disabled.

### Worker side (validation + body composition)

- [ ] T020 [US1] Add `author_required` and `author_too_long` to `ValidationError`; add `author: string` to `ValidatedPayload`; insert a required+length-cap author block immediately after the title block in `validatePayload` in `feedback-worker/src/validate.ts`. Reject missing/non-string/empty/whitespace-only as `author_required`; reject over-cap as `author_too_long`; store `value.trim()` on success (contract C4).
- [ ] T021 [US1] In `feedback-worker/src/body.ts#buildIssueBody`, prepend two lines to the output: `Submitted by: ${payload.author}` followed by an empty line, before the existing `> Category:` block (contract C6, R3).
- [ ] T022 [US1] Update `feedback-worker/test/validate.test.ts`: (a) add cases for missing/empty/whitespace-only/over-cap author returning the new error codes; (b) update every existing happy-path test fixture to include a valid `author` field so they still pass (contract C4, quickstart Step 3 #6, #8).
- [ ] T023 [P] [US1] Update `feedback-worker/test/body.test.ts`: assert the first line of the composed body is `Submitted by: <author>`, followed by exactly one blank line; update existing test fixtures to pass an `author` (contract C6, quickstart Step 3 #7).
- [ ] T024 [P] [US1] Update `feedback-worker/test/index.test.ts` and any other worker test that constructs a payload to include a valid `author` field (so existing tests do not regress on `author_required`).
- [ ] T025 [P] [US1] Update `feedback-worker/test/github-app.test.ts` and `feedback-worker/test/ratelimit.test.ts` payload fixtures to include `author` if they construct full request bodies.

### Frontend side (markup + wiring)

- [ ] T030 [US1] Edit `render/templates/report.html.tmpl` to insert the Author field block immediately before the `feedback-field-title` block. The input MUST have id `feedback-field-author`, name `author`, type `text`, autocomplete `name`, placeholder `Your name`, attribute `required`, and `maxlength="{{ .Feedback.AuthorMaxChars }}"`. The label reads `Author` with a visible `<span class="feedback-field-req">required</span>` adornment (contract C2).
- [ ] T031 [US1] Edit `render/assets/app-js/03.js#initFeedbackDialog`:
  - Add `var authorInput = document.getElementById("feedback-field-author");`.
  - In `updateSubmitEnabled`, add Author check after the existing `titleLen === 0` gate: disable Submit if `authorInput.value.trim().length === 0` OR `authorInput.value.length > LIMITS.authorMaxChars`.
  - In `closeDialog`, after the existing `titleInput.value = ""; bodyInput.value = ""; catSelect.value = "";`, add `authorInput.value = "";`.
  - Add `authorInput.addEventListener("input", onFormContentChange);` next to the existing input listeners.
  - Update `maybePrefixBody`: prepend `"Submitted by: " + authorInput.value.trim() + "\n\n"` before the existing optional `> Category:` block (contract C8, C9).
- [ ] T032 [US1] Edit `render/assets/app-js/04.js#doSubmit`: in the payload-building block, add `payload.author = authorInput.value.trim();` (contract C7).
- [ ] T033 [US1] Edit `render/feedback_test.go`: add an assertion that `BuildFeedbackView().AuthorMaxChars` equals the value in the embedded `feedback-contract.json` (proves the Go side reads from the canonical source, not a hardcoded constant).
- [ ] T034 [US1] Edit `render/report_feedback_test.go`: add assertions that the rendered HTML contains an element with `id="feedback-field-author"` AND that this element appears before the `feedback-field-title` element (substring index check on the rendered output).

**Checkpoint**: After T020–T034, run `go test ./...` and
`cd feedback-worker && npm test`. Both must pass. Manual smoke: build,
generate report, open dialog — Author field visible above Title,
Submit disabled until Author + Title both non-empty, successful
submission produces issue body whose first line is
`Submitted by: <name>`.

---

## Phase 4 — User Story 2: Don't retype your name every time (Priority: P2)

**Goal**: The last successfully submitted Author value is stored in
`localStorage` and pre-fills the field on next dialog open. Private
mode / quota errors degrade silently to "field is empty".

**Independent Test**: Submit feedback once with Author "Jane". Close
dialog. Reopen — Author field shows "Jane". Open in a private window
where localStorage is blocked — Author field is empty, submission
still works.

- [ ] T040 [US2] In `render/assets/app-js/03.js#initFeedbackDialog`, add two helpers (both `try/catch`-wrapped so a localStorage exception silently degrades to "no persistence"):
  - `function loadPersistedAuthor() { try { var v = window.localStorage.getItem("mygather.feedback.lastAuthor") || ""; return v.length > LIMITS.authorMaxChars ? v.slice(0, LIMITS.authorMaxChars) : v; } catch (_) { return ""; } }`.
  - `function savePersistedAuthor(v) { try { if (v) window.localStorage.setItem("mygather.feedback.lastAuthor", v); } catch (_) {} }`.
- [ ] T041 [US2] In `render/assets/app-js/03.js#openDialog`, after `form.hidden = false;` and before `dialog.showModal();`, add: `if (authorInput.value === "") { authorInput.value = loadPersistedAuthor(); }`.
- [ ] T042 [US2] In `render/assets/app-js/04.js#doSubmit`, in the worker-success arm right where `renderSuccess(data.issueUrl, data.issueNumber);` is called, add `savePersistedAuthor(payload.author);` immediately before `renderSuccess(...)` so the value is captured on definitive success only (R5).
- [ ] T043 [US2] Add browser-DOM coverage for the persistence behaviour. Since the existing repo has no JS test runner for the embedded report assets, document the manual acceptance walk in the quickstart (already present in Step 6 of `specs/021-feedback-author-field/quickstart.md`) and add a Go-side test in `render/feedback_test.go` (or a new file) that asserts the literal string `mygather.feedback.lastAuthor` appears exactly twice in the embedded JS bundle (once for `getItem`, once for `setItem`) — preventing typo drift between read and write call sites.

**Checkpoint**: After T040–T043, manual walk: submit once, close,
reopen — field pre-filled. Open in private window — field empty,
submission still succeeds.

---

## Phase 5 — Polish & Cross-Cutting

- [ ] T050 Run `go test ./...` from the repo root. The size test (`tests/coverage/file_size_test.go`), godoc coverage, and determinism test all must pass. Fix any failure caused by the diff.
- [ ] T051 Run `cd feedback-worker && npm test`. All tests must pass.
- [ ] T052 Run `bash scripts/hooks/pre-push-constitution-guard.sh` from the repo root. All gates must pass.
- [ ] T053 Confirm CLAUDE.md, AGENTS.md, and `.specify/feature.json` already point at `021-feedback-author-field` (done in `/speckit.plan` step). Re-verify with `cat .specify/feature.json` and `head -15 CLAUDE.md AGENTS.md`.
- [ ] T054 Update any committed `.golden` snapshots whose rendered HTML now includes the new Author field block, by running `go test ./... -update` once and reviewing the diff to confirm only the dialog markup changed (Principle VIII: golden refresh is an explicit reviewed step).
- [ ] T055 Commit, push, open PR titled `Required Author field on Report Feedback (#56)` with body referencing GitHub issue #56 and summarising the four touched surfaces (contract JSON, Go view, frontend dialog wiring, Cloudflare Worker validate+body+contract).

---

## Dependencies

- Phase 1 (T001) is informational; failures here block everything.
- Phase 2 (T010, T011, T012) is blocking foundation. Both user stories
  read `LIMITS.authorMaxChars` from the contract; without T010 the
  field cannot be defined; without T011 the Go side cannot expose it
  to the template; without T012 the worker cannot validate it.
- Phase 3 (US1) depends on Phase 2 complete.
- Phase 4 (US2) depends on Phase 3 complete (the Author field must
  exist in the dialog markup before persistence can pre-fill it).
- Phase 5 polish runs after both stories are implemented.

## Parallel opportunities

- T012 can run in parallel with T011 (different sub-projects).
- T023, T024, T025 can run in parallel with each other (each updates
  a separate worker test file).
- T030 (HTML template) is independent of T020–T025 (worker side) and
  could be drafted in parallel, but Phase 3 is best executed worker-
  first so the JS gate matches a worker contract that already
  validates author.

## Implementation strategy

- **MVP scope**: Phase 1 + 2 + 3 (User Story 1) only. That is the
  full content of GitHub issue #56: "Author field is required, body
  carries `Submitted by:`."
- **Incremental delivery**: Phase 4 (US2 — localStorage pre-fill) is
  a UX papercut fix and can ship in the same PR or a follow-up. Per
  the user decision sheet for this feature it ships in the same PR.
- **Constitution gates**: T050–T052 run all mechanical gates (file
  size, godoc, determinism, English-only, CGO/network/write
  forbidden in restricted packages). Fix red gates before push.
