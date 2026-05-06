---

description: "Task list for Report Feedback Button implementation"
---

# Tasks: Report Feedback Button

**Input**: Design documents from `/specs/002-report-feedback-button/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/ui.md, quickstart.md

**Historical state**: This task file preserves the original 2026-04-24
implementation plan for feature 002. The shipped feedback flow was later
reconciled and superseded by feature 003's Worker-backed submit path; do not
read the unchecked boxes below as live remaining work for the current product.
Use `specs/002-report-feedback-button/spec.md` for the still-authoritative
dialog and attachment behavior, and `specs/003-feedback-backend-worker/` for
the authoritative submit path.

**Tests**: Test tasks are included — this feature touches render-layer code, which Principle VIII (Reference Fixtures & Golden Tests) and the constitution's Quality Gate #3 (determinism byte-diff) hard-require tests for.

**Organization**: One phase per user story in priority order (US1 P1 → US2 P2 → US3/4/5 P3). Every story is independently testable per its spec acceptance criteria and its quickstart scenario.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no ordering dependency)
- **[Story]**: Which user story the task belongs to (US1/US2/US3/US4/US5)
- File paths are absolute-from-repo-root

## Path Conventions

This is a Go library-first project. Paths are rooted at the repo root (`/Users/matias/git/My-gather/My-gather/`). Source lives under `render/`, templates under `render/templates/`, embedded assets under `render/assets/`, tests colocated with sources. Golden files live under `testdata/golden/`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Minimal scaffolding — this feature reuses the existing `render/` package, so setup is nearly empty.

- [ ] T001 Add `render/feedback.go` skeleton with the `FeedbackView` struct signature and `BuildFeedbackView` function stub (returns zero value) — no logic yet, compilable. Satisfies Principle VI godoc requirement with a placeholder godoc comment.
- [ ] T002 [P] Add `render/feedback_test.go` skeleton with a single failing test `TestBuildFeedbackViewIsDeterministic` that calls `BuildFeedbackView()` twice and asserts `reflect.DeepEqual`. Run `go test ./render/... -run TestBuildFeedbackViewIsDeterministic` — confirm it fails (RED).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Wire the view into the root ViewModel so every downstream template change has something to reference.

**⚠️ CRITICAL**: Every user story phase below depends on T003 + T004 being complete.

- [ ] T003 In `render/render.go`, add a `Feedback FeedbackView` field to the root `ViewModel` struct and populate it in the ViewModel builder by calling `BuildFeedbackView()`. Keep the field nil-safe (zero-value works until T005 fills in real values).
- [ ] T004 In `render/feedback.go`, implement `BuildFeedbackView` to return the real constants per `data-model.md`: `GitHubURL = "https://github.com/matias-sanchez/My-gather/discussions/new?category=ideas"` and `Categories = []string{"UI", "Parser", "Advisor", "Other"}`. The test from T002 must turn GREEN.
- [ ] T005 [P] Add a second test in `render/feedback_test.go`: `TestFeedbackViewShape` — asserts `GitHubURL` equals the documented constant and `Categories` equals the documented slice in exact order. This guards against silent reshaping.
- [ ] T006 Run `go vet ./...` and `go test ./render/... -count=1`. Both must pass before any Phase 3+ work begins.

**Checkpoint**: Foundation ready — Phase 3 onward can now start.

---

## Phase 3: User Story 1 — Text-only feedback (Priority: P1) 🎯 MVP

**Goal**: Clicking the header button opens a modal dialog; title + body submit opens a pre-filled GitHub new-discussion tab.

**Independent Test**: Quickstart Scenario A. Open a rendered report with network off, click "Report feedback", fill title + body, Submit; verify new tab opens with URL-encoded values.

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation.**

- [ ] T007 [P] [US1] In `render/feedback_test.go`, add `TestFeedbackHeaderButtonMarkupPresent` — renders a minimal ViewModel via the existing render test harness, asserts the output contains `id="feedback-open"`, `aria-haspopup="dialog"`, and the literal text "Report feedback".
- [ ] T008 [P] [US1] Add `TestFeedbackDialogMarkupPresent` in the same file — asserts the output contains `id="feedback-dialog"`, `id="feedback-field-title"`, `id="feedback-field-body"`, `id="feedback-field-category"`, and the disabled submit button.
- [ ] T009 [P] [US1] Add `TestFeedbackMarkupIsDeterministic` — renders the full report twice via the existing render harness on the same input, asserts byte-identical output (Principle IV). This will also cover US2–US5 since the markup is all static.

### Implementation for User Story 1

- [ ] T010 [US1] In `render/templates/report.html.tmpl`, inside `<header class="app-header">` and after the `<div class="meta">` child, insert the feedback-open `<button>` per `contracts/ui.md`. Use `.Feedback.*` template references only — no raw string literals that could drift from the Go constants.
- [ ] T011 [US1] In `render/templates/report.html.tmpl`, append the `<dialog id="feedback-dialog">` markup at the end of `<body>` per `contracts/ui.md`. The `<select>` options are rendered with `{{range $i, $c := .Feedback.Categories}}<option value="{{$c}}">{{$c}}</option>{{end}}`.
- [ ] T012 [US1] In `render/assets/app.css`, add styling for `.feedback-btn` (small pill in header), `.feedback-dialog` + `.feedback-dialog::backdrop`, `.feedback-actions`, form field layout. Target ≤ 80 LOC (Principle XI — the button should be quiet, not loud).
- [ ] T013 [US1] In `render/assets/app.js`, add an IIFE module `initFeedbackDialog()` wired from the existing bootstrap section. Implement:
  - open: click on `#feedback-open` → `document.getElementById("feedback-dialog").showModal()`; focus title input.
  - close: Cancel button, backdrop click, Escape (native). On close, clear all field values + attachments + revoke any object URLs.
  - submit-enable state: title non-empty AND URL length ≤ 7500 → Submit button enabled.
  - submit: compute URL per `data-model.md`'s shape (`maybePrefixCategory` helper); `window.open(url, "_blank", "noopener,noreferrer")`. If the return value is null, render inline `<a>` fallback per FR-015.
  - keyboard: `Ctrl/Cmd+Enter` anywhere in the dialog → submit (matching the editor clarification). Enter inside the title input → submit. Enter inside the textarea → native newline.
