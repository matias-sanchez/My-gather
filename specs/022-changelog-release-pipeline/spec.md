# Feature Specification: Changelog-driven release pipeline + v0.4.0 backfill

**Feature Branch**: `022-changelog-release-pipeline`
**Created**: 2026-05-07
**Status**: Draft
**Input**: User description: "Wire CHANGELOG.md into the GitHub release pipeline (release notes sourced from the curated section, plus a release-time gate that fails when the tag has no matching section). Backfill the v0.4.0 (2026-05-07) section above v0.3.1 covering the six features (016 through 021) merged into main since v0.3.1."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Curated release notes appear on the GitHub Releases page (Priority: P1)

A maintainer cuts a tagged release by pushing `vX.Y.Z`. The CI release
pipeline produces signed binaries and attaches them to a GitHub Release.
The body of the GitHub Release page MUST be the curated
`## vX.Y.Z (YYYY-MM-DD)` section copied verbatim from `CHANGELOG.md`,
not GitHub's auto-generated commit list.

**Why this priority**: This is the visible deliverable. Today the
release page on `v0.3.1` has an empty body and a generic commit list,
which is exactly the artifact a downstream user reads first when
deciding whether to upgrade. Without this, the curated changelog might
as well not exist.

**Independent Test**: Push a `vX.Y.Z` tag whose matching `## vX.Y.Z (`
heading exists in `CHANGELOG.md`. Open the resulting GitHub Release
page. The body matches the lines from that heading down to (but not
including) the next `## v` heading.

**Acceptance Scenarios**:

1. **Given** `CHANGELOG.md` contains a `## v0.4.0 (2026-05-07)`
   section followed by `## v0.3.1 (2026-04-23)`, **When** the
   maintainer pushes the tag `v0.4.0`, **Then** the GitHub Release
   page body for `v0.4.0` contains exactly the lines of the v0.4.0
   section through (but not including) the `## v0.3.1` heading.
2. **Given** the previous scenario, **When** the body is rendered on
   the GitHub Release page, **Then** the `## v0.4.0 (2026-05-07)`
   heading line itself is omitted (the GitHub UI already shows the
   tag as the page title; including the heading would duplicate it).
3. **Given** the same tag is republished (workflow re-runs),
   **When** the release notes are re-extracted, **Then** the body is
   byte-identical to the first run for the same `CHANGELOG.md`
   content.

---

### User Story 2 - Tag without a curated changelog entry is blocked (Priority: P1)

A maintainer pushes a `vX.Y.Z` tag without first adding the matching
`## vX.Y.Z (YYYY-MM-DD)` section to `CHANGELOG.md`. CI MUST fail the
release workflow with a clear error message before any binary is
attached to a GitHub Release. The maintainer adds the section,
re-tags, and the release ships normally.

**Why this priority**: Without the gate, the curated-changelog
guarantee in User Story 1 is purely advisory. The whole point of
sourcing release notes from the changelog is that there is always a
curated section; if a missing section silently degrades to an empty
body, the original problem returns.

**Independent Test**: Locally, push (or simulate) a tag like
`v9.9.9-test` for which `CHANGELOG.md` has no matching heading. The
release workflow fails the gate job with the documented error
message; no GitHub Release is created.

**Acceptance Scenarios**:

1. **Given** `CHANGELOG.md` has no `## v0.4.0 (` heading, **When**
   the maintainer pushes the tag `v0.4.0`, **Then** the
   `changelog-gate` CI job fails with the message
   "CHANGELOG.md is missing a `## v0.4.0 (YYYY-MM-DD)` section. Add
   it before tagging." and the `publish-release` job does not run.
2. **Given** the previous failed run, **When** the maintainer adds
   the v0.4.0 section to `CHANGELOG.md`, deletes and re-pushes the
   tag, **Then** the gate passes and the release publishes with the
   curated body from User Story 1.
3. **Given** a non-tag push (branch push or pull request), **When**
   CI runs, **Then** the `changelog-gate` and `publish-release` jobs
   are both skipped (they run only on `refs/tags/v*`).

