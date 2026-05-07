# Implementation Plan: Required Author field on Report Feedback

**Branch**: `021-feedback-author-field` | **Date**: 2026-05-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/021-feedback-author-field/spec.md`

## Summary

Closes GitHub issue #56. The Report Feedback dialog gains a required
"Author" input field positioned above Title. The Submit button is gated
by Author non-emptiness in addition to the existing Title gates. The
trimmed Author value rides as a dedicated `author` JSON field in the
worker payload (and as part of the prefilled body in the legacy
`window.open` GitHub fallback). The Cloudflare Worker validates Author
as required + length-capped, and its issue-body composer prepends a
single `Submitted by: <name>` line so triagers can attribute every
issue. The last successfully submitted Author is persisted in
`localStorage` under a stable key and rehydrated on the next dialog
open. The new character cap lives in the canonical
`render/assets/feedback-contract.json` and is consumed by both the Go
renderer's frontend gate and the worker's validator (no duplication).

## Technical Context

**Language/Version**: Go (pinned, see `go.mod`) for the renderer; TypeScript on Cloudflare Workers for the feedback worker; vanilla ES5-style browser JS in the embedded report assets.
**Primary Dependencies**: Standard library on the Go side. On the worker: `vitest` for tests, `wrangler` for deploy. Browser-side: native `localStorage`, `fetch`, `crypto.randomUUID`. No new dependency added by this feature.
**Storage**: Browser `localStorage` (single key, plain text). No server-side persistence beyond the GitHub issue itself.
**Testing**: `go test ./...` for the Go renderer (including `render/feedback_test.go`, `render/report_feedback_test.go`, and the cross-cutting size/godoc/determinism coverage tests). `vitest` for the worker (`feedback-worker/test/*.test.ts`).
**Target Platform**: Single embedded HTML report rendered by the My-gather CLI; opened in a modern browser with `<dialog>`, `localStorage`, and `fetch` available. Cloudflare Workers runtime for the backend.
**Project Type**: CLI tool that emits a self-contained HTML report (`render/`) plus a small companion HTTP backend (`feedback-worker/`).
**Performance Goals**: Dialog open + pre-fill MUST remain visually instant (< 16 ms). Worker payload validation overhead negligible (string length check). No new render-time cost.
**Constraints**: Principle IV (deterministic byte-identical HTML output) — the Author field DOM is a static template element with no clock/random/env input. Principle V (self-contained report) — no new external assets. Principle IX (zero network) — only the existing named-exception POST on Submit. Principle XV (1000-line cap) — keep `render/assets/app-js/03.js` and `04.js` under 1000 lines after edits.
**Scale/Scope**: One frontend dialog edit, one Worker contract edit, one body composer edit. ~10-15 LOC of incremental JS, ~10 LOC of incremental Go, ~20 LOC of incremental TypeScript, plus tests.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Single Static Binary**: No CGO, no new runtime deps in the Go binary. PASS.
- **II. Read-Only Inputs**: No filesystem writes added. PASS.
- **III. Graceful Degradation**: localStorage absence (private mode, quota, security policy) is observable browser-boundary degradation; field still works, only the next-time pre-fill is skipped. Worker reachability degradation continues to use the existing `window.open` fallback path which now also carries the `Submitted by:` line. PASS.
- **IV. Deterministic Output**: New DOM element is a template literal; the contract JSON is embedded; no clock or randomness. The Go view returns DeepEqual values across calls (existing test in `render/feedback_test.go` covers this; the existing tests will fail loudly if a new non-deterministic input slips in). PASS.
- **V. Self-Contained HTML Reports**: No external fetch at view-time. The Author input is plain HTML, no new asset. PASS.
- **VI. Library-First Architecture**: Changes touch `render/` only via new fields on `FeedbackContract` + template; no `cmd/` coupling introduced. Godoc covers the one new exported field on the contract limits struct. PASS.
- **VII. Typed Errors**: Worker adds one new `ValidationError` literal `author_required` / `author_too_long` matching the existing pattern. No bare `fmt.Errorf` for branchable conditions on the Go side. PASS.
- **VIII. Reference Fixtures & Golden Tests**: This feature does not add a new pt-stalk parser. Render-side determinism is exercised by the existing report golden test, which will refresh in this PR with `-update` after explicit review. PASS.
- **IX. Zero Network at Runtime**: No new network call. Reuses the existing named-exception Submit POST. PASS.
- **X. Minimal Dependencies**: No new direct dependency in `go.mod` or `feedback-worker/package.json`. PASS.
- **XI. Reports Optimized for Humans Under Pressure**: The new field appears only inside the Report Feedback dialog (a user-initiated overlay), not in the report's main narrative. PASS.
- **XII. Pinned Go Version**: Unchanged. PASS.
- **XIII. Canonical Code Path**: See Canonical Path Audit below. PASS.
- **XIV. English-Only Durable Artifacts**: All spec text, code identifiers, comments, commit message, and PR text are English-only. PASS.
- **XV. Bounded Source File Size**: Pre-edit, `render/assets/app-js/03.js` is 898 lines and `04.js` is 490 lines. Author wiring lives entirely in `03.js` (input ref, gate, persistence) and `04.js` (payload field). Estimated added LOC: ~25 in `03.js` (still < 1000), ~3 in `04.js`. If a future iteration approaches the cap, the dialog wiring will be extracted into `app-js/05.js`; not required for this feature. PASS.

**Canonical Path Audit (Principle XIII)**:
- Canonical owner/path for touched behaviour:
  - Frontend dialog markup: `render/templates/report.html.tmpl` (the existing static `<dialog>` element).
  - Frontend dialog wiring: `render/assets/app-js/03.js#initFeedbackDialog` (refs, gate, persistence) and `render/assets/app-js/04.js#doSubmit` (payload composition).
  - Shared contract: `render/assets/feedback-contract.json` consumed by both `render/feedback.go#loadFeedbackContract` and `feedback-worker/src/feedback-contract.ts#assertFeedbackContract`.
  - Worker payload validator: `feedback-worker/src/validate.ts#validatePayload`.
  - Worker issue body composer: `feedback-worker/src/body.ts#buildIssueBody`.
- Replaced or retired paths: None. This feature adds one canonical Author field end-to-end. The legacy `window.open` GitHub-prefill body composer (`maybePrefixBody` in `app-js/03.js`) is updated in-place to also emit the `Submitted by:` line so the worker and fallback paths produce equivalent observable bodies; no parallel composer is introduced.
- External degradation paths: (a) `localStorage` absence — observable as an empty Author field on next open, no error shown, covered by US2 acceptance scenario 3. (b) Worker unreachable — existing fallback fires; the Author is included in the prefilled GitHub URL via the updated `maybePrefixBody`, covered by US1 acceptance via the same body line on both paths.
- Review check: Reviewer verifies (i) `authorMaxChars` is defined exactly once in `feedback-contract.json` and consumed by both Go and TypeScript; (ii) the `Submitted by:` literal appears exactly once in `feedback-worker/src/body.ts` and exactly once in the legacy URL builder, with identical formatting; (iii) no shim defaults `payload.author` to `""` or `"Anonymous"` — the worker rejects with 400 `author_required` for missing/empty/whitespace-only; (iv) `closeDialog` resets the Author field to its persisted value (or empty), not to a hardcoded default.

## Project Structure

### Documentation (this feature)

```text
specs/021-feedback-author-field/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── author-field.md  # Phase 1 output
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
render/
├── assets/
│   ├── feedback-contract.json   # +limits.authorMaxChars
│   ├── app-js/
│   │   ├── 03.js                # +author input ref, gate, persistence
│   │   └── 04.js                # +author field on payload
│   └── ...
├── templates/
│   └── report.html.tmpl         # +<input id="feedback-field-author"> above title
├── feedback.go                  # +AuthorMaxChars on contract limits + validation
├── feedback_test.go             # +author cap loaded from contract
└── report_feedback_test.go      # +author input present + above title

feedback-worker/
├── src/
│   ├── feedback-contract.ts     # +authorMaxChars in type + assert
│   ├── validate.ts              # +author required + length cap
│   └── body.ts                  # +Submitted by: line
└── test/
    ├── validate.test.ts         # +author_required + author_too_long
    └── body.test.ts             # +Submitted by line ordering
```

**Structure Decision**: The renderer (`render/`) and the Cloudflare Worker (`feedback-worker/`) remain separate sub-projects sharing one canonical contract JSON file. This is the existing layout; no new top-level directory is introduced.

## Complexity Tracking

> No Constitution Check violations to justify; this section is intentionally empty.
