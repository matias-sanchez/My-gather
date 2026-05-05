package parse

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// maxScanTokenBytes bounds a single scanner token (line) to 32 MiB.
// Consolidates the magic number that used to appear verbatim in every
// parser's bufio.Scanner setup.
const maxScanTokenBytes = 32 << 20

// reTimestampLine matches the per-sample boundary marker pt-stalk
// writes at the top of every TS-delimited collector capture:
//
//	TS 1776790303.009325313 2026-04-21 16:51:43
//
// The shape is identical across all TS-prefixed collectors; every
// parser that consumes them MUST share this one compiled regex so a
// format change gets a single edit site.
var reTimestampLine = regexp.MustCompile(`^TS\s+(\d+(?:\.\d+)?)\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})`)

// newLineScanner returns a bufio.Scanner configured with the standard
// line-buffer sizes used by every parser in this package. All callers
// route through this helper so the buffer cap is enforced in one
// place.
func newLineScanner(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64*1024), maxScanTokenBytes)
	return s
}

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

// LooksLikePtStalkRoot reports whether rootDir contains the top-level
// signals Discover uses to recognize a pt-stalk collection. It does
// not parse source files or enforce size limits; callers use it only
// for cheap root selection before the canonical Discover pass.
func LooksLikePtStalkRoot(rootDir string) (bool, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return false, &PathError{Op: "abs", Path: rootDir, Err: err}
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return false, &PathError{Op: "stat", Path: absRoot, Err: err}
	}
	if !info.IsDir() {
		return false, &PathError{Op: "stat", Path: absRoot, Err: errors.New("not a directory")}
	}
	entries, err := os.ReadDir(absRoot)
	if err != nil {
		return false, &PathError{Op: "readdir", Path: absRoot, Err: err}
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if isPtStalkRootSignal(entry.Name()) {
			return true, nil
		}
	}
	return false, nil
}

func isPtStalkRootSignal(name string) bool {
	if snapshotPrefix.FindStringSubmatch(name) != nil {
		return true
	}
	for _, s := range summaryFiles {
		if name == s {
			return true
		}
	}
	return false
}

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
				// Apply the same per-file size cap as snapshot files so
				// a pathological 500 MB summary doesn't slip past the
				// guard a snapshot file would have triggered (FR-025).
				if size > maxFile {
					return nil, &SizeError{
						Kind:  SizeErrorFile,
						Path:  fullPath,
						Bytes: size,
						Limit: maxFile,
					}
				}
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
			// Determinism (Principle IV): avoid time.Now() so a
			// re-render of the same tarball produces byte-identical
			// output. When only summary files exist we emit a fixed
			// placeholder prefix instead.
			synthTS = time.Time{}
			synthPrefix = "unknown"
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

	// Invoke per-collector parsers for every SourceFile. Each parser is
	// self-contained and captures its own diagnostics on the SourceFile;
	// the overall Discover call still returns err==nil.
	sidecarContents, sidecarTimestamps, sidecarDiags := loadEnvSidecars(absRoot)
	collection := &model.Collection{
		RootPath:             absRoot,
		Hostname:             hostname,
		PtStalkSize:          totalBytes,
		Snapshots:            snapshots,
		RawEnvSidecars:       sidecarContents,
		EnvSidecarTimestamps: sidecarTimestamps,
	}
	for _, d := range sidecarDiags {
		collection.Diagnostics = append(collection.Diagnostics, d)
		if opts.Sink != nil {
			opts.Sink.OnDiagnostic(d)
		}
	}
	// Propagate ctx cancellation out of runParsers so a
	// deadline/cancel during per-collector parsing surfaces to the
	// caller instead of returning a half-populated Collection as a
	// silent success. Without this, timeout-controlled invocations
	// would get (collection, nil) with many SourceFiles still at
	// their default zero-value status, producing incomplete
	// reports without any signal that the run was aborted.
	if err := runParsers(ctx, collection, opts.Sink); err != nil {
		return nil, err
	}
	return collection, nil
}

