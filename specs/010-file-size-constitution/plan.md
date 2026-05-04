# Implementation Plan: File Size Constitution Guard

**Branch**: `010-file-size-constitution` | **Date**: 2026-05-01 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/010-file-size-constitution/spec.md`

## Summary

Add a constitution article and mechanical test that prevent governed
first-party source code files from exceeding 1000 lines. Bring the repository
into compliance by splitting large embedded report JS/CSS assets without
changing runtime behavior.

## Technical Context

**Language/Version**: Go version per current `go.mod` declaration
**Primary Dependencies**: Go standard library only
**Storage**: Checked-in Markdown, Go tests, embedded JS/CSS assets
**Testing**: `go test -count=1 ./tests/coverage -run
TestGovernedSourceFileLineLimit`, then `go test -count=1 ./...`
**Target Platform**: Static CLI binary for Linux and macOS targets
**Project Type**: CLI/report generator with embedded offline report assets
**Performance Goals**: No runtime performance regression; asset concatenation
happens at process initialization from embedded strings
**Constraints**: No new dependencies, no network, no report behavior drift, no
raw fixture/reference rewriting
**Scale/Scope**: One constitution amendment, one coverage test, and asset
splitting for JS/CSS

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Single Static Binary | PASS | Assets still embed into the binary. |
| II. Read-Only Inputs | PASS | No input-tree behavior changes. |
| III. Graceful Degradation | PASS | No parser behavior changes. |
| IV. Deterministic Output | PASS | Asset parts concatenate in sorted order. |
| V. Self-Contained HTML Reports | PASS | Reports still inline CSS and JS. |
| VI. Library-First Architecture | PASS | Adds a coverage test and small embed helper. |
| VII. Typed Errors | PASS | No branchable runtime errors added. |
| VIII. Reference Fixtures & Golden Tests | PASS | Raw fixtures and goldens are exempt and unchanged. |
| IX. Zero Network at Runtime | PASS | No network code added. |
| X. Minimal Dependencies | PASS | No project dependency added. |
| XI. Reports Optimized for Humans Under Pressure | PASS | Report UI unchanged. |
| XII. Pinned Go Version | PASS | No Go version change. |
| XIII. Canonical Code Path | PASS | `render/assets.go` is the single embed/concat owner for app assets; old monolithic app asset paths are deleted. |
| XIV. English-Only Durable Artifacts | PASS | New durable text is English. |
| XV. Bounded File Size | PASS | This feature introduces and satisfies the new rule. |

## Project Structure

```text
.specify/memory/constitution.md
AGENTS.md
CLAUDE.md
.specify/feature.json
render/assets.go
render/assets/app-js/
render/assets/app-css/
tests/coverage/file_size_test.go
specs/010-file-size-constitution/
```

## Design Decisions

- The rule applies to governed first-party source code files, not specs, docs,
  JSON data, raw evidence, or generated snapshots.
- JS and CSS remain one inline block each in rendered reports; only their source
  storage is split.
- Asset part order is lexical by filename, making the concatenation stable and
  auditable.

## Canonical Path Audit

- **Canonical owner/path**: `render/assets.go` embeds and concatenates ordered app
  asset parts from `render/assets/app-js/` and `render/assets/app-css/`.
- **Replaced or retired paths**: the maintained monolithic paths
  `render/assets/app.js` and `render/assets/app.css` are deleted by this change.
- **External degradation paths**: N/A. The output contract remains one inline CSS
  block and one inline JS block.
- **Review check**: reviewers verify the deleted monolithic app assets are not
  retained, no alternate app asset loader exists, and tests cover stable
  concatenation order.

## Post-Design Constitution Check

No constitution violations were introduced.
