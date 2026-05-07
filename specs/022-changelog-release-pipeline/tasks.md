---
description: "Tasks: Changelog-driven release pipeline + v0.4.0 backfill"
---

# Tasks: Changelog-driven release pipeline + v0.4.0 backfill

**Input**: Design documents from `specs/022-changelog-release-pipeline/`
**Prerequisites**: plan.md, spec.md, research.md, contracts/extract.md, quickstart.md

**Tests**: Yes — FR-012 / FR-013 / SC-004 require a Go regression test
under `tests/release/` that exec's the canonical extraction script.

**Organization**: Tasks grouped by user story; T001 is shared
foundation since US1 and US2 both depend on the canonical extraction
script existing.

## Canonical Path Metadata *(Principle XIII)*

- **Canonical owner/path**:
  - `.github/workflows/ci.yml` (CI workflow)
  - `scripts/release/extract-changelog-section.sh` (extractor)
  - `tests/release/extract_changelog_section_test.go` (regression test)
  - `CHANGELOG.md` (curated content)
- **Old path treatment**: The `publish-release` step that invoked
  `softprops/action-gh-release@v2.0.8` without `body_path` is amended
  in T003. No parallel implementation remains.
- **External degradation evidence**: Section-with-empty-body case
  is documented in spec.md and verified by the regression test's
  shape (body must not contain `## v`, but is allowed to be content
  only - blank-body is observable on the release page).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Independent file, no ordering dependency
- **[Story]**: US1 (release notes from changelog), US2 (gate),
  US3 (v0.4.0 backfill)

---

## Phase 1: Foundation (canonical extraction script)

- [ ] **T001** [Foundation] Create `scripts/release/extract-changelog-section.sh`
  per `contracts/extract.md`. POSIX `sh` + `awk`. Reads
  `CHANGELOG.md` (default) or an explicit path argument. Prints
  the lines of the section matching `## <tag> (` excluding the
  heading, terminating before the next `## v` heading or
  end-of-file. Exits non-zero with a clear stderr message when
  the tag has no matching heading. `chmod +x`.

**Checkpoint**: T001 is the dependency for US1 (CI step) and for
the regression test under US2/US3.

---

## Phase 2: User Story 1 — Curated release notes appear on Releases page (P1)

**Goal**: The GitHub Release page body for any tag is the curated
section from `CHANGELOG.md`.

**Independent test**: Push (or re-run on) a tag with a matching
section; the GitHub Release page body matches the section.

- [ ] **T002** [US1] In `.github/workflows/ci.yml`, add
  `actions/checkout@v4` (with `fetch-depth: 0`) as the first step
  of `publish-release`. Without this, `CHANGELOG.md` is not on
  disk in the runner's workspace.
- [ ] **T003** [US1] In `.github/workflows/ci.yml`, BETWEEN the
  `download-artifact` step and the `Attach to GitHub Release`
  step, add a "Build release notes from CHANGELOG.md" step that:
  - sets `TAG="${GITHUB_REF#refs/tags/}"`
  - runs `LC_ALL=C scripts/release/extract-changelog-section.sh
    "$TAG" CHANGELOG.md > dist/RELEASE_NOTES.md`
  - prints the file size of `dist/RELEASE_NOTES.md` for log clarity.
- [ ] **T004** [US1] In `.github/workflows/ci.yml`, modify the
  `Attach to GitHub Release` step's `with:` block to add
  `body_path: dist/RELEASE_NOTES.md`. Keep
  `softprops/action-gh-release@v2.0.8` pin unchanged.

**Checkpoint**: US1 mechanically complete. The release page body
will be curated for any subsequent tag whose section exists.

---

## Phase 3: User Story 2 — Tag without curated section is blocked (P1)

**Goal**: A tag with no matching `## vX.Y.Z (` heading fails CI
before any binary is published.

**Independent test**: Push a tag for which `CHANGELOG.md` has no
heading; `changelog-gate` fails with the documented message.

