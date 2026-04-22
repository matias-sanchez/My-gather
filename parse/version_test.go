package parse

import (
	"strings"
	"testing"

	"github.com/matias-sanchez/My-gather/model"
)

// TestDetectFormat: T090 / FR-024 / research R2.
//
// parse.DetectFormat classifies a peeked byte slice from a pt-stalk
// source file into one of the supported FormatVersion enum values,
// branching on the file's Suffix. The shipped v1 of the detector
// implements per-collector V1 detection + conservative
// FormatUnknown for everything else; V1-vs-V2 discrimination is
// explicitly deferred (see the comment on the detect* helper
// family in parse/version.go).
//
// This test drives both halves of the contract:
//
//   - Positive half (7 subtests, one per Suffix): feed a
//     representative V1-shaped header byte sequence; assert
//     FormatV1. A regression that broke the header signature for
//     any collector would mean the parser stops receiving that
//     file and produces a blank section in the rendered report.
//
//   - Negative half (7 subtests, one per Suffix): feed bytes that
//     do NOT match the V1 header; assert FormatUnknown. Per FR-024,
//     the detector prefers FormatUnknown (which triggers an
//     "unsupported pt-stalk version" banner) over silently parsing
//     a format we don't recognise. A regression that broadened a
//     detector's match condition to accept arbitrary input would
//     be caught here.
//
// An additional default-case subtest feeds an unknown Suffix;
// expected result is FormatUnknown regardless of the peeked
// content.
//
// Why not also test FormatV2? Per version.go:43-48, V2 detection
// is explicitly deferred to future per-collector parser work. Once
// a parser learns to branch on V2, add a subtest here rather than
// expanding the detect* helpers silently.
func TestDetectFormat(t *testing.T) {
	// Each positive case contains a distinctive token the collector-
	// specific detector matches on. These are the stable header
	// words the shipped pt-stalk output emits across the supported
	// releases (see parse/version.go). Embedding them in a minimal
	// byte slice rather than a full fixture keeps the test fast and
	// focused — DetectFormat only reads the first few KB anyway.
	positives := []struct {
		name   string
		suffix model.Suffix
		peek   string
	}{
		{"iostat", model.SuffixIostat, "Linux 5.15.0 ...\n\navg-cpu:\nDevice            r/s     w/s ...\n"},
		{"top", model.SuffixTop, "top - 16:51:41 up 4 days, 2:03, 1 user, load average: ...\nPID USER      CPU%  COMMAND\n"},
		{"variables", model.SuffixVariables, "+-----------------------+----------------+\n| Variable_name         | Value          |\n+-----------------------+----------------+\n"},
		{"vmstat", model.SuffixVmstat, "procs -----------memory---------- ---swap-- -----io----\n r  b   swpd   free   buff   cache   si   so    bi    bo\n"},
		{"innodbstatus", model.SuffixInnodbStatus, "=====================================\n2026-04-21 16:51:41 INNODB MONITOR OUTPUT\n"},
		{"mysqladmin", model.SuffixMysqladmin, "+----------------------------------+----+\n| Variable_name                    | 00 |\n+----------------------------------+----+\n"},
		{"processlist", model.SuffixProcesslist, "+-----+------+-----------+------+---------+------+-------+----+\n| Id  | User | Host      | db   | Command | Time | State | In |\n"},
	}

	// Negative cases: bytes that don't match the expected header.
	// Using the same suffix set; the peek is either empty or a
	// plausible-looking mimic that lacks the real header token.
	// Each mimic intentionally includes a substring similar to the
	// collector's name to make sure the detector isn't accidentally
	// doing a filename check — it should pin on the STRUCTURAL
	// signature, not the collector's label.
	negatives := []struct {
		name   string
		suffix model.Suffix
		peek   string
	}{
		{"iostat-empty", model.SuffixIostat, ""},
		{"top-empty", model.SuffixTop, "just a banner without the process header\nthread list omitted\n"},
		{"variables-no-header", model.SuffixVariables, "some-mysql-output\nrandom table content\n"},
		{"vmstat-partial", model.SuffixVmstat, "procs\n"}, // has "procs" but no "memory" — both are required
		{"innodbstatus-empty", model.SuffixInnodbStatus, "this file is not an innodb status dump\n"},
		{"mysqladmin-no-rules", model.SuffixMysqladmin, "Variable_name\nno-pipe-rules\n"}, // has Variable_name but no "+-" ruler
		{"processlist-no-state", model.SuffixProcesslist, "Command\nno-state-column\n"},
	}

	for _, tc := range positives {
		tc := tc
		t.Run("positive_"+tc.name, func(t *testing.T) {
			got := DetectFormat([]byte(tc.peek), tc.suffix)
			if got != model.FormatV1 {
				t.Errorf("DetectFormat(%s V1-header) = %v; want FormatV1. The header signature that drives parse/version.go::detect%s regressed.",
					tc.suffix, got, capitaliseSuffix(string(tc.suffix)))
			}
		})
	}
	for _, tc := range negatives {
		tc := tc
		t.Run("negative_"+tc.name, func(t *testing.T) {
			got := DetectFormat([]byte(tc.peek), tc.suffix)
			if got != model.FormatUnknown {
				t.Errorf("DetectFormat(%s non-V1-header) = %v; want FormatUnknown. Per FR-024 the detector prefers Unknown over silent misclassification; a regression that broadened the match condition breaks the 'unsupported pt-stalk version' banner surface.",
					tc.suffix, got)
			}
		})
	}

	// Default branch: an unknown Suffix value should always resolve
	// to FormatUnknown regardless of the peeked content. Exercises
	// the `default:` case in DetectFormat.
	t.Run("unknown_suffix", func(t *testing.T) {
		got := DetectFormat([]byte("anything at all"), model.Suffix("this-suffix-does-not-exist"))
		if got != model.FormatUnknown {
			t.Errorf("DetectFormat(unknown suffix) = %v; want FormatUnknown. The default branch must never accidentally resolve to a known version.",
				got)
		}
	})
}

// capitaliseSuffix returns the Suffix string with its first rune
// upper-cased (iostat → Iostat, innodbstatus1 → Innodbstatus1). Used
// only for error message construction so the pointer to the broken
// detector helper name matches Go identifier conventions.
func capitaliseSuffix(s string) string {
	if s == "" {
		return ""
	}
	// InnodbStatus is the Go identifier for suffix innodbstatus1;
	// match the parse/version.go naming rather than naive title-case.
	if strings.HasPrefix(s, "innodbstatus") {
		return "InnodbStatus"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
