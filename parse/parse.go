package parse

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// DefaultMaxCollectionBytes is the default upper bound on total
// collection size (spec FR-025). 1 GB.
const DefaultMaxCollectionBytes int64 = 1 << 30

// DefaultMaxFileBytes is the default upper bound on any individual
// source-file size (spec FR-025). 200 MB.
const DefaultMaxFileBytes int64 = 200 << 20

// DiagnosticSink receives parser diagnostics as they are recorded.
// Implementations MUST be cheap; Discover blocks on the callback.
// cmd/my-gather wires this to mirror warnings and errors to stderr
// per spec FR-027.
type DiagnosticSink interface {
	OnDiagnostic(d model.Diagnostic)
}

// DiscoverOptions are optional knobs for Discover. The zero value is
// valid and applies DefaultMaxCollectionBytes and DefaultMaxFileBytes.
type DiscoverOptions struct {
	// Sink receives parser diagnostics synchronously as they are
	// recorded. May be nil.
	Sink DiagnosticSink

	// MaxCollectionBytes overrides DefaultMaxCollectionBytes. Zero
	// means use the default. Negative values are rejected.
	MaxCollectionBytes int64

	// MaxFileBytes overrides DefaultMaxFileBytes.
	MaxFileBytes int64
}

// snapshotPrefix matches a pt-stalk timestamp prefix followed by a
// collector suffix — e.g., "2026_04_21_16_52_11-iostat". The trailing
// dash and suffix are required.
var snapshotPrefix = regexp.MustCompile(`^(\d{4}_\d{2}_\d{2}_\d{2}_\d{2}_\d{2})-([a-zA-Z0-9._-]+)$`)

// Summary-file names used as a secondary directory-recognition
// signal per research R5.
var summaryFiles = []string{"pt-summary.out", "pt-mysql-summary.out"}

