# Research: Nested Root Discovery for Directory Inputs

**Feature**: 023-nested-root-discovery
**Date**: 2026-05-08

This document records the design decisions made before implementation
and the alternatives considered. There were no `[NEEDS
CLARIFICATION]` markers in the spec; this research file therefore
captures the architectural choices that flow from the spec's
constraints rather than resolving open questions.

## Decision 1: Canonical walker lives in `parse/`, not `cmd/`

**Decision**: The shared root-discovery walker is implemented in the
`parse/` package, exported as `parse.FindPtStalkRoot`.

**Rationale**:

- Principle VI (Library-First Architecture) requires that the
  packages under `parse/`, `model/`, and `render/` be usable by
  third-party Go code without depending on `main`. A walker that
  takes a directory and returns "where the pt-stalk root is" is
  exactly the kind of reusable library logic that belongs in
  `parse/`.
- The function uses `parse.LooksLikePtStalkRoot` internally; that
  helper is already in `parse/parse.go`. Putting the walker next
  to its only collaborator keeps the package self-contained.
- Both consumers (directory-input prepareInput; archive-input
  prepareInput) live in `cmd/my-gather/`. Putting the canonical
  walker in `cmd/` would force at least one of them to import an
  internal helper from a sibling file. Lifting it to `parse/`
  removes that asymmetry.

**Alternatives considered**:

- Keep the walker in `cmd/my-gather/` and have the directory-input
  branch call the existing private `findExtractedPtStalkRoot`.
  Rejected: the function was only used inside the archive path
  before, and exposing a private CLI-package helper to be the
  canonical implementation across packages violates the
  library-first separation. It would also leak archive-specific
  naming (`Extracted...`) into a path that has nothing to do with
  archive extraction.
- Put the walker in a new `parse/discovery/` subpackage. Rejected
  as overkill: one exported function and one exported error type
  do not justify a new package, and it would split logic that
  pairs naturally with `LooksLikePtStalkRoot`.

## Decision 2: Use `filepath.WalkDir` with `SkipDir` returns

**Decision**: Implement the walker with `filepath.WalkDir`. Use the
`fs.WalkDirFunc` signature to (a) compute the current depth from the
relative path, (b) skip hidden directories by name, (c) call
`parse.LooksLikePtStalkRoot` on each directory entry, (d) return
`filepath.SkipDir` to prune subtrees, and (e) tolerate per-subdir
read errors by returning `nil` when `walkErr != nil` for a directory
entry.

**Rationale**:

- `filepath.WalkDir` does not follow symlinks by default. That is
  exactly the behaviour FR-009 requires; we get it for free.
- Returning `filepath.SkipDir` from the walk function is the
  idiomatic way to prune both for depth and hidden-name filtering.
- The walker only enumerates directories and stat-tests entries;
  it never opens a snapshot file for read. This keeps the
  performance bound proportional to directory entry count, not
  total bytes, and is exactly what SC-004 requires.

**Alternatives considered**:

- Hand-roll a recursive `os.ReadDir` traversal. Rejected: more code,
  more bugs, and `WalkDir` already gives us the symlink behaviour we
  need. The symlink default in `WalkDir` is the strongest argument.
- Use `filepath.Walk` (the older API). Rejected: `WalkDir` is the
  modern equivalent and is faster because it does not stat each
  entry up front.

## Decision 3: Top-level fast path stays in `prepareInput`

**Decision**: `cmd/my-gather/archive_input.go::prepareInput` for
directory inputs first calls `parse.LooksLikePtStalkRoot`. If true,
return the input as-is. If false, call `parse.FindPtStalkRoot` to
walk subdirectories. The top-level fast path runs no walk at all.

**Rationale**:

- Principle IV (Deterministic Output): the existing already-a-root
  case must remain bit-identical. Adding even a one-level walk for
  inputs that are already roots would risk a failure in a corner
  where the root contains a subdirectory that itself looks like a
  pt-stalk root - the walker would now report multi-root for an
  input that today succeeds.
- Performance: zero walk overhead for the established workflow.

**Alternatives considered**:

- Always walk, even when the top is a root. Rejected per the
  determinism risk above.
- Move the top-level signal check inside `FindPtStalkRoot` so
  callers do not need to remember the order. Rejected because the
  behaviour we want at the top level (return immediately) and the
  behaviour we want for nested input (walk and require exactly one
  root) are different enough that combining them inside one
  function would create branching that is harder to reason about
  than the two-line check at the call site.

## Decision 4: Depth cap = 8, entry cap = 100000

**Decision**: The walker accepts options and uses defaults of
`MaxDepth = 8` and `MaxEntries = 100000`. Reaching either cap stops
the walk; the walk returns whatever roots it has accumulated.

**Rationale**:

- The deepest observed real-world layout in the local capture corpus
  puts the pt-stalk root 6 directories below the input root the user
  typically points at (CS0061420). `MaxDepth = 8` gives 2 levels of
  headroom, which is enough to swallow a one- or two-step variation
  in a future layout without re-architecting.
