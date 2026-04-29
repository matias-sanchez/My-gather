package findings

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

// mysqladmin returns the merged MysqladminData on the report, or nil
// if the collection has no -mysqladmin files. Rules that depend on
// counter/gauge series must skip when this returns nil.
func mysqladmin(r *model.Report) *model.MysqladminData {
	if r == nil || r.DBSection == nil {
		return nil
	}
	return r.DBSection.Mysqladmin
}

// counterTotal returns the sum of per-sample deltas for the named
// counter variable across the capture, excluding the bogus first
// sample (index 0: pt-mext stores the initial raw tally there — see
// render.go tStart handling). Returns ok=false when the variable is
// absent or is classified as a gauge.
func counterTotal(r *model.Report, name string) (float64, bool) {
	m := mysqladmin(r)
	if m == nil {
		return 0, false
	}
	if !m.IsCounter[name] {
		return 0, false
	}
	arr, ok := m.Deltas[name]
	if !ok || len(arr) < 2 {
		return 0, false
	}
	var total float64
	// Skip index 0 (initial tally). Also skip NaN slots (snapshot
	// boundary markers — see render/render.go concat logic).
	for i := 1; i < len(arr); i++ {
		v := arr[i]
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		total += v
	}
	return total, true
}

// counterRatePerSec divides counterTotal by the capture's observed
// seconds and returns the per-second rate. Skipped the same way as
// counterTotal.
//
// The denominator comes from captureSeconds, which sums per-snapshot
// spans only (gaps between distinct pt-stalk snapshots are excluded
// — see that function's doc). Within a snapshot the denominator is
// (N-1)·step, which matches counterTotal's (N-1) deltas, so the rate
// is accurate to the sampling precision.
func counterRatePerSec(r *model.Report, name string) (float64, bool) {
	total, ok := counterTotal(r, name)
	if !ok {
		return 0, false
	}
	secs := captureSeconds(r)
	if secs <= 0 {
		return 0, false
	}
	return total / secs, true
}

// gaugeLast returns the most recent observed value for a gauge
// variable. Returns ok=false when the variable is absent or
// classified as a counter.
func gaugeLast(r *model.Report, name string) (float64, bool) {
	m := mysqladmin(r)
	if m == nil {
		return 0, false
	}
	if m.IsCounter[name] {
		return 0, false
	}
	arr, ok := m.Deltas[name]
	if !ok || len(arr) == 0 {
		return 0, false
	}
	// Walk back from the tail until we find a non-NaN value.
	for i := len(arr) - 1; i >= 0; i-- {
		v := arr[i]
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			return v, true
		}
	}
	return 0, false
}

// gaugeMax returns the maximum observed value for a gauge variable
// across the capture.
func gaugeMax(r *model.Report, name string) (float64, bool) {
	m := mysqladmin(r)
	if m == nil {
		return 0, false
	}
	if m.IsCounter[name] {
		return 0, false
	}
	arr, ok := m.Deltas[name]
	if !ok || len(arr) == 0 {
		return 0, false
	}
	var (
		seen bool
		best = math.Inf(-1)
	)
	for _, v := range arr {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		if v > best {
			best = v
			seen = true
		}
	}
	if !seen {
		return 0, false
	}
	return best, true
}

// captureSeconds returns the total observed seconds covered by the
// mysqladmin capture — i.e. the sum of per-snapshot wall-clock spans
// only. Gaps BETWEEN snapshots (no collection was happening during
// those intervals) are excluded, because counter deltas are not
// observed across a boundary (model.MergeMysqladminData inserts NaN there) and
// including the gap would artificially inflate the denominator and
// suppress advisor rates.
//
// For a single-snapshot capture this reduces to `last - first`.
// Returns 0 when there is insufficient data.
func captureSeconds(r *model.Report) float64 {
	m := mysqladmin(r)
	if m == nil || len(m.Timestamps) < 2 {
		return 0
	}
	ts := m.Timestamps
	// Derive block boundaries. Each block spans [start, end] where
	// start is the first sample of a snapshot and end is the last
	// sample before the next snapshot's start (or the final sample
	// overall). SnapshotBoundaries is guaranteed to start with 0.
	boundaries := m.SnapshotBoundaries
	if len(boundaries) == 0 {
		boundaries = []int{0}
	}
	var total float64
	for i, start := range boundaries {
		var end int
		if i+1 < len(boundaries) {
			end = boundaries[i+1] - 1
		} else {
			end = len(ts) - 1
		}
		if end <= start || start < 0 || end >= len(ts) {
			continue
		}
		d := ts[end].Sub(ts[start]).Seconds()
		if d > 0 {
			total += d
		}
	}
	return total
}

