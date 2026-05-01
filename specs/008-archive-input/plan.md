# Implementation Plan: Archive Input

**Branch**: `008-archive-input` | **Date**: 2026-04-29 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `specs/008-archive-input/spec.md`

## Summary

Allow `my-gather` to accept either a pt-stalk directory or a supported
compressed archive as its single positional input. Archive handling stays in
the CLI boundary: resolve the input, extract supported archives into a private
temporary directory, identify the single extracted pt-stalk root, then pass that
directory into the existing `parse.Discover` pipeline. Directory input remains
unchanged.

## Technical Context

**Language/Version**: Go version per current `go.mod` declaration  
**Primary Dependencies**: Go standard library only (`archive/tar`,
`archive/zip`, `compress/gzip`) plus existing packages  
**Storage**: Reads one input directory or archive and writes one HTML file  
**Testing**: `go test ./cmd/my-gather ./parse`, then `go test ./...`  
**Target Platform**: Static CLI binary for Linux and macOS targets  
**Performance Goals**: Avoid manual extraction for common archive formats;
enforce bounded extraction before parsing; keep directory input cost unchanged  
**Constraints**: Read-only input tree, deterministic root selection, temporary
files removed on exit, self-contained HTML, zero runtime network, no new
third-party dependency  
**Scale/Scope**: One collection per command; multi-host archive selection is
explicitly rejected until a multi-report workflow exists

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Single Static Binary | PASS | Standard library archive readers only. |
| II. Read-Only Inputs | PASS | Archive extraction writes only to private temp directories and never mutates input paths. |
| III. Graceful Degradation | PASS | Unsupported or invalid archives fail with existing documented exit codes and clear stderr. |
| IV. Deterministic Output | PASS | Extracted roots are sorted; multiple roots are rejected instead of selected arbitrarily. |
| V. Self-Contained HTML Reports | PASS | Report rendering path is unchanged. |
| VI. Library-First Architecture | PASS | `parse` exposes a small root-recognition helper; CLI remains the archive boundary. |
| VII. Typed Errors | PASS | Existing parse errors plus typed archive path/multiple-root errors drive exit mapping. |
| VIII. Reference Fixtures & Golden Tests | PASS | Tests package archive fixture data from `testdata/example2`. |
| IX. Zero Network at Runtime | PASS | No network code. |
| X. Minimal Dependencies | PASS | No new module dependency. |
| XI. Reports Optimized for Humans Under Pressure | PASS | Eliminates manual extraction and fails clearly on ambiguous archives. |
| XII. Pinned Go Version | PASS | No Go version change. |
| XIII. Canonical Code Path | PASS | Archive input resolves to a directory before the single canonical parser/render path. |
| XIV. English-Only Durable Artifacts | PASS | All durable artifacts are English. |

## Project Structure

```text
cmd/my-gather/
├── archive_input.go       # archive detection, safe extraction, root selection
├── main.go                # positional wording and input-preparation mapping
└── main_test.go           # archive-input CLI coverage

parse/
└── parse.go               # exported root-recognition helper

specs/008-archive-input/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── cli.md
└── tasks.md
```

## Design Decisions

- Keep archive support out of `parse.Discover` so the parser remains a
  read-only directory parser.
- Add `parse.LooksLikePtStalkRoot` so archive root discovery uses the same
  recognition signals as `Discover` without parsing files twice for every
  directory.
- Reject multiple extracted pt-stalk roots because the current CLI renders one
  collection per invocation and silent root choice could hide the relevant host.
- Support `.gz` both as a single compressed file and, by content detection, as
  a tar stream with a non-standard filename.

## Post-Design Constitution Check

No constitution violations were introduced.
