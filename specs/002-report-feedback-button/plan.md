# Implementation Plan: Report Feedback Button

**Branch**: `iterations_2304b` (continuing current branch per owner direction) | **Date**: 2026-04-24 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/002-report-feedback-button/spec.md`

## Summary

Add a small "Report feedback" button in the top-right of the report header. Clicking it opens a self-contained native `<dialog>` with title + body + category selector, and attachment capture (clipboard-pasted image, MediaRecorder voice note). On submit:

1. Open a new tab to `https://github.com/matias-sanchez/My-gather/discussions/new?category=ideas&title=<enc>&body=<enc>` (user-initiated navigation; passes popup-blocker heuristics).
2. If an image is attached, best-effort copy it to the OS clipboard as `image/png`. On failure, render an inline download link.
3. If a voice note is attached, trigger a one-click file download of the `.webm`/`.mp4` blob.

All markup and behaviour ship as embedded assets (`//go:embed`). Go-side footprint is minimal: one small template partial and one view struct. The Go binary does no network I/O. Runtime network only occurs when the *user* clicks Submit, and only as a `target=_blank` navigation in the browser.

## Technical Context

**Language/Version**: Go 1.22 (pinned via `go.mod` `go` directive; unchanged). JavaScript ES2017+ (compatible with evergreen Chrome/Firefox/Safari within last two releases). CSS 3 (`grid`, `flex`, `@media`, `:has()`).
**Primary Dependencies**: Go stdlib only (`html/template`, `embed`). Browser primitives: HTML5 `<dialog>`, Clipboard API (`navigator.clipboard.write`), MediaRecorder API (`navigator.mediaDevices.getUserMedia`), Paste events (`ClipboardEvent.clipboardData`). No new Go modules.
**Storage**: None. The feedback draft lives only in the browser's JavaScript memory during the dialog's lifetime; no localStorage, no IndexedDB, no server persistence.
**Testing**: `go test ./...`, golden diff on `testdata/golden/*` (new markup in header), deterministic render test (two builds → byte-identical HTML), manual browser-based acceptance runs (one Chromium family, one Firefox family, on macOS + Linux — tracked in `quickstart.md`).
**Target Platform**: Browser-rendered HTML. Report is served over `file://` or any static HTTP. Binary runs on `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64` (unchanged from Principle I).
**Project Type**: Go CLI that emits a self-contained HTML report (library-first architecture under `parse/`, `model/`, `render/`).
**Performance Goals**: Feature adds ≤ 3 KB gzipped to the shipped binary's embedded assets. Dialog open latency ≤ 50 ms on a mid-range laptop. Voice recording cap 120 s; expected recorded blob ≤ 2 MB at 64 kbps opus.
**Constraints**: Byte-identical HTML output across runs on the same input (Principle IV). No CDN, no external font, no runtime fetch from report (Principle V, IX). Existing reports must keep rendering identically until the feature is enabled — i.e., the feature is additive, not a rewrite of the header.
**Scale/Scope**: Single feature, touching 3–5 files in `render/` and its embedded assets, plus one golden update.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Verdict | Rationale / Mitigation |
|---|---|---|
| I. Single Static Binary | PASS | No CGO, no dyn link. Only Go stdlib + embedded assets. |
| II. Read-Only Inputs | PASS | Feature touches render path only; does not read pt-stalk tree. |
| III. Graceful Degradation | PASS | Dialog guards every capability check: if `navigator.clipboard.write` rejects, inline download link; if `getUserMedia` rejects, record control disabled with error message; popup blocker fallback renders inline link. Text flow never regresses. |
| IV. Deterministic Output | PASS | Button markup is template-static — no `time.Now()`, no random IDs, no map iteration. Category labels are a const slice in display order. Golden tests re-run and diff. |
| V. Self-Contained HTML Reports | PASS | All new CSS/JS/markup embedded via existing `//go:embed` in `render/assets.go`. No CDN, no external font, no remote icon. The GitHub URL is a user-initiated `target=_blank` navigation — not an asset the report fetches. |
| VI. Library-First Architecture | PASS | `render/feedback.go` is a tiny view builder. `cmd/my-gather` gains no new flag. All new exported identifiers receive godoc. |
| VII. Typed Errors | PASS | No new Go error paths — the button is pure template output. (Client-side JS error handling uses DOM-event conventions, not Go errors.) |
| VIII. Reference Fixtures & Golden Tests | PASS | No new parser. Render golden for the full-report test updates by one line (the header additions). No new fixture needed. |
| IX. Zero Network at Runtime | PASS | Binary performs zero network I/O. Browser-side: Submit triggers user-initiated navigation; no `fetch`/`XMLHttpRequest` from the report's runtime code paths. |
| X. Minimal Dependencies | PASS | No new `go.mod require`. All browser APIs are native primitives, not bundled polyfills. |
| XI. Reports Optimized for Humans Under Pressure | PASS | Control is a single small icon-plus-label in the header margin; dialog only materialises on demand. Does not displace or resize the triage-critical sections. |
| XII. Pinned Go Version | PASS | No change to `go.mod` `go` directive. |
| XIII. Canonical Code Path (NON-NEGOTIABLE) | PASS | New feature, no existing code to replace. Single implementation per behaviour; no flags, no dual path. |
| XIV. English-Only Durable Artifacts | PASS | All code, comments, markup, commit messages, doc files English. Category labels English. |