- [ ] T014 [US1] Manual Quickstart Scenario A pass on Chrome/Edge. Record result in a short note attached to the PR description (not a file change).
- [ ] T015 [US1] Run `make build` and `go test ./... -count=1`. Regenerate goldens with `go test ./render/... -update` once, review the diff carefully (expect additive header markup + appended `<dialog>` block). Commit the updated goldens in the same commit as T010–T013.

**Checkpoint**: MVP is shippable. Text flow works offline end-to-end.

---

## Phase 4: User Story 2 — Keyboard-only flow (Priority: P2)

**Goal**: Every open/close/submit action reachable by keyboard alone, with visible focus rings and native focus trap.

**Independent Test**: Quickstart Scenario B. Tab through header, Enter on button, Tab between fields, Cmd/Ctrl+Enter submits, Escape returns focus to button.

### Tests for User Story 2

- [ ] T016 [P] [US2] In `render/feedback_test.go`, add `TestFeedbackButtonIsFocusable` — the rendered button has `type="button"` and no `tabindex="-1"`. Covers the Tab-order requirement from FR-008.
- [ ] T017 [P] [US2] Add `TestFeedbackDialogHasAccessibleLabelling` — dialog has `aria-labelledby` and `aria-describedby` pointing at existing IDs inside the dialog.

### Implementation for User Story 2

- [ ] T018 [US2] In `render/assets/app.css`, ensure `.feedback-btn:focus-visible` has a visible focus ring using the existing `--focus` token palette.
- [ ] T019 [US2] In `render/assets/app.js`, extend the submit handler added in T013 to also bind `keydown` on the dialog for Ctrl/Cmd+Enter. Ensure the submit path unifies both triggers (no duplicated branches per Principle XIII).
- [ ] T020 [US2] Manual Quickstart Scenario B pass on Chrome/Edge and Firefox.

**Checkpoint**: Keyboard flow validated. No regression to mouse flow.

---

## Phase 5: User Story 3 — Optional category marker (Priority: P3)

**Goal**: Category selector value is prepended to body as `> Category: <name>\n\n<body>` before URL-encoding.

**Independent Test**: Quickstart Scenario C (category portion).

### Tests for User Story 3

