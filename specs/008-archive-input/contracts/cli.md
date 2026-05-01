# CLI Contract: Archive Input

This contract amends the v1 CLI contract for feature `008-archive-input`.

## Invocation

```text
my-gather [flags] <input>
```

- `<input>` is required.
- `<input>` may be either:
  - a pt-stalk output directory
  - a supported archive file: `.zip`, `.tar`, `.tar.gz`, `.tgz`, `.gz`

Exactly one positional input is accepted. Zero or multiple positional inputs
remain usage errors.

## Archive Behavior

For directory input, behavior is unchanged.

For archive input, the CLI must:

1. Create a private temporary extraction directory.
2. Safely extract regular files and directories only.
3. Reject entries that would escape the temporary directory.
4. Reject links and other special entries.
5. Locate exactly one extracted pt-stalk root.
6. Call the existing parser with that extracted root.
7. Remove temporary extraction files before exit.

## Exit Code Mapping

| Code | Archive-specific meaning |
|------|--------------------------|
| `3` | Regular file uses an unsupported archive format, archive cannot be opened, or archive path is unsafe. |
| `4` | Supported archive contains zero pt-stalk roots or multiple pt-stalk roots. |
| `5` | Extracted content exceeds the supported total input bound. |
| `70` | Unexpected temporary directory or write failure. |

Existing directory-input exit code meanings remain unchanged.

## Help Text

Help output must describe `<input>` as a pt-stalk directory or supported
archive, not as a directory-only argument.
