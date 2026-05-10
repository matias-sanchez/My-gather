# Data Model: Nested Root Discovery for Directory Inputs

**Feature**: 023-nested-root-discovery
**Date**: 2026-05-08

This feature is a control-flow change, not a data-shape change. The
report data model (`model/`) is not touched. The "data model" here is
the small set of input options, output values, and error types that
the new canonical discovery function exchanges with its callers.

## Inputs

### `parse.FindPtStalkRootOptions`

Configuration for the canonical walker.

| Field           | Type   | Default                                        | Notes                                                                                                                          |
|-----------------|--------|------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------|
| `MaxDepth`      | `int`  | `parse.DefaultMaxRootSearchDepth` (= 8)        | Hard cap on subdirectory depth below the input root. Depth 0 = input root itself. Pass `parse.UnlimitedRootSearchDepth` to disable the cap. |
| `MaxEntries`    | `int`  | `parse.DefaultMaxRootSearchEntries` (= 100000) | Belt-and-braces guard against pathological trees. The walk stops after this many directory entries are visited and returns whatever roots it has. |
| `IncludeHidden` | `bool` | `false`                                        | When false, the walker skips subdirectories whose name starts with `.` (the directory-input default - real case folders contain `.git`, `.cache`, etc. that never hold pt-stalk captures). When true, hidden subdirectories are descended (the archive-input default, preserving the pre-feature `findExtractedPtStalkRoot` behaviour). |

A zero value on `MaxDepth` or `MaxEntries` means "use the default". A
negative value is an error returned synchronously from
`FindPtStalkRoot` (the shape of `parse.Discover`'s existing
zero/negative-bound rule, kept for symmetry). The
`UnlimitedRootSearchDepth` constant (`math.MaxInt32`) is the explicit
opt-out for the depth bound; archive inputs pass it together with
`IncludeHidden: true` so that customer archives whose pt-stalk root
nests deeper than 8 levels or sits under a hidden-named subdirectory
remain accepted.

The struct is intentionally extensible: future fields (e.g., a
custom diagnostic sink) can be added without breaking callers.

## Outputs

### Successful single-root return

```go
rootPath, err := parse.FindPtStalkRoot(ctx, inputDir, opts)
// err == nil, rootPath is an absolute filesystem path
```

`rootPath` is the absolute path of the directory inside the input
tree that satisfies `parse.LooksLikePtStalkRoot`. When multiple
candidates share the same depth (impossible in practice for a single
walk, but the contract still pins the order), the lexically-first
path wins.

### Errors

#### `parse.ErrNotAPtStalkDir`

Sentinel; existing. Returned when the walker visits the input root
plus the bounded subtree without finding a single directory that
satisfies `LooksLikePtStalkRoot`.

| Property | Value                                       |
|----------|---------------------------------------------|
| Kind     | sentinel `error` (`errors.New`-style)       |
| Match    | `errors.Is(err, parse.ErrNotAPtStalkDir)`   |

Reused as-is; no changes to its definition.

#### `parse.MultiplePtStalkRootsError`

New exported struct. Returned when the walker finds more than one
distinct directory satisfying `LooksLikePtStalkRoot` in the bounded
subtree.

| Field   | Type       | Description                                                      |
|---------|------------|------------------------------------------------------------------|
| `Roots` | `[]string` | Absolute paths of every discovered root, sorted lexically (`sort.Strings`). |

Methods:

- `Error() string` - returns a multi-line message suitable for
  printing to stderr. The first line names the count and the input
  path; the next `len(Roots)` lines list each path; the final line
  tells the operator to re-run pointing at one of them.

Match: `var mpe *parse.MultiplePtStalkRootsError; errors.As(err, &mpe)`.

Invariants:

- `len(Roots) >= 2`. A `MultiplePtStalkRootsError` with fewer than
  two roots is a programming error, not a domain outcome.
- `Roots` is sorted lexically. Tests compare against the sorted
  slice; the CLI's stderr output is byte-deterministic.
- Each entry in `Roots` is an absolute path (the walker computes
  absolute paths via `filepath.Abs` on the input root and joins from
  there).

#### Other errors

The walker may also return:

- `*parse.PathError` - when the input directory itself cannot be
  stat'ed or read (existing type; keep semantics).
- `ctx.Err()` - when the caller's context is cancelled mid-walk.
  The walker checks `ctx.Err()` between entries.

## Discovery Outcome (caller's perspective)

A caller of `parse.FindPtStalkRoot` reasons in three branches:

1. `err == nil` -> use `rootPath` as the parse root. This is the
   success case for both directory-input and archive-input call
   sites.
2. `errors.Is(err, parse.ErrNotAPtStalkDir)` -> the input tree did
   not contain a pt-stalk capture within the bounded subtree. The
   CLI converts this into the user-facing "no pt-stalk collection
   found" message and exits with the existing
   `is-not-a-pt-stalk-dir` exit code.
3. `errors.As(err, &MultiplePtStalkRootsError{})` -> the input tree
   contained more than one capture. The CLI converts this into the
   existing "multiple pt-stalk roots" message and exits with the
   existing exit code.

Branch (2) and (3) are mutually exclusive by construction.

## State Transitions

The walker is stateless from one invocation to the next; it owns no
on-disk or in-memory state outside its local accumulator. Each
invocation walks fresh. There are no resumable iterations and no
caching layer; the cost of repeated invocations on the same input
is the cost of the walk itself, which is bounded by the depth and
entry caps.

## Relationship to existing types

- `parse.LooksLikePtStalkRoot(rootDir)` (existing) - reused
  unchanged. The walker calls it on every visited directory.
- `parse.Discover(ctx, rootDir, opts)` (existing) - unchanged. It
  remains a single-root parser; the new walker is a separate
  concern that selects which root `Discover` runs on.
- `parse.ErrNotAPtStalkDir` (existing) - reused unchanged.
- `parse.PathError`, `parse.SizeError` (existing) - reused
  unchanged.
- `parse.DiscoverOptions` (existing) - unchanged. The new
  `FindPtStalkRootOptions` is a sibling type, not a replacement.