- [ ] **T005** [US2] In `.github/workflows/ci.yml`, add a new job
  `changelog-gate` after the existing jobs:
  - `runs-on: ubuntu-latest`
  - `if: startsWith(github.ref, 'refs/tags/v')`
  - steps:
    1. `actions/checkout@v4`
    2. shell step that sets `TAG="${GITHUB_REF#refs/tags/}"`,
       runs `grep -F "## ${TAG} (" CHANGELOG.md` and on failure
       echoes "CHANGELOG.md is missing a `## ${TAG} (YYYY-MM-DD)`
       section. Add it before tagging." to stderr and `exit 1`.
- [ ] **T006** [US2] In `.github/workflows/ci.yml`, change
  `publish-release.needs` from `[release]` to
  `[release, changelog-gate]` so a missing changelog section
  prevents `publish-release` from starting.
- [ ] **T007** [US2] Update the comment block at the top of
  `.github/workflows/ci.yml` (the ASCII job-graph diagram) so
  it shows `changelog-gate` as a precondition for
  `publish-release`. The diagram is documentation and MUST stay
  in sync with the actual job graph.

**Checkpoint**: US2 mechanically complete. The pipeline gates
correctly on the heading's presence.

---

## Phase 4: User Story 3 — v0.4.0 backfill (P1)

**Goal**: `CHANGELOG.md` carries a v0.4.0 (2026-05-07) section
above v0.3.1 covering features 016-021.

**Independent test**: Read `CHANGELOG.md`. The first dated entry
is `## v0.4.0 (2026-05-07)`, structured with subsections, and
mentions PRs #50, #51, #52, #53, #54, #55, #56.

