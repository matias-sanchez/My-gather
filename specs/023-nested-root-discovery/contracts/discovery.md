# Contract: Nested Root Discovery

**Feature**: 023-nested-root-discovery
**Date**: 2026-05-08

This contract pins the public Go-level surface of the canonical
discovery function and the user-visible CLI behaviour that depends
on it.

## Go API contract

### `parse.FindPtStalkRoot`

```go
// FindPtStalkRoot walks rootDir and returns the absolute path of
// the single pt-stalk root contained in its tree. It is the
// canonical implementation behind both the directory-input and the
// archive-extracted-input code paths.
//
// Behaviour:
//
//   - If rootDir does not exist, is unreadable, or is not a
//     directory, returns a wrapped *PathError. (Same shape as
//     parse.Discover.)
//   - If the bounded walk discovers exactly one directory satisfying
//     LooksLikePtStalkRoot, returns its absolute path with err==nil.
//   - If the bounded walk discovers two or more such directories,
//     returns a *MultiplePtStalkRootsError whose Roots field is
//     sorted lexically.
//   - If the bounded walk discovers zero such directories, returns
//     ErrNotAPtStalkDir.
//
// The walk is depth-bounded by opts.MaxDepth (default
// DefaultMaxRootSearchDepth = 8 below rootDir). It is also bounded
// by opts.MaxEntries (default DefaultMaxRootSearchEntries = 100000
// directory entries visited). Hidden subdirectories (name begins
// with ".") are skipped. Symbolic links to directories are not
// followed. Per-subdir read failures are tolerated; the walk
// continues with the rest of the tree.
//
// The walk is read-only: it opens no files, takes no locks, writes
// no files, and never mutates rootDir.
//
// Safe to call concurrently from different goroutines against
// different roots.
func FindPtStalkRoot(ctx context.Context, rootDir string, opts FindPtStalkRootOptions) (string, error)
```

```go
// FindPtStalkRootOptions configures FindPtStalkRoot.
//
// A zero MaxDepth or MaxEntries means "use the package default";
// negative values cause FindPtStalkRoot to return an error. Pass
// UnlimitedRootSearchDepth to disable the depth bound. IncludeHidden
// (default false) descends into hidden-named subdirectories; the
// archive-input call site sets it to true to preserve the unbounded
// pre-feature findExtractedPtStalkRoot behaviour.
type FindPtStalkRootOptions struct {
    MaxDepth      int
    MaxEntries    int
    IncludeHidden bool
}
```

```go
// UnlimitedRootSearchDepth is a sentinel callers can pass via
// FindPtStalkRootOptions.MaxDepth to disable the depth bound.
const UnlimitedRootSearchDepth = math.MaxInt32
```

```go
// MultiplePtStalkRootsError indicates that FindPtStalkRoot
// discovered more than one directory satisfying
// LooksLikePtStalkRoot in the bounded subtree of the input.
type MultiplePtStalkRootsError struct {
    // Roots holds the absolute paths of every discovered pt-stalk
    // root, sorted lexically. len(Roots) >= 2.
    Roots []string
}

// Error renders a multi-line, human-readable message that names
// every discovered root. The output is byte-deterministic across
// invocations on the same set of roots.
func (e *MultiplePtStalkRootsError) Error() string
```

```go
const (
    DefaultMaxRootSearchDepth   = 8
    DefaultMaxRootSearchEntries = 100000
    UnlimitedRootSearchDepth    = math.MaxInt32
)
```

### Caller-side defaults

| Caller                                      | `MaxDepth`                        | `IncludeHidden` |
|---------------------------------------------|-----------------------------------|-----------------|
| `cmd/my-gather/archive_input.go::prepareInput` directory branch | default (= 8)                     | `false`         |
| `cmd/my-gather/archive_input.go::prepareInput` archive branch   | `parse.UnlimitedRootSearchDepth` | `true`          |

The split exists because the directory-input path defends against
user-typed accidental misdirection, while the archive-input path
preserves the unbounded-walk behaviour of the pre-feature
`findExtractedPtStalkRoot` helper that the canonical walker
replaced. The bytes-extracted cap on archive input
(`maxArchiveExtractedBytes` = 1 GiB) and the per-walk
`MaxEntries` cap still bound resource use.

### Invariants

- **Determinism (Principle IV)**: For a given input tree,
  `FindPtStalkRoot` returns the same `rootPath` (or the same error
  with identically-ordered fields) on every invocation. This is
  guaranteed by sorting accumulated roots before returning.
- **Read-only (Principle II)**: The function performs no `os.Create`,
  `os.Mkdir`, `os.MkdirAll`, `os.Rename`, `os.Remove`, `os.RemoveAll`,
  `os.WriteFile`, or `ioutil.WriteFile` calls.
- **Single canonical owner (Principle XIII)**: This is the only
  function in the repository that walks a directory tree to locate
  a pt-stalk root. The previous private
  `cmd/my-gather/archive_input.go::findExtractedPtStalkRoot` is
  removed in the same change.