// Discover walks rootDir, recognises pt-stalk snapshots, and returns
// a fully-populated model.Collection. Behaviour:
//
//   - If rootDir does not exist, is unreadable, or is not a directory,
//     returns a wrapped *PathError.
//   - If the total collection or any individual file exceeds the
//     configured size bounds, returns a wrapped *SizeError.
//   - If rootDir contains zero timestamped pt-stalk files AND no
//     pt-summary.out / pt-mysql-summary.out, returns
//     ErrNotAPtStalkDir.
//   - Otherwise returns a *model.Collection. Per-collector parse
//     failures are NOT surfaced as errors here — they are recorded as
//     model.Diagnostic entries on the affected SourceFile, matching
//     Constitution Principle III (graceful degradation).
//
// Discover is read-only: it opens files with O_RDONLY, takes no locks,
// writes no files, and never mutates rootDir.
//
// Discover is safe to call concurrently from different goroutines
// against different directories. It does not itself spawn goroutines
// (determinism concerns dominate; parallel parsing is a future
// feature).
//
// In v1 Discover only discovers and classifies files. Per-collector
// parsing (populating each SourceFile.Parsed) is wired in by per-
// story parser implementations in tasks T051, T060, and T074.
func Discover(ctx context.Context, rootDir string, opts DiscoverOptions) (*model.Collection, error) {
	if opts.MaxCollectionBytes < 0 || opts.MaxFileBytes < 0 {
		return nil, errors.New("parse: DiscoverOptions size bounds must be non-negative")
	}
	maxCollection := opts.MaxCollectionBytes
	if maxCollection == 0 {
		maxCollection = DefaultMaxCollectionBytes
	}
	maxFile := opts.MaxFileBytes
	if maxFile == 0 {
		maxFile = DefaultMaxFileBytes
	}

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, &PathError{Op: "abs", Path: rootDir, Err: err}
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, &PathError{Op: "stat", Path: absRoot, Err: err}
	}
	if !info.IsDir() {
		return nil, &PathError{Op: "stat", Path: absRoot, Err: errors.New("not a directory")}
	}

	// First pass: enumerate top-level entries; compute total size;
	// enforce per-file size bound on individual source files; identify
	// timestamped snapshot files vs summary files.
	entries, err := os.ReadDir(absRoot)
	if err != nil {
		return nil, &PathError{Op: "readdir", Path: absRoot, Err: err}
	}

	var (
		totalBytes      int64
		snapshotMatches []snapshotFileMatch
		summaryPresent  bool
		hostname        string
	)

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entry.IsDir() {
			// Sub-directories (e.g., "samples/") are not walked in v1;
			// the seven supported collectors live at the root of a
			// pt-stalk dump.
			continue
		}
		fi, err := entry.Info()
		if err != nil {
			return nil, &PathError{Op: "stat", Path: filepath.Join(absRoot, entry.Name()), Err: err}
		}
		size := fi.Size()
		totalBytes += size

		name := entry.Name()
		fullPath := filepath.Join(absRoot, name)

		// Recognise snapshot files.
		if m := snapshotPrefix.FindStringSubmatch(name); m != nil {
			if size > maxFile {
				return nil, &SizeError{
					Kind:  SizeErrorFile,
					Path:  fullPath,
					Bytes: size,
					Limit: maxFile,
				}
			}
			ts, err := parseSnapshotTimestamp(m[1])
			if err != nil {
				// A clearly-non-timestamp prefix shouldn't match the regex,
				// but defend anyway: record the anomaly as a collection-
				// wide info diagnostic and skip.
				continue
			}
			snapshotMatches = append(snapshotMatches, snapshotFileMatch{
				prefix:    m[1],
				suffix:    m[2],
				timestamp: ts,
				path:      fullPath,
				size:      size,
			})
			continue
		}

		// Recognise summary files.
		for _, s := range summaryFiles {
			if name == s {
				summaryPresent = true
				break
			}
		}

		// A bare -hostname file at the root (not timestamped) is not a
		// pt-stalk convention we have seen, but some customer dumps
		// include one. Harmless to ignore.
		_ = name
	}

	if totalBytes > maxCollection {
		return nil, &SizeError{
			Kind:  SizeErrorTotal,
			Path:  absRoot,
			Bytes: totalBytes,
			Limit: maxCollection,
		}
	}

	if len(snapshotMatches) == 0 && !summaryPresent {
		return nil, ErrNotAPtStalkDir
	}

	// Derive hostname from the first -hostname snapshot file if present,
	// otherwise leave empty. pt-summary.out parsing is out of scope for
	// v1; the hostname field remains "" when only a summary file exists.
	hostname = readHostnameFrom(snapshotMatches)

	// Group matches by snapshot prefix. Only snapshots that contain at
	// least one recognised Suffix contribute SourceFiles entries; the
	// seven supported suffixes are defined in model.KnownSuffixes. Other
	// suffixes (hostname, disk-space, sysctl, …) are silently skipped
	// in v1.
	knownSuffixes := make(map[string]model.Suffix, len(model.KnownSuffixes))
	for _, s := range model.KnownSuffixes {
		knownSuffixes[string(s)] = s
	}

	type groupedSnapshot struct {
		prefix    string
		timestamp time.Time
		files     map[model.Suffix]*model.SourceFile
	}

	groups := make(map[string]*groupedSnapshot)

	for _, m := range snapshotMatches {
		s, ok := knownSuffixes[m.suffix]
		if !ok {
			continue
		}
		g := groups[m.prefix]
		if g == nil {
			g = &groupedSnapshot{
				prefix:    m.prefix,
				timestamp: m.timestamp,
				files:     make(map[model.Suffix]*model.SourceFile),
			}
			groups[m.prefix] = g
		}
		g.files[s] = &model.SourceFile{
			Suffix:  s,
			Path:    m.path,
			Size:    m.size,
			Version: model.FormatUnknown, // set by the per-collector parser when it runs
			Status:  model.ParseFailed,   // pessimistic default; overwritten by the parser
		}
	}

	// Even when no snapshot contains a supported collector, we still
	// construct at least one synthetic Snapshot so render can produce
	// a report with every section showing "data not available" (spec
	// FR-020 post-F18 fix, Edge Cases #2).
	//
	// Synthetic snapshot choice:
	//   - If any timestamped files exist at all, use the earliest
	//     timestamp (regardless of whether its suffix is supported).
	//   - Otherwise (only summary files present), use the current
	//     wall-clock time as a placeholder — render treats this as a
	//     non-time-series view and the GeneratedAt header is the
	//     user-facing timestamp anyway.
	if len(groups) == 0 {
		var synthTS time.Time
		var synthPrefix string
		if len(snapshotMatches) > 0 {
			earliest := snapshotMatches[0]
			for _, m := range snapshotMatches[1:] {
				if m.timestamp.Before(earliest.timestamp) {
					earliest = m
				}
			}
			synthTS = earliest.timestamp
			synthPrefix = earliest.prefix
		} else {
			synthTS = time.Now().UTC()
			synthPrefix = synthTS.Format("2006_01_02_15_04_05")
		}
		groups[synthPrefix] = &groupedSnapshot{
			prefix:    synthPrefix,
			timestamp: synthTS,
			files:     make(map[model.Suffix]*model.SourceFile),
		}
	}

	// Materialise Snapshots in timestamp-ascending order.
	snapshots := make([]*model.Snapshot, 0, len(groups))
	for _, g := range groups {
		snapshots = append(snapshots, &model.Snapshot{
			Timestamp:   g.timestamp,
			Prefix:      g.prefix,
			SourceFiles: g.files,
		})
	}
	sort.Slice(snapshots, func(i, j int) bool {
		if !snapshots[i].Timestamp.Equal(snapshots[j].Timestamp) {
			return snapshots[i].Timestamp.Before(snapshots[j].Timestamp)
		}
		// Stable fallback: ordering by prefix string is deterministic.
		return snapshots[i].Prefix < snapshots[j].Prefix
	})

	return &model.Collection{
		RootPath:    absRoot,
		Hostname:    hostname,
		PtStalkSize: totalBytes,
		Snapshots:   snapshots,
		Diagnostics: nil, // populated by per-collector parsers in US2-US4
	}, nil
}

