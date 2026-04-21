// Package parse reads a pt-stalk output directory and produces a
// fully-populated model.Collection. Its public surface is documented
// in specs/001-ptstalk-report-mvp/contracts/packages.md.
package parse

import (
	"errors"
	"fmt"
)

// ErrNotAPtStalkDir reports that the input directory does not look
// like a pt-stalk output directory — specifically, it contains zero
// timestamped pt-stalk files AND neither pt-summary.out nor
// pt-mysql-summary.out (see research R5).
//
// Callers branch on this sentinel via errors.Is.
var ErrNotAPtStalkDir = errors.New("parse: not a pt-stalk directory")

// SizeErrorKind distinguishes the two ways a collection can violate
// the supported size bounds (spec FR-025).
type SizeErrorKind int

const (
	// SizeErrorTotal: the sum of file sizes under the input root
	// exceeds DiscoverOptions.MaxCollectionBytes.
	SizeErrorTotal SizeErrorKind = iota + 1

	// SizeErrorFile: at least one individual source file exceeds
	// DiscoverOptions.MaxFileBytes.
	SizeErrorFile
)

// SizeError reports that Discover refused to proceed because the
// input exceeds configured size bounds. Callers branch via
// errors.As.
type SizeError struct {
	Kind  SizeErrorKind
	Path  string // root path for SizeErrorTotal; offending file path for SizeErrorFile
	Bytes int64  // observed size
	Limit int64  // configured bound
}

// Error implements the error interface.
func (e *SizeError) Error() string {
	switch e.Kind {
	case SizeErrorTotal:
		return fmt.Sprintf("parse: collection size %d bytes exceeds limit %d bytes at %s",
			e.Bytes, e.Limit, e.Path)
	case SizeErrorFile:
		return fmt.Sprintf("parse: file size %d bytes exceeds per-file limit %d bytes at %s",
			e.Bytes, e.Limit, e.Path)
	default:
		return fmt.Sprintf("parse: size error: %d bytes exceeds %d at %s",
			e.Bytes, e.Limit, e.Path)
	}
}

// PathError wraps an os-level path failure with tool-specific context.
// Callers use errors.As(err, &PathError{}) to branch.
type PathError struct {
	Op   string // "open", "stat", "readdir"
	Path string
	Err  error
}

// Error implements the error interface.
func (e *PathError) Error() string {
	return fmt.Sprintf("parse: %s %s: %v", e.Op, e.Path, e.Err)
}

// Unwrap returns the underlying os error, supporting errors.Is /
// errors.As against stdlib os errors.
func (e *PathError) Unwrap() error { return e.Err }

// ParseError is the typed form of a per-file parsing failure. It is
// NOT returned from Discover; it is wrapped inside
// model.Diagnostic records attached to the affected SourceFile and
// exposed here only so test code can assert on the structure.
type ParseError struct {
	File     string // absolute path to the offending source file
	Location string // e.g., "line 412", "byte 102938"
	Err      error  // concrete cause (io, strconv, etc.)
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	if e.Location != "" {
		return fmt.Sprintf("parse: %s: %s: %v", e.File, e.Location, e.Err)
	}
	return fmt.Sprintf("parse: %s: %v", e.File, e.Err)
}

// Unwrap returns the concrete underlying cause.
func (e *ParseError) Unwrap() error { return e.Err }
