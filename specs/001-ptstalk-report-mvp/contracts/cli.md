# CLI Contract: `my-gather`

Normative surface for the `my-gather` command-line tool in v1. Any
change to this contract requires a spec amendment.

## Invocation

```text
my-gather [flags] <input>
```

- `<input>` — **Required positional argument.** Path to a pt-stalk
  output directory (or to a directory that contains one in a
  subdirectory; subdirectories are searched up to 8 levels) or to a
  supported archive file (`.zip`, `.tar`, `.tar.gz`, `.tgz`, `.gz`).
  Relative paths are resolved against the CWD before any further
  processing. The directory-input subdirectory search uses the
  canonical `parse.FindPtStalkRoot` walker; see
  `specs/023-nested-root-discovery/contracts/discovery.md` for the
  full directory-discovery contract (depth bound, hidden-dir skip,
  symlink handling, multi-root and zero-root error shapes).

Exactly one positional argument is accepted. Zero positional arguments
or two or more positional arguments is a usage error (exit code 2).

## Flags

| Flag | Short | Value | Default | Meaning |
|------|-------|-------|---------|---------|
| `--out` | `-o` | path | `./report.html` (CWD) | Output HTML file path. |
| `--overwrite` | — | boolean | `false` | Permit overwriting an existing output file. |
| `--verbose` | `-v` | boolean | `false` | Emit per-file progress to stderr. |
| `--version` | — | boolean | `false` | Print version info and exit 0. |
| `--help` | `-h` | boolean | `false` | Print usage and exit 0. |

Flag parsing uses Go's `flag` package. Long flags use `--name`; short
flags are the letters shown above. Unknown flags are a usage error
(exit code 2).

No environment-variable fallbacks. No config file. Any future knob is
an explicit flag.

## Exit codes

| Code | Meaning | Stderr content |
|------|---------|----------------|
| `0` | Success. HTML file written. | Empty on success (unless `-v`); parser diagnostics mirrored as they occur. |
| `2` | Usage error (missing positional, unknown flag, bad flag value). | One line describing the usage error + usage summary. |
| `3` | Input path does not exist, is unreadable, is neither a directory nor supported archive, or archive extraction is unsafe. | One line naming the path and the condition. |
| `4` | Input is not recognised as a single pt-stalk collection (no timestamped collectors and no `pt-summary.out` anywhere within the searched depth, or the input contains multiple candidate roots). For multi-root inputs, the message lists every discovered root in lexical order. | One line for the zero-root case; a multi-line block for the multi-root case (one header line, one line per root, one trailer line). |
| `5` | Input size bound exceeded (> 1 GB total or > 200 MB for any single source file). | One line naming the violated bound and the offending path. |
| `6` | Output path already exists and `--overwrite` was not set. | One line naming the output path. |
| `7` | Output path resolves to a location inside the input directory tree (would violate read-only-inputs principle). | One line naming both the input path and the offending output path. |
| `70` | Internal error. Should not occur in normal operation. Indicates a bug. | One line with error type + message. |

No other exit codes are used. In particular, parse failures on
individual collectors never translate into a non-zero exit (Principle
III) — they surface in the report and as stderr warnings.

## Stdout contract

- `--help`: prints usage text to stdout.
- `--version`: prints version info to stdout. Format:

  ```text
  my-gather <semver>
    commit:   <short-sha>
    go:       <runtime go version>
    built:    <iso-8601 build date in UTC>
    platform: <os>/<arch>
  ```

- Any other successful run: **stdout is empty.**

## Stderr contract

Per spec FR-027:

- **Silent on clean success** by default — when no parser diagnostics
  are recorded and `-v` is not set, stderr is empty. On a successful
  run that nonetheless produces warnings or errors from partial
  sources, stderr still carries those diagnostic lines.