// snapshotFileMatch is an internal record of one timestamped pt-stalk
// file discovered at the collection root.
type snapshotFileMatch struct {
	prefix    string // e.g. "2026_04_21_16_52_11"
	suffix    string // e.g. "iostat", "hostname"
	timestamp time.Time
	path      string
	size      int64
}

// parseSnapshotTimestamp parses a pt-stalk prefix (format
// "YYYY_MM_DD_HH_MM_SS") into a UTC time.Time.
//
// pt-stalk emits the host's local time without a zone; without extra
// context we cannot convert to UTC reliably. For v1 we treat the
// timestamp as UTC for rendering purposes — the report's "generated at"
// banner indicates the render time, and per-sample timestamps relative
// to each other are preserved (which is what matters for the charts).
func parseSnapshotTimestamp(s string) (time.Time, error) {
	return time.Parse("2006_01_02_15_04_05", s)
}

// readHostnameFrom reads the first byte-stripped line of the first
// -hostname file encountered, if any. Returns "" when no -hostname file
// is present or the file is empty / unreadable.
func readHostnameFrom(matches []snapshotFileMatch) string {
	for _, m := range matches {
		if m.suffix != "hostname" {
			continue
		}
		data, err := os.ReadFile(m.path)
		if err != nil {
			return ""
		}
		text := strings.TrimSpace(string(data))
		if text == "" {
			continue
		}
		// First line only.
		if idx := strings.IndexByte(text, '\n'); idx >= 0 {
			return strings.TrimSpace(text[:idx])
		}
		return text
	}
	return ""
}