// envSidecarSuffixes are the pt-stalk per-snapshot files the
// Environment panel consumes. They are not part of model.KnownSuffixes
// because they aren't time-series collectors — they're one-shot
// machine-inventory dumps. Loaded once at Discover time into
// Collection.RawEnvSidecars so render is a pure function of the model.
var envSidecarSuffixes = []string{
	"hostname", "meminfo", "procstat", "sysctl", "top", "df", "output",
}

// loadEnvSidecars reads the env-only sidecar files once, picking the
// newest file that matches "<prefix>-<suffix>" across the entire root
// directory — NOT just across the Collection's Snapshots. Partial
// captures sometimes have newer timestamp prefixes that only carry
// environment sidecars (no KnownSuffixes data), so the Collection's
// Snapshots slice may omit them; scanning the directory directly
// ensures those are considered. Prefix sort order is lexicographic,
// which matches chronological order for pt-stalk's
// `YYYY_MM_DD_HH_MM_SS` timestamp scheme.
//
// Missing/unreadable files map to absent keys in the returned maps;
// the render layer treats them as "data unavailable". The second
// returned map carries the timestamp parsed from the prefix of the
// chosen file — consumers that anchor point-in-time derivations (e.g.
// OS uptime from /proc/stat btime) should prefer it over the last
// snapshot's timestamp, since sidecar files can come from newer
// sidecar-only prefixes. The third return value is the diagnostic
// list (SeverityWarning) emitted when the loader falls back past a
// newer empty/unreadable file to an older readable one, so the
// degraded path is never silent.
func loadEnvSidecars(rootPath string) (map[string]string, map[string]time.Time, []model.Diagnostic) {
	if rootPath == "" {
		return nil, nil, nil
	}
	out := make(map[string]string, len(envSidecarSuffixes))
	timestamps := make(map[string]time.Time, len(envSidecarSuffixes))
	var diagnostics []model.Diagnostic
	for _, suf := range envSidecarSuffixes {
		matches, err := filepath.Glob(filepath.Join(rootPath, "*-"+suf))
		if err != nil || len(matches) == 0 {
			continue
		}
		sort.Sort(sort.Reverse(sort.StringSlice(matches)))
		// Track newer-but-skipped timestamp-prefixed files so that if we
		// end up using an older sidecar we can emit a single structured
		// diagnostic naming which newer files were rejected (and why).
		// Principle III: graceful degradation is allowed, silent
		// fallback is not.
		var skippedNewer []string
		for _, path := range matches {
			// The glob "*-<suf>" also matches unrelated files such as
			// `notes-hostname` that happen to end in "-<suf>". Skip any
			// filename whose prefix isn't a pt-stalk timestamp — otherwise
			// a stray file could sort last and win over the real sidecar,
			// and we'd also lose the timestamp anchor used for uptime.
			name := filepath.Base(path)
			m := snapshotPrefix.FindStringSubmatch(name)
			if m == nil {
				continue
			}
			ts, err := parseSnapshotTimestamp(m[1])
			if err != nil {
				continue
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				skippedNewer = append(skippedNewer, fmt.Sprintf("%s (read: %v)", name, readErr))
				continue
			}
			if len(data) == 0 {
				skippedNewer = append(skippedNewer, name+" (empty)")
				continue
			}
			out[suf] = string(data)
			timestamps[suf] = ts
			if suf == "meminfo" {
				_, memDiags := ParseEnvMeminfoWithDiagnostics(string(data), path)
				diagnostics = append(diagnostics, memDiags...)
			}
			if len(skippedNewer) > 0 {
				diagnostics = append(diagnostics, model.Diagnostic{
					SourceFile: path,
					Severity:   model.SeverityWarning,
					Message: fmt.Sprintf(
						"env sidecar -%s: fell back past %d newer file(s) that were empty or unreadable (%s); Environment panel values may not reflect the latest capture",
						suf, len(skippedNewer), strings.Join(skippedNewer, ", "),
					),
				})
			}
			break
		}
	}
	return out, timestamps, diagnostics
}

