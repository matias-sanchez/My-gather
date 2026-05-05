// Package reportutil contains shared, deterministic helpers for report
// formatting and report value lookup.
package reportutil

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

// FormatNum renders a float with a stable precision for report text.
func FormatNum(v float64) string {
	if math.IsNaN(v) {
		return "NaN"
	}
	if math.IsInf(v, 0) {
		if v > 0 {
			return "\u221e"
		}
		return "-\u221e"
	}
	if v == 0 {
		return "0"
	}
	abs := math.Abs(v)
	if abs >= 1 && math.Abs(v-math.Round(v)) < 1e-9 {
		return HumanInt(int64(math.Round(v)))
	}
	if abs >= 1 {
		return fmt.Sprintf("%.2f", v)
	}
	if abs >= 0.001 {
		return fmt.Sprintf("%.4f", v)
	}
	return fmt.Sprintf("%.2e", v)
}

// HumanInt renders a signed integer with comma group separators.
func HumanInt(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
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

// HumanBytes renders a byte count using binary units.
func HumanBytes(v float64) string {
	const (
		KiB = 1024.0
		MiB = KiB * 1024
		GiB = MiB * 1024
		TiB = GiB * 1024
	)
	switch {
	case v >= TiB:
		return fmt.Sprintf("%.2f TiB", v/TiB)
	case v >= GiB:
		return fmt.Sprintf("%.2f GiB", v/GiB)
	case v >= MiB:
		return fmt.Sprintf("%.0f MiB", v/MiB)
	case v >= KiB:
		return fmt.Sprintf("%.0f KiB", v/KiB)
	default:
		return fmt.Sprintf("%.0f B", v)
	}
}

// HumanKBBytes renders a positive kibibyte count as bytes using binary units.
func HumanKBBytes(kb int64) string {
	if kb <= 0 {
		return ""
	}
	return HumanBytes(float64(kb) * 1024.0)
}

// GaugeLast returns the most recent finite value for a mysqladmin gauge.
func GaugeLast(r *model.Report, name string) (float64, bool) {
	if r == nil || r.DBSection == nil || r.DBSection.Mysqladmin == nil {
		return 0, false
	}
	m := r.DBSection.Mysqladmin
	if m.IsCounter[name] {
		return 0, false
	}
	arr, ok := m.Deltas[name]
	if !ok || len(arr) == 0 {
		return 0, false
	}
	for i := len(arr) - 1; i >= 0; i-- {
		v := arr[i]
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			return v, true
		}
	}
	return 0, false
}

// VariableFloat resolves a SHOW GLOBAL VARIABLES value as a float.
func VariableFloat(r *model.Report, name string) (float64, bool) {
	v, ok := VariableRaw(r, name)
	if !ok {
		return 0, false
	}
	v = strings.TrimSpace(v)
	if v == "" || strings.EqualFold(v, "NULL") {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// VariableRaw returns the latest raw SHOW GLOBAL VARIABLES value.
func VariableRaw(r *model.Report, name string) (string, bool) {
	if r == nil || r.VariablesSection == nil {
		return "", false
	}
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
