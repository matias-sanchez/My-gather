# Quickstart: Changelog-driven release pipeline

## Cut a release after this feature ships

1. Decide the new version (e.g., `v0.4.1`).
2. Edit `CHANGELOG.md`. Add a new section above the previous one:

   ```markdown
   ## v0.4.1 (YYYY-MM-DD)

   ### Fixed

   - One-line description of the fix, citing PR number.
   ```

   Use v0.2.0 / v0.4.0 as exemplars for the subsection structure
   (`### Added`, `### Changed`, `### Fixed`, `### Build / tooling`).
   Omit subsections that have no bullets.

3. Commit the changelog edit on `main`.
4. Tag the commit: `git tag v0.4.1 && git push origin v0.4.1`.
5. Watch GitHub Actions:
   - `changelog-gate` runs first on the tag and confirms
     `## v0.4.1 (` exists in `CHANGELOG.md`.
   - `release` builds and uploads the artifacts.
   - `publish-release` checks out the repo, extracts the
     v0.4.1 section into `dist/RELEASE_NOTES.md`, and attaches it
     as the GitHub Release body.

## What happens if the changelog section is missing

The `changelog-gate` job fails with:

```
CHANGELOG.md is missing a `## v0.4.1 (YYYY-MM-DD)` section. Add it before tagging.
```

`publish-release` is skipped (not just failed - it has
`needs: [release, changelog-gate]`, so the gate's failure prevents
it from starting). The maintainer:

1. Adds the missing section to `CHANGELOG.md`.
2. Commits and pushes the fix to `main`.
3. Deletes the failed tag locally and on GitHub:
   `git push origin :v0.4.1 && git tag -d v0.4.1`.
4. Re-tags the new commit and re-pushes.

## Run the regression test locally

```sh
go test ./tests/release/...
```

The test exec's `scripts/release/extract-changelog-section.sh`
against the in-repo `CHANGELOG.md` for both `v0.3.1` and `v0.4.0`
and asserts:

- The body does not contain any `## v` heading.
- The body's first non-blank line is the section's first non-blank
  line below the heading.
- For `v0.4.0`, the body mentions each of `016`, `017`, `018`,
  `019`, `020`, `021` as a substring.

## Run the extraction script by hand

From the repo root:

```sh
scripts/release/extract-changelog-section.sh v0.4.0
```

Output is the lines of the v0.4.0 section, excluding the
`## v0.4.0 (...)` heading itself, terminating before the next
`## v` heading.

Set `LC_ALL=C` if you want byte-stable output across locales:

```sh
LC_ALL=C scripts/release/extract-changelog-section.sh v0.4.0 > /tmp/notes.md
```