- [ ] T021 [P] [US3] In `render/feedback_test.go`, add `TestFeedbackCategoriesRenderInOrder` — asserts the rendered `<select>` contains exactly the expected `<option>` tags in the exact order from `.Feedback.Categories`.

### Implementation for User Story 3

- [ ] T022 [US3] In `render/assets/app.js`, implement the `maybePrefixCategory(body, category)` helper exactly as in `data-model.md`. Wire it into the submit URL builder from T013 — single call site, no dual code path.
- [ ] T023 [US3] Manual Quickstart Scenario C (category portion) pass. Verify the GitHub URL's decoded body starts with `> Category: <name>\n\n`.

**Checkpoint**: Category triage marker works.

---

## Phase 6: User Story 4 — Clipboard image paste (Priority: P3)

**Goal**: Pasting an image into the dialog attaches a thumbnail; Submit best-effort copies the image to the OS clipboard for pasting into GitHub.

**Independent Test**: Quickstart Scenario C (image portion) on Chromium; Scenario H on a browser with `clipboard.write` blocked.

### Implementation for User Story 4

- [ ] T024 [US4] In `render/assets/app.js`, add a `paste` listener on the dialog element per research.md R3. First `image/*` item in `event.clipboardData.items` → `URL.createObjectURL(blob)` → render `<img>` thumbnail in `.feedback-attachments`. Replacing an existing image revokes the previous object URL (FR-016 one-image-per-draft).
- [ ] T025 [US4] Add a Remove button per thumbnail that revokes the object URL and clears `draft.imageBlob`.
- [ ] T026 [US4] Extend the submit handler: if `draft.imageBlob` is non-null, `await navigator.clipboard.write([new ClipboardItem({"image/png": draft.imageBlob})])`. Wrap in try/catch. On catch, render an inline `<a href="<object URL>" download="feedback-image-<id>.png">` per FR-021.
- [ ] T027 [US4] Extend the Submit-button gating logic to also update the `#feedback-hint` with a single-line "Image will be on your clipboard — paste into GitHub's body." when an image is attached (FR-019).
- [ ] T028 [US4] Manual Quickstart Scenario C (image portion) on Chrome/Edge. Manual Scenario H on Firefox (disable clipboard-write via about:config if needed) to verify the download fallback.

**Checkpoint**: Image hand-off works on modern browsers; degrades cleanly.

---

## Phase 7: User Story 5 — Voice note recording (Priority: P3)

**Goal**: Record up to 10 minutes of voice inside the dialog; Submit triggers a download of the recording.

**Independent Test**: Quickstart Scenario D on Firefox.

### Implementation for User Story 5

- [ ] T029 [US5] In `render/assets/app.js`, add Record/Stop UI handlers. Record: `navigator.mediaDevices.getUserMedia({audio:true})`, `new MediaRecorder(stream)`, subscribe to `dataavailable`. On Stop or 10-minute cap, concatenate chunks, store as `draft.voiceBlob`, render an `<audio controls>` + Remove control. Revoke stream tracks (`track.stop()`).
- [ ] T030 [US5] Elapsed-time counter: drive via `requestAnimationFrame`, not `setInterval`. Format as `M:SS / 10:00`.
- [ ] T031 [US5] Extend submit handler: if `draft.voiceBlob` is non-null, create `<a href="<object URL>" download="feedback-voice-<crypto.randomUUID().slice(0,8)>.<ext>">`, append to the document, `.click()`, `remove()`. Extension is `webm` or `mp4` based on `draft.voiceBlob.type`.
- [ ] T032 [US5] Microphone-permission denial: catch the `getUserMedia` rejection, render an inline error in `.feedback-attachments`, re-enable the Record button so the user can retry after changing browser permission.
- [ ] T033 [US5] Extend `.feedback-hint` logic from T027: when voice is attached, append "Voice note will be downloaded — drag it into GitHub's body." When both image and voice are attached, show both lines (FR-019).
- [ ] T034 [US5] Manual Quickstart Scenario D on Firefox, then on Chrome/Edge. Verify GitHub's body field accepts the drag-dropped recording and inlines it.

**Checkpoint**: All 5 user stories work independently; integration preserved.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Final verification pass. No new feature work here.

