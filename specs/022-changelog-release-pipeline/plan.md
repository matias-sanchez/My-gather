# Implementation Plan: Changelog-driven release pipeline + v0.4.0 backfill

**Branch**: `022-changelog-release-pipeline` | **Date**: 2026-05-07 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/022-changelog-release-pipeline/spec.md`

## Summary

Wire `CHANGELOG.md` into the GitHub release pipeline so the GitHub
Release page body is the curated section for the pushed tag, and add
a release-time gate that fails the workflow when the tag has no
matching `## vX.Y.Z (YYYY-MM-DD)` heading. Backfill the
`## v0.4.0 (2026-05-07)` section above `## v0.3.1` covering the six
features (016-021) merged into main since v0.3.1.

Approach: a single canonical shell script under `scripts/release/`
extracts the section for a given tag from `CHANGELOG.md`. The
existing `publish-release` job in `.github/workflows/ci.yml` gains an
`actions/checkout` step (currently absent), runs the script to write
`dist/RELEASE_NOTES.md`, and passes `body_path: dist/RELEASE_NOTES.md`
to the existing pinned `softprops/action-gh-release@v2.0.8`. A new
job `changelog-gate` runs `grep -F` for the literal `## ${TAG} (`
heading on tag pushes and is added to `publish-release`'s `needs:`.
A Go regression test under `tests/release/` exec's the same script
against the in-repo `CHANGELOG.md` and asserts on its output for both
`v0.3.1` and `v0.4.0`.

## Technical Context

**Language/Version**: Go 1.23 (pinned in `go.mod`); POSIX `sh` + `awk`
for the extraction script; GitHub Actions YAML.
**Primary Dependencies**: `softprops/action-gh-release@v2.0.8` (pinned,
unchanged); `actions/checkout@v4` (already used elsewhere in the
workflow); `actions/download-artifact@v4` (already used). No new
third-party Go modules. No new GitHub Actions.
**Storage**: N/A. The release notes file `dist/RELEASE_NOTES.md` is
ephemeral, materialised in the runner's workspace and consumed in the
same job.
**Testing**: `go test ./tests/release/...` exec's the shell script via
`os/exec`. The shell script is also exercised in CI by being the
production extraction path.
**Target Platform**: GitHub Actions `ubuntu-latest` runner for the
release jobs. Local developers (macOS, Linux) for the regression test.
**Project Type**: Tooling / release pipeline. No application code
changes.
**Performance Goals**: N/A. Extraction runs once per release tag.
**Constraints**: Zero new third-party Go dependencies (Principle X).
Single static binary unaffected (Principle I) - this is workflow +
docs + a shell script + a Go test, none of which alter the shipped
binary. English-only artifacts (Principle XIV). No fallback paths
(Principle XIII): the extraction script is the only parser of
`CHANGELOG.md`; CI YAML calls it, the test calls it, no inline `awk`.
**Scale/Scope**: One new shell script (~30 lines), one new Go test
file (~80 lines), edits to one workflow file, one new section in
`CHANGELOG.md`, three context-file pointer updates.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Walking the 15 principles:

- **I. Single Static Binary**: Unaffected. The change is workflow
  + docs + a shell script + a Go test; none touch the Go binary's
  build path.
- **II. Read-Only Inputs**: Unaffected. The extraction script reads
  `CHANGELOG.md` (a tracked artifact, not a pt-stalk input) and
  writes to `dist/RELEASE_NOTES.md` inside the runner's workspace.
- **III. Graceful Degradation**: N/A. This feature is the release
  pipeline; "graceful degradation" applies to runtime parsing of
  user inputs, not to a maintainer-time release-tag workflow. The
  gate is intentionally fail-fast: a missing changelog section is
  a hard failure, not a degraded path.
- **IV. Deterministic Output**: The extraction script produces
  byte-identical output on byte-identical `CHANGELOG.md` input
  (POSIX `awk` operating in a single-byte locale; we set
  `LC_ALL=C`).
- **V. Self-Contained HTML Reports**: Unaffected.
- **VI. Library-First Architecture**: N/A. The extraction is a
  10-line shell script, not Go code. Adding it to the Go renderer
  would violate the "thin CLI over libraries" intent and Principle
  X (minimal Go dependencies); a shell script is the correct tool
  for "given a markdown file and a tag, print the section". The
  Go test exec's it, which means the contract is enforced
  type-safely from the test side.
- **VII. Typed Errors**: Unaffected. The Go test uses standard
  `t.Fatalf` / `t.Errorf`; no new error types are added.