- [ ] **T008** [US3] Edit `CHANGELOG.md`. Insert a new section
  `## v0.4.0 (2026-05-07)` between line 3
  (`All notable changes...`) and the existing
  `## v0.3.1 (2026-04-23)` heading. Use v0.2.0's Keep-a-Changelog
  structure (`### Added` / `### Changed` / `### Fixed` /
  `### Build / tooling`, omit subsections that have no bullets).
  Cover features:
  - **016 (#50)** under Changed: 1 GiB cap removed; streaming
    invariant enforced by peak-heap-sampler regression test (1.19
    GB at ~3 MiB heap). Spec-required 64 GiB archive-extraction
    ceiling kept.
  - **017 (#51, #52)** under Added: synchronized chart zoom and
    per-series Min/Avg/Max in the legend recomputed over the
    visible window via a single `__chartSyncStore` singleton;
    HLL sparkline canonically exempt; stacked processlist
    dimensions get windowed stats from raw values.
  - **018 (#54)** under Added: Top CPU processes caption +
    accurate aria-label + regression test on the mysqld-pin
    invariant.
  - **019 (#55)** under Added: InnoDB redo-log sizing panel with
    full MySQL version coverage (5.6/5.7
    `innodb_log_file_size` x `innodb_log_files_in_group` and
    8.0.30+ `innodb_redo_log_capacity`); observed write rate
    with negative-delta clamp; 15-minute rolling peak with
    full-window guard; coverage minutes; 15-minute and 1-hour
    recommendations; warning when coverage is under 15 minutes;
    Percona KB0010732 cited.
  - **020 (#53)** under Added: three themes (light, dark default,
    Okabe-Ito color-blind-safe) selectable at runtime, persisted
    in `localStorage["my-gather:theme"]`. Single CSS-variable
    token block in `04.css` (16 distinct `--series-N` per theme).
    `color-scheme` per theme. Pre-paint `<head>` script avoids
    FOUC. Live theme switches re-paint open charts via
    `mygather:theme` event; HLL sparkline has its own local
    listener; legend swatches use `var(--series-N)` so they
    re-resolve automatically.
  - **020 (#53)** under Changed: deleted parallel paths -
    `prefers-color-scheme` block, `SERIES_COLORS` JS array,
    hardcoded `#6b8eff` swatch.
  - **021 (#56)** under Added: required Author field on the
    Report Feedback dialog; Submit blocked when empty;
    last-used Author persisted in localStorage on definitive
    worker-success only (snapshot at submit time). "Submitted
    by: <name>" prepended to issue body in both worker and
    legacy `window.open` paths. Single canonical
    `authorMaxChars: 80` in `feedback-contract.json` consumed
    by the Go view, JS form gate, and TS worker.
  - Under Build / tooling: the changelog itself is now sourced
    into the GitHub Release body via
    `scripts/release/extract-changelog-section.sh`; tag pushes
    are gated by `changelog-gate` so a missing section blocks
    the release.
- [ ] **T009** [US3] Verify `CHANGELOG.md` is well-formed after
  the edit: every `## v` heading is followed by a date in
  parentheses; no markdown lint warnings; UTF-8 ASCII-only
  punctuation per Principle XIV.

**Checkpoint**: US3 done. Backfill complete.

---

## Phase 5: Regression test

- [ ] **T010** [US1+US2+US3] Create
  `tests/release/extract_changelog_section_test.go`. The test
  exec's `scripts/release/extract-changelog-section.sh` via
  `os/exec` from the repo root and asserts:
  - `extract v0.3.1` succeeds, stdout contains "Patch release"
    (existing v0.3.1 prose), stdout contains no `## v` substring.
  - `extract v0.4.0` succeeds, stdout contains all of `016`,
    `017`, `018`, `019`, `020`, `021` as substrings, stdout
    contains no `## v` substring, stdout's first non-blank line
    is `### Added` or another `### ` subsection heading.
  - `extract v9.9.9-nope` exits non-zero (no matching heading).
  - The test discovers the repo root via `runtime.Caller` so it
    runs from any working directory.

**Checkpoint**: regression test landed. Future drift caught.

---

## Phase 6: Context-file alignment

- [ ] **T011** [P] Update `.specify/feature.json` to point at
  `specs/022-changelog-release-pipeline`.
- [ ] **T012** [P] Update `CLAUDE.md` so "Active feature" is
  `022-changelog-release-pipeline`, plan/spec/contract/research/
  quickstart/tasks pointers reflect the new directory, and the
  prior-features list gains `020-report-theming` and
  `021-feedback-author-field`.
- [ ] **T013** [P] Update `AGENTS.md` to mirror `CLAUDE.md`'s
  feature pointer change.

**Checkpoint**: side-by-side agent contract honoured. CLAUDE.md,
AGENTS.md, and `.specify/feature.json` agree.

---

## Phase 7: Validation

- [ ] **T014** Run `gofmt -w .`
- [ ] **T015** Run `go vet ./...`
- [ ] **T016** Run `go test ./...` (includes
  `tests/release/...`, `tests/coverage/...`)
- [ ] **T017** Run the pre-push constitution guard locally
  (`scripts/hooks/pre-push-constitution-guard.sh`).
- [ ] **T018** Validate workflow YAML by parsing it (`python3 -c
  "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"`
  or `actionlint` if available).
- [ ] **T019** Smoke-test the extraction script locally:
  `scripts/release/extract-changelog-section.sh v0.3.1 |
  head -5` and `... v0.4.0 | head -5`.

---

## Phase 8: Ship

- [ ] **T020** Commit (single commit; conventional message
  citing PR-022, the three problems, and the three fixes).
- [ ] **T021** Push the branch.
- [ ] **T022** Open the PR via `gh pr create --title
  "Changelog automation + v0.4.0 backfill (PR 022)"` with a
  summary referencing the three problems and the three fixes
  plus a one-line note that this is a prerequisite for cutting
  v0.4.0.

## Dependencies

- T001 is required by T003, T010, T019.
- T002, T003, T004 are sequential edits to the same file
  (`ci.yml`) so they MUST run in that order on the same diff.
- T005, T006, T007 are sequential edits to `ci.yml` for the gate.
  T005 must precede T006 (the `needs:` reference must resolve).
- T008 (CHANGELOG backfill) is required by T010 (regression test
  asserts on v0.4.0 substrings).
- T011/T012/T013 are independent of each other and of T002-T008.
- T014-T019 follow all earlier tasks.
- T020-T022 follow all validation tasks.

## Parallelism notes

- T011, T012, T013 marked `[P]` (different files, no order).
- The script-creation T001, the YAML edits T002-T007, and the
  Go test T010 touch different files; T010 only depends on T001
  + T008 having landed. The CHANGELOG edit T008 is independent
  of the YAML and script work, but T010 needs both.
