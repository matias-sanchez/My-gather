# Tasks: Top CPU Processes Caption

**Input**: Design documents from `specs/018-top-cpu-caption/`
**Prerequisites**: plan.md, spec.md, contracts/caption.md, quickstart.md, research.md

**Tests**: Required. The caption asserts a runtime invariant; the
invariant must remain locked by a regression test.

## Canonical Path Metadata *(Principle XIII)*

- **Canonical owner/path**:
  - Caption: `render/templates/os.html.tmpl` (`sub-os-top` block).
  - Mysqld pin: `render/concat.go::concatTop` (unchanged).
- **Old path treatment**: no prior caption existed; no replacement
  required. Pin is reused as-is.
- **External degradation evidence**: caption shares the chart's
  `if .HasTop` gate; missing/unparseable top data shows the existing
  "Data not available" banner instead.

## Phase 1: Setup

- [x] T001 Create feature artifacts under `specs/018-top-cpu-caption/`.
- [x] T002 Point active feature metadata at `018-top-cpu-caption` (`.specify/feature.json`).

## Phase 2: Verify Existing Pin Behavior

- [x] T003 Inspect `render/concat.go::concatTop` and confirm the
      `isMysqldCommand` pin already appends mysqld to
      `Top3ByAverage` when it is not in the top 3. Capture evidence
      in `research.md`.

## Phase 3: Implementation

- [x] T004 Add the caption `<p class="chart-caption">…</p>` in
      `render/templates/os.html.tmpl`, inside the `sub-os-top`
      `<details>` body, inside the `if .HasTop` branch, immediately
      above the chart container `<div id="chart-top">`. Exact text
      per `contracts/caption.md`.
- [x] T005 Add a `.chart-caption` style rule in the appropriate
      ordered CSS source part under `render/assets/app-css/` (small
      muted-text block above the chart). Skip if an existing class
      already produces the desired appearance.
- [x] T006 Add new regression test
      `TestConcatTopMysqldPinnedWhenLowest` in `render/concat_test.go`
      that constructs a snapshot stream where `mysqld` is the
      lowest-CPU process and asserts mysqld is in
      `Top3ByAverage` after `concatTop`.
- [x] T007 Add new caption-presence test in `render/os_test.go`
      (or the closest existing renderer test file) that renders the
      OS section with non-empty top data and asserts the exact
      caption string appears, and renders again with `HasTop` false
      and asserts the caption string does NOT appear.

## Phase 4: Side-by-side Agent Contract

- [x] T008 Update `CLAUDE.md` and `AGENTS.md` to point the active
      feature at `018-top-cpu-caption` and add `015-compliance-closure`
      and `016-remove-collection-size-cap` to the prior-features list
      if not already present.

## Phase 5: Validation

- [x] T009 Run `go test -count=1 ./...` and fix any failures.
- [x] T010 Run `scripts/hooks/pre-push-constitution-guard.sh` and fix
      any findings.
- [x] T011 Render a fixture report and grep for the caption string
      (per `quickstart.md`).
- [x] T012 Commit, push, open PR with title
      `Top CPU processes chart caption (#54)` referencing issue #54.
