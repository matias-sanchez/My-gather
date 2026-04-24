package parse

import (
	"strings"
)

// ParseSysctl returns the subset of sysctl keys the Environment panel
// surfaces. A single linear pass over the file keeps iteration order
// deterministic; unknown keys are ignored. Missing keys are absent from
// the returned map, which the render layer handles as "—".
//
// Only the keys in sysctlWantedKeys are pulled. Adding a new key is a
// one-line change here and a template edit to render it.
func ParseSysctl(content string) map[string]string {
	wanted := map[string]struct{}{
		"kernel.osrelease":          {},
		"kernel.version":            {},
		"crypto.fips_name":          {},
		"vm.swappiness":             {},
		"vm.dirty_ratio":            {},
		"vm.dirty_background_ratio": {},
		"fs.file-max":               {},
	}
	out := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r")
		// "key = value" — value may contain = (e.g. some dev.cdrom.info
		// lines), so split on the FIRST ` = ` occurrence only.
		idx := strings.Index(line, " = ")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		if _, ok := wanted[key]; !ok {
			continue
		}
		value := strings.TrimSpace(line[idx+3:])
		// Last write wins — matches "most recent sample" semantics when
		// a file is a concatenation of several snapshots.
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
