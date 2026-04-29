# Data Model: Archive Input

## Input Path

The positional argument provided to `my-gather`.

Fields:

- `path`: absolute path after CLI resolution
- `kind`: directory, supported archive, or unsupported regular file
- `display_path`: original absolute path used for user-facing errors

Rules:

- Directories pass through to `parse.Discover`.
- Regular files must match a supported archive format.
- Other file types fail as input-path errors.

## Temporary Extraction Directory

A private directory created for one archive run.

Fields:

- `path`: absolute temporary directory path
- `owner`: the current `my-gather` process
- `lifetime`: from successful archive classification until command exit

Rules:

- Created outside the input tree.
- Removed on success and failure.
- Never used for directory input.

## Archive Entry

One member inside a compressed archive.

Fields:

- `name`: archive-provided path
- `type`: directory, regular file, metadata, or unsupported special entry
- `target`: safe resolved path under the temporary extraction directory
- `bytes_written`: uncompressed bytes written

Rules:

- Regular files and directories are allowed.
- Symlinks, hardlinks, devices, and traversal paths are rejected.
- Total extracted bytes are bounded by the existing collection-size limit.

## Extracted pt-stalk Root

The single extracted directory that contains pt-stalk recognition signals.

Recognition signals:

- A top-level timestamped pt-stalk collector filename:
  `YYYY_MM_DD_HH_MM_SS-<suffix>`
- `pt-summary.out`
- `pt-mysql-summary.out`

Rules:

- Exactly one extracted root is required.
- Zero roots map to "not a pt-stalk directory".
- Multiple roots map to an ambiguous input error because the current CLI
  renders one collection per invocation.
