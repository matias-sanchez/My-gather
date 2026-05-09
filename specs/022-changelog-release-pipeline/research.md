# Research: Changelog-driven release pipeline

## Decision: extraction tool

**Decision**: POSIX `sh` + `awk`, packaged as a single shell script
under `scripts/release/extract-changelog-section.sh`.

**Rationale**: The job is "given a markdown file and a tag string,
print the lines of the matching `## vX.Y.Z (...)` section excluding
the heading and stopping before the next `## v` heading". `awk`'s
range-and-pattern model expresses this in one expression. Using Go
would require either a new package under `tools/` (Principle X
overhead) or shipping the parser in the release binary (Principle I:
the binary's job is parsing pt-stalk, not its own changelog). A
shell script is the right tool for a 30-line text transform invoked
once per tag push.

**Alternatives considered**:
- Inline `awk` in workflow YAML: rejected by Principle XIII and by
  testability. The user task explicitly requires the regression test
  to exercise the same extraction logic that CI uses.
- A new Go binary `cmd/extract-changelog`: rejected as overhead;
  Principle X disfavours new dependencies and a 10-line shell
  script does not justify a Go module + binary + test scaffolding.
- `sed -n` + a hand-rolled state machine: works, but `awk`'s
  built-in line-range matching is clearer.

## Decision: heading inclusion in release-notes body

**Decision**: Omit the `## vX.Y.Z (date)` heading line from the
extracted body. Print only the lines AFTER the heading, up to (but
not including) the next `## v` line.

**Rationale**: The GitHub Release page renders the tag as the page
title. Including the heading in the body duplicates that title and
looks visually noisy. Other repositories (e.g., the Linux kernel
changelogs are not relevant here, but cargo, rustc, and ripgrep all
omit the heading from their GitHub release bodies) follow the same
convention.

**Alternatives considered**:
- Include the heading: rejected on cosmetic grounds, documented in
  spec.md Assumptions.

## Decision: gate as separate job vs. step in publish-release

**Decision**: Separate job `changelog-gate` with
`if: startsWith(github.ref, 'refs/tags/v')` and
`needs: []` (parallel to `release`).

**Rationale**: A separate job names the failure surface in the CI
graph - a maintainer looking at a failed run sees `changelog-gate`
red and goes directly to "the changelog is wrong", without scrolling
through the `publish-release` job's other steps. The job is cheap
(one `checkout` + one `grep`); the runner-minute cost is negligible
compared to the clarity benefit. `publish-release` declares
`needs: [release, changelog-gate]` so the binary attach step cannot
run when the gate fails.

**Alternatives considered**:
- Step inside `publish-release`: works mechanically but mixes the
  failure surface with the binary upload's failure surface.
  Rejected on UX grounds.

## Decision: gate's grep pattern

**Decision**: `grep -F "## ${TAG} ("` against `CHANGELOG.md`.

**Rationale**: `grep -F` (fixed string) avoids regex escaping for
the `.` in version numbers. The trailing `(` is the canonical
heading shape (`## v0.4.0 (2026-05-07)`), so it disambiguates the
heading from prose that incidentally mentions the version (e.g.,
"upgraded from v0.3.1").

**Alternatives considered**:
- `grep -E "^## ${TAG} \\("`: equivalent under `set -o pipefail`
  but requires regex escaping for the version's `.`. The fixed-string
  version is one fewer thing to get wrong.

## Decision: regression test placement

**Decision**: `tests/release/extract_changelog_section_test.go`,
exec'ing `scripts/release/extract-changelog-section.sh` via
`os/exec` and asserting on stdout for `v0.3.1` and `v0.4.0` against
the actual in-repo `CHANGELOG.md`.

**Rationale**: Matches the existing repo convention of test
packages under `tests/<area>/` (see `tests/coverage/`). Running
under `go test ./...` means the constitution-guard pre-push hook
already wraps it. Asserting against the live `CHANGELOG.md` rather
than a fixture means a future bullet-cleanup that drops a feature
attribution gets caught immediately.

**Alternatives considered**:
- A pure shell test under `scripts/release/`: works but doesn't
  show up under `go test ./...`, which is the canonical "run all
  tests" entry point in this repo.
- A fixture changelog under `tests/release/testdata/`: rejected
  because then the test wouldn't catch drift in the real
  `CHANGELOG.md`; FR-013 (the substring assertions for feature
  IDs 016-021) is the whole point.

## Decision: actions/checkout in changelog-gate and publish-release

**Decision**: Add `actions/checkout@v4` to both new/modified jobs.

**Rationale**: The current `publish-release` only runs
`actions/download-artifact@v4` and the release action; it does not
check out the repository, so `CHANGELOG.md` is not on disk in that
runner's workspace. Without checkout, both the extraction script
and the gate's grep have nothing to read. This is a non-obvious
correctness requirement called out by the advisor; the plan flags
it as a load-bearing detail.

**Alternatives considered**:
- Pull `CHANGELOG.md` from the upload-artifact bundle: rejected
  because `make release` does not put `CHANGELOG.md` into `dist/`,
  and modifying `make release` is explicitly out of scope per the
  task ("Do NOT change the existing release-artifact build steps").

## Decision: backfill style for v0.4.0

**Decision**: Use v0.2.0's `### Added` / `### Changed` / `### Fixed`
/ `### Build / tooling` Keep-a-Changelog structure. Each feature
gets bullets distributed across subsections by nature of change
(e.g., 020 has a heavy ### Added of new themes plus a ### Changed
note that the prefers-color-scheme block was deleted).

**Rationale**: v0.2.0 is the only prior section that uses the full
subsection structure; v0.3.0 and v0.3.1 are short paragraph entries
because their releases were narrow patches. v0.4.0 covers six
features with a mix of additive and replacement work, so it
warrants the full structure. The user task explicitly names
v0.2.0 as the structural exemplar.

**Alternatives considered**:
- Paragraph-style like v0.3.0/v0.3.1: rejected; six features need
  bullets to be scannable.
