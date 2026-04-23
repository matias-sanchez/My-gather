package parse

import (
	"strings"
)

// ParseEnvHostname returns the resolved hostname from a -hostname file
// contents string. pt-stalk writes the command output verbatim, so the
// file is typically just one line. On rare hosts the capture emits
// warnings (e.g. "sudo: unable to resolve …") before the real value —
// we take the LAST non-empty line that doesn't look like a warning.
//
// Returns "" when no usable line was found.
func ParseEnvHostname(content string) string {
	var best string
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		// Heuristics for warning lines we've seen in the wild.
		if strings.HasPrefix(lower, "sudo:") {
			continue
		}
		if strings.HasPrefix(lower, "warning:") {
			continue
		}
		if strings.Contains(lower, "unable to resolve host") {
			continue
		}
		best = line
	}
	return best
}
