# Implementation Plan: Repository Alignment Audit

**Branch**: `012-repo-alignment-audit` | **Date**: 2026-05-04 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/012-repo-alignment-audit/spec.md`

## Summary

Run a six-dimension repository audit, consolidate findings, apply minimal
confirmed fixes, and document any remaining follow-ups that require dedicated
source design.

## Technical Context

**Language/Version**: Go version per current `go.mod`; TypeScript for the
feedback Worker
**Primary Dependencies**: No new dependencies
**Storage**: Checked-in Markdown, Go/TypeScript source, CI configuration
**Testing**: `go test -count=1 ./...`, Worker typecheck/tests, constitution
guard, Spec Kit analysis
**Target Platform**: Static CLI binary plus Cloudflare Worker tests
**Project Type**: CLI/report generator with embedded offline report assets
**Constraints**: Minimal fixes only; no feature expansion; no duplicated
implementation paths; no hidden fallback-style behavior
**Scale/Scope**: One audit branch and PR

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Single Static Binary | PASS | No CGO or binary shape change intended. |
| II. Read-Only Inputs | PASS | No input mutation change intended. |
| III. Graceful Degradation | PASS | Existing degradation paths are reviewed; no new path added. |
| IV. Deterministic Output | PASS | Audit remediates wall-clock render drift. |
| V. Self-Contained HTML Reports | PASS | No external asset fetch added. |
| VI. Library-First Architecture | PASS | Runtime fixes land in existing package owners. |
| VII. Typed Errors | PASS | No new branchable Go error taxonomy required. |
| VIII. Reference Fixtures & Golden Tests | PASS | Parser fixture/golden policy unchanged. |
| IX. Zero Network at Runtime | PASS | Feedback exception remains the only outbound path. |
| X. Minimal Dependencies | PASS | No new dependency added. |
| XI. Reports Optimized for Humans Under Pressure | PASS | UI changes are limited to existing feedback success state. |
| XII. Pinned Go Version | PASS | Go line is updated as its own explicit branch change. |
| XIII. Canonical Code Path | PASS | Legacy mysqladmin persistence, duplicate helper paths, feedback contract duplication, and confirmed parser fallback observability gaps are resolved without adding parallel implementations. |
| XIV. English-Only Durable Artifacts | PASS | New durable text is English. |
| XV. Bounded File Size | PASS | No governed source file may exceed 1000 lines. |

## Canonical Path Audit

- **Canonical owner/path**: each touched behavior is edited in its existing owner
  (`render/assets/app-js/*`, `feedback-worker/src/index.ts`, CI/hook files,
  `reportutil`, parser files, Spec Kit artifacts).
- **Replaced or retired paths**: the legacy mysqladmin single-selection
  `localStorage` key, duplicate render/findings helpers, and legacy per-URL
  feedback dialog attributes are removed.
- **External degradation paths**: none introduced.
- **Review check**: verify no duplicate localStorage path, no duplicate helper
  owner, no alternate feedback contract path, and no CI bypass path remains.

## Project Structure

```text
specs/012-repo-alignment-audit/
  spec.md
  plan.md
  tasks.md
  research.md
  quickstart.md
  contracts/audit.md
```

## Post-Design Constitution Check

No planned remediation intentionally weakens a constitution principle.