// runParsers iterates every SourceFile in every Snapshot and invokes
// its per-collector parser. Invoked from Discover. Returns ctx.Err()
// if the context is cancelled mid-iteration; per-collector parse
// failures are attached to the SourceFile as diagnostics, not
// returned, matching Constitution Principle III (graceful
// degradation).
func runParsers(ctx context.Context, c *model.Collection, sink DiagnosticSink) error {
	for _, snap := range c.Snapshots {
		for _, suffix := range model.KnownSuffixes {
			sf, ok := snap.SourceFiles[suffix]
			if !ok {
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			runOneParser(snap, sf, sink)
		}
	}
	return nil
}

func runOneParser(snap *model.Snapshot, sf *model.SourceFile, sink DiagnosticSink) {
	file, err := os.Open(sf.Path)
	if err != nil {
		diag := model.Diagnostic{
			SourceFile: sf.Path,
			Severity:   model.SeverityError,
			Message:    fmt.Sprintf("open: %v", err),
		}
		sf.Diagnostics = append(sf.Diagnostics, diag)
		if sink != nil {
			sink.OnDiagnostic(diag)
		}
		sf.Status = model.ParseFailed
		return
	}
	defer file.Close()

	// Peek first 8 KiB for version detection.
	var peek [8 * 1024]byte
	n, _ := io.ReadFull(file, peek[:])
	_, _ = file.Seek(0, io.SeekStart)
	sf.Version = DetectFormat(peek[:n], sf.Suffix)
	if sf.Version == model.FormatUnknown {
		diag := model.Diagnostic{
			SourceFile: sf.Path,
			Severity:   model.SeverityWarning,
			Message:    "unsupported pt-stalk format (header did not match FormatV1 or FormatV2)",
		}
		sf.Diagnostics = append(sf.Diagnostics, diag)
		if sink != nil {
			sink.OnDiagnostic(diag)
		}
		sf.Status = model.ParseUnsupported
		return
	}

	var (
		parsed      any
		diagnostics []model.Diagnostic
	)
	switch sf.Suffix {
	case model.SuffixIostat:
		parsed, diagnostics = parseIostat(file, snap.Timestamp, sf.Path)
	case model.SuffixTop:
		parsed, diagnostics = parseTop(file, snap.Timestamp, sf.Path)
	case model.SuffixVmstat:
		parsed, diagnostics = parseVmstat(file, snap.Timestamp, sf.Path)
	case model.SuffixMeminfo:
		parsed, diagnostics = parseMeminfo(file, sf.Path)
	case model.SuffixVariables:
		parsed, diagnostics = parseVariables(file, sf.Path)
	case model.SuffixInnodbStatus:
		parsed, diagnostics = parseInnodbStatus(file, sf.Path)
	case model.SuffixMysqladmin:
		parsed, diagnostics = parseMysqladmin(file, snap.Timestamp, sf.Path)
	case model.SuffixProcesslist:
		parsed, diagnostics = parseProcesslist(file, sf.Path)
	case model.SuffixNetstat:
		parsed, diagnostics = parseNetstat(file, snap.Timestamp, sf.Path)
	case model.SuffixNetstatS:
		parsed, diagnostics = parseNetstatS(file, snap.Timestamp, sf.Path)
	default:
		sf.Status = model.ParseFailed
		return
	}

	for _, d := range diagnostics {
		sf.Diagnostics = append(sf.Diagnostics, d)
		if sink != nil {
			sink.OnDiagnostic(d)
		}
	}

	switch {
	case parsed == nil:
		sf.Status = model.ParseFailed
	case len(diagnostics) > 0 && anyWarning(diagnostics):
		sf.Parsed = parsed
		sf.Status = model.ParsePartial
	default:
		sf.Parsed = parsed
		sf.Status = model.ParseOK
	}
}

func anyWarning(ds []model.Diagnostic) bool {
	for _, d := range ds {
		if d.Severity >= model.SeverityWarning {
			return true
		}
	}
	return false
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