- **Parser diagnostics** (model `Diagnostic` with `Severity >=
  SeverityWarning`) are mirrored to stderr as they are emitted, one per
  line, in the format:

  ```text
  [warning] <source-file-basename>: <message>
  [error]   <source-file-basename>: <message>
  ```

- **`-v` / `--verbose`** additionally emits one progress line per
  high-level step, each prefixed with a bracketed tag. The minimum
  contract is:

  ```text
  [parse]  reading <absolute-input-path>
  [parse]  <N> snapshot(s), <T> MB total
  [render] writing <absolute-output-path>
  [done]   <bytes> bytes written
  ```

  Implementations MAY append elapsed-time information to the `[done]`
  line (for example, `in <seconds>s`), and MAY substitute richer
  per-file progress (such as `[parse] 2026_04_21_16_52_11-iostat
  (1.2 MB) -> 3421 samples, 8 devices`) for the `[parse] <N>
  snapshot(s), <T> MB total` summary; neither is required by the
  minimum contract. Any concrete format MUST keep the `[tag] …`
  prefix shape so tests and operators can parse the stream
  deterministically.

- **Structural errors** (exit codes 2–6 and 70) always write exactly
  one line to stderr regardless of verbosity.

Stderr is **unbuffered line output**; each line flushes before the next
log call so partial outputs survive a SIGKILL.

## File I/O contract

- The tool MUST NOT write, rename, or delete anything inside directory
  input, archive input, or any subdirectory of a directory input
  (Principle II, FR-003). Archive input may be extracted only into a
  process-owned temporary directory outside the input tree, and that
  temporary directory MUST be removed before exit.
- The tool MUST write exactly one file to the `--out` path. No
  temporary side-car files under the output's parent dir, no lock
  files, no `.bak` files. If atomic-replace behaviour is needed the
  implementation writes to `<out>.tmp` in the same parent and renames;
  this intermediate file MUST NOT persist on success or failure (the
  tool removes it on failure).

## Help output

`--help` or `-h` output:

```text
my-gather — self-contained HTML reports for pt-stalk collections

USAGE
  my-gather [flags] <input>

ARGUMENTS
  <input>    Path to a pt-stalk output directory or supported archive
             (.zip, .tar, .tar.gz, .tgz, .gz).

FLAGS
  -o, --out <path>    Output HTML file path (default: ./report.html).
      --overwrite     Overwrite the output file if it already exists.
  -v, --verbose       Print per-file progress to stderr.
      --version       Print version information and exit.
  -h, --help          Print this help and exit.

EXIT CODES
  0  success
  2  usage error
  3  input path missing or unreadable
  4  input is not a pt-stalk directory
  5  input exceeds supported size bounds (1 GB total / 200 MB per file)
  6  output path exists (use --overwrite to replace)
  7  output path resolves inside input directory (refusing to write)
  70 internal error (please report)
```

Help output goes to stdout, never stderr.

## Output-path safety

Before calling into `parse.Discover`, `cmd/my-gather` MUST:

1. Resolve `<input>` to an absolute path with symlinks expanded
   (`filepath.EvalSymlinks` after `filepath.Abs`).
2. Resolve `--out` to an absolute path with symlinks expanded on the
   parent directory (the output file itself does not exist yet).
3. Reject (exit code 7) if the resolved output path begins with the
   resolved input path, or equals it.

This check is what enforces spec FR-029 and closes the Principle II
foot-gun where a naive `-o ./report.html` from inside the input tree
would land a write inside the pt-stalk dump.

For archive input, `cmd/my-gather` MUST safely extract the archive into
a temporary directory, discover exactly one pt-stalk root, and pass that
root directory to `parse.Discover`. Unsafe archive paths, unsupported
special entries, and multiple extracted pt-stalk roots are rejected.

## Backwards-compatibility policy

- Adding new flags is non-breaking.
- Adding new exit codes ≥ 7 is non-breaking.
- Changing the meaning of an existing exit code, renaming a flag, or
  altering the stdout `--version` format IS breaking and requires
  bumping the tool's major version.
