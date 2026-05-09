package parse

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultMaxRootSearchDepth is the default subdirectory-depth ceiling
// applied by FindPtStalkRoot. It bounds the walk so a misdirected
// invocation against a large unrelated directory tree (for example a
// downloads folder) cannot drag the walk on indefinitely. The local
// real-world capture corpus surveyed at the time of writing places the
// deepest pt-stalk root 6 directories below the input root the
// operator typically points at; an 8-level cap leaves 2 levels of
// headroom while still preventing pathological walks.
const DefaultMaxRootSearchDepth = 8

// UnlimitedRootSearchDepth is a sentinel value callers can pass via
// FindPtStalkRootOptions.MaxDepth to disable the depth bound entirely.
// The archive-input call site uses it together with IncludeHidden=true
// to preserve the unbounded-walk behaviour of the pre-feature
// findExtractedPtStalkRoot helper, which never imposed a depth cap or
// hidden-directory filter on extracted archive contents. Directory
// inputs continue to use DefaultMaxRootSearchDepth so a misdirected
// invocation cannot drag the walk on indefinitely.
const UnlimitedRootSearchDepth = math.MaxInt32

// DefaultMaxRootSearchEntries is the default total-entry cap applied
// by FindPtStalkRoot as a belt-and-braces guard against pathological
// directory trees. It is set well above any real-world capture corpus
// observed; if the cap is ever exceeded, the walk stops and the
// outcome is computed from the entries visited so far.
//
// The cap is checked once per WalkDir callback, so the actual entry
// count when the walk stops can exceed the limit by up to one full
// directory listing. WalkDir reads each directory's entries in one
// shot via os.ReadDir before invoking the callback for any of them,
// so a single high-fanout directory at any depth is fully enumerated
// before the cap can fire. For the pt-stalk capture corpus the
// per-directory entry count is small (snapshots x collectors) and
// this looseness is irrelevant; for inputs that point at unrelated
// trees with millions of files in one directory the cap will not
// short-circuit the os.ReadDir cost itself.
const DefaultMaxRootSearchEntries = 100000

// FindPtStalkRootOptions configures FindPtStalkRoot. The zero value is
// valid and applies DefaultMaxRootSearchDepth and
// DefaultMaxRootSearchEntries.
type FindPtStalkRootOptions struct {
	// MaxDepth caps how many subdirectory levels below the input root
	// the walk will descend. Depth 0 is the input root itself; depth 1
	// is its immediate children, and so on. A zero value means "use
	// DefaultMaxRootSearchDepth"; negative values cause FindPtStalkRoot
	// to return an error synchronously.
	MaxDepth int

	// MaxEntries bounds the total number of directory entries the walk
	// will visit before stopping. A zero value means "use
	// DefaultMaxRootSearchEntries"; negative values cause
	// FindPtStalkRoot to return an error synchronously.
	MaxEntries int

	// IncludeHidden controls whether subdirectories whose name starts
	// with "." are descended into. The default (false) skips them,
	// which is correct for directory inputs: real-world case folders
	// contain .git, .cache, and similar metadata directories that
	// never hold pt-stalk captures, and excluding them keeps the walk
	// fast. The archive-input call site sets this to true to preserve
	// the unbounded-walk behaviour of the pre-feature
	// findExtractedPtStalkRoot helper, which had no such filter and
	// would therefore find a pt-stalk root even if the archive happened
	// to extract its contents under a hidden-named top-level
	// directory.
	IncludeHidden bool
}

// MultiplePtStalkRootsError indicates that FindPtStalkRoot discovered
// more than one directory satisfying LooksLikePtStalkRoot in the
// bounded subtree of the input. Callers branch on this type via
// errors.As. When produced by FindPtStalkRoot, Roots is sorted
// lexically and len(Roots) is at least 2; Error() defensively sorts
// any externally-constructed Roots slice so its rendering remains
// byte-deterministic regardless of how the value was built.
type MultiplePtStalkRootsError struct {
	// Roots holds the absolute paths of every discovered pt-stalk
	// root. When produced by FindPtStalkRoot, the slice is sorted
	// lexically and len(Roots) >= 2. Externally-constructed values
	// are accepted as-is; Error() sorts a local copy at format time.
	Roots []string
}

// Error renders a multi-line, human-readable message naming every
// discovered root. The output is byte-deterministic across invocations
// against the same set of roots: a sorted copy of e.Roots is used for
// formatting so external callers that construct this error directly
// still get deterministic output even if they pass an unsorted slice.
// e.Roots itself is not mutated.
func (e *MultiplePtStalkRootsError) Error() string {
	roots := make([]string, len(e.Roots))
	copy(roots, e.Roots)
	sort.Strings(roots)
	var b strings.Builder
	fmt.Fprintf(&b, "multiple pt-stalk roots found (%d):\n", len(roots))
	for _, r := range roots {
		fmt.Fprintf(&b, "  %s\n", r)
	}
	b.WriteString("re-run pointing at one of these paths")
	return b.String()
}

