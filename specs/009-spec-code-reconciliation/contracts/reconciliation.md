# Reconciliation Contract

## Active Feature Contract

On branch `009-spec-code-reconciliation`:

- `.specify/feature.json` contains
  `{"feature_directory": "specs/009-spec-code-reconciliation"}`.
- `AGENTS.md` and `CLAUDE.md` active feature blocks name
  `009-spec-code-reconciliation`.
- Both context files list the same artifact paths for the active feature.

## Status Contract

- `006-observed-slowest-queries`, `007-advisor-intelligence`, and
  `008-archive-input` are marked `Complete`.
- `001-ptstalk-report-mvp` remains `Draft` because it still tracks open and
  deferred historical quality work.
- `002-report-feedback-button` remains `Shipped (v1)` with an explicit note
  that its unchecked task list is historical.

## Archive CLI Test Contract

`go test ./cmd/my-gather` must include coverage for:

- supported archive with no pt-stalk root returns `exitNotAPtStalkDir`
- unsupported regular input file returns `exitInputPath`
- unsupported regular input stderr includes supported archive formats

## Validation Contract

Before opening the PR:

```bash
go test ./cmd/my-gather
go test ./...
```
