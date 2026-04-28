// my-gather — turn a pt-stalk output directory into one self-
// contained HTML diagnostic report.
//
// Exit codes are normative and listed in
// specs/001-ptstalk-report-mvp/contracts/cli.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/matias-sanchez/My-gather/model"
	"github.com/matias-sanchez/My-gather/parse"
	"github.com/matias-sanchez/My-gather/render"
)

// Build-time injectables. The Makefile wires these via -ldflags.
var (
	version = "v0.0.0-dev"
	commit  = "unknown"
	builtAt = "unknown"
)

// Exit codes (see contracts/cli.md).
const (
	exitOK             = 0
	exitUsage          = 2
	exitInputPath      = 3
	exitNotAPtStalkDir = 4
	exitSizeBound      = 5
	exitOutputExists   = 6
	exitOutputInsideIn = 7
	exitInternal       = 70
)

var errOutputExists = errors.New("output exists")

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point — main only wraps it with os.Exit
// against the real stdio. Tests pass recording Writers and assert on
// their contents.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("my-gather", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usageText) }

	var (
		outPath   string
		overwrite bool
		verbose   bool
		showVer   bool
		showHelp  bool
	)
	fs.StringVar(&outPath, "out", "", "output HTML path (default: ./report.html)")
	fs.StringVar(&outPath, "o", "", "output HTML path (default: ./report.html)")
	fs.BoolVar(&overwrite, "overwrite", false, "overwrite output if it already exists")
	fs.BoolVar(&verbose, "verbose", false, "emit per-file progress to stderr")
	fs.BoolVar(&verbose, "v", false, "emit per-file progress to stderr")
	fs.BoolVar(&showVer, "version", false, "print version and exit")
	fs.BoolVar(&showHelp, "help", false, "print help and exit")
	fs.BoolVar(&showHelp, "h", false, "print help and exit")

	if err := fs.Parse(args); err != nil {
		// ContinueOnError prints the error itself; we just map to the
		// usage exit code.
		return exitUsage
	}
	if showHelp {
		fmt.Fprint(stdout, usageText)
		return exitOK
	}
	if showVer {
		fmt.Fprintf(stdout, "my-gather %s\n  commit:   %s\n  go:       %s\n  built:    %s\n  platform: %s/%s\n",
			version, commit, runtime.Version(), builtAt, runtime.GOOS, runtime.GOARCH)
		return exitOK
	}

	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(stderr, "my-gather: missing required <input-dir>")
		fmt.Fprint(stderr, "See 'my-gather --help'.\n")
		return exitUsage
	}
	if len(rest) > 1 {
		fmt.Fprintf(stderr, "my-gather: expected exactly one <input-dir>, got %d\n", len(rest))
		return exitUsage
	}
	inputDir := rest[0]
	if outPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "my-gather: could not resolve working directory: %v\n", err)
			return exitInternal
		}
		outPath = filepath.Join(cwd, "report.html")
	}

	absInput, err := filepath.Abs(inputDir)
	if err != nil {
		fmt.Fprintf(stderr, "my-gather: input path: %v\n", err)
		return exitInputPath
	}
	absOut, err := filepath.Abs(outPath)
	if err != nil {
		fmt.Fprintf(stderr, "my-gather: output path: %v\n", err)
		return exitInputPath
	}

	// Resolve symlinks so the inside-input guard is robust against
	// pointer shenanigans. Tolerate a missing output parent for now —
	// we'll error cleanly at write time if it's genuinely unwritable.
	resolvedInput := resolveIfExists(absInput)
	resolvedOutParent := resolveIfExists(filepath.Dir(absOut))
	resolvedOut := filepath.Join(resolvedOutParent, filepath.Base(absOut))

	// FR-029: refuse if output path resolves inside input tree.
	if pathIsUnder(resolvedOut, resolvedInput) {
		fmt.Fprintf(stderr, "my-gather: output %q resolves inside input directory %q; refusing to write\n",
			absOut, absInput)
		return exitOutputInsideIn
	}

	// Early existence check for the overwrite guard (FR-002 / exit 6).
	if _, err := os.Stat(absOut); err == nil && !overwrite {
		fmt.Fprintf(stderr, "my-gather: output %q already exists; pass --overwrite to replace\n", absOut)
		return exitOutputExists
	} else if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "my-gather: stat output %q: %v\n", absOut, err)
		return exitInternal
	}

	// Wire a DiagnosticSink that mirrors warnings and errors to stderr
	// per spec FR-027. SeverityInfo NEVER mirrors to stderr (F13).
	sink := &stderrSink{w: stderr}

	ctx := context.Background()
	if verbose {
		fmt.Fprintf(stderr, "[parse] reading %s\n", absInput)
	}
	collection, err := parse.Discover(ctx, absInput, parse.DiscoverOptions{Sink: sink})
	if err != nil {
		return mapDiscoverError(err, absInput, stderr)
	}
	if verbose {
		fmt.Fprintf(stderr, "[parse] %d snapshot(s), %.1f MB total\n",
			len(collection.Snapshots), float64(collection.PtStalkSize)/(1024*1024))
	}

	if verbose {
		fmt.Fprintf(stderr, "[render] writing %s\n", absOut)
	}
	if err := writeAtomic(absOut, collection, overwrite); err != nil {
		if errors.Is(err, errOutputExists) {
			fmt.Fprintf(stderr, "my-gather: output %q already exists; pass --overwrite to replace\n", absOut)
			return exitOutputExists
		}
		fmt.Fprintf(stderr, "my-gather: render: %v\n", err)
		return exitInternal
	}
	if verbose {
		st, _ := os.Stat(absOut)
		if st != nil {
			fmt.Fprintf(stderr, "[done]  %d bytes written\n", st.Size())
		}
	}
	return exitOK
}