---

### User Story 3 - v0.4.0 changelog section reflects the six shipped features (Priority: P1)

A user reading `CHANGELOG.md` after the v0.4.0 release sees a
v0.4.0 (2026-05-07) section above v0.3.1 that lists every
user-visible change merged since v0.3.1: streaming over the 1 GiB
cap, synchronized chart zoom + windowed legend stats, top-CPU
caption + mysqld pin, the InnoDB redo-log sizing panel, runtime
theming, and the required Author field on the feedback dialog.

**Why this priority**: The other two stories build the pipeline; this
story populates the data the pipeline depends on for the next
release. Without the backfill, User Story 1 ships an empty release
body for `v0.4.0` and User Story 2's gate trips on the next tag.

**Independent Test**: Open `CHANGELOG.md`. The first dated section
is `## v0.4.0 (2026-05-07)`. It uses `### Added` / `### Changed` /
`### Fixed` / `### Build / tooling` subsections (matching v0.2.0's
structure). Each of features 016-021 is mentioned at least once,
attributed by feature number and the merged PR number.

**Acceptance Scenarios**:

1. **Given** `CHANGELOG.md` after this feature ships, **When** a
   reader scans the file top-down, **Then** the first dated entry is
   `## v0.4.0 (2026-05-07)`, placed above `## v0.3.1 (2026-04-23)`.
2. **Given** the v0.4.0 section, **When** the reader scans the
   subsections, **Then** every feature 016, 017, 018, 019, 020, 021
   is represented with a one-line bullet citing its PR number.
3. **Given** the v0.4.0 section, **When** the extraction script
   (User Story 1) is run with tag `v0.4.0`, **Then** the produced
   release-notes body terminates immediately before the
   `## v0.3.1 (2026-04-23)` heading and contains every bullet from
   each subsection.

---

### Edge Cases

- Tag named with extra suffix (e.g., `v0.4.0-rc1`): the gate looks
  for a heading containing the literal tag including suffix
  (`## v0.4.0-rc1 (`). If absent, the gate fails with the same
  message. Pre-release tagging therefore requires a matching
  pre-release section in `CHANGELOG.md`.
- Tag whose section is the LAST section in the file (no following
  `## v` heading): extraction terminates at end-of-file rather than
  at a missing successor heading, so the bullet at the bottom of the
  changelog is included.
- `CHANGELOG.md` contains a heading line whose tag matches but uses
  a non-date suffix (e.g., `## v0.4.0 (TBD)` rather than
  `## v0.4.0 (YYYY-MM-DD)`): the gate accepts any text after `(`
  because the gate's contract is "the section heading exists for
  this tag", not "the date is well-formed". Date-formatting hygiene
  is a review concern, not a release-blocking gate concern.
- Maintainer pushes a tag, the gate fails, the maintainer fixes
  `CHANGELOG.md` and re-pushes the tag without deleting the
  GitHub-side tag first: GitHub rejects the push (tag already
  exists). Documented in the gate's failure message? No - this is
  standard git tag behaviour; the gate's job is to prevent the bad
  release, not teach git. The gate message tells the maintainer to
  fix the changelog before tagging; deleting and re-pushing the tag
  is the maintainer's normal workflow.
- Two release attempts for the same tag interleave (rare; would
  require a force-push of the tag): each run independently extracts
  from the current `CHANGELOG.md` on the tagged commit, so the
  release body always matches the changelog at tag-creation time.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The release publishing step MUST attach a release-notes
  body to the GitHub Release that is the verbatim section of
  `CHANGELOG.md` corresponding to the pushed tag, sourced from the
  same `CHANGELOG.md` revision that is checked out at the tagged
  commit.
- **FR-002**: The extraction step MUST omit the `## vX.Y.Z (date)`
  heading line from the produced release-notes body, on the rationale
  that the GitHub Release page already displays the tag as the page
  title and including the heading would duplicate that title in the
  body.
