# Research: Archive Input

## Incident Inventory

Local scan target:

```text
/Users/matias/Documents/Incidents/
```

The scan excluded source-code attachments under `code/` and `mysql-server/`
because those folders contain unrelated compressed repository artifacts.

Results:

- Parse-compatible pt-stalk candidate directories: 171
- Archive files: 560
- Archive formats observed:
  - `.tar.gz`: 200
  - `.zip`: 188
  - `.gz`: 144
  - `.tar`: 18
  - `.tgz`: 10
- Archive filenames containing `stalk`: 3

Inventory files were written outside the repository for local review:

```text
/tmp/my-gather-pt-stalk-candidate-dirs.txt
/tmp/my-gather-archives.txt
/tmp/my-gather-likely-stalk-archives.txt
```

Conclusion: archive support cannot depend on filenames. It must inspect
extracted content and discover pt-stalk roots by the same signals used for
directory input.

## Decision: Supported Formats

Support `.zip`, `.tar`, `.tar.gz`, `.tgz`, and `.gz`.

Rationale:

- These are the only formats observed in the refined incident inventory.
- They are covered by Go standard-library readers.
- `.gz` appears frequently and can wrap either one file or a tar stream with a
  non-standard filename, so content detection is useful.

Rejected for this feature:

- `.bz2`, `.xz`, `.7z`, `.rar`: not observed in the refined inventory and would
  require either new dependencies or partial platform-specific tooling.

## Decision: Temporary Extraction

Archives extract into a private directory created with `os.MkdirTemp` and
removed with `os.RemoveAll` on every exit path.

Rationale:

- Preserves the read-only input-tree principle.
- Avoids leaving partially extracted customer data in the working directory.
- Keeps archive handling transparent to the existing parser and renderer.

## Decision: Safe Archive Paths

Extraction rejects:

- Absolute paths
- Parent-directory traversal
- Symlinks
- Hardlinks
- Devices and other non-regular entries

Rationale:

- Customer archives are untrusted input.
- The tool must never write outside its temporary extraction directory.
- Following links would violate deterministic and read-only behavior.

## Decision: Root Selection

After extraction, the CLI walks extracted directories and calls
`parse.LooksLikePtStalkRoot`. Exactly one matching root is required.

Rationale:

- Existing `parse.Discover` intentionally parses one root directory.
- Multiple extracted roots usually mean multi-host or multi-capture archives.
  Silent selection would risk rendering the wrong host.
- Reusing parse recognition signals avoids a second, divergent definition of
  "pt-stalk root".