// mapDiscoverError converts a parse.Discover error into the correct
// exit code and writes a one-line explanation to stderr.
func mapDiscoverError(err error, inputPath string, stderr io.Writer) int {
	if errors.Is(err, parse.ErrNotAPtStalkDir) {
		fmt.Fprintf(stderr, "my-gather: %s is not recognised as a pt-stalk output directory\n", inputPath)
		return exitNotAPtStalkDir
	}
	var sz *parse.SizeError
	if errors.As(err, &sz) {
		switch sz.Kind {
		case parse.SizeErrorTotal:
			fmt.Fprintf(stderr, "my-gather: collection size %d bytes exceeds %d-byte limit at %s\n",
				sz.Bytes, sz.Limit, sz.Path)
		case parse.SizeErrorFile:
			fmt.Fprintf(stderr, "my-gather: source file %s is %d bytes (limit %d)\n",
				sz.Path, sz.Bytes, sz.Limit)
		default:
			fmt.Fprintln(stderr, sz.Error())
		}
		return exitSizeBound
	}
	var pe *parse.PathError
	if errors.As(err, &pe) {
		fmt.Fprintf(stderr, "my-gather: %s: %v\n", pe.Path, pe.Err)
		return exitInputPath
	}
	fmt.Fprintf(stderr, "my-gather: %v\n", err)
	return exitInternal
}

// writeAtomic writes the rendered HTML to a sibling temp file, then
// installs that completed temp file at the target path. In overwrite
// mode, install uses os.Rename. In non-overwrite mode, install uses a
// same-directory hard link so the final step cannot replace a file
// created after the early existence check.
func writeAtomic(outPath string, c *model.Collection, overwrite bool) error {
	dir := filepath.Dir(outPath)
	base := filepath.Base(outPath)
	tmp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	defer func() {
		if tmp != nil {
			_ = tmp.Close()
		}
	}()

	opts := render.RenderOptions{
		GeneratedAt: time.Now().UTC(),
		Version:     version,
		GitCommit:   commit,
		BuiltAt:     builtAt,
	}
	if err := render.Render(tmp, c, opts); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp: %w", err)
	}
	tmp = nil
	return installRenderedFile(tmpPath, outPath, overwrite)
}

func installRenderedFile(tmpPath, outPath string, overwrite bool) error {
	if overwrite {
		if err := os.Rename(tmpPath, outPath); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("rename: %w", err)
		}
		return nil
	}
	if err := os.Link(tmpPath, outPath); err != nil {
		_ = os.Remove(tmpPath)
		if os.IsExist(err) {
			return errOutputExists
		}
		return fmt.Errorf("link output: %w", err)
	}
	if err := os.Remove(tmpPath); err != nil {
		return fmt.Errorf("remove temp: %w", err)
	}
	return nil
}

// stderrSink implements parse.DiagnosticSink and mirrors
// SeverityWarning / SeverityError entries to stderr. SeverityInfo is
// silent per spec FR-027 (F13 resolution).
type stderrSink struct {
	w io.Writer
}

func (s *stderrSink) OnDiagnostic(d model.Diagnostic) {
	switch d.Severity {
	case model.SeverityWarning:
		fmt.Fprintf(s.w, "[warning] %s: %s\n", shortPath(d.SourceFile), d.Message)
	case model.SeverityError:
		fmt.Fprintf(s.w, "[error]   %s: %s\n", shortPath(d.SourceFile), d.Message)
	}
}

func shortPath(p string) string {
	if p == "" {
		return "(collection)"
	}
	return filepath.Base(p)
}

// resolveIfExists returns filepath.EvalSymlinks(p) when p exists, else
// returns p unchanged. We tolerate a missing path on the output side
// (the target file doesn't exist yet) but still want the input side to
// resolve through any symlinks before the inside-input check.
func resolveIfExists(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// pathIsUnder reports whether child is inside (or equal to) parent,
// using cleaned absolute paths. Matches FR-029's output-path guard.
func pathIsUnder(child, parent string) bool {
	if parent == "" || child == "" {
		return false
	}
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	if child == parent {
		return true
	}
	sep := string(filepath.Separator)
	if !strings.HasSuffix(parent, sep) {
		parent += sep
	}
	return strings.HasPrefix(child, parent)
}

const usageText = `my-gather — self-contained HTML reports for pt-stalk collections

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
  0   success
  2   usage error
  3   input path missing or unreadable
  4   input is not a pt-stalk directory
  5   input exceeds supported size bounds (1 GB total / 200 MB per file)
  6   output path exists (use --overwrite to replace)
  7   output path resolves inside input directory (refusing to write)
  70  internal error (please report)
`