- **VIII. Reference Fixtures & Golden Tests**: N/A. No new collector
  parser. The regression test under `tests/release/` exercises the
  in-repo `CHANGELOG.md` directly rather than golden-diffing
  because the input IS the artifact being tested - a separate
  golden would just duplicate the source of truth and bit-rot.
- **IX. Zero Network at Runtime**: Unaffected. The extraction
  script is filesystem-only. The CI workflow already does network
  in the publish step (uploading artifacts to the GitHub release);
  this feature does not add new network calls.
- **X. Minimal Dependencies**: No new Go modules. No new GitHub
  Actions. The shell script uses POSIX `sh` + `awk`, both
  pre-installed on `ubuntu-latest` and macOS.
- **XI. Reports Optimized for Humans Under Pressure**: N/A
  (release pipeline, not the report).
- **XII. Pinned Go Version**: Unaffected.
- **XIII. Canonical Code Path (NON-NEGOTIABLE)**: This is the
  load-bearing principle for this feature. Audit follows below.
- **XIV. English-Only Durable Artifacts**: All new artifacts
  (spec, plan, tasks, research, quickstart, the shell script, the
  Go test, the v0.4.0 changelog section) are English-only.
- **XV. Bounded Source File Size**: All new files are well under
  1000 lines. `CHANGELOG.md` is documentation, exempt from XV.

**Canonical Path Audit (Principle XIII)**:
- Canonical owner/path for touched behaviour:
  - `.github/workflows/ci.yml` is the canonical CI workflow.
  - `scripts/release/extract-changelog-section.sh` is the new
    canonical extraction implementation.
  - `tests/release/extract_changelog_section_test.go` is the
    canonical regression test that exec's the script.
  - `CHANGELOG.md` is the canonical changelog content.
- Replaced or retired paths: The current `publish-release` step
  invocation of `softprops/action-gh-release@v2.0.8` without
  `body_path` is amended in place; no parallel step is added.
  GitHub's auto-generated commit-list body is exactly what this
  feature replaces.
- External degradation paths, if any: A tag whose changelog section
  is heading-only (no bullets) yields an empty release-notes body.
  This is observable on the release page and is a content-author
  problem, not a pipeline failure. The `changelog-gate` deliberately
  checks only "heading exists", not "section is non-empty", because
  the latter has too many failure modes (e.g., a section that is
  intentionally just a date pointer to a follow-up). This boundary
  is documented in `spec.md` and verified by the regression test
  shape.
- Review check: Reviewer verifies (a) the extraction script is the
  only parser of `CHANGELOG.md` for release notes (no inline `awk`
  in the workflow YAML, no second parser in the Go test); (b) the
  gate is a separate job rather than a step inside `publish-release`
  so the CI graph names the failure surface; (c) both
  `changelog-gate` and `publish-release` perform `actions/checkout`
  (the previous `publish-release` did not); (d) no
  `changelog/unreleased/` fragment directory or fragment-merge step
  is introduced (user explicitly rejected that approach in the
  feature task).

Constitution Check: PASS. No violations to track.

## Project Structure

### Documentation (this feature)

```text
specs/022-changelog-release-pipeline/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── extract.md       # Extraction script contract
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

A `data-model.md` is intentionally omitted: this feature has no
domain entities beyond the trivial "tag -> changelog section"
mapping already covered by `spec.md` Key Entities.

### Source Code (repository root)

```text
.github/
└── workflows/
    └── ci.yml                    # MODIFIED: add changelog-gate job;
                                  # add checkout + extraction step to
                                  # publish-release; add body_path

scripts/
└── release/
    └── extract-changelog-section.sh  # NEW: canonical extractor

tests/
└── release/
    └── extract_changelog_section_test.go  # NEW: regression test

CHANGELOG.md                       # MODIFIED: prepend v0.4.0 section
CLAUDE.md                          # MODIFIED: point at 022
AGENTS.md                          # MODIFIED: point at 022
.specify/feature.json              # MODIFIED: point at 022
```

**Structure Decision**: This is a tooling/pipeline change, not an
application change. There is no `src/` reorganisation. The new
canonical paths are listed above and match the existing repository
layout (`scripts/` for shell tooling, `tests/<package>/` for Go
tests, `.github/workflows/` for CI).

## Complexity Tracking

No constitutional violations require justification. The complexity
of "a separate job for the gate vs. a step inside publish-release"
is documented in spec.md (FR-007, Canonical Path Audit) as a
deliberate readability choice on the CI graph, not a complexity
escalation.
