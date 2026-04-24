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

// ParseTopHeader scans a -top file contents string and returns the
// load-average values from the LATEST `top - …` header line. pt-stalk
// `-top` captures typically contain many samples per file, so returning
// the first header understates current load on long/busy captures.
// Returns nil when no recognisable top header is found.
func ParseTopHeader(content string) *model.EnvTopHeader {
	var result *model.EnvTopHeader
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(line, "top - ") {
			continue
		}
		m := topLoadAvgRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		one, _ := strconv.ParseFloat(m[1], 64)
		five, _ := strconv.ParseFloat(m[2], 64)
		fifteen, _ := strconv.ParseFloat(m[3], 64)
		result = &model.EnvTopHeader{
			Loadavg1:  one,
			Loadavg5:  five,
			Loadavg15: fifteen,
		}
	}
	return result
}