// variableFloat resolves a SHOW GLOBAL VARIABLES entry to a float.
// Picks the last snapshot's value (pt-stalk may capture variables
// per-snapshot; the final value is the most representative of
// steady-state configuration). Returns ok=false when the variable is
// absent or not numeric.
func variableFloat(r *model.Report, name string) (float64, bool) {
	v, ok := variableRaw(r, name)
	if !ok {
		return 0, false
	}
	v = strings.TrimSpace(v)
	if v == "" || v == "NULL" {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// variableRaw returns the raw string value of a SHOW GLOBAL VARIABLES
// entry. Comparison is case-insensitive.
func variableRaw(r *model.Report, name string) (string, bool) {
	if r == nil || r.VariablesSection == nil {
		return "", false
	}
	// Walk snapshots in reverse; last one wins.
	snaps := r.VariablesSection.PerSnapshot
	for i := len(snaps) - 1; i >= 0; i-- {
		data := snaps[i].Data
		if data == nil {
			continue
		}
		for _, e := range data.Entries {
			if strings.EqualFold(e.Name, name) {
				return e.Value, true
			}
		}
	}
	return "", false
}

// missingInputEvidence records unavailable inputs as low-strength
// context. It is intentionally not used by rules that skip outright:
// missing inputs must not create or escalate a warning.
func missingInputEvidence(names ...string) EvidenceRef {
	return EvidenceRef{
		Name:     "Missing inputs",
		Value:    strings.Join(names, ", "),
		Kind:     EvidenceInference,
		Strength: EvidenceWeak,
		Note:     "not used to escalate severity",
	}
}

// formatNum renders a float with a sensible precision for display in
// the FormulaComputed line. Integers print without decimals;
// fractions get up to 2 decimal places; very small fractions use
// scientific notation. Used internally by findings rules; display
// formatting for the render layer lives in render/format.go.
func formatNum(v float64) string {
	if math.IsNaN(v) {
		return "NaN"
	}
	if math.IsInf(v, 0) {
		if v > 0 {
			return "∞"
		}
		return "-∞"
	}
	if v == 0 {
		return "0"
	}
	abs := math.Abs(v)
	if abs >= 1 && math.Abs(v-math.Round(v)) < 1e-9 {
		return humanInt(int64(math.Round(v)))
	}
	if abs >= 1 {
		return fmt.Sprintf("%.2f", v)
	}
	if abs >= 0.001 {
		return fmt.Sprintf("%.4f", v)
	}
	return fmt.Sprintf("%.2e", v)
}

// humanInt prints a signed integer with thousand-separator commas.
// Used internally by formatNum.
func humanInt(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	// Insert commas every 3 digits from the right.
	if len(s) <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var out strings.Builder
	first := len(s) % 3
	if first > 0 {
		out.WriteString(s[:first])
		if len(s) > first {
			out.WriteByte(',')
		}
	}
	for i := first; i < len(s); i += 3 {
		out.WriteString(s[i : i+3])
		if i+3 < len(s) {
			out.WriteByte(',')
		}
	}
	if neg {
		return "-" + out.String()
	}
	return out.String()
}

// formatPercent renders a ratio 0..1 as "99.88 %".
func formatPercent(v float64) string {
	return fmt.Sprintf("%.2f %%", v*100)
}
