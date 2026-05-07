# Contract: `scripts/release/extract-changelog-section.sh`

## Synopsis

```sh
scripts/release/extract-changelog-section.sh <tag> [<changelog-path>]
```

- `<tag>` (required): the git tag whose changelog section is
  extracted, including the `v` prefix. Examples: `v0.4.0`,
  `v0.3.1`, `v0.4.0-rc1`.
- `<changelog-path>` (optional): path to the changelog file.
  Defaults to `CHANGELOG.md` relative to the current working
  directory.

## Behaviour

The script reads `<changelog-path>` and writes to standard output
the lines of the section whose heading begins with `## <tag> (`,
beginning with the first line AFTER the heading, and ending
immediately BEFORE the next line that begins with `## v`. If the
matching section is the last in the file, extraction terminates at
end-of-file.

The script:
- MUST omit the `## <tag> (...)` heading line itself.
- MUST include all blank lines and bullet lines between the matched
  heading and the next `## v` heading verbatim.
- MUST exit non-zero with a clear stderr message when no matching
  heading is found.
- MUST exit non-zero with a clear stderr message when the
  required `<tag>` argument is absent or empty.
- MUST be invoked with `LC_ALL=C` so byte ordering is locale-stable.

## Output examples

Given a `CHANGELOG.md` containing:

```
## v0.4.0 (2026-05-07)

### Added

- New thing A.

### Fixed

- Bug B.

## v0.3.1 (2026-04-23)

Patch.
```

Then `extract-changelog-section.sh v0.4.0` prints to stdout:

```

### Added

- New thing A.

### Fixed

- Bug B.

```

(Note the leading and trailing blank lines, which are part of the
section. The release-notes body on GitHub renders them harmlessly.)

And `extract-changelog-section.sh v0.3.1` prints:

```

Patch.
```

## Error contract

- Missing or empty `<tag>` argument: stderr "usage:
  extract-changelog-section.sh <tag> [<changelog-path>]"; exit 2.
- `<changelog-path>` does not exist or is unreadable: stderr
  "extract-changelog-section.sh: cannot read <path>"; exit 2.
- No matching `## <tag> (` heading found in the file: stderr
  "extract-changelog-section.sh: no section for tag <tag> in
  <path>"; exit 1.
- Otherwise: exit 0.

## Caller obligations

- The CI workflow MUST `actions/checkout@v4` the repository before
  invoking the script so `CHANGELOG.md` is on disk.
- The Go regression test MUST exec the script via `os/exec` from
  the repository root (where `CHANGELOG.md` lives) and compare
  stdout to its assertions.

## Rationale notes

- The script is the SINGLE canonical parser of `CHANGELOG.md` for
  release-note extraction. CI YAML MUST NOT inline `awk` or `sed`
  duplicating this logic; the regression test MUST NOT
  re-implement it. (Principle XIII canonical code path.)
- The release-time gate `changelog-gate` does NOT call this script;
  it only `grep -F`s for the literal heading. The two checks have
  different shapes (gate = "exists?", extract = "give me the
  body"), and entangling them would be over-engineering. Both
  agree on the heading shape `## <tag> (`.
