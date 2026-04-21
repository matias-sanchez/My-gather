# CLI Contract: `my-gather`

Normative surface for the `my-gather` command-line tool in v1. Any
change to this contract requires a spec amendment.

## Invocation

```text
my-gather [flags] <input-dir>
```

- `<input-dir>` — **Required positional argument.** Path to a pt-stalk
  output directory. Relative paths are resolved against the CWD before
  any further processing.

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
| `3` | Input path does not exist, is unreadable, or is not a directory. | One line naming the path and the condition. |
| `4` | Input directory is not recognised as a pt-stalk output directory (no timestamped collectors and no `pt-summary.out`). | One line. |
| `5` | Input size bound exceeded (> 1 GB total or > 200 MB for any single source file). | One line naming the violated bound and the offending path. |
| `6` | Output path already exists and `--overwrite` was not set. | One line naming the output path. |
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

- **Silent on success** by default. No progress chatter.
- **Parser diagnostics** (model `Diagnostic` with `Severity >=
  SeverityWarning`) are mirrored to stderr as they are emitted, one per
  line, in the format:

  ```text
  [warning] <source-file-basename>: <message>
  [error]   <source-file-basename>: <message>
  ```

- **`-v` / `--verbose`** additionally emits per-file progress lines:

  ```text
  [parse]  2026_04_21_16_52_11-iostat (1.2 MB) -> 3421 samples, 8 devices
  [parse]  2026_04_21_16_52_11-top    (812 kB) -> 148 batches, 94 processes
  [render] writing report.html
  [done]   42,117 bytes written in 3.8s
  ```

- **Structural errors** (exit codes 2–6 and 70) always write exactly
  one line to stderr regardless of verbosity.

Stderr is **unbuffered line output**; each line flushes before the next
log call so partial outputs survive a SIGKILL.

## File I/O contract

- The tool MUST NOT write, rename, or delete anything inside
  `<input-dir>` or any subdirectory of it (Principle II, FR-003).
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
  my-gather [flags] <input-dir>

ARGUMENTS
  <input-dir>    Path to a pt-stalk output directory (required).

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
  70 internal error (please report)
```

Help output goes to stdout, never stderr.

## Backwards-compatibility policy

- Adding new flags is non-breaking.
- Adding new exit codes ≥ 7 is non-breaking.
- Changing the meaning of an existing exit code, renaming a flag, or
  altering the stdout `--version` format IS breaking and requires
  bumping the tool's major version.