- `MaxEntries = 100000` is well above any real-world capture corpus
  but bounds the worst case. Even on a downloads folder of
  unrelated material, the walker exits in time bounded by the cap.
- Both caps are exported as fields on `FindPtStalkRootOptions`, so
  callers can override them. The CLI uses the defaults.

**Alternatives considered**:

- `MaxDepth = 6` (zero headroom). Rejected: tight against the deepest
  observed layout, no margin for variation.
- `MaxDepth = 16` (very generous). Rejected: a higher cap means a
  more punishing worst case on unrelated trees, with no real-world
  benefit. 8 is the smallest safe number.
- No entry cap. Rejected: defence-in-depth against pathological
  trees costs almost nothing.

## Decision 5: Hidden-dir filter = name starts with `.`

**Decision**: Skip any directory entry whose `Name()` starts with
`.`. This is applied to both the input root's children and to all
descended subdirectories.

**Rationale**:

- Real-world case folders contain `.git`, `.cache`, `.DS_Store`-
  bearing directories, and similar metadata directories that never
  contain pt-stalk captures. Skipping them keeps the walk fast and
  prevents pathological output if some metadata directory ever
  contains a file that happened to match the recognition rule.
- The convention is universally understood: a leading `.` means
  hidden. Operators do not store pt-stalk captures inside a hidden
  directory.

**Alternatives considered**:

- Hard-code a denylist (`.git`, `.cache`, `.DS_Store`, ...).
  Rejected: brittle and incomplete; the leading-dot rule covers all
  current and future hidden directories.
- No filter; let the walker cover everything. Rejected for the
  performance and false-positive risks above.

## Decision 6: Tolerate per-subdir read errors silently

**Decision**: When `WalkDir` calls the walk function with a non-nil
`walkErr` for a directory entry, the function returns `nil` so the
walk continues. The walk does not abort and does not surface the
error. The single-root, multi-root, and zero-root outcomes are
computed from whatever subset of the tree the walker could read.

**Rationale**:

- Principle III (Graceful Degradation): the tool produces a useful
  outcome from whatever input is present.
- A common real-world cause of read errors deep in the tree is the
  user's own filesystem permissions on a downloaded archive's
  extracted contents (e.g., extracted with `sudo` then read as a
  regular user). Aborting the walk in that case would be unhelpful.
- The behaviour is by definition observable: if the unreadable
  subtree contained the only pt-stalk root, the walker reports
  zero roots and the user retries with elevated privileges. If it
  contained an extra root, the walker reports the visible roots
  and the user proceeds (or sees a multi-root error) - either way,
  no parallel hidden discovery path is taken.

**Alternatives considered**:

- Return an error to the caller. Rejected: makes the common case
  (one bad subdir, one good root elsewhere) hard to use.
- Log a structured diagnostic. Considered but rejected for v1: the
  CLI's discovery phase has no diagnostic sink wired up, and adding
  one is out of scope. The behaviour can be reconsidered if real
  reports show this matters.

## Decision 7: Promote `multiplePtStalkRootsError` to a public, exported type

**Decision**: Rename the private
`cmd/my-gather/archive_input.go::multiplePtStalkRootsError` to
`parse.MultiplePtStalkRootsError`, export it, and have both the CLI
and any future external caller branch on it via `errors.As`.

**Rationale**:

- Principle VII (Typed Errors): conditions that callers branch on
  must be expressed as typed errors with structured fields. The
  multi-root case is exactly such a condition.
- Principle XIII (Canonical Code Path): two distinct error types
  with the same meaning would be a duplication. One canonical type
  wins.

**Alternatives considered**:

- Keep the private type and have the CLI's error mapper handle the
  type via package-private `errors.As`. Rejected because it forces
  the canonical walker to live in `cmd/`, contradicting Decision 1.
- Use a sentinel `errors.New("multiple roots")` plus a sidecar
  function to extract the paths. Rejected: a struct error type with
  a field is the idiomatic Go pattern and is what Principle VII
  describes as the preferred shape.

## Decision 8: "No roots found" message wording

**Decision**: The zero-root error message format is:

```text
my-gather: <input-path> is not a pt-stalk output directory and no pt-stalk
collection was found in its subdirectories (searched up to depth 8)
```

The wording explicitly mentions both that the top level was checked
and that subdirectories were searched, and gives the exact depth
limit so the operator can tell whether they need to point deeper.

**Rationale**:

- FR-006 requires the message to "explicitly conveys both the
  directory was searched and that subdirectories were searched too."
- Naming the depth limit gives operators with unusually nested
  layouts a path forward.

**Alternatives considered**:

- Reuse the existing message verbatim ("is not recognised as a
  pt-stalk output directory"). Rejected: it predates the new walk
  behaviour and does not tell operators that subdirectories were
  searched.
- Print the depth limit only when verbose. Rejected: the failure
  surface is small and adding a verbose-only branch is more
  complexity than benefit.