- **FR-003**: The extraction MUST terminate immediately before the
  next `## v` heading (exclusive), or at end-of-file if the section
  is the last in the changelog.
- **FR-004**: The extraction MUST be implemented as a single canonical
  shell script under `scripts/release/`, invoked unmodified by the
  CI workflow and by the regression test. Inline `awk`/`sed` in
  workflow YAML is prohibited so the extraction can be exercised by
  tests.
- **FR-005**: A new CI job `changelog-gate` MUST run before
  `publish-release`, MUST trigger only on tag pushes
  (`startsWith(github.ref, 'refs/tags/v')`), MUST check out the
  repository so `CHANGELOG.md` is on disk, and MUST grep
  `CHANGELOG.md` for the literal heading `## ${TAG} (` (where TAG
  includes the `v` prefix).
- **FR-006**: When the heading is missing, `changelog-gate` MUST
  fail the workflow with the exact message
  "CHANGELOG.md is missing a `## ${TAG} (YYYY-MM-DD)` section. Add
  it before tagging." (with `${TAG}` interpolated to the actual
  tag).
- **FR-007**: The `publish-release` job MUST declare
  `needs: [release, changelog-gate]` so a tag without a curated
  section cannot ship binaries to a GitHub Release.
- **FR-008**: The `publish-release` job MUST check out the
  repository so `CHANGELOG.md` is on disk for the extraction step
  (the existing job does not check out today; that omission is
  load-bearing for this feature).
- **FR-009**: `CHANGELOG.md` MUST gain a new section
  `## v0.4.0 (2026-05-07)` placed immediately above the existing
  `## v0.3.1 (2026-04-23)` heading.
- **FR-010**: The v0.4.0 section MUST use the
  Keep-a-Changelog subsection structure already established by
  v0.2.0: `### Added`, `### Changed`, `### Fixed`,
  `### Build / tooling` (omit a subsection only if it has no
  bullets).
- **FR-011**: The v0.4.0 section MUST cover every feature merged
  into main since v0.3.1 (016, 017, 018, 019, 020, 021), with at
  least one bullet per feature, attributed by PR number where the
  bullet describes a single PR's outcome.
- **FR-012**: A regression test under `tests/release/` MUST exec
  the extraction script against the in-repo `CHANGELOG.md` for
  both `v0.3.1` and `v0.4.0` and MUST assert (a) the produced body
  starts with the first non-heading line of the section, (b) the
  body does NOT contain any `## v` heading, and (c) the body's
  last non-blank line is the last bullet of the section.
- **FR-013**: The regression test for `v0.4.0` MUST additionally
  assert that the body mentions each of `016`, `017`, `018`, `019`,
  `020`, `021` as a substring, so a future cleanup of the v0.4.0
  bullets cannot silently drop a feature attribution.

### Canonical Path Expectations

- **Canonical owner/path**: The release pipeline lives in
  `.github/workflows/ci.yml` (the `release` and `publish-release`
  jobs, plus the new `changelog-gate` job). The extraction logic
  lives in a single shell script
  `scripts/release/extract-changelog-section.sh` consumed by both
  the CI step in `publish-release` and the regression test under
  `tests/release/`. The curated content lives in `CHANGELOG.md` at
  the repository root.
- **Old path treatment**: The current `publish-release` step that
  invokes `softprops/action-gh-release@v2.0.8` without `body_path`
  is replaced in the same change: the action gains `body_path:
  dist/RELEASE_NOTES.md` and a preceding step that materialises
  that file. No parallel step is added; the existing one is
  amended. The current absence of a release-notes file is not a
  "fallback path" worth preserving - GitHub's auto-generated commit
  list is exactly what this feature exists to replace.
- **External degradation**: If the extraction script produces an
  empty body for a tag that has a heading but no bullets (e.g., a
  placeholder release entry), the release page body is empty. This
  is observable on the release page and is a content-authoring
  problem, not a pipeline failure. The gate's job is "section
  exists"; the body's contents are the maintainer's responsibility.
