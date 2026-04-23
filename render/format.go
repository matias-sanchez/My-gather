package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// formatNum renders a float with a sensible precision for display in
// the FormulaComputed line and the findings metrics table. Integers
// print without decimals; fractions get up to 2 decimal places; very
// small fractions use scientific notation. Display formatting is a
// render-layer concern; this helper lives here so findings/ does not
// need to export format helpers solely for the template layer.
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
func humanInt(n int64) string {
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
