// Package render converts a *model.Collection into one self-contained
// HTML document per Constitution Principle V. Its public surface is
// documented in specs/001-ptstalk-report-mvp/contracts/packages.md.
//
// deterministic.go is the project's single determinism choke point.
// Every render path that could introduce order- or locale-dependence
// routes through helpers here. Constitution Principle IV (Deterministic
// Output) is enforced by golden-file round-trip tests plus the
// TestDeterminism twin-render in render_test.go.
package render

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/matias-sanchez/My-gather/model"
)

// sortedKeys returns the keys of m sorted in ascending order. Templates
// and render-side logic iterate this slice instead of ranging over the
// map directly, guaranteeing deterministic output.
func sortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// entry is a (key, value) pair used by sortedEntries.
type entry[K cmp.Ordered, V any] struct {
	Key   K
	Value V
}

// sortedEntries returns the entries of m sorted ascending by key.
func sortedEntries[K cmp.Ordered, V any](m map[K]V) []entry[K, V] {
	entries := make([]entry[K, V], 0, len(m))
	for k, v := range m {
		entries = append(entries, entry[K, V]{Key: k, Value: v})
	}
	slices.SortFunc(entries, func(a, b entry[K, V]) int {
		return cmp.Compare(a.Key, b.Key)
	})
	return entries
}

// formatFloat renders a float64 as a decimal string with exactly
// `precision` fractional digits. Never uses scientific notation, never
// produces "-0" for zero, and is locale-insensitive (strconv is always
// "C locale").
//
// Used for every numeric value embedded in HTML output and in the JSON
// data payload. Callers MUST pick a precision appropriate to the
// metric: 4 for percentages, 2 for byte/sec values, 0 for integer
// counts.
func formatFloat(v float64, precision int) string {
	if v == 0 {
		v = 0 // normalise -0.0 to +0.0
	}
	return strconv.FormatFloat(v, 'f', precision, 64)
}

// formatTimestamp renders a time as RFC-3339 in UTC with nanosecond
// precision stripped to seconds. All rendered timestamps flow through
// this helper.
func formatTimestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// ordinalIDGenerator produces deterministic, ordinal element IDs. A
// single generator is created per render; every section-header,
// subview, and chart-container ID in the output comes from it. No
// UUIDs, no time-based IDs, no math/rand.
type ordinalIDGenerator struct {
	n int
}

// newOrdinalIDGenerator returns a fresh generator seeded to 1.
func newOrdinalIDGenerator() *ordinalIDGenerator {
	return &ordinalIDGenerator{n: 0}
}

// Next returns the next ID prefixed by the caller-supplied tag,
// incrementing the internal counter. Example: (*og).Next("sec") ->
// "sec-1", "sec-2", ...
func (g *ordinalIDGenerator) Next(tag string) string {
	g.n++
	return tag + "-" + strconv.Itoa(g.n)
}

// canonicalReportID computes the stable-per-report identifier used as
// the prefix for localStorage keys in the collapse/expand feature
// (spec FR-032). It is a SHA-256 hash of a canonicalised subset of
// the Collection truncated to 12 hex characters.
//
// Excluded from the hash (per F21 / research R9):
//   - Collection.RootPath: moving the dump should not reset state.
//   - Report.GeneratedAt: this is render metadata, not part of the
//     input identity.
//
// Included in the hash:
//   - Collection.Hostname
//   - Each Snapshot's Prefix and Timestamp, iterated in the order
//     they appear on the Collection (already timestamp-sorted by
//     parse.Discover).
//   - Each Snapshot's SourceFiles (suffix + size only; we do NOT hash
//     file contents, since that would require reading every file
//     twice and does not add meaningful distinguishing power at the
//     12-hex-char truncation level).
//
// canonicalReportID is deterministic: the same inputs produce the same
// 12-char output across runs and across hosts.
func canonicalReportID(c *model.Collection) string {
	h := sha256.New()
	h.Write([]byte("mygather-report-id-v1\n"))
	h.Write([]byte("hostname:"))
	h.Write([]byte(c.Hostname))
	h.Write([]byte{'\n'})
	for _, snap := range c.Snapshots {
		h.Write([]byte("snap:"))
		h.Write([]byte(snap.Prefix))
		h.Write([]byte{'\t'})
		h.Write([]byte(snap.Timestamp.UTC().Format(time.RFC3339)))
		h.Write([]byte{'\n'})
		// Iterate SourceFiles in the canonical Suffix order so the
		// hash is stable regardless of map iteration order.
		for _, suffix := range model.KnownSuffixes {
			sf, ok := snap.SourceFiles[suffix]
			if !ok {
				continue
			}
			h.Write([]byte("  file:"))
			h.Write([]byte(suffix))
			h.Write([]byte{'\t'})
			h.Write([]byte(strconv.FormatInt(sf.Size, 10)))
			h.Write([]byte{'\n'})
		}
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)[:12]
}

// sortStrings returns a sorted copy of s. Convenience wrapper used by
// render code that mustn't mutate its input slices.
func sortStrings(s []string) []string {
	out := make([]string, len(s))
	copy(out, s)
	sort.Strings(out)
	return out
}
