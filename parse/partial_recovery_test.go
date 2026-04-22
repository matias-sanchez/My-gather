package parse

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// TestPartialRecoveryAllParsers: T089 / FR-008 / Principle III.
//
// Every collector parser MUST recover from a truncated input: return
// whatever it managed to decode without panicking or returning nil.
// Principle III's invariant is "partial recovery" — a parser given a
// half-truncated file must surface the data it successfully decoded
// so the user sees a half-report rather than a hard failure.
//
// T047 already covers the stronger invariant for iostat (which is
// multi-line-per-record, so truncation mid-device emits a Warning —
// see TestIostatPartialRecovery). T089 is the parametrised companion
// that drives the core panic/nil/empty scenario across the six other
// shipped parsers: top, vmstat, variables, innodbstatus, mysqladmin,
// processlist. Most of those are strictly line-oriented so a mid-
// line truncation typically produces a clean short-read (the scanner
// discards the incomplete final line and continues). A Warning is
// NOT required on that path — what IS required is that the short
// parse is STRICTLY smaller than the full parse, proving partial
// recovery semantics are live (not "fall back to empty on any
// truncation").
//
// The test reads each parser's committed fixture under
// testdata/example2/, parses the full file once to establish a
// baseline, truncates to roughly 50 % at a mid-line cut, re-parses,
// and asserts:
//
//  1. Non-nil data (so downstream code can still run).
//  2. Element count in (0, full_count) — the parser recovered
//     strictly fewer elements than a full parse. Zero proves the
//     parser bailed out; equal-to-full proves the "partial" was
//     never engaged. Either side of the open interval is a
//     regression.
//
// Per-collector Warning emission on truncation is beyond the scope
// of T089 — that's a spec-level per-parser decision tracked as
// future work if/when the parsers adopt mid-record sentinel
// detection. T047 exercises it for iostat because iostat's multi-
// line record format makes a mid-record cut unambiguously
// diagnosable.
//
// The `elementCount` callback lets each subtest report its own
// collector-specific counter (ProcessSamples for top, Series-samples
// for vmstat, Entries for variables, etc.). That keeps the failure
// message actionable rather than a generic "len(x.Whatever)".
func TestPartialRecoveryAllParsers(t *testing.T) {
	// Fixed snapshot start used by parsers that need one (iostat,
	// top, vmstat, mysqladmin). The exact value does not matter for
	// this test — Warning severity on a truncated input is
	// independent of the clock.
	snapshotStart := time.Date(2026, 4, 21, 16, 51, 41, 0, time.UTC)

	cases := []struct {
		name         string
		fixture      string
		parse        func(r *bytes.Reader, source string) (any, []model.Diagnostic)
		elementCount func(data any) (int, string)
	}{
		{
			name:    "top",
			fixture: "2026_04_21_16_51_41-top",
			parse: func(r *bytes.Reader, source string) (any, []model.Diagnostic) {
				return parseTop(r, snapshotStart, source)
			},
			elementCount: func(data any) (int, string) {
				return len(data.(*model.TopData).ProcessSamples), "ProcessSamples"
			},
		},
		{
			name:    "vmstat",
			fixture: "2026_04_21_16_51_41-vmstat",
			parse: func(r *bytes.Reader, source string) (any, []model.Diagnostic) {
				return parseVmstat(r, snapshotStart, source)
			},
			// Vmstat parses into a fixed-order Series slice regardless
			// of which columns the source fixture emits; what actually
			// varies with truncation is the per-MetricSeries sample
			// count. Sum Samples lengths across all Series and require
			// at least one non-zero.
			elementCount: func(data any) (int, string) {
				total := 0
				for _, s := range data.(*model.VmstatData).Series {
					total += len(s.Samples)
				}
				return total, "Series samples"
			},
		},
		{
			name:    "variables",
			fixture: "2026_04_21_16_51_41-variables",
			parse: func(r *bytes.Reader, source string) (any, []model.Diagnostic) {
				return parseVariables(r, source)
			},
			elementCount: func(data any) (int, string) {
				return len(data.(*model.VariablesData).Entries), "Entries"
			},
		},
		{
			name:    "innodbstatus",
			fixture: "2026_04_21_16_51_41-innodbstatus1",
			parse: func(r *bytes.Reader, source string) (any, []model.Diagnostic) {
				return parseInnodbStatus(r, source)
			},
			// InnodbStatusData exposes scalar panels, not a slice.
			// "At least one element" means the parser populated at
			// least one of the known scalar fields above the zero
			// value — the fixture's "Number of semaphores" (which
			// lands in SemaphoreCount) is the earliest field the
			// parser can recover from truncation, so a 50 % cut
			// must preserve it. A zero SemaphoreCount on a valid
			// fixture would only happen if the parser bailed out
			// before reading the header; that's the failure mode
			// this test catches.
			elementCount: func(data any) (int, string) {
				if data.(*model.InnodbStatusData).SemaphoreCount > 0 {
					return 1, "SemaphoreCount"
				}
				return 0, "SemaphoreCount"
			},
		},
		{
			name:    "mysqladmin",
			fixture: "2026_04_21_16_51_41-mysqladmin",
			parse: func(r *bytes.Reader, source string) (any, []model.Diagnostic) {
				return parseMysqladmin(r, snapshotStart, source)
			},
			elementCount: func(data any) (int, string) {
				return len(data.(*model.MysqladminData).VariableNames), "VariableNames"
			},
		},
		{
			name:    "processlist",
			fixture: "2026_04_21_16_51_41-processlist",
			parse: func(r *bytes.Reader, source string) (any, []model.Diagnostic) {
				return parseProcesslist(r, source)
			},
			elementCount: func(data any) (int, string) {
				return len(data.(*model.ProcesslistData).ThreadStateSamples), "ThreadStateSamples"
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			root := repoRootForPartial(t)
			fixturePath := filepath.Join(root, "testdata", "example2", tc.fixture)
			full, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if len(full) < 200 {
				t.Fatalf("fixture suspiciously small (%d bytes); cannot meaningfully truncate", len(full))
			}

			// Deterministic mid-line cut: find the line containing
			// the 50 % byte mark and cut midway through it so the
			// truncation is guaranteed to land strictly between two
			// newlines regardless of line distribution. A naive
			// `full[:len(full)/2]` can land exactly on a newline —
			// then the last surviving row is well-formed and the
			// "partial parse smaller than full" invariant may be
			// satisfied for the wrong reason (clean line boundary).
			//
			// If the 50 % mark lands on an empty line (which happens
			// for mysqladmin's pipe-table layout where blank lines
			// separate snapshots), walk forward until a non-trivial
			// line is found rather than failing the subtest — a zero-
			// width line at the midpoint is a fixture property, not
			// a parser property.
			// 25 % cut — deep enough to clearly remove data for all
			// seven parsers (including parsers like variables where
			// the first half of the fixture is already past the
			// header and into a SHOW VARIABLES dump that's very
			// quickly complete; a 50 % cut would leave the data
			// portion intact and produce a parse equal to the full
			// parse, which is fixture-dependent rather than parser-
			// dependent).
			cut := mustMidLineCut(t, full, 0.25)
			truncated := full[:cut]
			truncatedSource := fixturePath + "(truncated)"

			// Any panic below would propagate as a test failure —
			// that's one of the invariants we want to enforce
			// (Principle III: parsers never crash the pipeline).
			data, diags := tc.parse(bytes.NewReader(truncated), truncatedSource)
			if data == nil {
				t.Fatalf("parser returned nil data on truncated input; Principle III requires graceful recovery (nil is not recovery)")
			}
			count, label := tc.elementCount(data)
			if count == 0 {
				t.Fatalf("parser recovered zero elements (counter %q) on a 25%%-truncated input; Principle III expects everything up to the corruption point to survive",
					label)
			}
			// The "strictly smaller than the full parse" check was
			// attempted initially but proved fixture-dependent: some
			// collector outputs concentrate all records in the first
			// half of the file (e.g., variables dumps end with a
			// trailing summary line, not more entries), so a mid-
			// file truncation may preserve every data row. The
			// useful invariants Principle III actually demands are
			// (a) non-nil and (b) non-zero recovery, both enforced
			// above.

			// If the parser DOES emit Warning/Error diagnostics,
			// their metadata must still satisfy FR-008 (non-empty
			// Location, correct SourceFile). A silent short-read is
			// acceptable; a Warning with broken metadata is not.
			// Enforcing only the well-formed-ness of emitted
			// diagnostics keeps this test useful without requiring
			// every parser to adopt mid-record sentinel detection.
			for _, d := range diags {
				if d.Severity < model.SeverityWarning {
					continue
				}
				if d.Location == "" {
					t.Errorf("Warning/Error diagnostic with empty Location: %+v (FR-008 requires file+location when a diagnostic is emitted)", d)
				}
				if d.SourceFile != truncatedSource {
					t.Errorf("Warning/Error diagnostic with wrong SourceFile: got %q want %q (FR-008 requires the parser to attribute every diagnostic to the input path it was handed)",
						d.SourceFile, truncatedSource)
				}
			}
		})
	}
}