## CLI contract

### Directory input

```text
my-gather <directory> [-o <output>]
```

- If `<directory>` is itself recognised as a pt-stalk root (top-level
  signal present), it is parsed directly. No subdirectory walk runs.
  Behaviour is bit-identical to today.
- Otherwise, the CLI walks subdirectories of `<directory>` up to a
  depth of 8, skipping hidden subdirectories (`.git`, `.cache`, etc.)
  and not following symlinks. The walk stops after the first 100000
  directory entries.
  - Exactly one pt-stalk root found: the CLI parses and renders that
    root.
  - Multiple roots found: the CLI exits non-zero, writes no output
    file, and writes a multi-line stderr message naming every
    discovered root in lexical order with a one-line instruction to
    re-run pointing at one of them.
  - Zero roots found: the CLI exits non-zero, writes no output file,
    and writes a stderr message stating that the directory and its
    subdirectories were searched and no pt-stalk collection was
    found, including the exact depth limit so the operator can
    diagnose unusually nested layouts.

### Archive input

The archive-input code path is unchanged in observable behaviour. It
extracts to a process-owned temp directory, then calls the same
`parse.FindPtStalkRoot` to locate the pt-stalk root inside the
extraction directory, and produces the same outcomes (single root,
multiple roots, zero roots) with the same error types and exit
codes.

### `--help` text

The directory description in the help text is updated. Today's
text:

```text
ARGUMENTS
  <input>    Path to a pt-stalk output directory or supported archive
             (.zip, .tar, .tar.gz, .tgz, .gz).
```

After this feature:

```text
ARGUMENTS
  <input>    Path to a pt-stalk output directory (or a directory that
             contains one, searched up to 8 levels) or a supported
             archive (.zip, .tar, .tar.gz, .tgz, .gz).
```

### Exit codes

- `0` - report generated.
- The CLI's existing exit code for "not a pt-stalk directory" is
  used for the zero-root case.
- The CLI's existing exit code for "multiple pt-stalk roots" is
  used for the multi-root case.
- All other exit codes (path not found, permission denied on the
  input root, archive errors, parse errors, render errors) are
  unchanged.

### Stderr output

- All discovery-related lines respect the existing rule that
  structural errors write exactly one line (or one logical block
  in the multi-root case) to stderr. The block format for
  multi-root is one header line, one line per discovered root,
  one trailer line, all written before exit.
- All output is byte-deterministic for the same input tree.

## Test obligations

The following tests must exist in the repository before this
contract is considered satisfied:

- `parse/discover_root_test.go::TestFindPtStalkRoot_TopLevel` -
  given a synthetic input that IS a pt-stalk root, returns the
  absolute path with no error.
- `parse/discover_root_test.go::TestFindPtStalkRoot_NestedSingle` -
  given a synthetic input with one pt-stalk root nested 6 levels
  deep, returns the nested absolute path with no error.
- `parse/discover_root_test.go::TestFindPtStalkRoot_NestedMultiple` -
  given a synthetic input with two roots, returns
  `*MultiplePtStalkRootsError` with `Roots` sorted lexically and
  `len(Roots)==2`.
- `parse/discover_root_test.go::TestFindPtStalkRoot_NoRoot` -
  given a synthetic input with no roots, returns
  `ErrNotAPtStalkDir`.
- `parse/discover_root_test.go::TestFindPtStalkRoot_HiddenDirSkipped` -
  given a synthetic input where the only root lives under a
  `.cache/...` directory, returns `ErrNotAPtStalkDir`.
- `parse/discover_root_test.go::TestFindPtStalkRoot_DepthCap` -
  given a synthetic input where the only root lives at depth >
  MaxDepth, returns `ErrNotAPtStalkDir`.
- `parse/discover_root_test.go::TestFindPtStalkRoot_Deterministic` -
  invokes `FindPtStalkRoot` twice against the same multi-root
  fixture and compares the two error strings byte-for-byte.
- `parse/discover_root_test.go::TestFindPtStalkRoot_SymlinkNotFollowed` -
  given a synthetic input whose only "root" is reachable only via
  a symlinked subdirectory, returns `ErrNotAPtStalkDir`.
- `cmd/my-gather/main_test.go::TestCLIDirInputNestedSingle` -
  end-to-end: build a synthetic six-level-deep fixture, invoke the
  binary, verify exit code 0 and a non-empty output file.
- `cmd/my-gather/main_test.go::TestCLIDirInputMultipleRoots` -
  end-to-end: build a multi-root fixture, invoke the binary, verify
  non-zero exit, no output file, and stderr lines named in lexical
  order.
- `cmd/my-gather/main_test.go::TestCLIDirInputNoRoot` - end-to-end:
  invoke the binary on a folder containing only unrelated files,
  verify non-zero exit, no output file, and stderr message
  mentioning subdirectories were searched.
