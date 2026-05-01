# Research: Spec And Code Reconciliation

## Finding: Active Feature Drift

The repository is on a new reconciliation branch, but prior context on `main`
still identified `008-archive-input` as active. Spec Kit branch validation
expects a numbered feature branch, so this branch uses
`009-spec-code-reconciliation` and updates all active pointers together.

## Finding: Completed Specs Still Marked Draft

Features `006-observed-slowest-queries`, `007-advisor-intelligence`, and
`008-archive-input` have task files with every listed task checked. Keeping
their specs as `Draft` makes the documentation disagree with the shipped
implementation.

Decision: mark those specs `Complete`.

## Finding: Feature 002 Task State Is Historical

Feature `002-report-feedback-button` is documented as shipped and partially
superseded by feature 003, but its task list still shows 40 unchecked tasks.
The task file is useful as original planning history, but it is misleading if
read as a live implementation checklist.

Decision: add a clear historical-state note instead of pretending the original
task list maps cleanly to the later reconciled implementation.

## Finding: Archive-Input Test Gaps

Feature `008-archive-input` documents failures for zero-root archives and
unsupported regular files. Existing tests cover supported archive success,
unsafe paths, corrupt archives, extracted size limits, and multiple roots, but
not those two documented branches.

Decision: add two CLI tests in `cmd/my-gather/main_test.go`.

## Finding: UX Checklist Placeholder

Feature 001's UX checklist included a deferred sample row with a placeholder
task ID. That is fine as an example in a template, but in a checked-in feature
checklist it reads like unresolved tracking.

Decision: replace it with an existing concrete task reference from the same
feature's UX audit phase.
