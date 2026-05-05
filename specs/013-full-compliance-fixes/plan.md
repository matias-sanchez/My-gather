# Implementation Plan: Full Compliance Fixes

**Branch**: `013-full-compliance-fixes` | **Date**: 2026-05-05 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/013-full-compliance-fixes/spec.md`

## Summary

Close every confirmed finding from the post-merge compliance audit by removing
retained duplicate/fallback implementation paths, aligning stale spec wording,
and tightening mechanical enforcement where it was weaker than the constitution.

## Technical Context

**Language/Version**: Go 1.26.2, TypeScript 5.7, Bash, PowerShell, Markdown  
**Primary Dependencies**: Go standard library; Worker dev dependencies already
declared in `feedback-worker/package.json`  
**Storage**: N/A  
**Testing**: `go test`, `go vet`, Vitest with Workers pool, Spec Kit scripts  
**Target Platform**: local repository and GitHub CI  
**Project Type**: Go CLI with TypeScript feedback Worker and Spec Kit tooling  
**Performance Goals**: no runtime performance change  
**Constraints**: no duplicate canonical paths, no hidden fallback behavior, no
new product features, no source file over 1000 lines  
**Scale/Scope**: confirmed findings from the May 5, 2026 audit only

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Static Binary | PASS | No CGO or release build change. |
| II. Preserve Inputs | PASS | No input-path writes added. |
| III. Graceful Degradation | PASS | Malformed netstat timestamps become observable diagnostics. |
| IV. Deterministic Output | PASS | Runtime output behavior remains deterministic. |
| V. Self-Contained HTML | PASS | No external asset change. |
| VI. Library-First | PASS | `reportutil` remains the shared formatting owner. |
| VII. Typed Errors | PASS | No new untyped error surface. |
| VIII. Fixtures and Goldens | PASS | Coverage test is tightened to require parser goldens. |
| IX. Zero Network | PASS | No Go runtime network path added. |
| X. Minimal Dependencies | PASS | No new dependency. Existing Worker test pool dependency is used. |
| XI. Human Pressure Optimization | PASS | Spec wording is aligned to current report structure. |
| XII. Pinned Go Version | PASS | No Go version change. |
| XIII. Canonical Code Path | PASS | Confirmed duplicate/fallback paths are removed. |
| XIV. English-Only | PASS | All artifacts are English. |
| XV. Bounded Source Size | PASS | Source-size gate expands to the governed source surface. |

**Canonical Path Audit (Principle XIII)**:
- Canonical owner/path for touched behaviour: `.specify/feature.json` for
  feature resolution, git extension scripts for branch creation,
  `ParseEnvMeminfoWithDiagnostics` for env meminfo, `reportutil` for shared
  report formatting, Workers Vitest pool for Worker runtime tests.
- Replaced or retired paths: old core feature-creation script, branch-name
  feature-directory fallback, quiet env meminfo wrapper, local InnoDB formatter.
- External degradation paths, if any: feedback Worker failure remains visible
  external degradation documented in feature 003.
- Review check: grep for retired symbols/paths and run full validation.

## Project Structure

### Documentation (this feature)

```text
specs/013-full-compliance-fixes/
├── spec.md
├── plan.md
├── research.md
├── quickstart.md
├── contracts/audit.md
└── tasks.md
```

### Source Code (repository root)

```text
.specify/
├── feature.json
├── memory/constitution.md
├── scripts/bash/common.sh
└── integrations/speckit.manifest.json

AGENTS.md
CLAUDE.md
feedback-worker/vitest.config.ts
parse/
render/
reportutil/
tests/coverage/
specs/001-ptstalk-report-mvp/
specs/003-feedback-backend-worker/
```

**Structure Decision**: apply focused edits in the existing canonical owners;
do not introduce new packages or parallel implementations.

## Complexity Tracking

No constitution violations require justification.