// FindPtStalkRoot walks rootDir and returns the absolute path of the
// single pt-stalk root contained in its tree. It is the canonical
// implementation behind both the directory-input and the archive-
// extracted-input code paths.
//
// Behaviour:
//
//   - If rootDir does not exist, is unreadable, or is not a directory,
//     returns a wrapped *PathError. (Same shape as parse.Discover.)
//   - If the bounded walk discovers exactly one directory satisfying
//     LooksLikePtStalkRoot, returns its absolute path with err==nil.
//   - If the bounded walk discovers two or more such directories,
//     returns a *MultiplePtStalkRootsError whose Roots field is
//     sorted lexically.
//   - If the bounded walk discovers zero such directories, returns
//     ErrNotAPtStalkDir.
//
// The walk is depth-bounded by opts.MaxDepth (default
// DefaultMaxRootSearchDepth = 8 below rootDir). It is also bounded by
// opts.MaxEntries (default DefaultMaxRootSearchEntries = 100000
// directory entries visited). Hidden subdirectories (name begins with
// ".") are skipped. Symbolic links to directories are not followed.
// Per-subdir read failures are tolerated; the walk continues with
// the rest of the tree.
//
// The walk is read-only: it opens no files, takes no locks, writes
// no files, and never mutates rootDir.
//
// Safe to call concurrently from different goroutines against
// different roots.
func FindPtStalkRoot(ctx context.Context, rootDir string, opts FindPtStalkRootOptions) (string, error) {
	if opts.MaxDepth < 0 {
		return "", errors.New("parse: FindPtStalkRootOptions.MaxDepth must be non-negative")
	}
	if opts.MaxEntries < 0 {
		return "", errors.New("parse: FindPtStalkRootOptions.MaxEntries must be non-negative")
	}
	maxDepth := opts.MaxDepth
	if maxDepth == 0 {
		maxDepth = DefaultMaxRootSearchDepth
	}
	maxEntries := opts.MaxEntries
	if maxEntries == 0 {
		maxEntries = DefaultMaxRootSearchEntries
	}

	absInput, err := filepath.Abs(rootDir)
	if err != nil {
		return "", &PathError{Op: "abs", Path: rootDir, Err: err}
	}
	// Resolve the root symlink before handing the path to WalkDir.
	// WalkDir uses Lstat on its root, which means a symlinked-dir
	// input presents to the callback as IsDir()==false and falls
	// through to ErrNotAPtStalkDir even though parse.Discover
	// (which follows symlinks via os.Stat) would have accepted it.
	// Resolving here keeps directory-input behaviour symmetric with
	// the pre-feature path and with archive inputs (whose tempDir
	// is never a symlink). EvalSymlinks is a no-op for paths that
	// are not symlinks.
	absRoot, err := filepath.EvalSymlinks(absInput)
	if err != nil {
		return "", &PathError{Op: "evalsymlinks", Path: absInput, Err: err}
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return "", &PathError{Op: "stat", Path: absRoot, Err: err}
	}
	if !info.IsDir() {
		return "", &PathError{Op: "stat", Path: absRoot, Err: errors.New("not a directory")}
	}

	var roots []string
	entries := 0

	// Walk every directory in the bounded subtree, including absRoot
	// itself. Both absRoot and any subdirectory that satisfies
	// LooksLikePtStalkRoot is appended to roots; the walker MUST keep
	// descending into a recognised root's children so a sibling or
	// cousin root in the same subtree is not missed (Codex round-3
	// finding: a pt-stalk root that lives directly at the input AND
	// has another nested root below must surface as a multi-root
	// ambiguity, not silently parse the top-level alone). Real
	// pt-stalk captures do not contain subdirectories that themselves
	// satisfy LooksLikePtStalkRoot, so this descent does not produce
	// false positives in practice.
	walkErr := filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			// Tolerate per-subdir read failures (FR-010, Principle III).
			// If the failure was on the input root itself, propagate so
			// the caller sees the structural error - otherwise prune
			// this branch and continue.
			if path == absRoot {
				return walkErr
			}
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry == nil {
			return nil
		}

		// Count every entry the walker visits - directories and files
		// alike - so the cap protects against pathological trees that
		// have few directories but many files. Bumping only on
		// directories would leave a high-fanout single directory able
		// to bypass the cap entirely.
		entries++
		if entries > maxEntries {
			// Stop the walk; outcome is computed from what we have.
			return fs.SkipAll
		}

		if !entry.IsDir() {
			return nil
		}

		// Compute depth relative to absRoot. depth==0 is absRoot
		// itself.
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return nil
		}
		depth := 0
		if rel != "." {
			depth = strings.Count(rel, string(filepath.Separator)) + 1
		}

		// Hidden-dir skip applies to descendants only; the input root
		// itself is whatever the caller passed. The filter is bypassed
		// when opts.IncludeHidden is true (used by the archive-input
		// call site to preserve unbounded-walk behaviour).
		name := entry.Name()
		if depth > 0 && !opts.IncludeHidden && strings.HasPrefix(name, ".") {
			return fs.SkipDir
		}

		// Recognise this directory as a pt-stalk root, including
		// absRoot itself. The walker keeps descending into a
		// recognised root so that any other roots in the same subtree
		// are also collected and the multi-root ambiguity surfaces.
		ok, err := LooksLikePtStalkRoot(path)
		if err != nil {
			// A read error on this directory is treated like a
			// per-subdir failure: prune and continue.
			return fs.SkipDir
		}
		if ok {
			roots = append(roots, path)
		}

		// Depth cap: do not descend past maxDepth.
		if depth >= maxDepth {
			return fs.SkipDir
		}
		return nil
	})

	if walkErr != nil && !errors.Is(walkErr, fs.SkipAll) {
		// Only surface walk errors that are not the deliberate
		// SkipAll signal. Context cancellation and structural input-
		// root failures arrive here.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		return "", &PathError{Op: "walk", Path: absRoot, Err: walkErr}
	}

	switch len(roots) {
	case 0:
		return "", ErrNotAPtStalkDir
	case 1:
		return roots[0], nil
	default:
		sort.Strings(roots)
		return "", &MultiplePtStalkRootsError{Roots: roots}
	}
}