- [ ] T035 Run `make build` and `go test ./... -count=1`. Run `make determinism` if available, or the existing "render twice, diff" golden test. Output MUST be byte-identical across two back-to-back runs on the same input.
- [ ] T036 Run `@agent-pre-review-constitution-guard` on the working tree. Expect verdict: READY TO PUSH. If any P1 or P2 surfaces, return to the offending phase.
- [ ] T037 Manually walk the full Quickstart acceptance checklist (Scenarios A–H). Mark each item. Attach the checklist to the PR description.
- [ ] T038 [P] Update `CHANGELOG.md` with a short "Added: Report feedback button with image/voice attachments" entry (English-only per Principle XIV).
- [ ] T039 Commit each checkpoint as its own commit for review clarity (one per phase, or one per story if Phase 3–7 are small enough). Never `git add -A`; stage specific paths.
- [ ] T040 Open a PR against `main` with a body that cites the spec, the plan, and the quickstart checklist. Use the existing `/pr-review-trigger-my-gather` skill to start the external review round.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 Setup**: No dependencies — can start immediately.
- **Phase 2 Foundational**: Depends on Phase 1 completion — BLOCKS all user-story phases.
- **Phase 3 (US1)**: Depends on Phase 2. Ships MVP.
- **Phase 4 (US2)**: Depends on Phase 3 (adds handlers to the same JS module).
- **Phase 5 (US3)**: Depends on Phase 3 (extends submit URL builder).
- **Phase 6 (US4)**: Depends on Phase 3 (adds attachment UI to the same dialog).
- **Phase 7 (US5)**: Depends on Phase 3. Independent from US4 but shares the `.feedback-attachments` container.
- **Phase 8 Polish**: Depends on every desired user-story phase being complete.

Note: US4 and US5 both touch `render/assets/app.js`. They are NOT [P] relative to each other. They are [P] in the sense that their *tests* could be written in parallel, but the implementation must be sequenced to avoid merge churn inside the same file.

### User-Story Dependencies

- US1 (P1): strictly after Phase 2. Standalone.
- US2 (P2): strictly after US1 — extends the dialog's JS handlers, not independent.
- US3 (P3): strictly after US1 — extends the submit URL builder.
- US4 (P3): strictly after US1. Independent of US2/US3/US5 from a user-value perspective; code-wise shares `app.js`.
- US5 (P3): strictly after US1. Independent of others from a user-value perspective; code-wise shares `app.js`.

### Within Each Phase

- Tests written first (red), implementation makes them green (Quality Gate, Principle VIII).
- Template changes before JS changes (so the JS has something to bind to).
- CSS can land alongside or after template changes; cannot precede them.

### Parallel Opportunities

- T001 ∥ T002 (setup skeletons touch different files).
- T007 ∥ T008 ∥ T009 (tests in the same file but they can be authored as a batch in one edit).
- T016 ∥ T017 (tests, different assertions).
- T021 standalone [P].
- Within Phase 6 and Phase 7, no intra-phase [P] — everything lands in the same JS file.

---

## Parallel Example: Phase 2 Foundational

```bash
# T005 is [P] — add a second render/feedback_test.go test alongside T004.
# No parallelism across files at this stage; the Go files land in order.
```

---

## Implementation Strategy

### MVP First (US1 only)

1. Phase 1 + Phase 2 (wire the view, no UI).
2. Phase 3 (render button + dialog + text submit).
3. Run Quickstart Scenario A on Chrome/Edge + Firefox.
4. Ship as MVP commit.

### Incremental Delivery

5. Phase 4 → Scenario B pass.
6. Phase 5 → Scenario C (category portion) pass.
7. Phase 6 → Scenario C (image) + Scenario H (clipboard-blocked) pass.
8. Phase 7 → Scenario D pass.
9. Phase 8 polish + PR.

### Parallel Team Strategy (if more than one developer)

Not recommended for this feature. The surface area is small and concentrated in a single JS file; parallel branches would just create merge friction.

---

## Notes

- Tests colocated with code (Go convention) — no separate `tests/` hierarchy for this feature.
- Every template change MUST be followed by `go test ./render/... -update` in a dedicated commit so golden churn is reviewable.
- No new Go dependencies; no new top-level directories; no changes to `go.mod` (Principles X, XII).
- Commit boundaries line up with phase checkpoints for review readability.
