package parse

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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

// DefaultMaxRootSearchEntries is the default total-entry cap applied
// by FindPtStalkRoot as a belt-and-braces guard against pathological
// directory trees. It is set well above any real-world capture corpus
// observed; if the cap is ever exceeded, the walk stops and the
// outcome is computed from the entries visited so far.
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
}

// MultiplePtStalkRootsError indicates that FindPtStalkRoot discovered
// more than one directory satisfying LooksLikePtStalkRoot in the
// bounded subtree of the input. Callers branch on this type via
// errors.As; the Roots field is sorted lexically and contains at
// least two entries.
type MultiplePtStalkRootsError struct {
	// Roots holds the absolute paths of every discovered pt-stalk
	// root, sorted lexically. len(Roots) is always at least 2.
	Roots []string
}

// Error renders a multi-line, human-readable message naming every
// discovered root. The output is byte-deterministic across invocations
// against the same set of roots (the Roots slice is sorted before
// formatting), satisfying the project's deterministic-output gate.
func (e *MultiplePtStalkRootsError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "multiple pt-stalk roots found (%d):\n", len(e.Roots))
	for _, r := range e.Roots {
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

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", &PathError{Op: "abs", Path: rootDir, Err: err}
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return "", &PathError{Op: "stat", Path: absRoot, Err: err}
	}
	if !info.IsDir() {
		return "", &PathError{Op: "stat", Path: absRoot, Err: errors.New("not a directory")}
	}

	// Top-level fast path: if rootDir itself is a pt-stalk root, do
	// not walk subdirectories at all. This preserves byte-identical
	// behaviour for the existing already-a-root invocation and avoids
	// the corner where a root contains a subdirectory that itself
	// looks like a pt-stalk root - which would otherwise produce a
	// spurious multi-root error.
	if ok, err := LooksLikePtStalkRoot(absRoot); err != nil {
		return "", err
	} else if ok {
		return absRoot, nil
	}

	var roots []string
	entries := 0

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
		if !entry.IsDir() {
			return nil
		}

		entries++
		if entries > maxEntries {
			// Stop the walk; outcome is computed from what we have.
			return fs.SkipAll
		}

		// Compute depth relative to absRoot. depth==0 is absRoot
		// itself; the fast path above already handled that case, but
		// keep the check in case of unusual layouts.
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return nil
		}
		depth := 0
		if rel != "." {
			depth = strings.Count(rel, string(filepath.Separator)) + 1
		}

		// Hidden-dir skip applies to descendants only; the input root
		// itself is whatever the caller passed.
		name := entry.Name()
		if depth > 0 && strings.HasPrefix(name, ".") {
			return fs.SkipDir
		}

		// Recognise this directory as a pt-stalk root.
		if depth > 0 {
			ok, err := LooksLikePtStalkRoot(path)
			if err != nil {
				// A read error on this directory is treated like a
				// per-subdir failure: prune and continue.
				return fs.SkipDir
			}
			if ok {
				roots = append(roots, path)
				// Do not descend into a recognised root - its
				// children are pt-stalk content, not nested roots.
				return fs.SkipDir
			}
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
