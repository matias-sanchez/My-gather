package parse

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/matias-sanchez/My-gather/model"
)

// topLoadAvgRE captures the three load-average numbers from a standard
// -top header line:
//
//	top - 15:57:38 up 5 days, 10:43, 10 users,  load average: 0.41, 0.49, 0.64
var topLoadAvgRE = regexp.MustCompile(
	`load average:\s*([0-9]+(?:\.[0-9]+)?),\s*([0-9]+(?:\.[0-9]+)?),\s*([0-9]+(?:\.[0-9]+)?)`)

// ParseTopHeader reads the first `top - …` line of a -top file contents
// string and returns the three load-average values. Returns nil when the
// input does not start with a recognisable top header.
func ParseTopHeader(content string) *model.EnvTopHeader {
	// Find the first header line anywhere near the start — pt-stalk
	// sometimes pre-pends a TS marker before the first top header.
	for _, line := range strings.SplitN(content, "\n", 8) {
		if !strings.HasPrefix(line, "top - ") {
			continue
		}
		m := topLoadAvgRE.FindStringSubmatch(line)
		if m == nil {
			return nil
		}
		one, _ := strconv.ParseFloat(m[1], 64)
		five, _ := strconv.ParseFloat(m[2], 64)
		fifteen, _ := strconv.ParseFloat(m[3], 64)
		return &model.EnvTopHeader{
			Loadavg1:  one,
			Loadavg5:  five,
			Loadavg15: fifteen,
		}
	}
	return nil
}