- **Review check**: Reviewer verifies (a) the extraction script is
  the only place that parses `CHANGELOG.md` for release notes -
  no inline `awk` in the workflow YAML, no second copy under
  `tests/`; (b) the gate is a separate job rather than a step
  inside `publish-release`, so the failure surface is explicit in
  the CI graph; (c) `publish-release` and `changelog-gate` both
  perform `actions/checkout` because the previous `publish-release`
  job did not (it only downloaded the artifact); (d) no
  `changelog/unreleased/` fragment directory or fragment-merge
  step is introduced - the user explicitly rejected that approach.

### Key Entities

- **CHANGELOG section**: A contiguous block in `CHANGELOG.md` whose
  first line is a heading of the form `## vX.Y.Z (YYYY-MM-DD)` and
  whose subsequent lines run until the next `## v` heading or
  end-of-file.
- **Release notes file**: `dist/RELEASE_NOTES.md`, materialised
  during the `publish-release` job by the extraction script,
  consumed by `softprops/action-gh-release@v2.0.8` via `body_path`.
- **Tag**: A git tag of the form `v*` (e.g., `v0.4.0`). The tag
  name (including the `v`) is the lookup key for the changelog
  section heading.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of tagged releases produced after this feature
  ships display the curated `CHANGELOG.md` section as their GitHub
  Release page body, with no GitHub auto-generated commit-list
  fallback.
- **SC-002**: 100% of attempts to push a `vX.Y.Z` tag whose
  matching `## vX.Y.Z (` heading is missing from `CHANGELOG.md`
  fail the CI release workflow before any binary is attached to a
  GitHub Release.
- **SC-003**: After this feature ships, `CHANGELOG.md` contains a
  v0.4.0 (2026-05-07) section that documents all six features
  (016 through 021) merged since v0.3.1, verifiable by reading the
  file.
- **SC-004**: The regression test under `tests/release/` runs as
  part of `go test ./...` on every CI build and fails if either
  the extraction script's behaviour drifts or the in-repo
  `CHANGELOG.md` loses a documented feature attribution.

## Assumptions

- Tag prefix is always `v` (e.g., `v0.4.0`), matching the existing
  workflow trigger `tags: ["v*"]` in `.github/workflows/ci.yml`.
  Pre-release suffixes (e.g., `v0.4.0-rc1`) are looked up by their
  full literal tag including suffix.
- The release-notes body intentionally omits the `## vX.Y.Z (date)`
  heading line. The GitHub Release page renders the tag as its title;
  including the heading in the body would duplicate it. This is the
  silent default chosen for this feature; the inverse choice
  (include the heading) was rejected as cosmetically inferior.
- The `softprops/action-gh-release@v2.0.8` pin is preserved; this
  feature only adds `body_path:` to the existing `with:` block, it
  does not bump the action.
- The gate uses `grep -F` (fixed string) on the literal
  `## ${TAG} (` to avoid regex escaping issues with `.` in version
  numbers.
- The extraction script is POSIX `sh` + `awk` so it runs unchanged
  on the `ubuntu-latest` runner used by the workflow and on a
  developer's macOS shell for local testing.
- The regression test is a Go test under `tests/release/` (matching
  the repo's `tests/coverage/...` convention) that exec's the
  extraction script via `os/exec` and asserts on its stdout. This
  keeps validation in the same `go test ./...` umbrella that CI
  already runs.
- No changelog-fragment automation under `changelog/unreleased/`
  is introduced. The user explicitly rejected that approach;
  the canonical content lives in `CHANGELOG.md` directly.
- File-size budget under Principle XV: `CHANGELOG.md` is a doc file
  (not first-party source code) so the 1000-line cap does not
  apply. The new shell script and Go test are well under cap.
- The v0.4.0 section text is English-only per Principle XIV. ASCII
  punctuation only (no smart quotes, em-dashes only via the
  permitted character set).