No violations. No entries required in Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/002-report-feedback-button/
├── plan.md              # This file (/speckit.plan output)
├── spec.md              # Written by /speckit.specify
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── ui.md            # Phase 1 output — UI contract
└── tasks.md             # Phase 2 output (/speckit.tasks — not by this command)
```

### Source Code (repository root)

```text
render/
├── feedback.go                  # NEW — view-builder for the header control
├── feedback_test.go             # NEW — unit tests for the view builder
├── render.go                    # MODIFIED — wire feedback view into report ViewModel
├── templates/
│   └── report.html.tmpl         # MODIFIED — header gets the button markup; <dialog> appended to <body>
└── assets/
    ├── app.js                   # MODIFIED — initFeedbackDialog() module
    └── app.css                  # MODIFIED — .feedback-btn, .feedback-dialog, .feedback-* rules

testdata/
└── golden/
    └── *.html                   # MODIFIED — goldens regenerated (additive header + dialog markup)
```

**Structure Decision**: Use the existing library-first Go layout (`render/` package). All new JS and CSS live in `render/assets/`, already embedded via `render/assets.go`'s `//go:embed`. No new packages. No new top-level directories.

### Files touched (exhaustive)

- `render/feedback.go` (new, ~80 LOC) — exports `type FeedbackView struct { GitHubURL string; Categories []string }` and `func BuildFeedbackView() FeedbackView`. The URL is a const; the Categories slice is a const in display order.
- `render/feedback_test.go` (new, ~40 LOC) — tests: categories in deterministic order; URL matches the constant; two successive `BuildFeedbackView()` calls are DeepEqual (no time, no random).
- `render/render.go` (modified, ~5 LOC) — inject `FeedbackView` into the root `ViewModel`.
- `render/templates/report.html.tmpl` (modified) — add the button inside `<header class="app-header">` after `<div class="meta">`; append the `<dialog id="feedback-dialog">` at end of `<body>` so native `showModal()` + focus-trap doesn't interact with the nav drawer.
- `render/assets/app.js` (modified, ~180 LOC) — feedback module: open/close, keyboard (Escape / Cmd-Ctrl+Enter / Enter-in-title), paste handler, MediaRecorder, submit-with-handoff. One IIFE, events delegated from `document`.
- `render/assets/app.css` (modified, ~80 LOC) — button style, dialog positioning, attachments area, recording indicator.
- `testdata/golden/*.html` (modified) — regenerate with `go test ./render/... -update`.

## Complexity Tracking

No violations. Section intentionally empty.

---

## Post-Design Re-check

*Re-verified after Phase 1 artifacts below were produced.*

| Concern | Outcome |
|---|---|
| Determinism of category slice | Categories are a package-level `var categoriesInOrder = []string{...}` — ranged with integer index in template. Deterministic. |
| Dialog markup contains no dynamic ID | Dialog uses a static `id="feedback-dialog"`; JS binds by that constant. No hashing, no counter. |
| Clipboard + MediaRecorder availability | Capability checks guard behaviour; fallback paths documented in `research.md`. No polyfills shipped. |
| Golden churn | Anticipated: every `testdata/golden/*.html` gets an additive diff in the header and a new `<dialog>` block at end of `<body>`. Byte-identical across runs on same input → re-runnable `-update` produces stable output. |

Constitution Check still PASS after design. Proceed to `/speckit.analyze` then `/speckit.tasks`.