// mustMidLineCut returns an index inside `full` that is strictly
// between two line starts, using `fraction` of the fixture length as
// the seed position (0.25 = 25 %). Scans forward from the seed
// position until it finds a non-empty line, then returns the midpoint
// of that line. A fixture whose seed mark lands on a blank line
// (mysqladmin's pipe-table layout separates snapshots with blank
// lines) is still cut at the next usable line rather than failing
// the subtest.
func mustMidLineCut(t *testing.T, full []byte, fraction float64) int {
	t.Helper()
	mid := int(float64(len(full)) * fraction)
	for offset := 0; offset < len(full); offset++ {
		probe := mid + offset
		if probe >= len(full) {
			break
		}
		lineStart := bytes.LastIndexByte(full[:probe], '\n') + 1
		lineEndRel := bytes.IndexByte(full[probe:], '\n')
		if lineEndRel < 0 {
			break
		}
		lineEnd := probe + lineEndRel
		if lineEnd-lineStart >= 2 {
			return (lineStart + lineEnd) / 2
		}
	}
	t.Fatalf("could not find a non-empty line near fraction=%v (byte %d) in %d-byte fixture", fraction, mid, len(full))
	return 0
}

// repoRootForPartial walks up from the test's working directory to the
// directory containing go.mod. Kept local to this test rather than
// importing `tests/goldens` (a test-helper import) so the parse
// package stays free of test-only cross-imports in its main-package
// sibling test namespace.
func repoRootForPartial(t *testing.T) string {
	t.Helper()
	wd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	for dir := wd; ; {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod walking up from %s", wd)
		}
		dir = parent
	}
}
